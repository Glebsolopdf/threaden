import { Injectable } from '@angular/core';

const storagePrefix = 'threaden:history-notice:';

@Injectable({ providedIn: 'root' })
export class HistoryNoticeState {
  markAfterJoin(groupID: string): void {
    if (!groupID) return;
    try { sessionStorage.setItem(storagePrefix + groupID, '1'); } catch { /* Storage can be unavailable in private browsing. */ }
  }

  consume(groupID: string): boolean {
    if (!groupID) return false;
    try {
      const key = storagePrefix + groupID;
      if (sessionStorage.getItem(key) !== '1') return false;
      sessionStorage.removeItem(key);
      return true;
    } catch { return false; }
  }
}
