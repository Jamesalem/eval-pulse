import { useCallback, useEffect, useRef, useState } from 'react';
import { apiFetch, isAbortError } from '../lib/api';
import { isPendingStatus } from '../lib/format';

const IDLE_INTERVAL_MS = 5000;
const ACTIVE_INTERVAL_MS = 1500; // faster while a job is queued/running
const MAX_BACKOFF_MS = 15000;
const JOBS_LIMIT = 25;

const initialState = {
  metrics: [],
  jobs: [],
  regression: null,
  health: null,
  status: 'loading', // loading | ok | offline
  error: null,
  lastUpdated: null,
};

/**
 * Polls the dashboard endpoints. Requests never overlap, in-flight requests
 * are aborted on unmount, polling pauses while the tab is hidden, and failures
 * back off exponentially instead of hammering an unavailable API.
 */
export function useDashboardData() {
  const [state, setState] = useState(initialState);
  const timerRef = useRef(null);
  const controllerRef = useRef(null);
  const failuresRef = useRef(0);
  const pendingRef = useRef(false);

  const load = useCallback(async () => {
    controllerRef.current?.abort();
    const controller = new AbortController();
    controllerRef.current = controller;
    const { signal } = controller;

    try {
      const [metrics, jobs, regression, health] = await Promise.all([
        apiFetch('/api/v1/metrics/comparison', { signal }),
        apiFetch(`/api/v1/eval/jobs?limit=${JOBS_LIMIT}`, { signal }),
        apiFetch('/api/v1/metrics/regression-check', { signal, acceptStatuses: [409] }),
        apiFetch('/healthz', { signal }).catch(err => (isAbortError(err) ? Promise.reject(err) : null)),
      ]);

      failuresRef.current = 0;
      const jobList = Array.isArray(jobs) ? jobs : [];
      pendingRef.current = jobList.some(j => isPendingStatus(j.status));
      setState({
        metrics: Array.isArray(metrics) ? metrics : [],
        jobs: jobList,
        regression,
        health,
        status: 'ok',
        error: null,
        lastUpdated: new Date(),
      });
    } catch (err) {
      if (isAbortError(err) || signal.aborted) return;
      failuresRef.current += 1;
      setState(prev => ({ ...prev, status: 'offline', error: err.message || 'Unable to load dashboard data.' }));
    }
  }, []);

  const schedule = useCallback(() => {
    clearTimeout(timerRef.current);
    if (document.visibilityState === 'hidden') return;
    const base = pendingRef.current ? ACTIVE_INTERVAL_MS : IDLE_INTERVAL_MS;
    const delay = failuresRef.current > 0 ? Math.min(base * 2 ** failuresRef.current, MAX_BACKOFF_MS) : base;
    timerRef.current = setTimeout(async () => {
      await load();
      schedule();
    }, delay);
  }, [load]);

  const refresh = useCallback(async () => {
    clearTimeout(timerRef.current);
    await load();
    schedule();
  }, [load, schedule]);

  useEffect(() => {
    refresh();
    const onVisibility = () => {
      if (document.visibilityState === 'visible') refresh();
      else clearTimeout(timerRef.current);
    };
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      document.removeEventListener('visibilitychange', onVisibility);
      clearTimeout(timerRef.current);
      controllerRef.current?.abort();
    };
  }, [refresh]);

  return { ...state, refresh };
}
