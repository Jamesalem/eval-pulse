import React, { useState } from 'react';
import { X, Key, CheckCircle2, AlertCircle, Loader2, Shield, ExternalLink, RefreshCw } from 'lucide-react';

export default function ProviderSettingsModal({ isOpen, onClose, keys, onSaveKeys }) {
  if (!isOpen) return null;

  const [formData, setFormData] = useState({ ...keys });
  const [testingStatus, setTestingStatus] = useState({});

  const providers = [
    {
      id: 'gemini',
      name: 'Google Gemini',
      models: 'Gemini 1.5 Pro, 1.5 Flash',
      pricing: '$0.075 / $0.30 per MTok (Flash)',
      keyName: 'gemini',
      placeholder: 'AIzaSy...',
      docsUrl: 'https://aistudio.google.com/app/apikey',
    },
    {
      id: 'openai',
      name: 'OpenAI',
      models: 'GPT-4o, GPT-4o-mini',
      pricing: '$0.15 / $0.60 per MTok (Mini)',
      keyName: 'openai',
      placeholder: 'sk-proj-...',
      docsUrl: 'https://platform.openai.com/api-keys',
    },
    {
      id: 'anthropic',
      name: 'Anthropic Claude',
      models: 'Claude 3.5 Sonnet, Haiku',
      pricing: '$3.00 / $15.00 per MTok (Sonnet)',
      keyName: 'anthropic',
      placeholder: 'sk-ant-...',
      docsUrl: 'https://console.anthropic.com/settings/keys',
    },
    {
      id: 'openrouter',
      name: 'OpenRouter / Unified',
      models: 'Llama 3.1, Mistral, DeepSeek',
      pricing: 'Unified Aggregator Rate Card',
      keyName: 'openrouter',
      placeholder: 'sk-or-v1-...',
      docsUrl: 'https://openrouter.ai/keys',
    },
    {
      id: 'ollama',
      name: 'Local Ollama / vLLM',
      models: 'Local Llama 3, DeepSeek, Phi-3',
      pricing: '$0.00 (Self-Hosted GPU/CPU)',
      keyName: 'ollama_endpoint',
      isEndpoint: true,
      placeholder: 'http://localhost:11434',
      docsUrl: 'https://ollama.ai',
    },
  ];

  const handleTestConnection = async (providerId, keyField) => {
    setTestingStatus(prev => ({ ...prev, [providerId]: { loading: true } }));
    const val = formData[keyField] || '';

    try {
      const res = await fetch('/api/v1/providers/validate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          provider: providerId,
          api_key: providerId === 'ollama' ? '' : val,
          endpoint_url: providerId === 'ollama' ? val : '',
        }),
      });

      const data = await res.json();
      if (res.ok && data.valid) {
        setTestingStatus(prev => ({
          ...prev,
          [providerId]: { loading: false, success: true, latency: data.latency_ms, message: data.message },
        }));
      } else {
        setTestingStatus(prev => ({
          ...prev,
          [providerId]: { loading: false, success: false, message: data.message || 'Validation failed' },
        }));
      }
    } catch (err) {
      setTestingStatus(prev => ({
        ...prev,
        [providerId]: { loading: false, success: false, message: 'Server unreachable or offline' },
      }));
    }
  };

  const handleSave = () => {
    onSaveKeys(formData);
    onClose();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-2xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        
        {/* Header */}
        <div className="px-6 py-4 border-b border-slate-800 flex items-center justify-between bg-slate-850">
          <div>
            <div className="flex items-center space-x-2">
              <h3 className="text-base font-semibold text-white">Cline-Style Account & Provider Manager</h3>
              <span className="px-2 py-0.5 text-[10px] font-bold rounded bg-indigo-500/20 text-indigo-300 border border-indigo-500/30">
                BYOK Architecture
              </span>
            </div>
            <p className="text-xs text-slate-400 mt-0.5">
              Connect your own existing accounts. Zero SaaS markup and zero external credential exfiltration.
            </p>
          </div>
          <button onClick={onClose} className="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800">
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Security Banner */}
        <div className="mx-6 mt-4 p-3 rounded-xl bg-emerald-950/40 border border-emerald-500/30 flex items-start space-x-3 text-xs text-emerald-200">
          <Shield className="h-4 w-4 text-emerald-400 mt-0.5 flex-shrink-0" />
          <div>
            <span className="font-semibold text-emerald-300">Zero-Exfiltration Guarantee: </span>
            Credentials are kept strictly in your local browser session and container runtime memory. They are never sent to external telemetry servers or stored in plaintext logs.
          </div>
        </div>

        {/* Provider List Form */}
        <div className="px-6 py-4 overflow-y-auto space-y-4 flex-1">
          {providers.map(p => {
            const status = testingStatus[p.id];
            const currentValue = formData[p.keyName] || '';

            return (
              <div key={p.id} className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 hover:border-slate-700 transition-colors">
                <div className="flex items-center justify-between mb-2">
                  <div>
                    <span className="text-sm font-semibold text-slate-200">{p.name}</span>
                    <span className="ml-2 text-[11px] text-slate-400">({p.models})</span>
                  </div>
                  <div className="flex items-center space-x-2">
                    <span className="text-[10px] font-mono text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/20">
                      {p.pricing}
                    </span>
                    <a
                      href={p.docsUrl}
                      target="_blank"
                      rel="noreferrer"
                      className="text-slate-400 hover:text-indigo-400 transition-colors"
                      title="Get API Key"
                    >
                      <ExternalLink className="h-3.5 w-3.5" />
                    </a>
                  </div>
                </div>

                <div className="flex space-x-2">
                  <div className="relative flex-1">
                    <input
                      type={p.isEndpoint ? "text" : "password"}
                      value={currentValue}
                      onChange={e => setFormData({ ...formData, [p.keyName]: e.target.value })}
                      placeholder={p.placeholder}
                      className="w-full px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-700 text-xs text-slate-200 focus:outline-none focus:border-indigo-500 font-mono"
                    />
                  </div>

                  <button
                    type="button"
                    onClick={() => handleTestConnection(p.id, p.keyName)}
                    disabled={status?.loading}
                    className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 text-xs font-medium text-slate-300 flex items-center space-x-1.5 transition-colors disabled:opacity-50"
                  >
                    {status?.loading ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin text-indigo-400" />
                    ) : (
                      <RefreshCw className="h-3.5 w-3.5 text-indigo-400" />
                    )}
                    <span>Test</span>
                  </button>
                </div>

                {/* Validation Status message */}
                {status && (
                  <div className={`mt-2 flex items-center space-x-1.5 text-xs ${status.success ? 'text-emerald-400' : 'text-rose-400'}`}>
                    {status.success ? (
                      <CheckCircle2 className="h-3.5 w-3.5 flex-shrink-0" />
                    ) : (
                      <AlertCircle className="h-3.5 w-3.5 flex-shrink-0" />
                    )}
                    <span>
                      {status.success
                        ? `Connected (${status.latency}ms) - Ready for evaluation`
                        : status.message}
                    </span>
                  </div>
                )}
              </div>
            );
          })}
        </div>

        {/* Footer Actions */}
        <div className="px-6 py-3 border-t border-slate-800 bg-slate-850 flex items-center justify-between">
          <button
            onClick={() => setFormData({})}
            className="text-xs text-rose-400 hover:text-rose-300 font-medium"
          >
            Clear All Credentials
          </button>
          <div className="flex items-center space-x-2">
            <button
              onClick={onClose}
              className="px-4 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs text-slate-300 font-medium transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={handleSave}
              className="px-4 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-xs text-white font-medium shadow-sm transition-colors"
            >
              Save Credentials
            </button>
          </div>
        </div>

      </div>
    </div>
  );
}
