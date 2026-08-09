const inviteTokenPattern = /^inv_[a-f0-9]+$/i;

export function parseInviteToken(value: string, origin: string): string | null {
  const input = value.trim();
  if (!input) return null;
  if (inviteTokenPattern.test(input)) return input;

  let url: URL;
  try {
    url = new URL(input, origin);
  } catch {
    return null;
  }
  if (url.origin !== origin || url.pathname.split('/').filter(Boolean).length !== 2) return null;
  const [route, token] = url.pathname.split('/').filter(Boolean);
  return route === 'invite' && token && inviteTokenPattern.test(token) ? token : null;
}
