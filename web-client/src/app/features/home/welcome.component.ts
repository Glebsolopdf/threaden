import { ChangeDetectionStrategy, Component } from '@angular/core';

@Component({
  selector: 'app-welcome',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="welcome">
      <h1 class="welcome__title">Добро пожаловать в Threaden</h1>
      <p class="welcome__subtitle">Общайтесь в группах и голосовых комнатах</p>
    </div>
  `,
})
export class WelcomeComponent {}
