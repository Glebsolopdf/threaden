import { byId } from "../dom";

export interface MainElements {
  list: HTMLElement;
  discover: HTMLElement;
  group: HTMLElement;
  discoverView: HTMLElement;
  settings: HTMLElement;
  empty: HTMLElement;
  name: HTMLElement;
  meta: HTMLElement;
  voiceButton: HTMLButtonElement;
  voiceStrip: HTMLElement;
  voiceTitle: HTMLElement;
  voiceMeta: HTMLElement;
  messages: HTMLElement;
  form: HTMLFormElement;
  input: HTMLInputElement;
  join: HTMLButtonElement;
  voiceList: HTMLElement;
  profileBar: HTMLButtonElement;
  profileAvatar: HTMLElement;
  profileName: HTMLElement;
  connectionStatus: HTMLElement;
  groupMenuButton: HTMLButtonElement;
  groupMenu: HTMLElement;
  groupSettings: HTMLButtonElement;
}

export function getMainElements(): MainElements {
  return {
    list: byId<HTMLElement>("group-list"),
    discover: byId<HTMLElement>("discover-list"),
    group: byId<HTMLElement>("group-view"),
    discoverView: byId<HTMLElement>("discover-view"),
    settings: byId<HTMLElement>("settings-view"),
    empty: byId<HTMLElement>("empty-state"),
    name: byId<HTMLElement>("group-name"),
    meta: byId<HTMLElement>("group-meta"),
    voiceButton: byId<HTMLButtonElement>("group-voice-button"),
    voiceStrip: byId<HTMLElement>("group-voice-strip"),
    voiceTitle: byId<HTMLElement>("group-voice-title"),
    voiceMeta: byId<HTMLElement>("group-voice-meta"),
    messages: byId<HTMLElement>("message-list"),
    form: byId<HTMLFormElement>("message-form"),
    input: byId<HTMLInputElement>("message-input"),
    join: byId<HTMLButtonElement>("join-group-button"),
    voiceList: byId<HTMLElement>("voice-list"),
    profileBar: byId<HTMLButtonElement>("profile-bar"),
    profileAvatar: byId<HTMLElement>("profile-bar-avatar"),
    profileName: byId<HTMLElement>("profile-bar-name"),
    connectionStatus: byId<HTMLElement>("connection-status"),
    groupMenuButton: byId<HTMLButtonElement>("group-menu-actions-button"),
    groupMenu: byId<HTMLElement>("group-menu-actions"),
    groupSettings: byId<HTMLButtonElement>("group-settings-button"),
  };
}
