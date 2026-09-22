import React from 'react';
import { Activity, Database, KeyRound, Play, ShieldCheck, ShieldAlert } from 'lucide-react';
import { cn } from '../lib/format';

function StatusPill({ icon: Icon, iconClass, label, value, valueClass, title }) {
  return (
    <div
      title={title}
      className="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-slate-800/80 border border-slate-700/60 text-slate-300 whitespace-nowrap"
    >
      <Icon className={cn('h-3.5 w-3.5', iconClass)} aria-hidden="true" />
      <span className="text-slate-400">{label}</span>
      <span className={cn('font-semibold font-mono', valueClass)}>{value}</span>
    </div>
  );
}

function backendStatus(apiStatus, health) {
  if (apiStatus === 'loading') return { value: 'Connecting…', cls: 'text-slate-400', dot: 'bg-slate-500' };
  if (apiStatus === 'offline') return { value: 'API offline', cls: 'text-rose-400', dot: 'bg-rose-500' };
  if (health?.redis === 'connected') return { value: 'Redis stream', cls: 'text-emerald-400', dot: 'bg-emerald-400' };
  if (health?.redis === 'unreachable') return { value: 'Redis down', cls: 'text-amber-400', dot: 'bg-amber-400' };
  return { value: 'Standalone', cls: 'text-sky-400', dot: 'bg-sky-400' };
}

export default function Navbar({ onOpenBYOK, onOpenTrigger, connectedCount, regression, health, apiStatus }) {
  const backend = backendStatus(apiStatus, health);
  const gateKnown = regression && typeof regression.passed === 'boolean';
  const gateFailed = gateKnown && !regression.passed;

  return (
    <header className="border-b border-slate-800 bg-slate-900/90 backdrop-blur sticky top-0 z-40">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0">
          <div className="h-9 w-9 flex-shrink-0 rounded-lg bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-400">
            <Activity className="h-5 w-5" aria-hidden="true" />
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <span className="text-lg font-bold tracking-tight text-white">EvalPulse</span>
              <span className="relative flex h-2 w-2" title={backend.value} aria-hidden="true">
                {apiStatus === 'ok' && <span className={cn('absolute inline-flex h-full w-full rounded-full opacity-60 animate-ping', backend.dot)} />}
                <span className={cn('relative inline-flex h-2 w-2 rounded-full', backend.dot)} />
              </span>
            </div>
            <p className="text-[11px] text-slate-400 hidden lg:block truncate">Multi-model LLM evaluation &amp; regression pipeline</p>
          </div>
        </div>

        <div className="hidden md:flex items-center gap-2 text-xs" aria-label="System status">
          <StatusPill
            icon={Database}
            iconClass="text-sky-400"
            label="Backend"
            value={backend.value}
            valueClass={backend.cls}
            title="Live status from /healthz"
          />
          <StatusPill
            icon={gateFailed ? ShieldAlert : ShieldCheck}
            iconClass={gateFailed ? 'text-rose-400' : 'text-emerald-400'}
            label="Gate"
            value={!gateKnown ? '—' : gateFailed ? 'FAIL' : 'PASS'}
            valueClass={!gateKnown ? 'text-slate-400' : gateFailed ? 'text-rose-400' : 'text-emerald-400'}
            title="CI/CD regression gate result"
          />
        </div>

        <div className="flex items-center gap-2 flex-shrink-0">
          <button type="button" onClick={onOpenBYOK} className="btn-secondary px-2.5 sm:px-3" aria-label={`Provider keys, ${connectedCount} configured`}>
            <KeyRound className="h-3.5 w-3.5 text-indigo-400" aria-hidden="true" />
            <span className="hidden sm:inline">Provider keys</span>
            <span className="px-1.5 rounded-full text-[10px] bg-indigo-500/20 text-indigo-300 font-mono">{connectedCount}</span>
          </button>

          <button type="button" onClick={onOpenTrigger} className="btn-primary px-2.5 sm:px-3.5" aria-label="Run benchmark">
            <Play className="h-3.5 w-3.5 fill-current" aria-hidden="true" />
            <span>
              Run<span className="hidden sm:inline"> benchmark</span>
            </span>
          </button>
        </div>
      </div>
    </header>
  );
}
