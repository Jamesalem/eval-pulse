import React from 'react';
import { AlertTriangle, RotateCcw } from 'lucide-react';

/**
 * Catches render/lifecycle errors in its subtree so a single broken widget
 * cannot blank the whole dashboard. Use `variant="page"` at the root and the
 * default section variant around individual panels. Changing any value in
 * `resetKeys` clears the error automatically (e.g. when new data arrives).
 */
export default class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props);
    this.state = { error: null };
    this.reset = this.reset.bind(this);
  }

  static getDerivedStateFromError(error) {
    return { error };
  }

  componentDidCatch(error, info) {
    console.error(`[EvalPulse] ${this.props.name || 'UI'} crashed:`, error, info?.componentStack);
  }

  componentDidUpdate(prevProps) {
    if (!this.state.error) return;
    const prev = prevProps.resetKeys || [];
    const next = this.props.resetKeys || [];
    if (prev.length !== next.length || prev.some((v, i) => !Object.is(v, next[i]))) {
      this.reset();
    }
  }

  reset() {
    this.setState({ error: null });
  }

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;

    if (this.props.variant === 'page') {
      return (
        <div role="alert" className="min-h-screen flex items-center justify-center p-6 bg-slate-950">
          <div className="max-w-md w-full surface p-6 text-center">
            <div className="mx-auto mb-4 h-12 w-12 rounded-xl bg-rose-500/10 border border-rose-500/30 flex items-center justify-center text-rose-400">
              <AlertTriangle className="h-6 w-6" aria-hidden="true" />
            </div>
            <h1 className="text-lg font-semibold text-white">Something went wrong</h1>
            <p className="mt-2 text-sm text-slate-400">
              The dashboard hit an unexpected error. Your saved provider keys are unaffected.
            </p>
            <pre className="mt-4 max-h-32 overflow-auto rounded-lg bg-slate-950 border border-slate-800 p-3 text-left text-[11px] text-rose-300 font-mono whitespace-pre-wrap">
              {String(error?.message || error)}
            </pre>
            <div className="mt-5 flex justify-center gap-2">
              <button type="button" onClick={this.reset} className="btn-secondary">
                <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" /> Try again
              </button>
              <button type="button" onClick={() => window.location.reload()} className="btn-primary">
                Reload page
              </button>
            </div>
          </div>
        </div>
      );
    }

    return (
      <div role="alert" className="surface p-5 flex flex-col sm:flex-row sm:items-center gap-3 border-rose-500/30">
        <AlertTriangle className="h-5 w-5 text-rose-400 flex-shrink-0" aria-hidden="true" />
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium text-white">{this.props.name || 'This panel'} failed to render</p>
          <p className="text-xs text-slate-400 truncate">{String(error?.message || error)}</p>
        </div>
        <button type="button" onClick={this.reset} className="btn-secondary self-start sm:self-auto">
          <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" /> Retry
        </button>
      </div>
    );
  }
}
