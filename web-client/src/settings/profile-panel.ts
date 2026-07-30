import { ApiError, api, type User } from "../api";
import { byId, showAvatar } from "../dom";
import { clearActiveRoomCode } from "../room/memory";
import { displayError } from "./errors";

export function createProfilePanel(options: {
  user: () => User | null;
  onUserUpdated: (user: User) => void;
  onError: (message: string) => void;
  onSuccess: (message: string) => void;
  onNeutral: (message: string) => void;
}) {
  const form = byId<HTMLFormElement>("account-profile-form");
  const save = byId<HTMLButtonElement>("account-save-profile");
  const name = byId<HTMLInputElement>("account-display-name");
  const file = byId<HTMLInputElement>("account-avatar-file");
  const avatar = byId<HTMLElement>("account-profile-avatar");
  const email = byId<HTMLElement>("account-profile-email");
  const deleteAvatar = byId<HTMLButtonElement>("account-delete-avatar");
  const logout = byId<HTMLButtonElement>("account-logout");
  const deleteProfile = byId<HTMLButtonElement>("account-delete-profile");
  const dialog = byId<HTMLDialogElement>("delete-profile-dialog");
  const confirm = byId<HTMLButtonElement>("confirm-delete-profile");
  const progress = byId<HTMLElement>("account-upload-progress");
  const bar = byId<HTMLElement>("account-upload-progress-bar");
  let selectedAvatar: File | undefined;
  let previewURL = "";

  file.onchange = () => pickAvatar();
  form.onsubmit = (event) => { event.preventDefault(); void saveProfile(); };
  deleteAvatar.onclick = () => void removeAvatar();
  logout.onclick = () => logoutAccount();
  deleteProfile.onclick = () => dialog.showModal();
  dialog.addEventListener("click", (event) => { if (event.target === dialog) dialog.close("cancel"); });
  dialog.addEventListener("close", () => { if (dialog.returnValue === "confirm") void removeProfile(); });

  function open(): void {
    const user = options.user();
    if (!user) return;
    setUser(user);
  }

  function setUser(user: User): void {
    name.value = user.display_name;
    email.textContent = user.email;
    setPreview(user.avatar || "", user.display_name);
  }

  function setProgress(value = 0): void {
    progress.hidden = value <= 0;
    bar.style.width = `${value}%`;
    avatar.classList.toggle("profile-avatar--uploading", value > 0);
  }

  function setPreview(value: string, label = name.value): void {
    if (previewURL) URL.revokeObjectURL(previewURL);
    previewURL = "";
    avatar.classList.remove("profile-avatar--pending", "profile-avatar--uploading");
    showAvatar(avatar, value, label);
  }

  function pickAvatar(): void {
    const picked = file.files?.[0];
    selectedAvatar = undefined;
    if (!picked) {
      const user = options.user();
      if (user) setPreview(user.avatar || "");
      return;
    }
    if (!["image/png", "image/jpeg", "image/gif", "image/webp"].includes(picked.type)) {
      file.value = "";
      avatar.classList.remove("profile-avatar--pending");
      options.onError("Аватар должен быть jpeg, png, gif или webp");
      return;
    }
    selectedAvatar = picked;
    previewURL = URL.createObjectURL(picked);
    showAvatar(avatar, previewURL, name.value);
    avatar.classList.add("profile-avatar--pending");
    options.onNeutral("Предпросмотр обновлён. Нажмите «Сохранить», чтобы применить.");
  }

  async function saveProfile(): Promise<void> {
    save.disabled = true;
    setProgress(selectedAvatar ? 1 : 0);
    try {
      const updated = await api.updateProfile(validateName(name), selectedAvatar, setProgress);
      selectedAvatar = undefined;
      file.value = "";
      options.onUserUpdated(updated);
      setUser(updated);
      setProgress();
      options.onSuccess("Профиль сохранён");
    } catch (error) {
      setProgress();
      options.onError(profileError(error));
    } finally {
      save.disabled = false;
    }
  }

  async function removeAvatar(): Promise<void> {
    deleteAvatar.disabled = true;
    try {
      selectedAvatar = undefined;
      file.value = "";
      const updated = await api.deleteAvatar();
      options.onUserUpdated(updated);
      setUser(updated);
      options.onSuccess("Аватар удалён");
    } catch (error) {
      options.onError(profileError(error));
    } finally {
      deleteAvatar.disabled = false;
    }
  }

  async function removeProfile(): Promise<void> {
    confirm.disabled = true;
    try {
      await api.deleteProfile();
      clearActiveRoomCode();
      location.assign("/register/");
    } catch (error) {
      confirm.disabled = false;
      options.onError(profileError(error));
    }
  }

  function logoutAccount(): void {
    void api.logout().finally(() => {
      clearActiveRoomCode();
      location.assign("/login/");
    });
  }

  return { open };
}

function validateName(input: HTMLInputElement): string {
  const value = input.value.trim();
  if ([...value].length < 1 || [...value].length > 50) throw new Error("Введите имя длиной от 1 до 50 символов");
  return value;
}

function profileError(error: unknown): string {
  if (error instanceof ApiError) return displayError(error);
  return error instanceof Error ? error.message : "Произошла ошибка. Попробуйте ещё раз";
}
