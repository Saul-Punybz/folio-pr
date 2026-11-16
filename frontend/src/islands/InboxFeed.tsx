import { useState, useEffect, useCallback, useRef } from 'react';
import hotkeys from 'hotkeys-js';
import { api, type Article, type Source } from '../lib/api';
import ArticleCard from './ArticleCard';
import NotesPanel from './NotesPanel';
import { formatDate } from '../lib/utils';
import { useNearEnd } from '../lib/useNearEnd';
import { Icon, Spinner, EmptyState, Modal, Notice, tagLabel } from './ui';

const TAG_OPTIONS = [
  'politics', 'economy', 'health', 'education', 'infrastructure',
  'environment', 'crime', 'grants', 'federal', 'legislation',
  'government', 'technology', 'culture', 'sports', 'business',
  'small-business', 'nonprofit', 'entrepreneurship', 'tourism',
  'energy', 'agriculture', 'housing',
];

const PAGE = 30;

const REGION_OPTIONS = ['PR', 'US', 'Global'];
// Región de una noticia, en palabras de la usuaria. Grants/Federal no son noticias:
// esos documentos van a Fondos y no se muestran aquí.
const REGION_LABELS: Record<string, string> = { PR: 'Puerto Rico', US: 'EE. UU.', Global: 'Mundo' };
const regionLabel = (r: string) => REGION_LABELS[r] ?? r;
const isOutsidePR = (r: string) => r === 'US' || r === 'Global';

const DATE_OPTIONS = [
  { label: 'Todo', value: '' },
  { label: 'Hoy', value: 'today' },
  { label: 'Ayer', value: 'yesterday' },
  { label: 'Esta semana', value: 'week' },
];

interface UndoAction {
  article: Article;
  action: 'save' | 'trash';
  timer: ReturnType<typeof setTimeout>;
}

function SkeletonCard() {
  return (
    <div className="p-5 rounded-[28px] bg-surface-container-low animate-pulse">
      <div className="h-3 w-24 bg-surface-container-high rounded-full mb-4" />
      <div className="h-5 w-3/4 bg-surface-container-high rounded-full mb-2" />
      <div className="h-5 w-1/2 bg-surface-container-high rounded-full mb-4" />
      <div className="h-4 w-full bg-surface-container-high rounded-full mb-2" />
      <div className="h-4 w-2/3 bg-surface-container-high rounded-full mb-5" />
      <div className="h-11 w-full bg-surface-container-high rounded-full" />
    </div>
  );
}

function ArticleBody({ text }: { text: string }) {
  if (!text) return null;
  const paragraphs = text.split(/\n\n+/).filter((p) => p.trim());
  return (
    <div className="space-y-4">
      {paragraphs.map((p, i) => (
        <p key={i} className="text-base text-on-surface leading-[1.75]">
          {p.trim()}
        </p>
      ))}
    </div>
  );
}

/** Agregar fuente */
function QuickAddSourceModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [url, setUrl] = useState('');
  const [region, setRegion] = useState('PR');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState<{ message: string; detected: boolean } | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => { inputRef.current?.focus(); }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.trim()) { setError('Falta el enlace'); return; }
    setSubmitting(true); setError(''); setResult(null);
    try {
      const data = await api.quickCreateSource(url.trim(), region);
      setResult({ message: data.message, detected: data.detected });
      onCreated();
      setTimeout(() => onClose(), 2500);
    } catch (err: any) {
      setError(err.message || 'No se pudo agregar la fuente');
    } finally { setSubmitting(false); }
  };

  return (
    <Modal title="Agregar fuente" onClose={onClose}>
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && <Notice>{error}</Notice>}
        {result && <Notice kind={result.detected ? 'ok' : 'accent'}>{result.message}</Notice>}
        <div>
          <label htmlFor="qa-url" className="field-label">Enlace del sitio o del feed</label>
          <input id="qa-url" ref={inputRef} type="url" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://ejemplo.com" className="field" />
        </div>
        <div>
          <span className="field-label">Región</span>
          <div className="chip-row">
            {REGION_OPTIONS.map((r) => (
              <button key={r} type="button" className="chip" aria-pressed={region === r} onClick={() => setRegion(r)}>{regionLabel(r)}</button>
            ))}
          </div>
        </div>
        <div className="flex items-center justify-end gap-2 pt-2">
          <button type="button" onClick={onClose} className="btn btn-text">Cancelar</button>
          <button type="submit" disabled={submitting || !!result} className="btn btn-filled">{submitting ? 'Buscando...' : 'Agregar'}</button>
        </div>
      </form>
    </Modal>
  );
}

