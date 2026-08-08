import { computed, inject, Injectable, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import {
  ConnectionState,
  Room,
} from 'livekit-client';
import { ApiService } from '../api/api.service';
import type { Member } from '../api/models';
import { PreferencesService } from '../preferences/preferences.service';
import { VoiceActivityDetector } from './activity-detector';
import { ScreenShareController } from './screen-share/screen-share.controller';
import type { ScreenShareMode } from './screen-share/screen-share.models';
import { VoiceAudio } from './runtime/audio';
import { bindVoiceRoomEvents, removeVoiceRoomEvents, type VoiceRoomHandlers } from './runtime/events';
import { VoicePing } from './runtime/ping';
import { testMicrophone } from './runtime/browser/microphone';
import { buildParticipants } from './runtime/participants';
import { clearRoom, loadRoom, saveRoom } from './runtime/browser/storage';
import {
  type ActiveVoiceRoom,
  type VoiceParticipant,
  type VoiceStatus,
  VOICE_DETAIL_LABELS,
  VOICE_STATUS_LABELS,
} from './runtime/models';

export type { ActiveVoiceRoom, VoiceParticipant, VoiceStatus } from './runtime/models';

@Injectable({ providedIn: 'root' })
export class VoiceService {
  private readonly api = inject(ApiService);
  private readonly preferences = inject(PreferencesService);
  private readonly room = new Room({
    adaptiveStream: true,
    dynacast: true,
    publishDefaults: { videoCodec: 'vp8', simulcast: true },
  });
  private readonly activity = new VoiceActivityDetector();
  private readonly roster = signal(new Map<string, { name: string; avatar: string }>());
  private connecting = false;
  readonly screenShare = new ScreenShareController(this.room, (identity, fallback) => this.roster().get(identity)?.name || fallback);
  private readonly audio = new VoiceAudio(this.room, this.screenShare, this.preferences);
  private readonly ping = new VoicePing(this.room, () => this.emitParticipants());

  readonly activeRoom = signal<ActiveVoiceRoom | null>(null);
  readonly status = signal<VoiceStatus>('disconnected');
  readonly participants = signal<VoiceParticipant[]>([]);
  readonly minimized = signal(false);
  readonly audioBlocked = signal(false);
  readonly microphoneEnabled = signal(false);
  readonly inputDevices = signal<MediaDeviceInfo[]>([]);
  readonly outputDevices = signal<MediaDeviceInfo[]>([]);
  readonly deviceMenuOpen = signal(false);
  readonly screenShareAudioMuted = signal(false);

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
    const stored = loadRoom();
    if (!stored) return false;
    try {
      if (stored.kind === 'group') await this.openGroup(stored.id, stored.title, stored.groupId, fullscreen);
      else await this.openTemporary(stored.id, fullscreen);
      return true;
    } catch {
      clearRoom();
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
      clearRoom();
      this.activeRoom.set(null);
      this.minimized.set(false);
      this.participants.set([]);
    }
  }

  async shutdown(): Promise<void> {
    await this.disconnect();
      clearRoom();
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

  async startScreenShare(mode: ScreenShareMode): Promise<void> { await this.screenShare.start(mode); }
  async stopScreenShare(): Promise<void> { await this.screenShare.stop(); }
  async changeScreenShareMode(mode: ScreenShareMode): Promise<void> { await this.screenShare.changeMode(mode); }

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
    this.audio.setOutputVolume(value);
    this.preferences.updateAudio({ outputVolume: value });
  }

  toggleScreenShareAudio(): void {
    const muted = !this.screenShareAudioMuted();
    this.audio.toggleScreenShareAudio(muted);
    this.screenShareAudioMuted.set(muted);
  }

  setScreenShareAudioMuted(identity: string, muted: boolean): void {
    this.audio.setScreenShareAudioMuted(identity, muted);
  }

  setScreenShareAudioVolume(identity: string, percent: number): void {
    this.audio.setScreenShareAudioVolume(identity, percent);
  }

  async testMicrophone(): Promise<void> { await testMicrophone(); }

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
    saveRoom(active);
    this.ping.start();
    this.emitParticipants();
    void this.loadRoster(active).then(() => this.emitParticipants()).catch(() => undefined);
    void this.applyPreferences().catch(() => undefined);
  }

  private async disconnect(): Promise<void> {
    await this.screenShare.destroy();
    for (const publication of this.room.localParticipant.trackPublications.values()) publication.track?.stop();
    this.audio.clearRemote();
    this.screenShareAudioMuted.set(false);
    try { await this.room.disconnect(); } finally {
      this.activity.reset();
      this.ping.stop();
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
    bindVoiceRoomEvents(this.room, this.eventHandlers());
  }

  private removeListeners(): void {
    removeVoiceRoomEvents(this.room, this.eventHandlers());
  }

  private eventHandlers(): VoiceRoomHandlers {
    return {
      reconnecting: this.handleReconnecting,
      reconnected: this.handleReconnected,
      disconnected: this.handleDisconnected,
      rosterChanged: this.handleRosterChanged,
      participantsChanged: this.emitParticipants,
      trackSubscribed: this.audio.handleTrackSubscribed,
      trackUnsubscribed: this.audio.handleTrackUnsubscribed,
      audioPlayback: this.handleAudioPlayback,
    };
  }

  private readonly handleReconnecting = (): void => this.status.set('reconnecting');
  private readonly handleReconnected = (): void => { this.status.set('connected'); this.emitParticipants(); };
  private readonly handleDisconnected = (): void => { this.audio.clearRemote(); this.screenShare.clearRemote(); this.screenShareAudioMuted.set(false); this.status.set('disconnected'); this.emitParticipants(); };
  private readonly handleRosterChanged = (): void => {
    const active = this.activeRoom();
    if (active) void this.loadRoster(active).finally(() => this.emitParticipants());
  };
  private readonly handleAudioPlayback = (): void => this.audioBlocked.set(!this.room.canPlaybackAudio);

  private readonly emitParticipants = (): void => {
    const pingMs = this.ping.value();
    const now = performance.now();
    const roster = this.roster();
    this.participants.set(buildParticipants(this.room, roster, this.activity, pingMs, now));
  };

  clearStoredRoom(): void { clearRoom(); }
}
