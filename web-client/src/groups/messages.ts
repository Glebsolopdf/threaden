import type { GroupMessage, User } from "../api";
import { showAvatar } from "../dom";

type MessageStatus = "sending" | "sent" | "error";
export type MessageAnimation = "outgoing" | "incoming";

export interface ChatMessageView {
  message: GroupMessage;
  status?: MessageStatus;
  animate?: MessageAnimation;
}

export function renderChatMessages(
  target: HTMLElement,
  messages: ChatMessageView[],
  currentUser: User | null,
): void {
  target.replaceChildren(...messages.map((item, index) => {
    const previous = messages[index - 1]?.message;
    return chatMessage(item, {
      own: item.message.author.id === currentUser?.id,
      compact: previous?.author.id === item.message.author.id,
    });
  }));
  target.scrollTop = target.scrollHeight;
}

function chatMessage(item: ChatMessageView, options: { own: boolean; compact: boolean }): HTMLElement {
  const article = document.createElement("article");
  article.className = `chat-message ${options.own ? "chat-message--own" : "chat-message--other"}`;
  article.dataset.status = item.status || "sent";
  article.dataset.compact = String(options.compact);
  if (item.animate) article.dataset.animate = item.animate;

  if (!options.own) article.append(authorAvatar(item.message, options.compact));
  article.append(messageBubble(item, options));
  return article;
}

function authorAvatar(message: GroupMessage, compact: boolean): HTMLElement {
  const avatar = document.createElement("span");
  avatar.className = "chat-message__avatar avatar";
  if (!compact) showAvatar(avatar, message.author.avatar || "", message.author.display_name);
  return avatar;
}

function messageBubble(item: ChatMessageView, options: { own: boolean; compact: boolean }): HTMLElement {
  const bubble = document.createElement("div");
  bubble.className = "chat-message__bubble";

  if (!options.own && !options.compact) {
    const author = document.createElement("strong");
    author.className = "chat-message__author";
    author.textContent = item.message.author.display_name;
    bubble.append(author);
  }

  const body = document.createElement("p");
  body.textContent = item.message.body;

  const meta = document.createElement("footer");
  const time = document.createElement("time");
  time.dateTime = item.message.created_at;
  time.textContent = new Date(item.message.created_at).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
  });
  meta.append(time, statusLabel(item.status || "sent", editedLabel(item.message)));

  bubble.append(body, meta);
  return bubble;
}

function editedLabel(message: GroupMessage): string {
  return (message as GroupMessage & { edited_at?: string }).edited_at ? "изменено" : "";
}

function statusLabel(status: MessageStatus, edited: string): Text {
  const label = status === "sending" ? "отправка" : status === "error" ? "ошибка" : edited;
  return document.createTextNode(label ? ` · ${label}` : "");
}
