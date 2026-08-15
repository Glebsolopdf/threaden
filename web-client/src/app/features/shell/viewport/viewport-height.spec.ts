import { describe, expect, it } from 'vitest';
import { getViewportHeight } from './viewport-height';

describe('getViewportHeight', () => {
  it('uses the visual viewport height while the mobile keyboard is open', () => {
    expect(getViewportHeight({ innerHeight: 800, visualHeight: 420 })).toBe(420);
  });

  it('falls back to the layout viewport when visual viewport is unavailable', () => {
    expect(getViewportHeight({ innerHeight: 800 })).toBe(800);
  });

  it('never returns a zero-height app while the visual viewport is settling', () => {
    expect(getViewportHeight({ innerHeight: 800, visualHeight: 0.4 })).toBe(1);
  });
});
