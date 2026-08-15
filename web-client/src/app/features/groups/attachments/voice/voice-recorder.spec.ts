import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
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

class FakeAudioContext {
  static instance: FakeAudioContext;
  readonly analyser = { fftSize: 0, getByteTimeDomainData: (data: Uint8Array) => data.fill(255), disconnect: vi.fn() };
  readonly resume = vi.fn(async () => undefined);
  readonly close = vi.fn(async () => undefined);
  readonly state = 'suspended';
  constructor() { FakeAudioContext.instance = this; }
  createAnalyser(): AnalyserNode { return this.analyser as unknown as AnalyserNode; }
  createMediaStreamSource(): { connect: () => void } { return { connect: vi.fn() }; }
}

describe('VoiceRecorder', () => {
  beforeEach(() => {
    vi.stubGlobal('MediaRecorder', FakeMediaRecorder);
    vi.stubGlobal('navigator', { mediaDevices: { getUserMedia: vi.fn(async () => ({ getTracks: () => [{ stop: vi.fn() }] })) } });
  });

  afterEach(() => vi.unstubAllGlobals());

  it('updates the audio level from analyser samples', async () => {
    let frame: FrameRequestCallback | undefined;
    vi.stubGlobal('AudioContext', FakeAudioContext);
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => { frame = callback; return 1; });
    vi.stubGlobal('cancelAnimationFrame', vi.fn());
    const recorder = new VoiceRecorder();

    await recorder.start();
    const result = recorder.result();
    frame?.(0);

    expect(FakeAudioContext.instance.resume).toHaveBeenCalled();
    expect(recorder.audioLevel()).toBeGreaterThan(0);
    recorder.cancel();
    await expect(result).rejects.toThrow('отменена');
  });

  it('records an audio blob after stop', async () => {
    const recorder = new VoiceRecorder();
    await recorder.start();
    expect(recorder.audioLevel()).toBeGreaterThanOrEqual(0);
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
    expect(recorder.audioLevel()).toBe(0);
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
