import type { VoiceParticipant } from "../voice";
import { avatarColor, showAvatar } from "../dom";

export function renderParticipantCards(target: HTMLElement, participants: VoiceParticipant[]): void {
  target.replaceChildren(...participants.map(participantCard));
}

function participantCard(participant: VoiceParticipant): HTMLLIElement {
  const item = document.createElement("li");
  item.className = participant.isSpeaking ? "voice-participant-card is-speaking" : "voice-participant-card";
  const color = avatarColor(participant.name || participant.identity);
  item.style.setProperty("--participant-bg", color);
  item.style.setProperty("--avatar-bg", color);
  if (participant.avatar.startsWith("data:image/") || participant.avatar.startsWith("blob:")) {
    item.style.setProperty("--participant-image", `url(${JSON.stringify(participant.avatar)})`);
  }

  const avatar = document.createElement("div");
  avatar.className = "voice-participant-card__avatar";
  showAvatar(avatar, participant.avatar, participant.name);

  const name = document.createElement("strong");
  name.textContent = participant.isLocal ? `${participant.name} (вы)` : participant.name;

  const footer = document.createElement("footer");
  footer.append(name);
  item.append(avatar, footer);
  if (participant.isMicrophoneMuted) item.append(mutedIcon());
  return item;
}

function mutedIcon(): HTMLElement {
  const icon = document.createElement("span");
  icon.className = "voice-participant-card__mic";
  icon.setAttribute("aria-label", "Микрофон выключен");
  return icon;
}
