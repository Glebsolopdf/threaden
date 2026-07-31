import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { NotificationStore } from '../../core/notifications/notification.store';

@Component({
  selector: 'app-notification-outlet',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (notifications.current(); as notification) {
      <section class="notification-region" animate.leave="notification-leave" aria-live="polite" aria-label="Статусные уведомления">
        <article
          class="status-notification"
          [attr.data-kind]="notification.kind"
          [attr.role]="notification.kind === 'error' ? 'alert' : 'status'"
          (click)="notifications.clear()"
        >
          <p>{{ notification.message }}</p><span></span>
        </article>
      </section>
    }
  `,
})
export class NotificationOutletComponent {
  protected readonly notifications = inject(NotificationStore);
}
