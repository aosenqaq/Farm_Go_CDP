import { describe, expect, it } from 'vitest';

import { mergeLandDetailsDelta } from './landDetailsPolling';

describe('land details polling', () => {
  it('preserves unchanged land records while applying an incremental image change', () => {
    const first = { id: '1', landId: 1, imageUrl: '/farm-assets/first', status: 'growing' };
    const current = {
      revision: 'r1',
      lands: [first, { id: '2', landId: 2, imageUrl: '/farm-assets/second', status: 'growing' }],
    };

    const merged = mergeLandDetailsDelta(current, {
      full: false,
      revision: 'r2',
      lands: [{ id: '2', landId: 2, imageUrl: '/farm-assets/new', status: 'growing' }],
      removedLandIds: [],
    });

    expect(merged.lands[0]).toBe(first);
    expect(merged.lands[1].imageUrl).toBe('/farm-assets/new');
    expect(merged.revision).toBe('r2');
  });

});
