export interface User {
  id: string;
  email: string;
  display_name: string;
  avatar?: string;
  created_at: string;
  security?: { can_manage: boolean; alert: boolean };
}

export interface SecuritySession {
  id: string;
  created_at: string;
  last_seen_at: string;
  current: boolean;
}

export interface Member {
  id: string;
  display_name: string;
  avatar?: string;
  joined_at: string;
}

export interface GroupMember extends Member {
  role: 'owner' | 'member';
}

export interface RoomInfo {
  code: string;
  owner: User;
  created_at: string;
  expires_at: string;
  participant_count: number;
  max_participants: number;
  members: Member[];
}

export interface JoinResponse {
  livekit_url: string;
  access_token: string;
  room_code: string;
}

export interface GroupMessage {
  id: string;
  group_id: string;
  author: User;
  body: string;
  created_at: string;
  edited_at?: string;
}

export interface GroupMemberEvent {
  member: Pick<User, 'id' | 'display_name' | 'avatar'>;
}

export type GroupMemberEventType = 'member_joined' | 'member_left' | 'member_removed';

export interface GroupVoiceRoom {
  id: string;
  group_id: string;
  name: string;
  created_at: string;
  participant_count: number;
}

export interface GroupInfo {
  id: string;
  visibility: 'public' | 'private';
  owner: User;
  name: string;
  avatar: string;
  invite_token?: string;
  created_at: string;
  last_activity_at: string;
  member_count: number;
  online_count: number;
  last_message?: GroupMessage;
  voice_rooms?: GroupVoiceRoom[];
}

export interface GroupProfile {
  group: GroupInfo;
  members: GroupMember[];
}

export interface GroupVoiceJoin {
  livekit_url: string;
  access_token: string;
  voice_room_id: string;
}

export interface EventEnvelope<T = unknown> {
  type: string;
  group_id?: string;
  data?: T;
}
