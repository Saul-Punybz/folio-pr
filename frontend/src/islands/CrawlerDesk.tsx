import { useState, useEffect, useCallback, useRef } from 'react';
import { api, type Opportunity, type OpportunityLevel, type FundType, type KeptDocument, type KeptDocumentsResponse, type CrawlDomain, type CrawlRun, type OpportunityStatsResponse } from '../lib/api';
import { Icon, Spinner, EmptyState, Modal, Notice } from './ui';

// ── Helpers ────────────────────────────────────────────────────────

function parseDay(s?: string | null): Date | null {
  if (!s) return null;
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(s);
  const d = m ? new Date(+m[1], +m[2] - 1, +m[3]) : new Date(s);
  if (isNaN(d.getTime()) || d.getFullYear() < 2000) return null;
  return d;
}

/** Días completos que faltan (0 = hoy, negativo = ya cerró). null si no hay fecha. */
function daysLeft(s?: string | null): number | null {
  const d = parseDay(s);
  if (!d) return null;
  const t = new Date();
  const today = new Date(t.getFullYear(), t.getMonth(), t.getDate());
  return Math.round((d.getTime() - today.getTime()) / 86400000);
}

function leftLabel(n: number): string {
  if (n < 0) return `Cerró hace ${Math.abs(n)} ${Math.abs(n) === 1 ? 'día' : 'días'}`;
  if (n === 0) return 'Cierra hoy';
  if (n === 1) return 'Falta 1 día';
  return `Faltan ${n} días`;
}

function formatDate(dateStr?: string | null): string {
  const d = parseDay(dateStr);
  if (!d) return 'Sin fecha';
  return d.toLocaleDateString('es-PR', { month: 'short', day: 'numeric', year: 'numeric' });
}

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
  return `hace ${days} d`;
}

/** Pone el número de fondos por revisar en el badge de la navegación. */
function setNavBadge(n: number) {
  document.querySelectorAll('.fondos-badge').forEach(b => {
    if (n > 0) { b.textContent = n > 99 ? '99+' : String(n); b.classList.remove('hidden'); }
    else b.classList.add('hidden');
  });
}

const CATEGORY_LABELS: Record<string, string> = {
  rfp: 'RFP', grant: 'Subvención', funding: 'Fondos', proposal: 'Propuesta',
  contract: 'Contrato', fellowship: 'Programa', scholarship: 'Beca', other: 'Otro',
};

const LEVELS: { value: OpportunityLevel; label: string }[] = [
  { value: 'federal', label: 'Federal' },
  { value: 'estatal', label: 'Gobierno central' },
  { value: 'municipal', label: 'Municipio' },
  { value: 'local', label: 'Local' },
];
const LEVEL_LABELS: Record<string, string> = Object.fromEntries(LEVELS.map(l => [l.value, l.label]));

const FUND_TYPES: { value: FundType; label: string }[] = [
  { value: 'grant', label: 'Grants' },
  { value: 'subasta', label: 'Subastas' },
  { value: 'rfp', label: 'RFP' },
  { value: 'otros', label: 'Otros' },
];

function formatSize(n: number): string {
  if (n >= 1073741824) return `${(n / 1073741824).toFixed(1)} GB`;
  if (n >= 1048576) return `${(n / 1048576).toFixed(1)} MB`;
  return `${Math.max(1, Math.round(n / 1024))} KB`;
}

const DOMAIN_CATEGORY_OPTIONS = [
  { value: 'government', label: 'Gobierno' },
  { value: 'legislature', label: 'Legislatura' },
  { value: 'executive', label: 'Ejecutivo' },
  { value: 'municipal', label: 'Municipal' },
  { value: 'news', label: 'Noticias' },
  { value: 'federal', label: 'Federal' },
  { value: 'other', label: 'Otro' },
];

const UNDO_MS = 6000;

type Tab = 'fondos' | 'paginas';
type View = 'pending' | 'saved' | 'pdfs';

// Process phases for the single-button flow
type ProcessPhase = 'idle' | 'crawling' | 'done';

export default function CrawlerDesk() {
  const [activeTab, setActiveTab] = useState<Tab>('fondos');

  // ─ Global process state (survives tab switches) ─
  const [phase, setPhase] = useState<ProcessPhase>('idle');
  const [statusMsg, setStatusMsg] = useState('');
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const refreshCallbackRef = useRef<(() => void) | null>(null);

  // ─ Shared domain list (used by both tabs) ─
  const [domains, setDomains] = useState<CrawlDomain[]>([]);

  const loadDomains = useCallback(async () => {
    try {
      const res = await api.getCrawlerDomains();
      setDomains(res.domains || []);
    } catch (e) { console.error(e); }
  }, []);

  useEffect(() => { loadDomains(); }, [loadDomains]);

  // Cleanup on unmount
  useEffect(() => {
    return () => { if (pollRef.current) clearInterval(pollRef.current); };
  }, []);

  // ─ Single "Buscar" action: dispara el crawl y refresca ─
  const handleFullScan = useCallback(async () => {
    if (phase !== 'idle') return;

    setPhase('crawling');
    setStatusMsg('Buscando en las páginas vigiladas...');

    try { await api.triggerCrawl(); } catch (e) { console.error(e); }
    refreshCallbackRef.current?.();

    // Refresca mientras el crawl avanza
    let elapsed = 0;
    const prevTotal = { current: 0 };
    pollRef.current = setInterval(async () => {
      elapsed += 5;
      try {
        const s = await api.getOpportunityStats();
        const newTotal = s.total || 0;
        if (newTotal > prevTotal.current) {
          setStatusMsg(`${newTotal} fondos encontrados...`);
          prevTotal.current = newTotal;
          refreshCallbackRef.current?.();
        }
      } catch { /* ignore */ }

      if (elapsed >= 60) {
        if (pollRef.current) clearInterval(pollRef.current);
        setPhase('done');
        setStatusMsg(prevTotal.current > 0
          ? `Búsqueda completa. ${prevTotal.current} fondos.`
          : 'Búsqueda completa.');
        refreshCallbackRef.current?.();
        setTimeout(() => { setPhase('idle'); setStatusMsg(''); }, 8000);
      }
    }, 5000);
  }, [phase]);

  const isBusy = phase !== 'idle' && phase !== 'done';

  return (
    <div className="space-y-5">
      <h1 className="title-xl">Fondos</h1>

      {phase !== 'idle' && (
        <div role="status" className={`flex items-center gap-3 px-5 py-4 rounded-[24px] text-sm font-bold ${phase === 'done' ? 'bg-success-container text-on-success-container' : 'bg-primary-container text-on-primary-container'}`}>
          {isBusy && <Spinner className="w-5 h-5 shrink-0" />}
          {phase === 'done' && <Icon name="check" className="w-6 h-6 shrink-0" />}
          <span className="flex-1">{statusMsg}</span>
        </div>
      )}

      <div className="chip-row" role="tablist" aria-label="Secciones">
        {(['fondos', 'paginas'] as Tab[]).map(tab => (
          <button key={tab} type="button" role="tab" aria-selected={activeTab === tab} aria-pressed={activeTab === tab} onClick={() => setActiveTab(tab)} className="chip">
            {tab === 'fondos' ? 'Fondos' : 'Páginas vigiladas'}
          </button>
        ))}
      </div>

      {activeTab === 'fondos' && (
        <OpportunityTab domains={domains} refreshCallbackRef={refreshCallbackRef} isBusy={isBusy} onScan={handleFullScan} />
      )}
      {activeTab === 'paginas' && (
        <DominiosTab domains={domains} onDomainsChanged={loadDomains} />
      )}

      {/* La acción principal de la pantalla */}
      <div className="fab-slot">
        <button
          type="button"
          onClick={handleFullScan}
          disabled={isBusy || domains.length === 0}
          className="fab disabled:opacity-50"
          title={domains.length === 0 ? 'Agrega páginas primero' : 'Buscar fondos nuevos'}
        >
          {isBusy ? <Spinner className="w-6 h-6" /> : <Icon name="search" className="w-6 h-6" />}
          {isBusy ? 'Buscando...' : 'Buscar fondos'}
        </button>
      </div>
    </div>
  );
}

