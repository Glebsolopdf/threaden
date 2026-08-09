import { ChangeDetectionStrategy, Component, inject, output } from '@angular/core';
import { FormControl, ReactiveFormsModule, Validators } from '@angular/forms';
import { NotificationStore } from '../../core/notifications/notification.store';
import { parseInviteInput } from './invite-link';

@Component({
  selector: 'app-invite-link-form',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ReactiveFormsModule],
  template: `
    <section class="invite-link-section" aria-labelledby="invite-link-title">
      <h3 id="invite-link-title">У вас есть ссылка приглашения?</h3>
      <input id="invite-link" type="text" [formControl]="value" placeholder="Вставьте её сюда" autocomplete="off" spellcheck="false" aria-label="Ссылка или токен приглашения">
      <button class="themed-button" type="button" [disabled]="value.invalid" (click)="submit()">Присоединиться</button>
    </section>
  `,
})
export class InviteLinkComponent {
  private readonly notifications = inject(NotificationStore);
  readonly openInvite = output<string>();
  protected readonly value = new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.pattern(/\S+/)] });

  protected submit(): void {
    const token = parseInviteInput(this.value.value, location.origin);
    if (!token) {
      this.notifications.error('Введите ссылку приглашения threaden или токен inv_…');
      return;
    }
    this.value.reset('');
    this.openInvite.emit(token);
  }
}
