import type { GroupMessage } from "../../api";
import { isGroupMessage } from "../messages/guards";
import { parseEventPayload } from "./payload";

export interface EventStreamOptions {
  onError: (error: unknown, fallback?: string) => void;
  onMessageCreated: (message: GroupMessage) => void;
  onRefreshNeeded: () => void;
  onStatusChange: (status: ConnectionStatus) => void;
}

export type ConnectionStatus =
  | { state: "good"; label: string }
  | { state: "reconnecting"; label: string }
  | { state: "lost"; label: string };

const MAX_RECONNECT_DELAY_MS = 60_000;
const RECONNECT_STEP_MS = 5_000;

export function createEventStream(options: EventStreamOptions): { connect: () => void } {
  let reconnectDelay = 0;
  let connecting = false;

  function connect(): void {
    if (connecting) return;
    connecting = true;
    if (reconnectDelay > 0) {
      options.onStatusChange({ state: "reconnecting", label: "Пытаемся восстановить соединение" });
    }
    void streamEvents().catch((error) => {
      reconnectDelay = nextReconnectDelay(reconnectDelay);
      options.onStatusChange({
        state: "lost",
        label: `Соединение потеряно. Повтор через ${Math.round(reconnectDelay / 1000)} сек.`,
      });
      options.onError(error, "Соединение потеряно");
      window.setTimeout(connect, reconnectDelay);
    }).finally(() => {
      connecting = false;
    });
  }

  async function streamEvents(): Promise<void> {
    const response = await fetch("/v1/events", { credentials: "include" });
    const reader = response.body?.getReader();
    if (!response.ok || !reader) throw new Error("events unavailable");
    reconnectDelay = 0;
    options.onStatusChange({ state: "good", label: "Хорошее соединение" });
    await readEvents(reader);
  }

  async function readEvents(reader: ReadableStreamDefaultReader<Uint8Array>): Promise<void> {
    const decoder = new TextDecoder();
    let text = "";
    while (true) {
      const chunk = await reader.read();
      if (chunk.done) throw new Error("events closed");
      text += decoder.decode(chunk.value, { stream: true });
      while (text.includes("\n\n")) {
        const index = text.indexOf("\n\n");
        handleEvent(text.slice(0, index));
        text = text.slice(index + 2);
      }
    }
  }

  function handleEvent(raw: string): void {
    if (!raw.startsWith("event:")) return;
    const message = readMessagePayload(raw);
    if (message) {
      options.onMessageCreated(message);
      options.onRefreshNeeded();
      return;
    }
    options.onRefreshNeeded();
  }

  return { connect };
}

function nextReconnectDelay(current: number): number {
  return Math.min(MAX_RECONNECT_DELAY_MS, current + RECONNECT_STEP_MS);
}

function readMessagePayload(raw: string): GroupMessage | null {
  const payload = parseEventPayload(raw);
  if (payload?.type === "message_created" && isGroupMessage(payload.data)) return payload.data;
  return null;
}
