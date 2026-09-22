import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export const cn = (...inputs) => twMerge(clsx(inputs));

const num = v => (Number.isFinite(v) ? v : 0);

export const formatUSD = (value, digits = 4) => `$${num(value).toFixed(digits)}`;

export const formatInt = value => Math.round(num(value)).toLocaleString();

export const formatPercent = (ratio, digits = 1) => `${(num(ratio) * 100).toFixed(digits)}%`;

export const formatMs = value => `${Math.round(num(value)).toLocaleString()} ms`;

const rtf = typeof Intl !== 'undefined' && Intl.RelativeTimeFormat ? new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' }) : null;

export function formatRelativeTime(iso, now = Date.now()) {
  const t = Date.parse(iso);
  if (!Number.isFinite(t) || t <= 0) return '—';
  const diffSec = Math.round((t - now) / 1000);
  const abs = Math.abs(diffSec);
  if (!rtf) return new Date(t).toLocaleString();
  if (abs < 45) return rtf.format(diffSec, 'second');
  if (abs < 60 * 60) return rtf.format(Math.round(diffSec / 60), 'minute');
  if (abs < 22 * 3600) return rtf.format(Math.round(diffSec / 3600), 'hour');
  return rtf.format(Math.round(diffSec / 86400), 'day');
}

export function durationBetween(startIso, endIso) {
  const start = Date.parse(startIso);
  const end = Date.parse(endIso);
  if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) return null;
  const ms = end - start;
  return ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(1)} s`;
}

/** Maps a model ID to the BYOK key slot that serves it (mirrors models.Family in Go). */
export function providerFamily(modelId = '') {
  const id = modelId.toLowerCase();
  if (id.startsWith('gemini') || id.startsWith('google')) return 'gemini';
  if (id.startsWith('gpt') || id.startsWith('openai') || id.startsWith('o1') || id.startsWith('o3')) return 'openai';
  if (id.startsWith('claude') || id.startsWith('anthropic')) return 'anthropic';
  if (id.startsWith('ollama') || id.startsWith('llama') || id.startsWith('deepseek')) return 'ollama';
  return 'mock';
}

export const isPendingStatus = status => status === 'queued' || status === 'running';
