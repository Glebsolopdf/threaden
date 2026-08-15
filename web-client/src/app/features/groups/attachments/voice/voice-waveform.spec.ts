import { describe, expect, it } from 'vitest';
import { getVoiceWaveform } from './voice-waveform';

describe('getVoiceWaveform', () => {
  it('keeps idle waves calm and returns the requested number of bars', () => {
    expect(getVoiceWaveform(0, 6)).toEqual([0.18, 0.18, 0.18, 0.18, 0.18, 0.18]);
  });

  it('raises the center bars when the microphone level increases', () => {
    const bars = getVoiceWaveform(1, 7);
    expect(bars).toHaveLength(7);
    expect(bars[3]).toBeGreaterThan(bars[0]);
    expect(Math.max(...bars)).toBeLessThanOrEqual(1);
  });
});
