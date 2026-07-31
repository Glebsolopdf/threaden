import { computed, inject, Injectable, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { ApiService } from '../api/api.service';
import type { User } from '../api/models';
import { toApiError } from '../api/api-error';

@Injectable({ providedIn: 'root' })
export class AuthStore {
  private readonly api = inject(ApiService);
  private loadingPromise?: Promise<User | null>;

  readonly user = signal<User | null>(null);
  readonly initialized = signal(false);
  readonly loading = signal(false);
  readonly authenticated = computed(() => Boolean(this.user()));

  async ensureUser(force = false): Promise<User | null> {
    if (!force && this.initialized()) return this.user();
    if (!force && this.loadingPromise) return this.loadingPromise;
    this.loading.set(true);
    this.loadingPromise = firstValueFrom(this.api.getMe())
      .then((user) => {
        this.user.set(user);
        return user;
      })
      .catch((error: unknown) => {
        const apiError = toApiError(error);
        if (apiError.status !== 401) throw apiError;
        this.user.set(null);
        return null;
      })
      .finally(() => {
        this.initialized.set(true);
        this.loading.set(false);
        this.loadingPromise = undefined;
      });
    return this.loadingPromise;
  }

  async login(email: string, password: string): Promise<User> {
    const user = await firstValueFrom(this.api.login(email, password));
    this.user.set(user);
    this.initialized.set(true);
    return user;
  }

  async register(email: string, password: string): Promise<User> {
    const user = await firstValueFrom(this.api.register(email, password));
    this.user.set(user);
    this.initialized.set(true);
    return user;
  }

  setUser(user: User): void {
    this.user.set(user);
    this.initialized.set(true);
  }

  async logout(): Promise<void> {
    try { await firstValueFrom(this.api.logout()); } finally {
      this.user.set(null);
      this.initialized.set(true);
    }
  }

  clear(): void {
    this.user.set(null);
    this.initialized.set(true);
  }
}
