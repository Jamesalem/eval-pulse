import React from 'react';
import { ResponsiveContainer, BarChart, Bar, XAxis, YAxis, Tooltip, Legend, CartesianGrid } from 'recharts';
import { BarChart3 } from 'lucide-react';

export default function LatencyChart({ metrics }) {
  if (!metrics || metrics.length === 0) return null;

  const chartData = metrics.map(m => ({
    name: m.model.replace('gemini-', 'gem-').replace('claude-', 'cl-').replace('ollama:', 'ollama/'),
    p50: m.p50_latency_ms,
    p95: m.p95_latency_ms,
    p99: m.p99_latency_ms,
  }));

  const CustomTooltip = ({ active, payload, label }) => {
    if (active && payload && payload.length) {
      return (
        <div className="bg-slate-900 border border-slate-700 p-3 rounded-xl shadow-xl text-xs space-y-1">
          <p className="font-bold text-white mb-1.5">{label}</p>
          {payload.map((entry, index) => (
            <div key={index} className="flex items-center justify-between space-x-4">
              <span style={{ color: entry.color }} className="font-medium">
                {entry.name.toUpperCase()}:
              </span>
              <span className="font-mono text-slate-200 font-bold">{entry.value} ms</span>
            </div>
          ))}
        </div>
      );
    }
    return null;
  };

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center space-x-2">
          <div className="p-1.5 rounded-lg bg-sky-500/10 border border-sky-500/30 text-sky-400">
            <BarChart3 className="h-4 w-4" />
          </div>
          <div>
            <h3 className="text-sm font-semibold text-white">Latency Distribution Percentiles</h3>
            <p className="text-[11px] text-slate-400">p50, p95, and p99 response times (lower is better)</p>
          </div>
        </div>
        <span className="text-[11px] text-slate-400 font-mono">Unit: milliseconds (ms)</span>
      </div>

      <div className="h-64 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" vertical={false} />
            <XAxis dataKey="name" stroke="#64748b" tick={{ fontSize: 11 }} />
            <YAxis stroke="#64748b" tick={{ fontSize: 11 }} />
            <Tooltip content={<CustomTooltip />} />
            <Legend
              verticalAlign="top"
              align="right"
              iconType="circle"
              wrapperStyle={{ fontSize: 11, paddingBottom: 10 }}
            />
            <Bar dataKey="p50" name="p50" fill="#38bdf8" radius={[4, 4, 0, 0]} />
            <Bar dataKey="p95" name="p95" fill="#818cf8" radius={[4, 4, 0, 0]} />
            <Bar dataKey="p99" name="p99" fill="#f43f5e" radius={[4, 4, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
