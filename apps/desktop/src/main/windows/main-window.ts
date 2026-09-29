import { BrowserWindow, nativeImage, screen } from 'electron';
import path from 'node:path';
import { loadView, viewWebPreferences } from './view-helper';
import { getAdvanced } from '../settings';

/** 主窗口默认宽度/高度：窄条便签列表，停靠桌面右下角 */
const MAIN_WIDTH = 320;
const MAIN_HEIGHT = 600;
/** 可调尺寸的下限：再小搜索框/标签栏就挤不下了 */
const MAIN_MIN_WIDTH = 280;
const MAIN_MIN_HEIGHT = 400;

/** 创建主窗口：笔记列表、搜索、管理入口（窄条面板）。
 *  宽高可调（issue #2 反馈：默认太高；2026-09 起放开，取代 2026-07-21 的
 *  固定尺寸评审结论）——但**不做尺寸记忆**，每次打开恢复默认 320×600：
 *  既满足临时拉大的需要，又规避高分屏缩放下 bounds 往返存储漂移越变越小的
 *  老问题。不做位置记忆：永远停靠主屏幕右下角，位置可预期 */
export function createMainWindow(): BrowserWindow {
  const { workArea } = screen.getPrimaryDisplay();
  const win = new BrowserWindow({
    // X11 下窗口/任务栏图标(Windows 无边框窗不显示、macOS 忽略,均无害)
    icon: nativeImage.createFromPath(path.join(__dirname, '../../../resources/icon.png')),
    width: MAIN_WIDTH,
    height: MAIN_HEIGHT,
    minWidth: MAIN_MIN_WIDTH,
    minHeight: MAIN_MIN_HEIGHT,
    x: workArea.x + workArea.width - MAIN_WIDTH - 12, // 离屏幕工作区边缘留 12px
    y: workArea.y + workArea.height - MAIN_HEIGHT - 12,
    title: 'PinSlip',
    resizable: true,
    maximizable: false,
    show: false,
    // 任务栏图标显隐（高级定制，缺省开）；仅主窗口，便签任务栏入口不受影响
    skipTaskbar: !getAdvanced().taskbarIcon,
    autoHideMenuBar: true,
    webPreferences: viewWebPreferences(),
  });

  loadView(win, '/');

  win.once('ready-to-show', () => win.show());
  return win;
}
