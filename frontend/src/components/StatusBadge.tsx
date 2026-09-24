import { phaseLabel, phaseTone } from '../lib/status';

type StatusBadgeProps = {
  phase: string;
};

export function StatusBadge({ phase }: StatusBadgeProps) {
  return <span className={`status-badge status-${phaseTone(phase)}`}>{phaseLabel(phase)}</span>;
}
