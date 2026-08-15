import { ChangeDetectionStrategy, Component, DestroyRef, inject, signal } from '@angular/core';
import { Router } from '@angular/router';
import { AuthStore } from '../../core/auth/auth.store';
import { toApiError } from '../../core/api/api-error';

@Component({
  selector: 'app-blocked',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <main class="app-shell">
      <section class="screen home-page auth-page" aria-labelledby="blocked-title">
        <header class="home-hero page-title blocked-page__hero" aria-live="polite">
          @if (released()) {
            <div class="blocked-state blocked-state--released" animate.enter="blocked-copy-enter" animate.leave="blocked-copy-leave">
              <h1 id="blocked-title">Ограничения сняты</h1>
              <p>Возврат на главную страницу произойдет автоматически через <strong>{{ countdown() }}</strong> {{ countdownLabel() }}.</p>
            </div>
          } @else {
            <div class="blocked-state blocked-state--blocked" animate.enter="blocked-copy-enter" animate.leave="blocked-copy-leave">
              <h1 id="blocked-title">Вы временно заблокированы</h1>
              <p>С этого аккаунта или адреса было отправлено слишком много запросов. Доступ восстановится автоматически — возвращайтесь позже.</p>
              <span class="blocked-state__status">{{ checking() ? 'Проверяем статус ограничений…' : 'Следующая проверка будет выполнена через минуту.' }}</span>
            </div>
          }
        </header>
      </section>
    </main>
  `,
})
export class BlockedComponent {
  private readonly auth = inject(AuthStore);
  private readonly router = inject(Router);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly released = signal(false);
  protected readonly checking = signal(true);
  protected readonly countdown = signal(10);
  private pollTimer?: ReturnType<typeof setTimeout>;
  private redirectTimer?: ReturnType<typeof setInterval>;

  constructor() {
    void this.checkAccess();
    this.destroyRef.onDestroy(() => {
      this.clearPollTimer();
      this.clearRedirectTimer();
    });
  }

  protected countdownLabel(): string {
    const value = this.countdown();
    if (value % 10 === 1 && value % 100 !== 11) return 'секунду';
    if ([2, 3, 4].includes(value % 10) && ![12, 13, 14].includes(value % 100)) return 'секунды';
    return 'секунд';
  }

  private async checkAccess(): Promise<void> {
    this.checking.set(true);
    try {
      await this.auth.ensureUser(true);
      this.handleReleased();
      return;
    } catch (error) {
      if (toApiError(error).status !== 429) throw error;
    }
    this.checking.set(false);
    this.scheduleNextCheck();
  }

  private handleReleased(): void {
    if (this.released()) return;
    this.released.set(true);
    this.checking.set(false);
    this.clearPollTimer();
    this.startRedirectCountdown();
  }

  private scheduleNextCheck(): void {
    this.clearPollTimer();
    this.pollTimer = setTimeout(() => void this.checkAccess(), 60000);
  }

  private startRedirectCountdown(): void {
    this.clearRedirectTimer();
    this.countdown.set(10);
    this.redirectTimer = setInterval(() => {
      const next = this.countdown() - 1;
      this.countdown.set(next);
      if (next > 0) return;
      this.clearRedirectTimer();
      void this.router.navigateByUrl('/');
    }, 1000);
  }

  private clearPollTimer(): void {
    if (!this.pollTimer) return;
    clearTimeout(this.pollTimer);
    this.pollTimer = undefined;
  }

  private clearRedirectTimer(): void {
    if (!this.redirectTimer) return;
    clearInterval(this.redirectTimer);
    this.redirectTimer = undefined;
  }
}
