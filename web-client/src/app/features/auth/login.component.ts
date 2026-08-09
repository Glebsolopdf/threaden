import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { AuthStore } from '../../core/auth/auth.store';
import { NotificationStore } from '../../core/notifications/notification.store';
import { AuthThemeToggleComponent } from './auth-theme-toggle.component';

@Component({
  selector: 'app-login',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ReactiveFormsModule, RouterLink, AuthThemeToggleComponent],
  template: `
    <main class="app-shell">
      <section class="screen home-page auth-page" aria-labelledby="login-title">
        <a class="brand page-logo" routerLink="/" aria-label="threaden, на главную"><img src="/threaden-logo.svg" width="24" height="24" alt=""><span>threaden</span></a>
        <app-auth-theme-toggle />
        <header class="home-hero page-title"><h1 id="login-title">Вход</h1><p>Вокруг нас крутятся токены</p></header>
        <div class="join-card auth-card">
          <form class="auth-form" [formGroup]="form" (ngSubmit)="submit()" [attr.aria-busy]="pending()">
            <label for="login-email">Email</label><input id="login-email" type="email" formControlName="email" autocomplete="email" placeholder="you@example.com">
            <label for="login-password">Пароль</label><input id="login-password" type="password" formControlName="password" autocomplete="current-password" placeholder="Введите пароль" maxlength="72">
            <button class="button button--primary" type="submit" [disabled]="form.invalid || pending()">{{ pending() ? 'Входим…' : 'Войти' }}</button>
          </form>
          <a class="text-link" routerLink="/register" [queryParams]="{ continue: continuePath }">Создать аккаунт</a>
        </div>
      </section>
    </main>
  `,
})
export class LoginComponent {
  private readonly auth = inject(AuthStore);
  private readonly notifications = inject(NotificationStore);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  protected readonly pending = signal(false);
  protected readonly continuePath = this.safeContinue(this.route.snapshot.queryParamMap.get('continue') ?? '');
  protected readonly form = new FormGroup({
    email: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.email] }),
    password: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.maxLength(72)] }),
  });

  protected async submit(): Promise<void> {
    if (this.form.invalid) return;
    this.pending.set(true);
    try {
      const value = this.form.getRawValue();
      await this.auth.login(value.email.trim().toLowerCase(), value.password);
      await this.router.navigateByUrl(this.continuePath || '/');
    } catch (error) { this.notifications.error(error, 'Не удалось войти'); }
    finally { this.pending.set(false); }
  }

  private safeContinue(value: string): string { return value.startsWith('/') && !value.startsWith('//') ? value : ''; }
}
