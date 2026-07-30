import { byId } from "../dom";

let stableViewportHeight = 0;

export function initMobileShell(): { closeSidebar: () => void } {
  const button = byId<HTMLButtonElement>("mobile-sidebar-button");
  const backdrop = byId<HTMLElement>("mobile-sidebar-backdrop");
  const closeSidebar = () => setSidebarOpen(button, backdrop, false);
  const toggleSidebar = () => setSidebarOpen(button, backdrop, document.body.dataset.sidebarOpen !== "true");

  button.onclick = toggleSidebar;
  backdrop.onclick = closeSidebar;
  window.addEventListener("keydown", (event) => {
    if (event.key === "Escape") closeSidebar();
  });
  window.addEventListener("resize", closeSidebar);
  updateViewportHeight();
  window.visualViewport?.addEventListener("resize", updateViewportHeight);
  window.visualViewport?.addEventListener("scroll", updateViewportHeight);
  window.addEventListener("orientationchange", resetViewportHeight);

  return { closeSidebar };
}

function setSidebarOpen(button: HTMLButtonElement, backdrop: HTMLElement, open: boolean): void {
  document.body.dataset.sidebarOpen = String(open);
  button.setAttribute("aria-expanded", String(open));
  backdrop.hidden = !open;
}

function resetViewportHeight(): void {
  stableViewportHeight = 0;
  updateViewportHeight();
}

function updateViewportHeight(): void {
  const height = Math.round(window.visualViewport?.height || window.innerHeight);
  const keyboardLikelyOpen = isTextInputFocused() && height < stableViewportHeight - 120;
  if (!keyboardLikelyOpen) stableViewportHeight = Math.max(stableViewportHeight, height);
  document.documentElement.style.setProperty("--app-height", `${stableViewportHeight || height}px`);
}

function isTextInputFocused(): boolean {
  const element = document.activeElement;
  if (!(element instanceof HTMLElement)) return false;
  return element.matches("input, textarea, [contenteditable='true']");
}
