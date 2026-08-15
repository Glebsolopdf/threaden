import { ChangeDetectionStrategy, Component, computed, ElementRef, inject, input, OnDestroy, output, signal, viewChild } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { GroupsStore } from '../../../core/events/groups.store';
import type { GroupMessage } from '../../../core/api/models';
import { NotificationStore } from '../../../core/notifications/notification.store';
import { TypingStore } from '../../../core/events/typing.store';
import { attachmentKind, canSendMessage, formatBytes, validateSelection } from './attachment-upload';
import { AttachmentIconComponent } from './icons/attachment-icon.component';
import { VoiceRecorder } from './voice/voice-recorder';
import { getVoiceWaveform } from './voice/voice-waveform';

@Component({
  selector: 'app-message-composer',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ReactiveFormsModule, AttachmentIconComponent],
  template: `
    <div class="composer-stack">
      @if (replyingTo(); as reply) {
        <div class="reply-banner"><span><strong>В ответ {{ reply.author.display_name }}</strong><small>{{ reply.body || 'Вложение' }}</small></span><button type="button" aria-label="Отменить ответ" (click)="cancelReply.emit()">×</button></div>
      }
      @if (files().length) {
        <div class="composer-files" aria-label="Выбранные файлы">
          @for (file of files(); track file.name + file.lastModified) {
            <article class="composer-file" [attr.title]="file.name">
              @if (attachmentKind(file) === 'image') {
                <img class="composer-file__preview" [src]="previewUrl(file)" [alt]="file.name">
              } @else if (attachmentKind(file) === 'audio') {
                <audio class="composer-file__preview" [src]="previewUrl(file)" controls preload="metadata"></audio>
              } @else {
                <span class="composer-file__preview"><app-attachment-icon [kind]="attachmentKind(file)" /></span>
              }
              <span class="composer-file__name">{{ file.name }}</span>
              <small>{{ formatBytes(file.size) }}</small>
              @if (sending()) { <span class="composer-file__loading" role="status" aria-label="Загрузка вложения"><span aria-hidden="true"></span></span> }
              <button type="button" (click)="removeFile(file)" [attr.aria-label]="'Удалить ' + file.name">×</button>
            </article>
          }
        </div>
      }
      <form class="composer" [formGroup]="messageForm" (ngSubmit)="send()" autocomplete="off">
        <input #fileInput type="file" multiple hidden (change)="selectFiles($event)">
        <button class="composer__attach" type="button" aria-label="Прикрепить файл" (click)="fileInput.click()">＋</button>
        @if (recording()) {
          <div class="composer__recording" role="status" aria-label="Идёт запись голосового сообщения">
            <div class="composer__waves" aria-hidden="true">
              @for (bar of waveformBars(); track $index) { <span [style.height.%]="bar * 100"></span> }
            </div>
            <button class="composer__cancel" type="button" (click)="cancelRecording()">Отмена</button>
          </div>
        } @else {
          <input formControlName="body" maxlength="2000" placeholder="Сообщение" autocomplete="off" spellcheck="true" (input)="typing.notify(groupId(), messageForm.controls.body.value.trim().length > 0)">
        }
        <button
          class="composer__action"
          [class.composer__action--recording]="recording()"
          [type]="canSend() && !recording() ? 'submit' : 'button'"
          [disabled]="sending()"
          [attr.aria-label]="recording() ? 'Остановить запись' : canSend() ? 'Отправить' : 'Записать голосовое сообщение'"
          (click)="actionClick()"
        >
          @if (canSend() && !recording()) { {{ sending() ? 'Отправка…' : 'Отправить' }} } @else { <img src="/microphone-on.svg" alt=""> }
        </button>
      </form>
    </div>
  `,
})
export class MessageComposerComponent implements OnDestroy {
  readonly groupId = input('');
  readonly replyingTo = input<GroupMessage | null>(null);
  readonly cancelReply = output<void>();
  private readonly groups = inject(GroupsStore);
  private readonly notifications = inject(NotificationStore);
  protected readonly typing = inject(TypingStore);
  protected readonly files = signal<File[]>([]);
  protected readonly sending = signal(false);
  protected readonly messageForm = new FormGroup({ body: new FormControl('', { nonNullable: true, validators: [Validators.maxLength(2000)] }) });
  private readonly fileInput = viewChild<ElementRef<HTMLInputElement>>('fileInput');
  protected readonly formatBytes = formatBytes;
  protected readonly attachmentKind = attachmentKind;
  protected readonly voiceRecorder = new VoiceRecorder();
  protected readonly waveformBars = computed(() => getVoiceWaveform(this.voiceRecorder.audioLevel(), 28));
  private readonly previewUrls = new Map<File, string>();

  protected canSend(): boolean { return canSendMessage(this.messageForm.controls.body.value, this.files().length); }

  protected recording(): boolean { return this.voiceRecorder.state() === 'recording'; }

  protected async actionClick(): Promise<void> {
    if (this.recording()) {
      await this.stopRecording();
      return;
    }
    if (!this.canSend()) await this.startRecording();
  }

  protected selectFiles(event: Event): void {
    const input = event.target as HTMLInputElement;
    const selected = Array.from(input.files ?? []);
    const error = validateSelection(selected);
    if (error) {
      this.notifications.error(error);
      input.value = '';
      return;
    }
    this.releasePreviewUrls();
    this.files.set(selected);
  }

  protected removeFile(file: File): void {
    const url = this.previewUrls.get(file);
    if (url) {
      URL.revokeObjectURL(url);
      this.previewUrls.delete(file);
    }
    this.files.update((items) => items.filter((item) => item !== file));
  }

  protected async startRecording(): Promise<void> {
    try {
      await this.voiceRecorder.start();
    } catch (error) {
      this.notifications.error(error, 'Не удалось включить микрофон');
    }
  }

  protected async stopRecording(): Promise<void> {
    try {
      const blob = await this.voiceRecorder.stop();
      const file = new File([blob], recordingName(blob.type), { type: blob.type });
      const selection = [...this.files(), file];
      const error = validateSelection(selection);
      if (error) {
        this.notifications.error(error);
        return;
      }
      this.files.set(selection);
    } catch (error) {
      this.notifications.error(error, 'Не удалось сохранить запись');
    }
  }

  protected cancelRecording(): void { this.voiceRecorder.cancel(); }

  protected previewUrl(file: File): string {
    const existing = this.previewUrls.get(file);
    if (existing) return existing;
    const url = URL.createObjectURL(file);
    this.previewUrls.set(file, url);
    return url;
  }

  ngOnDestroy(): void {
    this.voiceRecorder.cancel();
    this.releasePreviewUrls();
  }

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
    try {
      await this.groups.sendMessageWithFiles(body, files, this.replyingTo()?.id ?? '');
      this.reset();
      this.cancelReply.emit();
    } catch (error) {
      this.notifications.error(error, 'Не удалось отправить вложение');
    } finally {
      this.sending.set(false);
    }
  }

  private reset(): void {
    this.messageForm.reset({ body: '' });
    this.releasePreviewUrls();
    this.files.set([]);
    const input = this.fileInput()?.nativeElement;
    if (input) input.value = '';
    this.typing.notify(this.groupId(), false);
  }

  private releasePreviewUrls(): void {
    for (const url of this.previewUrls.values()) URL.revokeObjectURL(url);
    this.previewUrls.clear();
  }
}

function recordingName(mime: string): string {
  if (mime.includes('ogg')) return 'voice.ogg';
  if (mime.includes('mp4')) return 'voice.m4a';
  if (mime.includes('wav')) return 'voice.wav';
  return 'voice.webm';
}
