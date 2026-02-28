import { useState } from 'react';

const API_BASE = '/api';

export default function AdminTools() {
  const [reenrichStatus, setReenrichStatus] = useState<string | null>(null);
  const [reenrichLoading, setReenrichLoading] = useState(false);

  const handleReenrich = async () => {
    if (!confirm('This will clear garbage AI summaries and re-process them. Continue?')) return;
    setReenrichLoading(true);
    setReenrichStatus(null);
    try {
      const res = await fetch(`${API_BASE}/admin/reenrich`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
      });
      if (!res.ok) {
        throw new Error(`Failed: ${res.status}`);
      }
      const data = await res.json();
      setReenrichStatus(
        `Cleared ${data.cleared} garbage summaries. Queued ${data.queued} articles for re-enrichment.`
      );
    } catch (err: any) {
      setReenrichStatus(`Error: ${err.message}`);
    } finally {
      setReenrichLoading(false);
    }
  };

  return (
    <div className="space-y-4">
      {/* Re-enrich */}
      <div className="p-4 rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
              Re-enrich Articles
            </h3>
            <p className="text-xs text-zinc-500 dark:text-zinc-400 mt-0.5">
              Clears garbage AI summaries/tags and re-processes them with improved prompts.
            </p>
          </div>
          <button
            onClick={handleReenrich}
            disabled={reenrichLoading}
            className="inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium text-white bg-amber-600 hover:bg-amber-700 disabled:opacity-50 rounded-lg transition-colors shrink-0 ml-4"
          >
            {reenrichLoading ? (
              <>
                <svg className="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                </svg>
                Processing...
              </>
            ) : (
              'Run Re-enrichment'
            )}
          </button>
        </div>
        {reenrichStatus && (
          <div className={`mt-3 p-3 text-sm rounded-lg border ${
            reenrichStatus.startsWith('Error')
              ? 'bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20'
              : 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border-emerald-500/20'
          }`}>
            {reenrichStatus}
          </div>
        )}
      </div>
    </div>
  );
}
