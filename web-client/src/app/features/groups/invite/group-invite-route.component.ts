import { ChangeDetectionStrategy, Component, effect, inject, input, signal } from '@angular/core';
import { Router } from '@angular/router';
import { GroupsStore } from '../../../core/events/groups.store';
import { NotificationStore } from '../../../core/notifications/notification.store';
import { HistoryNoticeState } from '../history/history-notice';
import { GroupInviteComponent } from './group-invite.component';

@Component({
  selector: 'app-group-invite-route',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [GroupInviteComponent],
  template: `
    <section class="route-page" id="group-invite-view">
      @if (groups.groupLoading()) {
        <div class="group-loading" aria-busy="true" aria-label="Загрузка приглашения"></div>
      } @else if (groups.current(); as group) {
        <app-group-invite [group]="group" [pending]="joining()" (accept)="accept()" (cancel)="cancel()" />
      } @else {
        <div class="empty-state">Приглашение не найдено</div>
      }
    </section>
  `,
})
export class GroupInviteRouteComponent {
  readonly inviteToken = input('');
  protected readonly groups = inject(GroupsStore);
  private readonly notifications = inject(NotificationStore);
  private readonly router = inject(Router);
  private readonly historyNotice = inject(HistoryNoticeState);
  protected readonly joining = signal(false);
  private loadedToken = '';

  constructor() {
    effect(() => {
      const token = this.inviteToken();
      if (!token || token === this.loadedToken) return;
      this.loadedToken = token;
      void this.load(token);
    });
  }

  private async load(token: string): Promise<void> {
    try { await this.groups.openInvite(token); }
    catch (error) { this.notifications.error(error, 'Не удалось загрузить приглашение'); }
  }

  protected async accept(): Promise<void> {
    if (this.joining() || !this.inviteToken()) return;
    this.joining.set(true);
    try {
      await this.groups.joinCurrent(this.inviteToken());
      const joined = this.groups.current();
      if (joined?.visibility === 'private') this.historyNotice.markAfterJoin(joined.id);
      await this.router.navigate(['/groups', joined?.id]);
    } catch (error) { this.notifications.error(error, 'Не удалось присоединиться к группе'); }
    finally { this.joining.set(false); }
  }

  protected async cancel(): Promise<void> { await this.router.navigate(['/']); }
}
