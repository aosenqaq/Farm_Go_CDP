export type PhaseTone = 'good' | 'warn' | 'bad' | 'neutral';

const labels: Record<string, string> = {
  idle: '空闲',
  listening: '正在监听',
  handshaking: '握手中',
  ready: '已就绪',
  disconnected: '已断开',
  error: '错误',
};

export function phaseLabel(phase: string) {
  return labels[phase] ?? phase;
}

export function phaseTone(phase: string): PhaseTone {
  if (phase === 'ready' || phase === 'listening') return 'good';
  if (phase === 'handshaking') return 'warn';
  if (phase === 'error' || phase === 'disconnected') return 'bad';
  return 'neutral';
}
