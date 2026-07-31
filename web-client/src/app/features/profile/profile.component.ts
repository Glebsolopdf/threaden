import { ChangeDetectionStrategy, Component, DestroyRef, inject, signal } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
import { firstValueFrom, lastValueFrom } from 'rxjs';
import { ApiService } from '../../core/api/api.service';
import { AuthStore } from '../../core/auth/auth.store';
import { NotificationStore } from '../../core/notifications/notification.store';
import { VoiceService } from '../../core/voice/voice.service';
import { AvatarComponent } from '../../shared/avatar/avatar.component';

@Component({
  selector: 'app-profile',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ReactiveFormsModule, RouterLink, AvatarComponent],
  template: `
    <section class="route-page profile-route">
      <header class="group-header"><a class="group-header__icon" routerLink="/" aria-label="Назад"><img src="/back.svg" alt=""></a><strong>Профиль</strong></header>
      <div class="join-card panel auth-card profile-edit-card">
        <div class="profile-edit-preview">
          <app-avatar class="profile-avatar profile-avatar--large" [src]="previewUrl() || auth.user()?.avatar || ''" [label]="form.controls.displayName.value || auth.user()?.display_name || 'Профиль'" />
          <label class="avatar-picker-button" for="avatar-file" aria-label="Выбрать аватар"><img src="/avatar-select.svg" alt=""></label>
          <strong>{{ auth.user()?.email }}</strong>
        </div>
        <form class="auth-form" [formGroup]="form" (ngSubmit)="save()">
          <label for="display-name">Имя</label><input id="display-name" class="profile-input" type="text" formControlName="displayName" maxlength="50" autocomplete="name">
          <input id="avatar-file" class="visually-hidden-file" type="file" accept="image/png,image/jpeg,image/gif,image/webp" (change)="pickAvatar($event)">
          <div class="profile-actions">
            <button class="button button--secondary" type="button" [disabled]="pending() || !auth.user()?.avatar" (click)="deleteAvatar()">Удалить аватар</button>
            <button class="button button--secondary" type="button" [disabled]="pending()" (click)="logout()">Выйти</button>
            <button class="button button--primary" type="submit" [disabled]="form.invalid || !profileChanged() || pending()">{{ pending() ? 'Сохраняем…' : 'Сохранить' }}</button>
          </div>
        </form>
        <div class="profile-danger-zone"><span>Опасная зона</span><button class="profile-danger-zone__delete" type="button" aria-label="Удалить аккаунт" (click)="deleteConfirmOpen.set(true)"><img src="/trash.svg" alt=""></button></div>
        @if (progress() > 0 && progress() < 100) { <div class="upload-progress"><span [style.width.%]="progress()"></span></div> }
      </div>
    </section>

    @if (deleteConfirmOpen()) {
      <div class="dialog-backdrop" animate.leave="dialog-leave"><section class="dialog-card confirm-card" role="alertdialog" aria-modal="true"><h2>Удалить аккаунт?</h2><p>Ваш профиль будет удалён навсегда. Отменить это действие невозможно.</p><menu><button type="button" (click)="deleteConfirmOpen.set(false)">Отмена</button><button class="button button--danger" type="button" [disabled]="pending()" (click)="deleteProfile()">Удалить</button></menu></section></div>
    }
  `,
})
export class ProfileComponent {
  protected readonly auth = inject(AuthStore);
  private readonly api = inject(ApiService);
  private readonly voice = inject(VoiceService);
  private readonly notifications = inject(NotificationStore);
  private readonly router = inject(Router);
  protected readonly pending = signal(false);
  protected readonly progress = signal(0);
  protected readonly previewUrl = signal('');
  protected readonly deleteConfirmOpen = signal(false);
  private selectedAvatar?: File;

  protected readonly form = new FormGroup({
    displayName: new FormControl(this.auth.user()?.display_name ?? '', { nonNullable: true, validators: [Validators.required, Validators.maxLength(50)] }),
  });

  constructor() {
    inject(DestroyRef).onDestroy(() => this.revokePreview());
  }

  protected pickAvatar(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    if (!['image/png', 'image/jpeg', 'image/gif', 'image/webp'].includes(file.type)) {
      input.value = '';
      this.notifications.error('Аватар должен быть jpeg, png, gif или webp');
      return;
    }
    this.selectedAvatar = file;
    this.revokePreview();
    this.previewUrl.set(URL.createObjectURL(file));
    this.notifications.neutral('Предпросмотр обновлён. Нажмите «Сохранить», чтобы применить.');
  }

  protected profileChanged(): boolean {
    return this.form.controls.displayName.value.trim() !== this.auth.user()?.display_name || Boolean(this.selectedAvatar);
  }

  protected async save(): Promise<void> {
    if (this.form.invalid || !this.profileChanged()) return;
    const name = this.form.controls.displayName.value.trim();
    if ([...name].length < 1 || [...name].length > 50) return this.notifications.error('Введите имя длиной от 1 до 50 символов');
    this.pending.set(true);
    this.progress.set(this.selectedAvatar ? 1 : 0);
    try {
      const result = await lastValueFrom(this.api.updateProfile(name, this.selectedAvatar));
      if (!result.user) throw new Error('Сервер не вернул профиль');
      this.auth.setUser(result.user);
      this.selectedAvatar = undefined;
      this.revokePreview();
      this.progress.set(100);
      this.notifications.success('Профиль сохранён');
    } catch (error) { this.notifications.error(error, 'Не удалось сохранить профиль'); }
    finally { this.pending.set(false); this.progress.set(0); }
  }

  protected async deleteAvatar(): Promise<void> {
    this.pending.set(true);
    try {
      const user = await firstValueFrom(this.api.deleteAvatar());
      this.auth.setUser(user);
      this.selectedAvatar = undefined;
      this.revokePreview();
      this.notifications.success('Аватар удалён');
    } catch (error) { this.notifications.error(error, 'Не удалось удалить аватар'); }
    finally { this.pending.set(false); }
  }

  protected async logout(): Promise<void> {
    this.pending.set(true);
    try { await this.voice.shutdown(); await this.auth.logout(); await this.router.navigate(['/login']); }
    finally { this.pending.set(false); }
  }

  protected async deleteProfile(): Promise<void> {
    this.pending.set(true);
    try {
      await firstValueFrom(this.api.deleteProfile());
      this.auth.clear();
      await this.voice.shutdown();
      await this.router.navigate(['/register']);
    } catch (error) { this.notifications.error(error, 'Не удалось удалить аккаунт'); }
    finally { this.pending.set(false); }
  }

  private revokePreview(): void {
    const value = this.previewUrl();
    if (value) URL.revokeObjectURL(value);
    this.previewUrl.set('');
  }
}
