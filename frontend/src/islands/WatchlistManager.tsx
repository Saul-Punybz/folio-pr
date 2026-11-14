import { useState, useEffect, useCallback } from 'react';
import { api, type WatchlistOrg, type WatchlistHit } from '../lib/api';
import { Icon, Spinner, EmptyState, Modal } from './ui';

const SOURCE_TYPES = [
  { key: 'all', label: 'Todos' },
  { key: 'google_news', label: 'Google News' },
  { key: 'web', label: 'Web' },
  { key: 'local', label: 'Local' },
  { key: 'youtube', label: 'YouTube' },
  { key: 'reddit', label: 'Reddit' },
] as const;

function timeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diff = now - then;
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'ahora';
  if (mins < 60) return `hace ${mins} min`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `hace ${hrs} h`;
  const days = Math.floor(hrs / 24);
  if (days < 30) return `hace ${days} d`;
  return `hace ${Math.floor(days / 30)} m`;
}

// ── SVG Source Icons ─────────────────────────────────────────────

function SourceIcon({ type, className = 'w-5 h-5' }: { type: string; className?: string }) {
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
      if (editingOrg) {
        await api.updateWatchlistOrg(editingOrg.id, { name: formName, website, keywords, youtube_channels });
      } else {
        await api.createWatchlistOrg({ name: formName, website, keywords, youtube_channels });
      }
      setShowAddOrg(false);
      setEditingOrg(null);
      resetForm();
      await fetchOrgs();
    } catch (e) {
      console.error('Failed to save org:', e);
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
      <div className="flex items-center justify-center py-20 text-primary">
        <Spinner className="w-8 h-8" />
      </div>
    );
  }

  const closeOrgModal = () => { setShowAddOrg(false); setEditingOrg(null); };

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <h1 className="title-xl">Vigilancia</h1>
          {unseenCount > 0 && <span className="tag tag-accent !text-sm !px-3.5 !py-1.5">{unseenCount} sin leer</span>}
        </div>
        <button type="button" onClick={handleShowRSS} className="btn btn-soft btn-sm" title="Feed RSS">
          <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path d="M6.18 15.64a2.18 2.18 0 010 4.36 2.18 2.18 0 010-4.36zM4 4.44A15.56 15.56 0 0119.56 20h-2.83A12.73 12.73 0 004 7.27V4.44zm0 5.66a9.9 9.9 0 019.9 9.9h-2.83A7.07 7.07 0 004 12.93V10.1z" />
          </svg>
          Feed RSS
        </button>
      </div>

      <div className="flex flex-col lg:flex-row gap-6">
        {/* Organizaciones */}
        <div className="w-full lg:w-80 shrink-0 space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="title-lg">Organizaciones ({orgs.length})</h2>
            <button type="button" onClick={() => { resetForm(); setEditingOrg(null); setShowAddOrg(true); }} className="btn btn-tonal btn-sm">
              <Icon name="plus" className="w-5 h-5" />Agregar
            </button>
          </div>

          {orgs.length === 0 ? (
            <EmptyState icon="eye" text="Agrega una organización para empezar a vigilar." />
          ) : (
            <div className="seg-list">
              {orgs.map(org => (
                <div
                  key={org.id}
                  className={`p-4 cursor-pointer ${selectedOrg === org.id ? '!bg-primary-container text-on-primary-container' : ''}`}
                  onClick={() => setSelectedOrg(selectedOrg === org.id ? null : org.id)}
                  role="button"
                  tabIndex={0}
                  aria-pressed={selectedOrg === org.id}
                  onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setSelectedOrg(selectedOrg === org.id ? null : org.id); } }}
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex items-center gap-2 min-w-0">
                      <span className={`w-3 h-3 rounded-full shrink-0 ${org.active ? 'bg-success' : 'bg-outline'}`} title={org.active ? 'Activa' : 'En pausa'} />
                      <span className="title-md truncate">{org.name}</span>
                    </div>
                    <div className="flex items-center shrink-0 -mr-2">
                      <button type="button" onClick={(e) => { e.stopPropagation(); startEdit(org); }} className="icon-btn" title="Editar" aria-label={`Editar ${org.name}`}>
                        <Icon name="note" className="w-5 h-5" />
                      </button>
                      <button type="button" onClick={(e) => { e.stopPropagation(); handleToggleOrg(org.id, org.active); }} className="icon-btn" title={org.active ? 'Pausar' : 'Activar'} aria-label={`${org.active ? 'Pausar' : 'Activar'} ${org.name}`}>
                        {org.active ? (
                          <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75}><path strokeLinecap="round" strokeLinejoin="round" d="M14.25 9v6m-4.5 0V9M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                        ) : (
                          <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75}><path strokeLinecap="round" strokeLinejoin="round" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0zM15.91 11.672a.375.375 0 010 .656l-5.603 3.113a.375.375 0 01-.557-.328V8.887c0-.286.307-.466.557-.327l5.603 3.112z" /></svg>
                        )}
                      </button>
                      <button type="button" onClick={(e) => { e.stopPropagation(); if (confirm(`¿Quitar a ${org.name}?`)) handleDeleteOrg(org.id); }} className="icon-btn hover:text-error" title="Quitar" aria-label={`Quitar ${org.name}`}>
                        <Icon name="trash" className="w-5 h-5" />
                      </button>
                    </div>
                  </div>
                  <div className="mt-1 flex items-center gap-2 flex-wrap text-xs font-semibold opacity-80">
                    <span>{org.keywords.length} palabras clave</span>
                    {org.youtube_channels.length > 0 && <span>{org.youtube_channels.length} canales</span>}
                    {org.website && (
                      <span className="truncate max-w-[140px]" title={org.website}>
                        {org.website.replace(/^https?:\/\/(www\.)?/, '').replace(/\/$/, '')}
                      </span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Menciones */}
        <div className="flex-1 min-w-0 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="chip-row chip-scroll" role="group" aria-label="Origen">
              {SOURCE_TYPES.map(st => (
                <button key={st.key} type="button" onClick={() => setSourceFilter(st.key)} className="chip" aria-pressed={sourceFilter === st.key}>
                  {st.label}
                </button>
              ))}
            </div>
            {unseenCount > 0 && (
              <button type="button" onClick={handleMarkAllSeen} className="btn btn-text btn-sm">
                <Icon name="check" className="w-5 h-5" />Marcar todo leído
              </button>
            )}
          </div>

          {hitsLoading ? (
            <div className="flex justify-center py-10 text-primary"><Spinner className="w-7 h-7" /></div>
          ) : hits.length === 0 ? (
            <EmptyState
              icon="eye"
              text={orgs.length === 0 ? 'Agrega una organización para empezar.' : 'Aún no hay menciones. Toca «Buscar ahora».'}
            />
          ) : (
            <div className="stagger grid grid-cols-1 xl:grid-cols-2 gap-4 items-start">
              {hits.map(hit => (
                <HitCard key={hit.id} hit={hit} onMarkSeen={() => handleMarkSeen(hit.id)} onDelete={() => handleDeleteHit(hit.id)} />
              ))}
            </div>
          )}
        </div>
      </div>

      {/* La acción principal */}
      <div className="fab-slot">
        <button type="button" onClick={handleTriggerScan} disabled={scanning || orgs.length === 0} className="fab disabled:opacity-50">
          {scanning ? <Spinner className="w-6 h-6" /> : <Icon name="search" className="w-6 h-6" />}
          {scanning ? 'Buscando...' : 'Buscar ahora'}
        </button>
      </div>

      {showAddOrg && (
        <Modal title={editingOrg ? 'Editar organización' : 'Nueva organización'} onClose={closeOrgModal}>
          <div className="space-y-4">
            <div>
              <label htmlFor="wo-name" className="field-label">Nombre</label>
              <input id="wo-name" type="text" value={formName} onChange={e => setFormName(e.target.value)} placeholder="Ej. Vimenti" className="field" autoFocus />
            </div>
            <div>
              <label htmlFor="wo-web" className="field-label">Página web</label>
              <input id="wo-web" type="url" value={formWebsite} onChange={e => setFormWebsite(e.target.value)} placeholder="https://www.ejemplo.org/" className="field" />
            </div>
            <div>
              <label htmlFor="wo-kw" className="field-label">Palabras clave (separadas por coma)</label>
              <textarea id="wo-kw" value={formKeywords} onChange={e => setFormKeywords(e.target.value)} placeholder="Vimenti, educación, niños" rows={3} className="field" />
              {formKeywords && (
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {formKeywords.split(',').map(k => k.trim()).filter(Boolean).map((kw, i) => (
                    <span key={i} className="tag tag-primary">{kw}</span>
                  ))}
                </div>
              )}
            </div>
            <div>
              <label htmlFor="wo-yt" className="field-label">Canales de YouTube (uno por línea)</label>
              <textarea id="wo-yt" value={formYouTube} onChange={e => setFormYouTube(e.target.value)} placeholder="UCxxxxxxxxxxxxxx" rows={2} className="field" />
            </div>
          </div>
          <div className="mt-6 flex justify-end gap-2">
            <button type="button" onClick={closeOrgModal} className="btn btn-text">Cancelar</button>
            <button type="button" onClick={handleSaveOrg} disabled={!formName.trim()} className="btn btn-filled">{editingOrg ? 'Guardar' : 'Crear'}</button>
          </div>
        </Modal>
      )}

      {showRSSModal && (
        <Modal title="Feed RSS" onClose={() => setShowRSSModal(false)}>
          <p className="text-sm text-on-surface-variant mb-4">Pega este enlace en tu lector de noticias para recibir las menciones.</p>
          <div className="flex items-center gap-2">
            <input type="text" readOnly value={feedURL} aria-label="Enlace del feed" className="field font-mono text-xs select-all" onFocus={e => e.target.select()} />
            <button type="button" onClick={handleCopyFeedURL} className={`btn btn-sm shrink-0 ${feedCopied ? 'btn-tonal' : 'btn-filled'}`}>
              {feedCopied ? 'Copiado' : 'Copiar'}
            </button>
          </div>
          <div className="mt-4">
            <button type="button" onClick={handleRegenerateFeed} disabled={regenerating} className="btn btn-danger btn-sm">
              {regenerating ? 'Cambiando...' : 'Cambiar enlace'}
            </button>
            <p className="text-xs text-on-surface-variant mt-2">El enlace anterior dejará de servir.</p>
          </div>
        </Modal>
      )}
    </div>
  );
}

