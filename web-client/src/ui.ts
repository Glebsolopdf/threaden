import type { RoomInfo, User } from "./api";
import { showAvatar } from "./dom";
import type { VoiceParticipant } from "./voice";

export function renderParticipants(
  list: HTMLUListElement,
  count: HTMLElement,
  room: RoomInfo | null,
  user: User | null,
  participants: VoiceParticipant[],
): void {
  list.replaceChildren();
  const members = new Map(room?.members.map((member) => [member.id, member]));

  for (const participant of participants) {
    const member = members.get(participant.identity);
    const item = document.createElement("li");
    item.className = "participant";
    if (participant.isSpeaking) item.classList.add("participant--speaking");

    const avatar = document.createElement("span");
    avatar.className = "participant__avatar";
    showAvatar(avatar, member?.avatar || "", member?.display_name || participant.name);

    const details = document.createElement("span");
    details.className = "participant__details";
    const name = document.createElement("strong");
    name.textContent = member?.display_name || participant.name;
    const role = document.createElement("small");
    role.textContent = participantRole(participant, room, user);
    details.append(name, role);

    item.append(avatar, details);
    if (participant.isMicrophoneMuted) {
      const muted = document.createElement("img");
      muted.className = "participant__muted";
      muted.alt = "Микрофон выключен";
      muted.src = "/microphone-off.svg";
      item.append(muted);
    }
    list.append(item);
  }

  count.textContent = String(participants.length);
}

export function updateRoomProfile(
  avatar: HTMLElement,
  name: HTMLElement,
  user: User | null,
  fallbackAvatar: string,
  fallbackName: string,
): void {
  const label = user?.display_name || fallbackName.trim() || "Пользователь";
  showAvatar(avatar, user?.avatar || fallbackAvatar, label);
  name.textContent = label;
}

export function setMicrophoneLabel(button: HTMLButtonElement, enabled: boolean): void {
  const icon = document.createElement("img");
  icon.alt = "";
  icon.src = enabled ? "/microphone-on.svg" : "/microphone-off.svg";
  button.replaceChildren(icon);
  button.setAttribute("aria-label", enabled ? "Выключить микрофон" : "Включить микрофон");
}

export function setPrejoinMicrophoneLabel(button: HTMLButtonElement, enabled: boolean): void {
  const icon = document.createElement("img");
  icon.alt = "";
  icon.src = enabled ? "/microphone-on.svg" : "/microphone-off.svg";
  button.replaceChildren(icon);
  button.setAttribute("aria-label", enabled ? "Выключить микрофон" : "Включить микрофон");
  button.dataset.enabled = String(enabled);
}

function participantRole(
  participant: VoiceParticipant,
  room: RoomInfo | null,
  user: User | null,
): string {
  if (participant.identity === user?.id) return room?.owner.id === user.id ? "Создатель" : "Вы";
  return participant.isSpeaking ? "Говорит" : "В комнате";
}
