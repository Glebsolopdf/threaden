const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || "").replace(/\/$/, "");

export interface User {
  id: string;
  email: string;
  display_name: string;
  avatar?: string;
  created_at: string;
}

export interface Member {
  id: string;
  display_name: string;
  avatar?: string;
  joined_at: string;
}
export interface GroupMember extends Member { role: "owner" | "member"; }

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

export interface GroupMessage { id: string; group_id: string; author: User; body: string; created_at: string; }
export interface GroupVoiceRoom { id: string; group_id: string; name: string; created_at: string; participant_count: number; }
export interface GroupInfo { id: string; visibility: "public" | "private"; owner: User; name: string; avatar: string; invite_token?: string; created_at: string; last_activity_at: string; member_count: number; online_count: number; last_message?: GroupMessage; voice_rooms: GroupVoiceRoom[]; }
export interface GroupProfile { group: GroupInfo; members: GroupMember[]; }
export interface GroupVoiceJoin { livekit_url: string; access_token: string; voice_room_id: string; }

interface ErrorEnvelope {
  error?: {
    code?: string;
    message?: string;
    request_id?: string;
  };
}

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
    public readonly requestId = "",
  ) {
    super(message);
    this.name = "ApiError";
  }
}

function parsePayload(text: string): unknown {
  try {
    return text ? JSON.parse(text) : undefined;
  } catch {
    return undefined;
  }
}

function apiError(status: number, payload: unknown): ApiError {
  const envelope = payload as ErrorEnvelope | undefined;
  return new ApiError(
    status,
    envelope?.error?.code || "request_failed",
    envelope?.error?.message || `Ошибка сервера (${status})`,
    envelope?.error?.request_id,
  );
}

async function request<T>(
  path: string,
  options: { method?: string; body?: unknown } = {},
): Promise<T> {
  const headers = new Headers({ Accept: "application/json" });
  const isForm = options.body instanceof FormData;
  if (options.body !== undefined && !isForm) headers.set("Content-Type", "application/json");

  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      method: options.method || "GET",
      headers,
      credentials: "include",
      body: options.body === undefined || isForm ? (options.body as BodyInit | undefined) : JSON.stringify(options.body),
    });
  } catch {
    throw new ApiError(0, "network_error", "Не удалось связаться с сервером");
  }

  if (response.status === 204) {
    if (!response.ok) throw new ApiError(response.status, "request_failed", "Запрос не выполнен");
    return undefined as T;
  }

  const text = await response.text();
  const payload = parsePayload(text);
  if (!response.ok) throw apiError(response.status, payload);
  return payload as T;
}

function uploadProfile(displayName: string, avatar: File | undefined, onProgress?: (percent: number) => void): Promise<User> {
  const body = new FormData();
  body.set("display_name", displayName);
  if (avatar) body.set("avatar", avatar);
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PATCH", `${API_BASE_URL}/v1/me`);
    xhr.withCredentials = true;
    xhr.setRequestHeader("Accept", "application/json");
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) onProgress?.(Math.round((event.loaded / event.total) * 100));
    };
    xhr.onload = () => {
      const payload = parsePayload(xhr.responseText);
      if (xhr.status >= 200 && xhr.status < 300) resolve(payload as User);
      else reject(apiError(xhr.status, payload));
    };
    xhr.onerror = () => reject(new ApiError(0, "network_error", "Не удалось связаться с сервером"));
    xhr.send(body);
  });
}

export const api = {
  register: (email: string, password: string) => request<User>("/v1/auth/register", { method: "POST", body: { email, password } }),
  login: (email: string, password: string) => request<User>("/v1/auth/login", { method: "POST", body: { email, password } }),
  logout: () => request<void>("/v1/auth/logout", { method: "DELETE" }),
  getMe: () => request<User>("/v1/me"),
  updateProfile: uploadProfile,
  deleteAvatar: () => request<User>("/v1/me/avatar", { method: "DELETE" }),
  deleteProfile: () => request<void>("/v1/me", { method: "DELETE" }),
  createRoom: () => request<RoomInfo>("/v1/rooms", { method: "POST" }),
  getRoom: (code: string) => request<RoomInfo>(`/v1/rooms/${code}`),
  joinRoom: (code: string) => request<JoinResponse>(`/v1/rooms/${code}/join`, { method: "POST" }),
  leaveRoom: (code: string) => request<void>(`/v1/rooms/${code}/members/me`, { method: "DELETE" }),
  deleteRoom: (code: string) => request<void>(`/v1/rooms/${code}`, { method: "DELETE" }),
  groups: () => request<GroupInfo[]>("/v1/groups"),
  discover: (q = "", limit = 20, offset = 0) => request<GroupInfo[]>(`/v1/discover/groups?q=${encodeURIComponent(q)}&limit=${limit}&offset=${offset}`),
  group: (id: string) => request<GroupInfo>(`/v1/groups/${id}`),
  createGroup: (body: { name: string; avatar: string; visibility: string }) => request<GroupInfo>("/v1/groups", { method: "POST", body }),
  groupProfile: (id: string) => request<GroupProfile>(`/v1/groups/${id}/profile`),
  leaveGroup: (id: string) => request<void>(`/v1/groups/${id}/members/me`, { method: "DELETE" }),
  removeGroupMember: (id: string, memberID: string) => request<GroupProfile>(`/v1/groups/${id}/members/${memberID}`, { method: "DELETE" }),
  deleteGroup: (id: string) => request<void>(`/v1/groups/${id}`, { method: "DELETE" }),
  joinGroup: (id: string) => request<GroupInfo>(`/v1/groups/${id}/members`, { method: "POST" }),
  invite: (token: string) => request<GroupInfo>(`/v1/invites/${token}`),
  joinInvite: (token: string) => request<GroupInfo>(`/v1/invites/${token}/join`, { method: "POST" }),
  messages: (id: string, limit = 30) => request<GroupMessage[]>(`/v1/groups/${id}/messages?limit=${limit}`),
  sendMessage: (id: string, body: string) => request<GroupMessage>(`/v1/groups/${id}/messages`, { method: "POST", body: { body } }),
  createGroupVoice: (id: string, name: string) => request<GroupVoiceRoom>(`/v1/groups/${id}/voice-rooms`, { method: "POST", body: { name } }),
  joinGroupVoice: (id: string) => request<GroupVoiceJoin>(`/v1/group-voice-rooms/${id}/join`, { method: "POST" }),
  leaveGroupVoice: (id: string) => request<void>(`/v1/group-voice-rooms/${id}/members/me`, { method: "DELETE" }),
  deleteGroupVoice: (id: string) => request<void>(`/v1/group-voice-rooms/${id}`, { method: "DELETE" }),
};