// ── Mención ──────────────────────────────────────────────────────

function HitCard({
  hit,
  onMarkSeen,
  onDelete,
}: {
  hit: WatchlistHit;
  onMarkSeen: () => void;
  onDelete: () => void;
}) {
  return (
    <article className={`flex flex-col overflow-hidden rounded-[28px] hover:rounded-[36px] transition-[border-radius,background-color] duration-500 ease-[var(--spring)] ${hit.seen ? 'bg-surface-container-low' : 'bg-surface-container ring-[3px] ring-tertiary'}`}>
      <div className="px-5 pt-5 flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-xs font-semibold text-on-surface-variant min-w-0">
          <SourceIcon type={hit.source_type} className="w-5 h-5 shrink-0" />
          <span className="uppercase tracking-wider">{SOURCE_LABELS[hit.source_type] || hit.source_type}</span>
          {hit.org_name && (
            <>
              <span aria-hidden="true">&middot;</span>
              <span className="truncate">{hit.org_name}</span>
            </>
          )}
        </div>
        <span className="text-xs font-semibold text-on-surface-variant shrink-0">{timeAgo(hit.created_at)}</span>
      </div>

      <h3 className="px-5 pt-3 text-lg font-extrabold leading-tight tracking-tight line-clamp-3">
        <a href={hit.url} target="_blank" rel="noopener noreferrer" className="hover:text-primary transition-colors">
          {hit.title || 'Sin título'}
        </a>
      </h3>

      {hit.snippet && <p className="px-5 pt-2 text-sm text-on-surface-variant leading-relaxed line-clamp-3">{hit.snippet}</p>}

      <div className="flex items-center gap-2 p-3 pt-4">
        {!hit.seen && (
          <button type="button" onClick={onMarkSeen} className="btn btn-filled btn-sm">
            <Icon name="check" className="w-5 h-5" />Leído
          </button>
        )}
        <button type="button" onClick={onDelete} className="btn btn-soft btn-sm" title="Quitar">
          <Icon name="trash" className="w-5 h-5" />Quitar
        </button>
        {hit.url && (
          <a href={hit.url} target="_blank" rel="noopener noreferrer" className="icon-btn ml-auto" title="Abrir original" aria-label="Abrir original">
            <Icon name="open" />
          </a>
        )}
      </div>
    </article>
  );
}
