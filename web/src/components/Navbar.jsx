import React from 'react';
import { Activity, Cpu, Database, KeyRound, Play, ShieldCheck, Zap } from 'lucide-react';

export default function Navbar({ onOpenBYOK, onOpenTrigger, connectedCount, isRegressionTriggered }) {
  return (
    <header className="border-b border-slate-800 bg-slate-900/90 backdrop-blur sticky top-0 z-40">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
        
        {/* Brand & Logo */}
        <div className="flex items-center space-x-3">
          <div className="h-9 w-9 rounded-lg bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-400 shadow-sm shadow-emerald-950">
            <Activity className="h-5 w-5 animate-pulse" />
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <span className="text-lg font-bold tracking-tight text-white">EvalPulse</span>
              <span className="px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                v1.0 GA
              </span>
            </div>
            <p className="text-[11px] text-slate-400 hidden sm:block">
              Distributed Multi-Model LLM Evaluation & Regression Pipeline
            </p>
          </div>
        </div>

        {/* System Telemetry Badges */}
        <div className="hidden md:flex items-center space-x-3 text-xs">
          <div className="flex items-center space-x-1.5 px-2.5 py-1 rounded-full bg-slate-800/80 border border-slate-700/60 text-slate-300">
            <Database className="h-3.5 w-3.5 text-sky-400" />
            <span>Redis Stream:</span>
            <span className="font-semibold text-emerald-400 font-mono">eval:jobs</span>
          </div>

          <div className="flex items-center space-x-1.5 px-2.5 py-1 rounded-full bg-slate-800/80 border border-slate-700/60 text-slate-300">
            <Cpu className="h-3.5 w-3.5 text-amber-400" />
            <span>Workers:</span>
            <span className="font-semibold text-white font-mono">2 Active</span>
          </div>

          <div className="flex items-center space-x-1.5 px-2.5 py-1 rounded-full bg-slate-800/80 border border-slate-700/60 text-slate-300">
            <ShieldCheck className="h-3.5 w-3.5 text-emerald-400" />
            <span>Gate:</span>
            <span className={`font-semibold ${isRegressionTriggered ? 'text-rose-400' : 'text-emerald-400'}`}>
              {isRegressionTriggered ? 'ALERT (Exit 1)' : 'PASS (Exit 0)'}
            </span>
          </div>
        </div>

        {/* Action Controls */}
        <div className="flex items-center space-x-2 sm:space-x-3">
          {/* Cline BYOK Button */}
          <button
            onClick={onOpenBYOK}
            className="flex items-center space-x-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 text-xs font-medium text-slate-200 transition-colors shadow-sm"
          >
            <KeyRound className="h-3.5 w-3.5 text-indigo-400" />
            <span>BYOK Accounts</span>
            <span className="px-1.5 py-0.2 rounded-full text-[10px] bg-indigo-500/20 text-indigo-300 font-mono">
              {connectedCount}
            </span>
          </button>

          {/* Trigger Benchmark Button */}
          <button
            onClick={onOpenTrigger}
            className="flex items-center space-x-1.5 px-3.5 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-xs font-semibold text-white transition-all shadow-sm shadow-emerald-900/40 active:scale-95"
          >
            <Play className="h-3.5 w-3.5 fill-current" />
            <span>Run Benchmark</span>
          </button>
        </div>

      </div>
    </header>
  );
}
