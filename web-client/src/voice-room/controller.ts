import { api } from "../api";
import { byId } from "../dom";
import { clearActiveRoomCode, loadActiveRoom, saveActiveRoom } from "../room/memory";
import { createRoomWidget } from "../room/widget";
import { loadAudioPreferences } from "../settings/preferences";
import { type VoiceParticipant, type VoiceStatus, VoiceClient } from "../voice";
import { populateDevices, setDeviceMenuOpen } from "./devices";
import { renderParticipantCards } from "./participants";
import { applyRoster, loadVoiceRoster, type VoiceRoster } from "./roster";
import { copyRoomCode, errorMessage, pingLabel, renderRoomCode, statusText } from "./status";

type RoomKind = "group" | "temporary";

interface ActiveRoom {
  id: string;
  kind: RoomKind;
  title: string;
  groupId?: string;
}

interface VoiceRoomControllerOptions {
  onClose: () => void;
  onMinimize: () => void;
  onOpen: () => void;
  onRosterChanged: () => void;
  onError: (message: string) => void;
}

export class VoiceRoomController {
  private readonly client = new VoiceClient(byId("audio-container"));
  private readonly view = byId<HTMLElement>("voice-room-view");
  private readonly title = byId<HTMLElement>("voice-room-title");
  private readonly kicker = byId<HTMLElement>("voice-room-kicker");
  private readonly status = byId<HTMLElement>("voice-room-status");
  private readonly detail = byId<HTMLElement>("voice-room-detail");
  private readonly count = byId<HTMLElement>("voice-room-count");
  private readonly codeWrap = byId<HTMLElement>("voice-room-code-wrap");
  private readonly code = byId<HTMLElement>("voice-room-code");
  private readonly copyCode = byId<HTMLButtonElement>("voice-room-copy-code");
  private readonly participants = byId<HTMLUListElement>("voice-room-participants");
  private readonly microphone = byId<HTMLButtonElement>("voice-room-mic");
  private readonly startAudio = byId<HTMLButtonElement>("voice-room-audio");
  private readonly settings = byId<HTMLButtonElement>("voice-room-settings");
  private readonly menu = byId<HTMLElement>("voice-device-menu");
  private readonly input = byId<HTMLSelectElement>("voice-input-device");
  private readonly output = byId<HTMLSelectElement>("voice-output-device");
  private readonly volume = byId<HTMLInputElement>("voice-output-volume");
  private readonly test = byId<HTMLButtonElement>("voice-test-microphone");
  private active?: ActiveRoom;
  private lastSpeakers: VoiceParticipant[] = [];
  private lastParticipants: VoiceParticipant[] = [];
  private roster: VoiceRoster = new Map();
  private microphoneEnabled = false;
  private readonly widget = createRoomWidget({
    openRoom: () => this.restore(),
    toggleMicrophone: () => void this.toggleMicrophone(),
  });

  constructor(private readonly options: VoiceRoomControllerOptions) {
    byId<HTMLButtonElement>("voice-room-back").onclick = () => this.minimize();
    byId<HTMLButtonElement>("voice-room-leave").onclick = () => void this.leave();
    this.copyCode.onclick = () => void copyRoomCode(this.code, this.copyCode);
    this.microphone.onclick = () => void this.toggleMicrophone();
    this.startAudio.onclick = () => void this.enableAudio();
    this.settings.onclick = () => setDeviceMenuOpen(this.settings, this.menu, Boolean(this.menu.hidden));
    this.input.onchange = () => void this.selectDevice("input", this.input.value);
    this.output.onchange = () => void this.selectDevice("output", this.output.value);
    this.volume.oninput = () => this.client.setOutputVolume(Number(this.volume.value) / 100);
    this.test.onclick = () => void this.testMicrophone();
  }

  async openTemporary(code: string): Promise<void> {
    const join = await api.joinRoom(code);
    await this.open({ id: code, kind: "temporary", title: `Комната ${code}` }, join.livekit_url, join.access_token);
  }

