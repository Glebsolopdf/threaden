import { HttpEvent, HttpEventType, HttpResponse } from '@angular/common/http';
import { describe, expect, it } from 'vitest';
import { filter, firstValueFrom, from, map } from 'rxjs';
import type { GroupMessage } from '../models';
import { isCompletedMessageUpload, messageUploadResult } from './message-upload';

const message = { id: 'message-1', body: 'Фото' } as GroupMessage;

describe('message upload events', () => {
  it('waits for the response instead of resolving on upload progress', async () => {
    const events: HttpEvent<GroupMessage>[] = [
      { type: HttpEventType.UploadProgress, loaded: 1, total: 2 },
      new HttpResponse({ body: message }),
    ];
    const result = await firstValueFrom(from(events).pipe(map(messageUploadResult), filter(isCompletedMessageUpload)));

    expect(result.message).toBe(message);
  });
});
