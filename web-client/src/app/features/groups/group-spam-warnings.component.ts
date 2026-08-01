import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import type { GroupSpamWarning } from '../../core/api/models';

@Component({
  selector: 'app-group-spam-warnings',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (warnings().length) {
      <section class="group-info-dialog__section" aria-labelledby="group-spam-title">
        <h3 id="group-spam-title">Предупреждения</h3>
        <p class="empty-copy">За 30 дней: {{ warnings().length }}/3. После третьего предупреждения группа удаляется автоматически.</p>
        <ul class="group-members">
          @for (warning of warnings(); track warning.created_at) {
            <li class="group-member"><span><strong>{{ label(warning.reason) }}</strong><small>{{ warning.user_count }} участников, {{ warning.message_count }} похожих сообщений</small></span><small>{{ date(warning.created_at) }}</small></li>
          }
        </ul>
      </section>
    }
  `,
})
export class GroupSpamWarningsComponent {
  readonly warnings = input<GroupSpamWarning[]>([]);
  protected label(reason: string): string {
    return reason === 'near_duplicate_messages' ? 'Почти одинаковые сообщения' : reason === 'repeated_messages' ? 'Повторы сообщений' : 'Массовый спам';
  }
  protected date(value: string): string { return new Date(value).toLocaleString(); }
}
