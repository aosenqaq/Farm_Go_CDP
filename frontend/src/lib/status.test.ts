import { describe, expect, it } from 'vitest';

import { phaseLabel, phaseTone } from './status';

describe('phaseLabel', () => {
  it('maps runtime phases to Chinese labels', () => {
    expect(phaseLabel('idle')).toBe('空闲');
    expect(phaseLabel('listening')).toBe('正在监听');
    expect(phaseLabel('handshaking')).toBe('握手中');
    expect(phaseLabel('ready')).toBe('已就绪');
    expect(phaseLabel('disconnected')).toBe('已断开');
    expect(phaseLabel('error')).toBe('错误');
  });

  it('falls back to the raw phase for unknown values', () => {
    expect(phaseLabel('custom')).toBe('custom');
  });
});

describe('phaseTone', () => {
  it('groups phases by visual severity', () => {
    expect(phaseTone('ready')).toBe('good');
    expect(phaseTone('listening')).toBe('good');
    expect(phaseTone('handshaking')).toBe('warn');
    expect(phaseTone('error')).toBe('bad');
    expect(phaseTone('disconnected')).toBe('bad');
    expect(phaseTone('idle')).toBe('neutral');
  });
});
