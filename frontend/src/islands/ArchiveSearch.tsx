import { useState, useEffect, useCallback, useRef } from 'react';
import { api, type Article } from '../lib/api';
import { timeAgo, regionColor, statusColor, isExpired, formatDate, debounce } from '../lib/utils';

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
  const limit = 25;

  // Filters
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [regionFilter, setRegionFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [tagFilter, setTagFilter] = useState('');

  // Similar articles panel
  const [similarResults, setSimilarResults] = useState<Article[]>([]);
  const [similarLoading, setSimilarLoading] = useState(false);
  const [similarForId, setSimilarForId] = useState<string | null>(null);
  const [similarForTitle, setSimilarForTitle] = useState('');

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

  const showSimilar = async (articleId: string, articleTitle: string) => {
    // Toggle off if clicking the same article
    if (similarForId === articleId) {
      setSimilarForId(null);
      setSimilarResults([]);
      return;
    }

    setSimilarForId(articleId);
    setSimilarForTitle(articleTitle);
    setSimilarLoading(true);
    setSimilarResults([]);

    try {
      const data = await api.similar(articleId, 5);
      setSimilarResults(data.results || []);
    } catch (err: any) {
      console.error('Similar articles error:', err);
    } finally {
      setSimilarLoading(false);
    }
  };

  const selectClass =
    'px-3 py-2 text-sm bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 rounded-lg text-zinc-900 dark:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent transition-colors appearance-none';

  const inputClass =
    'px-3 py-2 text-sm bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 rounded-lg text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 dark:placeholder-zinc-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent transition-colors';

  return (
    <div className="space-y-4">
      {/* Search input */}
      <div className="relative">
        <svg
          className="absolute left-4 top-1/2 -translate-y-1/2 w-5 h-5 text-zinc-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z"
          />
        </svg>
        <input
          ref={inputRef}
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search all articles..."
          className="w-full pl-12 pr-4 py-3 text-base bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 rounded-xl text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 dark:placeholder-zinc-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent transition-colors"
        />
        {query && (
          <button
            onClick={() => {
              setQuery('');
              setResults([]);
              setHasSearched(false);
              inputRef.current?.focus();
            }}
            className="absolute right-3 top-1/2 -translate-y-1/2 p-1 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300 transition-colors"
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        )}
      </div>

      {/* Filter row */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-2">
          <label className="text-xs font-medium text-zinc-500 dark:text-zinc-400">From</label>
          <input
            type="date"
            value={dateFrom}
            onChange={(e) => setDateFrom(e.target.value)}
            className={inputClass}
          />
        </div>
        <div className="flex items-center gap-2">
          <label className="text-xs font-medium text-zinc-500 dark:text-zinc-400">To</label>
          <input
            type="date"
            value={dateTo}
            onChange={(e) => setDateTo(e.target.value)}
            className={inputClass}
          />
        </div>
        <select
          value={regionFilter}
          onChange={(e) => setRegionFilter(e.target.value)}
          className={selectClass}
        >
          <option value="">All Regions</option>
          <option value="PR">PR</option>
          <option value="Grants">Grants</option>
          <option value="Federal">Federal</option>
          <option value="Local">Local</option>
        </select>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className={selectClass}
        >
          <option value="">All Statuses</option>
          <option value="saved">Saved</option>
          <option value="trashed">Trashed</option>
          <option value="inbox">Inbox</option>
        </select>
        <select
          value={tagFilter}
          onChange={(e) => setTagFilter(e.target.value)}
          className={selectClass}
        >
          <option value="">All Tags</option>
          {TAG_OPTIONS.map((t) => (
            <option key={t} value={t}>
              {t.charAt(0).toUpperCase() + t.slice(1)}
            </option>
          ))}
        </select>

        {(dateFrom || dateTo || regionFilter || statusFilter || tagFilter) && (
          <button
            onClick={() => {
              setDateFrom('');
              setDateTo('');
              setRegionFilter('');
              setStatusFilter('');
              setTagFilter('');
            }}
            className="text-xs text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300 transition-colors"
          >
            Clear filters
          </button>
        )}
      </div>

      {/* Results count */}
      {hasSearched && !loading && (
        <p className="text-xs text-zinc-400 dark:text-zinc-500">
          {total} result{total !== 1 ? 's' : ''}
          {query && ` for "${query}"`}
          {tagFilter && ` tagged "${tagFilter}"`}
        </p>
      )}

      {/* Loading state */}
      {loading && (
        <div className="space-y-3 pt-2">
          {[...Array(5)].map((_, i) => (
            <div
              key={i}
              className="p-4 rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 animate-pulse"
            >
              <div className="flex items-center gap-2 mb-2">
                <div className="h-3 w-24 bg-zinc-200 dark:bg-zinc-800 rounded" />
                <div className="h-3 w-16 bg-zinc-200 dark:bg-zinc-800 rounded" />
              </div>
              <div className="h-4 w-2/3 bg-zinc-200 dark:bg-zinc-800 rounded" />
            </div>
          ))}
        </div>
      )}

      {/* No results */}
      {hasSearched && !loading && results.length === 0 && (
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <div className="w-14 h-14 rounded-full bg-zinc-100 dark:bg-zinc-800 flex items-center justify-center mb-4">
            <svg
              className="w-7 h-7 text-zinc-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z"
              />
            </svg>
          </div>
          <h3 className="text-lg font-medium text-zinc-900 dark:text-zinc-100 mb-1">
            No results found
          </h3>
          <p className="text-sm text-zinc-500 dark:text-zinc-400">
            Try adjusting your search or filters.
          </p>
        </div>
      )}

      {/* Results grid */}
      {!loading && results.length > 0 && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
          {results.map((article) => {
            const region = regionColor(article.region);
            const status = statusColor(article.status);
            const expired = isExpired(article.evidence_expires_at);
            const isSimilarOpen = similarForId === article.id;
            const description = article.summary || (article.clean_text ? article.clean_text.slice(0, 160) + '...' : '');

            return (
              <div key={article.id} className="flex flex-col">
                <div className="flex flex-col h-full border border-zinc-300 dark:border-zinc-700 rounded-sm overflow-hidden hover:shadow-md transition-all">
                  {/* Dark header */}
                  <div className="bg-zinc-800 dark:bg-zinc-950 px-3 py-2 flex items-center justify-between">
                    <div className="flex items-center gap-1.5 text-[10px] text-zinc-400 min-w-0">
                      <span className="font-medium text-zinc-300 truncate">{article.source}</span>
                      <span className="text-zinc-600 shrink-0">&middot;</span>
                      <span className="text-zinc-500 shrink-0">{formatDate(article.created_at)}</span>
                    </div>
                    <div className="flex items-center gap-1 shrink-0">
                      <span className="inline-flex items-center px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-widest bg-zinc-100 text-zinc-800 rounded-sm">
                        {article.region}
                      </span>
                      <span className={`inline-flex items-center px-1.5 py-0.5 text-[9px] font-bold capitalize rounded-sm ${status.bg} ${status.text}`}>
                        {article.status}
                      </span>
                    </div>
                  </div>

                  {/* Title */}
                  <div className="bg-zinc-50 dark:bg-zinc-900 px-3 py-2.5 border-b border-zinc-200 dark:border-zinc-800">
                    <h3 className="text-sm font-extrabold text-zinc-900 dark:text-zinc-50 leading-tight line-clamp-2 uppercase tracking-tight">
                      {article.url ? (
                        <a href={article.url} target="_blank" rel="noopener noreferrer" className="hover:text-indigo-500 transition-colors">
                          {article.title}
                        </a>
                      ) : article.title}
                    </h3>
                  </div>

                  {/* Body */}
                  <div className="flex-1 bg-white dark:bg-zinc-900 px-3 py-2">
                    {description && (
                      <p className="text-[11px] text-zinc-600 dark:text-zinc-400 leading-relaxed line-clamp-3 mb-2">{description}</p>
                    )}
                    {article.tags && article.tags.length > 0 && (
                      <div className="flex flex-wrap gap-1">
                        {article.tags.slice(0, 4).map((t) => (
                          <button key={t} onClick={() => setTagFilter(t)}
                            className="inline-flex items-center px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wider rounded-sm bg-zinc-100 dark:bg-zinc-800 text-zinc-500 hover:bg-indigo-100 hover:text-indigo-600 dark:hover:bg-indigo-900/30 dark:hover:text-indigo-400 transition-colors">
                            {t}
                          </button>
                        ))}
                      </div>
                    )}
                  </div>

                  {/* Image */}
                  {article.image_url && (
                    <div className="w-full h-28 overflow-hidden bg-zinc-200 dark:bg-zinc-800">
                      <img src={article.image_url} alt="" className="w-full h-full object-cover" loading="lazy"
                        onError={(e) => { (e.target as HTMLImageElement).parentElement!.style.display = 'none'; }} />
                    </div>
                  )}

                  {/* Actions bar */}
                  <div className="flex items-center gap-1 px-2 py-1.5 bg-zinc-50 dark:bg-zinc-950 border-t border-zinc-200 dark:border-zinc-800">
                    <button
                      onClick={() => showSimilar(article.id, article.title)}
                      className={`inline-flex items-center gap-1 px-2 py-0.5 text-[9px] font-bold uppercase rounded-sm transition-colors ${
                        isSimilarOpen ? 'bg-indigo-500/20 text-indigo-600' : 'text-zinc-500 hover:text-indigo-600'
                      }`}
                    >
                      <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
                      </svg>
                      Similar
                    </button>
                    {expired && (
                      <span className="text-[9px] font-bold text-amber-500">Expired</span>
                    )}
                  </div>
                </div>

                {/* Similar articles panel */}
                {isSimilarOpen && (
                  <div className="mt-1 mb-2 pl-3 border-l-2 border-indigo-300 dark:border-indigo-700">
                    <p className="text-xs font-medium text-indigo-600 dark:text-indigo-400 mb-2">
                      Similar to: {similarForTitle}
                    </p>
                    {similarLoading ? (
                      <div className="space-y-2">
                        {[...Array(3)].map((_, i) => (
                          <div key={i} className="p-2 rounded bg-zinc-50 dark:bg-zinc-800/50 animate-pulse">
                            <div className="h-3 w-2/3 bg-zinc-200 dark:bg-zinc-700 rounded" />
                          </div>
                        ))}
                      </div>
                    ) : similarResults.length === 0 ? (
                      <p className="text-xs text-zinc-400 py-2">No similar articles found.</p>
                    ) : (
                      <div className="space-y-1">
                        {similarResults.map((sim) => (
                          <div key={sim.id} className="p-2 rounded bg-zinc-50 dark:bg-zinc-800/50 hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors">
                            <div className="flex items-center gap-2 text-[10px] text-zinc-400 mb-0.5">
                              <span>{sim.source}</span>
                              <span>&middot;</span>
                              <span>{formatDate(sim.created_at)}</span>
                            </div>
                            <h4 className="text-xs font-medium text-zinc-800 dark:text-zinc-200">
                              {sim.url ? (
                                <a href={sim.url} target="_blank" rel="noopener noreferrer" className="hover:text-indigo-500 transition-colors">{sim.title}</a>
                              ) : sim.title}
                            </h4>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Load more */}
      {hasMore && !loading && (
        <div className="flex justify-center pt-4">
          <button
            onClick={loadMore}
            disabled={loadingMore}
            className="px-6 py-2.5 text-sm font-medium text-zinc-600 dark:text-zinc-300 bg-zinc-100 dark:bg-zinc-800 hover:bg-zinc-200 dark:hover:bg-zinc-700 rounded-lg transition-colors disabled:opacity-50"
          >
            {loadingMore ? (
              <span className="flex items-center gap-2">
                <svg className="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                </svg>
                Loading...
              </span>
            ) : (
              `Load more (${results.length} of ${total})`
            )}
          </button>
        </div>
      )}

      {/* Initial state -- before any search */}
      {!hasSearched && (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-14 h-14 rounded-full bg-zinc-100 dark:bg-zinc-800 flex items-center justify-center mb-4">
            <svg
              className="w-7 h-7 text-zinc-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5m8.25 3v6.75m0 0l-3-3m3 3l3-3M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z"
              />
            </svg>
          </div>
          <h3 className="text-lg font-medium text-zinc-900 dark:text-zinc-100 mb-1">
            Search the archive
          </h3>
          <p className="text-sm text-zinc-500 dark:text-zinc-400">
            Find any article by keyword, source, date, region, or tag.
          </p>
        </div>
      )}
    </div>
  );
}
