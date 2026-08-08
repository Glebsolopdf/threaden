import { ACTIVE_ROOM_KEY, type ActiveVoiceRoom } from '../models';

export function saveRoom(room: ActiveVoiceRoom): void {
  try { localStorage.setItem(ACTIVE_ROOM_KEY, JSON.stringify(room)); } catch { /* optional */ }
}

export function loadRoom(): ActiveVoiceRoom | null {
  try {
    const raw = localStorage.getItem(ACTIVE_ROOM_KEY);
    if (!raw) return null;
    const value = JSON.parse(raw) as Partial<ActiveVoiceRoom>;
    if (!value.id || !value.title) return null;
    return { id: value.id, title: value.title, kind: value.kind === 'group' ? 'group' : 'temporary', groupId: value.groupId };
  } catch { return null; }
}

export function clearRoom(): void {
  try { localStorage.removeItem(ACTIVE_ROOM_KEY); } catch { /* optional */ }
}
