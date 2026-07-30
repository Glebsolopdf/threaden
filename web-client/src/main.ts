import { ApiError, api, type GroupMessage } from "./api";
import type { User } from "./api";
import "./customization/chrome.css";
import { applyTheme, loadTheme } from "./customization/preferences";
import { showAvatar } from "./dom";
import { createGroupProfile } from "./groups/profile";
import { renderChatListSkeleton } from "./groups/skeletons";
import { createVoiceRoomDialog } from "./groups/voice-dialog";
import { bindAppHandlers, enterTemporaryRoom } from "./main/dialogs/handlers";
import { getMainElements } from "./main/elements";
import { createEventStream } from "./main/events/stream";
import { createGroupsController, type GroupsController } from "./main/groups/controller";
import { groupMeta } from "./main/groups/render";
import { createMessageController } from "./main/messages/controller";
import { parseInitialRoute } from "./main/navigation/routes";
import { setPage, showHome, showVoiceRoom } from "./main/navigation/views";
import { createAppState } from "./main/state";
import { createNotificationCenter } from "./notifications/controller";
import { notifyError } from "./notifications/policy";
import { createAccountDialog } from "./settings/account-dialog";
import { initMobileShell } from "./shell/mobile";
import { VoiceRoomController } from "./voice-room/controller";

void start();

async function start(): Promise<void> {
  applyTheme(loadTheme());
  const elements = getMainElements();
  const state = createAppState();
  const mobileShell = initMobileShell();
  const notifications = createNotificationCenter();
  const onError = (value: unknown = "", fallback?: string) => notifyError(notifications, value, fallback);

  let groups: GroupsController;
  const groupProfile = createGroupProfile({
    user: () => state.user,
    onVoiceRooms: () => groups.openVoiceList(),
    onChanged: (profile) => {
      state.current = profile.group;
      elements.meta.textContent = groupMeta(profile.group);
      groups.scheduleRefresh();
    },
    onDeleted: () => {
      groupProfile.close();
      state.current = null;
      showHome(elements);
      history.pushState({}, "", "/");
      void groups.refresh();
    },
    onError,
  });
  const voiceDialog = createVoiceRoomDialog({
    onCreate: async (name) => {
      if (!state.current) return;
      await api.createGroupVoice(state.current.id, name);
      await groups.openGroup(state.current.id);
    },
    onError: (error) => onError(error, "Произошла ошибка. Попробуйте ещё раз"),
    onNeutral: notifications.neutral,
  });
  const voiceRoom = new VoiceRoomController({
    onOpen: () => {
      showVoiceRoom(elements);
      groupProfile.close();
      closeVoiceDialog();
    },
    onMinimize: () => {
      if (state.current) {
        groups.renderCurrentGroup();
        history.pushState({}, "", `/groups/${state.current.id}`);
        return;
      }
      showHome(elements);
      history.pushState({}, "", "/");
    },
    onClose: () => {
      closeVoiceDialog();
      if (state.current) groups.renderCurrentGroup();
      else showHome(elements);
    },
    onRosterChanged: () => void groups.refresh(),
    onError,
  });
  const accountDialog = createAccountDialog({
    user: () => state.user,
    onUserUpdated: (updated) => {
      state.user = updated;
      renderProfileBar(elements, updated);
    },
    onError,
    onSuccess: notifications.success,
    onNeutral: notifications.neutral,
  });
  const messages = createMessageController({
    elements,
    onError,
    onScheduleRefresh: () => groups.scheduleRefresh(),
    state,
  });
  groups = createGroupsController({
    elements,
    groupProfile,
    messages,
    mobileShell,
    onError,
    state,
    voiceRoom,
  });
  const events = createEventStream({
    onError,
    onMessageCreated: (message: GroupMessage | null) => {
      if (message) messages.mergeMessage(message);
    },
    onRefreshNeeded: () => groups.scheduleRefresh(),
    onStatusChange: (status) => updateConnectionStatus(elements, status.state, status.label),
  });

  bindAppHandlers({
    accountDialog,
    elements,
    groupProfile,
    groups,
    messages,
    mobileShell,
    onError,
    state,
    voiceDialog,
    voiceRoom,
  });
  await bootInitialRoute(elements, state, groups, messages, events.connect, voiceRoom, accountDialog.open, onError);
}

async function bootInitialRoute(
  elements: ReturnType<typeof getMainElements>,
  state: ReturnType<typeof createAppState>,
  groups: GroupsController,
  messages: ReturnType<typeof createMessageController>,
  connectEvents: () => void,
  voiceRoom: VoiceRoomController,
  openAccount: ReturnType<typeof createAccountDialog>["open"],
  onError: (error: unknown, fallback?: string) => void,
): Promise<void> {
  const route = parseInitialRoute();
  setPage(route.isTemporary ? "temporary" : "home");
  renderChatListSkeleton(elements.list);
  try {
    state.user = await api.getMe();
    renderProfileBar(elements, state.user);
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      location.assign(`/login/?continue=${encodeURIComponent(location.pathname)}`);
      return;
    }
    onError(error, "Не удалось загрузить данные");
    return;
  }

  try {
    if (route.activeVoiceId) {
      void groups.refresh().catch((error) => onError(error, "Не удалось загрузить список групп"));
      connectEvents();
      await voiceRoom.restoreSaved(true);
      return;
    }
    await groups.refresh();
    connectEvents();
    if (route.inviteToken) {
      state.inviteToken = route.inviteToken;
      state.current = await api.invite(route.inviteToken);
      groups.renderCurrentGroup();
      messages.renderMessages([]);
      await voiceRoom.restoreSaved(false);
      return;
    }
    if (route.groupId) {
      await groups.openGroup(route.groupId);
      if (route.groupVoiceId) groups.openVoiceList(false);
    }
    if (route.isDiscover) await groups.discover();
    if (route.isSettings) openAccount("settings");
    if (route.temporaryCode) {
      await enterTemporaryRoom(voiceRoom, route.temporaryCode);
      return;
    }
    if (route.isTemporary) temporaryDialog().showModal();
    await voiceRoom.restoreSaved(false);
  } catch (error) {
    onError(error, "Не удалось загрузить список групп");
  }
}

function temporaryDialog(): HTMLDialogElement {
  return document.getElementById("temporary-dialog") as HTMLDialogElement;
}

function closeVoiceDialog(): void {
  const dialog = document.getElementById("group-voice-dialog") as HTMLDialogElement | null;
  if (dialog?.open) dialog.close();
}

function renderProfileBar(elements: ReturnType<typeof getMainElements>, user: User): void {
  showAvatar(elements.profileAvatar, user.avatar || "", user.display_name);
  elements.profileName.textContent = user.display_name;
}

function updateConnectionStatus(
  elements: ReturnType<typeof getMainElements>,
  state: "good" | "reconnecting" | "lost",
  label: string,
): void {
  elements.connectionStatus.dataset.state = state;
  elements.connectionStatus.setAttribute("aria-label", label);
}
