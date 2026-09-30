import { useEffect, useRef, useState } from 'react';
import type { RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import CaretUpIcon from '~icons/ph/caret-up';
import CaretDownIcon from '~icons/ph/caret-down';
import CaretRightIcon from '~icons/ph/caret-right';
import XIcon from '~icons/ph/x';
import type { EditorHandle, FindStatus } from './Editor';

interface FindBarProps {
  /** 编辑器句柄（NoteView 的 editorRef 透传） */
  editorRef: RefObject<EditorHandle>;
  /** 替换行是否展开（受控：Ctrl+H 由宿主置 true） */
  replaceOpen: boolean;
  /** 顶部偏移（px）：标题栏 34；有外部修改横幅时再 +26 避让 */
  topOffset: number;
  onToggleReplace(): void;
  onOpenReplace(): void;
  onClose(): void;
}

/** 便签内搜索条：编辑器区域顶部浮层（不进标题栏）。
 *  打开即聚焦查询框并带入编辑器单行选区文字；输入实时高亮命中并选中下一处；
 *  Enter/Shift+Enter 导航，Esc 关闭（焦点还回编辑器）。替换行默认收起。
 *  打开/关闭状态由宿主（NoteView）管理，本组件只在打开期间挂载。 */
export default function FindBar({
  editorRef,
  replaceOpen,
  topOffset,
  onToggleReplace,
  onOpenReplace,
  onClose,
}: FindBarProps) {
  const { t } = useTranslation();
  const [query, setQuery] = useState('');
  const [replace, setReplace] = useState('');
  const [status, setStatus] = useState<FindStatus>({ total: 0, active: 0 });
  const queryRef = useRef<HTMLInputElement>(null);
  const replaceRef = useRef<HTMLInputElement>(null);

  // 唤出：带入编辑器当前选区文字（单行才带）并聚焦查询框
  // （仅挂载时执行一次：组件随搜索条开关挂载/卸载）
  useEffect(() => {
    const selected = editorRef.current?.getSelectedText() ?? '';
    if (selected) setQuery(selected);
    queryRef.current?.focus();
    queryRef.current?.select();
  }, []);

  // 查询/替换词同步进编辑器文档模型：query 变更即重高亮并选中下一处命中；
  // replace 只随 query 存储（替换命令取用），不挪选区
  useEffect(() => {
    const st = editorRef.current?.setFindQuery(query, replace);
    if (st) setStatus(st);
    else if (!query) setStatus({ total: 0, active: 0 });
  }, [query, replace]);

  // 搜索条打开期间的窗口级按键：Ctrl+F 再次按下 = 聚焦查询框并全选（编辑器惯例）；
  // Ctrl+H = 展开替换行并聚焦替换框；Esc 任意焦点下关闭
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && !e.altKey && !e.shiftKey) {
        const k = e.key.toLowerCase();
        if (k === 'f') {
          e.preventDefault();
          queryRef.current?.focus();
          queryRef.current?.select();
        } else if (k === 'h') {
          e.preventDefault();
          onOpenReplace();
          // 替换行本轮渲染才挂载，等下一拍聚焦
          requestAnimationFrame(() => replaceRef.current?.focus());
        }
      } else if (e.key === 'Escape') {
        onClose();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [onOpenReplace, onClose]);

  const goNext = () => {
    const st = editorRef.current?.findNext();
    if (st) setStatus(st);
  };
  const goPrev = () => {
    const st = editorRef.current?.findPrev();
    if (st) setStatus(st);
  };
  const doReplace = () => {
    const st = editorRef.current?.replaceNext();
    if (st) setStatus(st);
  };
  const doReplaceAll = () => {
    const st = editorRef.current?.replaceAll();
    if (st) setStatus(st);
  };

  const noMatch = query !== '' && status.total === 0;
  const navDisabled = status.total === 0;

  return (
    <div className="sticky-note__findbar" style={{ top: topOffset }}>
      <div className="sticky-note__findbar-row">
        <button
          className="sticky-note__btn"
          data-tip={replaceOpen ? t('note.find.collapseReplace') : t('note.find.expandReplace')}
          data-tip-align="left"
          aria-label={replaceOpen ? t('note.find.collapseReplace') : t('note.find.expandReplace')}
          onMouseDown={(e) => e.preventDefault()}
          onClick={onToggleReplace}
        >
          {replaceOpen ? <CaretDownIcon /> : <CaretRightIcon />}
        </button>
        <input
          ref={queryRef}
          className="sticky-note__findbar-input"
          value={query}
          placeholder={t('note.find.placeholder')}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault();
              if (e.shiftKey) goPrev();
              else goNext();
            }
          }}
        />
        <div className="sticky-note__findbar-actions">
          {query !== '' && (
            <span className={`sticky-note__findbar-count${noMatch ? ' is-empty' : ''}`}>
              {noMatch ? t('note.find.noResult') : `${status.active}/${status.total}`}
            </span>
          )}
          <button
            className="sticky-note__btn"
            data-tip={t('note.find.prev')}
            data-tip-align="left"
            aria-label={t('note.find.prev')}
            disabled={navDisabled}
            onMouseDown={(e) => e.preventDefault()}
            onClick={goPrev}
          >
            <CaretUpIcon />
          </button>
          <button
            className="sticky-note__btn"
            data-tip={t('note.find.next')}
            data-tip-align="left"
            aria-label={t('note.find.next')}
            disabled={navDisabled}
            onMouseDown={(e) => e.preventDefault()}
            onClick={goNext}
          >
            <CaretDownIcon />
          </button>
          <button
            className="sticky-note__btn"
            data-tip={t('note.find.close')}
            data-tip-align="right"
            aria-label={t('note.find.close')}
            onClick={onClose}
          >
            <XIcon />
          </button>
        </div>
      </div>
      {replaceOpen && (
        <div className="sticky-note__findbar-row">
          {/* 与首行替换箭头同列的占位，两行输入框同列同宽 */}
          <span className="sticky-note__findbar-spacer" />
          <input
            ref={replaceRef}
            className="sticky-note__findbar-input"
            value={replace}
            placeholder={t('note.find.replacePlaceholder')}
            onChange={(e) => setReplace(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                doReplace();
              }
            }}
          />
          <div className="sticky-note__findbar-actions">
            <button
              className="sticky-note__findbar-textbtn"
              disabled={navDisabled}
              onMouseDown={(e) => e.preventDefault()}
              onClick={doReplace}
            >
              {t('note.find.replace')}
            </button>
            <button
              className="sticky-note__findbar-textbtn"
              disabled={navDisabled}
              onMouseDown={(e) => e.preventDefault()}
              onClick={doReplaceAll}
            >
              {t('note.find.replaceAll')}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
