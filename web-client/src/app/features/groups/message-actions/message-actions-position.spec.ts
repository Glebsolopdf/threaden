import { describe, expect, it } from 'vitest';
import { placeMessageMenu, type MessageRect } from './message-actions-position';

const rect = (values: Partial<MessageRect>): MessageRect => ({
  top: 200,
  bottom: 260,
  left: 120,
  right: 420,
  width: 300,
  height: 60,
  ...values,
});

describe('placeMessageMenu', () => {
  it('places the menu above the message when there is room', () => {
    expect(placeMessageMenu(rect({ top: 200, bottom: 260 }), rect({ width: 140, height: 80 }), { width: 800, height: 600 }, 8)).toEqual({
      top: 112,
      left: 200,
      above: true,
    });
  });

  it('falls below the message when the top edge has insufficient room', () => {
    expect(placeMessageMenu(rect({ top: 40, bottom: 100 }), rect({ width: 140, height: 80 }), { width: 800, height: 600 }, 8)).toEqual({
      top: 108,
      left: 200,
      above: false,
    });
  });

  it('clamps the menu to the viewport on both axes', () => {
    expect(placeMessageMenu(rect({ top: 10, bottom: 30, left: 0, right: 40 }), rect({ width: 500, height: 900 }), { width: 320, height: 480 }, 8)).toEqual({
      top: 8,
      left: 8,
      above: false,
    });
  });
});
