import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import type { MessageAttachment } from '../../../core/api/models';
import { formatBytes } from './attachment-upload';
import { AttachmentIconComponent } from './icons/attachment-icon.component';

@Component({
  selector: 'app-message-attachments',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [AttachmentIconComponent],
  template: `
    <div class="message-attachments">
      @for (attachment of attachments(); track attachment.id) {
        @if (attachment.kind === 'image') {
          <a class="message-attachment" [href]="attachment.url" [attr.download]="attachment.name"><img [src]="attachment.url" [alt]="attachment.name" loading="lazy"></a>
        } @else if (attachment.kind === 'video') {
          <video class="message-attachment" controls preload="metadata"><source [src]="attachment.url" [type]="attachment.mime"><a [href]="attachment.url">Скачать {{ attachment.name }}</a></video>
        } @else if (attachment.kind === 'audio') {
          <audio class="message-attachment" controls preload="metadata"><source [src]="attachment.url" [type]="attachment.mime"><a [href]="attachment.url">Скачать {{ attachment.name }}</a></audio>
        } @else {
          <a class="message-attachment" [href]="attachment.url" [attr.download]="attachment.name">
            <span class="message-attachment__file"><app-attachment-icon [kind]="attachment.kind" /><span class="message-attachment__meta"><strong>{{ attachment.name }}</strong><small>{{ formatBytes(attachment.size) }} · {{ attachment.kind === 'archive' ? 'Архив' : attachment.mime }}</small></span><span class="message-attachment__download" aria-hidden="true">↓</span></span>
          </a>
        }
      }
    </div>
  `,
})
export class MessageAttachmentsComponent {
  readonly attachments = input<MessageAttachment[]>([]);
  protected readonly formatBytes = formatBytes;
}
