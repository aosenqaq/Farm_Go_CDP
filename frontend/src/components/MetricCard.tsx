import type { ReactNode } from 'react';

type MetricCardProps = {
  label: string;
  value: ReactNode;
  detail?: ReactNode;
};

export function MetricCard({ label, value, detail }: MetricCardProps) {
  return (
    <section className="metric-card">
      <div className="metric-label">{label}</div>
      <div className="metric-value">{value}</div>
      {detail ? <div className="metric-detail">{detail}</div> : null}
    </section>
  );
}
