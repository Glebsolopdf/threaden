import type { GroupMessage } from "../../api";

export function isGroupMessage(value: unknown): value is GroupMessage {
  const item = value as Partial<GroupMessage> | undefined;
  return !!item
    && typeof item.id === "string"
    && typeof item.group_id === "string"
    && typeof item.body === "string"
    && typeof item.created_at === "string"
    && !!item.author
    && typeof item.author.id === "string";
}

export function isPendingMatch(pending: GroupMessage, incoming: GroupMessage): boolean {
  return pending.author.id === incoming.author.id
    && pending.body === incoming.body
    && Math.abs(Date.parse(incoming.created_at) - Date.parse(pending.created_at)) < 30000;
}
