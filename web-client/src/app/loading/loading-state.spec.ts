import { NavigationEnd, NavigationStart } from '@angular/router';
import { describe, expect, it } from 'vitest';
import { isNavigationSettled } from './loading-state';

describe('isNavigationSettled', () => {
  it('keeps the splash screen while navigation is starting', () => {
    expect(isNavigationSettled(new NavigationStart(1, '/'))).toBe(false);
  });

  it('removes the splash screen after navigation finishes', () => {
    expect(isNavigationSettled(new NavigationEnd(1, '/', '/'))).toBe(true);
  });
});
