import { api } from "../../api";
import { byId } from "../../dom";
import type { createGroupProfile } from "../../groups/profile";
import type { createVoiceRoomDialog } from "../../groups/voice-dialog";
import type { createAccountDialog } from "../../settings/account-dialog";
import type { VoiceRoomController } from "../../voice-room/controller";
import type { MainElements } from "../elements";
import type { GroupsController } from "../groups/controller";
import type { MessageController } from "../messages/controller";
import { showHome } from "../navigation/views";
import type { AppState } from "../state";

interface DialogHandlerOptions {
  accountDialog: ReturnType<typeof createAccountDialog>;
  elements: MainElements;
  groupProfile: ReturnType<typeof createGroupProfile>;
  groups: GroupsController;
  messages: MessageController;
  mobileShell: { closeSidebar: () => void };
  onError: (error: unknown, fallback?: string) => void;
  state: AppState;
  voiceDialog: ReturnType<typeof createVoiceRoomDialog>;
  voiceRoom: VoiceRoomController;
}

export function bindAppHandlers(options: DialogHandlerOptions): void {
  const { accountDialog, elements, groupProfile, groups, messages, mobileShell, onError, state, voiceDialog, voiceRoom } = options;

  byId<HTMLButtonElement>("discover-button").onclick = () => void groups.discover();
  byId<HTMLButtonElement>("discover-back").onclick = () => navigateHome(elements);
  byId<HTMLButtonElement>("settings-button").onclick = () => openSettings(mobileShell, accountDialog);
  elements.profileBar.onclick = () => openProfile(mobileShell, accountDialog);
  byId<HTMLButtonElement>("group-menu-button").onclick = () => byId<HTMLDialogElement>("create-dialog").showModal();
  byId<HTMLButtonElement>("group-voice-button").onclick = () => groups.openVoiceList();
  byId<HTMLButtonElement>("group-voice-back").onclick = () => byId<HTMLDialogElement>("group-voice-dialog").close();
  byId<HTMLButtonElement>("group-back-button").onclick = () => navigateHome(elements);
  byId<HTMLButtonElement>("temporary-button").onclick = () => byId<HTMLDialogElement>("temporary-dialog").showModal();
  byId<HTMLButtonElement>("cancel-create-group").onclick = () => byId<HTMLDialogElement>("create-dialog").close();
  byId<HTMLButtonElement>("cancel-temporary").onclick = () => byId<HTMLDialogElement>("temporary-dialog").close();
  byId<HTMLButtonElement>("create-temporary").onclick = () => void createTemporaryRoom(voiceRoom, onError);
  byId<HTMLButtonElement>("new-voice-button").onclick = () => voiceDialog.open();
  byId<HTMLInputElement>("discover-search").oninput = (event) => void groups.discover((event.target as HTMLInputElement).value);

  elements.groupMenuButton.onclick = () => toggleGroupMenu(elements);
  elements.groupSettings.onclick = () => openGroupSettings(elements, groupProfile, state, onError);
  elements.join.onclick = () => joinCurrentGroup(elements, groups, state, onError);
  elements.form.onsubmit = (event) => {
    event.preventDefault();
    messages.submitMessage();
  };
  byId<HTMLFormElement>("temporary-room-form").onsubmit = (event) => submitTemporaryRoom(event, voiceRoom, onError);
  byId<HTMLFormElement>("create-group-form").onsubmit = (event) => submitCreateGroup(event, groups, onError);
  document.getElementById("logout-button")?.addEventListener("click", () => {
    void api.logout().finally(() => location.assign("/"));
  });
  document.addEventListener("click", (event) => {
    if (!elements.groupMenu.contains(event.target as Node) && event.target !== elements.groupMenuButton) closeGroupMenu(elements);
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") closeGroupMenu(elements);
  });
}

export async function enterTemporaryRoom(voiceRoom: VoiceRoomController, code: string): Promise<void> {
  await voiceRoom.openTemporary(code);
  history.pushState({}, "", `/temporary/${code}/`);
}

function navigateHome(elements: MainElements): void {
  showHome(elements);
  history.pushState({}, "", "/");
}

function openSettings(mobileShell: { closeSidebar: () => void }, accountDialog: ReturnType<typeof createAccountDialog>): void {
  mobileShell.closeSidebar();
  accountDialog.open("settings");
}

function openProfile(mobileShell: { closeSidebar: () => void }, accountDialog: ReturnType<typeof createAccountDialog>): void {
  mobileShell.closeSidebar();
  accountDialog.open("profile");
}

function toggleGroupMenu(elements: MainElements): void {
  const open = elements.groupMenu.hidden;
  elements.groupMenu.hidden = !open;
  elements.groupMenuButton.setAttribute("aria-expanded", String(open));
  if (open) elements.groupSettings.focus();
}

function closeGroupMenu(elements: MainElements): void {
  elements.groupMenu.hidden = true;
  elements.groupMenuButton.setAttribute("aria-expanded", "false");
}

function openGroupSettings(
  elements: MainElements,
  groupProfile: ReturnType<typeof createGroupProfile>,
  state: AppState,
  onError: (error: unknown, fallback?: string) => void,
): void {
  closeGroupMenu(elements);
  if (!state.current) return;
  void groupProfile.open(state.current.id).catch((error) => onError(error, "Не удалось открыть настройки группы"));
}

function joinCurrentGroup(
  elements: MainElements,
  groups: GroupsController,
  state: AppState,
  onError: (error: unknown, fallback?: string) => void,
): void {
  if (!state.current) return;
  elements.join.disabled = true;
  elements.join.dataset.state = "loading";
  elements.join.textContent = "Присоединяем...";
  void (state.inviteToken ? api.joinInvite(state.inviteToken) : api.joinGroup(state.current.id))
    .then(() => groups.refresh().then(() => groups.openGroup(state.current!.id)))
    .catch((error) => {
      elements.join.disabled = false;
      elements.join.dataset.state = "idle";
      elements.join.textContent = "Присоединиться к группе";
      onError(error, "Произошла ошибка. Попробуйте ещё раз");
    });
}

function submitTemporaryRoom(
  event: SubmitEvent,
  voiceRoom: VoiceRoomController,
  onError: (error: unknown, fallback?: string) => void,
): void {
  event.preventDefault();
  const code = byId<HTMLInputElement>("temporary-code").value.trim().toUpperCase();
  byId<HTMLDialogElement>("temporary-dialog").close();
  void enterTemporaryRoom(voiceRoom, code).catch((error) => onError(error, "Не удалось подключиться"));
}

function submitCreateGroup(
  event: SubmitEvent,
  groups: GroupsController,
  onError: (error: unknown, fallback?: string) => void,
): void {
  event.preventDefault();
  void api.createGroup({
    name: byId<HTMLInputElement>("create-name").value,
    avatar: "",
    visibility: byId<HTMLSelectElement>("create-visibility").value,
  })
    .then((group) => {
      byId<HTMLDialogElement>("create-dialog").close();
      return groups.refresh().then(() => groups.openGroup(group.id));
    })
    .catch((error) => onError(error, "Произошла ошибка. Попробуйте ещё раз"));
}

async function createTemporaryRoom(
  voiceRoom: VoiceRoomController,
  onError: (error: unknown, fallback?: string) => void,
): Promise<void> {
  try {
    const room = await api.createRoom();
    byId<HTMLDialogElement>("temporary-dialog").close();
    await enterTemporaryRoom(voiceRoom, room.code);
  } catch (error) {
    onError(error, "Не удалось создать комнату");
  }
}
