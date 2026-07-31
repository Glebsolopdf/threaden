import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { AuthStore } from './auth.store';

export const authGuard: CanActivateFn = async (_route, state) => {
  const auth = inject(AuthStore);
  const router = inject(Router);
  const user = await auth.ensureUser();
  return user ? true : router.createUrlTree(['/login'], { queryParams: { continue: state.url } });
};

export const guestGuard: CanActivateFn = async () => {
  const auth = inject(AuthStore);
  const router = inject(Router);
  const user = await auth.ensureUser();
  return user ? router.createUrlTree(['/']) : true;
};
