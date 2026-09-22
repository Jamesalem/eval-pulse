import React, { useEffect, useRef, useState } from 'react';
import { Clock, Gauge, DollarSign, Copy, Check, AlertTriangle, FlaskConical, Target } from 'lucide-react';
import Modal from './Modal';
import { StatusBadge } from './EvaluationRunFeed';
import { cn, durationBetween, formatPercent, formatRelativeTime, formatUSD, isPendingStatus } from '../lib/format';

function Metric({ icon: Icon, iconClass, label, children }) {
  return (
    <div className="bg-slate-950 p-2.5 rounded-xl border border-slate-800">
      <dt className="text-[10px] text-slate-400 flex items-center gap-1">
        {Icon && <Icon className={cn('h-3 w-3', iconClass)} aria-hidden="true" />}
        {label}
      </dt>
      <dd className="text-xs font-bold text-white font-mono mt-0.5">{children}</dd>
    </div>
  );
}

export default function JobDetailModal({ job, onClose }) {
  const results = job.results || [];
  const [activeModel, setActiveModel] = useState(() => job.winning_model || results[0]?.model || null);
  const [copied, setCopied] = useState(false);
  const copyTimer = useRef(null);

  // Results arrive after the modal may already be open for a queued job.
  useEffect(() => {
    if (!activeModel && results.length) setActiveModel(job.winning_model || results[0].model);
  }, [activeModel, results, job.winning_model]);

  useEffect(() => () => clearTimeout(copyTimer.current), []);

  const current = results.find(r => r.model === activeModel) || results[0];
  const pending = isPendingStatus(job.status);
  const duration = durationBetween(job.created_at, job.completed_at);

  const handleCopy = async text => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      clearTimeout(copyTimer.current);
      copyTimer.current = setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };

  const onTabKeyDown = (e, idx) => {
    if (e.key !== 'ArrowRight' && e.key !== 'ArrowLeft') return;
    e.preventDefault();
    const next = (idx + (e.key === 'ArrowRight' ? 1 : -1) + results.length) % results.length;
    setActiveModel(results[next].model);
    document.getElementById(`result-tab-${next}`)?.focus();
  };

  return (
    <Modal
      size="xl"
      title="Run details"
      badge={<StatusBadge status={job.status} />}
      description={
        <>
          <span className="font-mono">{job.job_id}</span> · suite <span className="font-mono text-slate-300">{job.suite_id || 'ad-hoc'}</span> · submitted{' '}
          {formatRelativeTime(job.created_at)}
          {duration && <> · took {duration}</>} · total <span className="text-emerald-400 font-mono font-semibold">{formatUSD(job.total_cost_usd, 5)}</span>
        </>
      }
      onClose={onClose}
      footer={
        <div className="flex justify-end">
          <button type="button" onClick={onClose} className="btn-secondary">
            Close
          </button>
        </div>
      }
    >
      <div className="space-y-4">
        <div className="bg-slate-950 p-3.5 rounded-xl border border-slate-800">
          <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block mb-1">Prompt</span>
          <p className="text-xs text-slate-200 whitespace-pre-wrap break-words">{job.prompt}</p>
        </div>

        {job.ground_truth && (
          <div className="bg-slate-950 p-3 rounded-xl border border-slate-800">
            <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block mb-1">Ground truth</span>
            <p className="text-xs text-slate-300 font-mono whitespace-pre-wrap break-words">{job.ground_truth}</p>
          </div>
        )}

        {results.length === 0 ? (
          <div className="rounded-xl border border-dashed border-slate-800 p-6 text-center text-xs text-slate-400">
            {pending ? (
              <span className="inline-flex items-center gap-2">
                <span className="h-2 w-2 rounded-full bg-sky-400 animate-ping" aria-hidden="true" />
                Evaluating {job.target_models?.length || 0} models… results will appear here automatically.
              </span>
            ) : (
              'No results were recorded for this run.'
            )}
          </div>
        ) : (
          <div>
            <div role="tablist" aria-label="Model results" className="flex gap-2 overflow-x-auto pb-2 border-b border-slate-800">
              {results.map((res, idx) => {
                const selected = current?.model === res.model;
                return (
                  <button
                    key={res.model}
                    id={`result-tab-${idx}`}
                    type="button"
                    role="tab"
                    aria-selected={selected}
                    aria-controls="result-panel"
                    tabIndex={selected ? 0 : -1}
                    onClick={() => setActiveModel(res.model)}
                    onKeyDown={e => onTabKeyDown(e, idx)}
                    className={cn(
                      'px-3 py-1.5 rounded-lg text-xs font-semibold font-mono transition-colors whitespace-nowrap flex items-center gap-1.5',
                      selected ? 'bg-indigo-600 text-white shadow-sm' : 'bg-slate-800/80 text-slate-400 hover:text-slate-200 hover:bg-slate-800'
                    )}
                  >
                    {res.error_message && <AlertTriangle className="h-3 w-3 text-rose-400" aria-label="failed" />}
                    {res.model}
                    {job.winning_model === res.model && (
                      <span className="text-[10px] bg-emerald-400 text-slate-950 px-1 rounded font-bold">WINNER</span>
                    )}
                  </button>
                );
              })}
            </div>

            {current && (
              <div id="result-panel" role="tabpanel" aria-labelledby={`result-tab-${results.indexOf(current)}`} className="mt-4 space-y-4">
                {current.error_message ? (
                  <div role="alert" className="p-3.5 rounded-xl bg-rose-950/30 border border-rose-500/30">
                    <p className="text-xs font-semibold text-rose-300 flex items-center gap-1.5">
                      <AlertTriangle className="h-3.5 w-3.5" aria-hidden="true" /> This model call failed
                    </p>
                    <pre className="mt-2 text-[11px] text-rose-200/90 font-mono whitespace-pre-wrap break-words">{current.error_message}</pre>
                  </div>
                ) : (
                  <>
                    <dl className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                      <Metric icon={Clock} iconClass="text-sky-400" label="Latency / TTFT">
                        {current.latency_ms}ms / {current.ttft_ms}ms
                      </Metric>
                      <Metric icon={Gauge} iconClass="text-indigo-400" label={current.graded ? 'Semantic score' : 'Heuristic score'}>
                        <span className="text-emerald-400">{formatPercent(current.semantic_score)}</span>
                        {current.graded && current.drift_percent > 0 && (
                          <span className={cn('ml-1 text-[10px]', current.regression_alert ? 'text-rose-400' : 'text-slate-400')}>
                            −{current.drift_percent}%
                          </span>
                        )}
                      </Metric>
                      <Metric icon={DollarSign} iconClass="text-emerald-400" label="Cost (tokens)">
                        {formatUSD(current.cost_usd, 5)}{' '}
                        <span className="text-slate-400 font-normal">({(current.prompt_tokens || 0) + (current.completion_tokens || 0)}t)</span>
                      </Metric>
                      <Metric icon={Target} iconClass="text-amber-400" label="Exact match">
                        {!current.graded ? <span className="text-slate-500">n/a</span> : current.exact_match ? '✓ match' : '✕ no match'}
                      </Metric>
                    </dl>

                    {current.simulated && (
                      <p className="flex items-start gap-2 text-[11px] text-amber-300/90 bg-amber-950/20 border border-amber-500/20 rounded-lg px-3 py-2">
                        <FlaskConical className="h-3.5 w-3.5 mt-px flex-shrink-0" aria-hidden="true" />
                        Simulated response: no API key was configured for this provider, so an offline mock was used.
                      </p>
                    )}

                    <div className="bg-slate-950 rounded-xl border border-slate-800 p-3.5">
                      <div className="flex items-center justify-between mb-2">
                        <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Response</span>
                        <button type="button" onClick={() => handleCopy(current.response || '')} className="btn-ghost px-2 py-1">
                          {copied ? <Check className="h-3.5 w-3.5 text-emerald-400" aria-hidden="true" /> : <Copy className="h-3.5 w-3.5" aria-hidden="true" />}
                          <span aria-live="polite">{copied ? 'Copied' : 'Copy'}</span>
                        </button>
                      </div>
                      <pre className="text-xs text-slate-200 font-mono whitespace-pre-wrap break-words bg-slate-900/60 p-3 rounded-lg max-h-72 overflow-y-auto border border-slate-800">
                        {current.response || 'No response text captured.'}
                      </pre>
                    </div>
                  </>
                )}
              </div>
            )}
          </div>
        )}
      </div>
    </Modal>
  );
}
