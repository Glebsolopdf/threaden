import { ApiError } from "../api";
import { displayError } from "../settings/errors";
import type { NotificationCenter } from "./types";

const VOICE_ROOM_LIMIT_MESSAGE = "Достигнут лимит голосовых комнат в этой группе";

export function notifyError(
  center: NotificationCenter,
  value: unknown = "",
  fallback?: string,
): void {
  if (!value) {
    center.clear();
    return;
  }
  if (value instanceof ApiError && value.code === "voice_room_limit") {
    center.neutral(VOICE_ROOM_LIMIT_MESSAGE);
    return;
  }
  center.error(displayError(value, fallback));
}
