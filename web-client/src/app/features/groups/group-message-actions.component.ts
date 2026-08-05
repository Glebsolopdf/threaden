import { ChangeDetectionStrategy, Component, ElementRef, HostListener, inject, input, output, signal, viewChild } from '@angular/core';
import type { GroupMessage } from '../../core/api/models';

@Component({
  selector: 'app-group-message-actions',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="message-actions" (contextmenu)="$event.preventDefault()">
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
export class GroupMessageActionsComponent {
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
  private readonly host = inject(ElementRef<HTMLElement>);
  private readonly menu = viewChild<ElementRef<HTMLElement>>('menu');
  private static active?: GroupMessageActionsComponent;

  @HostListener('contextmenu', ['$event'])
  protected showMenu(event: MouseEvent): void { event.preventDefault(); this.openMenu(event.clientX, event.clientY); }

  @HostListener('pointerdown', ['$event'])
  protected startLongPress(event: PointerEvent): void {
    this.startX = event.clientX;
    this.startY = event.clientY;
    this.swipeX.set(0);
    if (event.pointerType !== 'mouse') this.longPress = window.setTimeout(() => this.openMenu(this.startX, this.startY), 500);
  }

  @HostListener('pointermove', ['$event'])
  protected trackSwipe(event: PointerEvent): void {
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
    if (this.swipeX() >= 56) { this.closeMenu(); this.reply.emit(this.message()); }
    this.cancelLongPress();
    this.resetSwipe();
  }

  @HostListener('pointercancel')
  protected cancelSwipe(): void { this.cancelLongPress(); this.resetSwipe(); }

  @HostListener('document:pointerdown', ['$event'])
  protected closeOutside(event: PointerEvent): void {
    if (!(event.target instanceof Node) || !(event.target as Node).parentElement?.closest('app-group-message-actions')) this.closeMenu();
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

  private openMenu(clientX: number, clientY: number): void {
    GroupMessageActionsComponent.active?.closeMenu(true);
    GroupMessageActionsComponent.active = this;
    if (this.closeTimer !== undefined) window.clearTimeout(this.closeTimer);
    this.menuMounted.set(true);
    requestAnimationFrame(() => {
      const bubble = this.host.nativeElement.parentElement?.getBoundingClientRect();
      const menu = this.menu()?.nativeElement.getBoundingClientRect();
      const width = menu?.width ?? 140;
      const height = menu?.height ?? 78;
      const above = clientY > window.innerHeight - height - 16;
      const anchor = bubble ?? { left: clientX, right: clientX };
      const preferredLeft = this.own() ? anchor.left - width - 8 : anchor.right + 8;
      this.menuAbove.set(above);
      this.menuLeft.set(Math.max(8, Math.min(preferredLeft, window.innerWidth - width - 8)));
      this.menuTop.set(Math.max(8, above ? clientY - height - 8 : clientY + 8));
      this.open.set(true);
    });
  }

  private closeMenu(immediate = false): void {
    if (!this.menuMounted()) return;
    if (GroupMessageActionsComponent.active === this) GroupMessageActionsComponent.active = undefined;
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
