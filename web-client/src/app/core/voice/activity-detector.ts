interface ActivityState {
  floor: number;
  level: number;
  activeUntil: number;
}

const DEFAULT_THRESHOLD = 0.045;
const HOLD_MS = 460;
const MIN_VISIBLE_LEVEL = 0.012;

export class VoiceActivityDetector {
  private readonly states = new Map<string, ActivityState>();

  update(identity: string, level: number, speaking: boolean, muted: boolean, now = performance.now()): boolean {
    const state = this.states.get(identity) ?? { floor: 0.008, level: 0, activeUntil: 0 };
    const normalized = Number.isFinite(level) ? Math.max(0, Math.min(1, level)) : 0;
    state.level = state.level * 0.62 + normalized * 0.38;
    if (!speaking && state.level < DEFAULT_THRESHOLD) state.floor = state.floor * 0.96 + state.level * 0.04;
    const threshold = Math.max(DEFAULT_THRESHOLD, state.floor + 0.028);
    if (!muted && (speaking || state.level > threshold)) state.activeUntil = now + HOLD_MS;
    this.states.set(identity, state);
    return !muted && now < state.activeUntil && state.level > MIN_VISIBLE_LEVEL;
  }

  prune(identities: Set<string>): void {
    for (const identity of this.states.keys()) if (!identities.has(identity)) this.states.delete(identity);
  }

  reset(): void { this.states.clear(); }
}
