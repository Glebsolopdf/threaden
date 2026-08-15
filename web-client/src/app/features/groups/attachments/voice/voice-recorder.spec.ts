import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MAX_RECORDING_MS, VoiceRecorder } from './voice-recorder';

class FakeMediaRecorder {
  static isTypeSupported(): boolean { return true; }
  readonly state = 'inactive';
  ondataavailable: ((event: { data: Blob }) => void) | null = null;
  onerror: (() => void) | null = null;
  onstop: (() => void) | null = null;
  start = vi.fn(() => { (this as { state: string }).state = 'recording'; });
  stop = vi.fn(() => {
    (this as { state: string }).state = 'inactive';
    this.ondataavailable?.({ data: new Blob(['voice'], { type: 'audio/webm' }) });
    this.onstop?.();
  });
}

describe('VoiceRecorder', () => {
  beforeEach(() => {
    vi.stubGlobal('MediaRecorder', FakeMediaRecorder);
    vi.stubGlobal('navigator', { mediaDevices: { getUserMedia: vi.fn(async () => ({ getTracks: () => [{ stop: vi.fn() }] })) } });
  });

  it('records an audio blob after stop', async () => {
    const recorder = new VoiceRecorder();
    await recorder.start();
    const result = await recorder.stop();
    expect(result.type).toBe('audio/webm');
    expect(recorder.state()).toBe('ready');
  });

  it('reports microphone errors without leaving recording state', async () => {
    vi.mocked(navigator.mediaDevices.getUserMedia).mockRejectedValueOnce(new Error('denied'));
    const recorder = new VoiceRecorder();
    await expect(recorder.start()).rejects.toThrow('denied');
    expect(recorder.state()).toBe('error');
  });

  it('cancels recording and rejects its pending result', async () => {
    const recorder = new VoiceRecorder();
    await recorder.start();
    const result = recorder.result();
    recorder.cancel();
    await expect(result).rejects.toThrow('отменена');
    expect(recorder.state()).toBe('idle');
  });

  it('automatically stops at the five minute limit', async () => {
    vi.useFakeTimers();
    const recorder = new VoiceRecorder();
    await recorder.start();
    const result = recorder.result();
    vi.advanceTimersByTime(MAX_RECORDING_MS);
    await expect(result).resolves.toBeInstanceOf(Blob);
    expect(recorder.state()).toBe('ready');
    vi.useRealTimers();
  });
});
