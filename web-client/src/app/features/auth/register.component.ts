import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { AbstractControl, FormControl, FormGroup, ReactiveFormsModule, ValidationErrors, Validators } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { AuthStore } from '../../core/auth/auth.store';
import { NotificationStore } from '../../core/notifications/notification.store';
import { AuthThemeToggleComponent } from './auth-theme-toggle.component';

function passwordsMatch(control: AbstractControl): ValidationErrors | null {
  return control.get('password')?.value === control.get('confirm')?.value ? null : { passwordsMismatch: true };
}

@Component({
  selector: 'app-register',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ReactiveFormsModule, RouterLink, AuthThemeToggleComponent],
  template: `
    <main class="app-shell">
      <section class="screen home-page auth-page" aria-labelledby="register-title">
        <a class="brand page-logo" routerLink="/" aria-label="threaden, на главную"><img src="/threaden-logo.svg" width="24" height="24" alt=""><span>threaden</span></a>
        <app-auth-theme-toggle />
        <header class="home-hero page-title"><h1 id="register-title">Регистрация</h1><p>Создайте аккаунт, чтобы начать общение.</p></header>
        <div class="join-card auth-card">
          <form class="auth-form" [formGroup]="form" (ngSubmit)="submit()" [attr.aria-busy]="pending()">
            <label for="register-email">Email</label><input id="register-email" type="email" formControlName="email" autocomplete="email" placeholder="you@example.com">
            <label for="register-password">Пароль</label><input id="register-password" type="password" formControlName="password" autocomplete="new-password" placeholder="От 6 до 72 символов" maxlength="72">
            <label for="register-confirm">Повтор пароля</label><input id="register-confirm" type="password" formControlName="confirm" autocomplete="new-password" placeholder="Повторите пароль" maxlength="72">
            @if (form.hasError('passwordsMismatch') && form.controls.confirm.touched) { <p class="validation-error">Пароли не совпадают</p> }
            <button class="button button--primary" type="submit" [disabled]="form.invalid || pending()">{{ pending() ? 'Создаём аккаунт…' : 'Зарегистрироваться' }}</button>
          </form>
          <a class="text-link" routerLink="/login" [queryParams]="{ continue: continuePath }">Уже есть аккаунт</a>
        </div>
      </section>
    </main>
  `,
})
export class RegisterComponent {
  private readonly auth = inject(AuthStore);
  private readonly notifications = inject(NotificationStore);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  protected readonly pending = signal(false);
  protected readonly continuePath = this.safeContinue(this.route.snapshot.queryParamMap.get('continue') ?? '');
  protected readonly form = new FormGroup({
    email: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.email] }),
    password: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.minLength(6), Validators.maxLength(72)] }),
    confirm: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
  }, { validators: [passwordsMatch] });

  protected async submit(): Promise<void> {
    if (this.form.invalid) return;
    this.pending.set(true);
    try {
      const value = this.form.getRawValue();
      const characters = [...value.password].length;
      if (characters < 6 || characters > 72 || new TextEncoder().encode(value.password).length > 72) throw new Error('Пароль должен содержать от 6 до 72 символов');
      await this.auth.register(value.email.trim().toLowerCase(), value.password);
      await this.router.navigateByUrl(this.continuePath || '/');
    } catch (error) { this.notifications.error(error, 'Не удалось создать аккаунт'); }
    finally { this.pending.set(false); }
  }

  private safeContinue(value: string): string { return value.startsWith('/') && !value.startsWith('//') ? value : ''; }
}
