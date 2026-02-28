import { useState, useEffect, useCallback, useMemo } from 'react';
import { api, type Article } from '../lib/api';
import { timeAgo, regionColor, formatDate } from '../lib/utils';
import NotesPanel from './NotesPanel';

function SkeletonCard() {
  return (
    <div className="p-5 rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 animate-pulse">
      <div className="flex items-center gap-2 mb-3">
        <div className="h-3 w-20 bg-zinc-200 dark:bg-zinc-800 rounded" />
        <div className="h-3 w-12 bg-zinc-200 dark:bg-zinc-800 rounded" />
      </div>
      <div className="h-5 w-3/4 bg-zinc-200 dark:bg-zinc-800 rounded mb-2" />
      <div className="h-4 w-full bg-zinc-200 dark:bg-zinc-800 rounded mb-1" />
      <div className="h-4 w-2/3 bg-zinc-200 dark:bg-zinc-800 rounded" />
    </div>
  );
}

/** Renders article body text as paragraphs, splitting on double newlines */
function ArticleBody({ text }: { text: string }) {
  if (!text) return null;
  const paragraphs = text.split(/\n\n+/).filter((p) => p.trim());
  return (
    <div className="space-y-4">
      {paragraphs.map((p, i) => (
        <p key={i} className="text-[15px] text-zinc-700 dark:text-zinc-300 leading-[1.75]">
          {p.trim()}
        </p>
      ))}
    </div>
  );
}

