import { describe, expect, it } from 'vitest';
import type { AccountQuotas } from '../../core/api/models';
import { attachmentDeletionCompleted, formatQuotaBytes, formatRetention } from './quota-view';

const quota = (pending_delete?: AccountQuotas['pending_delete']): AccountQuotas => ({
  usage: { stored_bytes: 0, daily_bytes: 0 },
  limits: {
    max_input_media_bytes: 0, max_archive_bytes: 0, max_output_media_bytes: 0, max_files_per_message: 0,
    max_user_stored_bytes: 0, max_user_daily_bytes: 0, max_total_bytes: 0, min_free_bytes: 0, retention_seconds: 0,
  },
  pending_delete,
});

describe('quota view formatting', () => {
  it('formats byte limits for a compact account summary', () => {
    expect(formatQuotaBytes(50 * 1024 * 1024)).toBe('50 МБ');
  });

  it('formats retention duration in hours and days', () => {
    expect(formatRetention(72 * 60 * 60)).toBe('72 ч (3 дн.)');
  });

  it('detects when a scheduled attachment deletion has completed', () => {
    expect(attachmentDeletionCompleted(quota({ id: 'adr', created_at: '', execute_at: '' }), quota())).toBe(true);
    expect(attachmentDeletionCompleted(quota(), quota())).toBe(false);
  });
});
