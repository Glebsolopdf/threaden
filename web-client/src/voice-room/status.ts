import type { VoiceParticipant, VoiceStatus } from "../voice";
import { displayError } from "../settings/errors";

export const statusText: Record<VoiceStatus, [string, string]> = {
  connecting: ["Подключение…", "Готовим аудиосоединение"],
  connected: ["Подключено", "Пинг вычисляется…"],
  reconnecting: ["Переподключение…", "Пытаемся восстановить соединение"],
  disconnected: ["Отключено", "Соединение с комнатой завершено"],
  error: ["Ошибка подключения", "Не удалось подключиться к комнате"],
};

export function pingLabel(participants: VoiceParticipant[]): string {
  const ping = participants.find((participant) => participant.isLocal)?.pingMs ?? participants[0]?.pingMs;
  return ping === undefined ? "Пинг вычисляется…" : `Пинг ${ping} мс`;
}

export function errorMessage(error: unknown): string {
  return displayError(error);
}

export function renderRoomCode(
  codeWrap: HTMLElement,
  code: HTMLElement,
  copyButton: HTMLButtonElement,
  room: { id: string; kind: "group" | "temporary" },
): void {
  const temporary = room.kind === "temporary";
  codeWrap.hidden = !temporary;
  code.textContent = temporary ? room.id : "";
  copyButton.title = "Скопировать";
}

export async function copyRoomCode(code: HTMLElement, copyButton: HTMLButtonElement): Promise<void> {
  if (!code.textContent) return;
  try {
    await navigator.clipboard.writeText(code.textContent);
    copyButton.title = "Скопировано";
  } catch {
    copyButton.title = "Не удалось скопировать";
  }
}
