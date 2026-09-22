import React, { useState } from 'react';
import { Play, Loader2, AlertCircle, Check, KeyRound, FlaskConical } from 'lucide-react';
import Modal from './Modal';
import { useToast } from './Toast';
import { apiFetch } from '../lib/api';
import { cn, providerFamily } from '../lib/format';

const AVAILABLE_MODELS = [
  { id: 'gemini-1.5-flash', name: 'Gemini 1.5 Flash', note: 'Fast · low cost' },
  { id: 'gemini-1.5-pro', name: 'Gemini 1.5 Pro', note: 'Deep reasoning' },
  { id: 'gpt-4o-mini', name: 'GPT-4o-mini', note: 'OpenAI' },
  { id: 'gpt-4o', name: 'GPT-4o', note: 'OpenAI' },
  { id: 'claude-3-5-sonnet', name: 'Claude 3.5 Sonnet', note: 'Anthropic' },
  { id: 'ollama:llama3', name: 'Llama 3 (Ollama)', note: 'Local · free' },
];

const MAX_OUTPUT_TOKENS = 8192;
const MAX_PROMPT_CHARS = 32000;

function hasCredential(modelId, keys) {
  const family = providerFamily(modelId);
  if (family === 'ollama') return true; // local daemon needs no key
  return Boolean(keys?.[family]?.trim());
}

