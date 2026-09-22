const DEFAULT_TIMEOUT_MS = 15000;

export class ApiError extends Error {
  constructor(message, { status = 0, data = null } = {}) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.data = data;
  }
}

/**
 * fetch wrapper that always resolves to parsed JSON or throws an ApiError with
 * a human-readable message. Handles timeouts, caller aborts, and non-JSON bodies.
 *
 * `acceptStatuses` lets callers treat specific non-2xx codes as data (e.g. the
 * regression gate answers 409 when a regression is detected).
 */
export async function apiFetch(path, { method = 'GET', body, signal, timeoutMs = DEFAULT_TIMEOUT_MS, acceptStatuses = [] } = {}) {
  const controller = new AbortController();
  const onAbort = () => controller.abort(signal.reason);
  if (signal) {
    if (signal.aborted) controller.abort(signal.reason);
    else signal.addEventListener('abort', onAbort, { once: true });
  }
  const timer = setTimeout(() => controller.abort(new DOMException('Request timed out', 'TimeoutError')), timeoutMs);

  let res;
  try {
    res = await fetch(path, {
      method,
      headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
      body: body !== undefined ? JSON.stringify(body) : undefined,
      signal: controller.signal,
    });
  } catch (err) {
    if (signal?.aborted) throw err; // caller cancelled; let them ignore it
    const timedOut = controller.signal.reason?.name === 'TimeoutError';
    throw new ApiError(timedOut ? 'The EvalPulse API took too long to respond.' : 'Cannot reach the EvalPulse API.', { status: 0 });
  } finally {
    clearTimeout(timer);
    signal?.removeEventListener('abort', onAbort);
  }

  let data = null;
  const text = await res.text();
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = null;
    }
  }

  if (!res.ok && !acceptStatuses.includes(res.status)) {
    // A 5xx without a JSON body comes from a proxy (Vite dev server / nginx), not the API itself.
    const fallback = res.status >= 500 && !data ? 'The EvalPulse API is unreachable.' : `Request failed (HTTP ${res.status}).`;
    const message = data?.error || data?.message || fallback;
    throw new ApiError(message, { status: res.status, data });
  }
  return data;
}

export const isAbortError = err => err?.name === 'AbortError';
