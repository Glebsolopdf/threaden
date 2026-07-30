import type { User } from "../api";
import { byId } from "../dom";
import { createProfilePanel } from "./profile-panel";
import { createSettingsPage } from "./preferences";

type AccountTab = "profile" | "settings" | "customization";

export function createAccountDialog(options: {
  user: () => User | null;
  onUserUpdated: (user: User) => void;
  onError: (message: string) => void;
  onSuccess: (message: string) => void;
  onNeutral: (message: string) => void;
}) {
  const dialog = byId<HTMLDialogElement>("account-dialog");
  const close = byId<HTMLButtonElement>("account-dialog-close");
  const heading = byId<HTMLElement>("account-dialog-heading");
  const profileTab = byId<HTMLButtonElement>("account-tab-profile");
  const settingsTab = byId<HTMLButtonElement>("account-tab-settings");
  const customizationTab = byId<HTMLButtonElement>("account-tab-customization");
  const profilePanel = byId<HTMLElement>("account-profile-panel");
  const settingsPanel = byId<HTMLElement>("account-settings-panel");
  const customizationPanel = byId<HTMLElement>("account-customization-panel");
  const profile = createProfilePanel({
    user: options.user,
    onUserUpdated: options.onUserUpdated,
    onError: options.onError,
    onSuccess: options.onSuccess,
    onNeutral: options.onNeutral,
  });
  const settings = createSettingsPage({
    onBack: () => dialog.close(),
    onError: options.onError,
    onSuccess: options.onSuccess,
  });

  close.onclick = () => dialog.close();
  dialog.addEventListener("click", (event) => { if (event.target === dialog) dialog.close(); });
  profileTab.onclick = () => open("profile");
  settingsTab.onclick = () => open("settings");
  customizationTab.onclick = () => open("customization");

  function open(tab: AccountTab): void {
    if (!dialog.open) dialog.showModal();
    setTab(tab);
  }

  function setTab(tab: AccountTab): void {
    const isProfile = tab === "profile";
    const isSettings = tab === "settings";
    profilePanel.hidden = !isProfile;
    settingsPanel.hidden = !isSettings;
    customizationPanel.hidden = tab !== "customization";
    profileTab.dataset.active = String(isProfile);
    settingsTab.dataset.active = String(isSettings);
    customizationTab.dataset.active = String(tab === "customization");
    heading.textContent = tabHeading(tab);
    if (isProfile) profile.open();
    else settings.open(tab);
  }

  return { open };
}

function tabHeading(tab: AccountTab): string {
  if (tab === "profile") return "Профиль";
  if (tab === "customization") return "Кастомизация";
  return "Настройки";
}
