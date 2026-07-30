import { api } from "../api";
import { byId } from "../dom";
import { displayError } from "../settings/errors";
import { type VoiceParticipant, type VoiceStatus, VoiceClient } from "../voice";
import { loadActiveRoom, type StoredActiveRoom } from "./memory";
import { createRoomWidget } from "./widget";

const statusText: Record<VoiceStatus, string> = {
  connecting: "Подключение...",
  connected: "Подключено",
  reconnecting: "Переподключение...",
  disconnected: "Отключено",
  error: "Ошибка подключения",
};

export function initPersistentRoomWidget(onError: (message: string) => void): void {
  const room = loadActiveRoom();
  if (!room) return;
  const client = new VoiceClient(byId("audio-container"));
  let microphoneEnabled = false;
  let lastSpeakers: VoiceParticipant[] = [];
  let currentStatus: VoiceStatus = "connecting";

  const widget = createRoomWidget({
    openRoom: () => location.assign(room.kind === "group" ? `/group-voice-rooms/${room.id}/` : `/temporary/${room.id}/`),
    toggleMicrophone: () => {
      void client.setMicrophoneEnabled(!microphoneEnabled).catch((error: unknown) => {
        onError(`Не удалось изменить состояние микрофона: ${displayError(error)}`);
      });
    },
  });

  widget.update(room.title, "Подключение...", true);
  void connect(room, client, {
    onStatus: (status) => {
      currentStatus = status;
      widget.update(room.title, statusText[status], true);
    },
    onParticipants: (participants) => {
      microphoneEnabled = participants.some((participant) => participant.isLocal && !participant.isMicrophoneMuted);
      lastSpeakers = rememberSpeakers(lastSpeakers, participants);
      widget.setMicrophone(microphoneEnabled);
      widget.setSpeakers(lastSpeakers.length ? lastSpeakers : participants);
      if (currentStatus === "connected") widget.update(room.title, pingLabel(participants), true);
    },
    onError,
  });
}

async function connect(
  room: StoredActiveRoom,
  client: VoiceClient,
  options: {
    onStatus: (status: VoiceStatus) => void;
    onParticipants: (participants: VoiceParticipant[]) => void;
    onError: (message: string) => void;
  },
): Promise<void> {
  try {
    const join = room.kind === "group" ? await api.joinGroupVoice(room.id) : await api.joinRoom(room.id);
    await client.connect(join.livekit_url, join.access_token, {
      onStatus: options.onStatus,
      onParticipants: options.onParticipants,
      onRosterChanged: () => undefined,
      onAudioBlocked: () => undefined,
    });
  } catch (error) {
    options.onStatus("error");
    options.onError(`Не удалось восстановить звонок: ${displayError(error)}`);
  }
}

function rememberSpeakers(current: VoiceParticipant[], participants: VoiceParticipant[]): VoiceParticipant[] {
  let result = current;
  for (const participant of participants.filter((item) => item.isSpeaking)) {
    result = [
      participant,
      ...result.filter((item) => item.identity !== participant.identity),
    ].slice(0, 2);
  }
  return result;
}

function pingLabel(participants: VoiceParticipant[]): string {
  const ping = participants.find((participant) => participant.isLocal)?.pingMs ?? participants[0]?.pingMs;
  return ping === undefined ? "Пинг вычисляется…" : `Пинг ${ping} мс`;
}
