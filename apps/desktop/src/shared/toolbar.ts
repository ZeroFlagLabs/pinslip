// 便签底部工具栏左区（编辑辅助区）按钮的可定制顺序。
// main/渲染共用：main 侧校验设置、渲染层按序构建工具栏与排序列表。

/** 全部可排序按钮的默认顺序（= 定制功能上线前的现状顺序）；
 *  新按钮上线时在末尾登记，旧配置经 sanitize 自动衔接 */
export const TOOLBAR_BUTTON_DEFAULT_ORDER = [
  'bold',
  'strike',
  'task',
  'image',
  'zoomIn',
  'zoomOut',
  'table',
  'export',
] as const;

export type ToolbarButtonId = (typeof TOOLBAR_BUTTON_DEFAULT_ORDER)[number];

/** 便签工具栏左区展示上限：超出位次的按钮在 ⋯ 菜单找回 */
export const TOOLBAR_DISPLAY_LIMIT = 8;

/** 清洗用户配置的顺序列表：过滤未知 id、去重、缺项按默认顺序补到末尾
 *  （永不丢按钮：旧配置遇到新版本新增的按钮自动衔接）。
 *  入参任意（settings.json 是用户可改的 JSON），返回完整有序 id 列表 */
export function sanitizeToolbarButtons(input: unknown): string[] {
  const known = new Set<string>(TOOLBAR_BUTTON_DEFAULT_ORDER);
  const result: string[] = [];
  if (Array.isArray(input)) {
    for (const id of input) {
      if (typeof id === 'string' && known.has(id) && !result.includes(id)) {
        result.push(id);
      }
    }
  }
  for (const id of TOOLBAR_BUTTON_DEFAULT_ORDER) {
    if (!result.includes(id)) result.push(id);
  }
  return result;
}
