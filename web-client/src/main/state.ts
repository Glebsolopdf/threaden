import type { GroupInfo, GroupMessage, User } from "../api";
import type { ChatMessageView } from "../groups/messages";

export interface AppState {
  user: User | null;
  current: GroupInfo | null;
  groups: GroupInfo[];
  inviteToken: string;
  currentMessages: GroupMessage[];
  pendingMessages: ChatMessageView[];
  refreshTimer: number | undefined;
}

export function createAppState(): AppState {
  return {
    user: null,
    current: null,
    groups: [],
    inviteToken: "",
    currentMessages: [],
    pendingMessages: [],
    refreshTimer: undefined,
  };
}

export function isCurrentMember(state: AppState, group: GroupInfo): boolean {
  return state.groups.some((item) => item.id === group.id);
}
