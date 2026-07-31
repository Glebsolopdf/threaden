import { Injectable, signal } from '@angular/core';

export interface AudioPreferences {
  inputDeviceId: string;
  outputDeviceId: string;
  microphoneEnabled: boolean;
  outputVolume: number;
}

export interface WebPreferences {
  debugErrors: boolean;
}

const AUDIO_KEY = 'voice_rooms_audio_preferences';
const WEB_KEY = 'voice_rooms_web_preferences';
const THEME_KEY = 'voice_rooms_theme';

@Injectable({ providedIn: 'root' })
export class PreferencesService {
  readonly audio = signal<AudioPreferences>(this.read(AUDIO_KEY, {
    inputDeviceId: '', outputDeviceId: '', microphoneEnabled: false, outputVolume: 100,
  }));
  readonly web = signal<WebPreferences>(this.read(WEB_KEY, { debugErrors: false }));
  readonly theme = signal<'default'>(this.read(THEME_KEY, 'default'));

  constructor() {
    document.documentElement.dataset['theme'] = this.theme();
  }

  updateAudio(patch: Partial<AudioPreferences>): void {
    const value = { ...this.audio(), ...patch };
    this.audio.set(value);
    this.write(AUDIO_KEY, value);
  }

  updateWeb(patch: Partial<WebPreferences>): void {
    const value = { ...this.web(), ...patch };
    this.web.set(value);
    this.write(WEB_KEY, value);
  }

  private read<T>(key: string, fallback: T): T {
    try {
      const raw = localStorage.getItem(key);
      if (!raw) return fallback;

      const parsed: unknown = JSON.parse(raw);
      const fallbackIsRecord = typeof fallback === 'object' && fallback !== null && !Array.isArray(fallback);
      const parsedIsRecord = typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed);

      return fallbackIsRecord && parsedIsRecord
        ? { ...fallback, ...parsed } as T
        : parsed as T;
    } catch {
      return fallback;
    }
  }

  private write(key: string, value: unknown): void {
    try { localStorage.setItem(key, JSON.stringify(value)); } catch { /* storage is optional */ }
  }
}
