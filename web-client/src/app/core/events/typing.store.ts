import { inject, Injectable, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { ApiService } from '../api/api.service';
import type { GroupTypingEvent } from '../api/models';

interface TypingMember {
  member: GroupTypingEvent['member'];
  expiresAt: number;
}

@Injectable({ providedIn: 'root' })
export class TypingStore {
  private readonly api = inject(ApiService);
  private readonly active = signal(new Map<string, Map<string, TypingMember>>());
  private readonly timers = new Map<string, number>();
  private readonly localActive = new Set<string>();
  private readonly stopTimers = new Map<string, number>();
  private readonly requests = new Map<string, Promise<void>>();

  update(groupID: string, event: GroupTypingEvent): void {
    const groups = new Map(this.active());
    const members = new Map(groups.get(groupID));
    if (event.active) {
      members.set(event.member.id, { member: event.member, expiresAt: Date.now() + 2500 });
      this.scheduleExpiry(groupID, event.member.id);
    } else {
      members.delete(event.member.id);
    }
    if (members.size) groups.set(groupID, members);
    else groups.delete(groupID);
    this.active.set(groups);
  }

  labelFor(groupID: string, ownID?: string): string {
    const members = this.active().get(groupID);
    const names = [...(members?.values() ?? [])]
      .filter(({ member }) => member.id !== ownID && Date.now() < (members?.get(member.id)?.expiresAt ?? 0))
      .map(({ member }) => member.display_name);
    if (!names.length) return '';
    return `${names.slice(0, 2).join(', ')} Печатает…`;
  }

  notify(groupID: string, active: boolean): void {
    if (!groupID) return;
    const oldTimer = this.stopTimers.get(groupID);
    if (oldTimer !== undefined) window.clearTimeout(oldTimer);
    this.stopTimers.delete(groupID);
    if (active) {
      this.stopTimers.set(groupID, window.setTimeout(() => this.notify(groupID, false), 1500));
      if (this.localActive.has(groupID)) return;
      this.localActive.add(groupID);
    } else if (!this.localActive.delete(groupID)) {
      return;
    }
    const previous = this.requests.get(groupID);
    const request = (previous ?? Promise.resolve())
      .catch(() => undefined)
      .then(() => firstValueFrom(this.api.setTyping(groupID, active)))
      .then(
        () => undefined,
        () => {
          if (active && this.requests.get(groupID) === request) this.localActive.delete(groupID);
        },
      );
    this.requests.set(groupID, request);
    void request.finally(() => {
      if (this.requests.get(groupID) === request) this.requests.delete(groupID);
    });
  }

  private scheduleExpiry(groupID: string, memberID: string): void {
    const key = `${groupID}:${memberID}`;
    const oldTimer = this.timers.get(key);
    if (oldTimer !== undefined) window.clearTimeout(oldTimer);
    const timer = window.setTimeout(() => {
      const current = this.active().get(groupID)?.get(memberID);
      if (current && current.expiresAt <= Date.now()) this.update(groupID, { active: false, member: current.member });
    }, 2600);
    this.timers.set(key, timer);
  }
}
