const WEB_PREFERENCES_KEY = "voice_rooms_web_preferences";

export interface WebPreferences {
  debugErrors: boolean;
}

export function loadWebPreferences(): WebPreferences {
  const stored = localStorage.getItem(WEB_PREFERENCES_KEY);
  if (!stored) return emptyWebPreferences();
  try {
    const value = JSON.parse(stored) as Partial<WebPreferences>;
    return { debugErrors: value.debugErrors === true };
  } catch {
    localStorage.removeItem(WEB_PREFERENCES_KEY);
    return emptyWebPreferences();
  }
}

export function saveWebPreferences(value: Partial<WebPreferences>): WebPreferences {
  const next = { ...loadWebPreferences(), ...value };
  localStorage.setItem(WEB_PREFERENCES_KEY, JSON.stringify(next));
  return next;
}

function emptyWebPreferences(): WebPreferences {
  return { debugErrors: false };
}
