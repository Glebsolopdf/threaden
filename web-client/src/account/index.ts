import { ApiError, api, type User } from "../api";
import { byId, showAvatar } from "../dom";
import { displayError } from "../settings/errors";

export interface AccountController {
  readonly buttons: HTMLButtonElement[];
  readonly user: User | null;
  errorMessage(error: unknown): string;
  requireUser(): User;
  restore(): Promise<void>;
}

export function createAccount(options: {
  homeError: HTMLElement;
  gatedButtons: HTMLButtonElement[];
  onLogout?: () => void;
  showMessage: (target: HTMLElement, message?: string, kind?: "error" | "success") => void;
}): AccountController {
  const authCard = byId<HTMLElement>("auth-card");
  const profileCard = byId<HTMLElement>("profile-card");
  const roomActionsCard = byId<HTMLElement>("room-actions-card");
  const settingsButton = byId<HTMLElement>("settings-button");
  const logoutButton = byId<HTMLButtonElement>("logout-button");
  const avatarPreview = byId<HTMLElement>("profile-avatar-preview");
  const profileName = byId<HTMLElement>("profile-name");

  let currentUser: User | null = null;

  function setUser(user: User | null): void {
    currentUser = user;
    authCard.hidden = !!user;
    profileCard.hidden = !user;
    roomActionsCard.hidden = !user;
    settingsButton.hidden = !user;
    for (const button of options.gatedButtons) button.disabled = !user;
    if (!user) return;
    profileName.textContent = user.display_name;
    showAvatar(avatarPreview, user.avatar || "", user.display_name);
  }

  logoutButton.addEventListener("click", () => {
    void api.logout().finally(() => {
      options.onLogout?.();
      setUser(null);
      options.showMessage(options.homeError);
    });
  });

  return {
    buttons: [],
    get user() {
      return currentUser;
    },
    errorMessage(error: unknown): string {
      if (error instanceof ApiError && error.status === 401) {
        setUser(null);
      }
      if (error instanceof ApiError) return displayError(error);
      if (error instanceof Error) return error.message;
      return "Произошла неизвестная ошибка";
    },
    requireUser(): User {
      if (!currentUser) throw new Error("Сначала войдите или зарегистрируйтесь");
      return currentUser;
    },
    async restore(): Promise<void> {
      try {
        setUser(await api.getMe());
      } catch {
        setUser(null);
      }
    },
  };
}
