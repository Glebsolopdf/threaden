import { Track, type RemoteParticipant, type RemoteTrack, type RemoteTrackPublication, type Room } from 'livekit-client';
import type { PreferencesService } from '../../preferences/preferences.service';
import type { ScreenShareController } from '../screen-share/screen-share.controller';

export class VoiceAudio {
  private readonly audioElements = new Map<string, HTMLMediaElement[]>();
  private readonly screenShareAudioSids = new Set<string>();
  private readonly screenShareAudioByIdentity = new Map<string, HTMLMediaElement[]>();
  private readonly audioContainer = this.createContainer();
  private screenShareAudioMuted = false;

  constructor(private readonly room: Room, private readonly screenShare: ScreenShareController, private readonly preferences: PreferencesService) {}

  setOutputVolume(percent: number): void {
    const value = Math.min(100, Math.max(0, percent));
    for (const elements of this.audioElements.values()) for (const element of elements) element.volume = value / 100;
  }

  toggleScreenShareAudio(muted: boolean): void {
    this.screenShareAudioMuted = muted;
    for (const sid of this.screenShareAudioSids) for (const element of this.audioElements.get(sid) ?? []) element.muted = muted;
  }

  setScreenShareAudioMuted(identity: string, muted: boolean): void {
    for (const element of this.screenShareAudioByIdentity.get(identity) ?? []) element.muted = muted;
  }

  setScreenShareAudioVolume(identity: string, percent: number): void {
    const volume = Math.min(100, Math.max(0, percent)) / 100;
    for (const element of this.screenShareAudioByIdentity.get(identity) ?? []) element.volume = volume;
  }

  handleTrackSubscribed = (track: RemoteTrack, publication: RemoteTrackPublication, participant: RemoteParticipant): void => {
    if (this.screenShare.handleTrackSubscribed(track, publication, participant)) return;
    if (track.kind !== Track.Kind.Audio) return;
    const element = track.attach();
    element.autoplay = true;
    element.volume = this.preferences.audio().outputVolume / 100;
    this.audioContainer.append(element);
    this.audioElements.set(publication.trackSid, [element]);
    if (publication.source === Track.Source.ScreenShareAudio) {
      element.muted = this.screenShareAudioMuted;
      this.screenShareAudioSids.add(publication.trackSid);
      const elements = this.screenShareAudioByIdentity.get(participant.identity) ?? [];
      elements.push(element);
      this.screenShareAudioByIdentity.set(participant.identity, elements);
    }
  };

  handleTrackUnsubscribed = (track: RemoteTrack, publication: RemoteTrackPublication, participant: RemoteParticipant): void => {
    if (this.screenShare.handleTrackUnsubscribed(track, publication)) return;
    const detached = track.detach();
    for (const element of detached) element.remove();
    this.audioElements.delete(publication.trackSid);
    this.screenShareAudioSids.delete(publication.trackSid);
    if (publication.source === Track.Source.ScreenShareAudio) {
      const elements = (this.screenShareAudioByIdentity.get(participant.identity) ?? []).filter((element) => !detached.includes(element));
      if (elements.length) this.screenShareAudioByIdentity.set(participant.identity, elements);
      else this.screenShareAudioByIdentity.delete(participant.identity);
    }
  };

  clearRemote(): void {
    for (const participant of this.room.remoteParticipants.values()) {
      for (const publication of participant.trackPublications.values()) publication.track?.detach().forEach((element) => element.remove());
    }
    for (const elements of this.audioElements.values()) for (const element of elements) element.remove();
    this.audioElements.clear();
    this.screenShareAudioSids.clear();
    this.screenShareAudioByIdentity.clear();
    this.screenShareAudioMuted = false;
  }

  private createContainer(): HTMLElement {
    const container = document.createElement('div');
    container.hidden = true;
    container.id = 'audio-container';
    document.body.append(container);
    return container;
  }
}
