import type { MainElements } from "../elements";

type AppPage = "home" | "group" | "group-voice" | "discover" | "voice" | "temporary";

export function setPage(name: AppPage): void {
  document.body.dataset.page = name;
}

export function showHome(elements: MainElements): void {
  setPage("home");
  elements.group.hidden = true;
  elements.discoverView.hidden = true;
  elements.settings.hidden = true;
  elements.empty.hidden = false;
}

export function showGroup(elements: MainElements): void {
  setPage("group");
  elements.empty.hidden = true;
  elements.discoverView.hidden = true;
  elements.settings.hidden = true;
  elements.group.hidden = false;
}

export function showGroupVoice(elements: MainElements): void {
  setPage("group-voice");
  elements.empty.hidden = true;
  elements.discoverView.hidden = true;
  elements.settings.hidden = true;
  elements.group.hidden = true;
}

export function showDiscover(elements: MainElements): void {
  setPage("discover");
  elements.group.hidden = true;
  elements.settings.hidden = true;
  elements.empty.hidden = true;
  elements.discoverView.hidden = false;
}

export function showVoiceRoom(elements: MainElements): void {
  setPage("voice");
  elements.group.hidden = true;
  elements.discoverView.hidden = true;
  elements.settings.hidden = true;
  elements.empty.hidden = true;
}
