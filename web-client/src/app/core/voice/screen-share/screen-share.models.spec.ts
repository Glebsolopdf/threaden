import { describe, expect, it } from 'vitest';
import { SCREEN_SHARE_PROFILES } from './screen-share.models';

describe('screen share profiles', () => {
  it('keeps the requested capture targets distinct from transport layers', () => {
    expect(SCREEN_SHARE_PROFILES.quality).toEqual({ width: 2560, height: 1440, frameRate: 15, maxBitrate: 3_500_000, contentHint: 'detail' });
    expect(SCREEN_SHARE_PROFILES.balanced).toEqual({ width: 1920, height: 1080, frameRate: 30, maxBitrate: 6_000_000, contentHint: 'detail' });
    expect(SCREEN_SHARE_PROFILES.smooth).toEqual({ width: 1280, height: 720, frameRate: 60, maxBitrate: 6_000_000, contentHint: 'motion' });
  });
});
