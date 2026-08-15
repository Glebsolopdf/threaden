import { ChangeDetectionStrategy, Component, DestroyRef, inject, signal } from '@angular/core';
import { Router, RouterOutlet } from '@angular/router';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { filter } from 'rxjs';
import { NotificationOutletComponent } from './shared/notification-outlet/notification-outlet.component';
import { RoomWidgetComponent } from './shared/room-widget/room-widget.component';
import { isNavigationSettled } from './loading/loading-state';

@Component({
  selector: 'app-root',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterOutlet, NotificationOutletComponent, RoomWidgetComponent],
  template: `
    @if (loading()) {
      <div class="app-loading" role="status" aria-live="polite">
        <img class="app-loading__logo" src="/threaden-logo.svg" alt="threaden">
        <span class="visually-hidden">Загрузка…</span>
      </div>
    }
    <router-outlet /><app-notification-outlet /><app-room-widget />
  `,
})
export class AppComponent {
  protected readonly loading = signal(true);

  constructor() {
    inject(Router).events.pipe(filter(isNavigationSettled), takeUntilDestroyed(inject(DestroyRef))).subscribe(() => this.loading.set(false));
  }
}
