import { HistoryNoticeState } from './history-notice';

describe('HistoryNoticeState', () => {
  beforeEach(() => sessionStorage.clear());

  it('consumes a join marker only once per group', () => {
    const state = new HistoryNoticeState();

    state.markAfterJoin('private-group');

    expect(state.consume('private-group')).toBe(true);
    expect(state.consume('private-group')).toBe(false);
  });

  it('keeps markers independent between groups', () => {
    const state = new HistoryNoticeState();
    state.markAfterJoin('private-group');

    expect(state.consume('another-group')).toBe(false);
    expect(state.consume('private-group')).toBe(true);
  });
});
