import type { NotificationCenter, NotificationKind, NotificationOptions } from "./types";

const DEFAULT_REGION_ID = "status-notifications";
const SUCCESS_TIMEOUT_MS = 5_000;
const ERROR_TIMEOUT_MS = 10_000;
const EXIT_TIMEOUT_MS = 180;

interface ActiveNotification {
  element: HTMLElement;
  timer: number;
  exitTimer?: number;
}

export function createNotificationCenter(region = ensureRegion()): NotificationCenter {
  let active: ActiveNotification | null = null;

  function clear(): void {
    dismiss(false);
  }

  function dismiss(animated: boolean): void {
    if (!active) return;
    const item = active;
    active = null;
    window.clearTimeout(item.timer);
    if (!animated) {
      removeNotification(region, item);
      return;
    }
    item.element.dataset.state = "closing";
    item.element.addEventListener("animationend", () => removeNotification(region, item), { once: true });
    item.exitTimer = window.setTimeout(() => removeNotification(region, item), EXIT_TIMEOUT_MS + 80);
  }

  function show(options: NotificationOptions): void {
    const text = options.message.trim();
    dismiss(false);
    if (!text) return;

    const kind = options.kind ?? "neutral";
    const element = renderNotification(text, kind, timeoutFor(kind), clear);
    region.append(element);
    showInTopLayer(region);
    const timer = window.setTimeout(() => dismiss(true), timeoutFor(kind));
    active = { element, timer };
  }

  return {
    show,
    success: (message) => show({ message, kind: "success" }),
    error: (message) => show({ message, kind: "error" }),
    neutral: (message) => show({ message, kind: "neutral" }),
    clear,
  };
}

function ensureRegion(): HTMLElement {
  const existing = document.getElementById(DEFAULT_REGION_ID);
  if (existing) return existing;
  const region = document.createElement("section");
  region.id = DEFAULT_REGION_ID;
  region.className = "notification-region";
  region.setAttribute("popover", "manual");
  region.setAttribute("aria-live", "polite");
  region.setAttribute("aria-label", "Статусные уведомления");
  document.body.append(region);
  return region;
}

function renderNotification(
  message: string,
  kind: NotificationKind,
  durationMs: number,
  onDismiss: () => void,
): HTMLElement {
  const element = document.createElement("article");
  element.className = "status-notification";
  element.dataset.kind = kind;
  element.style.setProperty("--notification-duration", `${durationMs}ms`);
  element.setAttribute("role", kind === "error" ? "alert" : "status");
  if (kind !== "error") {
    element.tabIndex = 0;
    element.title = "Свернуть уведомление";
    element.addEventListener("click", onDismiss);
    element.addEventListener("keydown", (event) => {
      if (event.key === "Enter" || event.key === " ") onDismiss();
    });
  }

  const text = document.createElement("p");
  text.textContent = message;
  element.append(text, document.createElement("span"));
  return element;
}

function timeoutFor(kind: NotificationKind): number {
  return kind === "error" ? ERROR_TIMEOUT_MS : SUCCESS_TIMEOUT_MS;
}

function removeNotification(region: HTMLElement, item: ActiveNotification): void {
  window.clearTimeout(item.timer);
  if (item.exitTimer !== undefined) window.clearTimeout(item.exitTimer);
  item.element.remove();
  if (!region.childElementCount) hideFromTopLayer(region);
}

function showInTopLayer(element: HTMLElement): void {
  if ("showPopover" in element && !element.matches(":popover-open")) element.showPopover();
}

function hideFromTopLayer(element: HTMLElement): void {
  if ("hidePopover" in element && element.matches(":popover-open")) element.hidePopover();
}
