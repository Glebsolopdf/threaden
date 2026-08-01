import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { AuthStore } from './auth.store';
import { toApiError } from '../api/api-error';

export const authGuard: CanActivateFn = async (_route, state) => {
  const auth = inject(AuthStore);
  const router = inject(Router);
  let user: Awaited<ReturnType<AuthStore['ensureUser']>>;
  try {
    user = await auth.ensureUser();
  } catch (error) {
    if (toApiError(error).status === 429) return router.createUrlTree(['/blocked']);
    throw error;
  }
  return user ? true : router.createUrlTree(['/login'], { queryParams: { continue: state.url } });
};

export const guestGuard: CanActivateFn = async () => {
  const auth = inject(AuthStore);
  const router = inject(Router);
  let user: Awaited<ReturnType<AuthStore['ensureUser']>>;
  try {
    user = await auth.ensureUser();
  } catch (error) {
    if (toApiError(error).status === 429) return router.createUrlTree(['/blocked']);
    throw error;
  }
  return user ? router.createUrlTree(['/']) : true;
};
