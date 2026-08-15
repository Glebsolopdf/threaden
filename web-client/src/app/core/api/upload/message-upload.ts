import { HttpEvent, HttpEventType } from '@angular/common/http';
import type { GroupMessage } from '../models';

export interface MessageUploadResult {
  message?: GroupMessage;
  progress: number;
}

export type CompletedMessageUpload = MessageUploadResult & { message: GroupMessage };

export function messageUploadResult(event: HttpEvent<GroupMessage>): MessageUploadResult {
  if (event.type === HttpEventType.UploadProgress) {
    const total = event.total ?? event.loaded;
    return { progress: total ? Math.round((event.loaded / total) * 100) : 0 };
  }
  if (event.type === HttpEventType.Response) return { progress: 100, message: event.body ?? undefined };
  return { progress: 0 };
}

export function isCompletedMessageUpload(result: MessageUploadResult): result is CompletedMessageUpload {
  return Boolean(result.message);
}
