import { TestBed } from '@angular/core/testing';
import { throwError } from 'rxjs';
import { ApiService } from '../api/api.service';
import { TypingStore } from './typing.store';

describe('TypingStore', () => {
  it('keeps the queue usable after a failed notify', async () => {
    let calls = 0;
    TestBed.configureTestingModule({
      providers: [
        TypingStore,
        {
          provide: ApiService,
          useValue: { setTyping: () => { calls++; return throwError(() => new Error('boom')); } },
        },
      ],
    });
    const store = TestBed.inject(TypingStore);

    store.notify('group-1', true);
    await new Promise((resolve) => setTimeout(resolve, 0));
    store.notify('group-1', true);
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(calls).toBe(2);
  });
});