/** Pegar un enlace */
function CollectModal({ onClose, onCollected }: { onClose: () => void; onCollected: () => void }) {
  const [url, setUrl] = useState('');
  const [title, setTitle] = useState('');
  const [region, setRegion] = useState('PR');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => { inputRef.current?.focus(); }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.trim()) { setError('Falta el enlace'); return; }
    setSubmitting(true); setError('');
    try {
      await api.collectItem(url.trim(), title.trim(), region);
      onCollected(); onClose();
    } catch (err: any) { setError(err.message || 'No se pudo guardar la noticia'); } finally { setSubmitting(false); }
  };

  return (
    <Modal title="Pegar enlace" onClose={onClose}>
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && <Notice>{error}</Notice>}
        <div>
          <label htmlFor="co-url" className="field-label">Enlace de la noticia</label>
          <input id="co-url" ref={inputRef} type="url" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://ejemplo.com/noticia" className="field" />
        </div>
        <div>
          <label htmlFor="co-title" className="field-label">Título (opcional)</label>
          <input id="co-title" type="text" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Título" className="field" />
        </div>
        <div>
          <span className="field-label">Región</span>
          <div className="chip-row">
            {REGION_OPTIONS.map((r) => (
              <button key={r} type="button" className="chip" aria-pressed={region === r} onClick={() => setRegion(r)}>{regionLabel(r)}</button>
            ))}
          </div>
        </div>
        <div className="flex items-center justify-end gap-2 pt-2">
          <button type="button" onClick={onClose} className="btn btn-text">Cancelar</button>
          <button type="submit" disabled={submitting} className="btn btn-filled">{submitting ? 'Guardando...' : 'Guardar'}</button>
        </div>
      </form>
    </Modal>
  );
}