export default function RunBenchmarkModal({ onClose, onJobSubmitted, activeKeys, onOpenBYOK }) {
  const { notify } = useToast();
  const [prompt, setPrompt] = useState('Implement a concurrent worker pool in Go with bounded channels and graceful context teardown.');
  const [groundTruth, setGroundTruth] = useState('package main\n\ntype WorkerPool struct {\n\tsem chan struct{}\n}');
  const [suiteID, setSuiteID] = useState('regression-suite-v1');
  const [selectedModels, setSelectedModels] = useState(['gemini-1.5-flash', 'gpt-4o-mini', 'claude-3-5-sonnet', 'ollama:llama3']);
  const [maxTokens, setMaxTokens] = useState('256');
  const [budgetCap, setBudgetCap] = useState('1.00');
  const [submitting, setSubmitting] = useState(false);
  const [errorMsg, setErrorMsg] = useState('');
  const [touched, setTouched] = useState(false);

  const tokens = Number(maxTokens);
  const budget = Number(budgetCap);
  const errors = {
    prompt: !prompt.trim() ? 'A prompt is required.' : prompt.length > MAX_PROMPT_CHARS ? `Keep the prompt under ${MAX_PROMPT_CHARS.toLocaleString()} characters.` : '',
    models: selectedModels.length === 0 ? 'Select at least one model.' : '',
    maxTokens: !Number.isInteger(tokens) || tokens < 1 || tokens > MAX_OUTPUT_TOKENS ? `Enter a whole number from 1 to ${MAX_OUTPUT_TOKENS}.` : '',
    budget: !Number.isFinite(budget) || budget <= 0 ? 'Enter a budget greater than $0.' : '',
  };
  const isValid = !Object.values(errors).some(Boolean);
  const simulatedModels = selectedModels.filter(m => !hasCredential(m, activeKeys));

  const toggleModel = id => {
    setSelectedModels(prev => (prev.includes(id) ? prev.filter(m => m !== id) : [...prev, id]));
  };

  const handleSubmit = async e => {
    e.preventDefault();
    setTouched(true);
    if (!isValid || submitting) return;

    setSubmitting(true);
    setErrorMsg('');
    try {
      // Only send the credentials the selected models actually need.
      const neededFamilies = new Set(selectedModels.map(providerFamily));
      const apiKeys = Object.fromEntries(
        Object.entries(activeKeys || {}).filter(([k]) => neededFamilies.has(k) || (k === 'ollama_endpoint' && neededFamilies.has('ollama')))
      );

      const data = await apiFetch('/api/v1/eval/jobs', {
        method: 'POST',
        body: {
          suite_id: suiteID.trim(),
          prompt,
          ground_truth: groundTruth,
          target_models: selectedModels,
          max_tokens: tokens,
          budget_cap_usd: budget,
          api_keys: apiKeys,
        },
      });
      notify({
        tone: 'success',
        title: 'Benchmark dispatched',
        message: `${selectedModels.length} model${selectedModels.length === 1 ? '' : 's'} · worst-case cost $${Number(data?.estimated_cost_usd || 0).toFixed(4)}`,
      });
      onJobSubmitted?.(data?.job_id);
      onClose();
    } catch (err) {
      setErrorMsg(err.message || 'Failed to submit evaluation job.');
    } finally {
      setSubmitting(false);
    }
  };

  const showError = field => touched && errors[field];

  return (
    <Modal
      title="Run benchmark"
      description="Fans the prompt out to every selected model in parallel and scores each response."
      onClose={onClose}
      footer={
        <div className="flex items-center justify-end gap-2">
          <button type="button" onClick={onClose} className="btn-ghost">
            Cancel
          </button>
          <button type="submit" form="benchmark-form" disabled={submitting} className="btn-primary">
            {submitting ? <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden="true" /> : <Play className="h-3.5 w-3.5 fill-current" aria-hidden="true" />}
            {submitting ? 'Dispatching…' : 'Dispatch'}
          </button>
        </div>
      }
    >
      <form id="benchmark-form" onSubmit={handleSubmit} noValidate className="space-y-4">
        {errorMsg && (
          <div role="alert" className="p-3 rounded-xl bg-rose-950/40 border border-rose-500/30 flex items-start gap-2 text-xs text-rose-300">
            <AlertCircle className="h-4 w-4 text-rose-400 flex-shrink-0" aria-hidden="true" />
            <span className="break-words">{errorMsg}</span>
          </div>
        )}

        <div>
          <label htmlFor="bm-prompt" className="field-label">
            Prompt
          </label>
          <textarea
            id="bm-prompt"
            data-autofocus
            rows={3}
            value={prompt}
            onChange={e => setPrompt(e.target.value)}
            className="input"
            placeholder="Enter the benchmark prompt…"
            aria-invalid={showError('prompt') ? true : undefined}
            aria-describedby="bm-prompt-hint"
          />
          <p id="bm-prompt-hint" className={cn('field-hint', showError('prompt') && 'text-rose-400')}>
            {showError('prompt') || `${prompt.length.toLocaleString()} characters`}
          </p>
        </div>

        <div>
          <label htmlFor="bm-truth" className="field-label">
            Ground truth <span className="font-normal text-slate-500">(optional)</span>
          </label>
          <textarea
            id="bm-truth"
            rows={3}
            value={groundTruth}
            onChange={e => setGroundTruth(e.target.value)}
            className="input font-mono text-[11px]"
            placeholder="Reference answer used for exact-match and cosine scoring…"
            aria-describedby="bm-truth-hint"
          />
          <p id="bm-truth-hint" className="field-hint">
            With a ground truth, real (non-simulated) responses are graded and count toward the regression gate. Otherwise only a heuristic score is shown.
          </p>
        </div>

        <fieldset>
          <legend className="field-label">Target models</legend>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
            {AVAILABLE_MODELS.map(m => {
              const isSelected = selectedModels.includes(m.id);
              const keyed = hasCredential(m.id, activeKeys);
              return (
                <label
                  key={m.id}
                  className={cn(
                    'flex items-center gap-3 px-3 py-2.5 rounded-xl text-xs border cursor-pointer transition-colors',
                    isSelected ? 'bg-emerald-950/40 border-emerald-500/50 text-emerald-100' : 'bg-slate-950/60 border-slate-800 text-slate-400 hover:border-slate-700'
                  )}
                >
                  <input type="checkbox" className="sr-only peer" checked={isSelected} onChange={() => toggleModel(m.id)} />
                  <span
                    aria-hidden="true"
                    className={cn(
                      'h-4 w-4 rounded border flex items-center justify-center flex-shrink-0 peer-focus-visible:ring-2 peer-focus-visible:ring-emerald-400',
                      isSelected ? 'border-emerald-400 bg-emerald-500 text-slate-950' : 'border-slate-600'
                    )}
                  >
                    {isSelected && <Check className="h-3 w-3" strokeWidth={3} />}
                  </span>
                  <span className="flex-1 min-w-0">
                    <span className="block font-medium truncate">{m.name}</span>
                    <span className="block text-[10px] text-slate-500">{m.note}</span>
                  </span>
                  {keyed ? (
                    <KeyRound className="h-3.5 w-3.5 text-indigo-400 flex-shrink-0" aria-label="Uses your key" />
                  ) : (
                    <FlaskConical className="h-3.5 w-3.5 text-amber-400/80 flex-shrink-0" aria-label="Simulated (no key)" />
                  )}
                </label>
              );
            })}
          </div>
          {showError('models') && <p className="field-hint text-rose-400">{errors.models}</p>}
          {simulatedModels.length > 0 && (
            <p className="mt-2 text-[11px] text-amber-300/90 flex items-start gap-1.5">
              <FlaskConical className="h-3.5 w-3.5 mt-px flex-shrink-0" aria-hidden="true" />
              <span>
                {simulatedModels.length} selected model{simulatedModels.length === 1 ? ' has' : 's have'} no key and will run in offline simulation.{' '}
                <button type="button" onClick={onOpenBYOK} className="underline underline-offset-2 hover:text-amber-200">
                  Add keys
                </button>
              </span>
            </p>
          )}
        </fieldset>

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <div>
            <label htmlFor="bm-suite" className="field-label">
              Suite ID
            </label>
            <input id="bm-suite" type="text" value={suiteID} onChange={e => setSuiteID(e.target.value)} maxLength={128} className="input font-mono" />
          </div>
          <div>
            <label htmlFor="bm-tokens" className="field-label">
              Max output tokens
            </label>
            <input
              id="bm-tokens"
              type="number"
              inputMode="numeric"
              min={1}
              max={MAX_OUTPUT_TOKENS}
              value={maxTokens}
              onChange={e => setMaxTokens(e.target.value)}
              aria-invalid={showError('maxTokens') ? true : undefined}
              className="input font-mono"
            />
            {showError('maxTokens') && <p className="field-hint text-rose-400">{errors.maxTokens}</p>}
          </div>
          <div>
            <label htmlFor="bm-budget" className="field-label">
              Budget cap (USD)
            </label>
            <input
              id="bm-budget"
              type="number"
              inputMode="decimal"
              min={0.01}
              step="0.1"
              value={budgetCap}
              onChange={e => setBudgetCap(e.target.value)}
              aria-invalid={showError('budget') ? true : undefined}
              className="input font-mono"
            />
            {showError('budget') && <p className="field-hint text-rose-400">{errors.budget}</p>}
          </div>
        </div>
        <p className="field-hint -mt-2">The job is rejected before any tokens are spent if its worst-case cost exceeds the budget cap.</p>
      </form>
    </Modal>
  );
}
