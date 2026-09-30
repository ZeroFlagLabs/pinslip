import type { NoteColor } from '@shared/types';

/** 六色定义：dot = 标题栏色；ink = 同色加深版（颜色按钮图标着色，
 *  直接用标题栏本色会在同色标题栏上隐身，故用 600 档深色）；
 *  exportBg = 导出图背景渐变色对 [from, to]（低饱和同色系浅色，从 dot 推导）。
 *  显示名在语言包 color.<key>，渲染时 t() 取 */
export const COLORS: { key: Exclude<NoteColor, ''>; dot: string; ink: string; exportBg: [string, string] }[] = [
  { key: 'yellow', dot: '#ffe97a', ink: '#f9a825', exportBg: ['#fffbe6', '#f9edc0'] },
  { key: 'pink', dot: '#f48fb1', ink: '#d81b60', exportBg: ['#fdf0f5', '#f6d5e2'] },
  { key: 'green', dot: '#a5d6a7', ink: '#43a047', exportBg: ['#eef7ee', '#d4ecd6'] },
  { key: 'blue', dot: '#90caf9', ink: '#1e88e5', exportBg: ['#ecf5fd', '#cfe6f9'] },
  { key: 'purple', dot: '#ce93d8', ink: '#8e24aa', exportBg: ['#f8f0fa', '#ead5f0'] },
  { key: 'orange', dot: '#ffcc80', ink: '#fb8c00', exportBg: ['#fff6e9', '#fce3bd'] },
];

/** localStorage key：上次使用的便签颜色（NoteView 换色时写入） */
export const LAST_NOTE_COLOR_KEY = 'pinslip-last-note-color';

/** 读上次用色：六色白名单校验，非法/缺省回退 yellow */
export function readLastNoteColor(): Exclude<NoteColor, ''> {
  return (
    COLORS.find(({ key }) => key === localStorage.getItem(LAST_NOTE_COLOR_KEY))?.key ?? 'yellow'
  );
}

/** 导出图背景渐变色对：空串（默认黄）回退 yellow */
export function exportBgOf(color: NoteColor): [string, string] {
  return (COLORS.find((c) => c.key === color) ?? COLORS[0]).exportBg;
}
