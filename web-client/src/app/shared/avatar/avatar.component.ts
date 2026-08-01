import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

type AvatarKind = 'user' | 'group';

const GROUP_AVATAR_COLORS = ['#2f6f73', '#59683a', '#7a4f73', '#765b35', '#4f5f8f', '#7b4b4b', '#3f6d4f', '#6b5c92'];

@Component({
  selector: 'app-avatar',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    class: 'avatar',
    '[class.avatar--group-fallback]': 'showGroupFallback()',
    '[style.--avatar-bg]': 'fallbackColor()',
  },
  template: `
    @if (showGroupFallback()) {
      <img class="avatar__group-icon" src="/group-icon.svg" [alt]="label()">
    } @else if (src()) {
      <img [src]="src()" [alt]="label()" referrerpolicy="no-referrer">
    } @else {
      <span aria-hidden="true">{{ initials() }}</span>
      <span class="visually-hidden">{{ label() }}</span>
    }
  `,
  styles: [`
    :host { display: inline-grid; place-items: center; overflow: hidden; flex: 0 0 auto; background: var(--avatar-bg, #343740); }
    img { width: 100%; height: 100%; object-fit: cover; }
    .avatar__group-icon { width: 58%; height: 58%; object-fit: contain; opacity: 0.94; filter: drop-shadow(0 1px 0 rgb(0 0 0 / 18%)); }
    span { line-height: 1; }
    .visually-hidden { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }
  `],
})
export class AvatarComponent {
  readonly src = input('');
  readonly label = input('Пользователь');
  readonly identity = input('');
  readonly kind = input<AvatarKind>('user');
  readonly showGroupFallback = computed(() => this.kind() === 'group' && (!this.src().trim() || this.src().trim() === '👥'));
  readonly fallbackColor = computed(() => {
    if (!this.showGroupFallback()) return '';
    return GROUP_AVATAR_COLORS[this.hash(this.identity() || this.label()) % GROUP_AVATAR_COLORS.length];
  });
  readonly initials = computed(() => {
    const words = this.label().trim().split(/\s+/).filter(Boolean);
    return words.slice(0, 2).map((word) => [...word][0]?.toUpperCase() ?? '').join('') || '•';
  });

  private hash(value: string): number {
    let hash = 0;
    for (const char of value) hash = ((hash << 5) - hash + char.charCodeAt(0)) | 0;
    return Math.abs(hash);
  }
}
