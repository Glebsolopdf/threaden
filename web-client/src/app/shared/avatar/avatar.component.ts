import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

@Component({
  selector: 'app-avatar',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { class: 'avatar' },
  template: `
    @if (src()) {
      <img [src]="src()" [alt]="label()" referrerpolicy="no-referrer">
    } @else {
      <span aria-hidden="true">{{ initials() }}</span>
      <span class="visually-hidden">{{ label() }}</span>
    }
  `,
  styles: [`
    :host { display: inline-grid; place-items: center; overflow: hidden; flex: 0 0 auto; background: var(--avatar-bg, #343740); }
    img { width: 100%; height: 100%; object-fit: cover; }
    span { line-height: 1; }
    .visually-hidden { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }
  `],
})
export class AvatarComponent {
  readonly src = input('');
  readonly label = input('Пользователь');
  readonly initials = computed(() => {
    const words = this.label().trim().split(/\s+/).filter(Boolean);
    return words.slice(0, 2).map((word) => [...word][0]?.toUpperCase() ?? '').join('') || '•';
  });
}
