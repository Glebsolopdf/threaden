import { AfterViewInit, ChangeDetectionStrategy, Component, ElementRef, HostListener, OnChanges, OnDestroy, input, signal, viewChild } from '@angular/core';
import type { ScreenShare } from '../../../core/voice/screen-share/screen-share.models';

@Component({
  selector: 'app-screen-share-stage',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      class="screen-share-stage__viewport"
      [class.screen-share-stage__viewport--draggable]="fullscreen() && zoom() > 1"
      (pointerdown)="startPan($event)"
      (pointermove)="movePan($event)"
      (pointerup)="endPan()"
      (pointercancel)="endPan()"
      (pointerleave)="endPan()"
    >
      <video
        #video
        autoplay
        playsinline
        [class.screen-share-stage__video--paused]="previewPaused()"
        [class.screen-share-stage__video--zoomed]="fullscreen() && zoom() > 1"
        [muted]="share().isLocal"
        [style.transform]="videoTransform()"
        [attr.aria-label]="'Демонстрация экрана: ' + share().participantName"
      ></video>
    </div>
    @if (fullscreen()) {
      <div class="screen-share-stage__controls" role="toolbar" aria-label="Управление масштабом">
        <button type="button" aria-label="Уменьшить" [disabled]="zoom() <= minZoom" (click)="stepZoom(-zoomStep)">−</button>
        <button type="button" aria-label="Сбросить масштаб" [disabled]="zoom() === 1" (click)="resetZoom()">{{ zoomLabel() }}</button>
        <button type="button" aria-label="Увеличить" [disabled]="zoom() >= maxZoom" (click)="stepZoom(zoomStep)">+</button>
      </div>
    }
    @if (previewPaused()) { <div class="screen-share-stage__notice" role="status"><strong>Ваш стрим продолжается</strong><span>Мы остановили предпросмотр для экономии ресурсов</span></div> }
  `,
})
export class ScreenShareStageComponent implements AfterViewInit, OnChanges, OnDestroy {
  readonly share = input.required<ScreenShare>();
  private readonly video = viewChild.required<ElementRef<HTMLVideoElement>>('video');
  private panPointerId?: number;
  private attachedTrack?: ScreenShare['videoTrack'];
  private ready = false;
  protected readonly previewPaused = signal(false);
  protected readonly fullscreen = signal(false);
  protected readonly zoom = signal(1);
  protected readonly offsetX = signal(0);
  protected readonly offsetY = signal(0);
  protected readonly minZoom = 1;
  protected readonly maxZoom = 3;
  protected readonly zoomStep = 0.25;

  ngAfterViewInit(): void { this.ready = true; this.syncPreview(); }
  ngOnChanges(): void { this.syncPreview(); }
  ngOnDestroy(): void { this.detach(); this.endPan(); }

  @HostListener('window:blur') protected pauseOnBlur(): void { this.syncPreview(); }
  @HostListener('window:focus') protected resumeOnFocus(): void { this.syncPreview(); }
  @HostListener('document:visibilitychange') protected visibilityChanged(): void { this.syncPreview(); }
  @HostListener('document:fullscreenchange') protected fullscreenChanged(): void {
    this.fullscreen.set(document.fullscreenElement?.contains(this.video().nativeElement) ?? false);
    if (!this.fullscreen()) this.resetZoom();
  }

  protected videoTransform(): string { return `translate(${this.offsetX()}px, ${this.offsetY()}px) scale(${this.zoom()})`; }
  protected zoomLabel(): string { return `${Math.round(this.zoom() * 100)}%`; }
  protected stepZoom(delta: number): void { this.setZoom(this.zoom() + delta); }
  protected resetZoom(): void {
    this.zoom.set(1);
    this.offsetX.set(0);
    this.offsetY.set(0);
  }
  protected startPan(event: PointerEvent): void {
    if (!this.fullscreen() || this.zoom() <= 1) return;
    this.panPointerId = event.pointerId;
    (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
  }
  protected movePan(event: PointerEvent): void {
    if (this.panPointerId !== event.pointerId || this.zoom() <= 1) return;
    this.offsetX.update((value) => this.clampOffset(value + event.movementX));
    this.offsetY.update((value) => this.clampOffset(value + event.movementY));
  }
  protected endPan(): void { this.panPointerId = undefined; }

  private attach(): void {
    const track = this.share().videoTrack;
    if (!this.ready || this.attachedTrack === track) return;
    this.detach();
    track.attach(this.video().nativeElement);
    this.attachedTrack = track;
  }

  private syncPreview(): void {
    if (!this.ready) return;
    const paused = this.share().isLocal && (!document.hasFocus() || document.visibilityState === 'hidden');
    if (paused) {
      this.detach();
      this.previewPaused.set(true);
      return;
    }
    this.previewPaused.set(false);
    this.attach();
  }
  private setZoom(next: number): void {
    const zoom = Math.max(this.minZoom, Math.min(this.maxZoom, Math.round(next * 100) / 100));
    this.zoom.set(zoom);
    if (zoom === 1) {
      this.offsetX.set(0);
      this.offsetY.set(0);
      return;
    }
    this.offsetX.update((value) => this.clampOffset(value));
    this.offsetY.update((value) => this.clampOffset(value));
  }
  private clampOffset(value: number): number {
    const bounds = Math.max(0, (this.zoom() - 1) * 240);
    return Math.max(-bounds, Math.min(bounds, value));
  }

  private detach(): void {
    if (!this.attachedTrack || !this.ready) return;
    this.attachedTrack.detach(this.video().nativeElement);
    this.attachedTrack = undefined;
  }
}
