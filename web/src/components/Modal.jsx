import React, { useEffect, useId, useRef } from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';
import { cn } from '../lib/format';

const FOCUSABLE = 'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

/**
 * Accessible dialog shell: portal-rendered, labelled, closes on Escape or a
 * backdrop click, traps Tab focus, locks page scroll, and returns focus to the
 * element that opened it. Render it conditionally from the parent.
 */
export default function Modal({ title, description, onClose, children, footer, size = 'md', badge }) {
  const titleId = useId();
  const descId = useId();
  const panelRef = useRef(null);
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  useEffect(() => {
    const previouslyFocused = document.activeElement;
    const panel = panelRef.current;
    const first = panel?.querySelector('[data-autofocus]') || panel?.querySelector(FOCUSABLE);
    (first || panel)?.focus();

    const { overflow } = document.body.style;
    document.body.style.overflow = 'hidden';

    const onKeyDown = e => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        onCloseRef.current();
        return;
      }
      if (e.key !== 'Tab' || !panel) return;
      const items = [...panel.querySelectorAll(FOCUSABLE)].filter(el => el.offsetParent !== null);
      if (items.length === 0) return;
      const firstEl = items[0];
      const lastEl = items[items.length - 1];
      if (e.shiftKey && document.activeElement === firstEl) {
        e.preventDefault();
        lastEl.focus();
      } else if (!e.shiftKey && document.activeElement === lastEl) {
        e.preventDefault();
        firstEl.focus();
      }
    };
    document.addEventListener('keydown', onKeyDown);

    return () => {
      document.removeEventListener('keydown', onKeyDown);
      document.body.style.overflow = overflow;
      if (previouslyFocused instanceof HTMLElement) previouslyFocused.focus();
    };
  }, []);

  const widths = { sm: 'max-w-md', md: 'max-w-xl', lg: 'max-w-2xl', xl: 'max-w-4xl' };

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-end sm:items-center justify-center sm:p-4 bg-black/70 backdrop-blur-sm animate-fade-in"
      onMouseDown={e => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={description ? descId : undefined}
        tabIndex={-1}
        className={cn(
          'bg-slate-900 border border-slate-800 w-full shadow-2xl overflow-hidden flex flex-col outline-none animate-scale-in',
          'rounded-t-2xl sm:rounded-2xl max-h-[92vh] sm:max-h-[90vh]',
          widths[size]
        )}
      >
        <div className="px-5 sm:px-6 py-4 border-b border-slate-800 flex items-start justify-between gap-4 bg-slate-850">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h2 id={titleId} className="text-base font-semibold text-white">
                {title}
              </h2>
              {badge}
            </div>
            {description && (
              <p id={descId} className="text-xs text-slate-400 mt-1">
                {description}
              </p>
            )}
          </div>
          <button type="button" onClick={onClose} className="icon-btn -mr-1" aria-label="Close dialog">
            <X className="h-5 w-5" aria-hidden="true" />
          </button>
        </div>

        <div className="px-5 sm:px-6 py-4 overflow-y-auto flex-1">{children}</div>

        {footer && <div className="px-5 sm:px-6 py-3 border-t border-slate-800 bg-slate-850">{footer}</div>}
      </div>
    </div>,
    document.body
  );
}
