import { ConnectionState, Track, type Participant, type Room } from 'livekit-client';
import type { VoiceActivityDetector } from '../activity-detector';
import type { VoiceParticipant } from './models';

export function buildParticipants(
  room: Room,
  roster: Map<string, { name: string; avatar: string }>,
  activity: VoiceActivityDetector,
  pingMs: number | undefined,
  now: number,
): VoiceParticipant[] {
  const active = new Set(room.activeSpeakers.map((participant) => participant.identity));
  const participants: Participant[] = [];
  if (room.state !== ConnectionState.Disconnected && room.localParticipant.identity) participants.push(room.localParticipant);
  participants.push(...room.remoteParticipants.values());
  const identities = new Set(participants.map((participant) => participant.identity));
  activity.prune(identities);
  return participants.map((participant) => {
    const publication = participant.getTrackPublication(Track.Source.Microphone);
    const muted = !publication || publication.isMuted;
    const profile = roster.get(participant.identity);
    return {
      identity: participant.identity,
      name: profile?.name || participant.name || participant.identity,
      avatar: profile?.avatar || '',
      isLocal: participant === room.localParticipant,
      isSpeaking: activity.update(participant.identity, Number((participant as Participant & { audioLevel?: number }).audioLevel ?? 0), active.has(participant.identity), muted, now),
      isMicrophoneMuted: muted,
      pingMs,
    };
  });
}
