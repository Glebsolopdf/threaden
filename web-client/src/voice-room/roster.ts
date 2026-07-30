import { api } from "../api";
import type { VoiceParticipant } from "../voice";

export interface ParticipantProfile {
  name: string;
  avatar: string;
}

export type VoiceRoster = Map<string, ParticipantProfile>;

export async function loadVoiceRoster(room: {
  id: string;
  kind: "group" | "temporary";
  groupId?: string;
}): Promise<VoiceRoster> {
  if (room.kind === "group") {
    const groupId = room.groupId || await findGroupId(room.id);
    if (!groupId) return new Map();
    const profile = await api.groupProfile(groupId);
    return new Map(profile.members.map((member) => [
      member.id,
      { name: member.display_name, avatar: member.avatar || "" },
    ]));
  }

  const profile = await api.getRoom(room.id);
  return new Map(profile.members.map((member) => [
    member.id,
    { name: member.display_name, avatar: member.avatar || "" },
  ]));
}

export function applyRoster(
  participants: VoiceParticipant[],
  roster: VoiceRoster,
): VoiceParticipant[] {
  return participants.map((participant) => {
    const profile = roster.get(participant.identity);
    if (!profile) return participant;
    return {
      ...participant,
      name: profile.name || participant.name,
      avatar: profile.avatar,
    };
  });
}

async function findGroupId(voiceRoomId: string): Promise<string> {
  const groups = await api.groups();
  return groups.find((group) =>
    group.voice_rooms.some((room) => room.id === voiceRoomId),
  )?.id || "";
}
