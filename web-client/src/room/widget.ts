import { avatarColor, byId, showAvatar } from "../dom";
import { setMicrophoneLabel } from "../ui";
import type { VoiceParticipant } from "../voice";

interface WidgetPosition {
  x: number;
  y: number;
}

const POSITION_KEY = "voice_rooms_widget_position";
const DEFAULT_OFFSET = 16;

export function createRoomWidget(options: {
  openRoom: () => void;
  toggleMicrophone: () => void;
}) {
  const widget = byId<HTMLElement>("room-widget");
  const title = byId<HTMLElement>("widget-room-title");
  const status = byId<HTMLElement>("widget-room-status");
  const open = byId<HTMLButtonElement>("widget-open-room");
  const mic = byId<HTMLButtonElement>("widget-microphone");
  const speakers = byId<HTMLElement>("widget-speakers");

  widget.setAttribute("popover", "manual");
  placeWidget(widget);
  keepAboveDialogs(widget);
  open.addEventListener("click", options.openRoom);
  mic.addEventListener("click", options.toggleMicrophone);
  window.addEventListener("resize", () => placeWidget(widget));

  widget.addEventListener("pointerdown", (event) => {
    if (!(event.target instanceof HTMLElement) || event.target.closest("button")) return;
    widget.setPointerCapture(event.pointerId);
    widget.dataset.dragging = "true";
    const rect = widget.getBoundingClientRect();
    const shiftX = event.clientX - rect.left;
    const shiftY = event.clientY - rect.top;

    const move = (moveEvent: PointerEvent) => {
      const next = clampPosition(widget, {
        x: moveEvent.clientX - shiftX,
        y: moveEvent.clientY - shiftY,
      });
      setPosition(widget, next);
    };
    const up = (upEvent: PointerEvent) => {
      widget.releasePointerCapture(upEvent.pointerId);
      delete widget.dataset.dragging;
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      localStorage.setItem(POSITION_KEY, JSON.stringify(readCurrentPosition(widget)));
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up, { once: true });
  });

  return {
    setBusy(value: boolean): void {
      mic.disabled = value;
    },
    setMicrophone(enabled: boolean): void {
      setMicrophoneLabel(mic, enabled);
    },
    setSpeakers(participants: VoiceParticipant[]): void {
      speakers.replaceChildren(...participants.slice(0, 2).map(speakerAvatar));
    },
    update(roomTitle: string, label: string, visible: boolean): void {
      title.textContent = roomTitle;
      status.textContent = label;
      if (visible) {
        widget.hidden = false;
        raiseInTopLayer(widget);
        placeWidget(widget);
        return;
      }
      hideFromTopLayer(widget);
      widget.hidden = true;
    },
  };
}

function speakerAvatar(participant: VoiceParticipant): HTMLSpanElement {
  const avatar = document.createElement("span");
  avatar.className = "room-widget__avatar";
  avatar.style.setProperty("--avatar-bg", avatarColor(participant.name));
  showAvatar(avatar, participant.avatar, participant.name);
  avatar.title = participant.name;
  return avatar;
}

function placeWidget(widget: HTMLElement): void {
  setPosition(widget, clampPosition(widget, readPosition(widget)));
}

function readPosition(widget: HTMLElement): WidgetPosition {
  const stored = localStorage.getItem(POSITION_KEY);
  if (stored) {
    try {
      const value = JSON.parse(stored) as Partial<WidgetPosition>;
      if (Number.isFinite(value.x) && Number.isFinite(value.y)) {
        return { x: Number(value.x), y: Number(value.y) };
      }
    } catch {
      localStorage.removeItem(POSITION_KEY);
    }
  }
  return {
    x: window.innerWidth - widget.offsetWidth - DEFAULT_OFFSET,
    y: window.innerHeight - widget.offsetHeight - DEFAULT_OFFSET,
  };
}

function readCurrentPosition(widget: HTMLElement): WidgetPosition {
  const rect = widget.getBoundingClientRect();
  return { x: rect.left, y: rect.top };
}

function setPosition(widget: HTMLElement, position: WidgetPosition): void {
  widget.style.left = `${position.x}px`;
  widget.style.top = `${position.y}px`;
}

function clampPosition(widget: HTMLElement, position: WidgetPosition): WidgetPosition {
  const maxX = Math.max(DEFAULT_OFFSET, window.innerWidth - widget.offsetWidth - DEFAULT_OFFSET);
  const maxY = Math.max(DEFAULT_OFFSET, window.innerHeight - widget.offsetHeight - DEFAULT_OFFSET);
  return {
    x: Math.min(maxX, Math.max(DEFAULT_OFFSET, position.x)),
    y: Math.min(maxY, Math.max(DEFAULT_OFFSET, position.y)),
  };
}

function showInTopLayer(element: HTMLElement): void {
  if ("showPopover" in element && !element.matches(":popover-open")) element.showPopover();
}

function hideFromTopLayer(element: HTMLElement): void {
  if ("hidePopover" in element && element.matches(":popover-open")) element.hidePopover();
}

function raiseInTopLayer(element: HTMLElement): void {
  if (!("showPopover" in element) || !("hidePopover" in element)) return;
  if (element.matches(":popover-open")) element.hidePopover();
  element.showPopover();
}

function keepAboveDialogs(widget: HTMLElement): void {
  const observer = new MutationObserver(() => {
    if (!widget.hidden) window.requestAnimationFrame(() => raiseInTopLayer(widget));
  });
  for (const dialog of document.querySelectorAll("dialog")) {
    observer.observe(dialog, { attributes: true, attributeFilter: ["open"] });
  }
}
