import { api, type GroupMessage } from "../../api";
import { renderChatMessages, type ChatMessageView, type MessageAnimation } from "../../groups/messages";
import type { MainElements } from "../elements";
import type { AppState } from "../state";
import { isPendingMatch } from "./guards";

interface MessageControllerOptions {
  elements: MainElements;
  onError: (error: unknown, fallback?: string) => void;
  onScheduleRefresh: () => void;
  state: AppState;
}

export interface MessageController {
  mergeMessage: (message: GroupMessage) => boolean;
  messageLimit: () => number;
  renderMessageViews: () => void;
  renderMessages: (messages: GroupMessage[]) => void;
  submitMessage: () => void;
}

export function createMessageController(options: MessageControllerOptions): MessageController {
  const { elements, onError, onScheduleRefresh, state } = options;

  function renderMessageViews(): void {
    renderChatMessages(
      elements.messages,
      [...state.currentMessages.map((message) => ({ message })), ...state.pendingMessages],
      state.user,
    );
  }

  function renderMessages(messages: GroupMessage[]): void {
    state.currentMessages = messages;
    state.pendingMessages.length = 0;
    renderMessageViews();
  }

  function mergeMessage(message: GroupMessage): boolean {
    if (!state.current || message.group_id !== state.current.id) return false;
    if (state.currentMessages.some((item) => item.id === message.id)) return true;
    const pendingIndex = state.pendingMessages.findIndex((item) => isPendingMatch(item.message, message));
    const animate = animationForMessage(message, pendingIndex);
    if (pendingIndex >= 0) state.pendingMessages.splice(pendingIndex, 1);
    state.currentMessages = [...state.currentMessages, message]
      .sort((a, b) => Date.parse(a.created_at) - Date.parse(b.created_at));
    renderChatMessages(
      elements.messages,
      [
        ...state.currentMessages.map((item) => ({ message: item, animate: item.id === message.id ? animate : undefined })),
        ...state.pendingMessages,
      ],
      state.user,
    );
    return true;
  }

  function messageLimit(): number {
    return Math.min(50, Math.max(12, Math.ceil(window.innerHeight / 56) + 6));
  }

  function submitMessage(): void {
    if (!state.current || !elements.input.value.trim() || !state.user) return;
    const body = elements.input.value;
    const pending = createPendingMessage(state.current.id, state.user, body);
    state.pendingMessages.push(pending);
    elements.input.value = "";
    renderMessageViews();
    void api.sendMessage(state.current.id, body)
      .then((message) => {
        const index = state.pendingMessages.indexOf(pending);
        if (index >= 0) state.pendingMessages.splice(index, 1);
        mergeMessage(message);
        onScheduleRefresh();
      })
      .catch((error) => {
        pending.status = "error";
        renderMessageViews();
        onError(error, "Не удалось отправить сообщение");
      });
  }

  function animationForMessage(message: GroupMessage, pendingIndex: number): MessageAnimation | undefined {
    if (pendingIndex >= 0) return "outgoing";
    return message.author.id === state.user?.id ? undefined : "incoming";
  }

  return { mergeMessage, messageLimit, renderMessageViews, renderMessages, submitMessage };
}

function createPendingMessage(groupId: string, author: NonNullable<AppState["user"]>, body: string): ChatMessageView {
  return {
    status: "sending",
    animate: "outgoing",
    message: {
      id: `pending-${Date.now()}`,
      group_id: groupId,
      author,
      body,
      created_at: new Date().toISOString(),
    },
  };
}
