import { api, type GroupInfo } from "../../api";
import { renderChatListSkeleton, renderMessageSkeleton } from "../../groups/skeletons";
import type { createGroupProfile } from "../../groups/profile";
import type { VoiceRoomController } from "../../voice-room/controller";
import type { MainElements } from "../elements";
import { showDiscover } from "../navigation/views";
import type { AppState } from "../state";
import { isCurrentMember } from "../state";
import type { MessageController } from "../messages/controller";
import { renderDiscoverList, renderGroup, renderGroupList, renderVoiceList } from "./render";

interface GroupsControllerOptions {
  elements: MainElements;
  groupProfile: ReturnType<typeof createGroupProfile>;
  messages: MessageController;
  mobileShell: { closeSidebar: () => void };
  onError: (error: unknown, fallback?: string) => void;
  state: AppState;
  voiceRoom: VoiceRoomController;
}

export interface GroupsController {
  discover: (query?: string) => Promise<void>;
  openGroup: (id: string) => Promise<void>;
  openVoiceList: (push?: boolean) => void;
  refresh: () => Promise<void>;
  renderCurrentGroup: () => void;
  scheduleRefresh: () => void;
}

export function createGroupsController(options: GroupsControllerOptions): GroupsController {
  const { elements, groupProfile, messages, mobileShell, onError, state, voiceRoom } = options;

  function renderList(): void {
    renderGroupList(elements.list, state.groups, (id) => {
      mobileShell.closeSidebar();
      void openGroup(id);
    });
  }

  function renderCurrentGroup(): void {
    if (!state.current) return;
    groupProfile.close();
    renderGroup(elements, state.current, isCurrentMember(state, state.current), joinVoice, deleteVoice, state.user?.id);
  }

  function openVoiceList(push = true): void {
    if (!state.current) return;
    groupProfile.close();
    renderVoiceList(elements, state.current, joinVoice, deleteVoice, isOwner());
    setVoiceCreationVisible();
    const dialog = document.getElementById("group-voice-dialog") as HTMLDialogElement;
    if (!dialog.open) dialog.showModal();
    if (push) history.replaceState({}, "", `/groups/${state.current.id}`);
  }

  async function openGroup(id: string): Promise<void> {
    try {
      renderMessageSkeleton(elements.messages);
      const group = await api.group(id);
      state.current = group;
      renderCurrentGroup();
      renderMessageSkeleton(elements.messages);
      messages.renderMessages(await api.messages(id, messages.messageLimit()));
      history.pushState({}, "", `/groups/${id}`);
    } catch (error) {
      onError(error, "Не удалось загрузить данные");
    }
  }

  async function refresh(): Promise<void> {
    if (!state.user) return;
    if (!state.groups.length) renderChatListSkeleton(elements.list);
    const result = await api.groups();
    state.groups = Array.isArray(result) ? result : [];
    renderList();
    refreshCurrentGroup();
  }

  async function discover(query = ""): Promise<void> {
    try {
      showDiscover(elements);
      history.pushState({}, "", "/discover/");
      const normalized = query.trim();
      if (normalized.length === 1) {
        elements.discover.replaceChildren();
        return;
      }
      renderChatListSkeleton(elements.discover, 6);
      const response = await api.discover(normalized);
      const groups = Array.isArray(response) ? response : [];
      renderDiscoverList(elements.discover, groups, (id) => {
        mobileShell.closeSidebar();
        void openGroup(id);
      });
    } catch (error) {
      onError(error, "Не удалось загрузить данные");
    }
  }

  function scheduleRefresh(): void {
    if (state.refreshTimer !== undefined) return;
    state.refreshTimer = window.setTimeout(() => {
      state.refreshTimer = undefined;
      void refresh().catch((error) => onError(error, "Не удалось обновить группы"));
    }, 250);
  }

  async function joinVoice(id: string): Promise<void> {
    try {
      const room = state.current?.voice_rooms?.find((item) => item.id === id);
      closeVoiceDialog();
      await voiceRoom.openGroup(id, room?.name || "Голосовая комната", state.current?.id);
      history.pushState({}, "", `/group-voice-rooms/${id}/`);
      await refresh();
    } catch (error) {
      onError(error, "Не удалось подключиться");
    }
  }

  async function deleteVoice(id: string): Promise<void> {
    try {
      await api.deleteGroupVoice(id);
      await refresh();
    } catch (error) {
      onError(error, "Не удалось удалить голосовую комнату");
    }
  }

  function refreshCurrentGroup(): void {
    if (!state.current || !isCurrentMember(state, state.current)) return;
    const group = state.groups.find((item) => item.id === state.current?.id);
    if (!group) return;
    state.current = group;
    if ((document.getElementById("group-voice-dialog") as HTMLDialogElement).open) {
      renderVoiceList(elements, group, joinVoice, deleteVoice, isOwner());
      setVoiceCreationVisible();
      return;
    }
    if (document.body.dataset.page !== "voice") renderCurrentGroup();
  }

  function isOwner(): boolean {
    return Boolean(state.current && state.user?.id === state.current.owner.id);
  }

  function setVoiceCreationVisible(): void {
    const button = document.getElementById("new-voice-button") as HTMLButtonElement | null;
    if (button) button.hidden = !isOwner();
  }

  return { discover, openGroup, openVoiceList, refresh, renderCurrentGroup, scheduleRefresh };
}

function closeVoiceDialog(): void {
  const dialog = document.getElementById("group-voice-dialog") as HTMLDialogElement | null;
  if (dialog?.open) dialog.close();
}
