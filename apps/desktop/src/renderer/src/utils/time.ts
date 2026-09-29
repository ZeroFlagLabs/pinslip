import type { TFunction } from 'i18next';

/** 相对时间：同步状态的「上次同步」用。Go 零值时间（0001 年）= 从未同步 */
export function formatRelativeTime(t: TFunction, iso?: string): string {
  if (!iso) return t('time.never');
  const d = new Date(iso);
  if (Number.isNaN(d.getTime()) || d.getFullYear() <= 1) return t('time.never');
  const diff = Date.now() - d.getTime();
  if (diff < 45_000) return t('time.justNow');
  const min = Math.floor(diff / 60_000);
  if (min < 60) return t('time.minutesAgo', { count: min });
  const h = Math.floor(min / 60);
  if (h < 24) return t('time.hoursAgo', { count: h });
  return t('time.daysAgo', { count: Math.floor(h / 24) });
}
