import { useState, useEffect, useCallback, useRef } from 'react';
import hotkeys from 'hotkeys-js';
import { api, type Article, type Source } from '../lib/api';
import ArticleCard from './ArticleCard';
import NotesPanel from './NotesPanel';
import { timeAgo, formatDate } from '../lib/utils';

const TAG_OPTIONS = [
  'politics', 'economy', 'health', 'education', 'infrastructure',
  'environment', 'crime', 'grants', 'federal', 'legislation',
  'government', 'technology', 'culture', 'sports',
];

const REGION_OPTIONS = ['PR', 'Federal', 'Grants', 'Local'];

interface UndoAction {
  article: Article;
  action: 'save' | 'trash';
  timer: ReturnType<typeof setTimeout>;
}

function SkeletonCard() {
  return (
    <div className="p-5 rounded-sm border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 animate-pulse">
      <div className="flex items-center gap-2 mb-3">
        <div className="h-3 w-20 bg-zinc-200 dark:bg-zinc-800 rounded" />
        <div className="h-3 w-12 bg-zinc-200 dark:bg-zinc-800 rounded" />
      </div>
      <div className="h-5 w-3/4 bg-zinc-200 dark:bg-zinc-800 rounded mb-2" />
      <div className="h-4 w-full bg-zinc-200 dark:bg-zinc-800 rounded mb-1" />
      <div className="h-4 w-2/3 bg-zinc-200 dark:bg-zinc-800 rounded mb-3" />
      <div className="flex gap-2">
        <div className="h-7 w-16 bg-zinc-200 dark:bg-zinc-800 rounded" />
        <div className="h-7 w-16 bg-zinc-200 dark:bg-zinc-800 rounded" />
      </div>
    </div>
  );
}

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

/** Quick Add Source modal */
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
    if (!url.trim()) { setError('URL is required'); return; }
    setSubmitting(true); setError(''); setResult(null);
    try {
      const data = await api.quickCreateSource(url.trim(), region);
      setResult({ message: data.message, detected: data.detected });
      onCreated();
      setTimeout(() => onClose(), 2500);
    } catch (err: any) {
      setError(err.message || 'Failed to add source');
    } finally { setSubmitting(false); }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={onClose} />
      <div className="relative w-full max-w-md bg-white dark:bg-zinc-900 rounded-xl border border-zinc-200 dark:border-zinc-800 shadow-xl p-6 animate-fade-in">
        <h2 className="text-sm font-bold uppercase tracking-wider text-zinc-900 dark:text-zinc-100 mb-1">Add Source</h2>
        <p className="text-xs text-zinc-400 dark:text-zinc-500 mb-4">Paste a website or RSS feed URL. We'll auto-detect the feed.</p>
        {error && <div className="mb-3 p-2 text-xs rounded-lg bg-red-500/10 text-red-600 dark:text-red-400 border border-red-500/20">{error}</div>}
        {result && (
          <div className={`mb-3 p-2 text-xs rounded-lg border ${result.detected ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20' : 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20'}`}>{result.message}</div>
        )}
        <form onSubmit={handleSubmit} className="space-y-3">
          <input ref={inputRef} type="url" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://example.com or https://example.com/feed/"
            className="w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700 rounded-lg text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 focus:outline-none focus:ring-2 focus:ring-indigo-500" />
          <select value={region} onChange={(e) => setRegion(e.target.value)}
            className="w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700 rounded-lg text-zinc-900 dark:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-indigo-500">
            {REGION_OPTIONS.map((r) => <option key={r} value={r}>{r}</option>)}
          </select>
          <div className="flex items-center justify-end gap-3 pt-1">
            <button type="button" onClick={onClose} className="px-3 py-1.5 text-sm text-zinc-500 hover:text-zinc-700 dark:hover:text-zinc-300 transition-colors">Cancel</button>
            <button type="submit" disabled={submitting || !!result} className="px-4 py-1.5 text-sm font-medium text-white bg-indigo-500 hover:bg-indigo-600 rounded-lg transition-colors disabled:opacity-50">{submitting ? 'Detecting...' : 'Add Source'}</button>
          </div>
        </form>
      </div>
    </div>
  );
}

