import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { exportBgOf } from '../utils/colors';
import type { ExportImagePayload } from '@shared/types';

/** 就绪上报的兜底超时：图片 decode / 字体加载挂起也不整单失败 */
const READY_TIMEOUT_MS = 3000;

/** 导出视图：隐藏离屏窗（#/export）内渲染「渐变+噪点背景 / 便签卡片 / 水印」构图。
 *  数据流：挂载即订阅 export:payload → 主进程 did-finish-load 后下发 → 渲染 →
 *  等全部图片 decode + 字体就绪（3s 超时兜底）→ 上报内容高度，主进程据此定稿
 *  窗口尺寸后 capturePage。
 *  html 是本应用编辑器显示态 DOM 的 innerHTML（本地用户内容，非外部输入），
 *  补齐 .pinslip-editor > .milkdown > .ProseMirror 三层外壳后既有编辑器样式全部命中 */
export default function ExportView() {
  const { t } = useTranslation();
  const [payload, setPayload] = useState<ExportImagePayload | null>(null);
  const stageRef = useRef<HTMLDivElement>(null);
  /** StrictMode 下 effect 双跑，上报只发一次 */
  const reportedRef = useRef(false);

  useEffect(() => window.api.onExportPayload(setPayload), []);

  useEffect(() => {
    if (!payload || reportedRef.current) return;
    reportedRef.current = true;
    let settled = false;
    const report = () => {
      if (settled) return;
      settled = true;
      const h = Math.ceil(stageRef.current?.scrollHeight ?? 0);
      window.api.exportReady(h);
    };
    const timer = setTimeout(report, READY_TIMEOUT_MS);
    // 图片逐张 decode（失败不阻塞：坏图按占位渲染）+ 字体就绪后上报精确高度
    void Promise.all(Array.from(document.images).map((img) => img.decode().catch(() => {})))
      .then(() => document.fonts.ready)
      .then(() => {
        clearTimeout(timer);
        report();
      });
    return () => clearTimeout(timer);
  }, [payload]);

  if (!payload) return <div ref={stageRef} className="export-stage" />;

  const [bgFrom, bgTo] = exportBgOf(payload.color);
  return (
    <div
      ref={stageRef}
      className="export-stage"
      style={{ background: `linear-gradient(160deg, ${bgFrom} 0%, ${bgTo} 100%)` }}
    >
      <div className="export-card" data-color={payload.color || undefined}>
        <div className="export-card__titlebar">
          <span className="export-card__title">{payload.title || t('note.newTitle')}</span>
        </div>
        <div className="export-card__body pinslip-editor">
          <div className="milkdown">
            <div className="ProseMirror" dangerouslySetInnerHTML={{ __html: payload.html }} />
          </div>
        </div>
        {payload.tags.length > 0 && (
          <div className="export-card__tags">
            {payload.tags.map((tag) => (
              <span key={tag} className="tag-chip">
                {tag}
              </span>
            ))}
          </div>
        )}
      </div>
      <div className="export-watermark">PinSlip</div>
    </div>
  );
}
