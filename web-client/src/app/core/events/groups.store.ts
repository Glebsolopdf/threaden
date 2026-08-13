import { computed, inject, Injectable, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { ApiService } from '../api/api.service';
import type { GroupInfo, GroupMessage, GroupSystemEvent } from '../api/models';
import { AuthStore } from '../auth/auth.store';

export interface ChatMessageView {
  message: GroupMessage;
  status: 'sending' | 'sent' | 'error';
  animate?: 'incoming' | 'outgoing';
  viewID?: string;
}

export interface SystemMessageView {
  kind: 'system';
  id: string;
  message: GroupMessage;
  body: string;
  animate: 'incoming';
}

export type MessageView = ChatMessageView | SystemMessageView;

@Injectable({ providedIn: 'root' })
export class GroupsStore {
  private readonly api = inject(ApiService);
  private readonly auth = inject(AuthStore);
  private refreshPromise?: Promise<void>;
  private refreshTimer?: number;

  readonly groups = signal<GroupInfo[]>([]);
  readonly current = signal<GroupInfo | null>(null);
  readonly messages = signal<MessageView[]>([]);
  readonly groupsLoading = signal(false);
  readonly groupLoading = signal(false);
  readonly messagesLoading = signal(false);

  readonly currentIsMember = computed(() => {
    const current = this.current();
    return Boolean(current && this.groups().some((group) => group.id === current.id));
  });
  readonly currentIsOwner = computed(() => this.current()?.owner.id === this.auth.user()?.id);

  async refresh(force = false): Promise<void> {
    if (this.refreshPromise && !force) return this.refreshPromise;
    this.groupsLoading.set(this.groups().length === 0);
    this.refreshPromise = firstValueFrom(this.api.groups())
      .then((groups) => {
        this.groups.set(Array.isArray(groups) ? groups : []);
        const current = this.current();
        if (current) {
          const updated = groups.find((group) => group.id === current.id);
          if (updated) this.current.set(updated);
        }
      })
      .finally(() => {
        this.groupsLoading.set(false);
        this.refreshPromise = undefined;
      });
    return this.refreshPromise;
  }

  scheduleRefresh(): void {
    if (this.refreshTimer !== undefined) return;
    this.refreshTimer = window.setTimeout(() => {
      this.refreshTimer = undefined;
      void this.refresh();
    }, 250);
  }

  async openGroup(id: string): Promise<GroupInfo> {
    this.groupLoading.set(true);
    this.messagesLoading.set(true);
    this.current.set(null);
    this.messages.set([]);
    try {
      const [group, messages] = await Promise.all([
        firstValueFrom(this.api.group(id)),
        firstValueFrom(this.api.messages(id, 50)),
      ]);
      this.current.set(group);
      this.messages.set(messages.map((message) => messageView(message)));
      const lastMessage = messages.at(-1);
      if (lastMessage && this.currentIsMember()) this.markRead(id, lastMessage.id);
      return group;
    } finally {
      this.groupLoading.set(false);
      this.messagesLoading.set(false);
    }
  }

  async openInvite(token: string): Promise<GroupInfo> {
    this.groupLoading.set(true);
    this.current.set(null);
    this.messages.set([]);
    try {
      const group = await firstValueFrom(this.api.invite(token));
      this.current.set(group);
      return group;
    } finally {
      this.groupLoading.set(false);
    }
  }

  async joinCurrent(inviteToken = ''): Promise<void> {
    const current = this.current();
    if (!current) return;
    if (current.join_blocked && (!current.join_blocked_until || new Date(current.join_blocked_until).getTime() > Date.now())) return;
    const group = inviteToken
      ? await firstValueFrom(this.api.joinInvite(inviteToken))
      : await firstValueFrom(this.api.joinGroup(current.id));
    this.current.set(group);
    await this.refresh(true);
    await this.openGroup(group.id);
  }

  async sendMessage(body: string, replyToID = ''): Promise<void> {
    const current = this.current();
    const user = this.auth.user();
    const text = body.trim();
    if (!current || !user || !text) return;

    const optimisticId = `pending-${crypto.randomUUID()}`;
    const pending: MessageView = {
      status: 'sending',
      animate: 'outgoing',
      viewID: optimisticId,
      message: {
        id: optimisticId,
        group_id: current.id,
        author: user,
        body: text,
        created_at: new Date().toISOString(),
      },
    };
    this.messages.update((items) => [...items, pending]);

    try {
      const sent = await firstValueFrom(replyToID ? this.api.sendReply(current.id, text, replyToID) : this.api.sendMessage(current.id, text));
      this.messages.update((items) => replaceOptimisticMessage(items, optimisticId, sent));
      this.scheduleRefresh();
    } catch (error) {
      this.messages.update((items) => items.map((item) => !isSystemMessage(item) && item.message.id === optimisticId ? { ...item, status: 'error' } : item));
      throw error;
    }
  }

  async sendMessageWithFiles(body: string, files: File[], replyToID = ''): Promise<void> {
    const current = this.current();
    if (!current || files.length === 0) return;
    const result = await firstValueFrom(this.api.sendMessageWithFiles(current.id, body.trim(), files, replyToID));
    if (result.message) {
      this.messages.update((items) => [...items, messageView(result.message!)]);
      this.scheduleRefresh();
    }
  }

  mergeMessage(message: GroupMessage): void {
    if (this.current()?.id === message.group_id) {
      this.messages.update((items) => mergeIncomingMessage(items, message, this.auth.user()?.id));
      if (message.author.id !== this.auth.user()?.id) this.markRead(message.group_id, message.id);
    }
    this.scheduleRefresh();
  }

  markMessageRead(messageID: string): void {
    this.messages.update((items) => items.map((item) => isSystemMessage(item) || item.message.id !== messageID ? item : { ...item, message: { ...item.message, read: true } }));
  }

  async deleteMessage(messageID: string): Promise<void> {
    const current = this.current();
    if (!current) return;
    await firstValueFrom(this.api.deleteMessage(current.id, messageID));
    this.removeMessage(messageID);
  }

  removeMessage(messageID: string): void {
    this.messages.update((items) => items.filter((item) => isSystemMessage(item) || item.message.id !== messageID));
  }

  private markRead(groupID: string, messageID: string): void {
    void firstValueFrom(this.api.markGroupRead(groupID, messageID)).catch(() => undefined);
  }

  updateProfile(profile: { id: string; display_name: string; avatar?: string }): void {
    const patch = <T extends { id: string; display_name: string; avatar?: string }>(user: T): T =>
      user.id === profile.id ? { ...user, display_name: profile.display_name, avatar: profile.avatar ?? '' } : user;
    this.messages.update((items) => items.map((item) => isSystemMessage(item) ? item : ({ ...item, message: { ...item.message, author: patch(item.message.author) } })));
    this.groups.update((groups) => groups.map((group) => ({
      ...group,
      owner: patch(group.owner),
      last_message: group.last_message ? { ...group.last_message, author: patch(group.last_message.author) } : undefined,
    })));
    this.current.update((group) => group ? ({ ...group, owner: patch(group.owner), last_message: group.last_message ? { ...group.last_message, author: patch(group.last_message.author) } : undefined }) : null);
  }

  async createGroup(name: string, visibility: 'public' | 'private'): Promise<GroupInfo> {
    const group = await firstValueFrom(this.api.createGroup({ name: name.trim(), avatar: '', visibility }));
    await this.refresh(true);
    return group;
  }

  async deleteCurrent(): Promise<void> {
    const current = this.current();
    if (!current) return;
    await firstValueFrom(this.api.deleteGroup(current.id));
    this.current.set(null);
    this.messages.set([]);
    await this.refresh(true);
  }

  async leaveCurrent(): Promise<void> {
    const current = this.current();
    if (!current) return;
    await firstValueFrom(this.api.leaveGroup(current.id));
    this.current.set(null);
    this.messages.set([]);
    await this.refresh(true);
  }
}

export function isSystemMessage(item: MessageView): item is SystemMessageView {
  return 'kind' in item && item.kind === 'system';
}

export function chatMessage(item: MessageView): ChatMessageView | null {
  return isSystemMessage(item) ? null : item;
}

export function systemMessageText(item: MessageView): string {
  if (!isSystemMessage(item)) return '';
  const prefix = systemEventPrefix(item.message.event);
  return prefix ? `${prefix}: ${item.message.author.display_name}` : item.body;
}

function systemEventPrefix(event?: GroupSystemEvent): string {
  return event === 'member_joined' ? 'К чату присоединился участник'
    : event === 'member_left' ? 'Из чата вышел участник'
      : event === 'member_removed' ? 'Из чата исключён участник' : '';
}

function replaceOptimisticMessage(items: MessageView[], optimisticId: string, sent: GroupMessage): MessageView[] {
  if (!items.some((item) => !isSystemMessage(item) && item.message.id === optimisticId)) {
    return items.map((item) => !isSystemMessage(item) && item.message.id === sent.id
      ? messageView(sent, 'outgoing')
      : item);
  }
  return items
    .filter((item) => isSystemMessage(item) || item.message.id !== sent.id)
    .map((item) => !isSystemMessage(item) && item.message.id === optimisticId ? messageView(sent, 'outgoing') : item);
}

function mergeIncomingMessage(items: MessageView[], message: GroupMessage, currentUserID?: string): MessageView[] {
  const pending = items.find((item) => !isSystemMessage(item) && item.status === 'sending' &&
    item.message.group_id === message.group_id && item.message.author.id === currentUserID && item.message.body === message.body);
  if (pending) {
    return items.map((item) => item === pending ? { ...item, message, status: 'sent', animate: 'outgoing' } : item);
  }
  if (items.some((item) => !isSystemMessage(item) && item.message.id === message.id)) return items;
  return [...items, messageView(message, 'incoming')];
}

function messageView(message: GroupMessage, animate?: 'incoming' | 'outgoing'): MessageView {
  if (message.kind === 'system') return { kind: 'system', id: message.id, message, body: message.body, animate: 'incoming' };
  return { message, status: 'sent', animate };
}