/** Collect URL modal */
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
    if (!url.trim()) { setError('URL is required'); return; }
    setSubmitting(true); setError('');
    try {
      await api.collectItem(url.trim(), title.trim(), region);
      onCollected(); onClose();
    } catch (err: any) { setError(err.message || 'Failed to collect article'); } finally { setSubmitting(false); }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={onClose} />
      <div className="relative w-full max-w-md bg-white dark:bg-zinc-900 rounded-xl border border-zinc-200 dark:border-zinc-800 shadow-xl p-6">
        <h2 className="text-sm font-bold uppercase tracking-wider text-zinc-900 dark:text-zinc-100 mb-4">Collect URL</h2>
        {error && <div className="mb-3 p-2 text-xs rounded-lg bg-red-500/10 text-red-600 dark:text-red-400 border border-red-500/20">{error}</div>}
        <form onSubmit={handleSubmit} className="space-y-3">
          <input ref={inputRef} type="url" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://example.com/article"
            className="w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700 rounded-lg text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 focus:outline-none focus:ring-2 focus:ring-indigo-500" />
          <input type="text" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Title (optional)"
            className="w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700 rounded-lg text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 focus:outline-none focus:ring-2 focus:ring-indigo-500" />
          <select value={region} onChange={(e) => setRegion(e.target.value)}
            className="w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700 rounded-lg text-zinc-900 dark:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-indigo-500">
            {REGION_OPTIONS.map((r) => <option key={r} value={r}>{r}</option>)}
          </select>
          <div className="flex items-center justify-end gap-3 pt-1">
            <button type="button" onClick={onClose} className="px-3 py-1.5 text-sm text-zinc-500 hover:text-zinc-700 dark:hover:text-zinc-300 transition-colors">Cancel</button>
            <button type="submit" disabled={submitting} className="px-4 py-1.5 text-sm font-medium text-white bg-indigo-500 hover:bg-indigo-600 rounded-lg transition-colors disabled:opacity-50">{submitting ? 'Collecting...' : 'Collect'}</button>
          </div>
        </form>
      </div>
    </div>
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

  // Filters
  const [activeTag, setActiveTag] = useState('');
  const [sourceFilter, setSourceFilter] = useState('');
  const [regionFilter, setRegionFilter] = useState('');

  const fetchItems = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.getItems('inbox');
      setArticles(data.items || []);
      const badge = document.getElementById('inbox-count');
      if (badge) { badge.textContent = `${data.items?.length ?? 0} new`; }
    } catch (err: any) { setError(err.message); } finally { setLoading(false); }
  }, []);

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
    const timer = setTimeout(() => { setUndoStack((prev) => prev.filter((u) => u.article.id !== id)); }, 10000);
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
    const timer = setTimeout(() => { setUndoStack((prev) => prev.filter((u) => u.article.id !== id)); }, 10000);
    setUndoStack((prev) => [...prev, { article, action: 'trash', timer }]);
    try { await api.trashItem(id); } catch {
      setArticles((prev) => [...prev, article].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()));
      setUndoStack((prev) => prev.filter((u) => u.article.id !== id)); clearTimeout(timer);
    }
  }, [articles, expandedId]);

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
    return (<div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">{Array.from({length:8}).map((_,i) => <SkeletonCard key={i} />)}</div>);
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <div className="w-12 h-12 rounded-full bg-red-500/10 flex items-center justify-center mb-4">
          <svg className="w-6 h-6 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" /></svg>
        </div>
        <p className="text-sm text-zinc-500 dark:text-zinc-400 mb-3">{error}</p>
        <button onClick={fetchItems} className="px-4 py-2 text-sm font-medium text-indigo-500 bg-indigo-500/10 hover:bg-indigo-500/20 rounded-lg transition-colors">Try again</button>
      </div>
    );
  }

  if (articles.length === 0) {
    return (
      <>
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-14 h-14 rounded-full bg-emerald-500/10 flex items-center justify-center mb-4">
            <svg className="w-7 h-7 text-emerald-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
          </div>
          <h3 className="text-lg font-bold text-zinc-900 dark:text-zinc-100 mb-1">All caught up!</h3>
          <p className="text-sm text-zinc-500 dark:text-zinc-400 mb-5">No new articles. Add sources or fetch news to get started.</p>
          <div className="flex gap-3">
            <button onClick={handleFetchNews} disabled={ingesting} className="inline-flex items-center gap-2 px-5 py-2.5 text-sm font-semibold text-white bg-indigo-500 hover:bg-indigo-600 rounded-lg transition-colors disabled:opacity-50">
              {ingesting ? <><svg className="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" /></svg>Fetching...</> : 'Fetch News'}
            </button>
            <button onClick={() => setShowAddSource(true)} className="px-4 py-2.5 text-sm font-medium text-zinc-600 dark:text-zinc-300 bg-zinc-100 dark:bg-zinc-800 hover:bg-zinc-200 dark:hover:bg-zinc-700 rounded-lg transition-colors">Add Source</button>
          </div>
        </div>
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
    if (regionFilter && a.region !== regionFilter) return false;
    return true;
  });

  const hasActiveFilters = activeTag || sourceFilter || regionFilter;
  const expandedArticle = expandedId ? filteredArticles.find((a) => a.id === expandedId) : null;
  const activeSources = sources.filter((s) => s.active);

  return (
    <>
      {/* ─── TOOLBAR ─── */}
      <div className="flex items-center gap-2 mb-4">
        {/* Search */}
        <div className="relative flex-1 max-w-sm">
          <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-zinc-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
          <input type="text" value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} placeholder="Search inbox..."
            className="w-full pl-10 pr-8 py-2 text-sm bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 rounded-lg text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 dark:placeholder-zinc-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 transition-colors" />
          {searchQuery && <button onClick={() => setSearchQuery('')} className="absolute right-2.5 top-1/2 -translate-y-1/2 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300"><svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg></button>}
        </div>

        <div className="flex items-center gap-1.5 ml-auto">
          {/* Fetch News */}
          <button onClick={handleFetchNews} disabled={ingesting}
            className="inline-flex items-center gap-1.5 px-3 py-2 text-xs font-semibold uppercase tracking-wider text-white bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 rounded-lg transition-colors"
            title="Fetch new articles from all sources">
            {ingesting ? <svg className="w-3.5 h-3.5 animate-spin" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" /></svg>
              : <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182" /></svg>}
            <span className="hidden sm:inline">{ingesting ? 'Fetching...' : 'Fetch'}</span>
          </button>

          {/* Add Source */}
          <button onClick={() => setShowAddSource(true)}
            className="inline-flex items-center gap-1.5 px-3 py-2 text-xs font-semibold uppercase tracking-wider text-white bg-indigo-500 hover:bg-indigo-600 rounded-lg transition-colors"
            title="Add Source URL">
            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" /></svg>
            <span className="hidden sm:inline">Source</span>
          </button>

          {/* Collect single URL */}
          <button onClick={() => setShowCollect(true)}
            className="inline-flex items-center gap-1.5 px-3 py-2 text-xs font-medium text-zinc-600 dark:text-zinc-300 bg-zinc-100 dark:bg-zinc-800 hover:bg-zinc-200 dark:hover:bg-zinc-700 rounded-lg transition-colors"
            title="Collect single article URL">
            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M13.19 8.688a4.5 4.5 0 011.242 7.244l-4.5 4.5a4.5 4.5 0 01-6.364-6.364l1.757-1.757m9.86-3.07a4.5 4.5 0 00-6.364-6.364L4.5 8.25" /></svg>
          </button>
        </div>
      </div>

      {/* ─── TOPIC TABS ─── */}
      <div className="flex flex-wrap gap-1.5 mb-4">
        <button onClick={() => setActiveTag('')}
          className={`px-3 py-1.5 text-xs font-bold uppercase tracking-wider rounded-sm transition-colors ${activeTag === '' ? 'bg-zinc-900 dark:bg-zinc-100 text-white dark:text-zinc-900' : 'bg-zinc-100 dark:bg-zinc-800 text-zinc-500 dark:text-zinc-400 hover:bg-zinc-200 dark:hover:bg-zinc-700'}`}>
          All <span className="font-normal ml-1 opacity-60">{articles.length}</span>
        </button>
        {TAG_OPTIONS.filter((t) => tagCounts[t]).map((tag) => (
          <button key={tag} onClick={() => setActiveTag(activeTag === tag ? '' : tag)}
            className={`px-3 py-1.5 text-xs font-bold uppercase tracking-wider rounded-sm transition-colors ${activeTag === tag ? 'bg-indigo-500 text-white' : 'bg-zinc-100 dark:bg-zinc-800 text-zinc-500 dark:text-zinc-400 hover:bg-zinc-200 dark:hover:bg-zinc-700'}`}>
            {tag} <span className="font-normal ml-1 opacity-60">{tagCounts[tag]}</span>
          </button>
        ))}
      </div>

      {/* ─── FILTER ROW ─── */}
      <div className="flex items-center gap-3 mb-4 pb-4 border-b border-zinc-100 dark:border-zinc-800">
        {/* Source filter */}
        <select value={sourceFilter} onChange={(e) => setSourceFilter(e.target.value)}
          className="px-2.5 py-1.5 text-xs bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 rounded-lg text-zinc-600 dark:text-zinc-400 focus:outline-none focus:ring-2 focus:ring-indigo-500">
          <option value="">All Sources ({uniqueSources.length})</option>
          {uniqueSources.map((s) => <option key={s} value={s}>{s}</option>)}
        </select>

        {/* Region filter */}
        <select value={regionFilter} onChange={(e) => setRegionFilter(e.target.value)}
          className="px-2.5 py-1.5 text-xs bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 rounded-lg text-zinc-600 dark:text-zinc-400 focus:outline-none focus:ring-2 focus:ring-indigo-500">
          <option value="">All Regions</option>
          {REGION_OPTIONS.map((r) => <option key={r} value={r}>{r}</option>)}
        </select>

        {hasActiveFilters && (
          <button onClick={() => { setActiveTag(''); setSourceFilter(''); setRegionFilter(''); }}
            className="text-xs text-indigo-500 hover:text-indigo-400 font-medium transition-colors">Clear filters</button>
        )}

        {/* Right: article count + manage link */}
        <div className="ml-auto flex items-center gap-3">
          <span className="text-xs text-zinc-400 dark:text-zinc-500">
            {filteredArticles.length} article{filteredArticles.length !== 1 ? 's' : ''}
          </span>
          <a href="/settings" className="text-xs text-indigo-500 hover:text-indigo-400 font-medium transition-colors">
            Manage Sources ({activeSources.length})
          </a>
        </div>
      </div>

      {/* ─── ARTICLE GRID ─── */}
      <div ref={listRef} className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
        {filteredArticles.map((article, index) => (
          <div key={article.id} data-article-card>
            <ArticleCard article={article} selected={index === selectedIndex} onSave={handleSave} onTrash={handleTrash} onPin={handlePin} onClick={() => handleCardClick(index, article.id)} showActions />
          </div>
        ))}
      </div>

      {/* ─── SLIDE-IN READER PANEL ─── */}
      {expandedArticle && (
        <>
          <div className="fixed inset-0 z-40 bg-black/30 backdrop-blur-sm" onClick={() => setExpandedId(null)} />
          <div className="fixed inset-y-0 right-0 z-50 w-full sm:w-[500px] lg:w-[600px] bg-white dark:bg-zinc-900 border-l border-zinc-200 dark:border-zinc-800 shadow-2xl flex flex-col animate-slide-in-right">
            {/* Header */}
            <div className="flex items-center justify-between px-6 py-3 border-b border-zinc-200 dark:border-zinc-800 bg-zinc-900 dark:bg-zinc-950 shrink-0">
              <h2 className="text-sm font-bold text-white truncate pr-4">{expandedArticle.title}</h2>
              <button onClick={() => setExpandedId(null)} className="shrink-0 p-1 text-zinc-400 hover:text-white transition-colors rounded">
                <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
              </button>
            </div>

            <div className="flex-1 overflow-y-auto">
              {expandedArticle.image_url && (
                <div className="w-full max-h-72 overflow-hidden">
                  <img src={expandedArticle.image_url} alt="" className="w-full h-full object-cover" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).parentElement!.style.display = 'none'; }} />
                </div>
              )}

              <div className="p-6 space-y-5">
                {/* Triage actions */}
                <div className="flex items-center gap-2">
                  <button onClick={() => handleSave(expandedArticle.id)} className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-bold uppercase tracking-wider text-white bg-indigo-500 hover:bg-indigo-600 rounded-sm transition-colors">
                    <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M17.593 3.322c1.1.128 1.907 1.077 1.907 2.185V21L12 17.25 4.5 21V5.507c0-1.108.806-2.057 1.907-2.185a48.507 48.507 0 0111.186 0z" /></svg>Save
                  </button>
                  <button onClick={() => handleTrash(expandedArticle.id)} className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-bold uppercase tracking-wider text-zinc-500 hover:text-red-500 bg-zinc-100 dark:bg-zinc-800 hover:bg-red-50 dark:hover:bg-red-500/10 rounded-sm transition-colors">
                    <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" /></svg>Trash
                  </button>
                  <button onClick={() => handlePin(expandedArticle.id)} className={`inline-flex items-center gap-1 px-2 py-1.5 text-xs font-bold rounded-sm transition-colors ${expandedArticle.pinned ? 'text-yellow-500 bg-yellow-500/10' : 'text-zinc-400 bg-zinc-100 dark:bg-zinc-800 hover:text-yellow-500'}`}>
                    <svg className="w-3.5 h-3.5" fill={expandedArticle.pinned ? 'currentColor' : 'none'} viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M11.48 3.499a.562.562 0 011.04 0l2.125 5.111a.563.563 0 00.475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 00-.182.557l1.285 5.385a.562.562 0 01-.84.61l-4.725-2.885a.563.563 0 00-.586 0L6.982 20.54a.562.562 0 01-.84-.61l1.285-5.386a.562.562 0 00-.182-.557l-4.204-3.602a.563.563 0 01.321-.988l5.518-.442a.563.563 0 00.475-.345L11.48 3.5z" /></svg>{expandedArticle.pinned ? 'Unpin' : 'Pin'}
                  </button>
                </div>

                {expandedArticle.summary && expandedArticle.clean_text && expandedArticle.summary !== expandedArticle.clean_text.slice(0, expandedArticle.summary.length) && (
                  <div className="px-4 py-3 rounded-sm bg-indigo-50 dark:bg-indigo-500/10 border-l-4 border-indigo-500">
                    <h4 className="text-xs font-bold uppercase tracking-wider text-indigo-600 dark:text-indigo-400 mb-1.5">Summary</h4>
                    <p className="text-sm text-indigo-900 dark:text-indigo-200 leading-relaxed">{expandedArticle.summary}</p>
                  </div>
                )}
                {expandedArticle.summary && !expandedArticle.clean_text && (
                  <div><h4 className="text-xs font-bold uppercase tracking-wider text-zinc-400 dark:text-zinc-500 mb-3">Summary</h4><p className="text-[15px] text-zinc-700 dark:text-zinc-300 leading-[1.75]">{expandedArticle.summary}</p></div>
                )}
                {expandedArticle.clean_text && (
                  <div><h4 className="text-xs font-bold uppercase tracking-wider text-zinc-400 dark:text-zinc-500 mb-3">Full Article</h4><div className="max-w-prose"><ArticleBody text={expandedArticle.clean_text} /></div></div>
                )}
                {!expandedArticle.clean_text && !expandedArticle.summary && (<p className="text-sm text-zinc-400 dark:text-zinc-500 italic">No article content available.</p>)}

                {expandedArticle.tags && expandedArticle.tags.length > 0 && (
                  <div>
                    <h4 className="text-xs font-bold uppercase tracking-wider text-zinc-400 dark:text-zinc-500 mb-2">Tags</h4>
                    <div className="flex flex-wrap gap-1.5">
                      {expandedArticle.tags.map((tag) => (
                        <button key={tag} onClick={() => { setActiveTag(tag); setExpandedId(null); }}
                          className="inline-flex items-center px-2.5 py-1 text-xs font-medium rounded-sm bg-zinc-100 dark:bg-zinc-800 text-zinc-600 dark:text-zinc-400 hover:bg-indigo-100 dark:hover:bg-indigo-500/20 hover:text-indigo-600 dark:hover:text-indigo-400 transition-colors cursor-pointer">{tag}</button>
                      ))}
                    </div>
                  </div>
                )}

                <div className="flex items-center justify-between pt-3 border-t border-zinc-100 dark:border-zinc-800">
                  <div className="flex items-center gap-3 text-xs text-zinc-400 dark:text-zinc-500">
                    <span>{formatDate(expandedArticle.published_at || expandedArticle.created_at)}</span>
                    {expandedArticle.source && <><span className="text-zinc-300 dark:text-zinc-600">&middot;</span><span>{expandedArticle.source}</span></>}
                  </div>
                  <div className="flex items-center gap-2">
                    {expandedArticle.url && (
                      <a href={expandedArticle.url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-indigo-500 hover:text-indigo-400 bg-indigo-500/10 hover:bg-indigo-500/20 rounded-sm transition-colors">
                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M13.5 6H5.25A2.25 2.25 0 003 8.25v10.5A2.25 2.25 0 005.25 21h10.5A2.25 2.25 0 0018 18.75V10.5m-10.5 6L21 3m0 0h-5.25M21 3v5.25" /></svg>Original
                      </a>
                    )}
                  </div>
                </div>

                <NotesPanel articleId={expandedArticle.id} />
              </div>
            </div>
          </div>
        </>
      )}

      {/* Modals */}
      {showAddSource && <QuickAddSourceModal onClose={() => setShowAddSource(false)} onCreated={fetchSources} />}
      {showCollect && <CollectModal onClose={() => setShowCollect(false)} onCollected={fetchItems} />}

      {/* Undo toasts */}
      {undoStack.length > 0 && (
        <div className="fixed bottom-6 right-6 z-50 space-y-2">
          {undoStack.map((entry) => (
            <div key={entry.article.id} className="toast-enter flex items-center gap-3 px-4 py-3 rounded-lg bg-zinc-800 dark:bg-zinc-100 text-zinc-100 dark:text-zinc-900 shadow-lg border border-zinc-700 dark:border-zinc-200">
              <span className="text-sm">{entry.action === 'save' ? 'Saved' : 'Trashed'} <span className="font-medium">{entry.article.title.length > 30 ? entry.article.title.slice(0, 30) + '...' : entry.article.title}</span></span>
              <button onClick={() => handleUndo(entry.article.id)} className="text-sm font-semibold text-indigo-400 dark:text-indigo-600 hover:text-indigo-300 dark:hover:text-indigo-500 transition-colors">Undo</button>
            </div>
          ))}
        </div>
      )}
    </>
  );
}
