import React, { Suspense, lazy, useCallback, useMemo, useState } from 'react';
import { Layers, Sparkles, WifiOff, RefreshCw, Play } from 'lucide-react';
import Navbar from './components/Navbar';
import ProviderSettingsModal from './components/ProviderSettingsModal';
import FinOpsCostBar from './components/FinOpsCostBar';
import RegressionAlertBanner from './components/RegressionAlertBanner';
import ModelComparisonCard, { ModelComparisonCardSkeleton } from './components/ModelComparisonCard';
import EvaluationRunFeed from './components/EvaluationRunFeed';
import RunBenchmarkModal from './components/RunBenchmarkModal';
import JobDetailModal from './components/JobDetailModal';
import ErrorBoundary from './components/ErrorBoundary';
import { useDashboardData } from './hooks/useDashboardData';
import { readJSON, writeJSON, removeKey } from './lib/storage';
import { formatRelativeTime } from './lib/format';

// Recharts is the largest dependency; load it after the rest of the dashboard.
const LatencyChart = lazy(() => import('./components/LatencyChart'));

const KEYS_STORAGE_KEY = 'evalpulse_byok_keys';

function loadKeys() {
  const saved = readJSON(KEYS_STORAGE_KEY, {});
  if (!saved || typeof saved !== 'object' || Array.isArray(saved)) return {};
  // Only keep string values; anything else is a corrupted entry.
  return Object.fromEntries(Object.entries(saved).filter(([, v]) => typeof v === 'string'));
}

