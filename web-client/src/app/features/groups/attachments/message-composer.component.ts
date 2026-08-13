import { ChangeDetectionStrategy, Component, ElementRef, inject, input, output, signal, viewChild } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { GroupsStore } from '../../../core/events/groups.store';
import type { GroupMessage } from '../../../core/api/models';
import { NotificationStore } from '../../../core/notifications/notification.store';
import { TypingStore } from '../../../core/events/typing.store';
import { formatBytes, validateSelection } from './attachment-upload';

@Component({
  selector: 'app-message-composer',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ReactiveFormsModule],
  template: `
    <div class="composer-stack">
      @if (replyingTo(); as reply) {
        <div class="reply-banner"><span><strong>В ответ {{ reply.author.display_name }}</strong><small>{{ reply.body }}</small></span><button type="button" aria-label="Отменить ответ" (click)="cancelReply.emit()">×</button></div>
      }
      @if (files().length) {
        <div class="composer-files" aria-label="Выбранные файлы">
          @for (file of files(); track file.name + file.lastModified) {
            <span class="composer-file"><span>{{ file.name }}</span><small>{{ formatBytes(file.size) }}</small><button type="button" (click)="removeFile(file)" [attr.aria-label]="'Удалить ' + file.name">×</button></span>
          }
        </div>
      }
      <form class="composer" [formGroup]="messageForm" (ngSubmit)="send()" autocomplete="off">
        <input #fileInput type="file" multiple hidden (change)="selectFiles($event)">
        <button class="composer__attach" type="button" aria-label="Прикрепить файл" (click)="fileInput.click()">＋</button>
        <input formControlName="body" maxlength="2000" placeholder="Сообщение" autocomplete="off" spellcheck="true" (input)="typing.notify(groupId(), messageForm.controls.body.value.trim().length > 0)">
        <button type="submit" [disabled]="!canSend() || sending()">{{ sending() ? 'Отправка…' : 'Отправить' }}</button>
      </form>
      @if (progress() > 0 && progress() < 100) { <progress [value]="progress()" max="100">{{ progress() }}%</progress> }
    </div>
  `,
})
export class MessageComposerComponent {
  readonly groupId = input('');
  readonly replyingTo = input<GroupMessage | null>(null);
  readonly cancelReply = output<void>();
  private readonly groups = inject(GroupsStore);
  private readonly notifications = inject(NotificationStore);
  protected readonly typing = inject(TypingStore);
  protected readonly files = signal<File[]>([]);
  protected readonly sending = signal(false);
  protected readonly progress = signal(0);
  protected readonly messageForm = new FormGroup({ body: new FormControl('', { nonNullable: true, validators: [Validators.maxLength(2000)] }) });
  private readonly fileInput = viewChild<ElementRef<HTMLInputElement>>('fileInput');
  protected readonly formatBytes = formatBytes;

  protected canSend(): boolean { return this.files().length > 0 || this.messageForm.controls.body.value.trim().length > 0; }

  protected selectFiles(event: Event): void {
    const input = event.target as HTMLInputElement;
    const selected = Array.from(input.files ?? []);
    const error = validateSelection(selected);
    if (error) {
      this.notifications.error(error);
      input.value = '';
      return;
    }
    this.files.set(selected);
  }

  protected removeFile(file: File): void { this.files.update((items) => items.filter((item) => item !== file)); }

  protected async send(): Promise<void> {
    if (!this.canSend() || this.sending()) return;
    const body = this.messageForm.controls.body.value;
    const files = this.files();
    if (!files.length) {
      await this.groups.sendMessage(body, this.replyingTo()?.id ?? '');
      this.reset();
      return;
    }
    this.sending.set(true);
    this.progress.set(1);
    try {
      await this.groups.sendMessageWithFiles(body, files, this.replyingTo()?.id ?? '');
      this.reset();
      this.cancelReply.emit();
    } catch (error) {
      this.notifications.error(error, 'Не удалось отправить вложение');
    } finally {
      this.sending.set(false);
      this.progress.set(0);
    }
  }

  private reset(): void {
    this.messageForm.reset({ body: '' });
    this.files.set([]);
    const input = this.fileInput()?.nativeElement;
    if (input) input.value = '';
    this.typing.notify(this.groupId(), false);
  }
}
