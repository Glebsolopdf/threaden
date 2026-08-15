import { signal } from '@angular/core';

export const MAX_RECORDING_MS = 5 * 60 * 1000;

export type VoiceRecorderState = 'idle' | 'recording' | 'ready' | 'error';

const mimeTypes = ['audio/webm;codecs=opus', 'audio/ogg;codecs=opus', 'audio/mp4', 'audio/wav'];

export class VoiceRecorder {
  readonly state = signal<VoiceRecorderState>('idle');
  readonly elapsedMs = signal(0);
  private recorder: MediaRecorder | null = null;
  private stream: MediaStream | null = null;
  private chunks: Blob[] = [];
  private pendingResult: Promise<Blob> | null = null;
  private resolveResult: ((value: Blob) => void) | null = null;
  private rejectResult: ((reason?: unknown) => void) | null = null;
  private timeout: ReturnType<typeof setTimeout> | null = null;
  private ticker: ReturnType<typeof setInterval> | null = null;
  private startedAt = 0;
  private cancelled = false;

  async start(): Promise<void> {
    if (this.state() === 'recording') return;
    if (!navigator.mediaDevices?.getUserMedia || typeof MediaRecorder === 'undefined') {
      return this.fail(new Error('Запись голосовых сообщений не поддерживается этим браузером'));
    }
    try {
      this.stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const mimeType = mimeTypes.find((type) => MediaRecorder.isTypeSupported(type));
      this.recorder = mimeType ? new MediaRecorder(this.stream, { mimeType }) : new MediaRecorder(this.stream);
      this.chunks = [];
      this.cancelled = false;
      this.startedAt = Date.now();
      this.elapsedMs.set(0);
      this.pendingResult = new Promise<Blob>((resolve, reject) => {
        this.resolveResult = resolve;
        this.rejectResult = reject;
      });
      this.recorder.ondataavailable = (event) => { if (event.data.size > 0) this.chunks.push(event.data); };
      this.recorder.onerror = () => this.fail(new Error('Не удалось записать голосовое сообщение'));
      this.recorder.onstop = () => this.complete();
      this.recorder.start();
      this.state.set('recording');
      this.ticker = setInterval(() => this.elapsedMs.set(Math.min(Date.now() - this.startedAt, MAX_RECORDING_MS)), 250);
      this.timeout = setTimeout(() => this.stop(), MAX_RECORDING_MS);
    } catch (error) {
      this.releaseStream();
      return this.fail(error);
    }
  }

  stop(): Promise<Blob> {
    if (this.state() !== 'recording' || !this.recorder || !this.pendingResult) {
      return Promise.reject(new Error('Запись не запущена'));
    }
    const result = this.pendingResult;
    this.recorder.stop();
    return result;
  }

  result(): Promise<Blob> {
    return this.pendingResult ?? Promise.reject(new Error('Запись не запущена'));
  }

  cancel(): void {
    if (this.state() !== 'recording') return;
    const reject = this.rejectResult;
    this.cancelled = true;
    this.clearTimeout();
    this.recorder?.stop();
    this.reset('idle');
    reject?.(new Error('Запись отменена'));
  }

  private complete(): void {
    if (this.cancelled) {
      this.releaseStream();
      this.recorder = null;
      this.chunks = [];
      this.clearPromise();
      return;
    }
    const result = new Blob(this.chunks, { type: this.recorder?.mimeType || 'audio/webm' });
    this.clearTimeout();
    this.releaseStream();
    this.state.set('ready');
    this.resolveResult?.(result);
    this.clearPromise();
  }

  private fail(error: unknown): Promise<never> {
    this.clearTimeout();
    this.releaseStream();
    this.state.set('error');
    this.rejectResult?.(error);
    this.clearPromise();
    return Promise.reject(error);
  }

  private reset(state: VoiceRecorderState): void {
    this.releaseStream();
    this.recorder = null;
    this.chunks = [];
    this.clearPromise();
    this.state.set(state);
    this.elapsedMs.set(0);
  }

  private releaseStream(): void {
    this.stream?.getTracks().forEach((track) => track.stop());
    this.stream = null;
  }

  private clearTimeout(): void {
    if (this.timeout) clearTimeout(this.timeout);
    this.timeout = null;
    if (this.ticker) clearInterval(this.ticker);
    this.ticker = null;
  }

  private clearPromise(): void {
    this.pendingResult = null;
    this.resolveResult = null;
    this.rejectResult = null;
  }
}
