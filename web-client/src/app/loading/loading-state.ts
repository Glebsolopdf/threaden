import { Event as RouterEvent, NavigationCancel, NavigationEnd, NavigationError, NavigationSkipped } from '@angular/router';

export function isNavigationSettled(event: RouterEvent): boolean {
  return event instanceof NavigationEnd
    || event instanceof NavigationCancel
    || event instanceof NavigationError
    || event instanceof NavigationSkipped;
}
