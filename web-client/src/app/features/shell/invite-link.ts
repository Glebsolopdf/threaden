import { parseInviteToken } from '../groups/invite/invite-token';

export function parseInviteInput(value: string, origin: string): string | null {
  return parseInviteToken(value, origin);
}
