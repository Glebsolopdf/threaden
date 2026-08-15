import { ChangeDetectionStrategy, Component, ElementRef, HostListener, inject, input, OnDestroy, output, signal, viewChild } from '@angular/core';
import type { GroupMessage } from '../../../core/api/models';
import { placeMessageMenu } from './message-actions-position';

@Component({
  selector: 'app-group-message-actions',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="message-actions" (contextmenu)="showMenu($event)">
      <span class="message-swipe-indicator" [style.opacity]="swipeOpacity()" [style.transform]="swipeTransform()" aria-hidden="true">
        <img src="/reply.svg" alt="">
      </span>
      @if (menuMounted()) {
        <div #menu class="message-actions__menu" [class.message-actions__menu--own]="own()" [class.message-actions__menu--other]="!own()" [class.message-actions__menu--above]="menuAbove()" [class.message-actions__menu--visible]="open()" [style.left.px]="menuLeft()" [style.top.px]="menuTop()" role="menu" aria-label="Действия с сообщением">
          <button type="button" role="menuitem" (click)="chooseReply()"><img src="/reply.svg" alt=""><span>Ответить</span></button>
          <button type="button" role="menuitem" (click)="chooseCopy()"><img src="/copy.svg" alt=""><span>Копировать</span></button>
          @if (canDelete()) { <button type="button" role="menuitem" class="message-actions__danger" aria-label="Удалить у всех" title="Удалить у всех" (click)="chooseDelete()"><img src="/trash.svg" alt=""><span>Удалить у всех</span></button> }
        </div>
      }
    </div>
  `,
})
export class GroupMessageActionsComponent implements OnDestroy {
  readonly message = input.required<GroupMessage>();
  readonly own = input(false);
  readonly canDelete = input(false);
  readonly reply = output<GroupMessage>();
  readonly remove = output<GroupMessage>();
  protected readonly open = signal(false);
  protected readonly menuMounted = signal(false);
  protected readonly menuAbove = signal(false);
  protected readonly menuLeft = signal(0);
  protected readonly menuTop = signal(0);
  protected readonly swipeX = signal(0);
  private longPress?: number;
  private closeTimer?: number;
  private startX = 0;
  private startY = 0;
  private positionFrame?: number;
  private resizeObserver?: ResizeObserver;
  private readonly repositionOnResize = (): void => this.schedulePosition();
  private readonly repositionOnScroll = (): void => this.schedulePosition();
  private readonly host = inject(ElementRef<HTMLElement>);
  private readonly menu = viewChild<ElementRef<HTMLElement>>('menu');
  private static active?: GroupMessageActionsComponent;

  protected showMenu(event: MouseEvent): void { event.preventDefault(); this.openMenu(); }

  @HostListener('pointerdown', ['$event'])
  protected startLongPress(event: PointerEvent): void {
    this.startX = event.clientX;
    this.startY = event.clientY;
    this.swipeX.set(0);
    if (event.pointerType !== 'mouse') this.longPress = window.setTimeout(() => this.openMenu(), 500);
  }

  @HostListener('pointermove', ['$event'])
  protected trackSwipe(event: PointerEvent): void {
    if (event.pointerType === 'mouse') return;
    const dx = this.startX - event.clientX;
    const dy = Math.abs(event.clientY - this.startY);
    if (dy > Math.abs(dx) && dy > 12) { this.cancelLongPress(); this.resetSwipe(); return; }
    if (dx > 8) {
      event.preventDefault();
      this.cancelLongPress();
      this.swipeX.set(Math.min(84, dx * 0.78));
      this.setBubbleSwipe(this.swipeX());
    }
  }

  @HostListener('pointerup', ['$event'])
  protected replyOnSwipe(event: PointerEvent): void {
    if (event.pointerType === 'mouse') { this.cancelLongPress(); this.resetSwipe(); return; }
    if (this.swipeX() >= 56) { this.closeMenu(); this.reply.emit(this.message()); }
    this.cancelLongPress();
    this.resetSwipe();
  }

  @HostListener('pointercancel')
  protected cancelSwipe(): void { this.cancelLongPress(); this.resetSwipe(); }

  @HostListener('document:pointerdown', ['$event'])
  protected closeOutside(event: PointerEvent): void {
    if (!(event.target instanceof Node) || !this.host.nativeElement.contains(event.target)) this.closeMenu();
  }

  @HostListener('document:keydown.escape')
  protected closeOnEscape(): void { this.closeMenu(); }

  protected chooseReply(): void { this.closeMenu(); this.reply.emit(this.message()); }
  protected async chooseCopy(): Promise<void> {
    this.closeMenu();
    try { await navigator.clipboard.writeText(this.message().body); } catch { /* Clipboard may be unavailable outside a secure context. */ }
  }
  protected chooseDelete(): void { this.closeMenu(); this.remove.emit(this.message()); }

  protected swipeOpacity(): number { return Math.min(1, this.swipeX() / 48); }
  protected swipeTransform(): string { return `translateY(-50%) scale(${0.72 + this.swipeOpacity() * 0.28})`; }

  private openMenu(): void {
    GroupMessageActionsComponent.active?.closeMenu(true);
    GroupMessageActionsComponent.active = this;
    if (this.closeTimer !== undefined) window.clearTimeout(this.closeTimer);
    this.menuMounted.set(true);
    requestAnimationFrame(() => { this.startPositionTracking(); this.schedulePosition(); });
  }

  private closeMenu(immediate = false): void {
    if (!this.menuMounted()) return;
    if (GroupMessageActionsComponent.active === this) GroupMessageActionsComponent.active = undefined;
    this.stopPositionTracking();
    if (immediate) {
      this.open.set(false);
      this.menuMounted.set(false);
      return;
    }
    this.open.set(false);
    if (this.closeTimer !== undefined) window.clearTimeout(this.closeTimer);
    this.closeTimer = window.setTimeout(() => {
      if (!this.open()) this.menuMounted.set(false);
    }, 200);
  }

  ngOnDestroy(): void {
    this.cancelLongPress();
    this.stopPositionTracking();
    if (this.closeTimer !== undefined) window.clearTimeout(this.closeTimer);
    this.setBubbleSwipe(0);
  }

  private startPositionTracking(): void {
    this.stopPositionTracking();
    window.addEventListener('resize', this.repositionOnResize);
    document.addEventListener('scroll', this.repositionOnScroll, true);
    const anchor = this.host.nativeElement.parentElement;
    const menu = this.menu()?.nativeElement;
    if (typeof ResizeObserver !== 'undefined' && anchor && menu) {
      this.resizeObserver = new ResizeObserver(() => this.schedulePosition());
      this.resizeObserver.observe(anchor);
      this.resizeObserver.observe(menu);
    }
  }

  private stopPositionTracking(): void {
    window.removeEventListener('resize', this.repositionOnResize);
    document.removeEventListener('scroll', this.repositionOnScroll, true);
    this.resizeObserver?.disconnect();
    this.resizeObserver = undefined;
    if (this.positionFrame !== undefined) cancelAnimationFrame(this.positionFrame);
    this.positionFrame = undefined;
  }

  private schedulePosition(): void {
    if (!this.menuMounted() || this.positionFrame !== undefined) return;
    this.positionFrame = requestAnimationFrame(() => {
      this.positionFrame = undefined;
      const anchor = this.host.nativeElement.parentElement?.getBoundingClientRect();
      const menu = this.menu()?.nativeElement.getBoundingClientRect();
      if (!anchor || !menu) return;
      const placement = placeMessageMenu(anchor, menu, { width: window.innerWidth, height: window.innerHeight }, 8);
      this.menuAbove.set(placement.above);
      this.menuLeft.set(placement.left);
      this.menuTop.set(placement.top);
      this.open.set(true);
    });
  }

  private cancelLongPress(): void {
    if (this.longPress !== undefined) window.clearTimeout(this.longPress);
    this.longPress = undefined;
  }

  private resetSwipe(): void {
    this.swipeX.set(0);
    this.setBubbleSwipe(0);
  }

  private setBubbleSwipe(value: number): void {
    const bubble = this.host.nativeElement.parentElement;
    if (!bubble) return;
    if (value) bubble.style.setProperty('--message-swipe-transform', `translate3d(-${value}px, 0, 0)`);
    else bubble.style.removeProperty('--message-swipe-transform');
  }
}
