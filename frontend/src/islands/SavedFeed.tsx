import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { api, type Article } from '../lib/api';
import { timeAgo, regionColor, formatDate } from '../lib/utils';
import NotesPanel from './NotesPanel';
import { useNearEnd } from '../lib/useNearEnd';
import { Icon, Spinner, EmptyState, tagLabel } from './ui';

const PAGE = 30;

function SkeletonCard() {
  return (
    <div className="p-5 rounded-[28px] bg-surface-container-low animate-pulse">
      <div className="h-3 w-24 bg-surface-container-high rounded-full mb-4" />
      <div className="h-5 w-3/4 bg-surface-container-high rounded-full mb-2" />
      <div className="h-4 w-full bg-surface-container-high rounded-full mb-1" />
      <div className="h-4 w-2/3 bg-surface-container-high rounded-full" />
    </div>
  );
}

/** Texto de la noticia en párrafos (se parte en saltos dobles). */
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

export default function SavedFeed() {
  const [articles, setArticles] = useState<Article[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [filter, setFilter] = useState('');
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const countRef = useRef(0);
  countRef.current = articles.length;

  const loadMore = useCallback(async () => {
    if (loadingMore || !hasMore) return;
    setLoadingMore(true);
    try {
      const data = await api.getItems('saved', PAGE, countRef.current);
      const next = data.items || [];
      setArticles((prev) => {
        const seen = new Set(prev.map((a) => a.id));
        return [...prev, ...next.filter((a) => !seen.has(a.id))];
      });
      setHasMore(next.length >= PAGE);
    } catch { /* se queda el botón para reintentar */ } finally { setLoadingMore(false); }
  }, [loadingMore, hasMore]);

  const sentinelRef = useNearEnd(loadMore, hasMore && !loading && !loadingMore);

  const fetchItems = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.getItems('saved', PAGE, 0);
      setArticles(data.items || []);
      setHasMore((data.items?.length ?? 0) >= PAGE);
      setError('');
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

  const handleTrash = useCallback(async (id: string) => {
    try { await api.trashItem(id); } catch { return; }
    setArticles((prev) => prev.filter((a) => a.id !== id));
    setExpandedId(null);
  }, []);

  const expandedArticle = expandedId ? sorted.find((a) => a.id === expandedId) : null;

  if (loading) {
    return (
      <div className="space-y-4">
        <div className="h-12 bg-surface-container-high rounded-2xl animate-pulse" />
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          <SkeletonCard />
          <SkeletonCard />
          <SkeletonCard />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <EmptyState icon="alert" text={error}>
        <button type="button" onClick={fetchItems} className="btn btn-filled">Intentar de nuevo</button>
      </EmptyState>
    );
  }

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative flex-1 min-w-[220px] max-w-md">
          <span className="absolute left-4 top-1/2 -translate-y-1/2 text-on-surface-variant pointer-events-none"><Icon name="search" className="w-5 h-5" /></span>
          <input
            type="search"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Buscar en guardados"
            aria-label="Buscar en guardados"
            className="field pl-12"
          />
        </div>
        <p className="text-sm font-semibold text-on-surface-variant">
          {sorted.length} {sorted.length === 1 ? 'noticia' : 'noticias'}
          {filter && ` con «${filter}»`}
        </p>
      </div>

      {sorted.length === 0 && !filter && (
        <EmptyState icon="save" text="Aquí van las noticias que guardes. Empieza desde el Inbox.">
          <a href="/inbox" className="btn btn-filled">Ir al Inbox</a>
        </EmptyState>
      )}

      {sorted.length === 0 && filter && (
        <EmptyState icon="search" text="Nada coincide. Prueba con otra palabra." />
      )}

      <div className="stagger grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        {sorted.map((article) => (
          <button
            type="button"
            key={article.id}
            onClick={() => setExpandedId(expandedId === article.id ? null : article.id)}
            className={`text-left flex flex-col h-full overflow-hidden !min-h-0 rounded-[28px] hover:rounded-[36px] transition-[border-radius,background-color,transform] duration-500 ease-[var(--spring)] active:scale-[0.985]
              ${expandedId === article.id ? 'ring-[3px] ring-primary bg-surface-container' : 'bg-surface-container-low hover:bg-surface-container'}`}
          >
            {article.image_url ? (
              <div className="w-[calc(100%-24px)] mx-3 mt-3 h-36 overflow-hidden rounded-[20px] bg-surface-container-high">
                <img src={article.image_url} alt="" className="w-full h-full object-cover" loading="lazy"
                  onError={(e) => { (e.target as HTMLImageElement).parentElement!.style.display = 'none'; }} />
              </div>
            ) : null}
            <div className="flex-1 flex flex-col p-5 gap-2 w-full">
              <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-1.5 text-xs text-on-surface-variant min-w-0">
                  {article.pinned && <span className="text-tertiary shrink-0"><Icon name="pin" className="w-4 h-4" fill /></span>}
                  <span className="font-semibold truncate">{article.source}</span>
                  <span aria-hidden="true">&middot;</span>
                  <span className="shrink-0">{timeAgo(article.published_at || article.created_at)}</span>
                </div>
                <span className={`tag shrink-0 uppercase tracking-wider ${regionColor(article.region).bg} ${regionColor(article.region).text}`}>{article.region}</span>
              </div>
              <h3 className="text-lg font-extrabold text-on-surface leading-tight line-clamp-3 tracking-tight">{article.title}</h3>
              {article.summary && <p className="text-sm text-on-surface-variant leading-relaxed line-clamp-2 flex-1">{article.summary}</p>}
              {article.tags && article.tags.length > 0 && (
                <div className="flex flex-wrap gap-1.5">
                  {article.tags.slice(0, 3).map((tag) => <span key={tag} className="tag">{tagLabel(tag)}</span>)}
                  {article.tags.length > 3 && <span className="tag">+{article.tags.length - 3}</span>}
                </div>
              )}
            </div>
          </button>
        ))}
      </div>

      {hasMore && (
        <div ref={sentinelRef} className="flex justify-center pt-2">
          <button type="button" onClick={loadMore} disabled={loadingMore} className="btn btn-tonal">
            {loadingMore ? (<><Spinner className="w-5 h-5" />Cargando...</>) : 'Ver más'}
          </button>
        </div>
      )}

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
                  <button type="button" onClick={() => handlePin(expandedArticle.id)} className={`btn btn-sm ${expandedArticle.pinned ? 'btn-accent' : 'btn-soft'}`} aria-pressed={!!expandedArticle.pinned}>
                    <Icon name="pin" className="w-5 h-5" fill={!!expandedArticle.pinned} />{expandedArticle.pinned ? 'Fijado' : 'Fijar'}
                  </button>
                  <button type="button" onClick={() => handleTrash(expandedArticle.id)} className="btn btn-soft btn-sm">
                    <Icon name="trash" className="w-5 h-5" />Borrar
                  </button>
                  <a href={api.exportArticle(expandedArticle.id)} className="btn btn-soft btn-sm">
                    <Icon name="open" className="w-5 h-5" />Exportar ZIP
                  </a>
                  {expandedArticle.url && (
                    <a href={expandedArticle.url} target="_blank" rel="noopener noreferrer" className="btn btn-tonal btn-sm">
                      <Icon name="open" className="w-5 h-5" />Abrir original
                    </a>
                  )}
                </div>

                {expandedArticle.summary && expandedArticle.clean_text && expandedArticle.summary !== expandedArticle.clean_text.slice(0, expandedArticle.summary.length) && (
                  <div className="card-hi px-5 py-4">
                    <h4 className="title-md mb-1.5">Resumen</h4>
                    <p className="text-sm leading-relaxed">{expandedArticle.summary}</p>
                  </div>
                )}
                {expandedArticle.summary && !expandedArticle.clean_text && (
                  <p className="text-base text-on-surface leading-[1.75]">{expandedArticle.summary}</p>
                )}
                {expandedArticle.clean_text && (
                  <div>
                    <h4 className="title-md mb-3">Noticia completa</h4>
                    <div className="max-w-prose"><ArticleBody text={expandedArticle.clean_text} /></div>
                  </div>
                )}
                {!expandedArticle.clean_text && !expandedArticle.summary && (
                  <p className="text-sm text-on-surface-variant">Esta noticia no trae texto. Abre el original.</p>
                )}
                <p className="text-sm font-semibold text-on-surface-variant">{formatDate(expandedArticle.published_at || expandedArticle.created_at)}</p>
                <NotesPanel articleId={expandedArticle.id} />
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
