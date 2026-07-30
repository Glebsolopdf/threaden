const ACTIVE_ROOM_KEY = "voice_rooms_active_room";

export type StoredRoomKind = "group" | "temporary";

export interface StoredActiveRoom {
  id: string;
  kind: StoredRoomKind;
  title: string;
  groupId?: string;
}

export function loadActiveRoomCode(): string {
  return loadActiveRoom()?.id || "";
}

export function saveActiveRoomCode(code: string): void {
  saveActiveRoom({ id: code, kind: "temporary", title: `Комната ${code}` });
}

export function loadActiveRoom(): StoredActiveRoom | null {
  const stored = localStorage.getItem(ACTIVE_ROOM_KEY);
  if (!stored) return null;
  try {
    return normalize(JSON.parse(stored));
  } catch {
    const legacy = stored.trim();
    return legacy ? { id: legacy, kind: "temporary", title: `Комната ${legacy}` } : null;
  }
}

export function saveActiveRoom(room: StoredActiveRoom): void {
  localStorage.setItem(ACTIVE_ROOM_KEY, JSON.stringify(room));
}

export function clearActiveRoomCode(): void {
  localStorage.removeItem(ACTIVE_ROOM_KEY);
}

function normalize(value: unknown): StoredActiveRoom | null {
  const room = value as Partial<StoredActiveRoom> | null;
  if (!room || typeof room.id !== "string") return null;
  const kind = room.kind === "group" ? "group" : "temporary";
  const fallback = kind === "group" ? "Голосовая комната" : `Комната ${room.id}`;
  return {
    id: room.id,
    kind,
    title: typeof room.title === "string" && room.title.trim() ? room.title : fallback,
    groupId: typeof room.groupId === "string" && room.groupId.trim() ? room.groupId : undefined,
  };
}