export default function App() {
  const { metrics, jobs, regression, health, status, error, lastUpdated, refresh } = useDashboardData();

  const [keys, setKeys] = useState(loadKeys);
  const [isBYOKOpen, setIsBYOKOpen] = useState(false);
  const [isTriggerOpen, setIsTriggerOpen] = useState(false);
  const [selectedJobId, setSelectedJobId] = useState(null);
  // Keep the last known version so the modal stays open if the job scrolls out of the feed.
  const [selectedJobSnapshot, setSelectedJobSnapshot] = useState(null);

  const handleSaveKeys = useCallback(newKeys => {
    const cleaned = Object.fromEntries(
      Object.entries(newKeys)
        .map(([k, v]) => [k, typeof v === 'string' ? v.trim() : ''])
        .filter(([, v]) => v)
    );
    setKeys(cleaned);
    if (Object.keys(cleaned).length) writeJSON(KEYS_STORAGE_KEY, cleaned);
    else removeKey(KEYS_STORAGE_KEY);
  }, []);

  const totals = useMemo(() => {
    let promptTokens = 0;
    let completionTokens = 0;
    let costUSD = 0;
    for (const j of jobs) {
      costUSD += j.total_cost_usd || 0;
      for (const r of j.results || []) {
        promptTokens += r.prompt_tokens || 0;
        completionTokens += r.completion_tokens || 0;
      }
    }
    return { promptTokens, completionTokens, costUSD, runs: jobs.length };
  }, [jobs]);

  // Same rule as the backend's per-job winner: highest score, ties broken by lower latency.
  const bestModel = useMemo(() => {
    const score = m => (m.graded_evals > 0 ? m.avg_graded_score : m.avg_semantic_score);
    let best = null;
    for (const m of metrics) {
      if (m.total_evals - (m.failed_evals || 0) <= 0) continue;
      if (!best || score(m) > score(best) || (score(m) === score(best) && m.p50_latency_ms < best.p50_latency_ms)) best = m;
    }
    return best?.model ?? '';
  }, [metrics]);

  const connectedCount = Object.values(keys).filter(Boolean).length;
  const selectedJob = jobs.find(j => j.job_id === selectedJobId) || (selectedJobSnapshot?.job_id === selectedJobId ? selectedJobSnapshot : null);
  const isLoading = status === 'loading';
  const isOffline = status === 'offline';

  const openJob = useCallback(job => {
    setSelectedJobId(job.job_id);
    setSelectedJobSnapshot(job);
  }, []);

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans selection:bg-emerald-500/30 selection:text-emerald-300">
      <a href="#main" className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50 btn-primary">
        Skip to content
      </a>

      <Navbar
        onOpenBYOK={() => setIsBYOKOpen(true)}
        onOpenTrigger={() => setIsTriggerOpen(true)}
        connectedCount={connectedCount}
        regression={regression}
        health={health}
        apiStatus={status}
      />

      <main id="main" className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-6">
        {isOffline && (
          <div role="alert" className="surface border-amber-500/40 bg-amber-950/20 p-4 flex flex-col sm:flex-row sm:items-center gap-3">
            <WifiOff className="h-5 w-5 text-amber-400 flex-shrink-0" aria-hidden="true" />
            <div className="flex-1">
              <p className="text-sm font-semibold text-white">Connection to the EvalPulse API lost</p>
              <p className="text-xs text-slate-400">
                {error} {lastUpdated ? `Showing data from ${formatRelativeTime(lastUpdated.toISOString())}.` : ''} Retrying automatically.
              </p>
            </div>
            <button type="button" onClick={refresh} className="btn-secondary self-start sm:self-auto">
              <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" /> Retry now
            </button>
          </div>
        )}

        <ErrorBoundary name="Regression gate" resetKeys={[regression]}>
          <RegressionAlertBanner regressionData={regression} loading={isLoading} />
        </ErrorBoundary>

        <ErrorBoundary name="FinOps ledger" resetKeys={[jobs]}>
          <FinOpsCostBar
            totalPromptTokens={totals.promptTokens}
            totalCompletionTokens={totals.completionTokens}
            totalCostUSD={totals.costUSD}
            runCount={totals.runs}
            loading={isLoading}
          />
        </ErrorBoundary>

        <section aria-labelledby="matrix-heading">
          <div className="flex flex-col sm:flex-row sm:items-end sm:justify-between gap-1 mb-3">
            <div className="flex items-center gap-2">
              <Layers className="h-4 w-4 text-emerald-400" aria-hidden="true" />
              <h2 id="matrix-heading" className="text-base font-bold text-white tracking-tight">
                Model Benchmark Matrix
              </h2>
            </div>
            <p className="text-xs text-slate-400">
              Aggregated over the latest 500 runs · failed calls excluded from latency &amp; score
            </p>
          </div>

          <ErrorBoundary name="Model comparison" resetKeys={[metrics]}>
            {isLoading ? (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                {[0, 1, 2, 3].map(i => (
                  <ModelComparisonCardSkeleton key={i} />
                ))}
              </div>
            ) : metrics.length === 0 ? (
              <div className="surface p-8 text-center">
                <Sparkles className="h-8 w-8 text-slate-600 mx-auto mb-2" aria-hidden="true" />
                <p className="text-sm text-slate-300">No model metrics yet</p>
                <p className="text-xs text-slate-500 mt-1 mb-4">Run your first benchmark to populate the comparison matrix.</p>
                <button type="button" onClick={() => setIsTriggerOpen(true)} className="btn-primary">
                  <Play className="h-3.5 w-3.5 fill-current" aria-hidden="true" /> Run benchmark
                </button>
              </div>
            ) : (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                {metrics.map(metric => (
                  <ModelComparisonCard key={metric.model} metric={metric} isWinner={metric.model === bestModel} />
                ))}
              </div>
            )}
          </ErrorBoundary>
        </section>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2 min-w-0">
            <ErrorBoundary name="Latency chart" resetKeys={[metrics]}>
              <Suspense fallback={<div className="surface p-5 h-[21.5rem]"><div className="skeleton h-full w-full rounded-xl" /></div>}>
                <LatencyChart metrics={metrics} loading={isLoading} />
              </Suspense>
            </ErrorBoundary>
          </div>

          <aside className="surface p-5 flex flex-col justify-between" aria-labelledby="arch-heading">
            <div>
              <div className="flex items-center gap-2 text-emerald-400 mb-2">
                <Sparkles className="h-4 w-4" aria-hidden="true" />
                <h3 id="arch-heading" className="text-sm font-bold text-white">
                  Distributed Go Worker Pool
                </h3>
              </div>
              <p className="text-xs text-slate-300 leading-relaxed mb-3">
                Jobs flow through Redis Stream consumer groups into bounded goroutine pools. Crashed workers' jobs are reclaimed
                automatically and credentials are deleted from the stream once a job is acknowledged.
              </p>
              <dl className="space-y-0 text-[11px] font-mono">
                {[
                  ['Stream protocol', 'XREADGROUP + XACK', 'text-slate-200'],
                  ['Execution', health?.redis === 'connected' ? 'Distributed workers' : 'Standalone (in-process)', 'text-slate-200'],
                  ['Regression gate', `Drift > ${regression?.threshold_percent ?? 5}% fails`, 'text-emerald-400'],
                  ['Crash recovery', 'XAUTOCLAIM reclaim', 'text-slate-200'],
                ].map(([k, v, cls], i, arr) => (
                  <div key={k} className={`flex justify-between gap-3 py-1.5 ${i < arr.length - 1 ? 'border-b border-slate-800' : ''}`}>
                    <dt className="text-slate-400">{k}</dt>
                    <dd className={`${cls} text-right`}>{v}</dd>
                  </div>
                ))}
              </dl>
            </div>

            <button type="button" onClick={() => setIsTriggerOpen(true)} className="btn-secondary mt-4 w-full">
              Trigger regression sweep
            </button>
          </aside>
        </div>

        <ErrorBoundary name="Run feed" resetKeys={[jobs]}>
          <EvaluationRunFeed jobs={jobs} loading={isLoading} onSelectJob={openJob} lastUpdated={lastUpdated} onRefresh={refresh} />
        </ErrorBoundary>
      </main>

      {isBYOKOpen && (
        <ErrorBoundary name="Provider settings">
          <ProviderSettingsModal onClose={() => setIsBYOKOpen(false)} keys={keys} onSaveKeys={handleSaveKeys} />
        </ErrorBoundary>
      )}

      {isTriggerOpen && (
        <ErrorBoundary name="Benchmark dispatcher">
          <RunBenchmarkModal
            onClose={() => setIsTriggerOpen(false)}
            onJobSubmitted={refresh}
            activeKeys={keys}
            onOpenBYOK={() => {
              setIsTriggerOpen(false);
              setIsBYOKOpen(true);
            }}
          />
        </ErrorBoundary>
      )}

      {selectedJob && (
        <ErrorBoundary name="Job details" resetKeys={[selectedJob]}>
          <JobDetailModal key={selectedJob.job_id} job={selectedJob} onClose={() => setSelectedJobId(null)} />
        </ErrorBoundary>
      )}

      <footer className="border-t border-slate-800 bg-slate-900/50 py-4 mt-8 text-center text-xs text-slate-500">
        <p>EvalPulse · Distributed Multi-Model LLM Evaluation &amp; Regression Pipeline{health?.version ? ` · v${health.version}` : ''}</p>
      </footer>
    </div>
  );
}
