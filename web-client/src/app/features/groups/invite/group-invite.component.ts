import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import type { GroupInfo } from '../../../core/api/models';
import { AvatarComponent } from '../../../shared/avatar/avatar.component';

@Component({
  selector: 'app-group-invite',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [AvatarComponent],
  template: `
    <section class="group-invite-screen" aria-labelledby="group-invite-title">
      <p class="group-invite-screen__eyebrow">Вас пригласили в группу</p>
      <app-avatar class="group-invite-screen__avatar" [src]="group().avatar" [label]="group().name" [identity]="group().id" [kind]="'group'" />
      <h1 id="group-invite-title">{{ group().name }}</h1>
      <p class="group-invite-screen__copy">Присоединитесь, чтобы видеть новые сообщения и общаться с участниками.</p>
      <div class="group-invite-screen__actions">
        <button class="themed-button" type="button" [disabled]="pending()" (click)="accept.emit()">{{ pending() ? 'Принимаем…' : 'Принять приглашение' }}</button>
        <button type="button" [disabled]="pending()" (click)="cancel.emit()">Отмена</button>
      </div>
    </section>
  `,
})
export class GroupInviteComponent {
  readonly group = input.required<GroupInfo>();
  readonly pending = input(false);
  readonly accept = output<void>();
  readonly cancel = output<void>();
}
