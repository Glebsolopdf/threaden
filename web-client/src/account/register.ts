import { api } from "../api";
import { byId } from "../dom";
import { createNotificationCenter } from "../notifications/controller";
import { errorMessage, validateEmail, validatePassword } from "./auth";

const form = byId<HTMLFormElement>("register-form");
const button = byId<HTMLButtonElement>("register-button");
const notifications = createNotificationCenter();

form.addEventListener("submit", (event) => {
  event.preventDefault();
  setPending(true);
  notifications.clear();
  void (async () => {
    try {
      const password = validatePassword(byId<HTMLInputElement>("register-password"));
      if (password !== byId<HTMLInputElement>("register-password-confirm").value) {
        throw new Error("Пароли не совпадают");
      }
      await api.register(validateEmail(byId<HTMLInputElement>("register-email")), password);
      const next = new URLSearchParams(location.search).get("continue") || "";
      location.assign(next.startsWith("/") && !next.startsWith("//") ? next : "/");
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
  button.textContent = pending ? "Создаём аккаунт…" : "Зарегистрироваться";
}
