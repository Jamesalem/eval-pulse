import React, { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { CheckCircle2, AlertCircle, Info, X } from 'lucide-react';
import { cn } from '../lib/format';

const ToastContext = createContext(null);

const ICONS = {
  success: <CheckCircle2 className="h-4 w-4 text-emerald-400" aria-hidden="true" />,
  error: <AlertCircle className="h-4 w-4 text-rose-400" aria-hidden="true" />,
  info: <Info className="h-4 w-4 text-sky-400" aria-hidden="true" />,
};

export function ToastProvider({ children }) {
  const [toasts, setToasts] = useState([]);
  const idRef = useRef(0);
  const timers = useRef(new Map());

  const dismiss = useCallback(id => {
    setToasts(ts => ts.filter(t => t.id !== id));
    clearTimeout(timers.current.get(id));
    timers.current.delete(id);
  }, []);

  const notify = useCallback(
    ({ title, message, tone = 'info', duration = 5000 }) => {
      const id = ++idRef.current;
      setToasts(ts => [...ts.slice(-3), { id, title, message, tone }]);
      timers.current.set(id, setTimeout(() => dismiss(id), duration));
      return id;
    },
    [dismiss]
  );

  useEffect(() => {
    const map = timers.current;
    return () => map.forEach(clearTimeout);
  }, []);

  const value = useMemo(() => ({ notify, dismiss }), [notify, dismiss]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div
        aria-live="polite"
        aria-atomic="false"
        className="fixed z-[60] bottom-4 right-4 left-4 sm:left-auto flex flex-col gap-2 sm:w-96 pointer-events-none"
      >
        {toasts.map(t => (
          <div
            key={t.id}
            role={t.tone === 'error' ? 'alert' : 'status'}
            className={cn(
              'pointer-events-auto surface p-3.5 flex items-start gap-3 shadow-xl animate-slide-up',
              t.tone === 'error' && 'border-rose-500/40',
              t.tone === 'success' && 'border-emerald-500/30'
            )}
          >
            <span className="mt-0.5">{ICONS[t.tone]}</span>
            <div className="flex-1 min-w-0">
              {t.title && <p className="text-sm font-medium text-white">{t.title}</p>}
              {t.message && <p className="text-xs text-slate-400 mt-0.5 break-words">{t.message}</p>}
            </div>
            <button type="button" className="icon-btn -m-1" onClick={() => dismiss(t.id)} aria-label="Dismiss notification">
              <X className="h-3.5 w-3.5" aria-hidden="true" />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error('useToast must be used within a ToastProvider');
  return ctx;
}
