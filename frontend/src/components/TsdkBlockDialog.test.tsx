import { act, create } from 'react-test-renderer';
import { describe, expect, it } from 'vitest';

import { TsdkBlockDialog } from './TsdkBlockDialog';

const oldEvent = {
  id: 1,
  timestamp: '2026-08-02T10:00:00+08:00',
  level: 'info',
  source: 'qq_ws',
  type: 'qqhost.log',
  message: '[TSDK-BLOCK] start',
};

const newEvent = {
  id: 2,
  timestamp: '2026-08-02T10:00:01+08:00',
  level: 'info',
  source: 'qq_ws',
  type: 'qqhost.log',
  message: '[TSDK-BLOCK] ready',
};

describe('TsdkBlockDialog', () => {
  it('keeps an open dialog live and places a new event first', () => {
    const renderer = create(
      <TsdkBlockDialog open events={[oldEvent]} onClose={() => undefined} />,
    );
    expect(JSON.stringify(renderer.toJSON())).toContain('[TSDK-BLOCK] start');

    act(() => renderer.update(
      <TsdkBlockDialog open events={[newEvent, oldEvent]} onClose={() => undefined} />,
    ));
    const rendered = JSON.stringify(renderer.toJSON());
    expect(rendered.indexOf('[TSDK-BLOCK] ready')).toBeLessThan(rendered.indexOf('[TSDK-BLOCK] start'));
  });
});
