import { ChangeDetectionStrategy, Component } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { NotificationOutletComponent } from './shared/notification-outlet/notification-outlet.component';
import { RoomWidgetComponent } from './shared/room-widget/room-widget.component';

@Component({
  selector: 'app-root',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterOutlet, NotificationOutletComponent, RoomWidgetComponent],
  template: `<router-outlet /><app-notification-outlet /><app-room-widget />`,
})
export class AppComponent {}
