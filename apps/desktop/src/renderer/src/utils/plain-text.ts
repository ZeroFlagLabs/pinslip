/**
 * noteContentToPlainText 把 markdown 正文转纯文本（复制全场景）：
 * 与便签编辑器序列化（doc-plain-text）同一「块间单换行」约定——
 * markdown 源文用空行分隔块（\n\n），直接拷会多出空行，这里折叠丢弃；
 * 用户有意的空行（Milkdown 写入的独立 <br /> 行）保留一个；
 * 行内 <br /> → 换行，避免粘贴到别处时出现字面 br 标签。
 * 与 NoteView copyAll 的回退路径、主界面列表项「复制全部」同一口径。
 */
export function noteContentToPlainText(content: string): string {
  const out: string[] = [];
  for (const raw of content.split('\n')) {
    if (/^[ \t]*<br\s*\/?>[ \t]*$/i.test(raw)) {
      out.push(''); // 用户有意的空行：保留
      continue;
    }
    if (!raw.trim()) continue; // markdown 块分隔空行：丢弃
    out.push(raw.replace(/<br\s*\/?>/gi, '\n'));
  }
  return out.join('\n');
}
