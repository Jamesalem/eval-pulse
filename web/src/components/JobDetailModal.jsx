import React, { useState } from 'react';
import { X, Clock, Gauge, DollarSign, CheckCircle, AlertTriangle, Copy, Check } from 'lucide-react';

export default function JobDetailModal({ job, onClose }) {
  if (!job) return null;

  const [activeTab, setActiveTab] = useState(0);
  const [copied, setCopied] = useState(false);

  const results = job.results || [];
  const currentResult = results[activeTab] || results[0];

  const handleCopy = (text) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-4xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        
        {/* Header */}
        <div className="px-6 py-4 border-b border-slate-800 flex items-center justify-between bg-slate-850">
          <div>
            <div className="flex items-center space-x-2">
              <h3 className="text-base font-semibold text-white">Evaluation Job Telemetry</h3>
              <span className="font-mono text-xs text-slate-400 bg-slate-800 px-2 py-0.5 rounded">
                {job.job_id}
              </span>
            </div>
            <p className="text-xs text-slate-400 mt-0.5">
              Suite: <span className="text-slate-300 font-mono">{job.suite_id || 'ad-hoc'}</span> | Total Job Cost: <span className="text-emerald-400 font-mono font-semibold">${(job.total_cost_usd || 0).toFixed(5)}</span>
            </p>
          </div>
          <button onClick={onClose} className="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800">
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Content Body */}
        <div className="px-6 py-4 overflow-y-auto space-y-4 flex-1">
          
          {/* Prompt Preview */}
          <div className="bg-slate-950 p-3.5 rounded-xl border border-slate-800">
            <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block mb-1">
              Evaluated Prompt
            </span>
            <p className="text-xs text-slate-200 whitespace-pre-wrap">{job.prompt}</p>
          </div>

          {job.ground_truth && (
            <div className="bg-slate-950 p-3 rounded-xl border border-slate-800">
              <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block mb-1">
                Ground Truth Baseline
              </span>
              <p className="text-xs text-slate-300 font-mono whitespace-pre-wrap">{job.ground_truth}</p>
            </div>
          )}

          {/* Model Tabs */}
          {results.length > 0 && (
            <div>
              <div className="flex border-b border-slate-800 space-x-2 overflow-x-auto pb-2">
                {results.map((res, idx) => (
                  <button
                    key={res.model}
                    onClick={() => setActiveTab(idx)}
                    className={`px-3 py-1.5 rounded-lg text-xs font-semibold font-mono transition-colors whitespace-nowrap flex items-center space-x-1.5 ${
                      activeTab === idx
                        ? 'bg-indigo-600 text-white shadow-sm'
                        : 'bg-slate-800/80 text-slate-400 hover:text-slate-200 hover:bg-slate-800'
                    }`}
                  >
                    <span>{res.model}</span>
                    {job.winning_model === res.model && (
                      <span className="text-[10px] bg-emerald-400 text-slate-950 px-1 rounded font-bold">
                        WINNER
                      </span>
                    )}
                  </button>
                ))}
              </div>

              {/* Active Model Metrics Bar */}
              {currentResult && (
                <div className="mt-4 space-y-4">
                  
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                    <div className="bg-slate-950 p-2.5 rounded-xl border border-slate-800">
                      <span className="text-[10px] text-slate-400 block flex items-center space-x-1">
                        <Clock className="h-3 w-3 text-sky-400" />
                        <span>Latency / TTFT</span>
                      </span>
                      <span className="text-xs font-bold text-white font-mono">
                        {currentResult.latency_ms}ms / {currentResult.ttft_ms}ms
                      </span>
                    </div>

                    <div className="bg-slate-950 p-2.5 rounded-xl border border-slate-800">
                      <span className="text-[10px] text-slate-400 block flex items-center space-x-1">
                        <Gauge className="h-3 w-3 text-indigo-400" />
                        <span>Semantic Score</span>
                      </span>
                      <span className="text-xs font-bold text-emerald-400 font-mono">
                        {((currentResult.semantic_score || 0) * 100).toFixed(1)}%
                      </span>
                    </div>

                    <div className="bg-slate-950 p-2.5 rounded-xl border border-slate-800">
                      <span className="text-[10px] text-slate-400 block flex items-center space-x-1">
                        <DollarSign className="h-3 w-3 text-emerald-400" />
                        <span>Cost (Tokens)</span>
                      </span>
                      <span className="text-xs font-bold text-slate-200 font-mono">
                        ${(currentResult.cost_usd || 0).toFixed(5)} ({currentResult.prompt_tokens + currentResult.completion_tokens}t)
                      </span>
                    </div>

                    <div className="bg-slate-950 p-2.5 rounded-xl border border-slate-800">
                      <span className="text-[10px] text-slate-400 block">Exact Match</span>
                      <span className="text-xs font-bold text-slate-200 font-mono">
                        {currentResult.exact_match ? '✓ MATCH' : '✕ NO MATCH'}
                      </span>
                    </div>
                  </div>

                  {/* Response Text Box */}
                  <div className="bg-slate-950 rounded-xl border border-slate-800 p-3.5 relative">
                    <div className="flex items-center justify-between mb-2">
                      <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">
                        Synthesized Response Output
                      </span>
                      <button
                        onClick={() => handleCopy(currentResult.response)}
                        className="text-slate-400 hover:text-white p-1 rounded hover:bg-slate-800 flex items-center space-x-1 text-xs"
                      >
                        {copied ? <Check className="h-3.5 w-3.5 text-emerald-400" /> : <Copy className="h-3.5 w-3.5" />}
                        <span>{copied ? 'Copied' : 'Copy'}</span>
                      </button>
                    </div>
                    <pre className="text-xs text-slate-200 font-mono whitespace-pre-wrap bg-slate-900/60 p-3 rounded-lg max-h-60 overflow-y-auto border border-slate-800">
                      {currentResult.response || currentResult.error_message || 'No response captured.'}
                    </pre>
                  </div>

                </div>
              )}
            </div>
          )}

        </div>

        {/* Footer */}
        <div className="px-6 py-3 border-t border-slate-800 bg-slate-850 flex items-center justify-end">
          <button
            onClick={onClose}
            className="px-4 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs text-slate-300 font-medium transition-colors"
          >
            Close
          </button>
        </div>

      </div>
    </div>
  );
}
