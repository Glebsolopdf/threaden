import type { WelcomeStats } from '../../core/api/models';

export const WELCOME_CACHE_TTL = 60 * 60 * 1000;
const CACHE_KEY = 'threaden_welcome_stats';

interface CachedWelcome {
  cached_at: number;
  stats: WelcomeStats;
}

export function readWelcomeCache(storage: Storage, now = Date.now()): WelcomeStats | null {
  try {
    const cached = JSON.parse(storage.getItem(CACHE_KEY) ?? '') as CachedWelcome;
    if (now - cached.cached_at >= WELCOME_CACHE_TTL) return null;
    return cached.stats;
  } catch {
    return null;
  }
}

export function writeWelcomeCache(storage: Storage, stats: WelcomeStats, now = Date.now()): void {
  try {
    storage.setItem(CACHE_KEY, JSON.stringify({ cached_at: now, stats } satisfies CachedWelcome));
  } catch {
    // Storage is optional; the API response remains usable.
  }
}
