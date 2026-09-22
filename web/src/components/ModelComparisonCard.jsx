import React from 'react';
import { Gauge, Award, Clock, AlertTriangle, CheckCircle2, CircleDashed, XCircle } from 'lucide-react';
import { cn, formatPercent, formatUSD } from '../lib/format';

const STATUS_STYLES = {
  stable: { cls: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20', icon: CheckCircle2, label: 'Stable' },
  degraded: { cls: 'bg-amber-500/10 text-amber-400 border-amber-500/20', icon: AlertTriangle, label: 'Degraded' },
  critical: { cls: 'bg-rose-500/10 text-rose-400 border-rose-500/20', icon: XCircle, label: 'Critical' },
  ungraded: { cls: 'bg-slate-800 text-slate-400 border-slate-700', icon: CircleDashed, label: 'Ungraded' },
};

export function ModelComparisonCardSkeleton() {
  return (
    <div className="surface p-5 space-y-4" aria-hidden="true">
      <div className="flex justify-between">
        <div className="space-y-2">
          <div className="skeleton h-4 w-32" />
          <div className="skeleton h-3 w-24" />
        </div>
        <div className="skeleton h-5 w-16 rounded-full" />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="skeleton h-16 rounded-xl" />
        <div className="skeleton h-16 rounded-xl" />
      </div>
      <div className="skeleton h-4 w-full" />
    </div>
  );
}

export default function ModelComparisonCard({ metric, isWinner }) {
  const {
    model,
    total_evals: totalEvals = 0,
    failed_evals: failedEvals = 0,
    graded_evals: gradedEvals = 0,
    p50_latency_ms: p50,
    p95_latency_ms: p95,
    avg_semantic_score: avgScore = 0,
    avg_graded_score: gradedScore = 0,
    total_cost_usd: totalCost = 0,
    regression_status: regressionStatus,
  } = metric;

  const status = STATUS_STYLES[regressionStatus] || STATUS_STYLES.ungraded;
  const StatusIcon = status.icon;
  const successful = totalEvals - failedEvals;
  const shownScore = gradedEvals > 0 ? gradedScore : avgScore;

  return (
    <article
      className={cn(
        'rounded-2xl p-5 border transition-colors relative overflow-hidden bg-slate-900/90 shadow-sm',
        isWinner ? 'border-emerald-500/50 ring-1 ring-emerald-500/30' : 'border-slate-800 hover:border-slate-700'
      )}
      aria-label={`${model} metrics${isWinner ? ', top performer' : ''}`}
    >
      {isWinner && (
        <div className="absolute top-0 right-0 bg-emerald-500/20 border-b border-l border-emerald-500/30 px-2.5 py-0.5 rounded-bl-lg flex items-center gap-1 text-[10px] font-bold text-emerald-300 uppercase tracking-wider">
          <Award className="h-3 w-3 text-emerald-400" aria-hidden="true" />
          Top performer
        </div>
      )}

      <div className={cn('flex items-start justify-between gap-2 mb-4', isWinner && 'mt-3')}>
        <div className="min-w-0">
          <h3 className="text-sm font-bold text-white tracking-tight truncate" title={model}>
            {model}
          </h3>
          <p className="text-[11px] text-slate-400 font-mono">
            {totalEvals} run{totalEvals === 1 ? '' : 's'}
            {failedEvals > 0 && <span className="text-rose-400"> · {failedEvals} failed</span>}
          </p>
        </div>
        <span
          className={cn('px-2 py-0.5 rounded-full text-[10px] font-semibold flex items-center gap-1 border flex-shrink-0', status.cls)}
          title={regressionStatus === 'ungraded' ? 'No runs with a ground truth yet' : `Based on ${gradedEvals} graded run(s)`}
        >
          <StatusIcon className="h-3 w-3" aria-hidden="true" />
          {status.label}
        </span>
      </div>

      <dl className="grid grid-cols-2 gap-3 mb-4">
        <div className="bg-slate-950/70 p-3 rounded-xl border border-slate-800/80">
          <dt className="flex items-center gap-1.5 text-slate-400 text-[11px] mb-1">
            <Clock className="h-3.5 w-3.5 text-sky-400" aria-hidden="true" />
            p50 / p95
          </dt>
          <dd className="flex items-baseline gap-1 font-mono">
            {successful > 0 ? (
              <>
                <span className="text-base font-bold text-white">{p50}ms</span>
                <span className="text-xs text-slate-400">/ {p95}</span>
              </>
            ) : (
              <span className="text-base font-bold text-slate-500">—</span>
            )}
          </dd>
        </div>

        <div className="bg-slate-950/70 p-3 rounded-xl border border-slate-800/80">
          <dt className="flex items-center gap-1.5 text-slate-400 text-[11px] mb-1">
            <Gauge className="h-3.5 w-3.5 text-indigo-400" aria-hidden="true" />
            {gradedEvals > 0 ? 'Graded score' : 'Heuristic score'}
          </dt>
          <dd className="text-base font-bold text-emerald-400 font-mono">{successful > 0 ? formatPercent(shownScore) : '—'}</dd>
        </div>
      </dl>

      {successful > 0 && (
        <div className="h-1.5 rounded-full bg-slate-800 overflow-hidden mb-4" aria-hidden="true">
          <div
            className={cn('h-full rounded-full', shownScore >= 0.85 ? 'bg-emerald-500' : shownScore >= 0.7 ? 'bg-amber-500' : 'bg-rose-500')}
            style={{ width: `${Math.min(100, Math.max(0, shownScore * 100))}%` }}
          />
        </div>
      )}

      <div className="flex items-center justify-between pt-3 border-t border-slate-800/80 text-xs">
        <span className="text-slate-400">Cumulative spend</span>
        <span className="font-mono font-bold text-slate-200">{formatUSD(totalCost, 4)}</span>
      </div>
    </article>
  );
}
