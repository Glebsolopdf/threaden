import { api, type User } from "../api";
import { byId, showAvatar } from "../dom";
import { createNotificationCenter } from "../notifications/controller";
import { clearActiveRoomCode } from "../room/memory";
import { initPersistentRoomWidget } from "../room/persistent-widget";
import { errorMessage } from "./auth";

const form = byId<HTMLFormElement>("profile-form");
const button = byId<HTMLButtonElement>("save-profile");
const nameInput = byId<HTMLInputElement>("display-name");
const avatarFile = byId<HTMLInputElement>("avatar-file");
const avatarPreview = byId<HTMLElement>("profile-avatar-preview");
const deleteAvatarButton = byId<HTMLButtonElement>("delete-avatar");
const deleteProfileButton = byId<HTMLButtonElement>("delete-profile");
const deleteProfileDialog = byId<HTMLDialogElement>("delete-profile-dialog");
const confirmDeleteProfileButton = byId<HTMLButtonElement>("confirm-delete-profile");
const profileEmail = byId<HTMLElement>("profile-email");
const progress = byId<HTMLElement>("upload-progress");
const progressBar = byId<HTMLElement>("upload-progress-bar");
const notifications = createNotificationCenter();

let currentUser: User | null = null;
let selectedAvatar: File | undefined;
let previewURL = "";

function setMessage(text = "", kind: "error" | "success" | "neutral" = "error"): void {
  if (!text) {
    notifications.clear();
    return;
  }
  if (kind === "success") notifications.success(text);
  else if (kind === "neutral") notifications.neutral(text);
  else notifications.error(text);
}

initPersistentRoomWidget((text) => setMessage(text));

function setProgress(value = 0): void {
  progress.hidden = value <= 0;
  progressBar.style.width = `${value}%`;
  avatarPreview.classList.toggle("profile-avatar--uploading", value > 0);
}

function setPreview(value: string, label = nameInput.value): void {
  if (previewURL) URL.revokeObjectURL(previewURL);
  previewURL = "";
  avatarPreview.classList.remove("profile-avatar--pending", "profile-avatar--uploading");
  showAvatar(avatarPreview, value, label);
}

function setUser(user: User): void {
  currentUser = user;
  nameInput.value = user.display_name;
  profileEmail.textContent = user.email;
  setPreview(user.avatar || "", user.display_name);
}

function validateName(): string {
  const name = nameInput.value.trim();
  if ([...name].length < 1 || [...name].length > 50) throw new Error("Введите имя длиной от 1 до 50 символов");
  return name;
}

avatarFile.addEventListener("change", () => {
  const file = avatarFile.files?.[0];
  selectedAvatar = undefined;
  if (!file) return currentUser && setPreview(currentUser.avatar || "");
  const allowed = ["image/png", "image/jpeg", "image/gif", "image/webp"].includes(file.type);
  if (!allowed) {
    avatarFile.value = "";
    avatarPreview.classList.remove("profile-avatar--pending");
    setMessage("Аватар должен быть jpeg, png, gif или webp");
    return;
  }
  selectedAvatar = file;
  previewURL = URL.createObjectURL(file);
  showAvatar(avatarPreview, previewURL, nameInput.value);
  avatarPreview.classList.add("profile-avatar--pending");
  setMessage("Предпросмотр обновлён. Нажмите «Сохранить», чтобы применить.", "neutral");
});

form.addEventListener("submit", (event) => {
  event.preventDefault();
  button.disabled = true;
  setMessage();
  setProgress(selectedAvatar ? 1 : 0);
  void (async () => {
    try {
      setUser(await api.updateProfile(validateName(), selectedAvatar, setProgress));
      selectedAvatar = undefined;
      avatarFile.value = "";
      avatarPreview.classList.remove("profile-avatar--pending");
      setProgress();
      setMessage("Профиль сохранён", "success");
    } catch (error) {
      setProgress();
      setMessage(errorMessage(error));
    } finally {
      button.disabled = false;
    }
  })();
});

deleteAvatarButton.addEventListener("click", () => {
  button.disabled = true;
  deleteAvatarButton.disabled = true;
  setMessage();
  void (async () => {
    try {
      selectedAvatar = undefined;
      avatarFile.value = "";
      setUser(await api.deleteAvatar());
      setMessage("Аватар удалён", "success");
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      button.disabled = false;
      deleteAvatarButton.disabled = false;
    }
  })();
});

deleteProfileButton.addEventListener("click", () => {
  deleteProfileDialog.showModal();
});

deleteProfileDialog.addEventListener("click", (event) => {
  if (event.target === deleteProfileDialog) deleteProfileDialog.close("cancel");
});

deleteProfileDialog.addEventListener("close", () => {
  if (deleteProfileDialog.returnValue !== "confirm") return;
  confirmDeleteProfileButton.disabled = true;
  void api.deleteProfile()
    .then(() => {
      clearActiveRoomCode();
      location.assign("/register/");
    })
    .catch((error) => {
      confirmDeleteProfileButton.disabled = false;
      setMessage(errorMessage(error));
    });
});

api.getMe()
  .then(setUser)
  .catch(() => {
    location.assign("/login/");
  });
