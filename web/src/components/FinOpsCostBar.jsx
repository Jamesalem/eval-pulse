import React from 'react';
import { DollarSign, Coins, TrendingDown, Layers } from 'lucide-react';

export default function FinOpsCostBar({ totalPromptTokens, totalCompletionTokens, totalCostUSD }) {
  // Typical third-party hosted evaluation SaaS markups average 300% to 500%
  const estimatedSaaSBrokerCost = totalCostUSD * 3.5;
  const directSavings = Math.max(0, estimatedSaaSBrokerCost - totalCostUSD);

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-2xl p-4 shadow-sm">
      <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
        
        {/* Title */}
        <div className="flex items-center space-x-2.5">
          <div className="h-8 w-8 rounded-lg bg-indigo-500/10 border border-indigo-500/30 flex items-center justify-center text-indigo-400">
            <Coins className="h-4 w-4" />
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <span className="text-sm font-semibold text-white">Cline-Style FinOps Ledger</span>
              <span className="text-[10px] uppercase font-bold text-emerald-400 bg-emerald-500/10 px-1.5 py-0.5 rounded border border-emerald-500/20">
                Zero Markup
              </span>
            </div>
            <p className="text-[11px] text-slate-400">
              Live token counts & direct upstream billing via your connected accounts
            </p>
          </div>
        </div>

        {/* Metrics Counters */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 lg:gap-6">
          
          <div className="bg-slate-950/60 border border-slate-800/80 rounded-xl px-3 py-2">
            <span className="text-[11px] text-slate-400 block">Prompt Tokens</span>
            <span className="text-sm font-bold text-slate-200 font-mono">
              {totalPromptTokens.toLocaleString()}
            </span>
          </div>

          <div className="bg-slate-950/60 border border-slate-800/80 rounded-xl px-3 py-2">
            <span className="text-[11px] text-slate-400 block">Completion Tokens</span>
            <span className="text-sm font-bold text-slate-200 font-mono">
              {totalCompletionTokens.toLocaleString()}
            </span>
          </div>

          <div className="bg-slate-950/60 border border-slate-800/80 rounded-xl px-3 py-2">
            <span className="text-[11px] text-slate-400 block">Total Spend (USD)</span>
            <span className="text-sm font-bold text-emerald-400 font-mono">
              ${totalCostUSD.toFixed(5)}
            </span>
          </div>

          <div className="bg-slate-950/60 border border-slate-800/80 rounded-xl px-3 py-2">
            <span className="text-[11px] text-indigo-300 block flex items-center space-x-1">
              <TrendingDown className="h-3 w-3 text-indigo-400" />
              <span>BYOK Savings</span>
            </span>
            <span className="text-sm font-bold text-indigo-400 font-mono">
              +${directSavings.toFixed(4)}
            </span>
          </div>

        </div>

      </div>
    </div>
  );
}
