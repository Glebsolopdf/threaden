export type VoiceStatus = 'connecting' | 'connected' | 'reconnecting' | 'disconnected' | 'error';
export type RoomKind = 'group' | 'temporary';

export interface ActiveVoiceRoom {
  id: string;
  kind: RoomKind;
  title: string;
  groupId?: string;
}

export interface VoiceParticipant {
  identity: string;
  name: string;
  avatar: string;
  isLocal: boolean;
  isSpeaking: boolean;
  isMicrophoneMuted: boolean;
  pingMs?: number;
}

export const ACTIVE_ROOM_KEY = 'voice_rooms_active_room';

export const VOICE_STATUS_LABELS: Readonly<Record<VoiceStatus, string>> = {
  connecting: 'Подключение…',
  connected: 'Подключено',
  reconnecting: 'Переподключение…',
  disconnected: 'Отключено',
  error: 'Ошибка подключения',
};

export const VOICE_DETAIL_LABELS: Readonly<Record<Exclude<VoiceStatus, 'connected'>, string>> = {
  connecting: 'Готовим аудиосоединение',
  reconnecting: 'Пытаемся восстановить соединение',
  disconnected: 'Соединение с комнатой завершено',
  error: 'Не удалось подключиться к комнате',
};