export default function InboxFeed() {
  const [articles, setArticles] = useState<Article[]>([]);
  const [sources, setSources] = useState<Source[]>([]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [undoStack, setUndoStack] = useState<UndoAction[]>([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [showCollect, setShowCollect] = useState(false);
  const [showAddSource, setShowAddSource] = useState(false);
  const [ingesting, setIngesting] = useState(false);
  const listRef = useRef<HTMLDivElement>(null);
  // Selección múltiple y borrado en bloque.
  const [selecting, setSelecting] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [confirmBulk, setConfirmBulk] = useState(false);
  const [bulkBusy, setBulkBusy] = useState(false);
  const [bulkUndo, setBulkUndo] = useState<{ articles: Article[]; timer: ReturnType<typeof setTimeout> } | null>(null);
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  // Siempre la lista vigente y el filtro de fecha, para pedir la siguiente página.
  const articlesRef = useRef<Article[]>([]);
  articlesRef.current = articles;
  const dateRef = useRef('');
  const gen = useRef(0);

  // Filters
  const [activeTag, setActiveTag] = useState('');
  const [sourceFilter, setSourceFilter] = useState('');
  const [regionFilter, setRegionFilter] = useState('');
  const [dateFilter, setDateFilter] = useState('');

  const fetchItems = useCallback(async (date?: string) => {
    try {
      setLoading(true);
      const d = date ?? dateFilter;
      dateRef.current = d;
      const g = ++gen.current;
      const data = await api.getItems('inbox', PAGE, 0, d);
      if (g !== gen.current) return;
      setArticles(data.items || []);
      setHasMore((data.items?.length ?? 0) >= PAGE);
      setError('');
    } catch (err: any) { setError(err.message); } finally { setLoading(false); }
  }, [dateFilter]);

  const loadMore = useCallback(async () => {
    if (loadingMore || !hasMore) return;
    setLoadingMore(true);
    const g = gen.current;
    try {
      // Lo guardado, borrado o deshecho ya cambió en el servidor: el offset es lo que tenemos en pantalla.
      const data = await api.getItems('inbox', PAGE, articlesRef.current.length, dateRef.current);
      if (g !== gen.current) return;
      const next = data.items || [];
      setArticles((prev) => {
        const seen = new Set(prev.map((a) => a.id));
        return [...prev, ...next.filter((a) => !seen.has(a.id))];
      });
      setHasMore(next.length >= PAGE);
    } catch { /* se queda el botón para reintentar */ } finally { setLoadingMore(false); }
  }, [loadingMore, hasMore]);

  const sentinelRef = useNearEnd(loadMore, hasMore && !loading && !loadingMore);

  useEffect(() => {
    const badge = document.getElementById('inbox-count');
    if (badge) badge.textContent = `${articles.length}${hasMore ? '+' : ''} nuevos`;
  }, [articles.length, hasMore]);

  const fetchSources = useCallback(async () => {
    try { setSources(await api.getSources()); } catch {}
  }, []);

  useEffect(() => { fetchItems(); fetchSources(); }, [fetchItems, fetchSources]);

  const handleFetchNews = async () => {
    setIngesting(true);
    try {
      await api.triggerIngest();
      // Wait then refresh
      setTimeout(() => { fetchItems(); setIngesting(false); }, 8000);
    } catch { setIngesting(false); }
  };

  const keyboardNavRef = useRef(false);
  useEffect(() => {
    if (!keyboardNavRef.current || !listRef.current) return;
    keyboardNavRef.current = false;
    const cards = listRef.current.querySelectorAll('[data-article-card]');
    const card = cards[selectedIndex] as HTMLElement | undefined;
    card?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  }, [selectedIndex]);

  const handleSave = useCallback(async (id: string) => {
    const article = articles.find((a) => a.id === id); if (!article) return;
    if (expandedId === id) setExpandedId(null);
    setArticles((prev) => prev.filter((a) => a.id !== id));
    setSelectedIndex((prev) => Math.min(prev, Math.max(0, articles.length - 2)));
    const timer = setTimeout(() => { setUndoStack((prev) => prev.filter((u) => u.article.id !== id)); }, 8000);
    setUndoStack((prev) => [...prev, { article, action: 'save', timer }]);
    try { await api.saveItem(id); } catch {
      setArticles((prev) => [...prev, article].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()));
      setUndoStack((prev) => prev.filter((u) => u.article.id !== id)); clearTimeout(timer);
    }
  }, [articles, expandedId]);

  const handleTrash = useCallback(async (id: string) => {
    const article = articles.find((a) => a.id === id); if (!article) return;
    if (expandedId === id) setExpandedId(null);
    setArticles((prev) => prev.filter((a) => a.id !== id));
    setSelectedIndex((prev) => Math.min(prev, Math.max(0, articles.length - 2)));
    const timer = setTimeout(() => { setUndoStack((prev) => prev.filter((u) => u.article.id !== id)); }, 8000);
    setUndoStack((prev) => [...prev, { article, action: 'trash', timer }]);
    try { await api.trashItem(id); } catch {
      setArticles((prev) => [...prev, article].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()));
      setUndoStack((prev) => prev.filter((u) => u.article.id !== id)); clearTimeout(timer);
    }
  }, [articles, expandedId]);

  const toggleSelected = useCallback((id: string) => {
    setSelected((prev) => { const n = new Set(prev); if (n.has(id)) n.delete(id); else n.add(id); return n; });
  }, []);

  const exitSelecting = () => { setSelecting(false); setSelected(new Set()); setConfirmBulk(false); };

  const handleBulkTrash = async () => {
    const ids = Array.from(selected).slice(0, 500);
    const gone = articles.filter((a) => ids.includes(a.id));
    if (gone.length === 0) return;
    setBulkBusy(true);
    try {
      await api.trashItems(ids);
      setArticles((prev) => prev.filter((a) => !ids.includes(a.id)));
      setExpandedId(null);
      setSelectedIndex(0);
      if (bulkUndo) clearTimeout(bulkUndo.timer);
      const timer = setTimeout(() => setBulkUndo(null), 8000);
      setBulkUndo({ articles: gone, timer });
      exitSelecting();
    } catch (err: any) {
      setError(err.message || 'No se pudieron borrar');
    } finally { setBulkBusy(false); }
  };

  const handleBulkUndo = () => {
    if (!bulkUndo) return;
    clearTimeout(bulkUndo.timer);
    const back = bulkUndo.articles;
    setBulkUndo(null);
    setArticles((prev) => [...prev, ...back].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()));
    api.undoItems(back.map((a) => a.id)).catch(() => {});
  };

  const handlePin = useCallback(async (id: string) => {
    setArticles((prev) => prev.map((a) => (a.id === id ? { ...a, pinned: !a.pinned } : a)));
    try { await api.pinItem(id); } catch {
      setArticles((prev) => prev.map((a) => (a.id === id ? { ...a, pinned: !a.pinned } : a)));
    }
  }, []);

  const handleUndo = useCallback((articleId: string) => {
    const entry = undoStack.find((u) => u.article.id === articleId); if (!entry) return;
    clearTimeout(entry.timer);
    setUndoStack((prev) => prev.filter((u) => u.article.id !== articleId));
    setArticles((prev) => [...prev, entry.article].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()));
    api.undoItem(articleId, 'inbox').catch(() => {});
  }, [undoStack]);

  const handleCardClick = useCallback((index: number, articleId: string) => {
    setSelectedIndex(index);
    setExpandedId((prev) => (prev === articleId ? null : articleId));
  }, []);

  // Keyboard shortcuts
  useEffect(() => {
    hotkeys('j', (e) => { e.preventDefault(); keyboardNavRef.current = true; setSelectedIndex((prev) => Math.min(prev + 1, articles.length - 1)); });
    hotkeys('k', (e) => { e.preventDefault(); keyboardNavRef.current = true; setSelectedIndex((prev) => Math.max(prev - 1, 0)); });
    hotkeys('s', (e) => { e.preventDefault(); const a = articles[selectedIndex]; if (a) handleSave(a.id); });
    hotkeys('t', (e) => { e.preventDefault(); const a = articles[selectedIndex]; if (a) handleTrash(a.id); });
    hotkeys('p', (e) => { e.preventDefault(); const a = articles[selectedIndex]; if (a) handlePin(a.id); });
    hotkeys('enter', (e) => { e.preventDefault(); const a = articles[selectedIndex]; if (a) setExpandedId((prev) => (prev === a.id ? null : a.id)); });
    hotkeys('escape', (e) => { e.preventDefault(); setExpandedId(null); });
    return () => { ['j','k','s','t','p','enter','escape'].forEach(k => hotkeys.unbind(k)); };
  }, [articles, selectedIndex, handleSave, handleTrash, handlePin]);

  if (loading) {
    return (<div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">{Array.from({length:8}).map((_,i) => <SkeletonCard key={i} />)}</div>);
  }

  if (error) {
    return (
      <EmptyState icon="alert" text={error}>
        <button type="button" onClick={() => fetchItems()} className="btn btn-filled">Intentar de nuevo</button>
      </EmptyState>
    );
  }

  const fetchLabel = ingesting ? 'Buscando...' : 'Traer noticias';
  const fab = (
    <div className="fab-slot">
      <button type="button" onClick={handleFetchNews} disabled={ingesting} className="fab disabled:opacity-60" aria-label="Traer noticias nuevas">
        {ingesting ? <Spinner className="w-6 h-6" /> : <Icon name="refresh" className="w-6 h-6" />}
        {fetchLabel}
      </button>
    </div>
  );

  if (articles.length === 0) {
    return (
      <>
        <EmptyState icon="check" text="Estás al día. Toca «Traer noticias» para buscar más.">
          <button type="button" onClick={() => setShowAddSource(true)} className="btn btn-tonal">
            <Icon name="plus" className="w-5 h-5" />Agregar fuente
          </button>
        </EmptyState>
        {fab}
        {showAddSource && <QuickAddSourceModal onClose={() => setShowAddSource(false)} onCreated={fetchSources} />}
      </>
    );
  }

  const uniqueSources = Array.from(new Set(articles.map((a) => a.source).filter(Boolean))).sort();
  const tagCounts: Record<string, number> = {};
  for (const a of articles) { if (a.tags) { for (const t of a.tags) { tagCounts[t] = (tagCounts[t] || 0) + 1; } } }

  const filteredArticles = articles.filter((a) => {
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      if (!a.title.toLowerCase().includes(q) && !a.source?.toLowerCase().includes(q) && !a.summary?.toLowerCase().includes(q) && !a.tags?.some((t) => t.toLowerCase().includes(q))) return false;
    }
    if (activeTag && !a.tags?.some((t) => t === activeTag)) return false;
    if (sourceFilter && a.source !== sourceFilter) return false;
    if (a.region === 'Grants' || a.region === 'Federal') return false;
    if (regionFilter === 'fuera' ? !isOutsidePR(a.region) : regionFilter && a.region !== regionFilter) return false;
    return true;
  });

  const hasActiveFilters = activeTag || sourceFilter || regionFilter || dateFilter;
  const expandedArticle = expandedId ? filteredArticles.find((a) => a.id === expandedId) : null;
  const activeSources = sources.filter((s) => s.active);

  return (
    <>
      {/* Búsqueda y acciones */}
      <div className="flex flex-wrap items-center gap-3 mb-5">
        <div className="relative flex-1 min-w-[220px] max-w-md">
          <span className="absolute left-4 top-1/2 -translate-y-1/2 text-on-surface-variant pointer-events-none"><Icon name="search" className="w-5 h-5" /></span>
          <input type="search" value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} placeholder="Buscar en el Inbox" aria-label="Buscar en el Inbox" className="field pl-12" />
        </div>
        <div className="flex items-center gap-2 ml-auto">
          <button type="button" onClick={() => (selecting ? exitSelecting() : setSelecting(true))} className="btn btn-soft btn-sm" aria-pressed={selecting}>
            {selecting ? 'Listo' : 'Seleccionar'}
          </button>
          <button type="button" onClick={() => setShowAddSource(true)} className="btn btn-tonal btn-sm" title="Agregar fuente">
            <Icon name="plus" className="w-5 h-5" /><span className="hidden sm:inline">Fuente</span>
          </button>
          <button type="button" onClick={() => setShowCollect(true)} className="icon-btn bg-surface-container-high text-on-surface" title="Pegar enlace" aria-label="Pegar enlace">
            <Icon name="link" />
          </button>
        </div>
      </div>

      {/* Fecha */}
      <div className="chip-row chip-scroll mb-3" role="group" aria-label="Fecha">
        {DATE_OPTIONS.map((opt) => (
          <button key={opt.value} type="button" onClick={() => { setDateFilter(opt.value); fetchItems(opt.value); }} className="chip" aria-pressed={dateFilter === opt.value}>
            {opt.label}
          </button>
        ))}
      </div>

      {/* Región */}
      <div className="chip-row chip-scroll mb-3" role="group" aria-label="Región">
        <button type="button" className="chip" aria-pressed={regionFilter === ''} onClick={() => setRegionFilter('')}>Todas</button>
        <button type="button" className="chip" aria-pressed={regionFilter === 'PR'} onClick={() => setRegionFilter(regionFilter === 'PR' ? '' : 'PR')}>Puerto Rico</button>
        <button type="button" className="chip" aria-pressed={regionFilter === 'fuera'} onClick={() => setRegionFilter(regionFilter === 'fuera' ? '' : 'fuera')}>Fuera de PR</button>
      </div>

      {/* Temas */}
      <div className="chip-row chip-scroll mb-4" role="group" aria-label="Temas">
        <button type="button" onClick={() => setActiveTag('')} className="chip" aria-pressed={activeTag === ''}>
          Todos <span className="opacity-70">{articles.length}</span>
        </button>
        {TAG_OPTIONS.filter((t) => tagCounts[t]).map((tag) => (
          <button key={tag} type="button" onClick={() => setActiveTag(activeTag === tag ? '' : tag)} className="chip" aria-pressed={activeTag === tag}>
            {tagLabel(tag)} <span className="opacity-70">{tagCounts[tag]}</span>
          </button>
        ))}
      </div>

      <div className="flex flex-wrap items-center gap-3 mb-5">
        <label className="sr-only" htmlFor="src-filter">Fuente</label>
        <select id="src-filter" value={sourceFilter} onChange={(e) => setSourceFilter(e.target.value)} className="field !w-auto max-w-full !min-h-[44px] text-sm font-semibold pr-8">
          <option value="">Todas las fuentes ({uniqueSources.length})</option>
          {uniqueSources.map((s) => <option key={s} value={s}>{s}</option>)}
        </select>
        {hasActiveFilters && (
          <button type="button" onClick={() => { setActiveTag(''); setSourceFilter(''); setRegionFilter(''); setDateFilter(''); fetchItems(''); }} className="btn btn-text btn-sm">Quitar filtros</button>
        )}
        <div className="ml-auto flex items-center gap-3 text-sm font-semibold text-on-surface-variant">
          <span>{filteredArticles.length} {filteredArticles.length === 1 ? 'noticia' : 'noticias'}</span>
          <a href="/settings" className="btn btn-text btn-sm">Fuentes ({activeSources.length})</a>
        </div>
      </div>

      {filteredArticles.length === 0 && (
        <EmptyState icon="search" text="Nada con esos filtros. Quítalos para ver todo.">
          <button type="button" onClick={() => { setActiveTag(''); setSourceFilter(''); setRegionFilter(''); setDateFilter(''); setSearchQuery(''); fetchItems(''); }} className="btn btn-tonal">Quitar filtros</button>
        </EmptyState>
      )}

      {selecting && (
        <div className="flex flex-wrap items-center gap-2 mb-4">
          <span className="text-sm font-semibold text-on-surface-variant">{selected.size} {selected.size === 1 ? 'seleccionada' : 'seleccionadas'}</span>
          <button type="button" className="btn btn-soft btn-sm" onClick={() => setSelected(new Set(filteredArticles.slice(0, 500).map((a) => a.id)))}>Seleccionar todo lo visible</button>
          {selected.size > 0 && <button type="button" className="btn btn-text btn-sm" onClick={() => setSelected(new Set())}>Quitar selección</button>}
          <button type="button" disabled={selected.size === 0} className="btn btn-danger btn-sm ml-auto disabled:opacity-60" onClick={() => setConfirmBulk(true)}>
            <Icon name="trash" className="w-5 h-5" />Borrar seleccionadas
          </button>
        </div>
      )}

      {/* Noticias */}
      <div ref={listRef} className="stagger grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        {filteredArticles.map((article, index) => (
          <div key={article.id} data-article-card>
            <ArticleCard article={article} selected={index === selectedIndex} onSave={handleSave} onTrash={handleTrash} onPin={handlePin} onClick={() => handleCardClick(index, article.id)} showActions selecting={selecting} checked={selected.has(article.id)} onToggle={toggleSelected} />
          </div>
        ))}
      </div>

      {hasMore && (
        <div ref={sentinelRef} className="flex justify-center pt-6 pb-2">
          <button type="button" onClick={loadMore} disabled={loadingMore} className="btn btn-tonal">
            {loadingMore ? (<><Spinner className="w-5 h-5" />Cargando...</>) : 'Ver más'}
          </button>
        </div>
      )}

      {fab}

      {/* Lector */}
      {expandedArticle && (
        <>
          <div className="fixed inset-0 z-[60] scrim" onClick={() => setExpandedId(null)} />
          <div className="fixed inset-y-0 right-0 z-[70] w-full sm:w-[520px] lg:w-[620px] bg-surface-container-low sm:rounded-l-[36px] shadow-2xl flex flex-col animate-slide-in-right" role="dialog" aria-modal="true" aria-label={expandedArticle.title}>
            <div className="flex items-start justify-between gap-3 px-6 pt-5 pb-3 shrink-0">
              <h2 className="title-lg !leading-tight">{expandedArticle.title}</h2>
              <button type="button" onClick={() => setExpandedId(null)} className="icon-btn shrink-0 -mr-2" aria-label="Cerrar"><Icon name="close" /></button>
            </div>

            <div className="flex-1 overflow-y-auto">
              {expandedArticle.image_url && (
                <div className="mx-4 mb-2 max-h-72 overflow-hidden rounded-[24px]">
                  <img src={expandedArticle.image_url} alt="" className="w-full h-full object-cover" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).parentElement!.style.display = 'none'; }} />
                </div>
              )}

              <div className="p-6 space-y-5">
                <div className="flex flex-wrap items-center gap-2">
                  <button type="button" onClick={() => handleSave(expandedArticle.id)} className="btn btn-filled btn-sm">
                    <Icon name="save" className="w-5 h-5" />Guardar
                  </button>
                  <button type="button" onClick={() => handleTrash(expandedArticle.id)} className="btn btn-soft btn-sm">
                    <Icon name="trash" className="w-5 h-5" />Borrar
                  </button>
                  <button type="button" onClick={() => handlePin(expandedArticle.id)} className={`btn btn-sm ${expandedArticle.pinned ? 'btn-accent' : 'btn-soft'}`} aria-pressed={!!expandedArticle.pinned}>
                    <Icon name="pin" className="w-5 h-5" fill={!!expandedArticle.pinned} />{expandedArticle.pinned ? 'Fijado' : 'Fijar'}
                  </button>
                </div>

                {expandedArticle.summary && expandedArticle.clean_text && expandedArticle.summary !== expandedArticle.clean_text.slice(0, expandedArticle.summary.length) && (
                  <div className="card-hi px-5 py-4">
                    <h4 className="title-md mb-1.5">Resumen</h4>
                    <p className="text-sm leading-relaxed">{expandedArticle.summary}</p>
                  </div>
                )}
                {expandedArticle.summary && !expandedArticle.clean_text && (
                  <div><h4 className="title-md mb-2">Resumen</h4><p className="text-base text-on-surface leading-[1.75]">{expandedArticle.summary}</p></div>
                )}
                {expandedArticle.clean_text && (
                  <div><h4 className="title-md mb-3">Noticia completa</h4><div className="max-w-prose"><ArticleBody text={expandedArticle.clean_text} /></div></div>
                )}
                {!expandedArticle.clean_text && !expandedArticle.summary && (<p className="text-sm text-on-surface-variant">Esta noticia no trae texto. Abre el original.</p>)}

                {expandedArticle.tags && expandedArticle.tags.length > 0 && (
                  <div>
                    <h4 className="title-md mb-2">Temas</h4>
                    <div className="chip-row">
                      {expandedArticle.tags.map((tag) => (
                        <button key={tag} type="button" onClick={() => { setActiveTag(tag); setExpandedId(null); }} className="chip">{tagLabel(tag)}</button>
                      ))}
                    </div>
                  </div>
                )}

                <div className="flex flex-wrap items-center justify-between gap-3 pt-2">
                  <div className="flex items-center gap-2 text-sm text-on-surface-variant font-semibold">
                    <span>{formatDate(expandedArticle.published_at || expandedArticle.created_at)}</span>
                    {expandedArticle.source && <><span aria-hidden="true">&middot;</span><span>{expandedArticle.source}</span></>}
                  </div>
                  {expandedArticle.url && (
                    <a href={expandedArticle.url} target="_blank" rel="noopener noreferrer" className="btn btn-tonal btn-sm">
                      <Icon name="open" className="w-5 h-5" />Abrir original
                    </a>
                  )}
                </div>

                <NotesPanel articleId={expandedArticle.id} />
              </div>
            </div>
          </div>
        </>
      )}

      {showAddSource && <QuickAddSourceModal onClose={() => setShowAddSource(false)} onCreated={fetchSources} />}
      {showCollect && <CollectModal onClose={() => setShowCollect(false)} onCollected={fetchItems} />}

      {confirmBulk && (
        <Modal title="Borrar noticias" onClose={() => setConfirmBulk(false)}>
          <p className="text-base mb-5">¿Borrar {selected.size} {selected.size === 1 ? 'noticia' : 'noticias'}? No volverán a aparecer.</p>
          <div className="flex items-center justify-end gap-2">
            <button type="button" onClick={() => setConfirmBulk(false)} className="btn btn-text">Cancelar</button>
            <button type="button" onClick={handleBulkTrash} disabled={bulkBusy} className="btn btn-danger">{bulkBusy ? 'Borrando...' : 'Borrar'}</button>
          </div>
        </Modal>
      )}

      {(undoStack.length > 0 || bulkUndo) && (
        <div className="snack-stack" role="status" aria-live="polite">
          {bulkUndo && (
            <div className="snack toast-enter">
              <span>Borradas: <span className="font-bold">{bulkUndo.articles.length}</span></span>
              <button type="button" onClick={handleBulkUndo} className="btn btn-text btn-sm">Deshacer</button>
            </div>
          )}
          {undoStack.map((entry) => (
            <div key={entry.article.id} className="snack toast-enter">
              <span>{entry.action === 'save' ? 'Guardada' : 'Borrada'}: <span className="font-bold">{entry.article.title.length > 30 ? entry.article.title.slice(0, 30) + '...' : entry.article.title}</span></span>
              <button type="button" onClick={() => handleUndo(entry.article.id)} className="btn btn-text btn-sm">Deshacer</button>
            </div>
          ))}
        </div>
      )}
    </>
  );
}
