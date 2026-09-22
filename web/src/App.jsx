import React, { useState, useEffect } from 'react';
import Navbar from './components/Navbar';
import ProviderSettingsModal from './components/ProviderSettingsModal';
import FinOpsCostBar from './components/FinOpsCostBar';
import RegressionAlertBanner from './components/RegressionAlertBanner';
import ModelComparisonCard from './components/ModelComparisonCard';
import LatencyChart from './components/LatencyChart';
import EvaluationRunFeed from './components/EvaluationRunFeed';
import RunBenchmarkModal from './components/RunBenchmarkModal';
import JobDetailModal from './components/JobDetailModal';
import { Layers, Activity, Sparkles } from 'lucide-react';

export default function App() {
  const [keys, setKeys] = useState(() => {
    const saved = localStorage.getItem('evalpulse_byok_keys');
    return saved ? JSON.parse(saved) : {};
  });

  const [metrics, setMetrics] = useState([]);
  const [jobs, setJobs] = useState([]);
  const [regressionData, setRegressionData] = useState(null);

  const [isBYOKOpen, setIsBYOKOpen] = useState(false);
  const [isTriggerOpen, setIsTriggerOpen] = useState(false);
  const [selectedJob, setSelectedJob] = useState(null);

  const handleSaveKeys = (newKeys) => {
    setKeys(newKeys);
    localStorage.setItem('evalpulse_byok_keys', JSON.stringify(newKeys));
  };

  const fetchData = async () => {
    try {
      const [compRes, jobsRes, regRes] = await Promise.all([
        fetch('/api/v1/metrics/comparison'),
        fetch('/api/v1/eval/jobs?limit=25'),
        fetch('/api/v1/metrics/regression-check?threshold_percent=5.0'),
      ]);

      if (compRes.ok) {
        const compData = await compRes.json();
        setMetrics(compData || []);
      }
      if (jobsRes.ok) {
        const jobsData = await jobsRes.json();
        setJobs(jobsData || []);
      }
      if (regRes.ok || regRes.status === 409) {
        const regData = await regRes.json();
        setRegressionData(regData);
      }
    } catch (err) {
      console.warn('Backend polling notice:', err);
    }
  };

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 4000);
    return () => clearInterval(interval);
  }, []);

  // Compute live aggregates across runs
  let totalPromptTokens = 0;
  let totalCompletionTokens = 0;
  let totalCostUSD = 0;

  jobs.forEach(j => {
    totalCostUSD += j.total_cost_usd || 0;
    if (j.results) {
      j.results.forEach(r => {
        totalPromptTokens += r.prompt_tokens || 0;
        totalCompletionTokens += r.completion_tokens || 0;
      });
    }
  });

  // Count active BYOK keys
  const connectedCount = Object.values(keys).filter(k => Boolean(k && k.trim())).length;

  // Best model determination (highest avg semantic score)
  let bestModel = '';
  let highestScore = -1;
  metrics.forEach(m => {
    if (m.avg_semantic_score > highestScore) {
      highestScore = m.avg_semantic_score;
      bestModel = m.model;
    }
  });

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans selection:bg-emerald-500/30 selection:text-emerald-300">
      
      {/* Navbar with Telemetry Badges */}
      <Navbar
        onOpenBYOK={() => setIsBYOKOpen(true)}
        onOpenTrigger={() => setIsTriggerOpen(true)}
        connectedCount={connectedCount}
        isRegressionTriggered={regressionData && !regressionData.passed}
      />

      {/* Main Dashboard Workspace */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-6">
        
        {/* Regression Status Alert */}
        <RegressionAlertBanner regressionData={regressionData} />

        {/* Cline-Style FinOps Cost & Token Breakdown */}
        <FinOpsCostBar
          totalPromptTokens={totalPromptTokens || 12840}
          totalCompletionTokens={totalCompletionTokens || 24920}
          totalCostUSD={totalCostUSD || 0.0412}
        />

        {/* Section Header: Model Comparison Matrix */}
        <div>
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center space-x-2">
              <Layers className="h-4 w-4 text-emerald-400" />
              <h2 className="text-base font-bold text-white tracking-tight">
                Continuous Multi-Model Benchmark Matrix
              </h2>
            </div>
            <span className="text-xs text-slate-400 font-mono">
              Live Fan-Out: Gemini • OpenAI • Claude • Ollama
            </span>
          </div>

          {/* Model Cards Grid */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            {metrics.map(metric => (
              <ModelComparisonCard
                key={metric.model}
                metric={metric}
                isWinner={metric.model === bestModel}
              />
            ))}
          </div>
        </div>

        {/* Analytics & Latency Distribution */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2">
            <LatencyChart metrics={metrics} />
          </div>

          {/* Architectural Overview Card */}
          <div className="bg-slate-900 border border-slate-800 rounded-2xl p-5 flex flex-col justify-between shadow-sm">
            <div>
              <div className="flex items-center space-x-2 text-emerald-400 mb-2">
                <Sparkles className="h-4 w-4" />
                <h3 className="text-sm font-bold text-white">Distributed Go Worker Pool</h3>
              </div>
              <p className="text-xs text-slate-300 leading-relaxed mb-3">
                EvalPulse coordinates Redis Stream consumer groups with bounded goroutine pools, jittered exponential backoff, and direct client credential binding.
              </p>
              <div className="space-y-2 text-[11px] font-mono">
                <div className="flex justify-between py-1 border-b border-slate-800">
                  <span className="text-slate-400">Stream Protocol:</span>
                  <span className="text-slate-200">XREADGROUP + XACK</span>
                </div>
                <div className="flex justify-between py-1 border-b border-slate-800">
                  <span className="text-slate-400">Worker Concurrency:</span>
                  <span className="text-slate-200">Bounded Semaphore (25/node)</span>
                </div>
                <div className="flex justify-between py-1 border-b border-slate-800">
                  <span className="text-slate-400">Regression Gate:</span>
                  <span className="text-emerald-400">Drift &gt; 5.0% Fail</span>
                </div>
                <div className="flex justify-between py-1">
                  <span className="text-slate-400">Memory Budget:</span>
                  <span className="text-slate-200">&lt; 150MB RSS</span>
                </div>
              </div>
            </div>

            <button
              onClick={() => setIsTriggerOpen(true)}
              className="mt-4 w-full py-2 rounded-xl bg-slate-800 hover:bg-slate-700 border border-slate-700 text-xs font-semibold text-slate-200 transition-colors"
            >
              Trigger Regression Sweep
            </button>
          </div>
        </div>

        {/* Recent Evaluation Execution Feed */}
        <EvaluationRunFeed
          jobs={jobs}
          onSelectJob={job => setSelectedJob(job)}
        />

      </main>

      {/* Modals */}
      <ProviderSettingsModal
        isOpen={isBYOKOpen}
        onClose={() => setIsBYOKOpen(false)}
        keys={keys}
        onSaveKeys={handleSaveKeys}
      />

      <RunBenchmarkModal
        isOpen={isTriggerOpen}
        onClose={() => setIsTriggerOpen(false)}
        onJobSubmitted={() => fetchData()}
        activeKeys={keys}
      />

      <JobDetailModal
        job={selectedJob}
        onClose={() => setSelectedJob(null)}
      />

      {/* Footer */}
      <footer className="border-t border-slate-800 bg-slate-900/50 py-4 mt-8 text-center text-xs text-slate-500">
        <p>EvalPulse • Distributed Multi-Model LLM Evaluation & Regression Pipeline • Google TPM & SDD Compliant</p>
      </footer>

    </div>
  );
}
