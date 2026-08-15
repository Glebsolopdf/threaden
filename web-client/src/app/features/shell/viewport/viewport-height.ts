export interface ViewportHeightInput {
  innerHeight: number;
  visualHeight?: number;
}

export function getViewportHeight(input: ViewportHeightInput): number {
  if (!input.visualHeight || input.visualHeight <= 0) return Math.round(input.innerHeight);
  return Math.round(Math.min(input.innerHeight, input.visualHeight));
}
