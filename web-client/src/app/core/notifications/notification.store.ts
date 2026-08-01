import { inject, Injectable, signal } from '@angular/core';
import { ApiError, toApiError } from '../api/api-error';
import { PreferencesService } from '../preferences/preferences.service';

export type NotificationKind = 'success' | 'error' | 'neutral';
export interface AppNotification { id: number; kind: NotificationKind; message: string; }

@Injectable({ providedIn: 'root' })
export class NotificationStore {
  private readonly preferences = inject(PreferencesService);
  private readonly sequence = signal(0);
  readonly current = signal<AppNotification | null>(null);
  private timer?: number;

  show(message: string, kind: NotificationKind = 'neutral'): void {
    const text = message.trim();
    if (!text) return this.clear();
    if (this.timer !== undefined) window.clearTimeout(this.timer);
    this.current.set({ id: this.sequence() + 1, kind, message: text });
    this.sequence.update((value) => value + 1);
    this.timer = window.setTimeout(() => this.clear(), kind === 'error' ? 10_000 : 5_000);
  }

  success(message: string): void { this.show(message, 'success'); }
  neutral(message: string): void { this.show(message, 'neutral'); }
  error(value: unknown, fallback = 'Произошла ошибка. Попробуйте ещё раз'): void {
    if (typeof value === 'string') return this.show(value, 'error');
    const error = toApiError(value);
    if (error.code === 'voice_room_limit') return this.neutral('Достигнут лимит голосовых комнат в этой группе');
    if (error.code === 'spam_warning') return this.neutral('Не надо, пожалуйста');
    this.show(this.displayError(error, fallback), 'error');
  }

  clear(): void {
    if (this.timer !== undefined) window.clearTimeout(this.timer);
    this.timer = undefined;
    this.current.set(null);
  }

  private displayError(error: ApiError, fallback: string): string {
    if (this.preferences.web().debugErrors) {
      return `${error.message} [${error.status}/${error.code}]${error.requestId ? ` request_id: ${error.requestId}` : ''}`;
    }
    return error.code === 'network_error' || error.code === 'events_unavailable' ? 'Соединение потеряно' : fallback;
  }
}
