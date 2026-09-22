import React from 'react';
import { History, CheckCircle2, XCircle, ChevronRight, Sparkles, RefreshCw, Loader2, Clock } from 'lucide-react';
import { cn, formatRelativeTime, formatUSD } from '../lib/format';

const STATUS = {
  completed: { cls: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20', icon: CheckCircle2 },
  failed: { cls: 'bg-rose-500/10 text-rose-400 border-rose-500/20', icon: XCircle },
  running: { cls: 'bg-sky-500/10 text-sky-400 border-sky-500/20', icon: Loader2, spin: true },
  queued: { cls: 'bg-amber-500/10 text-amber-400 border-amber-500/20', icon: Clock },
};

export function StatusBadge({ status }) {
  const s = STATUS[status] || STATUS.queued;
  const Icon = s.icon;
  return (
    <span className={cn('inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold border capitalize', s.cls)}>
      <Icon className={cn('h-2.5 w-2.5', s.spin && 'animate-spin')} aria-hidden="true" />
      {status}
    </span>
  );
}

function WinnerCell({ job }) {
  if (job.winning_model) {
    return (
      <span className="inline-flex items-center gap-1 text-emerald-400 font-mono text-[11px] max-w-[12rem] truncate" title={job.winning_model}>
        <Sparkles className="h-3 w-3 flex-shrink-0" aria-hidden="true" />
        {job.winning_model}
      </span>
    );
  }
  const label = job.status === 'failed' ? 'All models failed' : job.status === 'completed' ? '—' : 'Evaluating…';
  return <span className={cn('font-mono text-[11px]', job.status === 'failed' ? 'text-rose-400' : 'text-slate-500')}>{label}</span>;
}

export default function EvaluationRunFeed({ jobs, loading, onSelectJob, lastUpdated, onRefresh }) {
  const header = (
    <div className="px-5 py-4 border-b border-slate-800 flex items-center justify-between gap-3">
      <div className="flex items-center gap-2">
        <History className="h-4 w-4 text-emerald-400" aria-hidden="true" />
        <h2 id="feed-heading" className="text-sm font-semibold text-white">
          Recent benchmark runs
        </h2>
      </div>
      <div className="flex items-center gap-2 text-[11px] text-slate-400">
        {lastUpdated && <span className="hidden sm:inline">Updated {formatRelativeTime(lastUpdated.toISOString())}</span>}
        <button type="button" onClick={onRefresh} className="icon-btn" aria-label="Refresh runs">
          <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
        </button>
      </div>
    </div>
  );

  if (loading) {
    return (
      <section className="surface overflow-hidden" aria-labelledby="feed-heading" aria-busy="true">
        {header}
        <div className="p-5 space-y-3">
          {[0, 1, 2].map(i => (
            <div key={i} className="skeleton h-9 w-full" />
          ))}
        </div>
      </section>
    );
  }

  if (!jobs || jobs.length === 0) {
    return (
      <section className="surface overflow-hidden" aria-labelledby="feed-heading">
        {header}
        <div className="p-8 text-center">
          <History className="h-8 w-8 text-slate-600 mx-auto mb-2" aria-hidden="true" />
          <p className="text-sm text-slate-400">No evaluation runs recorded yet.</p>
          <p className="text-xs text-slate-500 mt-1">Run a benchmark to populate the live stream.</p>
        </div>
      </section>
    );
  }

  return (
    <section className="surface overflow-hidden" aria-labelledby="feed-heading">
      {header}

      {/* Mobile: stacked cards */}
      <ul className="md:hidden divide-y divide-slate-800/60">
        {jobs.map(job => (
          <li key={job.job_id}>
            <button
              type="button"
              onClick={() => onSelectJob(job)}
              className="w-full text-left px-4 py-3 hover:bg-slate-850/60 focus-visible:bg-slate-850/60 transition-colors flex items-center gap-3"
            >
              <div className="flex-1 min-w-0 space-y-1">
                <div className="flex items-center gap-2">
                  <StatusBadge status={job.status} />
                  <span className="text-[11px] text-slate-500">{formatRelativeTime(job.created_at)}</span>
                </div>
                <p className="text-xs text-slate-200 truncate">{job.prompt}</p>
                <div className="flex items-center justify-between gap-2">
                  <WinnerCell job={job} />
                  <span className="font-mono text-[11px] text-slate-400">{formatUSD(job.total_cost_usd, 5)}</span>
                </div>
              </div>
              <ChevronRight className="h-4 w-4 text-slate-500 flex-shrink-0" aria-hidden="true" />
            </button>
          </li>
        ))}
      </ul>

      {/* Desktop: table */}
      <div className="hidden md:block overflow-x-auto">
        <table className="w-full text-left text-xs text-slate-300">
          <caption className="sr-only">Recent benchmark runs. Select a row to view per-model results.</caption>
          <thead className="bg-slate-950/60 text-slate-400 uppercase font-mono text-[10px] border-b border-slate-800">
            <tr>
              <th scope="col" className="px-4 py-3">Status</th>
              <th scope="col" className="px-4 py-3">Prompt</th>
              <th scope="col" className="px-4 py-3">Models</th>
              <th scope="col" className="px-4 py-3">Winner</th>
              <th scope="col" className="px-4 py-3 text-right">Cost</th>
              <th scope="col" className="px-4 py-3">Submitted</th>
              <th scope="col" className="px-4 py-3"><span className="sr-only">Details</span></th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800/60">
            {jobs.map(job => {
              const failures = (job.results || []).filter(r => r.error_message).length;
              return (
                <tr
                  key={job.job_id}
                  onClick={() => onSelectJob(job)}
                  className="hover:bg-slate-850/60 transition-colors cursor-pointer group"
                >
                  <td className="px-4 py-3">
                    <StatusBadge status={job.status} />
                  </td>
                  <td className="px-4 py-3 max-w-xs">
                    <p className="truncate text-slate-200" title={job.prompt}>
                      {job.prompt}
                    </p>
                    <p className="font-mono text-[10px] text-slate-500 truncate">
                      {job.suite_id || 'ad-hoc'} · {job.job_id.slice(0, 8)}
                    </p>
                  </td>
                  <td className="px-4 py-3 text-slate-400 whitespace-nowrap">
                    <span className="font-mono">{job.target_models?.length || 0}</span>
                    {failures > 0 && <span className="ml-1.5 text-rose-400 text-[10px]">({failures} failed)</span>}
                  </td>
                  <td className="px-4 py-3">
                    <WinnerCell job={job} />
                  </td>
                  <td className="px-4 py-3 text-right font-mono text-slate-300 font-medium">{formatUSD(job.total_cost_usd, 5)}</td>
                  <td className="px-4 py-3 text-slate-400 whitespace-nowrap" title={new Date(job.created_at).toLocaleString()}>
                    {formatRelativeTime(job.created_at)}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      type="button"
                      onClick={e => {
                        e.stopPropagation();
                        onSelectJob(job);
                      }}
                      className="icon-btn group-hover:text-indigo-400"
                      aria-label={`View results for run ${job.job_id.slice(0, 8)}`}
                    >
                      <ChevronRight className="h-4 w-4" aria-hidden="true" />
                    </button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}
