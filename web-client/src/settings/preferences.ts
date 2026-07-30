import { Room } from "livekit-client";
import { createCustomizationPanel } from "../customization/panel";
import "../customization/styles.css";
import { populateDevices } from "../voice-room/devices";
import { loadWebPreferences, saveWebPreferences } from "./web-preferences";

const PREFERENCES_KEY = "voice_rooms_audio_preferences";

export interface AudioPreferences {
  inputDeviceId: string;
  outputDeviceId: string;
  microphoneEnabled: boolean;
}


export function loadAudioPreferences(): AudioPreferences {
  const stored = localStorage.getItem(PREFERENCES_KEY);
  if (!stored) return emptyPreferences();
  try {
    const value = JSON.parse(stored) as Partial<AudioPreferences>;
    return {
      inputDeviceId: typeof value.inputDeviceId === "string" ? value.inputDeviceId : "",
      outputDeviceId: typeof value.outputDeviceId === "string" ? value.outputDeviceId : "",
      microphoneEnabled: value.microphoneEnabled === true,
    };
  } catch {
    localStorage.removeItem(PREFERENCES_KEY);
    return emptyPreferences();
  }
}

export function saveAudioPreferences(value: Partial<AudioPreferences>): AudioPreferences {
  const next = { ...loadAudioPreferences(), ...value };
  localStorage.setItem(PREFERENCES_KEY, JSON.stringify(next));
  return next;
}


export function createSettingsPage(options: {
  onBack: () => void;
  onError: (message: string) => void;
  onSuccess: (message: string) => void;
}) {
  const input = document.getElementById("settings-input-device") as HTMLSelectElement;
  const output = document.getElementById("settings-output-device") as HTMLSelectElement;
  const microphone = document.getElementById("settings-mic-default") as HTMLInputElement;
  const debugErrors = document.getElementById("settings-debug-errors") as HTMLInputElement;
  const refresh = document.getElementById("settings-refresh-devices") as HTMLButtonElement;
  const back = document.getElementById("settings-back") as HTMLButtonElement | null;
  const customization = createCustomizationPanel(document.getElementById("client-customization-panel") as HTMLElement);

  async function loadDevices(): Promise<void> {
    const prefs = loadAudioPreferences();
    const webPrefs = loadWebPreferences();
    microphone.checked = prefs.microphoneEnabled;
    debugErrors.checked = webPrefs.debugErrors;
    const [inputs, outputs] = await Promise.all([
      Room.getLocalDevices("audioinput", false),
      Room.getLocalDevices("audiooutput", false),
    ]);
    populateDevices(input, inputs, "Системный микрофон");
    populateDevices(output, outputs, "Системные динамики");
    input.value = optionExists(input, prefs.inputDeviceId) ? prefs.inputDeviceId : "";
    output.value = optionExists(output, prefs.outputDeviceId) ? prefs.outputDeviceId : "";
  }

  input.onchange = () => saveAudioPreferences({ inputDeviceId: input.value });
  output.onchange = () => saveAudioPreferences({ outputDeviceId: output.value });
  microphone.onchange = () => saveAudioPreferences({ microphoneEnabled: microphone.checked });
  debugErrors.onchange = () => saveWebPreferences({ debugErrors: debugErrors.checked });
  refresh.onclick = () => void loadDevices()
    .then(() => options.onSuccess("Устройства обновлены"))
    .catch(() => options.onError("Не удалось получить список устройств"));
  if (back) back.onclick = options.onBack;

  return {
    open(tab: "settings" | "customization" = "settings"): void {
      customization.sync();
      if (tab === "customization") return;
      void loadDevices().catch(() => options.onError("Не удалось получить список устройств"));
    },
  };
}

function emptyPreferences(): AudioPreferences {
  return { inputDeviceId: "", outputDeviceId: "", microphoneEnabled: false };
}

function optionExists(select: HTMLSelectElement, value: string): boolean {
  return !value || Array.from(select.options).some((option) => option.value === value);
}