// ── Fondos ─────────────────────────────────────────────────────────

function OpportunityTab({ domains, refreshCallbackRef, isBusy, onScan }: {
  domains: CrawlDomain[];
  refreshCallbackRef: React.MutableRefObject<(() => void) | null>;
  isBusy: boolean;
  onScan: () => void;
}) {
  const [opps, setOpps] = useState<Opportunity[]>([]);
  const [stats, setStats] = useState<OpportunityStatsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [view, setView] = useState<View | null>(null);
  const [levelFilter, setLevelFilter] = useState<OpportunityLevel | ''>('');
  const [typeFilter, setTypeFilter] = useState<FundType | ''>('');
  const [info, setInfo] = useState('');
  const [categoryFilter, setCategoryFilter] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [error, setError] = useState('');
  const [hidden, setHidden] = useState<Set<string>>(new Set());
  const [undo, setUndo] = useState<Opportunity[]>([]);
  const timers = useRef(new Map<string, ReturnType<typeof setTimeout>>());

  // Vista inicial: por revisar si hay pendientes
  useEffect(() => {
    if (view !== null) return;
    api.getOpportunityStats()
      .then(s => { setStats(s); setView((s.pending || 0) > 0 ? 'pending' : 'saved'); })
      .catch(() => setView('saved'));
  }, [view]);

  const loadData = useCallback(async () => {
    if (view === null) return;
    if (view === 'pdfs') { setLoading(false); return; }
    setLoading(true);
    try {
      const [oppRes, statsRes] = await Promise.all([
        searchQuery
          ? api.searchOpportunities(searchQuery, 100)
          : api.getOpportunities({
              reviewed: view === 'saved',
              level: levelFilter || undefined,
              category: categoryFilter || undefined,
              sort: view === 'saved' ? 'deadline' : 'created_at',
              limit: view === 'saved' ? 500 : 100,
            }),
        api.getOpportunityStats(),
      ]);
      setOpps(oppRes.opportunities || []);
      setStats(statsRes);
    } catch (e) {
      console.error('Failed to load opportunities', e);
    } finally {
      setLoading(false);
    }
  }, [view, levelFilter, categoryFilter, searchQuery]);

  useEffect(() => { loadData(); }, [loadData]);

  // Register the refresh callback so parent can trigger refreshes
  useEffect(() => {
    refreshCallbackRef.current = loadData;
    return () => { refreshCallbackRef.current = null; };
  }, [loadData, refreshCallbackRef]);

  useEffect(() => {
    if (stats && typeof stats.pending === 'number') setNavBadge(stats.pending);
  }, [stats?.pending]);

  // Si se sale de la pantalla con un «Deshacer» pendiente, se aplica el descarte
  useEffect(() => {
    const flush = () => {
      timers.current.forEach((t, id) => { clearTimeout(t); api.deleteOpportunity(id).catch(() => {}); });
      timers.current.clear();
    };
    window.addEventListener('pagehide', flush);
    return () => { window.removeEventListener('pagehide', flush); flush(); };
  }, []);

  const adjustPending = (d: number) => {
    setStats(s => s ? { ...s, pending: Math.max(0, (s.pending || 0) + d) } : s);
  };

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setSearchQuery(searchInput.trim());
  };

  const handleSave = async (opp: Opportunity) => {
    setError('');
    setInfo('');
    setOpps(o => o.filter(x => x.id !== opp.id));
    adjustPending(-1);
    try {
      const res = await api.reviewOpportunity(opp.id);
      if (res.pdf === 'remote') setInfo('Se guardó el fondo, pero no se pudo bajar el PDF. Queda el enlace al sitio.');
    } catch (err) {
      console.error('Review error', err);
      setError('No se pudo guardar. Inténtalo otra vez.');
      loadData();
    }
  };

  const handleUnsave = async (opp: Opportunity) => {
    setError('');
    setInfo('');
    setOpps(o => o.filter(x => x.id !== opp.id));
    adjustPending(1);
    try {
      await api.unreviewOpportunity(opp.id);
    } catch (err) {
      console.error('Unreview error', err);
      setError('No se pudo quitar de guardados. Inténtalo otra vez.');
      loadData();
    }
  };

  const handleDiscard = (opp: Opportunity) => {
    setError('');
    setHidden(h => new Set(h).add(opp.id));
    if (!opp.reviewed) adjustPending(-1);
    setUndo(u => [...u, opp]);
    const t = setTimeout(async () => {
      timers.current.delete(opp.id);
      setUndo(u => u.filter(x => x.id !== opp.id));
      try {
        await api.deleteOpportunity(opp.id);
      } catch (err) {
        console.error('Delete opp error', err);
        setHidden(h => { const n = new Set(h); n.delete(opp.id); return n; });
        if (!opp.reviewed) adjustPending(1);
        setError('No se pudo descartar. Inténtalo otra vez.');
      }
    }, UNDO_MS);
    timers.current.set(opp.id, t);
  };

  const handleUndo = (opp: Opportunity) => {
    const t = timers.current.get(opp.id);
    if (t) clearTimeout(t);
    timers.current.delete(opp.id);
    setUndo(u => u.filter(x => x.id !== opp.id));
    setHidden(h => { const n = new Set(h); n.delete(opp.id); return n; });
    if (!opp.reviewed) adjustPending(1);
  };

  const pendingCount = stats?.pending || 0;
  const hasDomains = domains.length > 0;

  if (!hasDomains) {
    return (
      <EmptyState icon="globe" text="Agrega las páginas que quieres vigilar.">
        <span className="text-sm font-semibold">Abre «Páginas vigiladas» arriba.</span>
      </EmptyState>
    );
  }

  const wantReviewed = view === 'saved';
  const visible = opps.filter(o =>
    !hidden.has(o.id) &&
    (!!o.reviewed) === wantReviewed &&
    (!levelFilter || o.level === undefined || o.level === levelFilter) &&
    (!categoryFilter || o.category === categoryFilter) &&
    (!wantReviewed || !typeFilter || o.fund_type === typeFilter),
  );
  const typeChips = FUND_TYPES.filter(t => t.value !== 'otros' || opps.some(o => o.fund_type === 'otros'));

  const closing = wantReviewed
    ? visible
        .map(o => ({ o, n: daysLeft(o.deadline) }))
        .filter((x): x is { o: Opportunity; n: number } => x.n !== null && x.n >= 0 && x.n <= 14)
        .sort((a, b) => a.n - b.n)
    : [];

  const resetFilters = () => { setLevelFilter(''); setTypeFilter(''); setCategoryFilter(''); setSearchQuery(''); setSearchInput(''); };

  return (
    <div className="space-y-5">
      <div className="chip-row" role="group" aria-label="Vista">
        <button type="button" className="chip" aria-pressed={view === 'pending'} onClick={() => setView('pending')}>
          Por revisar ({pendingCount})
        </button>
        <button type="button" className="chip" aria-pressed={view === 'saved'} onClick={() => setView('saved')}>
          Guardados
        </button>
        <button type="button" className="chip" aria-pressed={view === 'pdfs'} onClick={() => setView('pdfs')}>
          PDF guardados
        </button>
      </div>

      {view === 'pdfs' && <KeptDocsView />}

      {view !== 'pdfs' && wantReviewed && (
        <div className="chip-row chip-scroll" role="group" aria-label="Tipo">
          <button type="button" className="chip" aria-pressed={typeFilter === ''} onClick={() => setTypeFilter('')}>Todos</button>
          {typeChips.map(t => (
            <button key={t.value} type="button" className="chip" aria-pressed={typeFilter === t.value}
              onClick={() => setTypeFilter(typeFilter === t.value ? '' : t.value)}>
              {t.label}
            </button>
          ))}
        </div>
      )}

      {view !== 'pdfs' && <div className="space-y-3">
        <div className="chip-row chip-scroll" role="group" aria-label="Nivel">
          {LEVELS.map(l => (
            <button key={l.value} type="button" className="chip" aria-pressed={levelFilter === l.value}
              onClick={() => setLevelFilter(levelFilter === l.value ? '' : l.value)}>
              {l.label}
            </button>
          ))}
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <label className="sr-only" htmlFor="op-cat">Categoría</label>
          <select id="op-cat" value={categoryFilter} onChange={e => setCategoryFilter(e.target.value)} className="field !w-auto font-semibold text-sm">
            <option value="">Todas las categorías</option>
            {Object.entries(CATEGORY_LABELS).map(([k, v]) => (
              <option key={k} value={k}>{v}</option>
            ))}
          </select>
          <form onSubmit={handleSearch} className="flex gap-2 ml-auto min-w-[220px] flex-1 max-w-sm" role="search">
            <input type="search" value={searchInput} onChange={e => setSearchInput(e.target.value)} placeholder="Buscar fondos" aria-label="Buscar fondos" className="field" />
            <button type="submit" className="icon-btn bg-primary-container text-on-primary-container shrink-0" aria-label="Buscar"><Icon name="search" /></button>
          </form>
        </div>
      </div>}

      {error && <Notice>{error}</Notice>}
      {info && <Notice kind="accent">{info}</Notice>}

      {view !== 'pdfs' && closing.length > 0 && (
        <section className="space-y-3" aria-label="Cierran pronto">
          <h2 className="title-lg">Cierran pronto</h2>
          <div className="chip-row chip-scroll !flex-nowrap overflow-x-auto">
            {closing.map(({ o, n }) => (
              <div key={o.id} className="shrink-0 w-64 p-4 rounded-[28px] bg-tertiary-container text-on-tertiary-container">
                <div className="text-2xl font-extrabold leading-tight tracking-tight">{leftLabel(n)}</div>
                <div className="text-sm font-bold mt-2 line-clamp-2">{o.title || 'Sin título'}</div>
                <div className="text-xs font-semibold mt-1 opacity-80">{formatDate(o.deadline)}</div>
              </div>
            ))}
          </div>
        </section>
      )}

      {view === 'pdfs' ? null : loading || view === null ? (
        <div className="space-y-2">
          {[0, 1, 2].map(i => <div key={i} className="h-28 rounded-[28px] bg-surface-container-low animate-pulse" />)}
        </div>
      ) : visible.length === 0 ? (
        levelFilter || typeFilter || categoryFilter || searchQuery ? (
          <EmptyState icon="search" text="Nada con esos filtros.">
            <button type="button" onClick={resetFilters} className="btn btn-tonal">Quitar filtros</button>
          </EmptyState>
        ) : wantReviewed ? (
          <EmptyState icon="inbox" text="Aún no has guardado fondos." />
        ) : (
          <EmptyState icon="search" text="No hay fondos por revisar.">
            {!isBusy && <button type="button" onClick={onScan} className="btn btn-filled">Buscar ahora</button>}
          </EmptyState>
        )
      ) : (
        <div className="stagger grid grid-cols-1 xl:grid-cols-2 gap-4 items-start">
          {visible.map(opp => (
            <OpportunityCard key={opp.id} opp={opp} onSave={handleSave} onUnsave={handleUnsave} onDiscard={handleDiscard} onSaved={loadData} />
          ))}
        </div>
      )}

      {undo.length > 0 && (
        <div className="snack-stack" aria-live="polite">
          {undo.map(o => (
            <div key={o.id} className="snack toast-enter">
              <span className="flex-1 line-clamp-1">Descartado</span>
              <button type="button" className="btn btn-text" onClick={() => handleUndo(o)}>
                <Icon name="undo" className="w-5 h-5" />Deshacer
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}


/** PDF guardados: lista, espacio que ocupan y cuánto pesa Folio en total. */
function KeptDocsView() {
  const [data, setData] = useState<KeptDocumentsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setData(await api.getKeptDocuments());
    } catch {
      setError('No se pudo cargar la lista. Inténtalo otra vez.');
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => { load(); }, [load]);

  const release = async (d: KeptDocument) => {
    setError('');
    try {
      await api.unkeepDocument(d.id);
      setData(x => x ? { ...x, documents: x.documents.filter(y => y.id !== d.id), kept_bytes: Math.max(0, x.kept_bytes - d.size_bytes) } : x);
    } catch {
      setError('No se pudo dejar de guardar el PDF.');
    }
  };

  if (loading) return <div className="h-28 rounded-[28px] bg-surface-container-low animate-pulse" />;
  const docs = data?.documents || [];
  return (
    <div className="space-y-4">
      {error && <Notice>{error}</Notice>}
      {data && (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <DataBlock label="PDF guardados" value={`${docs.length} · ${formatSize(data.kept_bytes)}`} empty="" />
          <DataBlock label="Folio en total" value={formatSize(data.total_bytes)} empty="" />
        </div>
      )}
      {docs.length === 0 ? (
        <EmptyState icon="inbox" text="Aún no has guardado PDF." />
      ) : (
        <div className="stagger grid grid-cols-1 xl:grid-cols-2 gap-4 items-start">
          {docs.map(d => (
            <div key={d.id} className="bg-surface-container-low rounded-[28px] p-5 space-y-3">
              <div className="flex flex-wrap gap-2">
                {d.saved && <span className="tag tag-ok !text-sm !px-3">Fondo guardado</span>}
                <span className="tag !text-sm !px-3 bg-surface-container-highest text-on-surface">{formatSize(d.size_bytes)}</span>
              </div>
              <h3 className="text-xl font-extrabold leading-tight tracking-tight break-words">{d.title || d.reference || 'Sin título'}</h3>
              {d.agency && <p className="text-sm font-bold text-primary">{d.agency}</p>}
              <p className="text-sm font-semibold text-on-surface-variant">{formatDate(d.created_at)}</p>
              <div className="flex flex-wrap gap-2">
                <button type="button" onClick={() => openExternal(d.file_url)} className="btn btn-tonal btn-sm">
                  <Icon name="open" className="w-5 h-5" />Abrir
                </button>
                <button type="button" onClick={() => openExternal(`${d.file_url}?download=1`)} className="btn btn-soft btn-sm">
                  <Icon name="save" className="w-5 h-5" />Descargar
                </button>
                <button type="button" onClick={() => release(d)} className="btn btn-text btn-sm">
                  <Icon name="pin" className="w-5 h-5" />Dejar de guardar
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

/** Un dato grande de la tarjeta; vacío = texto tenue, nunca inventado. */
function DataBlock({ label, value, empty, quote, tone }: { label: string; value?: string; empty: string; quote?: string; tone?: 'accent' }) {
  const has = !!(value && value.trim());
  return (
    <div className={`rounded-[20px] px-4 py-3 ${tone === 'accent' ? 'bg-tertiary-container text-on-tertiary-container' : 'bg-surface-container-high'}`}>
      <div className="text-xs font-bold opacity-80">{label}</div>
      <div className={`text-lg font-extrabold leading-tight break-words ${has ? '' : 'opacity-50 font-semibold'}`}>{has ? value : empty}</div>
      {quote && <p className="text-xs font-medium mt-2 opacity-80 break-words">«{quote}»</p>}
    </div>
  );
}

function OpportunityCard({ opp, onSave, onUnsave, onDiscard, onSaved }: {
  opp: Opportunity;
  onSave: (o: Opportunity) => void;
  onUnsave: (o: Opportunity) => void;
  onDiscard: (o: Opportunity) => void;
  onSaved: () => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const [editing, setEditing] = useState(false);
  const [pageContent, setPageContent] = useState<string | null>(null);
  const [loadingContent, setLoadingContent] = useState(false);
  const n = daysLeft(opp.deadline);
  const isClosing = n !== null && n >= 0 && n <= 14;
  const ev = opp.evidence || {};
  const scanned = !!opp.document_scanned;
  const hasDoc = !!opp.document_url;
  const [docLocal, setDocLocal] = useState(!!opp.document_local);
  const [docKept, setDocKept] = useState(!!opp.document_kept);
  const [docBusy, setDocBusy] = useState(false);
  const [docError, setDocError] = useState('');
  const localUrl = `/api/opportunities/${opp.id}/document`;
  const docHref = docLocal ? localUrl : opp.document_url!;

  const toggleKeep = async () => {
    setDocBusy(true);
    setDocError('');
    try {
      if (docKept) {
        await api.unkeepOpportunityDocument(opp.id);
        setDocKept(false);
      } else {
        await api.keepOpportunityDocument(opp.id);
        setDocKept(true);
        setDocLocal(true);
      }
    } catch {
      setDocError(docKept ? 'No se pudo dejar de guardar el PDF.' : 'No se pudo guardar el PDF. Ábrelo en el sitio.');
    } finally {
      setDocBusy(false);
    }
  };

  const loadPageContent = async () => {
    if (pageContent || !opp.page_id) return;
    setLoadingContent(true);
    try {
      const res = await api.getCrawlerPage(opp.page_id);
      setPageContent(res.page?.clean_text || 'Sin contenido disponible');
    } catch {
      setPageContent('Error cargando contenido');
    } finally {
      setLoadingContent(false);
    }
  };

  const activate = () => {
    if (scanned && !opp.reviewed) { setEditing(true); return; }
    const next = !expanded;
    setExpanded(next);
    if (next) loadPageContent();
  };

  return (
    <div className={`bg-surface-container-low transition-[border-radius,background-color] duration-500 ease-[var(--spring)] ${expanded ? 'rounded-[32px]' : 'rounded-[28px] hover:rounded-[36px] hover:bg-surface-container'} ${isClosing && n !== null && n <= 3 ? 'ring-[3px] ring-tertiary' : ''}`}>
      <div
        className="p-5 pb-3 cursor-pointer rounded-[inherit]"
        role="button"
        tabIndex={0}
        aria-expanded={expanded}
        onClick={activate}
        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); activate(); } }}
      >
        <div className="flex flex-wrap gap-2 mb-3">
          <span className="tag tag-primary !text-sm !px-3">{CATEGORY_LABELS[opp.category] || opp.category}</span>
          {opp.level && <span className="tag !text-sm !px-3 bg-surface-container-highest text-on-surface">{LEVEL_LABELS[opp.level] || opp.level}</span>}
          {opp.source_kind === 'pdf' && <span className="tag !text-sm !px-3 bg-surface-container-highest text-on-surface">PDF</span>}
          {opp.reviewed && <span className="tag tag-ok !text-sm !px-3">Guardado</span>}
          {docKept && <span className="tag tag-ok !text-sm !px-3">PDF guardado</span>}
        </div>

        <h3 className="text-xl font-extrabold leading-tight tracking-tight">{opp.title || 'Sin título'}</h3>
        {expanded && ev.title && <p className="text-xs font-medium mt-1 text-on-surface-variant break-words">«{ev.title}»</p>}
        {opp.agency && <p className="text-sm font-bold text-primary mt-1">{opp.agency}</p>}

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 mt-4">
          <DataBlock label="Referencia" value={opp.reference} empty="Sin referencia" quote={expanded ? ev.reference : undefined} />
          <DataBlock label="Monto" value={opp.amount} empty="Sin monto" quote={expanded ? ev.amount : undefined} />
          <div>
            <DataBlock label="Fecha límite" value={parseDay(opp.deadline) ? formatDate(opp.deadline) : ''} empty="Sin fecha" quote={expanded ? ev.deadline : undefined} tone={isClosing ? 'accent' : undefined} />
            {n !== null && <div className="text-sm font-extrabold mt-1 px-2">{leftLabel(n)}</div>}
          </div>
        </div>

        <div className="flex justify-center mt-2 text-on-surface-variant" aria-hidden="true">
          <svg className={`w-6 h-6 transition-transform duration-500 ease-[var(--spring)] ${expanded ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
          </svg>
        </div>
      </div>

      <div className="px-5 pb-5 space-y-3">
        {scanned && (
          <Notice kind="accent">PDF escaneado: léelo para completar los datos</Notice>
        )}
        <div className="flex flex-wrap gap-2">
          {!opp.reviewed ? (
            <button type="button" onClick={() => onSave(opp)} className="btn btn-filled flex-1 min-w-[140px]">
              <Icon name="save" className="w-5 h-5" />Guardar
            </button>
          ) : (
            <button type="button" onClick={() => onUnsave(opp)} className="btn btn-tonal">
              <Icon name="undo" className="w-5 h-5" />Quitar de guardados
            </button>
          )}
          <button type="button" onClick={() => onDiscard(opp)} className={`btn btn-danger ${opp.reviewed ? '' : 'flex-1 min-w-[140px]'}`}>
            <Icon name="trash" className="w-5 h-5" />Descartar
          </button>
          <button type="button" onClick={() => setEditing(true)} className="btn btn-text">
            <Icon name="note" className="w-5 h-5" />Corregir
          </button>
        </div>
        {hasDoc && (
          <div className="flex flex-wrap gap-2">
            <button type="button" onClick={() => openExternal(docHref)} className="btn btn-tonal btn-sm">
              <Icon name="open" className="w-5 h-5" />{docLocal ? 'Abrir PDF' : 'Abrir en el sitio'}
            </button>
            {docLocal && (
              <button type="button" onClick={() => openExternal(`${localUrl}?download=1`)} className="btn btn-soft btn-sm">
                <Icon name="save" className="w-5 h-5" />Descargar PDF
              </button>
            )}
            <button type="button" disabled={docBusy} onClick={toggleKeep} className="btn btn-soft btn-sm">
              <Icon name="pin" className="w-5 h-5" />{docKept ? 'Dejar de guardar PDF' : 'Guardar PDF'}
            </button>
          </div>
        )}
        {docError && <Notice>{docError}</Notice>}
      </div>

      {expanded && (
        <div className="px-5 pb-5 space-y-4 animate-fade-in">
          {opp.summary && <p className="text-sm text-on-surface leading-relaxed">{opp.summary}</p>}

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-sm">
            {opp.contact_name && (
              <div><span className="font-bold text-on-surface-variant">Contacto:</span> {opp.contact_name}</div>
            )}
            {opp.contact_phone && (
              <div><span className="font-bold text-on-surface-variant">Teléfono:</span> {opp.contact_phone}</div>
            )}
            {opp.contact_email && (
              <div>
                <span className="font-bold text-on-surface-variant">Correo:</span>{' '}
                <button type="button" onClick={() => openExternal(`mailto:${opp.contact_email}`)} className="text-primary font-semibold underline !min-h-0 !min-w-0 break-all">
                  {opp.contact_email}
                </button>
              </div>
            )}
          </div>

          <div className="flex flex-wrap gap-2">
            {opp.source_url && (
              <button type="button" onClick={() => openExternal(opp.source_url)} className="btn btn-soft btn-sm">
                <Icon name="link" className="w-5 h-5" />Ver fuente
              </button>
            )}
          </div>

          <div className="rounded-[24px] bg-surface-container p-4">
            <div className="title-md mb-2">Texto de la página</div>
            <div className="max-h-80 overflow-y-auto">
              {loadingContent ? (
                <div className="text-sm text-on-surface-variant py-4 text-center">Cargando...</div>
              ) : pageContent ? (
                (() => {
                  const cleaned = cleanPageContent(pageContent);
                  return cleaned ? (
                    <div className="text-sm text-on-surface-variant leading-relaxed whitespace-pre-wrap break-words">
                      {cleaned.slice(0, 5000)}
                      {cleaned.length > 5000 && <span className="block mt-2 font-semibold">... texto recortado</span>}
                    </div>
                  ) : (
                    <div className="text-sm text-on-surface-variant py-4 text-center">No hay texto limpio.</div>
                  );
                })()
              ) : (
                <div className="text-sm text-on-surface-variant py-4 text-center">Sin texto disponible</div>
              )}
            </div>
          </div>
        </div>
      )}

      {!expanded && opp.source_url && (
        <div className="px-5 pb-5 -mt-2">
          <button type="button" onClick={() => openExternal(opp.source_url)} className="btn btn-soft btn-sm">
            <Icon name="link" className="w-5 h-5" />Ver fuente
          </button>
        </div>
      )}

      {editing && <EditModal opp={opp} onClose={() => setEditing(false)} onSaved={(saved) => { setEditing(false); if (saved) onSave(opp); else onSaved(); }} />}
    </div>
  );
}

function EditModal({ opp, onClose, onSaved }: { opp: Opportunity; onClose: () => void; onSaved: (saved: boolean) => void }) {
  const d = parseDay(opp.deadline);
  const iso = d ? `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}` : '';
  const [title, setTitle] = useState(opp.title || '');
  const [deadline, setDeadline] = useState(iso);
  const [amount, setAmount] = useState(opp.amount || '');
  const [reference, setReference] = useState(opp.reference || '');
  const [contactName, setContactName] = useState(opp.contact_name || '');
  const [contactPhone, setContactPhone] = useState(opp.contact_phone || '');
  const [contactEmail, setContactEmail] = useState(opp.contact_email || '');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const save = async (andSave: boolean) => {
    setSaving(true);
    setError('');
    try {
      await api.updateOpportunity(opp.id, {
        title: title.trim(),
        deadline: deadline || '',
        amount: amount.trim(),
        reference: reference.trim(),
        contact_name: contactName.trim(),
        contact_phone: contactPhone.trim(),
        contact_email: contactEmail.trim(),
      } as Partial<Opportunity>);
      onSaved(andSave);
    } catch (e) {
      console.error('Update opp error', e);
      setError('No se pudo guardar. Inténtalo otra vez.');
      setSaving(false);
    }
  };

  return (
    <Modal title="Corregir datos" onClose={onClose}>
      <form onSubmit={e => { e.preventDefault(); save(false); }} className="space-y-3">
        {opp.document_url && (
          <button type="button" onClick={() => openExternal(opp.document_url!)} className="btn btn-tonal btn-sm">
            <Icon name="open" className="w-5 h-5" />Abrir PDF
          </button>
        )}
        <div>
          <label htmlFor="ed-title" className="field-label">Título</label>
          <input id="ed-title" value={title} onChange={e => setTitle(e.target.value)} className="field" />
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div>
            <label htmlFor="ed-date" className="field-label">Fecha límite</label>
            <input id="ed-date" type="date" value={deadline} onChange={e => setDeadline(e.target.value)} className="field" />
          </div>
          <div>
            <label htmlFor="ed-amount" className="field-label">Monto</label>
            <input id="ed-amount" value={amount} onChange={e => setAmount(e.target.value)} placeholder="$50,000" className="field" />
          </div>
        </div>
        <div>
          <label htmlFor="ed-ref" className="field-label">Referencia</label>
          <input id="ed-ref" value={reference} onChange={e => setReference(e.target.value)} className="field" />
        </div>
        <div>
          <label htmlFor="ed-cn" className="field-label">Contacto</label>
          <input id="ed-cn" value={contactName} onChange={e => setContactName(e.target.value)} className="field" />
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div>
            <label htmlFor="ed-ph" className="field-label">Teléfono</label>
            <input id="ed-ph" type="tel" value={contactPhone} onChange={e => setContactPhone(e.target.value)} className="field" />
          </div>
          <div>
            <label htmlFor="ed-em" className="field-label">Correo</label>
            <input id="ed-em" type="email" value={contactEmail} onChange={e => setContactEmail(e.target.value)} className="field" />
          </div>
        </div>
        {error && <Notice>{error}</Notice>}
        <div className="flex flex-wrap gap-2 pt-1">
          <button type="submit" disabled={saving} className="btn btn-filled">{saving ? 'Guardando...' : 'Guardar cambios'}</button>
          {!opp.reviewed && (
            <button type="button" disabled={saving} onClick={() => save(true)} className="btn btn-tonal">Guardar cambios y el fondo</button>
          )}
          <button type="button" onClick={onClose} className="btn btn-text">Cancelar</button>
        </div>
      </form>
    </Modal>
  );
}

function openExternal(url: string) {
  if (url.startsWith('mailto:')) {
    window.location.href = url;
  } else {
    window.open(url, '_blank', 'noopener');
  }
}

// Strip CSS/JS garbage and navigation boilerplate from page text for display
function cleanPageContent(text: string): string {
  if (!text) return '';

  // Phase 1: Remove CSS/JS patterns
  let clean = text
    .replace(/[\w-]+\s*:\s*[^;\n]{5,};/g, '')    // CSS property: value;
    .replace(/@[\w-]+[^{]*\{[^}]*\}/g, '')        // @media, @import blocks
    .replace(/\.-[\w-]+/g, '')                      // CSS class selectors
    .replace(/--[\w-]+:[^;]+;/g, '')                // CSS custom properties
    .replace(/\{[^}]{10,}\}/g, '')                  // CSS blocks { ... }
    .replace(/\b(rgba?|hsla?)\([^)]+\)/g, '')       // color functions
    .replace(/\b\d+(\.\d+)?(px|rem|em|vh|vw|%)\b/g, '') // CSS units
    .replace(/!important/gi, '')
    .replace(/url\([^)]*\)/g, '')                   // url(...)
    .replace(/(\.is-layout-\w+|\.wp-[\w-]+|\.has-[\w-]+)[^{\n]*/g, '') // WP classes
    // JS patterns
    .replace(/\bga\s*\([^)]*\)\s*;?/g, '')          // Google Analytics
    .replace(/\bgtag\s*\([^)]*\)\s*;?/g, '')        // gtag
    .replace(/\bfunction\s+\w+\s*\([^)]*\)\s*\{/g, '') // function declarations
    .replace(/\b(var|let|const)\s+\w+\s*=/g, '')    // variable declarations
    .replace(/window\.\w+\s*=\s*[^;\n]+;?/g, '')    // window.x = ...
    .replace(/document\.\w+/g, '')                   // document.xxx
    .replace(/\btry\s*\{|\bcatch\s*\([^)]*\)\s*\{/g, '') // try/catch
    .trim();

  // Phase 2: Filter out boilerplate lines
  const boilerplatePatterns = [
    /^(skip to|ir al|saltar al)/i,
    /^(cookie|accept all|aceptar)/i,
    /^(search|buscar|formulario de)/i,
    /^(toggle|menu|nav)/i,
    /^(copyright|all rights|todos los derechos|\u00a9)/i,
    /^(share|compartir|follow us|s[ií]guenos)/i,
    /^(back to top|volver|read more|leer m[aá]s)/i,
    /^(powered by|built with)/i,
    /^(home|inicio|contact|about)\s*$/i,
    /^(facebook|twitter|instagram|linkedin|youtube|whatsapp|telegram)\s*$/i,
    /^(tel|fax|email|phone|correo)\s*:/i,
    /^(select|choose|escoge|selecciona)\s+(language|idioma|a page)/i,
  ];

  const lines = clean.split('\n').filter(l => {
    const trimmed = l.trim();
    if (!trimmed || trimmed.length < 4) return false;
    // Skip CSS/code
    if (trimmed.includes('{') || trimmed.includes('}')) return false;
    if (/^[.#@]/.test(trimmed) && !trimmed.startsWith('...')) return false;
    if (/:\s*(linear-gradient|rgb|#[0-9a-f]{3,})/i.test(trimmed)) return false;
    if (/^(margin|padding|border|display|position|font|color|background|width|height|overflow|flex)/i.test(trimmed)) return false;
    // Skip JS
    if (/^(var |let |const |function |if\s*\(|for\s*\(|}\s*$|{\s*$)/i.test(trimmed)) return false;
    // Skip boilerplate
    for (const pat of boilerplatePatterns) {
      if (pat.test(trimmed)) return false;
    }
    // Skip very short lines that look like nav items (1-2 words, under 25 chars)
    if (trimmed.length < 25 && trimmed.split(/\s+/).length <= 2 && !/[.!?]$/.test(trimmed)) {
      // Allow if it looks like a header (capitalized, contains numbers, or is a title)
      if (!/[A-ZÁÉÍÓÚÑ]/.test(trimmed[0]) || trimmed === trimmed.toUpperCase()) return false;
    }
    return true;
  });

  return lines.join('\n').replace(/\n{3,}/g, '\n\n').trim();
}
// ── Páginas vigiladas ───────────────────────────────────────────────────────

function DominiosTab({ domains, onDomainsChanged }: {
  domains: CrawlDomain[];
  onDomainsChanged: () => void;
}) {
  const [runs, setRuns] = useState<CrawlRun[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);

  useEffect(() => {
    api.getCrawlerRuns(5).then(r => setRuns(r.runs || [])).catch(() => {});
  }, []);

  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);

  const handleToggle = async (id: string, active: boolean) => {
    try {
      await api.toggleCrawlerDomain(id, !active);
      onDomainsChanged();
    } catch (err) {
      console.error('Toggle domain error', err);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.deleteCrawlerDomain(id);
      setConfirmDeleteId(null);
      onDomainsChanged();
    } catch (err) {
      console.error('Delete domain error', err);
    }
  };

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="title-lg">Páginas vigiladas ({domains.length})</h2>
        <button type="button" onClick={() => setShowForm(!showForm)} className={showForm ? 'btn btn-soft' : 'btn btn-tonal'}>
          {showForm ? 'Cancelar' : (<><Icon name="plus" className="w-5 h-5" />Agregar página</>)}
        </button>
      </div>

      {showForm && <DomainForm onCreated={() => { setShowForm(false); onDomainsChanged(); }} />}

      {domains.length === 0 ? (
        <EmptyState icon="globe" text="Aún no hay páginas. Agrega una para empezar." />
      ) : (
        <div className="seg-list">
          {domains.map(d => (
            editingId === d.id ? (
              <DomainEditRow key={d.id} domain={d} onSaved={() => { setEditingId(null); onDomainsChanged(); }} onCancel={() => setEditingId(null)} />
            ) : (
              <div key={d.id} className="p-4 flex flex-wrap items-center gap-3">
                <span className={`w-3 h-3 rounded-full shrink-0 ${d.active ? 'bg-success' : 'bg-outline'}`} title={d.active ? 'Activa' : 'En pausa'} />
                <div className="flex-1 min-w-[180px]">
                  <div className="title-md">{d.label || d.domain}</div>
                  <div className="text-sm text-on-surface-variant">
                    {d.domain} &middot; {d.level ? LEVEL_LABELS[d.level] : d.category} &middot; {d.page_count} páginas
                    {d.last_crawled_at && ` · ${timeAgo(d.last_crawled_at)}`}
                  </div>
                </div>
                <div className="flex flex-wrap gap-2 shrink-0">
                  <button type="button" onClick={(e) => { e.stopPropagation(); handleToggle(d.id, d.active); }} className="btn btn-soft btn-sm">
                    {d.active ? 'Pausar' : 'Activar'}
                  </button>
                  <button type="button" onClick={(e) => { e.stopPropagation(); setEditingId(d.id); }} className="btn btn-soft btn-sm">Editar</button>
                  {confirmDeleteId === d.id ? (
                    <>
                      <button type="button" onClick={(e) => { e.stopPropagation(); handleDelete(d.id); }} className="btn btn-filled btn-sm !bg-error !text-on-error">Sí, quitar</button>
                      <button type="button" onClick={(e) => { e.stopPropagation(); setConfirmDeleteId(null); }} className="btn btn-soft btn-sm">No</button>
                    </>
                  ) : (
                    <button type="button" onClick={(e) => { e.stopPropagation(); setConfirmDeleteId(d.id); }} className="btn btn-danger btn-sm">Quitar</button>
                  )}
                </div>
              </div>
            )
          ))}
        </div>
      )}

      {runs.length > 0 && (
        <div>
          <h3 className="title-md mb-2">Búsquedas recientes</h3>
          <div className="seg-list">
            {runs.slice(0, 5).map(run => (
              <div key={run.id} className="flex items-center gap-3 px-4 py-3 text-sm">
                <span className={`w-2.5 h-2.5 rounded-full ${run.status === 'completed' ? 'bg-success' : run.status === 'running' ? 'bg-primary animate-pulse' : 'bg-error'}`} />
                <span className="font-semibold">{run.pages_crawled} páginas, {run.pages_new} nuevas</span>
                <span className="ml-auto text-on-surface-variant">{timeAgo(run.started_at)}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function DomainForm({ onCreated }: { onCreated: () => void }) {
  const [domain, setDomain] = useState('');
  const [label, setLabel] = useState('');
  const [category, setCategory] = useState('government');
  const [maxDepth, setMaxDepth] = useState(3);
  const [recrawlHours, setRecrawlHours] = useState(168);
  const [priority, setPriority] = useState(5);
  const [level, setLevel] = useState<OpportunityLevel>('estatal');
  const [saving, setSaving] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!domain.trim()) return;
    setSaving(true);
    try {
      await api.createCrawlerDomain({ domain: domain.trim(), label, category, level, max_depth: maxDepth, recrawl_hours: recrawlHours, priority });
      onCreated();
    } catch (err) {
      console.error('Create domain error', err);
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="card space-y-4">
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label htmlFor="df-dom" className="field-label">Dirección de la página *</label>
          <input id="df-dom" value={domain} onChange={e => setDomain(e.target.value)} placeholder="www.ejemplo.gov" className="field" />
        </div>
        <div>
          <label htmlFor="df-label" className="field-label">Nombre</label>
          <input id="df-label" value={label} onChange={e => setLabel(e.target.value)} placeholder="Agencia X" className="field" />
        </div>
        <div>
          <label htmlFor="df-cat" className="field-label">Tipo</label>
          <select id="df-cat" value={category} onChange={e => setCategory(e.target.value)} className="field font-semibold">
            {DOMAIN_CATEGORY_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </div>
        <div>
          <label htmlFor="df-pri" className="field-label">Prioridad (1 a 10)</label>
          <input id="df-pri" type="number" min={1} max={10} value={priority} onChange={e => setPriority(+e.target.value)} className="field" />
        </div>
        <div className="sm:col-span-2">
          <div className="field-label">Nivel</div>
          <LevelChips value={level} onChange={setLevel} />
        </div>
      </div>
      <button type="submit" disabled={saving} className="btn btn-filled">{saving ? 'Guardando...' : 'Guardar página'}</button>
    </form>
  );
}

function DomainEditRow({ domain, onSaved, onCancel }: { domain: CrawlDomain; onSaved: () => void; onCancel: () => void }) {
  const [label, setLabel] = useState(domain.label);
  const [category, setCategory] = useState(domain.category);
  const [maxDepth, setMaxDepth] = useState(domain.max_depth);
  const [recrawlHours, setRecrawlHours] = useState(domain.recrawl_hours);
  const [priority, setPriority] = useState(domain.priority);
  const [level, setLevel] = useState<OpportunityLevel>(domain.level || 'estatal');
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    try {
      await api.updateCrawlerDomain(domain.id, { label, category, level, max_depth: maxDepth, recrawl_hours: recrawlHours, priority });
      onSaved();
    } catch (err) {
      console.error('Update domain error', err);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="p-4 space-y-3 !bg-primary-container text-on-primary-container">
      <div className="title-md">{domain.domain}</div>
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <input value={label} onChange={e => setLabel(e.target.value)} placeholder="Nombre" aria-label="Nombre" className="field" />
        <select value={category} onChange={e => setCategory(e.target.value)} aria-label="Tipo" className="field font-semibold">
          {DOMAIN_CATEGORY_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
        </select>
        <input type="number" value={priority} onChange={e => setPriority(+e.target.value)} min={1} max={10} aria-label="Prioridad" className="field" />
      </div>
      <LevelChips value={level} onChange={setLevel} />
      <div className="flex gap-2">
        <button type="button" onClick={handleSave} disabled={saving} className="btn btn-filled btn-sm">{saving ? 'Guardando...' : 'Guardar'}</button>
        <button type="button" onClick={onCancel} className="btn btn-soft btn-sm">Cancelar</button>
      </div>
    </div>
  );
}

function LevelChips({ value, onChange }: { value: OpportunityLevel; onChange: (v: OpportunityLevel) => void }) {
  return (
    <div className="chip-row" role="group" aria-label="Nivel">
      {LEVELS.map(l => (
        <button key={l.value} type="button" className="chip" aria-pressed={value === l.value} onClick={() => onChange(l.value)}>
          {l.label}
        </button>
      ))}
    </div>
  );
}
