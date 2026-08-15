import { HttpClient, HttpEvent, HttpEventType, HttpParams } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { filter, map, Observable } from 'rxjs';
import type {
  GroupInfo,
  GroupMessage,
  GroupProfile,
  GroupVoiceJoin,
  GroupVoiceRoom,
  JoinResponse,
  RoomInfo,
  User,
  SecuritySession,
  AccountQuotas,
  PendingAttachmentDeletion,
  WelcomeStats,
} from './models';
import { isCompletedMessageUpload, messageUploadResult, type MessageUploadResult } from './upload/message-upload';

export interface UploadResult {
  user?: User;
  progress: number;
}

@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly http = inject(HttpClient);

  register(email: string, password: string): Observable<User> {
    return this.http.post<User>('/v1/auth/register', { email, password });
  }

  login(email: string, password: string): Observable<User> {
    return this.http.post<User>('/v1/auth/login', { email, password });
  }

  logout(): Observable<void> {
    return this.http.delete<void>('/v1/auth/logout');
  }

  getMe(): Observable<User> {
    return this.http.get<User>('/v1/me');
  }

  welcome(): Observable<WelcomeStats> {
    return this.http.get<WelcomeStats>('/v1/welcome');
  }

  updateProfile(displayName: string, avatar?: File): Observable<UploadResult> {
    const body = new FormData();
    body.set('display_name', displayName);
    if (avatar) body.set('avatar', avatar);
    return this.http.patch<User>('/v1/me', body, { observe: 'events', reportProgress: true }).pipe(
      map((event: HttpEvent<User>): UploadResult => {
        if (event.type === HttpEventType.UploadProgress) {
          const total = event.total ?? event.loaded;
          return { progress: total ? Math.round((event.loaded / total) * 100) : 0 };
        }
        if (event.type === HttpEventType.Response) return { progress: 100, user: event.body ?? undefined };
        return { progress: 0 };
      }),
      filter((result) => result.progress > 0 || Boolean(result.user)),
    );
  }

  deleteAvatar(): Observable<User> { return this.http.delete<User>('/v1/me/avatar'); }
  deleteProfile(): Observable<void> { return this.http.delete<void>('/v1/me'); }
  changePassword(password: string): Observable<void> { return this.http.patch<void>('/v1/me/password', { password }); }
  securitySessions(): Observable<SecuritySession[]> { return this.http.get<SecuritySession[]>('/v1/me/sessions'); }
  revokeSecuritySession(id: string): Observable<void> { return this.http.delete<void>(`/v1/me/sessions/${encodeURIComponent(id)}`); }

  quotas(): Observable<AccountQuotas> { return this.http.get<AccountQuotas>('/v1/account/quotas'); }
  scheduleAttachmentDeletion(): Observable<PendingAttachmentDeletion> { return this.http.post<PendingAttachmentDeletion>('/v1/account/attachments/delete-all', {}); }
  cancelAttachmentDeletion(): Observable<void> { return this.http.delete<void>('/v1/account/attachments/delete-all'); }

  createRoom(): Observable<RoomInfo> { return this.http.post<RoomInfo>('/v1/rooms', {}); }
  getRoom(code: string): Observable<RoomInfo> { return this.http.get<RoomInfo>(`/v1/rooms/${encodeURIComponent(code)}`); }
  joinRoom(code: string): Observable<JoinResponse> { return this.http.post<JoinResponse>(`/v1/rooms/${encodeURIComponent(code)}/join`, {}); }
  leaveRoom(code: string): Observable<void> { return this.http.delete<void>(`/v1/rooms/${encodeURIComponent(code)}/members/me`); }
  deleteRoom(code: string): Observable<void> { return this.http.delete<void>(`/v1/rooms/${encodeURIComponent(code)}`); }

  groups(): Observable<GroupInfo[]> { return this.http.get<GroupInfo[]>('/v1/groups'); }

  discover(query = '', limit = 20, offset = 0): Observable<GroupInfo[]> {
    const params = new HttpParams().set('q', query).set('limit', limit).set('offset', offset);
    return this.http.get<GroupInfo[]>('/v1/discover/groups', { params });
  }

  group(id: string): Observable<GroupInfo> { return this.http.get<GroupInfo>(`/v1/groups/${encodeURIComponent(id)}`); }
  createGroup(body: { name: string; avatar: string; visibility: 'public' | 'private' }): Observable<GroupInfo> {
    return this.http.post<GroupInfo>('/v1/groups', body);
  }
  groupProfile(id: string): Observable<GroupProfile> { return this.http.get<GroupProfile>(`/v1/groups/${encodeURIComponent(id)}/profile`); }
  leaveGroup(id: string): Observable<void> { return this.http.delete<void>(`/v1/groups/${encodeURIComponent(id)}/members/me`); }
  removeGroupMember(id: string, memberId: string): Observable<GroupProfile> {
    return this.http.delete<GroupProfile>(`/v1/groups/${encodeURIComponent(id)}/members/${encodeURIComponent(memberId)}`);
  }
  deleteGroup(id: string): Observable<void> { return this.http.delete<void>(`/v1/groups/${encodeURIComponent(id)}`); }
  joinGroup(id: string): Observable<GroupInfo> { return this.http.post<GroupInfo>(`/v1/groups/${encodeURIComponent(id)}/members`, {}); }
  invite(token: string): Observable<GroupInfo> { return this.http.get<GroupInfo>(`/v1/invites/${encodeURIComponent(token)}`); }
  joinInvite(token: string): Observable<GroupInfo> { return this.http.post<GroupInfo>(`/v1/invites/${encodeURIComponent(token)}/join`, {}); }

  messages(id: string, limit = 30): Observable<GroupMessage[]> {
    return this.http.get<GroupMessage[]>(`/v1/groups/${encodeURIComponent(id)}/messages`, { params: { limit } });
  }
  sendMessage(id: string, body: string): Observable<GroupMessage> {
    return this.http.post<GroupMessage>(`/v1/groups/${encodeURIComponent(id)}/messages`, { body });
  }
  sendReply(id: string, body: string, replyToID: string): Observable<GroupMessage> {
    return this.http.post<GroupMessage>(`/v1/groups/${encodeURIComponent(id)}/messages`, { body, reply_to_id: replyToID });
  }
  sendMessageWithFiles(id: string, body: string, files: File[], replyToID = ''): Observable<MessageUploadResult> {
    const form = new FormData();
    form.set('body', body);
    if (replyToID) form.set('reply_to_id', replyToID);
    for (const file of files) form.append('files[]', file, file.name);
    return this.http.post<GroupMessage>(`/v1/groups/${encodeURIComponent(id)}/messages`, form, { observe: 'events', reportProgress: true }).pipe(
      map(messageUploadResult),
      filter(isCompletedMessageUpload),
    );
  }
  deleteMessage(groupID: string, messageID: string): Observable<void> {
    return this.http.delete<void>(`/v1/groups/${encodeURIComponent(groupID)}/messages/${encodeURIComponent(messageID)}`);
  }
  markGroupRead(id: string, messageID: string): Observable<void> {
    return this.http.post<void>(`/v1/groups/${encodeURIComponent(id)}/read`, { message_id: messageID });
  }
  setTyping(id: string, active: boolean): Observable<void> {
    return this.http.post<void>(`/v1/groups/${encodeURIComponent(id)}/typing`, { active });
  }

  createGroupVoice(id: string, name: string): Observable<GroupVoiceRoom> {
    return this.http.post<GroupVoiceRoom>(`/v1/groups/${encodeURIComponent(id)}/voice-rooms`, { name });
  }
  joinGroupVoice(id: string): Observable<GroupVoiceJoin> {
    return this.http.post<GroupVoiceJoin>(`/v1/group-voice-rooms/${encodeURIComponent(id)}/join`, {});
  }
  leaveGroupVoice(id: string): Observable<void> {
    return this.http.delete<void>(`/v1/group-voice-rooms/${encodeURIComponent(id)}/members/me`);
  }
  deleteGroupVoice(id: string): Observable<void> {
    return this.http.delete<void>(`/v1/group-voice-rooms/${encodeURIComponent(id)}`);
  }
}
