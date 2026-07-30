import {
  ConnectionState,
  type Participant,
  type RemoteParticipant,
  type RemoteTrack,
  type RemoteTrackPublication,
  Room,
  RoomEvent,
  Track,
} from "livekit-client";
import { VoiceActivityDetector } from "./voice-room/activity/detector";

export type VoiceStatus =
  | "connecting"
  | "connected"
  | "reconnecting"
  | "disconnected"
  | "error";

export interface VoiceParticipant {
  identity: string;
  name: string;
  avatar: string;
  isLocal: boolean;
  isSpeaking: boolean;
  isMicrophoneMuted: boolean;
  pingMs?: number;
}

export interface VoiceCallbacks {
  onStatus: (status: VoiceStatus) => void;
  onParticipants: (participants: VoiceParticipant[]) => void;
  onRosterChanged: () => void;
  onAudioBlocked: (blocked: boolean) => void;
}

export class VoiceClient {
  private readonly room = new Room();
  private callbacks?: VoiceCallbacks;
  private connecting = false;
  private pingTimer?: number;
  private outputVolume = 1;
  private readonly activity = new VoiceActivityDetector();
  private readonly audioElements = new Map<string, HTMLMediaElement[]>();

  constructor(private readonly audioContainer: HTMLElement) {}

  async connect(url: string, token: string, callbacks: VoiceCallbacks): Promise<void> {
    if (this.connecting || this.room.state !== ConnectionState.Disconnected) {
      throw new Error("Подключение уже выполняется");
    }
    this.connecting = true;
    this.callbacks = callbacks;
    this.bindListeners();
    callbacks.onStatus("connecting");

    try {
      await this.room.connect(url, token, { autoSubscribe: true });
      callbacks.onStatus("connected");
      callbacks.onAudioBlocked(!this.room.canPlaybackAudio);
      this.startPingUpdates();
      this.emitParticipants();
    } catch (error) {
      callbacks.onStatus("error");
      this.removeListeners();
      this.callbacks = undefined;
      throw error;
    } finally {
      this.connecting = false;
    }
  }

  async setMicrophoneEnabled(enabled: boolean): Promise<void> {
    if (this.room.state !== ConnectionState.Connected) {
      throw new Error("Сначала подключитесь к комнате");
    }
    await this.room.localParticipant.setMicrophoneEnabled(enabled);
  }

  async startAudio(): Promise<void> {
    await this.room.startAudio();
    this.callbacks?.onAudioBlocked(false);
  }

  async inputDevices(): Promise<MediaDeviceInfo[]> {
    return Room.getLocalDevices("audioinput", false);
  }

  async outputDevices(): Promise<MediaDeviceInfo[]> {
    return Room.getLocalDevices("audiooutput", false);
  }

  async selectInputDevice(deviceId: string): Promise<void> {
    await this.room.switchActiveDevice("audioinput", deviceId);
  }

  async selectOutputDevice(deviceId: string): Promise<void> {
    await this.room.switchActiveDevice("audiooutput", deviceId);
  }

  setOutputVolume(value: number): void {
    this.outputVolume = Math.min(1, Math.max(0, value));
    for (const elements of this.audioElements.values()) {
      for (const element of elements) element.volume = this.outputVolume;
    }
  }

  async disconnect(): Promise<void> {
    for (const publication of this.room.localParticipant.trackPublications.values()) {
      publication.track?.stop();
    }
    this.clearRemoteAudio();
    try {
      await this.room.disconnect();
    } finally {
      this.activity.reset();
      this.stopPingUpdates();
      this.callbacks?.onStatus("disconnected");
      this.removeListeners();
      this.callbacks = undefined;
      this.connecting = false;
    }
  }

  private bindListeners(): void {
    this.room
      .on(RoomEvent.Reconnecting, this.handleReconnecting)
      .on(RoomEvent.Reconnected, this.handleReconnected)
      .on(RoomEvent.Disconnected, this.handleDisconnected)
      .on(RoomEvent.ParticipantConnected, this.handleRosterChange)
      .on(RoomEvent.ParticipantDisconnected, this.handleRosterChange)
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
      .off(RoomEvent.ParticipantConnected, this.handleRosterChange)
      .off(RoomEvent.ParticipantDisconnected, this.handleRosterChange)
      .off(RoomEvent.ActiveSpeakersChanged, this.emitParticipants)
      .off(RoomEvent.TrackMuted, this.emitParticipants)
      .off(RoomEvent.TrackUnmuted, this.emitParticipants)
      .off(RoomEvent.TrackSubscribed, this.handleTrackSubscribed)
      .off(RoomEvent.TrackUnsubscribed, this.handleTrackUnsubscribed)
      .off(RoomEvent.AudioPlaybackStatusChanged, this.handleAudioPlayback);
  }

  private readonly handleReconnecting = (): void => {
    this.callbacks?.onStatus("reconnecting");
  };

  private readonly handleReconnected = (): void => {
    this.callbacks?.onStatus("connected");
    this.emitParticipants();
  };

  private readonly handleDisconnected = (): void => {
    this.clearRemoteAudio();
    this.callbacks?.onStatus("disconnected");
    this.emitParticipants();
  };

  private readonly handleRosterChange = (): void => {
    this.emitParticipants();
    this.callbacks?.onRosterChanged();
  };

  private readonly handleAudioPlayback = (): void => {
    this.callbacks?.onAudioBlocked(!this.room.canPlaybackAudio);
  };

  private readonly handleTrackSubscribed = (
    track: RemoteTrack,
    publication: RemoteTrackPublication,
    _participant: RemoteParticipant,
  ): void => {
    if (track.kind !== Track.Kind.Audio) return;
    const element = track.attach();
    element.autoplay = true;
    element.volume = this.outputVolume;
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
    if (
      this.room.state !== ConnectionState.Disconnected &&
      this.room.localParticipant.identity
    ) {
      participants.push(this.room.localParticipant);
    }
    participants.push(...this.room.remoteParticipants.values());
    const identities = new Set(participants.map((participant) => participant.identity));
    this.activity.prune(identities);
    this.callbacks?.onParticipants(
      participants.map((participant) => {
        const muted = this.isMicrophoneMuted(participant);
        const level = audioLevel(participant);
        return {
        identity: participant.identity,
        name: participant.name || participant.identity,
        avatar: "",
        isLocal: participant === this.room.localParticipant,
        isSpeaking: this.activity.update(participant.identity, level, active.has(participant.identity), muted, now),
        isMicrophoneMuted: muted,
        pingMs,
      };
      }),
    );
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
      for (const publication of participant.trackPublications.values()) {
        publication.track?.detach().forEach((element) => element.remove());
      }
    }
    for (const elements of this.audioElements.values()) {
      for (const element of elements) element.remove();
    }
    this.audioElements.clear();
  }
}

function audioLevel(participant: Participant): number {
  return Number((participant as Participant & { audioLevel?: number }).audioLevel || 0);
}
