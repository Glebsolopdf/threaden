import { applyTheme, loadTheme, saveTheme, THEME_PRESETS, type ThemeID } from "./preferences";

const STORAGE_WARNING =
  "Не удалось сохранить настройки на этом устройстве. Изменения могут сбрасываться после закрытия страницы.";

export function createCustomizationPanel(target: HTMLElement) {
  const warning = document.createElement("p");
  const controls = new Map<ThemeID, HTMLInputElement>();

  target.className = "customization-panel";
  target.replaceChildren(...THEME_PRESETS.map((preset) => themeControl(preset.id, controls)), warning);
  warning.className = "customization-panel__warning";
  warning.hidden = true;
  warning.setAttribute("role", "status");
  sync();

  function choose(theme: ThemeID): void {
    applyTheme(theme);
    if (saveTheme(theme) === "storage-unavailable") {
      warning.textContent = STORAGE_WARNING;
      warning.hidden = false;
      return;
    }
    warning.hidden = true;
    warning.textContent = "";
  }

  function sync(): void {
    const theme = loadTheme();
    applyTheme(theme);
    for (const [id, input] of controls) input.checked = id === theme;
  }

  for (const [theme, input] of controls) {
    input.addEventListener("change", () => {
      if (input.checked) choose(theme);
    });
  }

  return { sync };
}

function themeControl(theme: ThemeID, controls: Map<ThemeID, HTMLInputElement>): HTMLLabelElement {
  const preset = THEME_PRESETS.find((item) => item.id === theme)!;
  const label = document.createElement("label");
  const input = document.createElement("input");
  const text = document.createElement("span");
  const name = document.createElement("strong");
  const description = document.createElement("small");

  label.className = "customization-panel__theme";
  input.type = "radio";
  input.name = "interface-theme";
  input.value = theme;
  name.textContent = preset.name;
  description.textContent = preset.description;
  text.append(name, description);
  label.append(input, text);
  controls.set(theme, input);
  return label;
}
