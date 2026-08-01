import { signal } from '@angular/core';
import {
  ConnectionState, LocalAudioTrack, LocalVideoTrack, type LocalTrack, type RemoteParticipant,
  type RemoteTrack, type RemoteTrackPublication, RemoteVideoTrack, Room, Track, VideoPreset,
} from 'livekit-client';
import { SCREEN_SHARE_PROFILES, type ScreenCaptureSettings, type ScreenShare, type ScreenShareError, type ScreenShareMode } from './screen-share.models';
import { readScreenShareStats, type ScreenShareStats } from './screen-share.stats';

export class ScreenShareController {
  private localTracks: LocalTrack[] = [];
  private readonly remoteAudioIdentityBySid = new Map<string, string>();
  private stopPromise?: Promise<void>;
  private startPromise?: Promise<void>;
  private statsTimer?: number;

  readonly isActive = signal(false);
  readonly isTransitioning = signal(false);
  readonly selectedMode = signal<ScreenShareMode>('balanced');
  readonly includeSystemAudio = signal(true);
  readonly capture = signal<ScreenCaptureSettings | undefined>(undefined);
  readonly localPreview = signal<ScreenShare | undefined>(undefined);
  readonly remoteShares = signal<ScreenShare[]>([]);
  readonly error = signal<ScreenShareError | undefined>(undefined);
  readonly stats = signal<ScreenShareStats | undefined>(undefined);

  constructor(private readonly room: Room, private readonly nameForIdentity: (identity: string, fallback: string) => string) {}

  async start(mode: ScreenShareMode): Promise<void> {
    if (this.startPromise) return this.startPromise;
    if (this.isActive()) throw new Error('Демонстрация экрана уже запущена');
    if (this.room.state !== ConnectionState.Connected) throw new Error('Сначала подключитесь к комнате');
    this.startPromise = this.startInternal(mode).finally(() => { this.startPromise = undefined; });
    return this.startPromise;
  }

  async stop(): Promise<void> {
    if (this.stopPromise) return this.stopPromise;
    this.stopPromise = this.stopInternal().finally(() => { this.stopPromise = undefined; });
    return this.stopPromise;
  }

  async changeMode(mode: ScreenShareMode): Promise<void> {
    if (!this.isActive()) return this.start(mode);
    await this.stop();
    await this.start(mode);
  }

  handleTrackSubscribed(track: RemoteTrack, publication: RemoteTrackPublication, participant: RemoteParticipant): boolean {
    if (publication.source === Track.Source.ScreenShareAudio) {
      this.remoteAudioIdentityBySid.set(publication.trackSid, participant.identity);
      this.setRemoteAudio(participant.identity, true);
      return false;
    }
    if (track.kind !== Track.Kind.Video || publication.source !== Track.Source.ScreenShare || !(track instanceof RemoteVideoTrack)) return false;
    const share: ScreenShare = {
      participantIdentity: participant.identity,
      participantName: this.nameForIdentity(participant.identity, participant.name || participant.identity),
      publicationSid: publication.trackSid,
      trackSid: track.sid || publication.trackSid,
      videoTrack: track,
      hasAudio: this.participantHasScreenAudio(participant),
      isMuted: publication.isMuted,
      dimensions: this.trackDimensions(track),
      isLocal: false,
    };
    this.remoteShares.update((items) => [...items.filter((item) => item.publicationSid !== share.publicationSid), share]);
    track.on('videoDimensionsChanged', this.refreshRemoteShares);
    return true;
  }

  handleTrackUnsubscribed(track: RemoteTrack, publication: RemoteTrackPublication): boolean {
    if (publication.source === Track.Source.ScreenShareAudio) {
      const identity = this.remoteAudioIdentityBySid.get(publication.trackSid);
      if (identity) this.setRemoteAudio(identity, false);
      this.remoteAudioIdentityBySid.delete(publication.trackSid);
      return false;
    }
    if (publication.source !== Track.Source.ScreenShare) return false;
    track.off('videoDimensionsChanged', this.refreshRemoteShares);
    for (const element of track.detach()) element.remove();
    this.remoteShares.update((items) => items.filter((item) => item.publicationSid !== publication.trackSid));
    return true;
  }

  clearRemote(): void {
    for (const share of this.remoteShares()) for (const element of share.videoTrack.detach()) element.remove();
    this.remoteAudioIdentityBySid.clear();
    this.remoteShares.set([]);
  }

  disableRemoteShare(share: ScreenShare): void {
    if (share.isLocal) return;
    const participant = this.room.remoteParticipants.get(share.participantIdentity);
    if (participant) {
      for (const publication of participant.trackPublications.values()) {
        if (publication.source === Track.Source.ScreenShare || publication.source === Track.Source.ScreenShareAudio) publication.setSubscribed(false);
      }
    }
    for (const element of share.videoTrack.detach()) element.remove();
    this.remoteShares.update((items) => items.filter((item) => item.publicationSid !== share.publicationSid));
  }

  async destroy(): Promise<void> {
    await this.stop();
    this.clearRemote();
  }

