import { TestBed } from '@angular/core/testing';
import { signal } from '@angular/core';
import { of } from 'rxjs';
import { ApiService } from '../api/api.service';
import { TypingStore } from './typing.store';

describe('TypingStore', () => {
  it('keeps private-group typing scoped to the event group', () => {
    TestBed.configureTestingModule({
      providers: [
        TypingStore,
        { provide: ApiService, useValue: { setTyping: () => of(undefined) } },
      ],
    });
    const store = TestBed.inject(TypingStore);
    const member = { id: 'member-1', display_name: 'Участник' };

    store.update('private-group', { member, active: true });

    expect(store.labelFor('private-group', 'other-user')).toBe('Участник Печатает…');
    expect(store.labelFor('another-group', 'other-user')).toBe('');
  });
});
