import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { PreferencesService } from '../../core/preferences/preferences.service';

@Component({
  selector: 'app-auth-theme-toggle',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <button class="auth-theme-toggle" type="button" [attr.aria-label]="label()" [attr.aria-pressed]="preferences.theme() === 'light'" (click)="toggle()">
      <span aria-hidden="true">{{ preferences.theme() === 'light' ? '☾' : '☀' }}</span>
      <span>{{ preferences.theme() === 'light' ? 'Тёмная' : 'Светлая' }}</span>
    </button>
  `,
})
export class AuthThemeToggleComponent {
  protected readonly preferences = inject(PreferencesService);

  protected label(): string {
    return this.preferences.theme() === 'light' ? 'Переключить на тёмную тему' : 'Переключить на светлую тему';
  }

  protected toggle(): void {
    this.preferences.setTheme(this.preferences.theme() === 'light' ? 'dark' : 'light');
  }
}
