import React from 'react';
import { Gauge, DollarSign, Award, Clock, AlertTriangle, CheckCircle2 } from 'lucide-react';

export default function ModelComparisonCard({ metric, isWinner }) {
  const {
    model,
    total_evals,
    p50_latency_ms,
    p95_latency_ms,
    avg_semantic_score,
    total_cost_usd,
    regression_status,
  } = metric;

  const isStable = regression_status === 'stable';

  return (
    <div
      className={`rounded-2xl p-5 border transition-all relative overflow-hidden bg-slate-900/90 shadow-sm ${
        isWinner
          ? 'border-emerald-500/50 ring-1 ring-emerald-500/30 shadow-emerald-950/20'
          : 'border-slate-800 hover:border-slate-700'
      }`}
    >
      {isWinner && (
        <div className="absolute top-0 right-0 bg-emerald-500/20 border-b border-l border-emerald-500/30 px-2.5 py-0.5 rounded-bl-lg flex items-center space-x-1 text-[10px] font-bold text-emerald-300 uppercase tracking-wider">
          <Award className="h-3 w-3 text-emerald-400" />
          <span>Top Performer</span>
        </div>
      )}

      {/* Model Name & Status */}
      <div className="flex items-center justify-between mb-4">
        <div>
          <h4 className="text-sm font-bold text-white tracking-tight">{model}</h4>
          <span className="text-[11px] text-slate-400 font-mono">{total_evals} benchmarks evaluated</span>
        </div>
        <span
          className={`px-2 py-0.5 rounded-full text-[10px] font-semibold flex items-center space-x-1 border ${
            isStable
              ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
              : 'bg-rose-500/10 text-rose-400 border-rose-500/20'
          }`}
        >
          {isStable ? (
            <CheckCircle2 className="h-3 w-3" />
          ) : (
            <AlertTriangle className="h-3 w-3" />
          )}
          <span className="capitalize">{regression_status}</span>
        </span>
      </div>

      {/* Primary Metrics Grid */}
      <div className="grid grid-cols-2 gap-3 mb-4">
        
        {/* Latency P50 / P95 */}
        <div className="bg-slate-950/70 p-3 rounded-xl border border-slate-800/80">
          <div className="flex items-center space-x-1.5 text-slate-400 text-[11px] mb-1">
            <Clock className="h-3.5 w-3.5 text-sky-400" />
            <span>Latency (p50 / p95)</span>
          </div>
          <div className="flex items-baseline space-x-1">
            <span className="text-base font-bold text-white font-mono">{p50_latency_ms}ms</span>
            <span className="text-xs text-slate-400 font-mono">/ {p95_latency_ms}ms</span>
          </div>
        </div>

        {/* Semantic Score */}
        <div className="bg-slate-950/70 p-3 rounded-xl border border-slate-800/80">
          <div className="flex items-center space-x-1.5 text-slate-400 text-[11px] mb-1">
            <Gauge className="h-3.5 w-3.5 text-indigo-400" />
            <span>Semantic Score</span>
          </div>
          <div className="flex items-baseline space-x-1">
            <span className="text-base font-bold text-emerald-400 font-mono">
              {(avg_semantic_score * 100).toFixed(1)}%
            </span>
            <span className="text-[10px] text-slate-400">acc</span>
          </div>
        </div>

      </div>

      {/* Cost Ledger */}
      <div className="flex items-center justify-between pt-3 border-t border-slate-800/80 text-xs">
        <span className="text-slate-400">Cumulative Spend</span>
        <span className="font-mono font-bold text-slate-200">
          ${total_cost_usd.toFixed(4)}
        </span>
      </div>
    </div>
  );
}
