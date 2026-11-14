import { useState, useEffect, useCallback, useRef } from 'react';
import { api, type Opportunity, type CrawlDomain, type CrawlRun, type CrawlerStatsResponse } from '../lib/api';

// ── Helpers ────────────────────────────────────────────────────────

function daysUntil(dateStr: string | null): string {
  if (!dateStr) return '';
  const deadline = new Date(dateStr).getTime();
  const now = Date.now();
  const days = Math.ceil((deadline - now) / (1000 * 60 * 60 * 24));
  if (days < 0) return `${Math.abs(days)}d pasado`;
  if (days === 0) return 'Hoy';
  if (days === 1) return 'Manana';
  return `${days} dias`;
}

function formatDate(dateStr: string | null): string {
  if (!dateStr) return 'N/A';
  return new Date(dateStr).toLocaleDateString('es-PR', { month: 'short', day: 'numeric', year: 'numeric' });
}

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
  return `${days}d`;
}

const CATEGORY_COLORS: Record<string, string> = {
  rfp: 'bg-indigo-100 text-indigo-800 dark:bg-indigo-900/40 dark:text-indigo-300',
  grant: 'bg-violet-100 text-violet-800 dark:bg-violet-900/40 dark:text-violet-300',
  funding: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300',
  proposal: 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300',
  contract: 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300',
  fellowship: 'bg-pink-100 text-pink-800 dark:bg-pink-900/40 dark:text-pink-300',
  scholarship: 'bg-cyan-100 text-cyan-800 dark:bg-cyan-900/40 dark:text-cyan-300',
  other: 'bg-zinc-100 text-zinc-800 dark:bg-zinc-800 dark:text-zinc-300',
};

const STATUS_COLORS: Record<string, string> = {
  open: 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300',
  closing_soon: 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300',
  closed: 'bg-zinc-200 text-zinc-600 dark:bg-zinc-700 dark:text-zinc-400',
  unknown: 'bg-zinc-100 text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400',
};

const STATUS_LABELS: Record<string, string> = {
  open: 'Abierta', closing_soon: 'Pronto Cierra', closed: 'Cerrada', unknown: 'Sin Fecha',
};

const CATEGORY_LABELS: Record<string, string> = {
  rfp: 'RFP', grant: 'Grant', funding: 'Fondos', proposal: 'Propuesta',
  contract: 'Contrato', fellowship: 'Fellowship', scholarship: 'Beca', other: 'Otro',
};

const DOMAIN_CATEGORY_OPTIONS = [
  { value: 'government', label: 'Gobierno' },
  { value: 'legislature', label: 'Legislatura' },
  { value: 'executive', label: 'Ejecutivo' },
  { value: 'municipal', label: 'Municipal' },
  { value: 'news', label: 'Noticias' },
  { value: 'federal', label: 'Federal' },
  { value: 'other', label: 'Otro' },
];

type Tab = 'oportunidades' | 'dominios';

// Process phases for the single-button flow
type ProcessPhase = 'idle' | 'crawling' | 'done';

// ── Main Component ─────────────────────────────────────────────────
// All process state is here so tab switching never kills a running scan.

