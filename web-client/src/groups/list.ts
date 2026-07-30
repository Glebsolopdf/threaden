import type { GroupInfo } from "../api";
import { showAvatar } from "../dom";

export function groupRow(group: GroupInfo, onOpen: (id: string) => void): HTMLButtonElement {
  const button = document.createElement("button");
  button.className = "group-row";
  button.type = "button";

  const avatar = document.createElement("span");
  avatar.className = "avatar";
  showAvatar(avatar, group.avatar, group.name);

  const text = document.createElement("span");
  const name = document.createElement("strong");
  const preview = document.createElement("small");
  name.textContent = group.name;
  preview.textContent = group.last_message?.body || "Нет сообщений";

  text.append(name, preview);
  button.append(avatar, text);
  button.onclick = () => onOpen(group.id);
  return button;
}
