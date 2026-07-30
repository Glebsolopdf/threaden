import type { GroupInfo, GroupVoiceRoom } from "../../api";
import { groupRow } from "../../groups/list";
import type { MainElements } from "../elements";
import { showGroup } from "../navigation/views";

export function renderGroupList(
  target: HTMLElement,
  groups: GroupInfo[],
  onOpen: (id: string) => void,
): void {
  target.replaceChildren();
  if (!groups.length) target.textContent = "Пока нет групп";
  for (const group of groups) target.append(groupRow(group, onOpen));
}

export function renderDiscoverList(
  target: HTMLElement,
  groups: GroupInfo[],
  onOpen: (id: string) => void,
): void {
  target.replaceChildren();
  if (!groups.length) target.textContent = "Ничего не найдено";
  for (const group of groups) target.append(groupRow(group, onOpen));
}

export function renderGroup(
  elements: MainElements,
  group: GroupInfo,
  member: boolean,
  onJoinVoice: (id: string) => void,
  onDeleteVoice: (id: string) => void,
  currentUserID = "",
): void {
  showGroup(elements);
  elements.name.textContent = group.name;
  elements.meta.textContent = groupMeta(group);
  renderVoiceList(elements, group, onJoinVoice, onDeleteVoice, group.owner.id === currentUserID);
  renderVoiceStrip(elements, group);
  elements.voiceButton.hidden = !member;
  elements.join.hidden = member;
  elements.join.disabled = false;
  elements.join.dataset.state = "idle";
  elements.join.textContent = "Присоединиться к группе";
  elements.form.hidden = !member;
}

function renderVoiceStrip(elements: MainElements, group: GroupInfo): void {
  const count = (group.voice_rooms ?? []).reduce((sum, room) => sum + room.participant_count, 0);
  elements.voiceButton.dataset.live = String(count > 0);
  elements.voiceStrip.hidden = count === 0;
  if (count === 0) {
    elements.voiceStrip.replaceChildren();
    return;
  }
  const icon = document.createElement("img");
  const label = document.createElement("span");
  icon.src = "/microphone-on.svg";
  icon.alt = "";
  label.textContent = String(count);
  elements.voiceStrip.replaceChildren(icon, label);
}

export function renderVoiceList(
  elements: MainElements,
  group: GroupInfo,
  onJoinVoice: (id: string) => void,
  onDeleteVoice: (id: string) => void,
  canManage = false,
): void {
  const rooms = group.voice_rooms ?? [];
  elements.voiceTitle.textContent = `${group.name}: голосовые комнаты`;
  elements.voiceMeta.textContent = `${rooms.length} комнат`;
  elements.voiceList.replaceChildren(...rooms.map((room) => voiceRoomRow(room, onJoinVoice, onDeleteVoice, canManage)));
  if (!rooms.length) elements.voiceList.textContent = "Голосовых комнат пока нет";
}

export function groupMeta(group: GroupInfo): string {
  return `${group.member_count} участников · ${group.online_count} онлайн`;
}

function voiceRoomRow(
  room: GroupVoiceRoom,
  onJoinVoice: (id: string) => void,
  onDeleteVoice: (id: string) => void,
  canManage: boolean,
): HTMLElement {
  const row = document.createElement("article");
  const text = document.createElement("span");
  const button = document.createElement("button");
  const name = document.createElement("strong");
  const meta = document.createElement("span");

  row.className = "group-voice__row";
  text.className = "group-voice__details";
  button.type = "button";
  button.className = "group-voice__join themed-button";
  button.textContent = "Присоединиться";
  name.textContent = room.name;
  meta.textContent = `${room.participant_count} участников`;
  text.append(name, meta);
  row.append(text, button);
  button.onclick = () => onJoinVoice(room.id);
  if (canManage) row.append(deleteVoiceButton(() => onDeleteVoice(room.id)));
  return row;
}

function deleteVoiceButton(onClick: () => void): HTMLButtonElement {
  const button = document.createElement("button");
  const icon = document.createElement("img");
  button.type = "button";
  button.className = "group-voice__delete";
  button.setAttribute("aria-label", "Удалить голосовую комнату");
  icon.src = "/trash.svg";
  icon.alt = "";
  button.append(icon);
  button.onclick = onClick;
  return button;
}
