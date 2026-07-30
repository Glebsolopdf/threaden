export function renderChatListSkeleton(target: HTMLElement, count = 8): void {
  target.replaceChildren(...Array.from({ length: count }, (_, index) => chatRow(index)));
}

export function renderMessageSkeleton(target: HTMLElement, count = 7): void {
  target.replaceChildren(...Array.from({ length: count }, (_, index) => messageRow(index)));
}

function chatRow(index: number): HTMLElement {
  const row = document.createElement("div");
  row.className = "skeleton-chat-row";
  row.style.setProperty("--skeleton-delay", `${index * 55}ms`);
  row.append(skeleton("skeleton-chat-row__avatar"), textBlock("skeleton-chat-row__text", 2));
  return row;
}

function messageRow(index: number): HTMLElement {
  const row = document.createElement("div");
  row.className = `skeleton-message ${index % 3 === 1 ? "skeleton-message--own" : ""}`;
  row.style.setProperty("--skeleton-delay", `${index * 65}ms`);
  if (!row.classList.contains("skeleton-message--own")) row.append(skeleton("skeleton-message__avatar"));
  row.append(textBlock("skeleton-message__bubble", index % 2 === 0 ? 3 : 2));
  return row;
}

function textBlock(className: string, lines: number): HTMLElement {
  const block = document.createElement("span");
  block.className = className;
  block.append(...Array.from({ length: lines }, (_, index) => skeleton(`skeleton-line skeleton-line--${index + 1}`)));
  return block;
}

function skeleton(className: string): HTMLElement {
  const item = document.createElement("span");
  item.className = `skeleton ${className}`;
  item.setAttribute("aria-hidden", "true");
  return item;
}
