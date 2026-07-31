import { describe, expect, it } from 'vitest';
import { VoiceActivityDetector } from './activity-detector';

describe('VoiceActivityDetector', () => {
  it('holds a visible speaking state briefly after a loud sample', () => {
    const detector = new VoiceActivityDetector();

    expect(detector.update('user-1', 0.2, false, false, 0)).toBe(true);
    expect(detector.update('user-1', 0, false, false, 200)).toBe(true);
    expect(detector.update('user-1', 0, false, false, 700)).toBe(false);
  });

  it('never reports a muted participant as speaking', () => {
    const detector = new VoiceActivityDetector();
    expect(detector.update('user-1', 1, true, true, 0)).toBe(false);
  });

  it('forgets activity after reset', () => {
    const detector = new VoiceActivityDetector();
    expect(detector.update('user-1', 0.4, true, false, 0)).toBe(true);
    detector.reset();
    expect(detector.update('user-1', 0, false, false, 10)).toBe(false);
  });
});
