import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { ApiService } from '../../core/api/api.service';
import { AuthStore } from '../../core/auth/auth.store';
import type { WelcomeStats } from '../../core/api/models';
import { readWelcomeCache, writeWelcomeCache } from './welcome-cache';

@Component({
  selector: 'app-welcome',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="welcome" aria-live="polite">
      <div class="welcome__brand" aria-label="Threaden">
        <img src="/threaden-logo.svg" alt="">
        <span>threaden</span>
      </div>
      <h1 class="welcome__title">Привет, {{ displayName() }}!</h1>
      <p class="welcome__subtitle">За последние сутки в Threaden</p>
      <div class="welcome__rule" aria-hidden="true"></div>
      <section class="welcome__stats" data-welcome-stats aria-label="Статистика за последние сутки">
        <dl class="welcome__metrics">
          <div class="welcome__metric"><dt>Сообщений</dt><dd>{{ number(stats().messages) }}</dd></div>
          <div class="welcome__metric"><dt>Новых аккаунтов</dt><dd>{{ number(stats().new_users) }}</dd></div>
          <div class="welcome__metric"><dt>Новых групп</dt><dd>{{ number(stats().new_groups) }}</dd></div>
        </dl>
      </section>
    </div>
  `,
})
export class WelcomeComponent {
  private readonly api = inject(ApiService);
  protected readonly auth = inject(AuthStore);
  protected readonly stats = signal<WelcomeStats>({ messages: 0, new_users: 0, new_groups: 0 });
  protected readonly displayName = computed(() => this.auth.user()?.display_name || 'вам');

  constructor() {
    void this.load();
  }

  protected number(value: number): string {
    return new Intl.NumberFormat('ru-RU').format(value);
  }

  private async load(): Promise<void> {
    try {
      const cached = readWelcomeCache(sessionStorage);
      if (cached) {
        this.stats.set(cached);
        return;
      }
      const stats = await firstValueFrom(this.api.welcome());
      this.stats.set(stats);
      writeWelcomeCache(sessionStorage, stats);
    } catch {
      this.stats.set({ messages: 0, new_users: 0, new_groups: 0 });
    }
  }
}
