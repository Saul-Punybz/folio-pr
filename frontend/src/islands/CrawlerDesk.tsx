import { useState, useEffect, useCallback, useRef } from 'react';
import { api, type Opportunity, type CrawlDomain, type CrawlRun, type CrawlerStatsResponse } from '../lib/api';
import { Icon, Spinner, EmptyState } from './ui';

// ── Helpers ────────────────────────────────────────────────────────

function daysUntil(dateStr: string | null): string {
  if (!dateStr) return '';
  const deadline = new Date(dateStr).getTime();
  const now = Date.now();
  const days = Math.ceil((deadline - now) / (1000 * 60 * 60 * 24));
  if (days < 0) return `hace ${Math.abs(days)} d`;
  if (days === 0) return 'Hoy';
  if (days === 1) return 'Mañana';
  return `en ${days} días`;
}

function formatDate(dateStr: string | null): string {
  if (!dateStr) return 'Sin fecha';
  return new Date(dateStr).toLocaleDateString('es-PR', { month: 'short', day: 'numeric', year: 'numeric' });
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

const CATEGORY_COLORS: Record<string, string> = {
  rfp: 'bg-primary-container text-on-primary-container',
  grant: 'bg-primary-container text-on-primary-container',
  funding: 'bg-primary-container text-on-primary-container',
  proposal: 'bg-primary-container text-on-primary-container',
  contract: 'bg-primary-container text-on-primary-container',
  fellowship: 'bg-primary-container text-on-primary-container',
  scholarship: 'bg-primary-container text-on-primary-container',
  other: 'bg-surface-container-highest text-on-surface',
};

const STATUS_COLORS: Record<string, string> = {
  open: 'bg-success-container text-on-success-container',
  closing_soon: 'bg-tertiary-container text-on-tertiary-container',
  closed: 'bg-surface-container-highest text-on-surface-variant',
  unknown: 'bg-surface-container-highest text-on-surface-variant',
};

const STATUS_LABELS: Record<string, string> = {
  open: 'Abierta', closing_soon: 'Cierra pronto', closed: 'Cerrada', unknown: 'Sin fecha',
};

const CATEGORY_LABELS: Record<string, string> = {
  rfp: 'RFP', grant: 'Subvención', funding: 'Fondos', proposal: 'Propuesta',
  contract: 'Contrato', fellowship: 'Programa', scholarship: 'Beca', other: 'Otro',
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
        {(['oportunidades', 'dominios'] as Tab[]).map(tab => (
          <button key={tab} type="button" role="tab" aria-selected={activeTab === tab} aria-pressed={activeTab === tab} onClick={() => setActiveTab(tab)} className="chip">
            {tab === 'oportunidades' ? 'Oportunidades' : 'Agencias'}
          </button>
        ))}
      </div>

      {activeTab === 'oportunidades' && (
        <OpportunityTab domains={domains} refreshCallbackRef={refreshCallbackRef} isBusy={isBusy} onScan={handleFullScan} />
      )}
      {activeTab === 'dominios' && (
        <DominiosTab domains={domains} onDomainsChanged={loadDomains} />
      )}

      {/* La acción principal de la pantalla */}
      <div className="fab-slot">
        <button
          type="button"
          onClick={handleFullScan}
          disabled={isBusy || domains.length === 0}
          className="fab disabled:opacity-50"
          title={domains.length === 0 ? 'Agrega agencias primero' : 'Buscar fondos nuevos'}
        >
          {isBusy ? <Spinner className="w-6 h-6" /> : <Icon name="search" className="w-6 h-6" />}
          {isBusy ? 'Buscando...' : 'Buscar fondos'}
        </button>
      </div>
    </div>
  );
}

