/** 文档/切片 → 纯文本序列化（「复制全部」getPlainText 与 Ctrl+C clipboardTextSerializer 共用）。
 *  不用 doc.textBetween：它只取文字内容，而有序列表序号是渲染产物（DOM 计数器），
 *  不在文档文本里，会被整体丢掉（2026-09-30 复制全部丢序号 bug 根因，引入于 c5087de）。
 *  这里按块结构重建：列表项补回标记（有序递增序号 / 无序 - / 任务 [x]/[ ]），
 *  嵌套列表每级缩进两空格，块间单换行（与既有约定一致），图片/分割线等叶节点跳过 */

/** 结构最小类型：ProseMirror Node 与 Fragment 都满足
 * （pnpm 严格隔离不能值导入 prosemirror-*，结构化声明即可） */
export interface FragmentLike {
  forEach(cb: (node: NodeLike, offset: number, index: number) => void): void;
}

interface NodeLike {
  type: { name: string };
  attrs: Record<string, unknown>;
  isText: boolean;
  isLeaf: boolean;
  isTextblock: boolean;
  text?: string;
  content: FragmentLike;
}

function isList(name: string): boolean {
  return name === 'bullet_list' || name === 'ordered_list';
}

/** 行内文本收集：text 直取，hard_break 转换行，容器递归；图片等叶节点跳过 */
function inlineText(node: NodeLike): string {
  if (node.isText) return node.text ?? '';
  if (node.type.name === 'hard_break') return '\n';
  if (node.isLeaf) return '';
  let out = '';
  node.content.forEach((child) => {
    out += inlineText(child);
  });
  return out;
}

export function fragmentToPlainText(fragment: FragmentLike): string {
  const lines: string[] = [];

  function walkBlocks(frag: FragmentLike, depth: number): void {
    frag.forEach((node) => {
      if (isList(node.type.name)) {
        walkList(node, depth);
        return;
      }
      if (node.isTextblock) {
        lines.push(inlineText(node));
        return;
      }
      if (!node.isLeaf) walkBlocks(node.content, depth); // blockquote 等容器
    });
  }

  function walkList(list: NodeLike, depth: number): void {
    const ordered = list.type.name === 'ordered_list';
    const start = typeof list.attrs.order === 'number' ? list.attrs.order : 1;
    list.content.forEach((item, _offset, index) => {
      const marker = ordered ? `${start + index}. ` : '- ';
      // 任务项（GFM：attrs.checked 非空即任务）补复选标记，与 markdown/Obsidian 同写法
      const checked = item.attrs.checked;
      const task = checked == null ? '' : checked ? '[x] ' : '[ ] ';
      const indent = '  '.repeat(depth);
      let markerLineUsed = false;
      item.content.forEach((child) => {
        if (isList(child.type.name)) {
          walkList(child, depth + 1);
          return;
        }
        if (child.isTextblock) {
          const text = inlineText(child);
          if (!markerLineUsed) {
            lines.push(`${indent}${marker}${task}${text}`);
            markerLineUsed = true;
          } else {
            // 同一 item 的后续段落：对齐到标记后的文本位
            lines.push(`${indent}${' '.repeat(marker.length + task.length)}${text}`);
          }
          return;
        }
        if (!child.isLeaf) walkBlocks(child.content, depth + 1);
      });
      if (!markerLineUsed) lines.push(`${indent}${marker}${task}`.trimEnd()); // 空 item
    });
  }

  walkBlocks(fragment, 0);
  return lines.join('\n');
}
