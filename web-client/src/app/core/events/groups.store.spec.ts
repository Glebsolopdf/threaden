import { TestBed } from '@angular/core/testing';
import { signal } from '@angular/core';
import { of, Subject } from 'rxjs';
import type { GroupMessage, User } from '../api/models';
import { ApiService } from '../api/api.service';
import { AuthStore } from '../auth/auth.store';
import { chatMessage, GroupsStore } from './groups.store';

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

  it('keeps a persisted system message in the common message stream', () => {
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
      body: 'Глеб исключён из чата', created_at: '2026-01-01T00:00:00Z',
    };
    store.mergeMessage(message);

    expect(store.messages()).toMatchObject([{ kind: 'system', body: 'Глеб исключён из чата', message }]);
  });
});
