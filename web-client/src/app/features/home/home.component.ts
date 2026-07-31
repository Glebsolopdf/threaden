import { ChangeDetectionStrategy, Component } from '@angular/core';
import { WelcomeComponent } from './welcome.component';

@Component({
  selector: 'app-home',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [WelcomeComponent],
  template: `
    <section class="route-page welcome-route" aria-label="Рабочая область">
      @defer (on idle) {
        <app-welcome />
      } @placeholder (minimum 250ms) {
        <div class="welcome-placeholder" aria-hidden="true"></div>
      }
    </section>
  `,
})
export class HomeComponent {}