export default function SavedFeed() {
  const [articles, setArticles] = useState<Article[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [filter, setFilter] = useState('');
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const fetchItems = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.getItems('saved');
      setArticles(data.items || []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchItems();
  }, [fetchItems]);

  // Close panel on ESC
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setExpandedId(null);
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  // Sort: pinned first, then by date
  const sorted = useMemo(() => {
    let filtered = articles;
    if (filter.trim()) {
      const q = filter.toLowerCase();
      filtered = articles.filter(
        (a) =>
          a.title.toLowerCase().includes(q) ||
          a.summary?.toLowerCase().includes(q) ||
          a.source?.toLowerCase().includes(q) ||
          a.tags?.some((t) => t.toLowerCase().includes(q))
      );
    }
    return [...filtered].sort((a, b) => {
      if (a.pinned && !b.pinned) return -1;
      if (!a.pinned && b.pinned) return 1;
      return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
    });
  }, [articles, filter]);

  const handlePin = useCallback(async (id: string) => {
    setArticles((prev) =>
      prev.map((a) => (a.id === id ? { ...a, pinned: !a.pinned } : a))
    );
    try {
      await api.pinItem(id);
    } catch {
      setArticles((prev) =>
        prev.map((a) => (a.id === id ? { ...a, pinned: !a.pinned } : a))
      );
    }
  }, []);

  const expandedArticle = expandedId ? sorted.find((a) => a.id === expandedId) : null;

  if (loading) {
    return (
      <div className="space-y-3">
        <div className="h-10 bg-zinc-200 dark:bg-zinc-800 rounded-lg animate-pulse" />
        <SkeletonCard />
        <SkeletonCard />
        <SkeletonCard />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <p className="text-sm text-zinc-500 dark:text-zinc-400 mb-3">{error}</p>
        <button
          onClick={fetchItems}
          className="px-4 py-2 text-sm font-medium text-indigo-500 bg-indigo-500/10 hover:bg-indigo-500/20 rounded-lg transition-colors"
        >
          Try again
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Filter input */}
      <div className="relative">
        <svg
          className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-zinc-400"
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
          type="text"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Filter saved articles..."
          className="w-full pl-10 pr-4 py-2.5 text-sm bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 rounded-xl text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 dark:placeholder-zinc-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent transition-colors"
        />
      </div>

      {/* Count */}
      <p className="text-xs text-zinc-400 dark:text-zinc-500">
        {sorted.length} article{sorted.length !== 1 ? 's' : ''}
        {filter && ` matching "${filter}"`}
      </p>

      {/* Empty state */}
      {sorted.length === 0 && !filter && (
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
                d="M17.593 3.322c1.1.128 1.907 1.077 1.907 2.185V21L12 17.25 4.5 21V5.507c0-1.108.806-2.057 1.907-2.185a48.507 48.507 0 0111.186 0z"
              />
            </svg>
          </div>
          <h3 className="text-lg font-medium text-zinc-900 dark:text-zinc-100 mb-1">
            No saved articles
          </h3>
          <p className="text-sm text-zinc-500 dark:text-zinc-400">
            Save articles from your inbox to see them here.
          </p>
        </div>
      )}

      {sorted.length === 0 && filter && (
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <p className="text-sm text-zinc-500 dark:text-zinc-400">
            No articles match your filter.
          </p>
        </div>
      )}

      {/* Article grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
        {sorted.map((article) => (
          <div
            key={article.id}
            className="cursor-pointer"
            onClick={() => setExpandedId(expandedId === article.id ? null : article.id)}
          >
            <div
              className={`
                rounded-xl border transition-all duration-150 flex flex-col h-full overflow-hidden
                ${expandedId === article.id
                  ? 'ring-2 ring-indigo-500 border-indigo-500/30 bg-zinc-50 dark:bg-zinc-800/80'
                  : 'border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 hover:border-zinc-300 dark:hover:border-zinc-700'}
              `}
            >
              <div className="w-full h-36 overflow-hidden bg-zinc-100 dark:bg-zinc-800">
                {article.image_url ? (
                  <img src={article.image_url} alt="" className="w-full h-full object-cover" loading="lazy"
                    onError={(e) => {
                      const parent = (e.target as HTMLImageElement).parentElement!;
                      parent.innerHTML = '<div class="w-full h-full flex items-center justify-center bg-gradient-to-br from-zinc-100 to-zinc-200 dark:from-zinc-800 dark:to-zinc-700"><svg class="w-10 h-10 text-zinc-300 dark:text-zinc-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 7.5h1.5m-1.5 3h1.5m-7.5 3h7.5m-7.5 3h7.5m3-9h3.375c.621 0 1.125.504 1.125 1.125V18a2.25 2.25 0 01-2.25 2.25M16.5 7.5V18a2.25 2.25 0 002.25 2.25M16.5 7.5V4.875c0-.621-.504-1.125-1.125-1.125H4.125C3.504 3.75 3 4.254 3 4.875V18a2.25 2.25 0 002.25 2.25h13.5M6 7.5h3v3H6V7.5z" /></svg></div>';
                    }} />
                ) : (
                  <div className="w-full h-full flex items-center justify-center bg-gradient-to-br from-zinc-100 to-zinc-200 dark:from-zinc-800 dark:to-zinc-700">
                    <svg className="w-10 h-10 text-zinc-300 dark:text-zinc-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M12 7.5h1.5m-1.5 3h1.5m-7.5 3h7.5m-7.5 3h7.5m3-9h3.375c.621 0 1.125.504 1.125 1.125V18a2.25 2.25 0 01-2.25 2.25M16.5 7.5V18a2.25 2.25 0 002.25 2.25M16.5 7.5V4.875c0-.621-.504-1.125-1.125-1.125H4.125C3.504 3.75 3 4.254 3 4.875V18a2.25 2.25 0 002.25 2.25h13.5M6 7.5h3v3H6V7.5z" />
                    </svg>
                  </div>
                )}
              </div>
              <div className="flex-1 flex flex-col p-4">
                <div className="flex items-center justify-between mb-1.5">
                  <div className="flex items-center gap-1.5 text-[11px] text-zinc-500 dark:text-zinc-400 min-w-0">
                    {article.pinned && (
                      <span className="text-yellow-500 shrink-0">
                        <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 24 24">
                          <path d="M11.48 3.499a.562.562 0 011.04 0l2.125 5.111a.563.563 0 00.475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 00-.182.557l1.285 5.385a.562.562 0 01-.84.61l-4.725-2.885a.563.563 0 00-.586 0L6.982 20.54a.562.562 0 01-.84-.61l1.285-5.386a.562.562 0 00-.182-.557l-4.204-3.602a.563.563 0 01.321-.988l5.518-.442a.563.563 0 00.475-.345L11.48 3.5z" />
                        </svg>
                      </span>
                    )}
                    <span className="font-medium truncate">{article.source}</span>
                    <span className="text-zinc-300 dark:text-zinc-600 shrink-0">&middot;</span>
                    <span className="shrink-0">{timeAgo(article.published_at || article.created_at)}</span>
                  </div>
                  <span className={`inline-flex items-center px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wider rounded-full shrink-0 ${regionColor(article.region).bg} ${regionColor(article.region).text}`}>
                    {article.region}
                  </span>
                </div>
                <h3 className="text-sm font-semibold text-zinc-900 dark:text-zinc-100 leading-snug line-clamp-2 mb-1">
                  {article.title}
                </h3>
                {article.summary && (
                  <p className="text-xs text-zinc-500 dark:text-zinc-400 leading-relaxed line-clamp-2 mb-2 flex-1">
                    {article.summary}
                  </p>
                )}
                {article.tags && article.tags.length > 0 && (
                  <div className="flex flex-wrap gap-1 mb-2">
                    {article.tags.slice(0, 3).map((tag) => (
                      <span key={tag} className="inline-flex items-center px-1.5 py-0.5 text-[10px] font-medium rounded bg-zinc-100 dark:bg-zinc-800 text-zinc-500 dark:text-zinc-400">
                        {tag}
                      </span>
                    ))}
                    {article.tags.length > 3 && <span className="text-[10px] text-zinc-400">+{article.tags.length - 3}</span>}
                  </div>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Slide-in reader panel */}
      {expandedArticle && (
        <>
          {/* Backdrop */}
          <div
            className="fixed inset-0 z-40 bg-black/30 backdrop-blur-sm"
            onClick={() => setExpandedId(null)}
          />
          {/* Panel */}
          <div className="fixed inset-y-0 right-0 z-50 w-full sm:w-[500px] lg:w-[600px] bg-white dark:bg-zinc-900 border-l border-zinc-200 dark:border-zinc-800 shadow-2xl flex flex-col animate-slide-in-right">
            {/* Header */}
            <div className="flex items-center justify-between px-6 py-3 border-b border-zinc-200 dark:border-zinc-800 bg-zinc-50 dark:bg-zinc-800/50 shrink-0">
              <h2 className="text-sm font-semibold text-zinc-900 dark:text-zinc-100 truncate pr-4">{expandedArticle.title}</h2>
              <button onClick={() => setExpandedId(null)} className="shrink-0 p-1 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300 transition-colors rounded">
                <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            {/* Scrollable content */}
            <div className="flex-1 overflow-y-auto">
              <div className="w-full max-h-72 overflow-hidden">
                {expandedArticle.image_url ? (
                  <img src={expandedArticle.image_url} alt="" className="w-full h-full object-cover" loading="lazy"
                    onError={(e) => {
                      const parent = (e.target as HTMLImageElement).parentElement!;
                      parent.innerHTML = '<div class="w-full h-48 flex items-center justify-center bg-gradient-to-br from-zinc-100 to-zinc-200 dark:from-zinc-800 dark:to-zinc-700"><svg class="w-16 h-16 text-zinc-300 dark:text-zinc-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 7.5h1.5m-1.5 3h1.5m-7.5 3h7.5m-7.5 3h7.5m3-9h3.375c.621 0 1.125.504 1.125 1.125V18a2.25 2.25 0 01-2.25 2.25M16.5 7.5V18a2.25 2.25 0 002.25 2.25M16.5 7.5V4.875c0-.621-.504-1.125-1.125-1.125H4.125C3.504 3.75 3 4.254 3 4.875V18a2.25 2.25 0 002.25 2.25h13.5M6 7.5h3v3H6V7.5z" /></svg></div>';
                    }} />
                ) : (
                  <div className="w-full h-48 flex items-center justify-center bg-gradient-to-br from-zinc-100 to-zinc-200 dark:from-zinc-800 dark:to-zinc-700">
                    <svg className="w-16 h-16 text-zinc-300 dark:text-zinc-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M12 7.5h1.5m-1.5 3h1.5m-7.5 3h7.5m-7.5 3h7.5m3-9h3.375c.621 0 1.125.504 1.125 1.125V18a2.25 2.25 0 01-2.25 2.25M16.5 7.5V18a2.25 2.25 0 002.25 2.25M16.5 7.5V4.875c0-.621-.504-1.125-1.125-1.125H4.125C3.504 3.75 3 4.254 3 4.875V18a2.25 2.25 0 002.25 2.25h13.5M6 7.5h3v3H6V7.5z" />
                    </svg>
                  </div>
                )}
              </div>
              <div className="p-6 space-y-5">
                {expandedArticle.summary && expandedArticle.clean_text && expandedArticle.summary !== expandedArticle.clean_text.slice(0, expandedArticle.summary.length) && (
                  <div className="px-4 py-3 rounded-lg bg-indigo-50 dark:bg-indigo-500/10 border border-indigo-100 dark:border-indigo-500/20">
                    <h4 className="text-xs font-semibold uppercase tracking-wider text-indigo-600 dark:text-indigo-400 mb-1.5">Summary</h4>
                    <p className="text-sm text-indigo-900 dark:text-indigo-200 leading-relaxed">{expandedArticle.summary}</p>
                  </div>
                )}
                {expandedArticle.summary && !expandedArticle.clean_text && (
                  <p className="text-[15px] text-zinc-700 dark:text-zinc-300 leading-[1.75]">{expandedArticle.summary}</p>
                )}
                {expandedArticle.clean_text && (
                  <div>
                    <h4 className="text-xs font-medium uppercase tracking-wider text-zinc-400 dark:text-zinc-500 mb-3">Full Article</h4>
                    <div className="max-w-prose"><ArticleBody text={expandedArticle.clean_text} /></div>
                  </div>
                )}
                {!expandedArticle.clean_text && !expandedArticle.summary && (
                  <p className="text-sm text-zinc-400 dark:text-zinc-500 italic">No article content available.</p>
                )}
                <NotesPanel articleId={expandedArticle.id} />
                <div className="flex items-center justify-between pt-3 border-t border-zinc-100 dark:border-zinc-800">
                  <div className="flex items-center gap-3 text-xs text-zinc-400 dark:text-zinc-500">
                    <span>{formatDate(expandedArticle.published_at || expandedArticle.created_at)}</span>
                    {expandedArticle.url && (
                      <a href={expandedArticle.url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-indigo-500 hover:text-indigo-400 transition-colors">
                        <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                          <path strokeLinecap="round" strokeLinejoin="round" d="M13.5 6H5.25A2.25 2.25 0 003 8.25v10.5A2.25 2.25 0 005.25 21h10.5A2.25 2.25 0 0018 18.75V10.5m-10.5 6L21 3m0 0h-5.25M21 3v5.25" />
                        </svg>
                        Open original
                      </a>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <a href={api.exportArticle(expandedArticle.id)} className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-zinc-500 dark:text-zinc-400 bg-zinc-100 dark:bg-zinc-800 hover:bg-zinc-200 dark:hover:bg-zinc-700 rounded-lg transition-colors">
                      <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3" />
                      </svg>
                      Export ZIP
                    </a>
                    <button onClick={() => handlePin(expandedArticle.id)}
                      className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${
                        expandedArticle.pinned ? 'text-yellow-500 bg-yellow-500/10 hover:bg-yellow-500/20' : 'text-zinc-400 bg-zinc-100 dark:bg-zinc-800 hover:text-yellow-500 hover:bg-yellow-500/10'
                      }`}>
                      <svg className="w-3.5 h-3.5" fill={expandedArticle.pinned ? 'currentColor' : 'none'} viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M11.48 3.499a.562.562 0 011.04 0l2.125 5.111a.563.563 0 00.475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 00-.182.557l1.285 5.385a.562.562 0 01-.84.61l-4.725-2.885a.563.563 0 00-.586 0L6.982 20.54a.562.562 0 01-.84-.61l1.285-5.386a.562.562 0 00-.182-.557l-4.204-3.602a.563.563 0 01.321-.988l5.518-.442a.563.563 0 00.475-.345L11.48 3.5z" />
                      </svg>
                      {expandedArticle.pinned ? 'Unpin' : 'Pin'}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
