export const byId = <T extends HTMLElement>(id: string): T => {
  const element = document.getElementById(id);
  if (!element) throw new Error(`Missing element: ${id}`);
  return element as T;
};

const avatarColors = ["#3157d5", "#7c3aed", "#0f8f86", "#c24172", "#b7791f", "#2563a9"];

export function avatarInitial(label: string): string {
  return [...label.trim()][0]?.toUpperCase() || "?";
}

export function avatarColor(label: string): string {
  const source = label.trim() || "?";
  const index = [...source].reduce((sum, char) => sum + char.charCodeAt(0), 0) % avatarColors.length;
  return avatarColors[index];
}

export function showAvatar(target: HTMLElement, value = "", label = value): void {
  target.replaceChildren();
  target.style.setProperty("--avatar-bg", avatarColor(label));
  if (value.startsWith("data:image/") || value.startsWith("blob:")) {
    const image = document.createElement("img");
    image.alt = "";
    image.src = value;
    target.append(image);
    return;
  }
  target.textContent = value.trim() || avatarInitial(label);
}