  async openGroup(id: string, title: string, groupId?: string): Promise<void> {
    const join = await api.joinGroupVoice(id);
    await this.open({ id, kind: "group", title, groupId }, join.livekit_url, join.access_token);
  }

  async restoreSaved(fullscreen = false): Promise<boolean> {
    if (this.active) return true;
    const room = loadActiveRoom();
    if (!room) return false;
    try {
      const join = room.kind === "group" ? await api.joinGroupVoice(room.id) : await api.joinRoom(room.id);
      await this.open(room, join.livekit_url, join.access_token, fullscreen);
      return true;
    } catch (error) {
      this.options.onError(`Не удалось восстановить звонок: ${errorMessage(error)}`);
      return false;
    }
  }

  async leave(): Promise<void> {
    const active = this.active;
    await this.client.disconnect().catch(() => undefined);
    this.active = undefined;
    this.lastSpeakers = [];
    this.lastParticipants = [];
    this.roster = new Map();
    this.view.hidden = true;
    this.widget.update("", "", false);
    clearActiveRoomCode();
    if (active) {
      const request = active.kind === "group" ? api.leaveGroupVoice(active.id) : api.leaveRoom(active.id);
      await request.catch((error: unknown) => this.options.onError(errorMessage(error)));
    }
    this.options.onClose();
  }

  private async open(room: ActiveRoom, url: string, token: string, fullscreen = true): Promise<void> {
    if (this.active) await this.leave();
    this.active = room;
    saveActiveRoom(room);
    this.microphoneEnabled = false;
    this.lastSpeakers = [];
    this.lastParticipants = [];
    this.roster = new Map();
    this.title.textContent = room.title;
    this.kicker.textContent = room.kind === "group" ? "Голосовая комната группы" : "Временная комната";
    renderRoomCode(this.codeWrap, this.code, this.copyCode, room);
    setDeviceMenuOpen(this.settings, this.menu, false);
    if (fullscreen) this.restore();
    else {
      this.view.hidden = true;
      this.syncWidget(true);
    }
    this.renderStatus("connecting");
    try {
      await this.client.connect(url, token, {
        onStatus: (status) => this.renderStatus(status),
        onParticipants: (participants) => this.renderParticipants(participants),
        onRosterChanged: () => void this.handleRosterChanged(),
        onAudioBlocked: (blocked) => { this.startAudio.hidden = !blocked; },
      });
      await this.refreshRoster();
      await this.loadDevices();
      await this.applyAudioPreferences().catch((error) => {
        this.detail.textContent = `Настройки звука не применены: ${errorMessage(error)}`;
      });
    } catch (error) {
      this.active = undefined;
      this.widget.update("", "", false);
      this.renderStatus("error");
      this.options.onError(errorMessage(error));
    }
  }

  private minimize(): void {
    if (!this.active) return;
    setDeviceMenuOpen(this.settings, this.menu, false);
    this.view.hidden = true;
    this.syncWidget(true);
    this.options.onMinimize();
  }

  private restore(): void {
    if (!this.active) return;
    this.view.hidden = false;
    this.syncWidget(false);
    this.options.onOpen();
  }

  private renderStatus(status: VoiceStatus): void {
    const [title, detail] = statusText[status];
    this.status.textContent = title;
    this.detail.textContent = detail;
    this.status.dataset.state = status;
    this.syncWidget(this.view.hidden === true);
  }

  private renderParticipants(participants: VoiceParticipant[]): void {
    const enriched = applyRoster(participants, this.roster);
    this.lastParticipants = enriched;
    this.count.textContent = `Участники: ${enriched.length}`;
    this.microphoneEnabled = enriched.some((participant) => participant.isLocal && !participant.isMicrophoneMuted);
    this.rememberSpeakers(enriched);
    this.renderMicrophoneButton();
    this.microphone.setAttribute("aria-pressed", String(this.microphoneEnabled));
    renderParticipantCards(this.participants, enriched);
    this.widget.setMicrophone(this.microphoneEnabled);
    this.widget.setSpeakers(this.lastSpeakers.length ? this.lastSpeakers : enriched);
    if (this.status.dataset.state === "connected") {
      this.detail.textContent = pingLabel(enriched);
      this.syncWidget(this.view.hidden === true);
    }
  }

