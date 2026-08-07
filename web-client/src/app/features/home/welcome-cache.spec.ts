import { readWelcomeCache, writeWelcomeCache } from './welcome-cache';

describe('welcome cache', () => {
  it('keeps stats for one hour and expires them afterwards', () => {
    const storage = new StorageMock();
    const stats = { messages: 1284, new_users: 12, new_groups: 4 };

    writeWelcomeCache(storage, stats, 1000);

    expect(readWelcomeCache(storage, 1000 + 59 * 60 * 1000)).toEqual(stats);
    expect(readWelcomeCache(storage, 1000 + 60 * 60 * 1000)).toBeNull();
  });
});

class StorageMock implements Storage {
  private readonly values = new Map<string, string>();
  readonly length = 0;

  clear(): void { this.values.clear(); }
  getItem(key: string): string | null { return this.values.get(key) ?? null; }
  key(index: number): string | null { return [...this.values.keys()][index] ?? null; }
  removeItem(key: string): void { this.values.delete(key); }
  setItem(key: string, value: string): void { this.values.set(key, value); }
}
