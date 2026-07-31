import { computed, inject, Injectable, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import {
  ConnectionState,
  type Participant,
  type RemoteParticipant,
  type RemoteTrack,
  type RemoteTrackPublication,
  Room,
  RoomEvent,
  Track,
} from 'livekit-client';
import { ApiService } from '../api/api.service';
import type { Member } from '../api/models';
import { PreferencesService } from '../preferences/preferences.service';
import { VoiceActivityDetector } from './activity-detector';

export type VoiceStatus = 'connecting' | 'connected' | 'reconnecting' | 'disconnected' | 'error';
export type RoomKind = 'group' | 'temporary';

export interface ActiveVoiceRoom {
  id: string;
  kind: RoomKind;
  title: string;
  groupId?: string;
}

export interface VoiceParticipant {
  identity: string;
  name: string;
  avatar: string;
  isLocal: boolean;
  isSpeaking: boolean;
  isMicrophoneMuted: boolean;
  pingMs?: number;
}

const ACTIVE_ROOM_KEY = 'voice_rooms_active_room';

const VOICE_STATUS_LABELS: Readonly<Record<VoiceStatus, string>> = {
  connecting: 'Подключение…',
  connected: 'Подключено',
  reconnecting: 'Переподключение…',
  disconnected: 'Отключено',
  error: 'Ошибка подключения',
};

const VOICE_DETAIL_LABELS: Readonly<Record<Exclude<VoiceStatus, 'connected'>, string>> = {
  connecting: 'Готовим аудиосоединение',
  reconnecting: 'Пытаемся восстановить соединение',
  disconnected: 'Соединение с комнатой завершено',
  error: 'Не удалось подключиться к комнате',
};

@Injectable({ providedIn: 'root' })
export class VoiceService {
  private readonly api = inject(ApiService);
  private readonly preferences = inject(PreferencesService);
  private readonly room = new Room();
  private readonly activity = new VoiceActivityDetector();
  private readonly audioElements = new Map<string, HTMLMediaElement[]>();
  private readonly roster = signal(new Map<string, { name: string; avatar: string }>());
  private readonly audioContainer = this.createAudioContainer();
  private pingTimer?: number;
  private connecting = false;

  readonly activeRoom = signal<ActiveVoiceRoom | null>(null);
  readonly status = signal<VoiceStatus>('disconnected');
  readonly participants = signal<VoiceParticipant[]>([]);
  readonly minimized = signal(false);
  readonly audioBlocked = signal(false);
  readonly microphoneEnabled = signal(false);
  readonly inputDevices = signal<MediaDeviceInfo[]>([]);
  readonly outputDevices = signal<MediaDeviceInfo[]>([]);
  readonly deviceMenuOpen = signal(false);

  readonly isActive = computed(() => Boolean(this.activeRoom()));
  readonly statusLabel = computed(() => VOICE_STATUS_LABELS[this.status()]);
  readonly detailLabel = computed(() => {
    const status = this.status();
    if (status === 'connected') {
      const ping = this.participants().find((participant) => participant.isLocal)?.pingMs;
      return ping === undefined ? 'Пинг вычисляется…' : `Пинг ${ping} мс`;
    }
    return VOICE_DETAIL_LABELS[status];
  });

  async openTemporary(code: string, fullscreen = true): Promise<void> {
    const normalized = code.trim().toUpperCase();
    const join = await firstValueFrom(this.api.joinRoom(normalized));
    await this.open({ id: join.room_code, kind: 'temporary', title: `Комната ${join.room_code}` }, join.livekit_url, join.access_token, fullscreen);
  }

  async createTemporary(): Promise<string> {
    const room = await firstValueFrom(this.api.createRoom());
    return room.code;
  }

  async openGroup(id: string, title: string, groupId?: string, fullscreen = true): Promise<void> {
    const join = await firstValueFrom(this.api.joinGroupVoice(id));
    await this.open({ id: join.voice_room_id, kind: 'group', title, groupId }, join.livekit_url, join.access_token, fullscreen);
  }

  async restoreSaved(fullscreen = false): Promise<boolean> {
    const stored = this.loadStoredRoom();
    if (!stored) return false;
    try {
      if (stored.kind === 'group') await this.openGroup(stored.id, stored.title, stored.groupId, fullscreen);
      else await this.openTemporary(stored.id, fullscreen);
      return true;
    } catch {
      this.clearStoredRoom();
      return false;
    }
  }

  minimize(): void { if (this.activeRoom()) this.minimized.set(true); }
  restore(): void { if (this.activeRoom()) this.minimized.set(false); }

  async leave(): Promise<void> {
    const active = this.activeRoom();
    try {
      if (active?.kind === 'group') await firstValueFrom(this.api.leaveGroupVoice(active.id));
      if (active?.kind === 'temporary') await firstValueFrom(this.api.leaveRoom(active.id));
    } finally {
      await this.disconnect();
      this.clearStoredRoom();
      this.activeRoom.set(null);
      this.minimized.set(false);
      this.participants.set([]);
    }
  }

  async shutdown(): Promise<void> {
    await this.disconnect();
    this.clearStoredRoom();
    this.activeRoom.set(null);
    this.minimized.set(false);
    this.participants.set([]);
  }

  async toggleMicrophone(): Promise<void> {
    await this.setMicrophoneEnabled(!this.microphoneEnabled());
  }

  async setMicrophoneEnabled(enabled: boolean): Promise<void> {
    if (this.room.state !== ConnectionState.Connected) throw new Error('Сначала подключитесь к комнате');
    await this.room.localParticipant.setMicrophoneEnabled(enabled);
    this.microphoneEnabled.set(enabled);
    this.preferences.updateAudio({ microphoneEnabled: enabled });
    this.emitParticipants();
  }

  updateProfile(profile: { id: string; display_name: string; avatar?: string }): void {
    this.roster.update((items) => {
      const next = new Map(items);
      next.set(profile.id, { name: profile.display_name, avatar: profile.avatar ?? '' });
      return next;
    });
    this.emitParticipants();
  }

  async startAudio(): Promise<void> {
    await this.room.startAudio();
    this.audioBlocked.set(false);
  }

  async loadDevices(): Promise<void> {
    const [inputs, outputs] = await Promise.all([
      Room.getLocalDevices('audioinput', false),
      Room.getLocalDevices('audiooutput', false),
    ]);
    this.inputDevices.set(inputs);
    this.outputDevices.set(outputs);
  }

  async selectInputDevice(deviceId: string): Promise<void> {
    if (deviceId) await this.room.switchActiveDevice('audioinput', deviceId);
    this.preferences.updateAudio({ inputDeviceId: deviceId });
  }

  async selectOutputDevice(deviceId: string): Promise<void> {
    if (deviceId) await this.room.switchActiveDevice('audiooutput', deviceId);
    this.preferences.updateAudio({ outputDeviceId: deviceId });
  }

  setOutputVolume(percent: number): void {
    const value = Math.min(100, Math.max(0, percent));
    for (const elements of this.audioElements.values()) for (const element of elements) element.volume = value / 100;
    this.preferences.updateAudio({ outputVolume: value });
  }

  async testMicrophone(): Promise<void> {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    window.setTimeout(() => stream.getTracks().forEach((track) => track.stop()), 1500);
  }

  private async open(active: ActiveVoiceRoom, url: string, token: string, fullscreen: boolean): Promise<void> {
    if (this.connecting || this.room.state !== ConnectionState.Disconnected) {
      if (this.activeRoom()?.id === active.id) {
        this.minimized.set(!fullscreen);
        return;
      }
      await this.disconnect();
    }
    this.connecting = true;
    this.activeRoom.set(active);
    this.minimized.set(!fullscreen);
    this.status.set('connecting');
    this.bindListeners();
    try {
      await this.room.connect(url, token, { autoSubscribe: true });
    } catch (error) {
      this.status.set('error');
      await this.disconnect();
      throw error;
    } finally {
      this.connecting = false;
    }
    this.status.set('connected');
    this.audioBlocked.set(!this.room.canPlaybackAudio);
    this.saveStoredRoom(active);
    this.startPingUpdates();
    this.emitParticipants();
    void this.loadRoster(active).then(() => this.emitParticipants()).catch(() => undefined);
    void this.applyPreferences().catch(() => undefined);
  }

  private async disconnect(): Promise<void> {
    for (const publication of this.room.localParticipant.trackPublications.values()) publication.track?.stop();
    this.clearRemoteAudio();
    try { await this.room.disconnect(); } finally {
      this.activity.reset();
      this.stopPingUpdates();
      this.removeListeners();
      this.status.set('disconnected');
      this.microphoneEnabled.set(false);
      this.connecting = false;
    }
  }

  private async loadRoster(active: ActiveVoiceRoom): Promise<void> {
    let members: Member[] = [];
    if (active.kind === 'temporary') {
      members = (await firstValueFrom(this.api.getRoom(active.id))).members;
    } else {
      let groupId = active.groupId;
      if (!groupId) {
        const groups = await firstValueFrom(this.api.groups());
        groupId = groups.find((group) => (group.voice_rooms ?? []).some((room) => room.id === active.id))?.id;
      }
      if (groupId) members = (await firstValueFrom(this.api.groupProfile(groupId))).members;
    }
    this.roster.set(new Map(members.map((member) => [member.id, { name: member.display_name, avatar: member.avatar ?? '' }])));
  }

  private async applyPreferences(): Promise<void> {
    const prefs = this.preferences.audio();
    this.setOutputVolume(prefs.outputVolume);
    if (prefs.inputDeviceId) await this.room.switchActiveDevice('audioinput', prefs.inputDeviceId).catch(() => undefined);
    if (prefs.outputDeviceId) await this.room.switchActiveDevice('audiooutput', prefs.outputDeviceId).catch(() => undefined);
    if (prefs.microphoneEnabled) await this.setMicrophoneEnabled(true).catch(() => undefined);
  }

  private bindListeners(): void {
    this.room
      .on(RoomEvent.Reconnecting, this.handleReconnecting)
      .on(RoomEvent.Reconnected, this.handleReconnected)
      .on(RoomEvent.Disconnected, this.handleDisconnected)
      .on(RoomEvent.ParticipantConnected, this.handleRosterChanged)
      .on(RoomEvent.ParticipantDisconnected, this.handleRosterChanged)
      .on(RoomEvent.ActiveSpeakersChanged, this.emitParticipants)
      .on(RoomEvent.TrackMuted, this.emitParticipants)
      .on(RoomEvent.TrackUnmuted, this.emitParticipants)
      .on(RoomEvent.TrackSubscribed, this.handleTrackSubscribed)
      .on(RoomEvent.TrackUnsubscribed, this.handleTrackUnsubscribed)
      .on(RoomEvent.AudioPlaybackStatusChanged, this.handleAudioPlayback);
  }

  private removeListeners(): void {
    this.room
      .off(RoomEvent.Reconnecting, this.handleReconnecting)
      .off(RoomEvent.Reconnected, this.handleReconnected)
      .off(RoomEvent.Disconnected, this.handleDisconnected)
      .off(RoomEvent.ParticipantConnected, this.handleRosterChanged)
      .off(RoomEvent.ParticipantDisconnected, this.handleRosterChanged)
      .off(RoomEvent.ActiveSpeakersChanged, this.emitParticipants)
      .off(RoomEvent.TrackMuted, this.emitParticipants)
      .off(RoomEvent.TrackUnmuted, this.emitParticipants)
      .off(RoomEvent.TrackSubscribed, this.handleTrackSubscribed)
      .off(RoomEvent.TrackUnsubscribed, this.handleTrackUnsubscribed)
      .off(RoomEvent.AudioPlaybackStatusChanged, this.handleAudioPlayback);
  }

  private readonly handleReconnecting = (): void => this.status.set('reconnecting');
  private readonly handleReconnected = (): void => { this.status.set('connected'); this.emitParticipants(); };
  private readonly handleDisconnected = (): void => { this.clearRemoteAudio(); this.status.set('disconnected'); this.emitParticipants(); };
  private readonly handleRosterChanged = (): void => {
    const active = this.activeRoom();
    if (active) void this.loadRoster(active).finally(() => this.emitParticipants());
  };
  private readonly handleAudioPlayback = (): void => this.audioBlocked.set(!this.room.canPlaybackAudio);

  private readonly handleTrackSubscribed = (
    track: RemoteTrack,
    publication: RemoteTrackPublication,
    _participant: RemoteParticipant,
  ): void => {
    if (track.kind !== Track.Kind.Audio) return;
    const element = track.attach();
    element.autoplay = true;
    element.volume = this.preferences.audio().outputVolume / 100;
    this.audioContainer.append(element);
    this.audioElements.set(publication.trackSid, [element]);
  };

  private readonly handleTrackUnsubscribed = (
    track: RemoteTrack,
    publication: RemoteTrackPublication,
    _participant: RemoteParticipant,
  ): void => {
    for (const element of track.detach()) element.remove();
    this.audioElements.delete(publication.trackSid);
  };

  private readonly emitParticipants = (): void => {
    const active = new Set(this.room.activeSpeakers.map((participant) => participant.identity));
    const pingMs = this.signalPingMs();
    const now = performance.now();
    const participants: Participant[] = [];
    if (this.room.state !== ConnectionState.Disconnected && this.room.localParticipant.identity) participants.push(this.room.localParticipant);
    participants.push(...this.room.remoteParticipants.values());
    const identities = new Set(participants.map((participant) => participant.identity));
    this.activity.prune(identities);
    const roster = this.roster();
    this.participants.set(participants.map((participant) => {
      const muted = this.isMicrophoneMuted(participant);
      const profile = roster.get(participant.identity);
      return {
        identity: participant.identity,
        name: profile?.name || participant.name || participant.identity,
        avatar: profile?.avatar || '',
        isLocal: participant === this.room.localParticipant,
        isSpeaking: this.activity.update(participant.identity, Number((participant as Participant & { audioLevel?: number }).audioLevel ?? 0), active.has(participant.identity), muted, now),
        isMicrophoneMuted: muted,
        pingMs,
      };
    }));
  };

  private isMicrophoneMuted(participant: Participant): boolean {
    const publication = participant.getTrackPublication(Track.Source.Microphone);
    return !publication || publication.isMuted;
  }

  private signalPingMs(): number | undefined {
    const rtt = this.room.engine.client.rtt;
    if (rtt <= 0) return undefined;
    return Math.round(rtt < 1 ? rtt * 1000 : rtt);
  }

  private startPingUpdates(): void {
    this.stopPingUpdates();
    this.pingTimer = window.setInterval(this.emitParticipants, 180);
  }

  private stopPingUpdates(): void {
    if (this.pingTimer === undefined) return;
    window.clearInterval(this.pingTimer);
    this.pingTimer = undefined;
  }

  private clearRemoteAudio(): void {
    for (const participant of this.room.remoteParticipants.values()) {
      for (const publication of participant.trackPublications.values()) publication.track?.detach().forEach((element) => element.remove());
    }
    for (const elements of this.audioElements.values()) for (const element of elements) element.remove();
    this.audioElements.clear();
  }

  private createAudioContainer(): HTMLElement {
    const container = document.createElement('div');
    container.hidden = true;
    container.id = 'audio-container';
    document.body.append(container);
    return container;
  }

  private saveStoredRoom(room: ActiveVoiceRoom): void {
    try { localStorage.setItem(ACTIVE_ROOM_KEY, JSON.stringify(room)); } catch { /* optional */ }
  }

  private loadStoredRoom(): ActiveVoiceRoom | null {
    try {
      const raw = localStorage.getItem(ACTIVE_ROOM_KEY);
      if (!raw) return null;
      const value = JSON.parse(raw) as Partial<ActiveVoiceRoom>;
      if (!value.id || !value.title) return null;
      return { id: value.id, title: value.title, kind: value.kind === 'group' ? 'group' : 'temporary', groupId: value.groupId };
    } catch { return null; }
  }

  clearStoredRoom(): void {
    try { localStorage.removeItem(ACTIVE_ROOM_KEY); } catch { /* optional */ }
  }
}