// ── Oportunidades ──────────────────────────────────────────────────

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

  if (!hasDomains) {
    return (
      <EmptyState icon="globe" text="Agrega las páginas de las agencias que quieres vigilar.">
        <span className="text-sm font-semibold">Abre «Agencias» arriba.</span>
      </EmptyState>
    );
  }

  const resetFilters = () => { setStatusFilter(''); setCategoryFilter(''); setDomainFilter(''); setSearchQuery(''); setSearchInput(''); };

  return (
    <div className="space-y-5">
      {totalCount > 0 && (
        <div className="stagger grid grid-cols-3 gap-3">
          <StatCard label="Abiertas" value={openCount} tone="ok" />
          <StatCard label="Cierran pronto" value={closingSoonCount} tone="accent" />
          <StatCard label="Total" value={totalCount} tone="primary" />
        </div>
      )}

      {totalCount > 0 && (
        <div className="space-y-3">
          <div className="chip-row chip-scroll" role="group" aria-label="Estado">
            {[
              { value: '', label: 'Todas' },
              { value: 'open', label: 'Abiertas' },
              { value: 'closing_soon', label: 'Cierran pronto' },
              { value: 'closed', label: 'Cerradas' },
            ].map(s => (
              <button key={s.value} type="button" aria-pressed={statusFilter === s.value}
                onClick={() => { setStatusFilter(s.value); setSearchQuery(''); setSearchInput(''); }} className="chip">
                {s.label}
              </button>
            ))}
          </div>

          <div className="chip-row chip-scroll" role="group" aria-label="Agencia">
            <button type="button" aria-pressed={!domainFilter} onClick={() => { setDomainFilter(''); setSearchQuery(''); setSearchInput(''); }} className="chip">
              Todas las agencias
            </button>
            {domains.filter(d => d.active).map(d => (
              <button key={d.id} type="button" aria-pressed={domainFilter === d.id} title={d.domain}
                onClick={() => { setDomainFilter(d.id); setSearchQuery(''); setSearchInput(''); }} className="chip">
                {d.label || d.domain}
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
            <label className="sr-only" htmlFor="op-sort">Orden</label>
            <select id="op-sort" value={sortBy} onChange={e => setSortBy(e.target.value)} className="field !w-auto font-semibold text-sm">
              <option value="created_at">Más recientes</option>
              <option value="deadline">Fecha límite</option>
              <option value="amount">Monto</option>
              <option value="confidence">Más seguras</option>
            </select>
            <form onSubmit={handleSearch} className="flex gap-2 ml-auto min-w-[220px] flex-1 max-w-sm" role="search">
              <input type="search" value={searchInput} onChange={e => setSearchInput(e.target.value)} placeholder="Buscar fondos" aria-label="Buscar fondos" className="field" />
              <button type="submit" className="icon-btn bg-primary-container text-on-primary-container shrink-0" aria-label="Buscar"><Icon name="search" /></button>
            </form>
          </div>
        </div>
      )}

      {loading ? (
        <div className="space-y-2">
          {[0, 1, 2].map(i => <div key={i} className="h-28 rounded-[28px] bg-surface-container-low animate-pulse" />)}
        </div>
      ) : opps.length === 0 ? (
        totalCount === 0 ? (
          <EmptyState icon="search" text="Aún no hay fondos. Toca «Buscar fondos» para empezar.">
            {!isBusy && <button type="button" onClick={onScan} className="btn btn-filled">Buscar ahora</button>}
          </EmptyState>
        ) : (
          <EmptyState icon="search" text="Nada con esos filtros.">
            <button type="button" onClick={resetFilters} className="btn btn-tonal">Quitar filtros</button>
          </EmptyState>
        )
      ) : (
        <div className="stagger grid grid-cols-1 xl:grid-cols-2 gap-4 items-start">
          {opps.map(opp => (
            <OpportunityCard key={opp.id} opp={opp} onReview={handleReview} onDelete={handleDeleteOpp} />
          ))}
        </div>
      )}
    </div>
  );
}

function StatCard({ label, value, tone }: { label: string; value: number; tone: 'ok' | 'accent' | 'primary' }) {
  const cls = tone === 'ok' ? 'bg-success-container text-on-success-container rounded-[28px_8px_28px_28px]'
    : tone === 'accent' ? 'bg-tertiary-container text-on-tertiary-container rounded-[8px_28px_28px_28px]'
    : 'bg-primary-container text-on-primary-container rounded-[28px_28px_8px_28px]';
  return (
    <div className={`p-4 sm:p-5 ${cls}`}>
      <div className="text-4xl sm:text-5xl font-extrabold leading-none tracking-tight">{value}</div>
      <div className="text-sm font-bold mt-2">{label}</div>
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
    <div className={`bg-surface-container-low transition-[border-radius,background-color] duration-500 ease-[var(--spring)] ${expanded ? 'rounded-[32px]' : 'rounded-[28px] hover:rounded-[36px] hover:bg-surface-container'} ${isClosing ? 'ring-[3px] ring-tertiary' : ''}`}>
      <div
        className="p-5 cursor-pointer rounded-[inherit]"
        role="button"
        tabIndex={0}
        aria-expanded={expanded}
        onClick={handleExpand}
        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleExpand(); } }}
      >
        <div className="flex flex-wrap gap-2 mb-3">
          <span className={`tag !text-sm !px-3 ${CATEGORY_COLORS[opp.category] || CATEGORY_COLORS.other}`}>
            {CATEGORY_LABELS[opp.category] || opp.category}
          </span>
          <span className={`tag !text-sm !px-3 ${STATUS_COLORS[opp.status] || STATUS_COLORS.unknown}`}>
            {STATUS_LABELS[opp.status] || opp.status}
          </span>
          {opp.reviewed && <span className="tag tag-ok !text-sm !px-3">Revisada</span>}
        </div>

        <h3 className="text-xl font-extrabold leading-tight tracking-tight">{opp.title || 'Sin título'}</h3>
        {opp.agency && <p className="text-sm font-bold text-primary mt-1">{opp.agency}</p>}

        {!expanded && opp.summary && (
          <p className="text-sm text-on-surface-variant mt-2 line-clamp-2">{opp.summary}</p>
        )}

        <div className="grid grid-cols-2 gap-3 mt-4">
          <div className="rounded-[20px] bg-surface-container-high px-4 py-3">
            <div className="text-xs font-bold text-on-surface-variant">Monto</div>
            <div className="text-lg font-extrabold leading-tight">{opp.amount || 'Sin monto'}</div>
          </div>
          <div className={`rounded-[20px] px-4 py-3 ${isClosing ? 'bg-tertiary-container text-on-tertiary-container' : 'bg-surface-container-high'}`}>
            <div className="text-xs font-bold opacity-80">Fecha límite</div>
            <div className="text-lg font-extrabold leading-tight">{opp.deadline ? formatDate(opp.deadline) : 'Sin fecha'}</div>
            {opp.deadline && <div className="text-xs font-bold">{deadlineDays}</div>}
          </div>
        </div>

        <div className="flex justify-center mt-2 text-on-surface-variant" aria-hidden="true">
          <svg className={`w-6 h-6 transition-transform duration-500 ease-[var(--spring)] ${expanded ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
          </svg>
        </div>
      </div>

      {expanded && (
        <div className="px-5 pb-5 space-y-4 animate-fade-in">
          {opp.summary && <p className="text-sm text-on-surface leading-relaxed">{opp.summary}</p>}

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-sm">
            {opp.contact_name && (
              <div><span className="font-bold text-on-surface-variant">Contacto:</span> {opp.contact_name}</div>
            )}
            {opp.contact_email && (
              <div>
                <span className="font-bold text-on-surface-variant">Correo:</span>{' '}
                <button type="button" onClick={(e) => { e.stopPropagation(); openExternal(`mailto:${opp.contact_email}`); }} className="text-primary font-semibold underline !min-h-0 !min-w-0 break-all">
                  {opp.contact_email}
                </button>
              </div>
            )}
          </div>

          <div className="flex flex-wrap gap-2">
            {opp.source_url && (
              <button type="button" onClick={(e) => { e.stopPropagation(); openExternal(opp.source_url); }} className="btn btn-filled btn-sm">
                <Icon name="open" className="w-5 h-5" />Abrir página
              </button>
            )}
            {!opp.reviewed && (
              <button type="button" onClick={(e) => { e.stopPropagation(); onReview(opp.id); }} className="btn btn-tonal btn-sm">
                <Icon name="check" className="w-5 h-5" />Marcar revisada
              </button>
            )}
            <button type="button" onClick={(e) => { e.stopPropagation(); onDelete(opp.id); }} className="btn btn-danger btn-sm">
              <Icon name="trash" className="w-5 h-5" />Quitar
            </button>
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
                    <div className="text-sm text-on-surface-variant py-4 text-center">
                      No hay texto limpio.
                      {opp.source_url && (
                        <button type="button" onClick={() => openExternal(opp.source_url)} className="ml-1 text-primary underline font-semibold !min-h-0 !min-w-0">
                          Abrir la página
                        </button>
                      )}
                    </div>
                  );
                })()
              ) : (
                <div className="text-sm text-on-surface-variant py-4 text-center">Sin texto disponible</div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Agencias ───────────────────────────────────────────────────────

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
        <h2 className="title-lg">Agencias ({domains.length})</h2>
        <button type="button" onClick={() => setShowForm(!showForm)} className={showForm ? 'btn btn-soft' : 'btn btn-tonal'}>
          {showForm ? 'Cancelar' : (<><Icon name="plus" className="w-5 h-5" />Agregar agencia</>)}
        </button>
      </div>

      {showForm && <DomainForm onCreated={() => { setShowForm(false); onDomainsChanged(); }} />}

      {domains.length === 0 ? (
        <EmptyState icon="globe" text="Aún no hay agencias. Agrega una para empezar." />
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
                    {d.domain} &middot; {d.category} &middot; {d.page_count} páginas
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
    <form onSubmit={handleSubmit} className="card space-y-4">
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label htmlFor="df-dom" className="field-label">Página de la agencia *</label>
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
      </div>
      <button type="submit" disabled={saving} className="btn btn-filled">{saving ? 'Guardando...' : 'Crear agencia'}</button>
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
    <div className="p-4 space-y-3 !bg-primary-container text-on-primary-container">
      <div className="title-md">{domain.domain}</div>
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <input value={label} onChange={e => setLabel(e.target.value)} placeholder="Nombre" aria-label="Nombre" className="field" />
        <select value={category} onChange={e => setCategory(e.target.value)} aria-label="Tipo" className="field font-semibold">
          {DOMAIN_CATEGORY_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
        </select>
        <input type="number" value={priority} onChange={e => setPriority(+e.target.value)} min={1} max={10} aria-label="Prioridad" className="field" />
      </div>
      <div className="flex gap-2">
        <button type="button" onClick={handleSave} disabled={saving} className="btn btn-filled btn-sm">{saving ? 'Guardando...' : 'Guardar'}</button>
        <button type="button" onClick={onCancel} className="btn btn-soft btn-sm">Cancelar</button>
      </div>
    </div>
  );
}
