import { useState, useEffect, useCallback } from 'react';
import { api, type WatchlistOrg, type WatchlistHit } from '../lib/api';

const SOURCE_TYPES = [
  { key: 'all', label: 'Todos' },
  { key: 'google_news', label: 'Google News' },
  { key: 'web', label: 'Web' },
  { key: 'local', label: 'Local' },
  { key: 'youtube', label: 'YouTube' },
  { key: 'reddit', label: 'Reddit' },
] as const;

const SENTIMENT_STYLES: Record<string, { bg: string; text: string; label: string; pill: string }> = {
  positive: { bg: 'bg-emerald-500/10', text: 'text-emerald-600 dark:text-emerald-400', label: 'Positivo', pill: 'bg-emerald-100 text-emerald-800' },
  neutral: { bg: 'bg-zinc-500/10', text: 'text-zinc-600 dark:text-zinc-400', label: 'Neutral', pill: 'bg-zinc-100 text-zinc-800' },
  negative: { bg: 'bg-red-500/10', text: 'text-red-600 dark:text-red-400', label: 'Negativo', pill: 'bg-red-100 text-red-800' },
  unknown: { bg: 'bg-amber-500/10', text: 'text-amber-600 dark:text-amber-400', label: 'Pendiente', pill: 'bg-amber-100 text-amber-800' },
};

function timeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diff = now - then;
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'ahora';
  if (mins < 60) return `${mins}m`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h`;
  const days = Math.floor(hrs / 24);
  if (days < 30) return `${days}d`;
  return `${Math.floor(days / 30)}mo`;
}

// ── SVG Source Icons ─────────────────────────────────────────────

function SourceIcon({ type, className = 'w-3.5 h-3.5' }: { type: string; className?: string }) {
  switch (type) {
    case 'google_news':
      return (
        <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M12 7.5h1.5m-1.5 3h1.5m-7.5 3h7.5m-7.5 3h7.5m3-9h3.375c.621 0 1.125.504 1.125 1.125V18a2.25 2.25 0 01-2.25 2.25M16.5 7.5V18a2.25 2.25 0 002.25 2.25M16.5 7.5V4.875c0-.621-.504-1.125-1.125-1.125H4.125C3.504 3.75 3 4.254 3 4.875V18a2.25 2.25 0 002.25 2.25h13.5M6 7.5h3v3H6V7.5z" />
        </svg>
      );
    case 'web':
      return (
        <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418" />
        </svg>
      );
    case 'local':
      return (
        <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 21h19.5m-18-18v18m10.5-18v18m6-13.5V21M6.75 6.75h.75m-.75 3h.75m-.75 3h.75m3-6h.75m-.75 3h.75m-.75 3h.75M6.75 21v-3.375c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21M3 3h12m-.75 4.5H21m-3.75 0h.008v.008h-.008V7.5z" />
        </svg>
      );
    case 'youtube':
      return (
        <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0zM15.91 11.672a.375.375 0 010 .656l-5.603 3.113a.375.375 0 01-.557-.328V8.887c0-.286.307-.466.557-.327l5.603 3.112z" />
        </svg>
      );
    case 'reddit':
      return (
        <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M20.25 8.511c.884.284 1.5 1.128 1.5 2.097v4.286c0 1.136-.847 2.1-1.98 2.193-.34.027-.68.052-1.02.072v3.091l-3-3c-1.354 0-2.694-.055-4.02-.163a2.115 2.115 0 01-.825-.242m9.345-8.334a2.126 2.126 0 00-.476-.095 48.64 48.64 0 00-8.048 0c-1.131.094-1.976 1.057-1.976 2.192v4.286c0 .837.46 1.58 1.155 1.951m9.345-8.334V6.637c0-1.621-1.152-3.026-2.76-3.235A48.455 48.455 0 0011.25 3c-2.115 0-4.198.137-6.24.402-1.608.209-2.76 1.614-2.76 3.235v6.226c0 1.621 1.152 3.026 2.76 3.235.577.075 1.157.14 1.74.194V21l4.155-4.155" />
        </svg>
      );
    default:
      return (
        <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M13.19 8.688a4.5 4.5 0 011.242 7.244l-4.5 4.5a4.5 4.5 0 01-6.364-6.364l1.757-1.757m13.35-.622l1.757-1.757a4.5 4.5 0 00-6.364-6.364l-4.5 4.5a4.5 4.5 0 001.242 7.244" />
        </svg>
      );
  }
}

const SOURCE_LABELS: Record<string, string> = {
  google_news: 'Google News',
  web: 'Web',
  local: 'Local',
  youtube: 'YouTube',
  reddit: 'Reddit',
};

export default function WatchlistManager() {
  const [orgs, setOrgs] = useState<WatchlistOrg[]>([]);
  const [hits, setHits] = useState<WatchlistHit[]>([]);
  const [selectedOrg, setSelectedOrg] = useState<string | null>(null);
  const [sourceFilter, setSourceFilter] = useState('all');
  const [loading, setLoading] = useState(true);
  const [hitsLoading, setHitsLoading] = useState(false);

  // Modals
  const [showAddOrg, setShowAddOrg] = useState(false);
  const [editingOrg, setEditingOrg] = useState<WatchlistOrg | null>(null);
  const [scanning, setScanning] = useState(false);
  const [showRSSModal, setShowRSSModal] = useState(false);
  const [feedURL, setFeedURL] = useState('');
  const [feedCopied, setFeedCopied] = useState(false);
  const [regenerating, setRegenerating] = useState(false);

  // Form
  const [formName, setFormName] = useState('');
  const [formWebsite, setFormWebsite] = useState('');
  const [formKeywords, setFormKeywords] = useState('');
  const [formYouTube, setFormYouTube] = useState('');
  const [enriching, setEnriching] = useState(false);

  // Expanded drafts
  const [expandedDraft, setExpandedDraft] = useState<string | null>(null);

  const fetchOrgs = useCallback(async () => {
    try {
      const data = await api.getWatchlistOrgs();
      setOrgs(data.orgs || []);
    } catch (e) {
      console.error('Failed to fetch orgs:', e);
    }
  }, []);

  const fetchHits = useCallback(async () => {
    setHitsLoading(true);
    try {
      const params: { limit?: number; org_id?: string } = { limit: 100 };
      if (selectedOrg) params.org_id = selectedOrg;
      const data = await api.getWatchlistHits(params);
      let filtered = data.hits || [];
      if (sourceFilter !== 'all') {
        filtered = filtered.filter(h => h.source_type === sourceFilter);
      }
      setHits(filtered);
    } catch (e) {
      console.error('Failed to fetch hits:', e);
    } finally {
      setHitsLoading(false);
    }
  }, [selectedOrg, sourceFilter]);

  useEffect(() => {
    (async () => {
      await fetchOrgs();
      setLoading(false);
    })();
  }, [fetchOrgs]);

  useEffect(() => {
    if (!loading) fetchHits();
  }, [loading, fetchHits]);

  // Org CRUD
  const handleSaveOrg = async () => {
    const keywords = formKeywords.split(',').map(k => k.trim()).filter(Boolean);
    const youtube_channels = formYouTube.split('\n').map(c => c.trim()).filter(Boolean);
    const website = formWebsite.trim();

    try {
      let savedOrg: import('../lib/api').WatchlistOrg;
      if (editingOrg) {
        savedOrg = await api.updateWatchlistOrg(editingOrg.id, { name: formName, website, keywords, youtube_channels });
      } else {
        savedOrg = await api.createWatchlistOrg({ name: formName, website, keywords, youtube_channels });
      }
      setShowAddOrg(false);
      setEditingOrg(null);
      resetForm();
      await fetchOrgs();

      // Auto-enrich if website was provided and org has few keywords.
      if (website && keywords.length < 3) {
        handleEnrichOrg(savedOrg.id);
      }
    } catch (e) {
      console.error('Failed to save org:', e);
    }
  };

  const handleEnrichOrg = async (orgId: string) => {
    setEnriching(true);
    try {
      const result = await api.enrichWatchlistOrg(orgId);
      if (result.status === 'enriched') {
        await fetchOrgs();
      }
    } catch (e) {
      console.error('Failed to enrich org:', e);
    } finally {
      setEnriching(false);
    }
  };

  const handleDeleteOrg = async (id: string) => {
    try {
      await api.deleteWatchlistOrg(id);
      if (selectedOrg === id) setSelectedOrg(null);
      await fetchOrgs();
      await fetchHits();
    } catch (e) {
      console.error('Failed to delete org:', e);
    }
  };

  const handleToggleOrg = async (id: string, active: boolean) => {
    try {
      await api.toggleWatchlistOrg(id, !active);
      await fetchOrgs();
    } catch (e) {
      console.error('Failed to toggle org:', e);
    }
  };

  const startEdit = (org: WatchlistOrg) => {
    setEditingOrg(org);
    setFormName(org.name);
    setFormWebsite(org.website || '');
    setFormKeywords(org.keywords.join(', '));
    setFormYouTube(org.youtube_channels.join('\n'));
    setShowAddOrg(true);
  };

  const resetForm = () => {
    setFormName('');
    setFormWebsite('');
    setFormKeywords('');
    setFormYouTube('');
  };

  // Hit actions
  const handleMarkSeen = async (id: string) => {
    try {
      await api.markHitSeen(id);
      setHits(prev => prev.map(h => h.id === id ? { ...h, seen: true } : h));
    } catch (e) {
      console.error('Failed to mark seen:', e);
    }
  };

  const handleMarkAllSeen = async () => {
    try {
      await api.markAllHitsSeen();
      setHits(prev => prev.map(h => ({ ...h, seen: true })));
    } catch (e) {
      console.error('Failed to mark all seen:', e);
    }
  };

  const handleDeleteHit = async (id: string) => {
    try {
      await api.deleteHit(id);
      setHits(prev => prev.filter(h => h.id !== id));
    } catch (e) {
      console.error('Failed to delete hit:', e);
    }
  };

  const handleTriggerScan = async () => {
    if (scanning || orgs.length === 0) return;
    setScanning(true);
    try {
      await api.triggerWatchlistScan();
      // Poll for new results every 3 seconds for 30 seconds
      let polls = 0;
      const interval = setInterval(async () => {
        polls++;
        await fetchHits();
        if (polls >= 10) clearInterval(interval);
      }, 3000);
      // Also stop scanning indicator after 15s
      setTimeout(() => setScanning(false), 15000);
    } catch (e) {
      console.error('Failed to trigger scan:', e);
      setScanning(false);
    }
  };

  const handleShowRSS = async () => {
    try {
      const data = await api.getWatchlistFeedURL();
      setFeedURL(window.location.origin + data.url);
      setShowRSSModal(true);
      setFeedCopied(false);
    } catch (e) {
      console.error('Failed to get feed URL:', e);
    }
  };

  const handleCopyFeedURL = async () => {
    try {
      await navigator.clipboard.writeText(feedURL);
      setFeedCopied(true);
      setTimeout(() => setFeedCopied(false), 2000);
    } catch {
      // Fallback
      const input = document.createElement('input');
      input.value = feedURL;
      document.body.appendChild(input);
      input.select();
      document.execCommand('copy');
      document.body.removeChild(input);
      setFeedCopied(true);
      setTimeout(() => setFeedCopied(false), 2000);
    }
  };

  const handleRegenerateFeed = async () => {
    if (regenerating) return;
    setRegenerating(true);
    try {
      const data = await api.regenerateWatchlistFeedURL();
      setFeedURL(window.location.origin + data.url);
      setFeedCopied(false);
    } catch (e) {
      console.error('Failed to regenerate feed URL:', e);
    } finally {
      setRegenerating(false);
    }
  };

  const unseenCount = hits.filter(h => !h.seen).length;

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="w-6 h-6 border-2 border-indigo-500 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">Vigilancia</h1>
          <p className="text-sm text-zinc-500 dark:text-zinc-400">Monitoreo de organizaciones y coordinacion de respuestas</p>
        </div>
        <div className="flex items-center gap-2">
          {/* RSS button */}
          <button
            onClick={handleShowRSS}
            className="p-2.5 text-orange-500 hover:bg-orange-500/10 rounded-sm transition-colors"
            title="Feed RSS"
          >
            <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
              <path d="M6.18 15.64a2.18 2.18 0 010 4.36 2.18 2.18 0 010-4.36zM4 4.44A15.56 15.56 0 0119.56 20h-2.83A12.73 12.73 0 004 7.27V4.44zm0 5.66a9.9 9.9 0 019.9 9.9h-2.83A7.07 7.07 0 004 12.93V10.1z" />
            </svg>
          </button>
          {/* Scan button */}
          <button
            onClick={handleTriggerScan}
            disabled={scanning || orgs.length === 0}
            className="px-5 py-2.5 text-sm font-medium bg-emerald-600 hover:bg-emerald-500 disabled:opacity-40 disabled:hover:bg-emerald-600 text-white rounded-sm transition-colors flex items-center gap-2 shadow-lg shadow-emerald-500/20"
          >
            {scanning ? (
              <>
                <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                Escaneando...
              </>
            ) : (
              <>
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" />
                </svg>
                Buscar ahora
              </>
            )}
          </button>
        </div>
      </div>

      <div className="flex flex-col lg:flex-row gap-6 min-h-[calc(100vh-12rem)]">
        {/* Left panel: Org list */}
        <div className="w-full lg:w-72 shrink-0 space-y-2">
          <div className="flex items-center justify-between px-1">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-zinc-400 dark:text-zinc-500">
              Organizaciones ({orgs.length})
            </h3>
            <button
              onClick={() => { resetForm(); setEditingOrg(null); setShowAddOrg(true); }}
              className="px-2 py-1 text-[10px] font-semibold text-indigo-500 hover:bg-indigo-500/10 rounded-md transition-colors"
            >
              + Agregar
            </button>
          </div>

          {orgs.length === 0 && (
            <div className="p-4 rounded-sm border border-dashed border-zinc-300 dark:border-zinc-700 text-center">
              <p className="text-sm text-zinc-500 dark:text-zinc-400">Sin organizaciones</p>
              <p className="text-xs text-zinc-400 dark:text-zinc-500 mt-1">Agrega una para comenzar el monitoreo</p>
            </div>
          )}

          {orgs.map(org => (
            <div
              key={org.id}
              onClick={() => setSelectedOrg(selectedOrg === org.id ? null : org.id)}
              className={`group p-3 rounded-sm border cursor-pointer transition-all ${
                selectedOrg === org.id
                  ? 'border-indigo-500 bg-indigo-500/5 dark:bg-indigo-500/10'
                  : 'border-zinc-200 dark:border-zinc-800 hover:border-zinc-300 dark:hover:border-zinc-700 bg-white dark:bg-zinc-900'
              }`}
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 min-w-0">
                  <div className={`w-2 h-2 rounded-full shrink-0 ${org.active ? 'bg-emerald-500' : 'bg-zinc-400'}`} />
                  <span className="text-sm font-medium text-zinc-900 dark:text-zinc-100 truncate">{org.name}</span>
                </div>
                <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button
                    onClick={(e) => { e.stopPropagation(); handleEnrichOrg(org.id); }}
                    className={`p-1 transition-colors ${enriching ? 'text-amber-500 animate-pulse' : 'text-zinc-400 hover:text-emerald-500'}`}
                    title="Extraer palabras clave"
                    disabled={enriching}
                  >
                    <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.455 2.456L21.75 6l-1.036.259a3.375 3.375 0 00-2.455 2.456z" />
                    </svg>
                  </button>
                  <button
                    onClick={(e) => { e.stopPropagation(); startEdit(org); }}
                    className="p-1 text-zinc-400 hover:text-indigo-500 transition-colors"
                    title="Editar"
                  >
                    <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931z" />
                    </svg>
                  </button>
                  <button
                    onClick={(e) => { e.stopPropagation(); handleToggleOrg(org.id, org.active); }}
                    className="p-1 text-zinc-400 hover:text-amber-500 transition-colors"
                    title={org.active ? 'Desactivar' : 'Activar'}
                  >
                    {org.active ? (
                      <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M14.25 9v6m-4.5 0V9M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                      </svg>
                    ) : (
                      <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0zM15.91 11.672a.375.375 0 010 .656l-5.603 3.113a.375.375 0 01-.557-.328V8.887c0-.286.307-.466.557-.327l5.603 3.112z" />
                      </svg>
                    )}
                  </button>
                  <button
                    onClick={(e) => { e.stopPropagation(); handleDeleteOrg(org.id); }}
                    className="p-1 text-zinc-400 hover:text-red-500 transition-colors"
                    title="Eliminar"
                  >
                    <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
                    </svg>
                  </button>
                </div>
              </div>
              <div className="mt-1.5 flex items-center gap-2 flex-wrap">
                <span className="text-[10px] text-zinc-500 dark:text-zinc-400">
                  {org.keywords.length} palabras clave
                </span>
                {org.youtube_channels.length > 0 && (
                  <span className="text-[10px] text-red-400">{org.youtube_channels.length} YT</span>
                )}
                {org.website && (
                  <span className="text-[10px] text-emerald-500 truncate max-w-[120px]" title={org.website}>
                    {org.website.replace(/^https?:\/\/(www\.)?/, '').replace(/\/$/, '')}
                  </span>
                )}
              </div>
            </div>
          ))}
        </div>

        {/* Right panel: Hits feed */}
        <div className="flex-1 min-w-0 space-y-3">
          {/* Filter bar — newspaper tab style */}
          <div className="flex items-center justify-between gap-3 flex-wrap">
            <div className="flex items-center gap-1 overflow-x-auto">
              {SOURCE_TYPES.map(st => (
                <button
                  key={st.key}
                  onClick={() => setSourceFilter(st.key)}
                  className={`px-3 py-1.5 text-xs font-bold uppercase tracking-wider rounded-sm whitespace-nowrap transition-colors ${
                    sourceFilter === st.key
                      ? 'bg-zinc-900 dark:bg-zinc-100 text-white dark:text-zinc-900'
                      : 'text-zinc-500 dark:text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800'
                  }`}
                >
                  {st.label}
                </button>
              ))}
            </div>
            <div className="flex items-center gap-2">
              {unseenCount > 0 && (
                <span className="text-xs text-red-500 font-bold">{unseenCount} sin leer</span>
              )}
              <button
                onClick={handleMarkAllSeen}
                className="px-3 py-1.5 text-xs font-bold uppercase tracking-wider text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-100 border border-zinc-200 dark:border-zinc-700 rounded-sm transition-colors"
              >
                Marcar todo leido
              </button>
            </div>
          </div>

          {/* Hits list */}
          {hitsLoading ? (
            <div className="flex justify-center py-10">
              <div className="w-5 h-5 border-2 border-indigo-500 border-t-transparent rounded-full animate-spin" />
            </div>
          ) : hits.length === 0 ? (
            <div className="text-center py-16">
              <svg className="w-10 h-10 mx-auto mb-3 text-zinc-300 dark:text-zinc-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" />
              </svg>
              <p className="text-sm text-zinc-500 dark:text-zinc-400">
                {orgs.length === 0
                  ? 'Agrega una organizacion para comenzar el monitoreo'
                  : 'Sin menciones encontradas. Los agentes escanean cada 6 horas.'}
              </p>
            </div>
          ) : (
            <div className="space-y-2">
              {hits.map(hit => (
                <HitCard
                  key={hit.id}
                  hit={hit}
                  expanded={expandedDraft === hit.id}
                  onToggleDraft={() => setExpandedDraft(expandedDraft === hit.id ? null : hit.id)}
                  onMarkSeen={() => handleMarkSeen(hit.id)}
                  onDelete={() => handleDeleteHit(hit.id)}
                />
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Add/Edit Org Modal */}
      {showAddOrg && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => { setShowAddOrg(false); setEditingOrg(null); }} />
          <div className="relative w-full max-w-md mx-4 p-6 bg-white dark:bg-zinc-900 rounded-2xl border border-zinc-200 dark:border-zinc-800 shadow-2xl animate-fade-in">
            <h3 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100 mb-4">
              {editingOrg ? 'Editar organizacion' : 'Nueva organizacion'}
            </h3>

            <div className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-zinc-500 dark:text-zinc-400 mb-1">Nombre</label>
                <input
                  type="text"
                  value={formName}
                  onChange={e => setFormName(e.target.value)}
                  placeholder="ej. Vimenti"
                  className="w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700 rounded-lg text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-zinc-500 dark:text-zinc-400 mb-1">
                  Pagina web <span className="text-zinc-400">(se usara para extraer palabras clave)</span>
                </label>
                <input
                  type="url"
                  value={formWebsite}
                  onChange={e => setFormWebsite(e.target.value)}
                  placeholder="ej. https://www.projectmakerspr.org/"
                  className="w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700 rounded-lg text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-zinc-500 dark:text-zinc-400 mb-1">
                  Palabras clave <span className="text-zinc-400">(separadas por coma — se autoextraen del website)</span>
                </label>
                <textarea
                  value={formKeywords}
                  onChange={e => setFormKeywords(e.target.value)}
                  placeholder="ej. Vimenti, educacion, ninos, afterschool"
                  rows={3}
                  className="w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700 rounded-lg text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 focus:outline-none focus:ring-2 focus:ring-indigo-500 resize-none"
                />
                {formKeywords && (
                  <div className="mt-1.5 flex flex-wrap gap-1">
                    {formKeywords.split(',').map(k => k.trim()).filter(Boolean).map((kw, i) => (
                      <span key={i} className="px-2 py-0.5 text-[10px] rounded-full bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 font-medium">
                        {kw}
                      </span>
                    ))}
                  </div>
                )}
              </div>

              <div>
                <label className="block text-xs font-medium text-zinc-500 dark:text-zinc-400 mb-1">
                  Canales de YouTube <span className="text-zinc-400">(IDs, uno por linea)</span>
                </label>
                <textarea
                  value={formYouTube}
                  onChange={e => setFormYouTube(e.target.value)}
                  placeholder="ej. UCxxxxxxxxxxxxxx"
                  rows={2}
                  className="w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700 rounded-lg text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 focus:outline-none focus:ring-2 focus:ring-indigo-500 resize-none"
                />
              </div>
            </div>

            <div className="mt-6 flex justify-end gap-2">
              <button
                onClick={() => { setShowAddOrg(false); setEditingOrg(null); }}
                className="px-4 py-2 text-sm text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200 transition-colors"
              >
                Cancelar
              </button>
              <button
                onClick={handleSaveOrg}
                disabled={!formName.trim()}
                className="px-4 py-2 text-sm font-medium bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 disabled:hover:bg-indigo-600 text-white rounded-lg transition-colors"
              >
                {editingOrg ? 'Guardar' : 'Crear'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* RSS Feed Modal */}
      {showRSSModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setShowRSSModal(false)} />
          <div className="relative w-full max-w-md mx-4 p-6 bg-white dark:bg-zinc-900 rounded-2xl border border-zinc-200 dark:border-zinc-800 shadow-2xl animate-fade-in">
            <div className="flex items-center gap-3 mb-4">
              <div className="p-2 bg-orange-500/10 rounded-sm">
                <svg className="w-5 h-5 text-orange-500" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M6.18 15.64a2.18 2.18 0 010 4.36 2.18 2.18 0 010-4.36zM4 4.44A15.56 15.56 0 0119.56 20h-2.83A12.73 12.73 0 004 7.27V4.44zm0 5.66a9.9 9.9 0 019.9 9.9h-2.83A7.07 7.07 0 004 12.93V10.1z" />
                </svg>
              </div>
              <h3 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">Feed RSS</h3>
            </div>

            <p className="text-sm text-zinc-500 dark:text-zinc-400 mb-4">
              Suscribete a este feed RSS en cualquier lector (Feedly, Thunderbird, NetNewsWire) para recibir las menciones automaticamente.
            </p>

            <div className="flex items-center gap-2">
              <input
                type="text"
                readOnly
                value={feedURL}
                className="flex-1 px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700 rounded-sm text-zinc-900 dark:text-zinc-100 font-mono text-[11px] select-all"
                onFocus={e => e.target.select()}
              />
              <button
                onClick={handleCopyFeedURL}
                className={`px-3 py-2 text-sm font-medium rounded-sm transition-colors ${
                  feedCopied
                    ? 'bg-emerald-600 text-white'
                    : 'bg-zinc-800 hover:bg-indigo-600 text-white'
                }`}
              >
                {feedCopied ? 'Copiado' : 'Copiar'}
              </button>
            </div>

            <div className="mt-3">
              <button
                onClick={handleRegenerateFeed}
                disabled={regenerating}
                className="text-xs text-red-500 hover:text-red-600 dark:text-red-400 dark:hover:text-red-300 disabled:opacity-50 transition-colors"
              >
                {regenerating ? 'Regenerando...' : 'Regenerar token'}
              </button>
              <p className="text-[10px] text-zinc-400 dark:text-zinc-500 mt-0.5">
                Esto invalidara el URL anterior.
              </p>
            </div>

            <div className="mt-4 flex justify-end">
              <button
                onClick={() => setShowRSSModal(false)}
                className="px-4 py-2 text-sm text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200 transition-colors"
              >
                Cerrar
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Hit Card Component — Newspaper Style ─────────────────────────

function HitCard({
  hit,
  expanded,
  onToggleDraft,
  onMarkSeen,
  onDelete,
}: {
  hit: WatchlistHit;
  expanded: boolean;
  onToggleDraft: () => void;
  onMarkSeen: () => void;
  onDelete: () => void;
}) {
  const sentiment = SENTIMENT_STYLES[hit.sentiment] || SENTIMENT_STYLES.unknown;

  return (
    <div className={`overflow-hidden rounded-sm border transition-all ${
      !hit.seen
        ? 'border-l-4 border-l-red-500 border-zinc-200 dark:border-zinc-800'
        : 'border-zinc-200 dark:border-zinc-800'
    }`}>
      {/* Dark header bar */}
      <div className="bg-zinc-800 dark:bg-zinc-950 px-3 py-2 flex items-center justify-between">
        <div className="flex items-center gap-1.5 text-[10px] text-zinc-400 min-w-0">
          <SourceIcon type={hit.source_type} className="w-3.5 h-3.5 text-zinc-400 shrink-0" />
          <span className="font-medium text-zinc-300 uppercase tracking-wider">
            {SOURCE_LABELS[hit.source_type] || hit.source_type}
          </span>
          {hit.org_name && (
            <>
              <span className="text-zinc-600 shrink-0">&middot;</span>
              <span className="text-zinc-400 truncate">{hit.org_name}</span>
            </>
          )}
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <span className="text-[10px] text-zinc-500">{timeAgo(hit.created_at)}</span>
          <span className={`inline-flex items-center px-2 py-0.5 text-[9px] font-bold uppercase tracking-widest rounded-sm ${sentiment.pill}`}>
            {sentiment.label}
          </span>
        </div>
      </div>

      {/* Title area — newspaper style */}
      <div className="bg-zinc-50 dark:bg-zinc-900 px-3 py-3">
        <a
          href={hit.url}
          target="_blank"
          rel="noopener noreferrer"
          className="text-[15px] font-extrabold text-zinc-900 dark:text-zinc-50 leading-tight line-clamp-2 uppercase tracking-tight hover:text-indigo-600 dark:hover:text-indigo-400 transition-colors"
        >
          {hit.title || 'Sin titulo'}
        </a>
      </div>

      {/* Snippet body */}
      {hit.snippet && (
        <div className="bg-white dark:bg-zinc-900 px-3 pt-1 pb-2">
          <p className="text-[11px] text-zinc-600 dark:text-zinc-400 leading-relaxed line-clamp-3">{hit.snippet}</p>
        </div>
      )}

      {/* AI Draft expansion */}
      {hit.ai_draft && expanded && (
        <div className="px-3 pb-2 bg-white dark:bg-zinc-900">
          <div className="p-3 rounded-sm bg-indigo-500/5 border border-indigo-500/20 text-sm text-zinc-700 dark:text-zinc-300 whitespace-pre-wrap leading-relaxed">
            {hit.ai_draft}
          </div>
        </div>
      )}

      {/* Bottom bar */}
      <div className="flex items-center gap-1 px-2 py-1.5 bg-zinc-50 dark:bg-zinc-950 border-t border-zinc-200 dark:border-zinc-800">
        {!hit.seen && (
          <button
            onClick={onMarkSeen}
            className="inline-flex items-center gap-1 px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-white bg-zinc-800 hover:bg-indigo-600 rounded-sm transition-colors"
          >
            <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
            </svg>
            Leido
          </button>
        )}
        <button
          onClick={onDelete}
          className="inline-flex items-center gap-1 px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-zinc-500 hover:text-red-500 bg-zinc-200 dark:bg-zinc-800 hover:bg-red-50 dark:hover:bg-red-500/10 rounded-sm transition-colors"
          title="Eliminar"
        >
          <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
          </svg>
          Eliminar
        </button>
        {hit.ai_draft && (
          <button
            onClick={onToggleDraft}
            className="inline-flex items-center gap-1 px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-indigo-500 hover:bg-indigo-500/10 rounded-sm transition-colors"
          >
            <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
            </svg>
            {expanded ? 'Ocultar PR' : 'Borrador PR'}
          </button>
        )}
        {hit.url && (
          <a
            href={hit.url}
            target="_blank"
            rel="noopener noreferrer"
            className="ml-auto p-1 text-zinc-400 hover:text-indigo-500 transition-colors"
            title="Abrir original"
          >
            <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M13.5 6H5.25A2.25 2.25 0 003 8.25v10.5A2.25 2.25 0 005.25 21h10.5A2.25 2.25 0 0018 18.75V10.5m-10.5 6L21 3m0 0h-5.25M21 3v5.25" />
            </svg>
          </a>
        )}
      </div>
    </div>
  );
}
