export type MessageRect = Pick<DOMRect, 'top' | 'bottom' | 'left' | 'right' | 'width' | 'height'>;

export interface MenuPlacement {
  top: number;
  left: number;
  above: boolean;
}

const VIEWPORT_INSET = 8;

export function placeMessageMenu(
  anchor: MessageRect,
  menu: MessageRect,
  viewport: { width: number; height: number },
  gap: number,
): MenuPlacement {
  const centeredLeft = anchor.left + (anchor.width - menu.width) / 2;
  const canFitAbove = anchor.top >= menu.height + gap + VIEWPORT_INSET;
  const preferredTop = canFitAbove ? anchor.top - menu.height - gap : anchor.bottom + gap;
  const maxLeft = Math.max(VIEWPORT_INSET, viewport.width - menu.width - VIEWPORT_INSET);
  const maxTop = Math.max(VIEWPORT_INSET, viewport.height - menu.height - VIEWPORT_INSET);
  return {
    top: Math.min(maxTop, Math.max(VIEWPORT_INSET, preferredTop)),
    left: Math.min(maxLeft, Math.max(VIEWPORT_INSET, centeredLeft)),
    above: canFitAbove,
  };
}
