import React, { useState } from 'react';
import { X, Play, Loader2, Sparkles, AlertCircle } from 'lucide-react';

export default function RunBenchmarkModal({ isOpen, onClose, onJobSubmitted, activeKeys }) {
  if (!isOpen) return null;

  const [prompt, setPrompt] = useState('Implement a concurrent worker pool in Go with bounded channels and graceful context teardown.');
  const [groundTruth, setGroundTruth] = useState('package main\n\ntype WorkerPool struct {\n\tsem chan struct{}\n}');
  const [suiteID, setSuiteID] = useState('regression-suite-v1');
  const [selectedModels, setSelectedModels] = useState([
    'gemini-1.5-flash',
    'gpt-4o-mini',
    'claude-3-5-sonnet',
    'ollama:llama3',
  ]);
  const [maxTokens, setMaxTokens] = useState(256);
  const [budgetCap, setBudgetCap] = useState(1.0);
  const [submitting, setSubmitting] = useState(false);
  const [errorMsg, setErrorMsg] = useState('');

  const allAvailableModels = [
    { id: 'gemini-1.5-flash', name: 'Gemini 1.5 Flash (Ultra Fast / Low Cost)' },
    { id: 'gemini-1.5-pro', name: 'Gemini 1.5 Pro (Deep Reasoning)' },
    { id: 'gpt-4o-mini', name: 'OpenAI GPT-4o-mini' },
    { id: 'gpt-4o', name: 'OpenAI GPT-4o' },
    { id: 'claude-3-5-sonnet', name: 'Anthropic Claude 3.5 Sonnet' },
    { id: 'ollama:llama3', name: 'Local Ollama (Llama 3 - Free)' },
  ];

  const toggleModel = (id) => {
    if (selectedModels.includes(id)) {
      if (selectedModels.length > 1) {
        setSelectedModels(selectedModels.filter(m => m !== id));
      }
    } else {
      setSelectedModels([...selectedModels, id]);
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setSubmitting(true);
    setErrorMsg('');

    try {
      const res = await fetch('/api/v1/eval/jobs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          suite_id: suiteID,
          prompt,
          ground_truth: groundTruth,
          target_models: selectedModels,
          max_tokens: Number(maxTokens),
          budget_cap_usd: Number(budgetCap),
          api_keys: activeKeys,
        }),
      });

      const data = await res.json();
      if (res.ok) {
        onJobSubmitted(data.job_id);
        onClose();
      } else {
        setErrorMsg(data.error || 'Failed to submit evaluation job');
      }
    } catch (err) {
      setErrorMsg('Failed to reach EvalPulse API gateway');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        
        {/* Header */}
        <div className="px-6 py-4 border-b border-slate-800 flex items-center justify-between bg-slate-850">
          <div>
            <h3 className="text-base font-semibold text-white">Dispatch Distributed Benchmark Suite</h3>
            <p className="text-xs text-slate-400 mt-0.5">
              Enqueues evaluation tasks to Redis Stream with Cline-style BYOK execution
            </p>
          </div>
          <button onClick={onClose} className="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800">
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Form Content */}
        <form onSubmit={handleSubmit} className="px-6 py-4 overflow-y-auto space-y-4 flex-1">
          {errorMsg && (
            <div className="p-3 rounded-xl bg-rose-950/40 border border-rose-500/30 flex items-center space-x-2 text-xs text-rose-300">
              <AlertCircle className="h-4 w-4 text-rose-400 flex-shrink-0" />
              <span>{errorMsg}</span>
            </div>
          )}

          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1">
              Benchmark Suite Identifier
            </label>
            <input
              type="text"
              value={suiteID}
              onChange={e => setSuiteID(e.target.value)}
              className="w-full px-3 py-1.5 rounded-lg bg-slate-950 border border-slate-700 text-xs text-slate-200 focus:outline-none focus:border-emerald-500 font-mono"
            />
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1">
              Evaluation Prompt
            </label>
            <textarea
              rows={3}
              value={prompt}
              onChange={e => setPrompt(e.target.value)}
              className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-700 text-xs text-slate-200 focus:outline-none focus:border-emerald-500"
              placeholder="Enter benchmark prompt..."
              required
            />
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1">
              Ground Truth / Reference Schema (Optional)
            </label>
            <textarea
              rows={2}
              value={groundTruth}
              onChange={e => setGroundTruth(e.target.value)}
              className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-700 text-xs text-slate-200 focus:outline-none focus:border-emerald-500 font-mono text-[11px]"
              placeholder="Reference answer for Exact Match and Cosine scoring..."
            />
          </div>

          {/* Model Target Selection */}
          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-2">
              Target Models (Parallel Fan-Out)
            </label>
            <div className="space-y-1.5">
              {allAvailableModels.map(m => {
                const isSelected = selectedModels.includes(m.id);
                return (
                  <button
                    type="button"
                    key={m.id}
                    onClick={() => toggleModel(m.id)}
                    className={`w-full flex items-center justify-between px-3 py-2 rounded-xl text-xs border transition-colors ${
                      isSelected
                        ? 'bg-emerald-950/40 border-emerald-500/50 text-emerald-200'
                        : 'bg-slate-950/60 border-slate-800 text-slate-400 hover:border-slate-700'
                    }`}
                  >
                    <span className="font-mono">{m.name}</span>
                    <span className={`h-4 w-4 rounded border flex items-center justify-center text-[10px] ${isSelected ? 'border-emerald-400 bg-emerald-500 text-slate-950 font-bold' : 'border-slate-700'}`}>
                      {isSelected ? '✓' : ''}
                    </span>
                  </button>
                );
              })}
            </div>
          </div>

          {/* Budget & Max Tokens */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-semibold text-slate-300 mb-1">
                Max Output Tokens
              </label>
              <input
                type="number"
                value={maxTokens}
                onChange={e => setMaxTokens(e.target.value)}
                className="w-full px-3 py-1.5 rounded-lg bg-slate-950 border border-slate-700 text-xs text-slate-200 font-mono"
              />
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-300 mb-1">
                Budget Hard-Cap ($ USD)
              </label>
              <input
                type="number"
                step="0.1"
                value={budgetCap}
                onChange={e => setBudgetCap(e.target.value)}
                className="w-full px-3 py-1.5 rounded-lg bg-slate-950 border border-slate-700 text-xs text-slate-200 font-mono"
              />
            </div>
          </div>
        </form>

        {/* Footer */}
        <div className="px-6 py-3 border-t border-slate-800 bg-slate-850 flex items-center justify-end space-x-2">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs text-slate-300 font-medium transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleSubmit}
            disabled={submitting}
            className="px-4 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-xs text-white font-medium shadow-sm transition-colors flex items-center space-x-1.5 disabled:opacity-50"
          >
            {submitting ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Play className="h-3.5 w-3.5 fill-current" />
            )}
            <span>Dispatch to Stream</span>
          </button>
        </div>

      </div>
    </div>
  );
}
