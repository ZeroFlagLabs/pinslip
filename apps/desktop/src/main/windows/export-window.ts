import { BrowserWindow, clipboard, dialog, ipcMain } from 'electron';
import { writeFile } from 'node:fs/promises';
import { IPC } from '../../shared/ipc-channels';
import type { ExportImagePayload, ExportImageResult } from '../../shared/types';
import { loadView, viewWebPreferences } from './view-helper';
import { tMain } from '../i18n';

/** 导出图内容宽度（CSS px）；实际分辨率 = 所在屏 scaleFactor × 该值 */
const EXPORT_WIDTH = 840;
/** 上报高度的合法区间：低于下限按下限、超出上限裁切（不做长图分页） */
const MIN_HEIGHT = 120;
const MAX_HEIGHT = 5000;
/** 等 ExportReady 的超时：超时按当前窗口内容直接截图兜底，不整单失败 */
const READY_TIMEOUT_MS = 10_000;
/** setContentSize 后的重排/重绘稳定延时（隐藏窗 capturePage 空白帧规避） */
const SETTLE_DELAY_MS = 80;

/** 并发纪律：同一时间只允许一次导出（隐藏窗/IPC 匹配都是单例假设） */
let exporting = false;

/** 保存对话框默认文件名：去 Windows 非法字符、截 50 字，空标题回退 note */
function sanitizeFileName(title: string): string {
  const cleaned = title.replace(/[\\/:*?"<>|]/g, '').trim().slice(0, 50);
  return cleaned || 'note';
}

/** 导出全流程：隐藏离屏窗渲染 ExportView → 就绪后定稿高度 → capturePage →
 *  copy 写剪贴板 / save 弹保存对话框写盘。任何一步出错都清理现场并返回
 *  { ok:false, error }；取消保存对话框返回 { ok:true, canceled:true } */
export async function runExport(
  payload: ExportImagePayload,
  event: Electron.IpcMainInvokeEvent,
): Promise<ExportImageResult> {
  if (exporting) return { ok: false, error: 'export already in progress' };
  exporting = true;
  const caller = BrowserWindow.fromWebContents(event.sender);
  let win: BrowserWindow | null = null;
  try {
    win = new BrowserWindow({
      show: false, // 隐藏窗；paintWhenInitiallyHidden 缺省 true，离屏可绘制
      frame: false,
      width: EXPORT_WIDTH,
      height: 600, // 初始高度占位，ExportReady 后按内容定稿
      enableLargerThanScreen: true, // 超高内容（上限 5000）可超出屏幕
      resizable: false,
      skipTaskbar: true,
      webPreferences: viewWebPreferences(),
    });
    const contents = win.webContents;
    loadView(win, '/export');
    await new Promise<void>((resolve, reject) => {
      contents.once('did-finish-load', () => resolve());
      contents.once('did-fail-load', (_e, code, desc) =>
        reject(new Error(`export view load failed (${code}): ${desc}`)),
      );
    });
    contents.send(IPC.ExportPayload, payload);

    // 等 ExportView 上报内容高度；按 sender.id 匹配导出窗，避免频道多窗口串扰
    const reported = await new Promise<number>((resolve) => {
      const cleanup = () => {
        clearTimeout(timer);
        ipcMain.removeListener(IPC.ExportReady, listener);
      };
      const listener = (e: Electron.IpcMainEvent, height: number) => {
        if (e.sender.id !== contents.id) return;
        cleanup();
        resolve(typeof height === 'number' && height > 0 ? height : 0);
      };
      const timer = setTimeout(() => {
        cleanup();
        resolve(0); // 超时兜底：保留初始高度直接截图
      }, READY_TIMEOUT_MS);
      ipcMain.on(IPC.ExportReady, listener);
    });
    if (reported > 0) {
      win.setContentSize(EXPORT_WIDTH, Math.min(Math.max(Math.ceil(reported), MIN_HEIGHT), MAX_HEIGHT));
    }
    await new Promise((r) => setTimeout(r, SETTLE_DELAY_MS));
    const image = await contents.capturePage();

    if (payload.action === 'copy') {
      clipboard.writeImage(image);
      return { ok: true };
    }
    const res = await dialog.showSaveDialog(caller ?? win, {
      title: tMain('dialog.exportSaveTitle'),
      buttonLabel: tMain('dialog.exportSaveButton'),
      defaultPath: `${sanitizeFileName(payload.title)}.png`,
      filters: [{ name: 'PNG', extensions: ['png'] }],
    });
    if (res.canceled || !res.filePath) return { ok: true, canceled: true };
    await writeFile(res.filePath, image.toPNG());
    return { ok: true };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : String(err) };
  } finally {
    exporting = false;
    win?.destroy();
  }
}
