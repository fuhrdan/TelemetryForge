"use client";

type SparklineProps = {
  values: number[];
  label: string;
};

export function Sparkline({ values, label }: SparklineProps) {
  const width = 640;
  const height = 170;
  const pad = 10;

  if (values.length < 2) {
    return <div className="chart-empty">Waiting for live metric samples…</div>;
  }

  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = Math.max(max - min, 1);

  const points = values
    .map((value, index) => {
      const x = pad + (index / Math.max(values.length - 1, 1)) * (width - pad * 2);
      const y = height - pad - ((value - min) / range) * (height - pad * 2);
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");

  return (
    <div className="chart-wrap" aria-label={label}>
      <svg viewBox={`0 0 ${width} ${height}`} role="img">
        <defs>
          <linearGradient id="fill" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="currentColor" stopOpacity="0.24" />
            <stop offset="100%" stopColor="currentColor" stopOpacity="0.02" />
          </linearGradient>
        </defs>
        <polyline className="chart-line" points={points} />
      </svg>
      <div className="chart-scale">
        <span>{max.toFixed(1)}</span>
        <span>{min.toFixed(1)}</span>
      </div>
    </div>
  );
}
