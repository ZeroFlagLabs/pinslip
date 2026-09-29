/**
 * noteContentToPlainText 把 markdown 正文转纯文本（复制全场景）：
 * 过滤 Milkdown 写入的空行标记 <br />——独立成行 → 空行；行内 → 换行，
 * 避免粘贴到别处时出现字面 br 标签。
 * 与 NoteView copyAll 的回退路径、主界面列表项「复制全部」同一口径。
 */
export function noteContentToPlainText(content: string): string {
  return content.replace(/^[ \t]*<br\s*\/?>[ \t]*$/gim, '').replace(/<br\s*\/?>/gi, '\n');
}
