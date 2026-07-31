import { ChangeDetectionStrategy, Component, ElementRef, HostListener, computed, effect, inject, viewChild } from '@angular/core';
import { Router } from '@angular/router';
import { VoiceService } from '../../core/voice/voice.service';
import { NotificationStore } from '../../core/notifications/notification.store';
import { AvatarComponent } from '../avatar/avatar.component';

const POSITION_KEY = 'voice_rooms_widget_position';

@Component({
  selector: 'app-room-widget',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [AvatarComponent],
  template: `
    @if (voice.activeRoom(); as room) {
      @if (voice.minimized()) {
        <section #widget class="room-widget" aria-label="Активная голосовая комната" (pointerdown)="beginDrag($event)">
          <div class="room-widget__handle" aria-hidden="true"></div>
          <div class="room-widget__speakers" aria-label="Последние говорившие">
            @for (participant of speakers(); track participant.identity) {
              <app-avatar [src]="participant.avatar" [label]="participant.name" />
            }
          </div>
          <div class="room-widget__summary">
            <strong>{{ room.title }}</strong>
            <small>{{ voice.statusLabel() }} · {{ voice.participants().length }}</small>
          </div>
          <button class="room-widget__button" type="button" [attr.aria-label]="voice.microphoneEnabled() ? 'Выключить микрофон' : 'Включить микрофон'" (click)="toggleMic($event)">
            <img [src]="voice.microphoneEnabled() ? '/microphone-on.svg' : '/microphone-off.svg'" alt="">
          </button>
          <button class="room-widget__button" type="button" aria-label="Вернуться в полноэкранный режим" (click)="openRoom($event)">
            <span class="room-widget__fullscreen-mark" aria-hidden="true"></span>
          </button>
        </section>
      }
    }
  `,
})
export class RoomWidgetComponent {
  protected readonly voice = inject(VoiceService);
  private readonly router = inject(Router);
  private readonly notifications = inject(NotificationStore);
  private readonly widget = viewChild<ElementRef<HTMLElement>>('widget');
  private drag?: { pointerId: number; offsetX: number; offsetY: number };
  protected readonly speakers = computed(() => {
    const people = this.voice.participants();
    return [...people.filter((person) => person.isSpeaking), ...people.filter((person) => !person.isSpeaking)].slice(0, 3);
  });

  constructor() {
    effect(() => {
      if (!this.voice.activeRoom() || !this.voice.minimized()) return;
      const element = this.widget()?.nativeElement;
      if (element) queueMicrotask(() => this.restorePosition());
    });
  }

  protected async toggleMic(event: Event): Promise<void> {
    event.stopPropagation();
    try { await this.voice.toggleMicrophone(); } catch (error) { this.notifications.error(error, 'Не удалось изменить состояние микрофона'); }
  }

  protected openRoom(event: Event): void {
    event.stopPropagation();
    this.voice.restore();
    const room = this.voice.activeRoom();
    if (room) void this.router.navigate(room.kind === 'group' ? ['/group-voice-rooms', room.id] : ['/temporary', room.id]);
  }

  protected beginDrag(event: PointerEvent): void {
    if ((event.target as HTMLElement).closest('button')) return;
    const element = this.widget()?.nativeElement;
    if (!element) return;
    const rect = element.getBoundingClientRect();
    this.drag = { pointerId: event.pointerId, offsetX: event.clientX - rect.left, offsetY: event.clientY - rect.top };
    element.setPointerCapture(event.pointerId);
  }

  @HostListener('document:pointermove', ['$event'])
  protected moveDrag(event: PointerEvent): void {
    const element = this.widget()?.nativeElement;
    if (!element || !this.drag || event.pointerId !== this.drag.pointerId) return;
    this.setPosition(element, event.clientX - this.drag.offsetX, event.clientY - this.drag.offsetY);
  }

  @HostListener('document:pointerup', ['$event'])
  protected endDrag(event: PointerEvent): void {
    const element = this.widget()?.nativeElement;
    if (!element || !this.drag || event.pointerId !== this.drag.pointerId) return;
    this.drag = undefined;
    const rect = element.getBoundingClientRect();
    try { localStorage.setItem(POSITION_KEY, JSON.stringify({ x: rect.left, y: rect.top })); } catch { /* optional */ }
  }

  @HostListener('window:resize')
  protected restorePosition(): void {
    const element = this.widget()?.nativeElement;
    if (!element) return;
    try {
      const value = JSON.parse(localStorage.getItem(POSITION_KEY) ?? '{}') as { x?: number; y?: number };
      this.setPosition(element, value.x ?? window.innerWidth - element.offsetWidth - 16, value.y ?? 16);
    } catch {
      this.setPosition(element, window.innerWidth - element.offsetWidth - 16, 16);
    }
  }

  private setPosition(element: HTMLElement, x: number, y: number): void {
    const left = Math.max(16, Math.min(window.innerWidth - element.offsetWidth - 16, x));
    const top = Math.max(16, Math.min(window.innerHeight - element.offsetHeight - 16, y));
    element.style.left = `${left}px`;
    element.style.top = `${top}px`;
  }
}
