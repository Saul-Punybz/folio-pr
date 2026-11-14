import { useState, useEffect, useCallback, useRef } from 'react';
import { api, type Article } from '../lib/api';
import { regionColor, statusColor, isExpired, formatDate, debounce } from '../lib/utils';
import { useNearEnd } from '../lib/useNearEnd';
import { Icon, Spinner, EmptyState, tagLabel } from './ui';

const TAG_OPTIONS = [
  'politics', 'economy', 'health', 'education', 'infrastructure',
  'environment', 'crime', 'grants', 'federal', 'legislation',
  'government', 'technology', 'culture', 'sports',
];

export default function ArchiveSearch() {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<Article[]>([]);
  const [total, setTotal] = useState(0);
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
    if (q) {
      setQuery(q);
      performSearch(q, '', '', '', '', '', 0);
    }
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
        setTotal(data.total || 0);
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
    if (query.trim() || dateFrom || dateTo || regionFilter || statusFilter || tagFilter) {
      debouncedSearch(query, dateFrom, dateTo, regionFilter, statusFilter, tagFilter);
    }
  }, [query, dateFrom, dateTo, regionFilter, statusFilter, tagFilter, debouncedSearch]);

  const loadMore = () => {
    performSearch(query, dateFrom, dateTo, regionFilter, statusFilter, tagFilter, offset + limit);
  };

  const hasMore = results.length < total;
  const sentinelRef = useNearEnd(loadMore, hasMore && !loading && !loadingMore);

  const STATUS_LABELS: Record<string, string> = { saved: 'Guardada', trashed: 'Descartada', inbox: 'Inbox', archived: 'Archivada' };
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
          <button type="button" className="chip" aria-pressed={statusFilter === ''} onClick={() => setStatusFilter('')}>Todo</button>
          {(['saved', 'inbox', 'trashed'] as const).map((st) => (
            <button key={st} type="button" className="chip" aria-pressed={statusFilter === st} onClick={() => setStatusFilter(statusFilter === st ? '' : st)}>{STATUS_LABELS[st]}</button>
          ))}
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
          {total} {total === 1 ? 'resultado' : 'resultados'}
          {query && ` para «${query}»`}
          {tagFilter && ` en ${tagLabel(tagFilter)}`}
        </p>
      )}

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
            const expired = isExpired(article.evidence_expires_at);
            const description = article.summary || (article.clean_text ? article.clean_text.slice(0, 160) + '...' : '');

            return (
              <article key={article.id} className="flex flex-col h-full overflow-hidden bg-surface-container-low hover:bg-surface-container rounded-[28px] hover:rounded-[36px] transition-[border-radius,background-color] duration-500 ease-[var(--spring)]">
                <div className="px-5 pt-5 flex items-center justify-between gap-2">
                  <div className="flex items-center gap-1.5 text-xs text-on-surface-variant min-w-0">
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

                <div className="p-4 min-h-[16px]">
                  {expired && <span className="tag tag-accent">Vencido</span>}
                </div>
              </article>
            );
          })}
        </div>
      )}

      {hasMore && !loading && (
        <div ref={sentinelRef} className="flex justify-center pt-4">
          <button type="button" onClick={loadMore} disabled={loadingMore} className="btn btn-tonal">
            {loadingMore ? (<><Spinner className="w-5 h-5" />Cargando...</>) : `Ver más (${results.length} de ${total})`}
          </button>
        </div>
      )}

      {!hasSearched && (
        <EmptyState icon="search" text="Escribe una palabra para buscar en todo el archivo." />
      )}
    </div>
  );
}
