import { inject, Injectable, signal } from '@angular/core';
import { Subject } from 'rxjs';
import type { EventEnvelope, GroupMemberEvent, GroupMemberEventType, GroupMessage, GroupTypingEvent } from '../api/models';
import { apiBaseUrl } from '../api/runtime-config';
import { TypingStore } from './typing.store';
import { NotificationStore } from '../notifications/notification.store';

export type ConnectionStatus =
  | { state: 'good'; label: string }
  | { state: 'reconnecting'; label: string }
  | { state: 'lost'; label: string };

@Injectable({ providedIn: 'root' })
export class EventStreamService {
  private readonly notifications = inject(NotificationStore);
  private readonly typing = inject(TypingStore);
  private source?: EventSource;
  private connectedOnce = false;

  readonly status = signal<ConnectionStatus>({ state: 'good', label: 'Хорошее соединение' });
  readonly messageCreated = new Subject<GroupMessage>();
  readonly profileUpdated = new Subject<GroupMemberEvent['member']>();
  readonly memberEvent = new Subject<{ type: GroupMemberEventType; groupID: string; member: GroupMemberEvent['member'] }>();
  readonly refreshRequested = new Subject<void>();

  connect(): void {
    if (this.source) return;
    const source = new EventSource(`${apiBaseUrl()}/v1/events`, { withCredentials: true });
    this.source = source;

    source.onopen = () => {
      this.connectedOnce = true;
      this.status.set({ state: 'good', label: 'Хорошее соединение' });
    };
    source.onerror = () => {
      this.status.set({
        state: this.connectedOnce ? 'reconnecting' : 'lost',
        label: this.connectedOnce ? 'Пытаемся восстановить соединение' : 'Соединение потеряно',
      });
    };

    source.addEventListener('message_created', (event) => this.handleEvent(event, true));
    source.addEventListener('typing_updated', (event) => this.handleTyping(event));
    source.addEventListener('profile_updated', (event) => this.handleProfileUpdated(event));
    for (const type of ['member_joined', 'member_left', 'member_removed'] as const) {
      source.addEventListener(type, (event) => this.handleMemberEvent(event, type));
    }
    for (const type of ['group_created', 'group_updated', 'group_deleted', 'voice_room_created', 'voice_room_deleted']) {
      source.addEventListener(type, (event) => this.handleEvent(event, false));
    }
  }

  private handleProfileUpdated(event: MessageEvent<string>): void {
    try {
      const payload = JSON.parse(event.data) as EventEnvelope<GroupMemberEvent>;
      if (payload.data && this.isMemberEvent(payload.data)) this.profileUpdated.next(payload.data.member);
      this.refreshRequested.next();
    } catch (error) {
      this.notifications.error(error, 'Получено некорректное событие сервера');
    }
  }

  private handleMemberEvent(event: MessageEvent<string>, type: GroupMemberEventType): void {
    try {
      const payload = JSON.parse(event.data) as EventEnvelope<GroupMemberEvent>;
      if (payload.group_id && payload.data && this.isMemberEvent(payload.data)) {
        this.memberEvent.next({ type, groupID: payload.group_id, member: payload.data.member });
      }
      this.refreshRequested.next();
    } catch (error) {
      this.notifications.error(error, 'Получено некорректное событие сервера');
    }
  }

  private handleTyping(event: MessageEvent<string>): void {
    try {
      const payload = JSON.parse(event.data) as EventEnvelope<GroupTypingEvent>;
      if (payload.group_id && payload.data && this.isTypingEvent(payload.data)) {
        this.typing.update(payload.group_id, payload.data);
      }
    } catch (error) {
      this.notifications.error(error, 'Получено некорректное событие сервера');
    }
  }

  disconnect(): void {
    this.source?.close();
    this.source = undefined;
    this.connectedOnce = false;
  }

  private handleEvent(event: MessageEvent<string>, hasMessage: boolean): void {
    try {
      const payload = JSON.parse(event.data) as EventEnvelope<GroupMessage>;
      if (hasMessage && payload.data && this.isGroupMessage(payload.data)) this.messageCreated.next(payload.data);
      this.refreshRequested.next();
    } catch (error) {
      this.notifications.error(error, 'Получено некорректное событие сервера');
    }
  }

  private isGroupMessage(value: unknown): value is GroupMessage {
    const item = value as Partial<GroupMessage> | null;
    return Boolean(item && typeof item.id === 'string' && typeof item.group_id === 'string' && typeof item.body === 'string');
  }

  private isMemberEvent(value: unknown): value is GroupMemberEvent {
    const member = (value as Partial<GroupMemberEvent> | null)?.member;
    return Boolean(member && typeof member.id === 'string' && typeof member.display_name === 'string');
  }

  private isTypingEvent(value: unknown): value is GroupTypingEvent {
    const item = value as Partial<GroupTypingEvent> | null;
    return Boolean(item && typeof item.active === 'boolean' && item.member && this.isMemberEvent(item));
  }
}
