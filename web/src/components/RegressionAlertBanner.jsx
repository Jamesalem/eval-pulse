import React from 'react';
import { AlertTriangle, CheckCircle2, ShieldAlert, ArrowRight } from 'lucide-react';

export default function RegressionAlertBanner({ regressionData, onInspect }) {
  if (!regressionData) return null;

  const { passed, threshold_percent, max_observed_drift_percent, failing_models, message } = regressionData;

  return (
    <div
      className={`rounded-2xl border p-4 shadow-sm transition-all ${
        passed
          ? 'bg-emerald-950/20 border-emerald-500/30 text-emerald-300'
          : 'bg-rose-950/30 border-rose-500/40 text-rose-300'
      }`}
    >
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div className="flex items-start space-x-3">
          {passed ? (
            <div className="p-1.5 rounded-lg bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 mt-0.5">
              <CheckCircle2 className="h-5 w-5" />
            </div>
          ) : (
            <div className="p-1.5 rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-400 mt-0.5 animate-pulse">
              <ShieldAlert className="h-5 w-5" />
            </div>
          )}

          <div>
            <div className="flex items-center space-x-2">
              <span className="text-sm font-bold text-white">
                {passed ? 'CI/CD Regression Gate: PASSED' : 'CI/CD Regression Gate: ALERT TRIGGERED'}
              </span>
              <span
                className={`text-[10px] font-mono font-bold px-2 py-0.5 rounded border ${
                  passed
                    ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/30'
                    : 'bg-rose-500/20 text-rose-300 border-rose-500/40'
                }`}
              >
                Exit Code: {passed ? '0' : '1'}
              </span>
            </div>
            <p className="text-xs text-slate-300 mt-0.5">
              {message} (Max drift: <span className="font-mono font-semibold">{max_observed_drift_percent?.toFixed(1)}%</span> vs. allowed threshold: <span className="font-mono">{threshold_percent}%</span>)
            </p>
          </div>
        </div>

        {!passed && failing_models && failing_models.length > 0 && (
          <div className="flex items-center space-x-2">
            <span className="text-xs font-medium text-rose-400">Degraded:</span>
            <div className="flex space-x-1.5">
              {failing_models.map(m => (
                <span key={m} className="px-2 py-0.5 rounded bg-rose-900/60 border border-rose-700 text-xs font-mono text-rose-200">
                  {m}
                </span>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
