import { api } from "../api";
import { byId } from "../dom";
import { createNotificationCenter } from "../notifications/controller";
import { errorMessage, validateEmail, validatePassword } from "./auth";

const form = byId<HTMLFormElement>("login-form");
const button = byId<HTMLButtonElement>("login-button");
const notifications = createNotificationCenter();

form.addEventListener("submit", (event) => {
  event.preventDefault();
  setPending(true);
  notifications.clear();
  void (async () => {
    try {
      await api.login(
        validateEmail(byId<HTMLInputElement>("login-email")),
        validatePassword(byId<HTMLInputElement>("login-password"), 1),
      );
      location.assign(continuePath() || "/");
    } catch (error) {
      notifications.error(errorMessage(error));
    } finally {
      setPending(false);
    }
  })();
});

function setPending(pending: boolean): void {
  button.disabled = pending;
  form.setAttribute("aria-busy", String(pending));
  button.textContent = pending ? "Входим…" : "Войти";
}

function continuePath(): string { const value = new URLSearchParams(location.search).get("continue") || ""; return value.startsWith("/") && !value.startsWith("//") ? value : ""; }
