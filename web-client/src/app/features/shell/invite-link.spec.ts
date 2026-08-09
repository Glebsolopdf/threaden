import { parseInviteInput } from './invite-link';

describe('parseInviteInput', () => {
  const origin = 'https://threaden.example';

  it('accepts raw and same-origin invite values', () => {
    expect(parseInviteInput('inv_0123456789abcdef', origin)).toBe('inv_0123456789abcdef');
    expect(parseInviteInput(`${origin}/invite/inv_0123456789abcdef`, origin)).toBe('inv_0123456789abcdef');
  });

  it('rejects external invite values', () => {
    expect(parseInviteInput('https://evil.example/invite/inv_0123456789abcdef', origin)).toBeNull();
    expect(parseInviteInput('file:///tmp/invite/inv_0123456789abcdef', origin)).toBeNull();
  });
});
