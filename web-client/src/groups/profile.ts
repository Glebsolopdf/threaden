import { api, type GroupProfile, type User } from "../api";
import "../customization/styles.css";
import { byId, showAvatar } from "../dom";
import { displayError } from "../settings/errors";

type GroupProfileMember = GroupProfile["members"][number];

export function createGroupProfile(options: {
  user: () => User | null;
  onVoiceRooms: () => void;
  onChanged: (profile: GroupProfile) => void;
  onDeleted: () => void;
  onError: (message: string) => void;
}) {
  const dialog = byId<HTMLDialogElement>("group-info-dialog");
  const avatar = byId<HTMLElement>("group-profile-avatar");
  const name = byId<HTMLElement>("group-info-name");
  const meta = byId<HTMLElement>("group-profile-meta");
  const inviteRow = byId<HTMLElement>("group-invite-row");
  const inviteLink = byId<HTMLAnchorElement>("group-invite-link");
  const copyInvite = byId<HTMLButtonElement>("copy-group-invite");
  const members = byId<HTMLUListElement>("group-members");
  const deleteButton = byId<HTMLButtonElement>("delete-group-button");
  const leaveButton = byId<HTMLButtonElement>("leave-group-button");
  const closeButton = byId<HTMLButtonElement>("group-info-close");
  const voiceButton = byId<HTMLButtonElement>("group-info-voice");
  const deleteDialog = byId<HTMLDialogElement>("delete-group-dialog");
  const confirm = byId<HTMLButtonElement>("confirm-delete-group");
  let profile: GroupProfile | null = null;

  closeButton.addEventListener("click", () => dialog.close());
  voiceButton.addEventListener("click", () => {
    dialog.close();
    options.onVoiceRooms();
  });
  deleteButton.addEventListener("click", () => deleteDialog.showModal());
  leaveButton.addEventListener("click", () => void leave());
  copyInvite.addEventListener("click", () => void copyInviteLink());
  dialog.addEventListener("click", (event) => { if (event.target === dialog) dialog.close(); });
  deleteDialog.addEventListener("click", (event) => { if (event.target === deleteDialog) deleteDialog.close(); });
  confirm.addEventListener("click", () => void removeGroup());

  function render(value: GroupProfile): void {
    profile = value;
    showAvatar(avatar, value.group.avatar, value.group.name);
    name.textContent = value.group.name;
    meta.textContent = `${value.members.length} ${pluralMembers(value.members.length)}`;
    renderInvite(value);
    members.replaceChildren(...value.members.map(memberRow));
    const owner = value.group.owner.id === options.user()?.id;
    const member = value.members.some((item) => item.id === options.user()?.id);
    deleteButton.hidden = !owner;
    leaveButton.hidden = owner || !member;
  }

  function renderInvite(value: GroupProfile): void {
    const token = value.group.invite_token || "";
    const hasInvite = token.length > 0;
    inviteRow.hidden = !hasInvite;
    if (!hasInvite) return;
    const url = new URL(`/invite/${token}`, location.origin).toString();
    inviteLink.href = url;
    inviteLink.textContent = url;
    copyInvite.setAttribute("aria-label", "Скопировать приглашение");
    copyInvite.title = "Скопировать";
  }

  function memberRow(member: GroupProfileMember): HTMLLIElement {
    const item = document.createElement("li");
    const memberAvatar = document.createElement("span");
    const details = document.createElement("span");
    const displayName = document.createElement("strong");
    const role = document.createElement("small");

    item.className = "group-member";
    memberAvatar.className = "avatar";
    showAvatar(memberAvatar, member.avatar || "", member.display_name);
    displayName.textContent = member.display_name;
    role.textContent = member.role === "owner" ? "Владелец" : "Участник";
    details.append(displayName, role);
    item.append(memberAvatar, details);
    maybeAppendRemoveButton(item, member);
    return item;
  }

  function maybeAppendRemoveButton(item: HTMLLIElement, member: GroupProfileMember): void {
    const current = options.user();
    if (!profile || !current || profile.group.owner.id !== current.id) return;
    if (member.id === current.id || member.id === profile.group.owner.id) return;
    const button = document.createElement("button");
    button.className = "group-member__remove";
    button.type = "button";
    button.textContent = "Удалить";
    button.setAttribute("aria-label", `Удалить ${member.display_name} из группы`);
    button.addEventListener("click", () => void removeMember(member, button));
    item.append(button);
  }

  async function removeMember(member: GroupProfileMember, button: HTMLButtonElement): Promise<void> {
    if (!profile) return;
    button.disabled = true;
    try {
      const updated = await api.removeGroupMember(profile.group.id, member.id);
      render(updated);
      options.onChanged(updated);
    } catch (error) {
      options.onError(displayError(error, "Не удалось удалить участника"));
    } finally {
      button.disabled = false;
    }
  }

  async function removeGroup(): Promise<void> {
    if (!profile) return;
    confirm.disabled = true;
    try {
      await api.deleteGroup(profile.group.id);
      deleteDialog.close();
      dialog.close();
      options.onDeleted();
    } catch (error) {
      options.onError(displayError(error, "Не удалось удалить группу"));
    } finally {
      confirm.disabled = false;
    }
  }

  async function leave(): Promise<void> {
    if (!profile) return;
    leaveButton.disabled = true;
    try {
      await api.leaveGroup(profile.group.id);
      dialog.close();
      options.onDeleted();
    } catch (error) {
      options.onError(displayError(error, "Не удалось покинуть группу"));
    } finally {
      leaveButton.disabled = false;
    }
  }

  async function copyInviteLink(): Promise<void> {
    if (!inviteLink.href) return;
    try {
      await navigator.clipboard.writeText(inviteLink.href);
      copyInvite.setAttribute("aria-label", "Приглашение скопировано");
      copyInvite.title = "Скопировано";
    } catch {
      inviteLink.focus();
      copyInvite.setAttribute("aria-label", "Не удалось скопировать приглашение");
      copyInvite.title = "Ссылка выделена";
    }
  }

  return {
    async open(groupID: string): Promise<void> {
      render(await api.groupProfile(groupID));
      if (!dialog.open) dialog.showModal();
    },
    close(): void {
      if (dialog.open) dialog.close();
    },
  };
}

function pluralMembers(count: number): string {
  if (count % 10 === 1 && count % 100 !== 11) return "участник";
  if ([2, 3, 4].includes(count % 10) && ![12, 13, 14].includes(count % 100)) return "участника";
  return "участников";
}
