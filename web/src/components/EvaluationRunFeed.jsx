import React, { useState } from 'react';
import { History, CheckCircle2, ChevronRight, Eye, Sparkles } from 'lucide-react';

export default function EvaluationRunFeed({ jobs, onSelectJob }) {
  if (!jobs || jobs.length === 0) {
    return (
      <div className="bg-slate-900 border border-slate-800 rounded-2xl p-8 text-center">
        <History className="h-8 w-8 text-slate-600 mx-auto mb-2" />
        <p className="text-sm text-slate-400">No evaluation runs recorded yet.</p>
        <p className="text-xs text-slate-500 mt-1">Run a benchmark to populate the live distributed stream.</p>
      </div>
    );
  }

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
      <div className="px-5 py-4 border-b border-slate-800 flex items-center justify-between">
        <div className="flex items-center space-x-2">
          <History className="h-4 w-4 text-emerald-400" />
          <h3 className="text-sm font-semibold text-white">Live Benchmark Execution Stream</h3>
        </div>
        <span className="text-[11px] text-slate-400 font-mono">
          Showing {jobs.length} recent runs
        </span>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs text-slate-300">
          <thead className="bg-slate-950/60 text-slate-400 uppercase font-mono text-[10px] border-b border-slate-800">
            <tr>
              <th className="px-4 py-3">Job ID</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3">Prompt</th>
              <th className="px-4 py-3">Models</th>
              <th className="px-4 py-3">Winning Model</th>
              <th className="px-4 py-3 text-right">Cost (USD)</th>
              <th className="px-4 py-3 text-center">Inspect</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800/60 font-sans">
            {jobs.map(job => {
              const isCompleted = job.status === 'completed';

              return (
                <tr key={job.job_id} className="hover:bg-slate-850/50 transition-colors">
                  
                  {/* Job ID */}
                  <td className="px-4 py-3 font-mono text-slate-400 text-[11px]">
                    {job.job_id.slice(0, 11)}...
                  </td>

                  {/* Status */}
                  <td className="px-4 py-3">
                    <span
                      className={`inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold border ${
                        isCompleted
                          ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                          : 'bg-amber-500/10 text-amber-400 border-amber-500/20'
                      }`}
                    >
                      {isCompleted ? (
                        <CheckCircle2 className="h-2.5 w-2.5 mr-1" />
                      ) : (
                        <span className="h-1.5 w-1.5 rounded-full bg-amber-400 mr-1 animate-ping" />
                      )}
                      {job.status}
                    </span>
                  </td>

                  {/* Prompt Preview */}
                  <td className="px-4 py-3 max-w-xs truncate text-slate-200">
                    {job.prompt}
                  </td>

                  {/* Target Models */}
                  <td className="px-4 py-3 text-slate-400">
                    <div className="flex items-center space-x-1">
                      <span className="font-mono">{job.target_models?.length || 0}</span>
                      <span className="text-[10px] text-slate-500">providers</span>
                    </div>
                  </td>

                  {/* Winning Model */}
                  <td className="px-4 py-3">
                    {job.winning_model ? (
                      <span className="inline-flex items-center space-x-1 text-emerald-400 font-mono text-[11px]">
                        <Sparkles className="h-3 w-3" />
                        <span>{job.winning_model}</span>
                      </span>
                    ) : (
                      <span className="text-slate-500 font-mono text-[11px]">Evaluating...</span>
                    )}
                  </td>

                  {/* Cost */}
                  <td className="px-4 py-3 text-right font-mono text-slate-300 font-medium">
                    ${(job.total_cost_usd || 0).toFixed(5)}
                  </td>

                  {/* Action */}
                  <td className="px-4 py-3 text-center">
                    <button
                      onClick={() => onSelectJob(job)}
                      className="p-1 rounded hover:bg-slate-800 text-slate-400 hover:text-indigo-400 transition-colors"
                      title="View Detailed Results"
                    >
                      <Eye className="h-4 w-4" />
                    </button>
                  </td>

                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
