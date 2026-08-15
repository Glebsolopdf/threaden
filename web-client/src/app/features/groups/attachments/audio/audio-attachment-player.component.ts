import { ChangeDetectionStrategy, Component, computed, effect, ElementRef, input, signal, viewChild } from '@angular/core';
import type { MessageAttachment } from '../../../../core/api/models';

@Component({
  selector: 'app-audio-attachment-player',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="message-audio">
      <button class="message-audio__play" type="button" [attr.aria-label]="playing() ? 'Поставить голосовое на паузу' : 'Воспроизвести голосовое'" (click)="toggle()">
        <span aria-hidden="true">{{ playing() ? 'Ⅱ' : '▶' }}</span>
      </button>
      <div
        class="message-audio__track"
        role="slider"
        tabindex="0"
        aria-label="Позиция голосового сообщения"
        [attr.aria-valuemin]="0"
        [attr.aria-valuemax]="duration()"
        [attr.aria-valuenow]="currentTime()"
        (click)="seek($event)"
        (keydown)="adjust($event)"
      >
        <span class="message-audio__progress" [style.width.%]="progress()"></span>
        <span class="message-audio__duration">{{ formatAudioTime(duration()) }}</span>
      </div>
      <audio #audio class="message-audio__native" [src]="attachment().url" preload="metadata" (loadedmetadata)="readDuration()" (timeupdate)="readTime()" (ended)="finish()"></audio>
    </div>
  `,
})
export class AudioAttachmentPlayerComponent {
  readonly attachment = input.required<MessageAttachment>();
  private readonly audio = viewChild<ElementRef<HTMLAudioElement>>('audio');
  protected readonly duration = signal(0);
  protected readonly currentTime = signal(0);
  protected readonly playing = signal(false);
  protected readonly progress = computed(() => this.duration() ? this.currentTime() / this.duration() * 100 : 0);
  protected readonly formatAudioTime = formatAudioTime;

  constructor() {
    effect(() => {
      const value = this.attachment().duration;
      if (value && Number.isFinite(value)) this.duration.set(value);
    });
  }

  protected async toggle(): Promise<void> {
    const element = this.audio()?.nativeElement;
    if (!element) return;
    if (element.paused) {
      try { await element.play(); } catch { this.playing.set(false); }
    } else {
      element.pause();
    }
    this.playing.set(!element.paused);
  }

  protected seek(event: MouseEvent): void {
    const element = this.audio()?.nativeElement;
    const track = event.currentTarget as HTMLElement;
    if (!element || !this.duration()) return;
    const ratio = Math.max(0, Math.min(1, (event.clientX - track.getBoundingClientRect().left) / track.clientWidth));
    element.currentTime = ratio * this.duration();
    this.currentTime.set(element.currentTime);
  }

  protected adjust(event: KeyboardEvent): void {
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
    event.preventDefault();
    const element = this.audio()?.nativeElement;
    if (!element) return;
    element.currentTime = Math.max(0, Math.min(this.duration(), element.currentTime + (event.key === 'ArrowRight' ? 5 : -5)));
    this.currentTime.set(element.currentTime);
  }

  protected readDuration(): void {
    const value = this.audio()?.nativeElement.duration;
    if (value && Number.isFinite(value)) this.duration.set(value);
  }

  protected readTime(): void { this.currentTime.set(this.audio()?.nativeElement.currentTime ?? 0); }
  protected finish(): void { this.playing.set(false); this.currentTime.set(0); }
}

export function formatAudioTime(seconds: number): string {
  const safeSeconds = Math.max(0, Math.floor(seconds || 0));
  return `${Math.floor(safeSeconds / 60)}:${String(safeSeconds % 60).padStart(2, '0')}`;
}
