import { describe, expect, it } from 'vitest';
import { formatAudioTime } from './audio-attachment-player.component';

describe('formatAudioTime', () => {
  it('formats a voice message duration for the track label', () => {
    expect(formatAudioTime(0)).toBe('0:00');
    expect(formatAudioTime(74.9)).toBe('1:14');
  });
});