  private async handleRosterChanged(): Promise<void> {
    this.options.onRosterChanged();
    await this.refreshRoster();
  }

  private async refreshRoster(): Promise<void> {
    if (!this.active) return;
    try {
      this.roster = await loadVoiceRoster(this.active);
      this.renderParticipants(this.lastParticipants);
    } catch {
      this.roster = new Map();
    }
  }

  private renderMicrophoneButton(): void {
    const icon = document.createElement("img");
    icon.alt = "";
    icon.src = this.microphoneEnabled ? "/microphone-on.svg" : "/microphone-off.svg";
    this.microphone.replaceChildren(icon);
    this.microphone.setAttribute(
      "aria-label",
      this.microphoneEnabled ? "Выключить микрофон" : "Включить микрофон",
    );
  }

  private async toggleMicrophone(): Promise<void> {
    try {
      await this.client.setMicrophoneEnabled(!this.microphoneEnabled);
    } catch (error) {
      this.options.onError(`Не удалось изменить состояние микрофона: ${errorMessage(error)}`);
    }
  }

  private async enableAudio(): Promise<void> {
    try {
      await this.client.startAudio();
    } catch (error) {
      this.options.onError(`Не удалось включить звук: ${errorMessage(error)}`);
    }
  }

  private async testMicrophone(): Promise<void> {
    this.detail.textContent = "Проверяем микрофон...";
    try {
      const devices = await this.client.inputDevices();
      this.detail.textContent = devices.length ? "Микрофон доступен" : "Микрофон не найден";
    } catch (error) {
      this.options.onError(`Не удалось проверить микрофон: ${errorMessage(error)}`);
    }
  }

  private async loadDevices(): Promise<void> {
    try {
      const [inputs, outputs] = await Promise.all([this.client.inputDevices(), this.client.outputDevices()]);
      populateDevices(this.input, inputs, "Системный микрофон");
      populateDevices(this.output, outputs, "Системные динамики");
    } catch (error) {
      this.detail.textContent = `Устройства недоступны: ${errorMessage(error)}`;
    }
  }

  private async applyAudioPreferences(): Promise<void> {
    const preferences = loadAudioPreferences();
    await this.selectSavedDevice("input", preferences.inputDeviceId);
    await this.selectSavedDevice("output", preferences.outputDeviceId);
    if (preferences.microphoneEnabled) await this.client.setMicrophoneEnabled(true);
  }

  private async selectSavedDevice(kind: "input" | "output", deviceId: string): Promise<void> {
    if (!deviceId) return;
    const select = kind === "input" ? this.input : this.output;
    if (!Array.from(select.options).some((option) => option.value === deviceId)) return;
    select.value = deviceId;
    await this.selectDevice(kind, deviceId);
  }

  private async selectDevice(kind: "input" | "output", deviceId: string): Promise<void> {
    if (!deviceId) return;
    try {
      if (kind === "input") await this.client.selectInputDevice(deviceId);
      else await this.client.selectOutputDevice(deviceId);
    } catch (error) {
      this.options.onError(`Не удалось переключить устройство: ${errorMessage(error)}`);
    }
  }

  private rememberSpeakers(participants: VoiceParticipant[]): void {
    for (const participant of participants.filter((item) => item.isSpeaking)) {
      this.lastSpeakers = [
        participant,
        ...this.lastSpeakers.filter((item) => item.identity !== participant.identity),
      ].slice(0, 2);
    }
  }

  private syncWidget(visible: boolean): void {
    this.widget.update(this.active?.title || "", this.widgetLabel(), visible);
  }

  private widgetLabel(): string {
    if (this.status.dataset.state !== "connected") return this.status.textContent || "";
    return pingLabel(this.lastParticipants);
  }
}