export default function CrawlerDesk() {
  const [activeTab, setActiveTab] = useState<Tab>('oportunidades');

  // ─ Global process state (survives tab switches) ─
  const [phase, setPhase] = useState<ProcessPhase>('idle');
  const [statusMsg, setStatusMsg] = useState('');
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const refreshCallbackRef = useRef<(() => void) | null>(null);

  // ─ Shared domain list (used by both tabs) ─
  const [domains, setDomains] = useState<CrawlDomain[]>([]);
  const [domainsLoaded, setDomainsLoaded] = useState(false);

  const loadDomains = useCallback(async () => {
    try {
      const res = await api.getCrawlerDomains();
      setDomains(res.domains || []);
      setDomainsLoaded(true);
    } catch (e) { console.error(e); }
  }, []);

  useEffect(() => { loadDomains(); }, [loadDomains]);

  // Cleanup on unmount
  useEffect(() => {
    return () => { if (pollRef.current) clearInterval(pollRef.current); };
  }, []);

  // ─ Single "Escanear" action: dispara el crawl y refresca ─
  const handleFullScan = useCallback(async () => {
    if (phase !== 'idle') return;

    setPhase('crawling');
    setStatusMsg('Rastreando las páginas de los fondos...');

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
          ? `Escaneo completo. ${prevTotal.current} fondos.`
          : 'Escaneo completo.');
        refreshCallbackRef.current?.();
        setTimeout(() => { setPhase('idle'); setStatusMsg(''); }, 8000);
      }
    }, 5000);
  }, [phase]);

  const isBusy = phase !== 'idle' && phase !== 'done';

  return (
    <div className="space-y-3">

      {/* ── Global Progress Bar ── visible on any tab */}
      {phase !== 'idle' && (
        <div className={`flex items-center gap-3 px-4 py-2.5 rounded-lg text-xs font-medium ${
          phase === 'done'
            ? 'bg-green-50 text-green-700 border border-green-200 dark:bg-green-900/20 dark:text-green-300 dark:border-green-800'
            : 'bg-violet-50 text-violet-700 border border-violet-200 dark:bg-violet-900/20 dark:text-violet-300 dark:border-violet-800'
        }`}>
          {isBusy && (
            <div className="relative w-4 h-4 shrink-0">
              <span className="absolute inset-0 border-2 border-violet-300 dark:border-violet-600 rounded-full" />
              <span className="absolute inset-0 border-2 border-transparent border-t-violet-600 dark:border-t-violet-300 rounded-full animate-spin" />
            </div>
          )}
          {phase === 'done' && (
            <svg className="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" /></svg>
          )}
          <span className="flex-1">{statusMsg}</span>
        </div>
      )}

      {/* ── Tab Switcher + Scan Button ── */}
      <div className="flex items-center justify-between border-b border-zinc-200 dark:border-zinc-700 pb-0">
        <div className="flex gap-1">
          {(['oportunidades', 'dominios'] as Tab[]).map(tab => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`px-4 py-2 text-sm font-medium rounded-t-lg transition-colors ${
                activeTab === tab
                  ? 'bg-violet-50 text-violet-700 border-b-2 border-violet-500 dark:bg-violet-900/20 dark:text-violet-300'
                  : 'text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200'
              }`}
            >
              {tab === 'oportunidades' ? 'Oportunidades' : 'Dominios'}
            </button>
          ))}
        </div>

        {/* THE button — always visible in the header */}
        <button
          onClick={handleFullScan}
          disabled={isBusy || domains.length === 0}
          className="flex items-center gap-2 text-xs bg-violet-600 text-white px-4 py-2 rounded-lg hover:bg-violet-700 disabled:opacity-40 transition-all font-medium shadow-sm mb-1"
          title={domains.length === 0 ? 'Agrega dominios primero' : 'Escanear agencias y extraer oportunidades'}
        >
          {isBusy ? (
            <>
              <span className="w-3 h-3 border-2 border-white/30 border-t-white rounded-full animate-spin" />
              {phase === 'crawling' ? 'Escaneando...' : 'Escanear'}
            </>
          ) : (
            <>
              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" /></svg>
              Escanear
            </>
          )}
        </button>
      </div>

      {/* ── Tab Content ── */}
      {activeTab === 'oportunidades' && (
        <OpportunityTab
          domains={domains}
          refreshCallbackRef={refreshCallbackRef}
          isBusy={isBusy}
          onScan={handleFullScan}
        />
      )}
      {activeTab === 'dominios' && (
        <DominiosTab
          domains={domains}
          onDomainsChanged={loadDomains}
        />
      )}
    </div>
  );
}

// ── Opportunity Tab ────────────────────────────────────────────────

