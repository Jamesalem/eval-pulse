import React from 'react';
import { CheckCircle2, ShieldAlert, CircleDashed } from 'lucide-react';
import { cn } from '../lib/format';

export default function RegressionAlertBanner({ regressionData, loading }) {
  if (loading && !regressionData) {
    return (
      <div className="surface p-4 flex items-center gap-3" aria-busy="true">
        <div className="skeleton h-8 w-8 rounded-lg" />
        <div className="flex-1 space-y-2">
          <div className="skeleton h-3.5 w-56" />
          <div className="skeleton h-3 w-80 max-w-full" />
        </div>
      </div>
    );
  }
  if (!regressionData) return null;

  const {
    passed,
    threshold_percent: threshold,
    max_observed_drift_percent: maxDrift,
    failing_models: failingModels = [],
    graded_models: gradedModels,
    message,
  } = regressionData;
  const ungraded = passed && gradedModels === 0;

  const tone = !passed ? 'fail' : ungraded ? 'idle' : 'pass';
  const styles = {
    pass: { box: 'bg-emerald-950/20 border-emerald-500/30', icon: 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400', chip: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30' },
    fail: { box: 'bg-rose-950/30 border-rose-500/40', icon: 'bg-rose-500/10 border-rose-500/30 text-rose-400', chip: 'bg-rose-500/15 text-rose-300 border-rose-500/40' },
    idle: { box: 'bg-slate-900 border-slate-800', icon: 'bg-slate-800 border-slate-700 text-slate-400', chip: 'bg-slate-800 text-slate-300 border-slate-700' },
  }[tone];
  const Icon = tone === 'fail' ? ShieldAlert : tone === 'idle' ? CircleDashed : CheckCircle2;
  const heading = tone === 'fail' ? 'Regression gate: failing' : tone === 'idle' ? 'Regression gate: awaiting graded runs' : 'Regression gate: passing';

  return (
    <section
      role={tone === 'fail' ? 'alert' : 'status'}
      aria-label="CI/CD regression gate"
      className={cn('rounded-2xl border p-4 shadow-sm transition-colors', styles.box)}
    >
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div className="flex items-start gap-3 min-w-0">
          <div className={cn('p-1.5 rounded-lg border mt-0.5 flex-shrink-0', styles.icon)}>
            <Icon className="h-5 w-5" aria-hidden="true" />
          </div>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-bold text-white">{heading}</span>
              <span className={cn('text-[10px] font-mono font-bold px-2 py-0.5 rounded border', styles.chip)}>
                exit {passed ? '0' : '1'}
              </span>
            </div>
            <p className="text-xs text-slate-300 mt-0.5">{message}</p>
            {!ungraded && (
              <p className="text-[11px] text-slate-400 mt-0.5">
                Max drift <span className="font-mono font-semibold text-slate-200">{Number(maxDrift ?? 0).toFixed(1)}%</span> · threshold{' '}
                <span className="font-mono">{threshold}%</span>
                {Number.isFinite(gradedModels) && <> · {gradedModels} graded model{gradedModels === 1 ? '' : 's'}</>}
              </p>
            )}
          </div>
        </div>

        {!passed && failingModels.length > 0 && (
          <div className="flex flex-wrap items-center gap-1.5 sm:justify-end">
            <span className="text-xs font-medium text-rose-400">Regressed:</span>
            {failingModels.map(m => (
              <span key={m} className="px-2 py-0.5 rounded bg-rose-900/60 border border-rose-700 text-xs font-mono text-rose-200">
                {m}
              </span>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
