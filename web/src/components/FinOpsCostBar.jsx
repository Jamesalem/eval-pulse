import React from 'react';
import { Coins, TrendingDown } from 'lucide-react';
import { formatInt, formatUSD } from '../lib/format';

// Typical hosted evaluation SaaS markups range from 300% to 500%; 3.5x is used
// as an illustrative midpoint for the savings estimate.
const SAAS_MARKUP_MULTIPLIER = 3.5;

export default function FinOpsCostBar({ totalPromptTokens, totalCompletionTokens, totalCostUSD, runCount, loading }) {
  const directSavings = Math.max(0, totalCostUSD * SAAS_MARKUP_MULTIPLIER - totalCostUSD);

  const tiles = [
    { label: 'Prompt tokens', value: formatInt(totalPromptTokens), cls: 'text-slate-200' },
    { label: 'Completion tokens', value: formatInt(totalCompletionTokens), cls: 'text-slate-200' },
    { label: 'Total spend (USD)', value: formatUSD(totalCostUSD, 5), cls: 'text-emerald-400' },
    {
      label: 'Est. BYOK savings',
      value: `+${formatUSD(directSavings, 4)}`,
      cls: 'text-indigo-400',
      icon: TrendingDown,
      title: `Compared with a hypothetical ${SAAS_MARKUP_MULTIPLIER}x SaaS markup on the same token spend`,
    },
  ];

  return (
    <section className="surface p-4" aria-labelledby="finops-heading">
      <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
        <div className="flex items-center gap-2.5">
          <div className="h-8 w-8 flex-shrink-0 rounded-lg bg-indigo-500/10 border border-indigo-500/30 flex items-center justify-center text-indigo-400">
            <Coins className="h-4 w-4" aria-hidden="true" />
          </div>
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <h2 id="finops-heading" className="text-sm font-semibold text-white">
                FinOps ledger
              </h2>
              <span className="text-[10px] uppercase font-bold text-emerald-400 bg-emerald-500/10 px-1.5 py-0.5 rounded border border-emerald-500/20">
                Zero markup
              </span>
            </div>
            <p className="text-[11px] text-slate-400">
              {loading ? 'Loading usage…' : `Token usage and direct provider spend across the last ${runCount} run${runCount === 1 ? '' : 's'}`}
            </p>
          </div>
        </div>

        <dl className="grid grid-cols-2 sm:grid-cols-4 gap-3 lg:gap-4">
          {tiles.map(({ label, value, cls, icon: Icon, title }) => (
            <div key={label} className="stat-tile min-w-[8.5rem]" title={title}>
              <dt className="text-[11px] text-slate-400 flex items-center gap-1">
                {Icon && <Icon className="h-3 w-3 text-indigo-400" aria-hidden="true" />}
                {label}
              </dt>
              <dd className={`text-sm font-bold font-mono mt-0.5 ${cls}`}>{loading ? <span className="skeleton inline-block h-4 w-16 align-middle" /> : value}</dd>
            </div>
          ))}
        </dl>
      </div>
    </section>
  );
}