function OpportunityTab({ domains, refreshCallbackRef, isBusy, onScan }: {
  domains: CrawlDomain[];
  refreshCallbackRef: React.MutableRefObject<(() => void) | null>;
  isBusy: boolean;
  onScan: () => void;
}) {
  const [opps, setOpps] = useState<Opportunity[]>([]);
  const [stats, setStats] = useState<{ total: number; by_status: Record<string, number> } | null>(null);
  const [loading, setLoading] = useState(true);
  const [statusFilter, setStatusFilter] = useState('');
  const [categoryFilter, setCategoryFilter] = useState('');
  const [domainFilter, setDomainFilter] = useState('');
  const [sortBy, setSortBy] = useState('created_at');
  const [searchQuery, setSearchQuery] = useState('');
  const [searchInput, setSearchInput] = useState('');

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [oppRes, statsRes] = await Promise.all([
        searchQuery
          ? api.searchOpportunities(searchQuery, 100)
          : api.getOpportunities({
              status: statusFilter || undefined,
              category: categoryFilter || undefined,
              domain_id: domainFilter || undefined,
              sort: sortBy,
              limit: 100,
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
  }, [statusFilter, categoryFilter, domainFilter, sortBy, searchQuery]);

  useEffect(() => { loadData(); }, [loadData]);

  // Register the refresh callback so parent can trigger refreshes
  useEffect(() => {
    refreshCallbackRef.current = loadData;
    return () => { refreshCallbackRef.current = null; };
  }, [loadData, refreshCallbackRef]);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setSearchQuery(searchInput);
  };

  const handleReview = async (id: string) => {
    try {
      await api.reviewOpportunity(id);
      loadData();
    } catch (err) { console.error('Review error', err); }
  };

  const handleDeleteOpp = async (id: string) => {
    try {
      await api.deleteOpportunity(id);
      loadData();
    } catch (err) { console.error('Delete opp error', err); }
  };

  const openCount = stats?.by_status?.open || 0;
  const closingSoonCount = stats?.by_status?.closing_soon || 0;
  const totalCount = stats?.total || 0;
  const hasDomains = domains.length > 0;

  // If no domains configured yet, show setup prompt
  if (!hasDomains) {
    return (
      <div className="text-center py-20">
        <svg className="w-16 h-16 mx-auto text-zinc-300 dark:text-zinc-600 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418" /></svg>
        <h3 className="text-base font-semibold text-zinc-600 dark:text-zinc-300">Configura tus agencias</h3>
        <p className="text-sm text-zinc-400 mt-1 max-w-sm mx-auto">
          Ve a la pestana <strong>Dominios</strong> para agregar los sitios web de agencias gubernamentales que quieres monitorear.
        </p>
      </div>
    );
  }

  return (
    <div className="flex gap-4">

      {/* ── Left: Domain Sidebar Filter ── */}
      <div className="w-44 shrink-0 space-y-1">
        <div className="text-[10px] font-semibold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider px-2 py-1">
          Agencias
        </div>
        <button
          onClick={() => { setDomainFilter(''); setSearchQuery(''); setSearchInput(''); }}
          className={`w-full text-left text-xs px-2 py-1.5 rounded-md transition-colors ${
            !domainFilter
              ? 'bg-violet-100 text-violet-700 font-medium dark:bg-violet-900/30 dark:text-violet-300'
              : 'text-zinc-600 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800'
          }`}
        >
          Todas ({totalCount})
        </button>
        {domains.filter(d => d.active).map(d => (
          <button
            key={d.id}
            onClick={() => { setDomainFilter(d.id); setSearchQuery(''); setSearchInput(''); }}
            className={`w-full text-left text-xs px-2 py-1.5 rounded-md transition-colors truncate ${
              domainFilter === d.id
                ? 'bg-violet-100 text-violet-700 font-medium dark:bg-violet-900/30 dark:text-violet-300'
                : 'text-zinc-600 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800'
            }`}
            title={d.domain}
          >
            <span className={`inline-block w-1.5 h-1.5 rounded-full mr-1.5 ${d.active ? 'bg-green-500' : 'bg-zinc-300'}`} />
            {d.label || d.domain}
          </button>
        ))}
      </div>

      {/* ── Right: Results ── */}
      <div className="flex-1 min-w-0 space-y-3">

        {/* Stats Cards */}
        {totalCount > 0 && (
          <div className="grid grid-cols-3 gap-2">
            <StatCard label="Abiertas" value={openCount} color="text-green-600 dark:text-green-400" />
            <StatCard label="Pronto Cierra" value={closingSoonCount} color="text-amber-600 dark:text-amber-400" />
            <StatCard label="Total" value={totalCount} color="text-violet-600 dark:text-violet-400" />
          </div>
        )}

        {/* Filters */}
        {totalCount > 0 && (
          <div className="flex flex-wrap gap-2 items-center">
            <div className="flex gap-1">
              {[
                { value: '', label: 'Todas' },
                { value: 'open', label: 'Abiertas' },
                { value: 'closing_soon', label: 'Pronto' },
                { value: 'closed', label: 'Cerradas' },
              ].map(s => (
                <button
                  key={s.value}
                  onClick={() => { setStatusFilter(s.value); setSearchQuery(''); setSearchInput(''); }}
                  className={`px-3 py-1 text-xs rounded-full transition-colors ${
                    statusFilter === s.value
                      ? 'bg-violet-500 text-white'
                      : 'bg-zinc-100 text-zinc-600 hover:bg-zinc-200 dark:bg-zinc-800 dark:text-zinc-300 dark:hover:bg-zinc-700'
                  }`}
                >
                  {s.label}
                </button>
              ))}
            </div>

            <select
              value={categoryFilter}
              onChange={e => setCategoryFilter(e.target.value)}
              className="text-xs bg-zinc-50 border border-zinc-200 rounded px-2 py-1 dark:bg-zinc-800 dark:border-zinc-700 dark:text-zinc-300"
            >
              <option value="">Categoria</option>
              {Object.entries(CATEGORY_LABELS).map(([k, v]) => (
                <option key={k} value={k}>{v}</option>
              ))}
            </select>

            <select
              value={sortBy}
              onChange={e => setSortBy(e.target.value)}
              className="text-xs bg-zinc-50 border border-zinc-200 rounded px-2 py-1 dark:bg-zinc-800 dark:border-zinc-700 dark:text-zinc-300"
            >
              <option value="created_at">Recientes</option>
              <option value="deadline">Fecha Limite</option>
              <option value="amount">Monto</option>
              <option value="confidence">Confianza</option>
            </select>

            <form onSubmit={handleSearch} className="flex gap-1 ml-auto">
              <input
                type="text"
                value={searchInput}
                onChange={e => setSearchInput(e.target.value)}
                placeholder="Buscar..."
                className="text-xs bg-zinc-50 border border-zinc-200 rounded px-2 py-1 w-36 dark:bg-zinc-800 dark:border-zinc-700 dark:text-zinc-300"
              />
              <button type="submit" className="text-xs bg-violet-500 text-white px-2 py-1 rounded hover:bg-violet-600">
                Ir
              </button>
            </form>
          </div>
        )}

        {/* Content */}
        {loading ? (
          <div className="text-center py-12 text-zinc-400 text-sm">Cargando...</div>
        ) : opps.length === 0 ? (
          <div className="text-center py-16">
            <svg className="w-10 h-10 mx-auto text-zinc-300 dark:text-zinc-600 mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
            <h3 className="text-sm font-semibold text-zinc-600 dark:text-zinc-300">No hay oportunidades aun</h3>
            <p className="text-xs text-zinc-400 mt-1 max-w-xs mx-auto">
              Presiona <strong>"Escanear"</strong> arriba para rastrear las agencias y detectar grants, RFPs y fondos automaticamente.
            </p>
            {!isBusy && (
              <button
                onClick={onScan}
                className="mt-4 text-xs bg-violet-600 text-white px-5 py-2 rounded-lg hover:bg-violet-700 font-medium"
              >
                Escanear Ahora
              </button>
            )}
          </div>
        ) : (
          <div className="space-y-2">
            {opps.map(opp => (
              <OpportunityCard key={opp.id} opp={opp} onReview={handleReview} onDelete={handleDeleteOpp} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function StatCard({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div className="bg-white dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700 rounded-lg p-3">
      <div className={`text-xl font-bold ${color}`}>{value}</div>
      <div className="text-[10px] text-zinc-500 dark:text-zinc-400">{label}</div>
    </div>
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

function OpportunityCard({ opp, onReview, onDelete }: { opp: Opportunity; onReview: (id: string) => void; onDelete: (id: string) => void }) {
  const [expanded, setExpanded] = useState(false);
  const [pageContent, setPageContent] = useState<string | null>(null);
  const [loadingContent, setLoadingContent] = useState(false);
  const deadlineDays = daysUntil(opp.deadline);
  const isClosing = opp.status === 'closing_soon';

  // Fetch full page content when expanded
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

  const handleExpand = () => {
    const next = !expanded;
    setExpanded(next);
    if (next) loadPageContent();
  };

  return (
    <div
      className={`bg-white dark:bg-zinc-800 border rounded-lg transition-all ${
        expanded
          ? 'border-violet-300 dark:border-violet-700 shadow-md'
          : 'border-zinc-200 dark:border-zinc-700 hover:border-violet-200 dark:hover:border-violet-800'
      }`}
    >
      {/* Clickable header */}
      <div className="p-4 cursor-pointer" onClick={handleExpand}>
        <div className="flex items-start gap-3">
          <div className="flex-1 min-w-0">
            {/* Badges */}
            <div className="flex flex-wrap gap-1.5 mb-2">
              <span className={`inline-flex items-center px-2 py-0.5 text-[10px] font-semibold rounded-full ${CATEGORY_COLORS[opp.category] || CATEGORY_COLORS.other}`}>
                {CATEGORY_LABELS[opp.category] || opp.category}
              </span>
              <span className={`inline-flex items-center px-2 py-0.5 text-[10px] font-semibold rounded-full ${STATUS_COLORS[opp.status] || STATUS_COLORS.unknown} ${isClosing ? 'animate-pulse' : ''}`}>
                {STATUS_LABELS[opp.status] || opp.status}
              </span>
              {opp.reviewed && (
                <span className="inline-flex items-center px-2 py-0.5 text-[10px] font-semibold rounded-full bg-green-50 text-green-700 dark:bg-green-900/30 dark:text-green-400">
                  Revisada
                </span>
              )}
              {opp.confidence > 0 && (
                <span className="inline-flex items-center px-2 py-0.5 text-[10px] rounded-full bg-zinc-100 text-zinc-500 dark:bg-zinc-700 dark:text-zinc-400">
                  {opp.confidence}%
                </span>
              )}
            </div>

            <h3 className="text-sm font-semibold text-zinc-900 dark:text-zinc-100 leading-tight">
              {opp.title || 'Sin titulo'}
            </h3>
            {opp.agency && (
              <p className="text-xs text-violet-600 dark:text-violet-400 mt-0.5">{opp.agency}</p>
            )}

            {!expanded && opp.summary && (
              <p className="text-xs text-zinc-600 dark:text-zinc-400 mt-1 line-clamp-1">{opp.summary}</p>
            )}

            <div className="flex flex-wrap gap-3 mt-2 text-xs text-zinc-500 dark:text-zinc-400">
              {opp.amount && (
                <span className="text-green-600 dark:text-green-400 font-medium">{opp.amount}</span>
              )}
              {opp.deadline && (
                <span className={isClosing ? 'text-red-600 dark:text-red-400 font-medium' : ''}>
                  {formatDate(opp.deadline)} ({deadlineDays})
                </span>
              )}
            </div>
          </div>

          <svg className={`w-4 h-4 text-zinc-400 shrink-0 transition-transform ${expanded ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </div>
      </div>

      {/* Expanded: full detail + page content reader */}
      {expanded && (
        <div className="border-t border-zinc-100 dark:border-zinc-700">
          {/* Info section */}
          <div className="px-4 py-3 space-y-3">
            {opp.summary && (
              <p className="text-xs text-zinc-600 dark:text-zinc-400 leading-relaxed">{opp.summary}</p>
            )}

            <div className="grid grid-cols-2 gap-2 text-xs">
              {opp.contact_name && (
                <div>
                  <span className="text-zinc-400">Contacto:</span>{' '}
                  <span className="text-zinc-700 dark:text-zinc-300">{opp.contact_name}</span>
                </div>
              )}
              {opp.contact_email && (
                <div>
                  <span className="text-zinc-400">Email:</span>{' '}
                  <button onClick={(e) => { e.stopPropagation(); openExternal(`mailto:${opp.contact_email}`); }}
                    className="text-violet-600 dark:text-violet-400 hover:underline">
                    {opp.contact_email}
                  </button>
                </div>
              )}
              {opp.source_url && (
                <div className="col-span-2">
                  <span className="text-zinc-400">Fuente:</span>{' '}
                  <button onClick={(e) => { e.stopPropagation(); openExternal(opp.source_url); }}
                    className="text-violet-600 dark:text-violet-400 hover:underline text-[11px] break-all">
                    {opp.source_url}
                  </button>
                </div>
              )}
            </div>

            {/* Action buttons */}
            <div className="flex gap-2">
              {opp.source_url && (
                <button
                  onClick={(e) => { e.stopPropagation(); openExternal(opp.source_url); }}
                  className="text-xs bg-violet-600 text-white px-3 py-1.5 rounded-lg hover:bg-violet-700 font-medium flex items-center gap-1"
                >
                  <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg>
                  Abrir en Navegador
                </button>
              )}
              {!opp.reviewed && (
                <button
                  onClick={(e) => { e.stopPropagation(); onReview(opp.id); }}
                  className="text-xs bg-green-600 text-white px-3 py-1.5 rounded-lg hover:bg-green-700 font-medium"
                >
                  Marcar Revisada
                </button>
              )}
              <button
                onClick={(e) => { e.stopPropagation(); onDelete(opp.id); }}
                className="text-xs bg-zinc-100 text-red-600 px-3 py-1.5 rounded-lg hover:bg-red-50 dark:bg-zinc-700 dark:text-red-400 dark:hover:bg-red-900/30 font-medium"
              >
                Eliminar
              </button>
            </div>
          </div>

          {/* Page content reader */}
          <div className="border-t border-zinc-100 dark:border-zinc-700 bg-zinc-50/50 dark:bg-zinc-900/30">
            <div className="px-4 py-2 flex items-center gap-2 text-[10px] font-semibold text-zinc-400 uppercase tracking-wider">
              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
              Contenido de la pagina
            </div>
            <div className="px-4 pb-4 max-h-80 overflow-y-auto">
              {loadingContent ? (
                <div className="text-xs text-zinc-400 py-4 text-center">Cargando contenido...</div>
              ) : pageContent ? (
                (() => {
                  const cleaned = cleanPageContent(pageContent);
                  return cleaned ? (
                    <div className="text-xs text-zinc-700 dark:text-zinc-300 leading-relaxed whitespace-pre-wrap break-words">
                      {cleaned.slice(0, 5000)}
                      {cleaned.length > 5000 && (
                        <span className="text-zinc-400 italic block mt-2">... contenido truncado</span>
                      )}
                    </div>
                  ) : (
                    <div className="text-xs text-zinc-400 py-4 text-center">
                      Contenido no disponible en texto limpio.
                      {opp.source_url && (
                        <button onClick={() => openExternal(opp.source_url)} className="ml-1 text-violet-500 underline">
                          Ver en navegador
                        </button>
                      )}
                    </div>
                  );
                })()
              ) : (
                <div className="text-xs text-zinc-400 py-4 text-center">Sin contenido disponible</div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Dominios Tab ───────────────────────────────────────────────────

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
    <div className="space-y-6">
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold text-zinc-700 dark:text-zinc-300">
            Agencias Configuradas ({domains.length})
          </h3>
          <button
            onClick={() => setShowForm(!showForm)}
            className="text-xs bg-violet-500 text-white px-3 py-1.5 rounded hover:bg-violet-600"
          >
            {showForm ? 'Cancelar' : '+ Agregar Dominio'}
          </button>
        </div>

        {showForm && (
          <DomainForm onCreated={() => { setShowForm(false); onDomainsChanged(); }} />
        )}

        {domains.length === 0 ? (
          <div className="text-center py-8 text-zinc-400 text-sm">No hay dominios. Agrega uno para comenzar.</div>
        ) : (
          <div className="space-y-2">
            {domains.map(d => (
              editingId === d.id ? (
                <DomainEditRow key={d.id} domain={d} onSaved={() => { setEditingId(null); onDomainsChanged(); }} onCancel={() => setEditingId(null)} />
              ) : (
                <div key={d.id} className="bg-white dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700 rounded-lg p-3 flex items-center gap-3">
                  <div className={`w-2 h-2 rounded-full shrink-0 ${d.active ? 'bg-green-500' : 'bg-zinc-300'}`} />
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium text-zinc-800 dark:text-zinc-200">{d.label || d.domain}</div>
                    <div className="text-xs text-zinc-500 dark:text-zinc-400">
                      {d.domain} &middot; {d.category} &middot; {d.page_count} pag
                      {d.last_crawled_at && ` \u00b7 ${timeAgo(d.last_crawled_at)}`}
                    </div>
                  </div>
                  <div className="flex gap-1 text-[10px] shrink-0">
                    <button
                      onClick={(e) => { e.stopPropagation(); handleToggle(d.id, d.active); }}
                      className="px-2 py-1 rounded bg-zinc-100 dark:bg-zinc-700 text-zinc-600 dark:text-zinc-300 hover:bg-zinc-200"
                    >
                      {d.active ? 'Pausar' : 'Activar'}
                    </button>
                    <button
                      onClick={(e) => { e.stopPropagation(); setEditingId(d.id); }}
                      className="px-2 py-1 rounded bg-zinc-100 dark:bg-zinc-700 text-zinc-600 dark:text-zinc-300 hover:bg-zinc-200"
                    >
                      Editar
                    </button>
                    {confirmDeleteId === d.id ? (
                      <>
                        <button
                          onClick={(e) => { e.stopPropagation(); handleDelete(d.id); }}
                          className="px-2 py-1 rounded bg-red-600 text-white hover:bg-red-700 font-semibold"
                        >
                          Confirmar
                        </button>
                        <button
                          onClick={(e) => { e.stopPropagation(); setConfirmDeleteId(null); }}
                          className="px-2 py-1 rounded bg-zinc-100 dark:bg-zinc-700 text-zinc-600 dark:text-zinc-300 hover:bg-zinc-200"
                        >
                          No
                        </button>
                      </>
                    ) : (
                      <button
                        onClick={(e) => { e.stopPropagation(); setConfirmDeleteId(d.id); }}
                        className="px-2 py-1 rounded bg-red-50 text-red-600 dark:bg-red-900/30 dark:text-red-400 hover:bg-red-100"
                      >
                        Eliminar
                      </button>
                    )}
                  </div>
                </div>
              )
            ))}
          </div>
        )}
      </div>

      {/* Recent Runs */}
      {runs.length > 0 && (
        <div className="border-t border-zinc-200 dark:border-zinc-700 pt-4">
          <h4 className="text-xs font-medium text-zinc-500 dark:text-zinc-400 mb-2">Ejecuciones Recientes</h4>
          <div className="space-y-1">
            {runs.slice(0, 5).map(run => (
              <div key={run.id} className="flex items-center gap-2 text-xs text-zinc-500 dark:text-zinc-400">
                <span className={`w-1.5 h-1.5 rounded-full ${
                  run.status === 'completed' ? 'bg-green-500' : run.status === 'running' ? 'bg-blue-500 animate-pulse' : 'bg-red-500'
                }`} />
                <span>{run.pages_crawled} pag, {run.pages_new} nuevas</span>
                <span className="text-zinc-400">{timeAgo(run.started_at)}</span>
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
  const [saving, setSaving] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!domain.trim()) return;
    setSaving(true);
    try {
      await api.createCrawlerDomain({ domain: domain.trim(), label, category, max_depth: maxDepth, recrawl_hours: recrawlHours, priority });
      onCreated();
    } catch (err) {
      console.error('Create domain error', err);
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="bg-zinc-50 dark:bg-zinc-800/50 border border-zinc-200 dark:border-zinc-700 rounded-lg p-4 space-y-3">
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="block text-[10px] text-zinc-500 mb-0.5">Dominio *</label>
          <input value={domain} onChange={e => setDomain(e.target.value)} placeholder="www.ejemplo.gov"
            className="w-full text-xs border border-zinc-200 dark:border-zinc-700 rounded px-2 py-1.5 bg-white dark:bg-zinc-800 dark:text-zinc-200" />
        </div>
        <div>
          <label className="block text-[10px] text-zinc-500 mb-0.5">Nombre</label>
          <input value={label} onChange={e => setLabel(e.target.value)} placeholder="Agencia X"
            className="w-full text-xs border border-zinc-200 dark:border-zinc-700 rounded px-2 py-1.5 bg-white dark:bg-zinc-800 dark:text-zinc-200" />
        </div>
        <div>
          <label className="block text-[10px] text-zinc-500 mb-0.5">Categoria</label>
          <select value={category} onChange={e => setCategory(e.target.value)}
            className="w-full text-xs border border-zinc-200 dark:border-zinc-700 rounded px-2 py-1.5 bg-white dark:bg-zinc-800 dark:text-zinc-200">
            {DOMAIN_CATEGORY_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </div>
        <div>
          <label className="block text-[10px] text-zinc-500 mb-0.5">Prioridad (1-10)</label>
          <input type="number" min={1} max={10} value={priority} onChange={e => setPriority(+e.target.value)}
            className="w-full text-xs border border-zinc-200 dark:border-zinc-700 rounded px-2 py-1.5 bg-white dark:bg-zinc-800 dark:text-zinc-200" />
        </div>
      </div>
      <button type="submit" disabled={saving}
        className="text-xs bg-violet-500 text-white px-4 py-1.5 rounded hover:bg-violet-600 disabled:opacity-50">
        {saving ? 'Guardando...' : 'Crear Dominio'}
      </button>
    </form>
  );
}

function DomainEditRow({ domain, onSaved, onCancel }: { domain: CrawlDomain; onSaved: () => void; onCancel: () => void }) {
  const [label, setLabel] = useState(domain.label);
  const [category, setCategory] = useState(domain.category);
  const [maxDepth, setMaxDepth] = useState(domain.max_depth);
  const [recrawlHours, setRecrawlHours] = useState(domain.recrawl_hours);
  const [priority, setPriority] = useState(domain.priority);
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    try {
      await api.updateCrawlerDomain(domain.id, { label, category, max_depth: maxDepth, recrawl_hours: recrawlHours, priority });
      onSaved();
    } catch (err) {
      console.error('Update domain error', err);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="bg-violet-50/50 dark:bg-violet-900/10 border border-violet-200 dark:border-violet-800 rounded-lg p-3 space-y-2">
      <div className="text-xs font-medium text-violet-700 dark:text-violet-300">{domain.domain}</div>
      <div className="grid grid-cols-3 gap-2">
        <input value={label} onChange={e => setLabel(e.target.value)} placeholder="Nombre"
          className="text-xs border border-zinc-200 dark:border-zinc-700 rounded px-2 py-1 bg-white dark:bg-zinc-800 dark:text-zinc-200" />
        <select value={category} onChange={e => setCategory(e.target.value)}
          className="text-xs border border-zinc-200 dark:border-zinc-700 rounded px-2 py-1 bg-white dark:bg-zinc-800 dark:text-zinc-200">
          {DOMAIN_CATEGORY_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
        </select>
        <input type="number" value={priority} onChange={e => setPriority(+e.target.value)} min={1} max={10}
          className="text-xs border border-zinc-200 dark:border-zinc-700 rounded px-2 py-1 bg-white dark:bg-zinc-800 dark:text-zinc-200" />
      </div>
      <div className="flex gap-2">
        <button onClick={handleSave} disabled={saving}
          className="text-[10px] bg-violet-500 text-white px-3 py-1 rounded hover:bg-violet-600 disabled:opacity-50">
          {saving ? 'Guardando...' : 'Guardar'}
        </button>
        <button onClick={onCancel}
          className="text-[10px] bg-zinc-200 text-zinc-600 px-3 py-1 rounded hover:bg-zinc-300 dark:bg-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-600">
          Cancelar
        </button>
      </div>
    </div>
  );
}
