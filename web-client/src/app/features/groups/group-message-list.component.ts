import { ChangeDetectionStrategy, Component, ElementRef, effect, input, output, viewChild, viewChildren } from '@angular/core';
import type { GroupMessage } from '../../core/api/models';
import { chatMessage, isSystemMessage, systemMessageText, type MessageView } from '../../core/events/groups.store';
import { AvatarComponent } from '../../shared/avatar/avatar.component';
import { GroupMessageActionsComponent } from './message-actions/group-message-actions.component';
import { MessageAttachmentsComponent } from './attachments/message-attachments.component';

@Component({
  selector: 'app-group-message-list',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [AvatarComponent, GroupMessageActionsComponent, MessageAttachmentsComponent],
  template: `
    <div #messageList class="message-list">
      @if (loading()) {
        @for (item of skeletons; track item) {
          <div class="skeleton-message" [class.skeleton-message--own]="item % 3 === 1">
            @if (item % 3 !== 1) { <span class="skeleton skeleton-message__avatar"></span> }
            <span class="skeleton-message__bubble"><span class="skeleton skeleton-line skeleton-line--1"></span><span class="skeleton skeleton-line skeleton-line--2"></span></span>
          </div>
        }
      } @else {
        @for (item of messages(); track messageId(item); let index = $index) {
          @if (chatMessage(item); as chat) {
            <article
              class="chat-message"
              animate.leave="message-leave"
              [class.chat-message--own]="isOwn(chat)"
              [class.chat-message--other]="!isOwn(chat)"
              [attr.data-status]="chat.status"
              [attr.data-compact]="isCompact(index)"
              [attr.data-animate]="chat.animate || null"
            >
              @if (!isOwn(chat)) {
                <app-avatar class="chat-message__avatar" [src]="isCompact(index) ? '' : (chat.message.author.avatar || '')" [label]="chat.message.author.display_name" />
              }
              <div class="chat-message__bubble" (contextmenu)="openContextMenu($event, chat.message)">
                @if (!isOwn(chat) && !isCompact(index)) { <strong class="chat-message__author">{{ chat.message.author.display_name }}</strong> }
                @if (chat.message.reply_to; as reply) { <div class="message-reply-preview">@if (reply.kind === 'system') { <strong>Системное уведомление</strong> } @else { <strong>В ответ {{ reply.author.display_name }}</strong> }<span>{{ reply.body || 'Вложение' }}</span></div> }
                @if (chat.message.body) { <p>{{ chat.message.body }}</p> }
                @if (chat.message.attachments?.length) { <app-message-attachments [attachments]="chat.message.attachments ?? []" /> }
                <footer><time [attr.datetime]="chat.message.created_at">{{ formatTime(chat.message.created_at) }}</time>@if (isOwn(chat) && chat.status === 'sent') { <span class="message-status" [attr.aria-label]="chat.message.read ? 'Прочитано' : 'Отправлено'">{{ chat.message.read ? '✓✓' : '✓' }}</span> }{{ statusSuffix(chat) }}</footer>
                @if (chat.status === 'sent') { <app-group-message-actions [message]="chat.message" [own]="isOwn(chat)" [canDelete]="isOwn(chat) || groupOwnerId() === currentUserId()" (reply)="reply.emit($event)" (remove)="remove.emit($event)" /> }
              </div>
            </article>
          } @else {
            <article class="system-message" animate.leave="message-leave" [attr.data-animate]="item.animate">
              <span class="system-message__body">{{ systemMessageText(item) }}</span>
              @if (systemMessage(item); as system) {
                <app-group-message-actions [message]="system" [own]="false" [canDelete]="groupOwnerId() === currentUserId()" (reply)="reply.emit($event)" (remove)="remove.emit($event)" />
              }
            </article>
          }
        } @empty { <p class="empty-copy">Сообщений пока нет</p> }
      }
    </div>
  `,
})
export class GroupMessageListComponent {
  readonly messages = input<MessageView[]>([]);
  readonly loading = input(false);
  readonly currentUserId = input<string | undefined>(undefined);
  readonly groupOwnerId = input<string | undefined>(undefined);
  readonly reply = output<GroupMessage>();
  readonly remove = output<GroupMessage>();
  private readonly messageList = viewChild<ElementRef<HTMLElement>>('messageList');
  private readonly messageActions = viewChildren(GroupMessageActionsComponent);
  protected readonly skeletons = Array.from({ length: 7 }, (_, index) => index);
  protected readonly chatMessage = chatMessage;
  protected readonly systemMessageText = systemMessageText;
  protected readonly systemMessage = (item: MessageView): GroupMessage | null => isSystemMessage(item) ? item.message : null;
  constructor() {
    effect(() => {
      this.messages();
      window.requestAnimationFrame(() => {
        const element = this.messageList()?.nativeElement;
        if (element) element.scrollTop = element.scrollHeight;
      });
    });
  }
  protected messageId(item: MessageView): string { return isSystemMessage(item) ? item.id : item.viewID ?? item.message.id; }
  protected openContextMenu(event: MouseEvent, message: GroupMessage): void {
    this.messageActions().find((actions) => actions.message().id === message.id)?.openContextMenu(event);
  }
  protected isOwn(item: MessageView): boolean { return !isSystemMessage(item) && item.message.author.id === this.currentUserId(); }
  protected isCompact(index: number): boolean {
    const items = this.messages();
    const previous = items[index - 1];
    const current = items[index];
    return Boolean(previous && current && !isSystemMessage(previous) && !isSystemMessage(current) && previous.message.author.id === current.message.author.id);
  }
  protected formatTime(value: string): string { return new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }); }
  protected statusSuffix(item: MessageView): string {
    return isSystemMessage(item) ? '' : item.status === 'sending' ? ' · отправка' : item.status === 'error' ? ' · ошибка' : item.message.edited_at ? ' · изменено' : '';
  }
}
