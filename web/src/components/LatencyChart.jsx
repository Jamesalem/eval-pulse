import React, { useMemo } from 'react';
import { ResponsiveContainer, BarChart, Bar, XAxis, YAxis, Tooltip, Legend, CartesianGrid } from 'recharts';
import { BarChart3 } from 'lucide-react';

const SERIES = [
  { key: 'p50', color: '#38bdf8' },
  { key: 'p95', color: '#818cf8' },
  { key: 'p99', color: '#f43f5e' },
];

const shortName = model => model.replace('gemini-', 'gem-').replace('claude-', 'cl-').replace('ollama:', 'ollama/');

function ChartTooltip({ active, payload, label }) {
  if (!active || !payload?.length) return null;
  return (
    <div className="bg-slate-900 border border-slate-700 p-3 rounded-xl shadow-xl text-xs space-y-1">
      <p className="font-bold text-white mb-1.5">{payload[0]?.payload?.fullName || label}</p>
      {payload.map(entry => (
        <div key={entry.dataKey} className="flex items-center justify-between gap-4">
          <span style={{ color: entry.color }} className="font-medium uppercase">
            {entry.name}
          </span>
          <span className="font-mono text-slate-200 font-bold">{entry.value} ms</span>
        </div>
      ))}
    </div>
  );
}

export default function LatencyChart({ metrics, loading }) {
  const chartData = useMemo(
    () =>
      (metrics || [])
        .filter(m => m.total_evals - (m.failed_evals || 0) > 0)
        .map(m => ({
          name: shortName(m.model),
          fullName: m.model,
          p50: m.p50_latency_ms,
          p95: m.p95_latency_ms,
          p99: m.p99_latency_ms,
        })),
    [metrics]
  );

  return (
    <section className="surface p-5 h-full" aria-labelledby="latency-heading">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-4">
        <div className="flex items-center gap-2">
          <div className="p-1.5 rounded-lg bg-sky-500/10 border border-sky-500/30 text-sky-400">
            <BarChart3 className="h-4 w-4" aria-hidden="true" />
          </div>
          <div>
            <h3 id="latency-heading" className="text-sm font-semibold text-white">
              Latency percentiles
            </h3>
            <p className="text-[11px] text-slate-400">p50, p95 and p99 response time per model · lower is better</p>
          </div>
        </div>
      </div>

      <div className="h-64 w-full">
        {loading ? (
          <div className="skeleton h-full w-full rounded-xl" aria-hidden="true" />
        ) : chartData.length === 0 ? (
          <div className="h-full flex items-center justify-center rounded-xl border border-dashed border-slate-800 text-xs text-slate-500">
            Latency data appears after the first successful run.
          </div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={chartData} margin={{ top: 10, right: 10, left: -10, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" vertical={false} />
              <XAxis dataKey="name" stroke="#64748b" tick={{ fontSize: 11 }} interval={0} />
              <YAxis stroke="#64748b" tick={{ fontSize: 11 }} unit="ms" width={56} />
              <Tooltip content={<ChartTooltip />} cursor={{ fill: 'rgba(148,163,184,0.06)' }} />
              <Legend verticalAlign="top" align="right" iconType="circle" wrapperStyle={{ fontSize: 11, paddingBottom: 10 }} />
              {SERIES.map(s => (
                <Bar key={s.key} dataKey={s.key} name={s.key} fill={s.color} radius={[4, 4, 0, 0]} isAnimationActive={false} />
              ))}
            </BarChart>
          </ResponsiveContainer>
        )}
      </div>
    </section>
  );
}
