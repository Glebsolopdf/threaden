import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import type { AttachmentKind } from '../attachment-upload';

@Component({
  selector: 'app-attachment-icon',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg class="attachment-icon" viewBox="0 0 48 48" aria-hidden="true" focusable="false">
      @if (kind() === 'archive') {
        <path class="attachment-icon__fill" d="M12 7h18l6 6v28H12z" />
        <path class="attachment-icon__line" d="M30 7v8h6M18 7v34M24 7v6M18 19h6M18 25h6M18 31h6" />
        <path class="attachment-icon__accent" d="M29 24h8v11h-8z" />
      } @else if (kind() === 'video') {
        <rect class="attachment-icon__fill" x="7" y="10" width="34" height="28" rx="5" />
        <path class="attachment-icon__accent" d="m21 18 11 6-11 6z" />
      } @else if (kind() === 'image') {
        <rect class="attachment-icon__fill" x="7" y="9" width="34" height="30" rx="5" />
        <circle class="attachment-icon__accent" cx="18" cy="19" r="3" />
        <path class="attachment-icon__line" d="m10 34 9-9 6 6 4-4 9 7" />
      } @else {
        <path class="attachment-icon__fill" d="M13 6h17l7 7v29H13z" />
        <path class="attachment-icon__line" d="M30 6v9h7M20 23h10M20 29h10M20 35h7" />
        <path class="attachment-icon__accent" d="M17 18h5v5h-5z" />
      }
    </svg>
  `,
})
export class AttachmentIconComponent {
  readonly kind = input<AttachmentKind>('file');
}
