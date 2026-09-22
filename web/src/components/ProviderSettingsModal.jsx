import React, { useState } from 'react';
import { CheckCircle2, AlertCircle, Loader2, Shield, ExternalLink, PlugZap, Eye, EyeOff } from 'lucide-react';
import Modal from './Modal';
import { apiFetch } from '../lib/api';
import { cn } from '../lib/format';

// keyName values must match the provider families resolved by the backend
// (internal/models.Family) so a saved key is actually used for evaluations.
const PROVIDERS = [
  {
    id: 'gemini',
    name: 'Google Gemini',
    models: 'Gemini 1.5 Pro · 1.5 Flash',
    pricing: '$0.075 / $0.30 per MTok (Flash)',
    keyName: 'gemini',
    placeholder: 'AIzaSy…',
    docsUrl: 'https://aistudio.google.com/app/apikey',
  },
  {
    id: 'openai',
    name: 'OpenAI',
    models: 'GPT-4o · GPT-4o-mini',
    pricing: '$0.15 / $0.60 per MTok (Mini)',
    keyName: 'openai',
    placeholder: 'sk-proj-…',
    docsUrl: 'https://platform.openai.com/api-keys',
  },
  {
    id: 'anthropic',
    name: 'Anthropic Claude',
    models: 'Claude 3.5 Sonnet · Haiku',
    pricing: '$3.00 / $15.00 per MTok (Sonnet)',
    keyName: 'anthropic',
    placeholder: 'sk-ant-…',
    docsUrl: 'https://console.anthropic.com/settings/keys',
  },
  {
    id: 'ollama',
    name: 'Local Ollama / vLLM',
    models: 'Llama 3 · DeepSeek · Phi-3',
    pricing: '$0.00 (self-hosted)',
    keyName: 'ollama_endpoint',
    isEndpoint: true,
    placeholder: 'http://localhost:11434',
    docsUrl: 'https://ollama.com',
  },
];

function ProviderRow({ provider: p, value, onChange }) {
  const [status, setStatus] = useState(null);
  const [reveal, setReveal] = useState(false);
  const inputId = `provider-${p.id}`;
  const statusId = `${inputId}-status`;

  const test = async () => {
    setStatus({ loading: true });
    try {
      const data = await apiFetch('/api/v1/providers/validate', {
        method: 'POST',
        body: {
          provider: p.id,
          api_key: p.isEndpoint ? '' : value,
          endpoint_url: p.isEndpoint ? value : '',
        },
        timeoutMs: 15000,
      });
      setStatus({ success: true, message: `Connected in ${data.latency_ms} ms. Ready for evaluation.` });
    } catch (err) {
      setStatus({ success: false, message: err.data?.message || err.message });
    }
  };

  const canTest = p.isEndpoint || value.trim().length > 0;

  return (
    <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 hover:border-slate-700 transition-colors">
      <div className="flex flex-wrap items-start justify-between gap-2 mb-2">
        <div>
          <label htmlFor={inputId} className="text-sm font-semibold text-slate-200">
            {p.name}
          </label>
          <p className="text-[11px] text-slate-400">{p.models}</p>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-mono text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/20">
            {p.pricing}
          </span>
          <a
            href={p.docsUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="icon-btn p-1 hover:text-indigo-400"
            aria-label={`Get a ${p.name} ${p.isEndpoint ? 'setup guide' : 'API key'} (opens in new tab)`}
          >
            <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
          </a>
        </div>
      </div>

      <div className="flex gap-2">
        <div className="relative flex-1">
          <input
            id={inputId}
            type={p.isEndpoint || reveal ? 'text' : 'password'}
            value={value}
            onChange={e => {
              onChange(e.target.value);
              setStatus(null);
            }}
            placeholder={p.placeholder}
            autoComplete="off"
            spellCheck={false}
            aria-describedby={status ? statusId : undefined}
            aria-invalid={status && !status.loading && !status.success ? true : undefined}
            className={cn('input font-mono', !p.isEndpoint && 'pr-9')}
          />
          {!p.isEndpoint && value && (
            <button
              type="button"
              onClick={() => setReveal(r => !r)}
              className="absolute inset-y-0 right-0 px-2.5 text-slate-500 hover:text-slate-300"
              aria-label={reveal ? 'Hide key' : 'Show key'}
            >
              {reveal ? <EyeOff className="h-3.5 w-3.5" aria-hidden="true" /> : <Eye className="h-3.5 w-3.5" aria-hidden="true" />}
            </button>
          )}
        </div>

        <button type="button" onClick={test} disabled={status?.loading || !canTest} className="btn-secondary">
          {status?.loading ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin text-indigo-400" aria-hidden="true" />
          ) : (
            <PlugZap className="h-3.5 w-3.5 text-indigo-400" aria-hidden="true" />
          )}
          Test
        </button>
      </div>

      {status && !status.loading && (
        <p id={statusId} role="status" className={cn('mt-2 flex items-start gap-1.5 text-xs', status.success ? 'text-emerald-400' : 'text-rose-400')}>
          {status.success ? (
            <CheckCircle2 className="h-3.5 w-3.5 flex-shrink-0 mt-px" aria-hidden="true" />
          ) : (
            <AlertCircle className="h-3.5 w-3.5 flex-shrink-0 mt-px" aria-hidden="true" />
          )}
          <span className="break-words min-w-0">{status.message}</span>
        </p>
      )}
    </div>
  );
}

export default function ProviderSettingsModal({ onClose, keys, onSaveKeys }) {
  const [formData, setFormData] = useState(() => ({ ...keys }));
  const hasAny = Object.values(formData).some(v => v && v.trim());

  const handleSave = e => {
    e.preventDefault();
    onSaveKeys(formData);
    onClose();
  };

  return (
    <Modal
      size="lg"
      title="Provider accounts"
      badge={<span className="px-2 py-0.5 text-[10px] font-bold rounded bg-indigo-500/20 text-indigo-300 border border-indigo-500/30">BYOK</span>}
      description="Bring your own keys. Runs are billed directly by each provider with zero markup. Providers without a key run in offline simulation."
      onClose={onClose}
      footer={
        <div className="flex flex-col-reverse sm:flex-row sm:items-center sm:justify-between gap-2">
          <button type="button" onClick={() => setFormData({})} disabled={!hasAny} className="btn-danger-ghost self-start">
            Clear all
          </button>
          <div className="flex gap-2 justify-end">
            <button type="button" onClick={onClose} className="btn-ghost">
              Cancel
            </button>
            <button type="submit" form="provider-form" className="btn bg-indigo-600 hover:bg-indigo-500 text-white">
              Save keys
            </button>
          </div>
        </div>
      }
    >
      <form id="provider-form" onSubmit={handleSave} className="space-y-4">
        <div className="p-3 rounded-xl bg-emerald-950/40 border border-emerald-500/30 flex items-start gap-3 text-xs text-emerald-200">
          <Shield className="h-4 w-4 text-emerald-400 mt-0.5 flex-shrink-0" aria-hidden="true" />
          <p>
            <span className="font-semibold text-emerald-300">Where your keys go: </span>
            they are saved in this browser&apos;s local storage and sent only to your own EvalPulse API when a benchmark runs.
            Workers delete the job message (and its keys) from Redis as soon as it is processed. Keys are never logged or stored in results.
          </p>
        </div>

        {PROVIDERS.map(p => (
          <ProviderRow
            key={p.id}
            provider={p}
            value={formData[p.keyName] || ''}
            onChange={v => setFormData(prev => ({ ...prev, [p.keyName]: v }))}
          />
        ))}
      </form>
    </Modal>
  );
}
