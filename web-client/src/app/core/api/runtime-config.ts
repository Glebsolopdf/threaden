declare global {
  interface Window {
    __THREADEN_CONFIG__?: { apiBaseUrl?: string };
  }
}

export function apiBaseUrl(): string {
  return (window.__THREADEN_CONFIG__?.apiBaseUrl ?? '').replace(/\/$/, '');
}
