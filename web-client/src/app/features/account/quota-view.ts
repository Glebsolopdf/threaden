import type { AccountQuotas } from '../../core/api/models';

export function formatQuotaBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} Б`;
  const units = ['КБ', 'МБ', 'ГБ'];
  let value = bytes;
  let unit = units[0];
  for (const next of units) {
    value /= 1024;
    unit = next;
    if (value < 1024 || next === units.at(-1)) break;
  }
  return `${Number.isInteger(value) ? value : value.toFixed(1)} ${unit}`;
}

export function attachmentDeletionCompleted(previous: AccountQuotas | null, current: AccountQuotas): boolean {
  return Boolean(previous?.pending_delete && !current.pending_delete);
}

export function formatRetention(seconds: number): string {
  const hours = Math.round(seconds / 3600);
  const days = Math.floor(hours / 24);
  return days > 0 ? `${hours} ч (${days} дн.)` : `${hours} ч`;
}