  private async startInternal(mode: ScreenShareMode): Promise<void> {
    const profile = SCREEN_SHARE_PROFILES[mode];
    this.isTransitioning.set(true);
    this.error.set(undefined);
    try {
      const tracks = await this.room.localParticipant.createScreenTracks({
        audio: this.includeSystemAudio() ? { echoCancellation: false, noiseSuppression: false, restrictOwnAudio: true } : false,
        video: true, resolution: { width: profile.width, height: profile.height, frameRate: profile.frameRate },
        contentHint: profile.contentHint, systemAudio: this.includeSystemAudio() ? 'include' : 'exclude', suppressLocalAudioPlayback: true,
      });
      this.localTracks = tracks;
      const video = tracks.find((track): track is LocalVideoTrack => track instanceof LocalVideoTrack);
      if (!video) throw new Error('Браузер не создал видеотрек демонстрации');
      video.mediaStreamTrack.contentHint = profile.contentHint;
      const capture = this.readCapture(video.mediaStreamTrack.getSettings());
      this.capture.set(capture);
      video.mediaStreamTrack.addEventListener('ended', this.handleEnded, { once: true });
      const options = {
        source: Track.Source.ScreenShare, stream: 'screen_share', simulcast: true, videoCodec: 'vp8' as const,
        screenShareEncoding: { maxBitrate: profile.maxBitrate, maxFramerate: profile.frameRate },
        screenShareSimulcastLayers: this.layersFor(profile.width, profile.height, profile.frameRate),
        degradationPreference: (profile.contentHint === 'motion' ? 'maintain-framerate' : 'maintain-resolution') as RTCDegradationPreference,
      };
      await this.room.localParticipant.publishTrack(video, options);
      const audio = tracks.find((track): track is LocalAudioTrack => track instanceof LocalAudioTrack);
      if (audio) await this.room.localParticipant.publishTrack(audio, { source: Track.Source.ScreenShareAudio, stream: 'screen_share' });
      this.selectedMode.set(mode);
      this.isActive.set(true);
      this.localPreview.set({ participantIdentity: this.room.localParticipant.identity, participantName: 'Вы', publicationSid: video.sid || video.id, trackSid: video.sid || video.id, videoTrack: video, hasAudio: Boolean(audio), isMuted: false, dimensions: video.dimensions, isLocal: true });
      this.startStats(video);
    } catch (error) {
      await this.cleanupTracks();
      this.error.set(this.toUserError(error));
      throw error;
    } finally { this.isTransitioning.set(false); }
  }

  private async stopInternal(): Promise<void> {
    this.isTransitioning.set(true);
    try { await this.cleanupTracks(); }
    finally { this.isTransitioning.set(false); }
  }

  private async cleanupTracks(): Promise<void> {
    this.stopStats();
    const tracks = this.localTracks;
    this.localTracks = [];
    for (const track of tracks) {
      track.mediaStreamTrack.removeEventListener('ended', this.handleEnded);
      await this.room.localParticipant.unpublishTrack(track, false).catch(() => undefined);
      track.stop();
    }
    this.localPreview.set(undefined);
    this.capture.set(undefined);
    this.stats.set(undefined);
    this.isActive.set(false);
  }

  private startStats(track: LocalVideoTrack): void {
    this.stopStats();
    const update = () => void readScreenShareStats(track, this.capture() ?? {}, this.room.engine.client.rtt * 1000).then((stats) => this.stats.set(stats)).catch(() => undefined);
    update();
    this.statsTimer = window.setInterval(update, 1500);
  }
  private stopStats(): void { if (this.statsTimer !== undefined) window.clearInterval(this.statsTimer); this.statsTimer = undefined; }
  private readonly handleEnded = (): void => { void this.stop(); };
  private readonly refreshRemoteShares = (): void => this.remoteShares.update((items) => items.map((item) => ({ ...item, dimensions: this.trackDimensions(item.videoTrack) })));
  private setRemoteAudio(identity: string, hasAudio: boolean): void {
    this.remoteShares.update((items) => items.map((item) => item.participantIdentity === identity ? { ...item, hasAudio } : item));
  }
  private participantHasScreenAudio(participant: RemoteParticipant): boolean {
    return Boolean(participant.getTrackPublication(Track.Source.ScreenShareAudio));
  }
  private readCapture(settings: MediaTrackSettings): ScreenCaptureSettings { return { width: settings.width, height: settings.height, frameRate: settings.frameRate, displaySurface: settings.displaySurface }; }
  private layersFor(width: number, height: number, fps: number): VideoPreset[] {
    const layers = [[640, 360, 600_000, Math.min(fps, 15)], [1280, 720, 2_500_000, Math.min(fps, 30)]];
    return layers.filter(([layerWidth, layerHeight]) => layerWidth < width && layerHeight < height).map(([layerWidth, layerHeight, bitrate, frameRate]) => new VideoPreset(layerWidth, layerHeight, bitrate, frameRate));
  }
  private trackDimensions(track: LocalVideoTrack | RemoteVideoTrack): { width: number; height: number } | undefined {
    const settings = track.mediaStreamTrack.getSettings();
    return settings.width && settings.height ? { width: settings.width, height: settings.height } : undefined;
  }
  private toUserError(error: unknown): ScreenShareError {
    const name = error instanceof DOMException ? error.name : '';
    const technicalMessage = error instanceof Error ? error.message : String(error);
    if (name === 'NotAllowedError') return { message: 'Демонстрация отменена или доступ к экрану запрещён.', technicalMessage };
    if (name === 'NotFoundError') return { message: 'Источник для демонстрации не найден.', technicalMessage };
    if (!navigator.mediaDevices?.getDisplayMedia) return { message: 'Ваш браузер не поддерживает демонстрацию экрана.', technicalMessage };
    return { message: 'Не удалось начать демонстрацию экрана.', technicalMessage };
  }
}
