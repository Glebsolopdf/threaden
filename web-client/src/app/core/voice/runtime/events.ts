import { RoomEvent, type Room } from 'livekit-client';

export interface VoiceRoomHandlers {
  reconnecting: () => void;
  reconnected: () => void;
  disconnected: () => void;
  rosterChanged: () => void;
  participantsChanged: () => void;
  trackSubscribed: (...args: any[]) => void;
  trackUnsubscribed: (...args: any[]) => void;
  audioPlayback: () => void;
}

export function bindVoiceRoomEvents(room: Room, handlers: VoiceRoomHandlers): void {
  room.on(RoomEvent.Reconnecting, handlers.reconnecting).on(RoomEvent.Reconnected, handlers.reconnected)
    .on(RoomEvent.Disconnected, handlers.disconnected).on(RoomEvent.ParticipantConnected, handlers.rosterChanged)
    .on(RoomEvent.ParticipantDisconnected, handlers.rosterChanged).on(RoomEvent.ActiveSpeakersChanged, handlers.participantsChanged)
    .on(RoomEvent.TrackMuted, handlers.participantsChanged).on(RoomEvent.TrackUnmuted, handlers.participantsChanged)
    .on(RoomEvent.TrackSubscribed, handlers.trackSubscribed).on(RoomEvent.TrackUnsubscribed, handlers.trackUnsubscribed)
    .on(RoomEvent.AudioPlaybackStatusChanged, handlers.audioPlayback);
}

export function removeVoiceRoomEvents(room: Room, handlers: VoiceRoomHandlers): void {
  room.off(RoomEvent.Reconnecting, handlers.reconnecting).off(RoomEvent.Reconnected, handlers.reconnected)
    .off(RoomEvent.Disconnected, handlers.disconnected).off(RoomEvent.ParticipantConnected, handlers.rosterChanged)
    .off(RoomEvent.ParticipantDisconnected, handlers.rosterChanged).off(RoomEvent.ActiveSpeakersChanged, handlers.participantsChanged)
    .off(RoomEvent.TrackMuted, handlers.participantsChanged).off(RoomEvent.TrackUnmuted, handlers.participantsChanged)
    .off(RoomEvent.TrackSubscribed, handlers.trackSubscribed).off(RoomEvent.TrackUnsubscribed, handlers.trackUnsubscribed)
    .off(RoomEvent.AudioPlaybackStatusChanged, handlers.audioPlayback);
}
