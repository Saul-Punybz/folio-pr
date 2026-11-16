import { useState, useEffect, useCallback, useRef } from 'react';
import { api, type Article } from '../lib/api';
import { statusColor, formatDate, debounce } from '../lib/utils';
import { useNearEnd } from '../lib/useNearEnd';
import { Icon, Spinner, EmptyState, Modal, tagLabel } from './ui';

const TAG_OPTIONS = [
  'politics', 'economy', 'health', 'education', 'infrastructure',
  'environment', 'crime', 'grants', 'federal', 'legislation',
  'government', 'technology', 'culture', 'sports',
];

export default function ArchiveSearch() {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<Article[]>([]);
  const [hasMoreServer, setHasMoreServer] = useState(false);
  const [selecting, setSelecting] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [confirmBulk, setConfirmBulk] = useState(false);
  const [bulkBusy, setBulkBusy] = useState(false);
  const [undoEntry, setUndoEntry] = useState<{ ids: string[]; articles: Article[]; timer: ReturnType<typeof setTimeout> } | null>(null);
  const [loading, setLoading] = useState(false);
  const [hasSearched, setHasSearched] = useState(false);
  const [offset, setOffset] = useState(0);
  const [loadingMore, setLoadingMore] = useState(false);
  const limit = 30;

  // Filters
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [regionFilter, setRegionFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [tagFilter, setTagFilter] = useState('');


  const inputRef = useRef<HTMLInputElement>(null);

  // Read initial query from URL
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const q = params.get('q');
    if (q) setQuery(q);
    performSearch(q || '', '', '', '', '', '', 0);
    inputRef.current?.focus();
  }, []);

  const performSearch = useCallback(
    async (
      q: string,
      from: string,
      to: string,
      region: string,
      status: string,
      tag: string,
      searchOffset: number
    ) => {
      if (searchOffset === 0) {
        setLoading(true);
      } else {
        setLoadingMore(true);
      }
      setHasSearched(true);

      try {
        const params: Record<string, string> = {};
        if (q.trim()) params.q = q.trim();
        if (from) params.from = from;
        if (to) params.to = to;
        if (region) params.region = region;
        if (status) params.status = status;
        if (tag) params.tag = tag;
        params.limit = String(limit);
        params.offset = String(searchOffset);

        const data = await api.search(params);

        if (searchOffset === 0) {
          setResults(data.results || []);
        } else {
          setResults((prev) => [...prev, ...(data.results || [])]);
        }
        setHasMoreServer((data.results || []).length >= limit);
        setOffset(searchOffset);
      } catch (err: any) {
        console.error('Search error:', err);
      } finally {
        setLoading(false);
        setLoadingMore(false);
      }
    },
    []
  );

  // Debounced search
  const debouncedSearch = useCallback(
    debounce((q: string, from: string, to: string, region: string, status: string, tag: string) => {
      performSearch(q, from, to, region, status, tag, 0);
    }, 300),
    [performSearch]
  );

  // Trigger search on filter changes
  useEffect(() => {
    debouncedSearch(query, dateFrom, dateTo, regionFilter, statusFilter, tagFilter);
  }, [query, dateFrom, dateTo, regionFilter, statusFilter, tagFilter, debouncedSearch]);

  const loadMore = () => {
    performSearch(query, dateFrom, dateTo, regionFilter, statusFilter, tagFilter, offset + limit);
  };

  const hasMore = hasMoreServer;
  const sentinelRef = useNearEnd(loadMore, hasMore && !loading && !loadingMore);

  const STATUS_LABELS: Record<string, string> = { saved: 'Guardada', inbox: 'Nueva', archived: 'Archivada' };
  const toggleSelected = (id: string) => setSelected((prev) => { const n = new Set(prev); if (n.has(id)) n.delete(id); else n.add(id); return n; });
  const exitSelecting = () => { setSelecting(false); setSelected(new Set()); setConfirmBulk(false); };

  const trashIds = async (ids: string[]) => {
    const gone = results.filter((a) => ids.includes(a.id));
    if (gone.length === 0) return;
    try {
      if (ids.length === 1) await api.trashItem(ids[0]); else await api.trashItems(ids);
    } catch { return; }
    setResults((prev) => prev.filter((a) => !ids.includes(a.id)));
    if (undoEntry) clearTimeout(undoEntry.timer);
    const timer = setTimeout(() => setUndoEntry(null), 8000);
    setUndoEntry({ ids, articles: gone, timer });
  };

  const bulkTrash = async () => {
    setBulkBusy(true);
    await trashIds(Array.from(selected).slice(0, 500));
    setBulkBusy(false);
    exitSelecting();
  };

  const undoTrash = () => {
    if (!undoEntry) return;
    clearTimeout(undoEntry.timer);
    const back = undoEntry.articles;
    setUndoEntry(null);
    setResults((prev) => [...prev, ...back].sort((a, b) => new Date(b.published_at || b.created_at).getTime() - new Date(a.published_at || a.created_at).getTime()));
    api.undoItems(back.map((a) => a.id)).catch(() => {});
  };

  const saveOne = async (id: string) => {
    setResults((prev) => prev.map((a) => (a.id === id ? { ...a, status: 'saved' } : a)));
    try { await api.saveItem(id); } catch {
      setResults((prev) => prev.map((a) => (a.id === id ? { ...a, status: 'archived' } : a)));
    }
  };

  const hasFilters = !!(dateFrom || dateTo || regionFilter || statusFilter || tagFilter);

  return (
    <div className="space-y-5">
      <div className="relative">
        <span className="absolute left-4 top-1/2 -translate-y-1/2 text-on-surface-variant pointer-events-none"><Icon name="search" className="w-6 h-6" /></span>
        <input
          ref={inputRef}
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Buscar en todo el archivo"
          aria-label="Buscar en todo el archivo"
          className="field !min-h-[60px] pl-14 pr-14 text-lg !rounded-[28px]"
        />
        {query && (
          <button
            type="button"
            onClick={() => { setQuery(''); setResults([]); setHasSearched(false); inputRef.current?.focus(); }}
            className="icon-btn absolute right-2 top-1/2 -translate-y-1/2"
            aria-label="Borrar búsqueda"
          >
            <Icon name="close" />
          </button>
        )}
      </div>

      <div className="space-y-3">
        <div className="chip-row chip-scroll" role="group" aria-label="Estado">
          <button type="button" className="chip" aria-pressed={statusFilter === ''} onClick={() => setStatusFilter('')}>Todas</button>
          <button type="button" className="chip" aria-pressed={statusFilter === 'saved'} onClick={() => setStatusFilter('saved')}>Guardadas</button>
          <button type="button" className="chip" aria-pressed={statusFilter === 'archived'} onClick={() => setStatusFilter('archived')}>Archivadas</button>
        </div>
        <div className="chip-row chip-scroll" role="group" aria-label="Región">
          <button type="button" className="chip" aria-pressed={regionFilter === ''} onClick={() => setRegionFilter('')}>Todas las regiones</button>
          {['PR', 'Grants', 'Federal', 'Local'].map((r) => (
            <button key={r} type="button" className="chip" aria-pressed={regionFilter === r} onClick={() => setRegionFilter(regionFilter === r ? '' : r)}>{r}</button>
          ))}
        </div>
        <div className="flex flex-wrap items-end gap-3">
          <div className="min-w-[150px]">
            <label htmlFor="ar-from" className="field-label">Desde</label>
            <input id="ar-from" type="date" value={dateFrom} onChange={(e) => setDateFrom(e.target.value)} className="field" />
          </div>
          <div className="min-w-[150px]">
            <label htmlFor="ar-to" className="field-label">Hasta</label>
            <input id="ar-to" type="date" value={dateTo} onChange={(e) => setDateTo(e.target.value)} className="field" />
          </div>
          <div className="min-w-[180px]">
            <label htmlFor="ar-tag" className="field-label">Tema</label>
            <select id="ar-tag" value={tagFilter} onChange={(e) => setTagFilter(e.target.value)} className="field font-semibold">
              <option value="">Todos los temas</option>
              {TAG_OPTIONS.map((t) => (
                <option key={t} value={t}>{tagLabel(t)}</option>
              ))}
            </select>
          </div>
          {hasFilters && (
            <button type="button" onClick={() => { setDateFrom(''); setDateTo(''); setRegionFilter(''); setStatusFilter(''); setTagFilter(''); }} className="btn btn-text">
              Quitar filtros
            </button>
          )}
        </div>
      </div>

      {hasSearched && !loading && (
        <p className="text-sm font-semibold text-on-surface-variant">
          {results.length}{hasMore ? '+' : ''} {results.length === 1 ? 'resultado' : 'resultados'}
          {query && ` para «${query}»`}
          {tagFilter && ` en ${tagLabel(tagFilter)}`}
        </p>
      )}

      <div className="flex flex-wrap items-center gap-2">
        <button type="button" className="btn btn-soft btn-sm" aria-pressed={selecting} onClick={() => (selecting ? exitSelecting() : setSelecting(true))}>
          {selecting ? 'Listo' : 'Seleccionar'}
        </button>
        {selecting && (
          <>
            <span className="text-sm font-semibold text-on-surface-variant">{selected.size} {selected.size === 1 ? 'seleccionada' : 'seleccionadas'}</span>
            <button type="button" className="btn btn-soft btn-sm" onClick={() => setSelected(new Set(results.slice(0, 500).map((a) => a.id)))}>Seleccionar todo lo visible</button>
            <button type="button" disabled={selected.size === 0} className="btn btn-danger btn-sm ml-auto disabled:opacity-60" onClick={() => setConfirmBulk(true)}>
              <Icon name="trash" className="w-5 h-5" />Borrar seleccionadas
            </button>
          </>
        )}
      </div>

      {loading && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {[...Array(6)].map((_, i) => (
            <div key={i} className="p-5 rounded-[28px] bg-surface-container-low animate-pulse">
              <div className="h-3 w-24 bg-surface-container-high rounded-full mb-3" />
              <div className="h-5 w-3/4 bg-surface-container-high rounded-full mb-2" />
              <div className="h-4 w-2/3 bg-surface-container-high rounded-full" />
            </div>
          ))}
        </div>
      )}

      {hasSearched && !loading && results.length === 0 && (
        <EmptyState icon="search" text="No encontramos nada. Prueba con otra palabra o quita filtros." />
      )}

      {!loading && results.length > 0 && (
        <div className="stagger grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {results.map((article) => {
            const status = statusColor(article.status);
            const description = article.summary || (article.clean_text ? article.clean_text.slice(0, 160) + '...' : '');

            return (
              <article key={article.id} onClick={selecting ? () => toggleSelected(article.id) : undefined} className={`${selected.has(article.id) ? 'ring-[3px] ring-primary ' : ''}flex flex-col h-full overflow-hidden bg-surface-container-low hover:bg-surface-container rounded-[28px] hover:rounded-[36px] transition-[border-radius,background-color] duration-500 ease-[var(--spring)]`}>
                <div className="px-5 pt-5 flex items-center justify-between gap-2">
                  <div className="flex items-center gap-1.5 text-xs text-on-surface-variant min-w-0">
                    {selecting && <input type="checkbox" checked={selected.has(article.id)} onChange={() => toggleSelected(article.id)} onClick={(e) => e.stopPropagation()} aria-label={`Seleccionar: ${article.title}`} className="shrink-0" />}
                    <span className="font-semibold truncate">{article.source}</span>
                    <span aria-hidden="true">&middot;</span>
                    <span className="shrink-0">{formatDate(article.created_at)}</span>
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    <span className="tag tag-primary uppercase tracking-wider">{article.region}</span>
                    <span className={`tag ${status.bg} ${status.text}`}>{STATUS_LABELS[article.status] || article.status}</span>
                  </div>
                </div>

                <h3 className="px-5 pt-3 text-lg font-extrabold leading-tight tracking-tight line-clamp-3">
                  {article.url ? (
                    <a href={article.url} target="_blank" rel="noopener noreferrer" className="hover:text-primary transition-colors">
                      {article.title}
                    </a>
                  ) : article.title}
                </h3>

                {description && <p className="px-5 pt-2 text-sm text-on-surface-variant leading-relaxed line-clamp-3 flex-1">{description}</p>}
                {!description && <div className="flex-1" />}

                {article.tags && article.tags.length > 0 && (
                  <div className="px-5 pt-3 flex flex-wrap gap-1.5">
                    {article.tags.slice(0, 4).map((t) => (
                      <button key={t} type="button" onClick={() => setTagFilter(t)} className="tag hover:bg-primary-container hover:text-on-primary-container">
                        {tagLabel(t)}
                      </button>
                    ))}
                  </div>
                )}

                {article.image_url && (
                  <div className="mx-3 mt-4 h-28 overflow-hidden rounded-[20px] bg-surface-container-high">
                    <img src={article.image_url} alt="" className="w-full h-full object-cover" loading="lazy"
                      onError={(e) => { (e.target as HTMLImageElement).parentElement!.style.display = 'none'; }} />
                  </div>
                )}

                {!selecting && (
                  <div className="flex items-center gap-2 p-3 pt-4">
                    {article.status !== 'saved' && (
                      <button type="button" onClick={() => saveOne(article.id)} className="btn btn-filled btn-sm flex-1">
                        <Icon name="save" className="w-5 h-5" />Guardar
                      </button>
                    )}
                    <button type="button" onClick={() => trashIds([article.id])} className="btn btn-soft btn-sm">
                      <Icon name="trash" className="w-5 h-5" />Borrar
                    </button>
                  </div>
                )}
              </article>
            );
          })}
        </div>
      )}

      {hasMore && !loading && (
        <div ref={sentinelRef} className="flex justify-center pt-4">
          <button type="button" onClick={loadMore} disabled={loadingMore} className="btn btn-tonal">
            {loadingMore ? (<><Spinner className="w-5 h-5" />Cargando...</>) : 'Ver más'}
          </button>
        </div>
      )}

      {confirmBulk && (
        <Modal title="Borrar noticias" onClose={() => setConfirmBulk(false)}>
          <p className="text-base mb-5">¿Borrar {selected.size} {selected.size === 1 ? 'noticia' : 'noticias'}? No volverán a aparecer.</p>
          <div className="flex items-center justify-end gap-2">
            <button type="button" onClick={() => setConfirmBulk(false)} className="btn btn-text">Cancelar</button>
            <button type="button" onClick={bulkTrash} disabled={bulkBusy} className="btn btn-danger">{bulkBusy ? 'Borrando...' : 'Borrar'}</button>
          </div>
        </Modal>
      )}

      {undoEntry && (
        <div className="snack-stack" role="status" aria-live="polite">
          <div className="snack toast-enter">
            <span>Borrada{undoEntry.articles.length > 1 ? 's' : ''}: <span className="font-bold">{undoEntry.articles.length}</span></span>
            <button type="button" onClick={undoTrash} className="btn btn-text btn-sm">Deshacer</button>
          </div>
        </div>
      )}
    </div>
  );
}
