import { globalShortcut } from 'electron';
import type { WindowManager } from './windows/window-manager';
import { getAdvanced } from './settings';
import type { BlankNoteShortcut } from '../shared/types';

/** 空白便签快捷键：预设键位 → electron accelerator 白名单映射。
 *  与既有速记 CommandOrControl+Shift+N 无交集 */
const BLANK_NOTE_ACCELERATORS: Record<Exclude<BlankNoteShortcut, 'off'>, string> = {
  'ctrl+alt+n': 'CommandOrControl+Alt+N',
  'ctrl+shift+alt+n': 'CommandOrControl+Shift+Alt+N',
  'ctrl+alt+insert': 'CommandOrControl+Alt+Insert',
};

/** 当前已注册的空白便签快捷键键位（'off' = 未注册）；原子重绑的回滚依据 */
let blankNoteBinding: BlankNoteShortcut = 'off';
/** 快捷键按下 handler：registerShortcuts 时登记，set-advanced 重绑复用同一份 */
let blankNoteOnFire: () => void = () => {};

/** 键位 → accelerator；非法值（settings.json 手改/渲染层注入）回退 null = 不注册 */
function blankNoteAccelerator(key: string | undefined): string | null {
  if (!key || key === 'off') return null;
  return BLANK_NOTE_ACCELERATORS[key as Exclude<BlankNoteShortcut, 'off'>] ?? null;
}

/**
 * 空白便签快捷键原子重绑：新键注册失败（被他应用占用）→ 回滚旧绑定 → 抛错，
 * 不静默丢键；'off' 时注销该键。
 */
export function registerBlankNoteShortcut(onFire: () => void, key: BlankNoteShortcut): void {
  const nextAcc = blankNoteAccelerator(key);
  const prevKey = blankNoteBinding;
  const prevAcc = blankNoteAccelerator(prevKey);
  if (nextAcc === prevAcc) return;
  if (prevAcc) globalShortcut.unregister(prevAcc);
  blankNoteBinding = 'off';
  if (!nextAcc) return;
  if (globalShortcut.register(nextAcc, onFire)) {
    blankNoteBinding = key;
    return;
  }
  // 新键被占用：回滚旧绑定（回滚也失败则保持未注册，错误照常上报）
  if (prevAcc && globalShortcut.register(prevAcc, onFire)) {
    blankNoteBinding = prevKey;
  }
  throw new Error(`blank note shortcut register failed: ${nextAcc}`);
}

/** set-advanced 联动：按新键位原子重绑（handler 复用启动时登记的那份） */
export function rebindBlankNoteShortcut(key: BlankNoteShortcut): void {
  registerBlankNoteShortcut(blankNoteOnFire, key);
}

/** 注册全局快捷键。 */
export function registerShortcuts(windowManager: WindowManager): void {
  // 速记浮窗：即使应用没有窗口打开也能呼出
  globalShortcut.register('CommandOrControl+Shift+N', () => {
    windowManager.showQuickCapture();
  });

  // 空白便签快捷键（缺省 off 不注册）：按下在根目录新建空白便签并聚焦——
  // 仅快捷键路径 focus，会话恢复/列表打开等建窗路径不抢焦点
  blankNoteOnFire = () => {
    windowManager
      .createNoteWindow()
      .then((id) => windowManager.focusNoteWindow(id))
      .catch((err) => console.error('[shortcut] blank note create failed:', err));
  };
  try {
    registerBlankNoteShortcut(blankNoteOnFire, getAdvanced().blankNoteShortcut);
  } catch (err) {
    console.error('[shortcut] blank note shortcut register failed at startup:', err);
  }
}

/** 注销全部全局快捷键（退出前调用）。unregisterAll 天然覆盖空白便签快捷键。 */
export function unregisterShortcuts(): void {
  globalShortcut.unregisterAll();
  blankNoteBinding = 'off';
}
