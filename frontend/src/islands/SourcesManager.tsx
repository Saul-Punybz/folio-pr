import { useState, useEffect, useCallback } from 'react';
import { api, type Source } from '../lib/api';
import { formatDate } from '../lib/utils';

interface SourceFormData {
  name: string;
  base_url: string;
  region: string;
  feed_type: string;
  feed_url: string;
  list_urls: string;
  link_selector: string;
  title_selector: string;
  body_selector: string;
  date_selector: string;
}

const emptyForm: SourceFormData = {
  name: '',
  base_url: '',
  region: 'PR',
  feed_type: 'rss',
  feed_url: '',
  list_urls: '',
  link_selector: '',
  title_selector: '',
  body_selector: '',
  date_selector: '',
};

function SourceFormModal({
  initial,
  onSubmit,
  onCancel,
  isEdit,
}: {
  initial: SourceFormData;
  onSubmit: (data: SourceFormData) => void;
  onCancel: () => void;
  isEdit: boolean;
}) {
  const [form, setForm] = useState<SourceFormData>(initial);
  const [error, setError] = useState('');

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');

    if (!form.name.trim()) {
      setError('Name is required');
      return;
    }
    if (!form.base_url.trim()) {
      setError('Base URL is required');
      return;
    }

    if (form.feed_type === 'rss' && !form.feed_url.trim()) {
      setError('Feed URL is required for RSS sources');
      return;
    }

    if (form.feed_type === 'scrape' && !form.list_urls.trim()) {
      setError('At least one List URL is required for scrape sources');
      return;
    }

    if (form.feed_type === 'scrape' && !form.link_selector.trim()) {
      setError('Link selector is required for scrape sources');
      return;
    }

    onSubmit(form);
  }

  const inputClass =
    'w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700 rounded-lg text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 dark:placeholder-zinc-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent transition-colors';

  const labelClass = 'block text-xs font-medium text-zinc-500 dark:text-zinc-400 mb-1';
  const hintClass = 'text-[10px] text-zinc-400 dark:text-zinc-500 mt-0.5';

  const isScrape = form.feed_type === 'scrape';

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/50 backdrop-blur-sm"
        onClick={onCancel}
      />

      {/* Modal */}
      <div className="relative w-full max-w-lg max-h-[90vh] overflow-y-auto bg-white dark:bg-zinc-900 rounded-xl border border-zinc-200 dark:border-zinc-800 shadow-xl p-6 animate-fade-in">
        <h2 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100 mb-4">
          {isEdit ? 'Edit Source' : 'Add Source'}
        </h2>

        {error && (
          <div className="mb-4 p-3 text-sm rounded-lg bg-red-500/10 text-red-600 dark:text-red-400 border border-red-500/20">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className={labelClass}>Name</label>
              <input
                type="text"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="El Nuevo Dia"
                className={inputClass}
                autoFocus
              />
            </div>
            <div>
              <label className={labelClass}>Region</label>
              <select
                value={form.region}
                onChange={(e) => setForm({ ...form, region: e.target.value })}
                className={inputClass}
              >
                <option value="PR">PR</option>
                <option value="Grants">Grants</option>
                <option value="Federal">Federal</option>
                <option value="Local">Local</option>
              </select>
            </div>
          </div>

          <div>
            <label className={labelClass}>Base URL</label>
            <input
              type="url"
              value={form.base_url}
              onChange={(e) => setForm({ ...form, base_url: e.target.value })}
              placeholder="https://www.elnuevodia.com"
              className={inputClass}
            />
          </div>

          {/* Feed Type — visual tabs */}
          <div>
            <label className={labelClass}>Feed Type</label>
            <div className="grid grid-cols-3 gap-2 mt-1">
              {[
                { value: 'rss', label: 'RSS', desc: 'Standard RSS/Atom feed' },
                { value: 'scrape', label: 'Scrape', desc: 'HTML page with CSS selectors' },
                { value: 'sitemap', label: 'Sitemap', desc: 'XML sitemap index' },
              ].map((opt) => (
                <button
                  key={opt.value}
                  type="button"
                  onClick={() => setForm({ ...form, feed_type: opt.value })}
                  className={`p-3 rounded-lg border text-left transition-all ${
                    form.feed_type === opt.value
                      ? 'border-indigo-500 bg-indigo-50 dark:bg-indigo-500/10 ring-1 ring-indigo-500'
                      : 'border-zinc-200 dark:border-zinc-700 bg-zinc-50 dark:bg-zinc-800 hover:border-zinc-300 dark:hover:border-zinc-600'
                  }`}
                >
                  <span className={`block text-sm font-semibold ${
                    form.feed_type === opt.value ? 'text-indigo-600 dark:text-indigo-400' : 'text-zinc-700 dark:text-zinc-300'
                  }`}>{opt.label}</span>
                  <span className="block text-[10px] text-zinc-400 dark:text-zinc-500 mt-0.5">{opt.desc}</span>
                </button>
              ))}
            </div>
          </div>

          {/* RSS / Sitemap URL — shown for rss and sitemap types */}
          {(form.feed_type === 'rss' || form.feed_type === 'sitemap') && (
            <div>
              <label className={labelClass}>{form.feed_type === 'rss' ? 'Feed URL' : 'Sitemap URL'}</label>
              <input
                type="url"
                value={form.feed_url}
                onChange={(e) => setForm({ ...form, feed_url: e.target.value })}
                placeholder={form.feed_type === 'rss' ? 'https://www.elnuevodia.com/arcio/rss/' : 'https://example.com/sitemap.xml'}
                className={inputClass}
              />
              <p className={hintClass}>{form.feed_type === 'rss' ? 'RSS or Atom feed URL' : 'XML sitemap URL'}</p>
            </div>
          )}

          {/* Scrape-specific fields — always visible when scrape selected */}
          {isScrape && (
            <div className="space-y-4 p-4 rounded-lg border-2 border-indigo-200 dark:border-indigo-500/30 bg-indigo-50/50 dark:bg-indigo-500/5">
              <h3 className="text-xs font-bold uppercase tracking-wider text-indigo-600 dark:text-indigo-400">
                Scrape Configuration
              </h3>

              <div>
                <label className={labelClass}>List URLs (one per line)</label>
                <textarea
                  value={form.list_urls}
                  onChange={(e) => setForm({ ...form, list_urls: e.target.value })}
                  placeholder={"https://www.elnuevodia.com/noticias/\nhttps://www.metro.pr/noticias/"}
                  rows={3}
                  className={`${inputClass} font-mono text-xs`}
                />
                <p className={hintClass}>Pages to scan for article links, one URL per line</p>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className={labelClass}>Link Selector</label>
                  <input
                    type="text"
                    value={form.link_selector}
                    onChange={(e) => setForm({ ...form, link_selector: e.target.value })}
                    placeholder="article a, .story-card a"
                    className={`${inputClass} font-mono text-xs`}
                  />
                  <p className={hintClass}>e.g. article a, .headline a</p>
                </div>
                <div>
                  <label className={labelClass}>Title Selector</label>
                  <input
                    type="text"
                    value={form.title_selector}
                    onChange={(e) => setForm({ ...form, title_selector: e.target.value })}
                    placeholder="h1, .article-title"
                    className={`${inputClass} font-mono text-xs`}
                  />
                  <p className={hintClass}>e.g. h1, .article-title</p>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className={labelClass}>Body Selector</label>
                  <input
                    type="text"
                    value={form.body_selector}
                    onChange={(e) => setForm({ ...form, body_selector: e.target.value })}
                    placeholder=".article-body, .story-content"
                    className={`${inputClass} font-mono text-xs`}
                  />
                  <p className={hintClass}>e.g. .article-body, article p</p>
                </div>
                <div>
                  <label className={labelClass}>Date Selector</label>
                  <input
                    type="text"
                    value={form.date_selector}
                    onChange={(e) => setForm({ ...form, date_selector: e.target.value })}
                    placeholder="time, .publish-date"
                    className={`${inputClass} font-mono text-xs`}
                  />
                  <p className={hintClass}>e.g. time, .date, [datetime]</p>
                </div>
              </div>
            </div>
          )}

          <div className="flex items-center justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={onCancel}
              className="px-4 py-2 text-sm font-medium text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-100 transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              className="px-4 py-2 text-sm font-medium text-white bg-indigo-500 hover:bg-indigo-600 rounded-lg transition-colors"
            >
              {isEdit ? 'Save Changes' : 'Add Source'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

/** Convert form data to the API shape */
function formToPayload(data: SourceFormData): Partial<Source> {
  const payload: Partial<Source> = {
    name: data.name,
    base_url: data.base_url,
    region: data.region,
    feed_type: data.feed_type,
    feed_url: data.feed_url,
    link_selector: data.link_selector,
    title_selector: data.title_selector,
    body_selector: data.body_selector,
    date_selector: data.date_selector,
  };

  // Parse list_urls from newline-separated text.
  if (data.list_urls.trim()) {
    payload.list_urls = data.list_urls
      .split('\n')
      .map((u) => u.trim())
      .filter((u) => u.length > 0);
  } else {
    payload.list_urls = [];
  }

  return payload;
}

/** Convert a Source object to form data */
function sourceToForm(source: Source): SourceFormData {
  return {
    name: source.name,
    base_url: source.base_url,
    region: source.region,
    feed_type: source.feed_type,
    feed_url: source.feed_url || '',
    list_urls: (source.list_urls || []).join('\n'),
    link_selector: source.link_selector || '',
    title_selector: source.title_selector || '',
    body_selector: source.body_selector || '',
    date_selector: source.date_selector || '',
  };
}

export default function SourcesManager() {
  const [sources, setSources] = useState<Source[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [editingSource, setEditingSource] = useState<Source | null>(null);

  const fetchSources = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.getSources();
      setSources(Array.isArray(data) ? data : []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchSources();
  }, [fetchSources]);

  const handleToggle = async (id: string, currentActive: boolean) => {
    // Optimistic
    setSources((prev) =>
      prev.map((s) => (s.id === id ? { ...s, active: !currentActive } : s))
    );
    try {
      await api.toggleSource(id, !currentActive);
    } catch {
      setSources((prev) =>
        prev.map((s) => (s.id === id ? { ...s, active: currentActive } : s))
      );
    }
  };

  const handleAdd = async (data: SourceFormData) => {
    try {
      await api.createSource(formToPayload(data));
      setShowModal(false);
      fetchSources();
    } catch (err: any) {
      console.error('Failed to create source:', err);
    }
  };

  const handleEdit = async (data: SourceFormData) => {
    if (!editingSource) return;
    try {
      await api.updateSource(editingSource.id, formToPayload(data));
      setEditingSource(null);
      fetchSources();
    } catch (err: any) {
      console.error('Failed to update source:', err);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this source? This cannot be undone.')) return;
    try {
      await api.deleteSource(id);
      setSources((prev) => prev.filter((s) => s.id !== id));
    } catch (err: any) {
      console.error('Failed to delete source:', err);
    }
  };

  if (loading) {
    return (
      <div className="space-y-3">
        {[...Array(4)].map((_, i) => (
          <div
            key={i}
            className="p-4 rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 animate-pulse"
          >
            <div className="flex items-center justify-between">
              <div className="space-y-2">
                <div className="h-4 w-32 bg-zinc-200 dark:bg-zinc-800 rounded" />
                <div className="h-3 w-48 bg-zinc-200 dark:bg-zinc-800 rounded" />
              </div>
              <div className="h-6 w-12 bg-zinc-200 dark:bg-zinc-800 rounded-full" />
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <p className="text-sm text-zinc-500 dark:text-zinc-400 mb-3">{error}</p>
        <button
          onClick={fetchSources}
          className="px-4 py-2 text-sm font-medium text-indigo-500 bg-indigo-500/10 hover:bg-indigo-500/20 rounded-lg transition-colors"
        >
          Try again
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Header with add button */}
      <div className="flex items-center justify-between">
        <p className="text-sm text-zinc-500 dark:text-zinc-400">
          {sources.length} source{sources.length !== 1 ? 's' : ''} configured
          {' '}({sources.filter((s) => s.active).length} active)
        </p>
        <button
          onClick={() => setShowModal(true)}
          className="inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium text-white bg-indigo-500 hover:bg-indigo-600 rounded-lg transition-colors"
        >
          <svg
            className="w-4 h-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
          </svg>
          Add Source
        </button>
      </div>

      {/* Empty state */}
      {sources.length === 0 && (
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
                d="M12 7.5h1.5m-1.5 3h1.5m-7.5 3h7.5m-7.5 3h7.5m3-9h3.375c.621 0 1.125.504 1.125 1.125V18a2.25 2.25 0 01-2.25 2.25M16.5 7.5V18a2.25 2.25 0 002.25 2.25M16.5 7.5V4.875c0-.621-.504-1.125-1.125-1.125H4.125C3.504 3.75 3 4.254 3 4.875V18a2.25 2.25 0 002.25 2.25h13.5M6 7.5h3v3H6v-3z"
              />
            </svg>
          </div>
          <h3 className="text-lg font-medium text-zinc-900 dark:text-zinc-100 mb-1">
            No sources configured
          </h3>
          <p className="text-sm text-zinc-500 dark:text-zinc-400 mb-4">
            Add your first news source to start collecting articles.
          </p>
          <button
            onClick={() => setShowModal(true)}
            className="px-4 py-2 text-sm font-medium text-white bg-indigo-500 hover:bg-indigo-600 rounded-lg transition-colors"
          >
            Add Source
          </button>
        </div>
      )}

      {/* Sources list */}
      <div className="space-y-2">
        {sources.map((source) => (
          <div
            key={source.id}
            className={`p-4 rounded-xl border bg-white dark:bg-zinc-900 transition-colors ${
              source.active
                ? 'border-zinc-200 dark:border-zinc-800 hover:border-zinc-300 dark:hover:border-zinc-700'
                : 'border-zinc-200 dark:border-zinc-800 opacity-60'
            }`}
          >
            <div className="flex items-center justify-between">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2 mb-1">
                  <h3 className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                    {source.name}
                  </h3>
                  <span className="inline-flex items-center px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider rounded-full bg-zinc-100 dark:bg-zinc-800 text-zinc-500 dark:text-zinc-400">
                    {source.feed_type}
                  </span>
                  <span className="inline-flex items-center px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider rounded-full bg-blue-500/10 text-blue-600 dark:text-blue-400">
                    {source.region}
                  </span>
                  {!source.active && (
                    <span className="inline-flex items-center px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider rounded-full bg-red-500/10 text-red-600 dark:text-red-400">
                      Inactive
                    </span>
                  )}
                </div>
                <p className="text-xs text-zinc-500 dark:text-zinc-400 truncate">
                  {source.feed_url || source.base_url}
                </p>
                {source.feed_type === 'scrape' && source.list_urls && source.list_urls.length > 0 && (
                  <p className="text-[10px] text-zinc-400 dark:text-zinc-500 mt-0.5">
                    {source.list_urls.length} list URL{source.list_urls.length !== 1 ? 's' : ''} configured
                    {source.link_selector && ` | Link: ${source.link_selector}`}
                  </p>
                )}
                {source.created_at && (
                  <p className="text-xs text-zinc-400 dark:text-zinc-500 mt-1">
                    Added {formatDate(source.created_at)}
                  </p>
                )}
              </div>

              <div className="flex items-center gap-3 shrink-0 ml-4">
                {/* Active toggle */}
                <button
                  onClick={() => handleToggle(source.id, source.active)}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                    source.active
                      ? 'bg-indigo-500'
                      : 'bg-zinc-200 dark:bg-zinc-700'
                  }`}
                  title={source.active ? 'Active' : 'Inactive'}
                >
                  <span
                    className={`inline-block h-4 w-4 transform rounded-full bg-white shadow-sm transition-transform ${
                      source.active ? 'translate-x-6' : 'translate-x-1'
                    }`}
                  />
                </button>

                {/* Edit */}
                <button
                  onClick={() => setEditingSource(source)}
                  className="p-1.5 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300 rounded-lg hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors"
                  title="Edit"
                >
                  <svg
                    className="w-4 h-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0115.75 21H5.25A2.25 2.25 0 013 18.75V8.25A2.25 2.25 0 015.25 6H10"
                    />
                  </svg>
                </button>

                {/* Delete */}
                <button
                  onClick={() => handleDelete(source.id)}
                  className="p-1.5 text-zinc-400 hover:text-red-500 rounded-lg hover:bg-red-50 dark:hover:bg-red-500/10 transition-colors"
                  title="Delete"
                >
                  <svg
                    className="w-4 h-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0"
                    />
                  </svg>
                </button>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Add modal */}
      {showModal && (
        <SourceFormModal
          initial={emptyForm}
          onSubmit={handleAdd}
          onCancel={() => setShowModal(false)}
          isEdit={false}
        />
      )}

      {/* Edit modal */}
      {editingSource && (
        <SourceFormModal
          initial={sourceToForm(editingSource)}
          onSubmit={handleEdit}
          onCancel={() => setEditingSource(null)}
          isEdit
        />
      )}
    </div>
  );
}
