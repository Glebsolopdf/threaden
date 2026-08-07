import { TestBed } from '@angular/core/testing';
import { signal } from '@angular/core';
import { of, Subject } from 'rxjs';
import type { GroupMessage, User } from '../api/models';
import { ApiService } from '../api/api.service';
import { AuthStore } from '../auth/auth.store';
import { chatMessage, GroupsStore, systemMessageText } from './groups.store';

describe('GroupsStore', () => {
  it('merges an SSE message that arrives before the send response', async () => {
    const response = new Subject<GroupMessage>();
    const user: User = { id: 'user-1', email: 'user@example.com', display_name: 'User', created_at: '' };
    const message: GroupMessage = {
      id: 'message-1', group_id: 'group-1', author: user, body: 'Hello', created_at: '2026-01-01T00:00:00Z',
    };

    TestBed.configureTestingModule({
      providers: [
        GroupsStore,
        { provide: ApiService, useValue: { sendMessage: () => response.asObservable(), groups: () => of([]) } },
        { provide: AuthStore, useValue: { user: signal(user) } },
      ],
    });
    const store = TestBed.inject(GroupsStore);
    store.current.set({ id: 'group-1' } as never);

    const sending = store.sendMessage('Hello');
    const viewID = chatMessage(store.messages()[0])?.viewID;
    store.mergeMessage(message);
    expect(store.messages()).toMatchObject([{ message, status: 'sent', animate: 'outgoing' }]);
    expect(chatMessage(store.messages()[0])?.viewID).toBe(viewID);
    response.next(message);
    response.complete();
    await sending;

    expect(store.messages()).toMatchObject([{ message, status: 'sent', animate: 'outgoing' }]);
  });

  it('renders a structured persisted system message with client-owned copy', () => {
    TestBed.configureTestingModule({
      providers: [
        GroupsStore,
        { provide: ApiService, useValue: { markGroupRead: () => of(undefined), groups: () => of([]) } },
        { provide: AuthStore, useValue: { user: signal({ id: 'user-1' }) } },
      ],
    });
    const store = TestBed.inject(GroupsStore);
    store.current.set({ id: 'group-1' } as never);
    const message: GroupMessage = {
      id: 'system-1', group_id: 'group-1', kind: 'system',
      author: { id: 'user-2', email: '', display_name: 'Глеб', created_at: '' },
      body: '', event: 'member_removed', created_at: '2026-01-01T00:00:00Z',
    };
    store.mergeMessage(message);

    const item = store.messages()[0];
    expect(item).toMatchObject({ kind: 'system', body: '', message });
    expect(systemMessageText(item)).toBe('Из чата исключён участник: Глеб');
  });

  it('keeps the backend body for legacy system messages without an event', () => {
    const message = {
      id: 'legacy-system', group_id: 'group-1', kind: 'system' as const,
      author: { id: 'user-2', email: '', display_name: 'Глеб', created_at: '' },
      body: 'Старое системное сообщение', created_at: '2026-01-01T00:00:00Z',
    };
    expect(systemMessageText({ kind: 'system', id: message.id, message, body: message.body, animate: 'incoming' })).toBe(message.body);
  });

  it('uses the existing copy for every membership event', () => {
    const author = { id: 'user-2', email: '', display_name: 'Глеб', created_at: '' };
    const cases = [
      ['member_joined', 'К чату присоединился участник: Глеб'],
      ['member_left', 'Из чата вышел участник: Глеб'],
      ['member_removed', 'Из чата исключён участник: Глеб'],
    ] as const;
    for (const [event, text] of cases) {
      const message: GroupMessage = { id: event, group_id: 'group-1', kind: 'system', event, author, body: '', created_at: '' };
      expect(systemMessageText({ kind: 'system', id: event, message, body: '', animate: 'incoming' })).toBe(text);
    }
  });
});
