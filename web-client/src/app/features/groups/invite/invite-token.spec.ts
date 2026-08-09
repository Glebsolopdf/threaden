import { parseInviteToken } from './invite-token';

describe('parseInviteToken', () => {
  const origin = 'https://threaden.example';

  it('accepts a raw invite token', () => {
    expect(parseInviteToken('inv_0123456789abcdef', origin)).toBe('inv_0123456789abcdef');
  });

  it('accepts an invite path and same-origin URL', () => {
    expect(parseInviteToken('/invite/inv_0123456789abcdef', origin)).toBe('inv_0123456789abcdef');
    expect(parseInviteToken(`${origin}/invite/inv_0123456789abcdef`, origin)).toBe('inv_0123456789abcdef');
  });

  it('rejects external URLs and unsupported schemes', () => {
    expect(parseInviteToken('https://evil.example/invite/inv_0123456789abcdef', origin)).toBeNull();
    expect(parseInviteToken('javascript:fetch("https://evil.example")', origin)).toBeNull();
    expect(parseInviteToken('data:text/plain,inv_0123456789abcdef', origin)).toBeNull();
    expect(parseInviteToken('file:///tmp/invite/inv_0123456789abcdef', origin)).toBeNull();
  });

  it('rejects malformed invite paths and empty values', () => {
    expect(parseInviteToken('', origin)).toBeNull();
    expect(parseInviteToken('/tmp/inv_0123456789abcdef', origin)).toBeNull();
    expect(parseInviteToken('/invite/inv_0123456789abcdef/extra', origin)).toBeNull();
    expect(parseInviteToken('/invite/not-a-token', origin)).toBeNull();
  });
});
