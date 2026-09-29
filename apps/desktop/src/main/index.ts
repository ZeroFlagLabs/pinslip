import { app, BrowserWindow, nativeTheme, net, protocol } from 'electron';
import { electronApp, optimizer } from '@electron-toolkit/utils';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { WindowManager } from './windows/window-manager';
import { GoProcess, VAULT_NOT_SET } from './services/go-process';
import { registerIpcHandlers } from './ipc';
import { createTray } from './tray';
import { initAutoStart } from './autostart';
import { getVaultPath, getAdvanced } from './settings';
import { initMainI18n } from './i18n';
import { registerShortcuts, unregisterShortcuts } from './shortcuts';
import { startVaultWatch, stopVaultWatch } from './services/vault-watch';
import { initAutoUpdater } from './updater';
import { flushWindowState } from './windows/window-state';
import { IPC } from '../shared/ipc-channels';

// 只做装配：app 生命周期 + 各模块注册，业务逻辑分散到各模块
const goProcess = new GoProcess();
let windowManager: WindowManager;

// 单实例锁：重复启动（任务栏固定图标/安装后自动运行 + 托盘常驻）时，
// 第二实例直接退出并把已有实例的主窗口带到前台。
// 没有这把锁时每个实例都会建托盘图标 + 在记忆的同一位置开主窗口，
// 半透明面板下两个窗口叠成「文字重影」，难以察觉是多开了实例。
// 未拿到锁时 app.quit()，whenReady 不会触发，后续注册全部无副作用。
if (!app.requestSingleInstanceLock()) {
  app.quit();
}
app.on('second-instance', () => {
  windowManager?.showMainWindow();
});

app.whenReady().then(() => {
  electronApp.setAppUserModelId('app.pinslip');

  windowManager = new WindowManager(goProcess);

  // pinslip-img://attachments/<name> → vault 内图片（渲染端 img src 专用）。
  // 选自定义协议而非 http://127.0.0.1:<port>：Go 端口每次启动随机，
  // 写进 markdown 会过期；协议由主进程直接从 vault 读文件，与端口无关。
  // 只允许 attachments/ 前缀且拒绝 ..，防目录穿越。
  protocol.handle('pinslip-img', (req) => {
    const vault = getVaultPath();
    const rel = decodeURIComponent(req.url.slice('pinslip-img://'.length));
    if (!vault || !rel.startsWith('attachments/') || rel.includes('..')) {
      return new Response('forbidden', { status: 403 });
    }
    return net
      .fetch(pathToFileURL(path.join(vault, rel)).toString())
      .catch(() => new Response('not found', { status: 404 }));
  });

  // 拉起 Go 本地服务（未设置保险库时静默跳过，等用户选择后启动；
  // 其他失败不阻塞 UI，渲染层会提示服务不可用）
  goProcess.ensureStarted().catch((err) => {
    if (err.message !== VAULT_NOT_SET) {
      console.error('[main] Go 服务启动失败:', err);
    }
  });

  registerIpcHandlers({ windowManager, goProcess });
  // OS 深色模式变更广播：managerTheme='system' 时渲染层即时跟进（复用语言广播模式）；
  // main 只供事实，「system ? osDark : 偏好」的合成留在渲染层
  nativeTheme.on('updated', () => {
    for (const win of BrowserWindow.getAllWindows()) {
      if (!win.isDestroyed()) win.webContents.send(IPC.OsThemeChanged, nativeTheme.shouldUseDarkColors);
    }
  });
  initMainI18n(); // 主进程 i18n（托盘/更新文案），须在 createTray 之前
  // 托盘图标可在「高级定制」关闭：关闭时启动不创建托盘（运行中切换走 IPC 立即应用）
  if (getAdvanced().trayIcon) createTray(windowManager);
  registerShortcuts(windowManager);
  initAutoStart(); // 打包后首次运行默认开启开机自启
  initAutoUpdater(); // 打包后启动 15s 静默检查更新（dev 短路）

  // 外部文件变更监听：同步盘/手动改 vault 时主界面列表自动刷新（未设保险库时为 no-op）
  startVaultWatch(() => windowManager.broadcastNotesChanged());

  windowManager.showMainWindow();
  // 会话恢复：已设保险库时，重新打开上次退出时开着的便签
  if (getVaultPath()) {
    windowManager.restoreNoteWindows().catch((err) =>
      console.error('[main] 会话恢复失败:', err),
    );
  }

  app.on('browser-window-created', (_, window) => {
    optimizer.watchWindowShortcuts(window);
  });
});

// 托盘常驻应用：托盘开启时所有窗口关闭不退出，由托盘菜单退出；
// 托盘关闭（高级定制）时没有常驻入口，全部窗口关闭即退出（不做幽灵常驻）
app.on('window-all-closed', () => {
  if (!getAdvanced().trayIcon) app.quit();
});

app.on('before-quit', () => {
  if (windowManager) windowManager.quitting = true; // 放行主窗口的 close 拦截，确保能真正退出
  unregisterShortcuts();
  stopVaultWatch();
  flushWindowState(); // 窗口状态保存有 500ms 防抖，退出前立即落盘避免丢最后一笔
  goProcess.stop();
});

app.on('activate', () => {
  if (BrowserWindow.getAllWindows().length === 0) {
    windowManager.showMainWindow();
  }
});
