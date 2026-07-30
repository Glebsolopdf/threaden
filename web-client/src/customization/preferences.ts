const STORAGE_KEY = "voice_rooms_theme";

export type ThemeID = "default";
export type ThemeSaveResult = "saved" | "storage-unavailable";

export interface ThemePreset {
  id: ThemeID;
  name: string;
  description: string;
}

export const THEME_PRESETS: ThemePreset[] = [
  {
    id: "default",
    name: "По умолчанию",
    description: "Цветная тёмная тема threaden без ручного акцента.",
  },
];

export function loadTheme(): ThemeID {
  const stored = readStoredValue();
  return stored === "default" ? stored : "default";
}

export function saveTheme(value: ThemeID): ThemeSaveResult {
  try {
    localStorage.setItem(STORAGE_KEY, value);
    return "saved";
  } catch {
    return "storage-unavailable";
  }
}

export function applyTheme(value: ThemeID): void {
  document.documentElement.dataset.theme = value;
}

function readStoredValue(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch {
    return null;
  }
}
