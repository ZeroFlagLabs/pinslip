import { postRaw } from './client';

/** 粘贴板 MIME → 扩展名（与服务端白名单一致；不收 SVG——可含脚本/外链的 XML,
 *  位图之外的渲染风险与本次拖拽需求不匹配） */
const EXT_BY_MIME: Record<string, string> = {
  'image/png': '.png',
  'image/jpeg': '.jpg',
  'image/gif': '.gif',
  'image/webp': '.webp',
};

export const SUPPORTED_IMAGE_MIME_TYPES = new Set(Object.keys(EXT_BY_MIME));

/** 附件上传 API（vault attachments/ 目录） */
export const attachmentsApi = {
  /** 上传图片，返回 vault 相对路径；MIME 不在白名单时返回 null */
  upload: (file: File): Promise<{ path: string } | null> => {
    const ext = EXT_BY_MIME[file.type];
    if (!ext) return Promise.resolve(null);
    return postRaw<{ path: string }>(`/api/attachments?ext=${encodeURIComponent(ext)}`, file);
  },
};
