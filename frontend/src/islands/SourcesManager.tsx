import { useState, useEffect, useCallback } from 'react';
import { api, type Source } from '../lib/api';
import { formatDate } from '../lib/utils';
import { Icon, Spinner, EmptyState, Modal, Notice } from './ui';

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

const REGIONS = ['PR', 'Grants', 'Federal', 'Local'];
const FEED_TYPES = [
  { value: 'rss', label: 'RSS', desc: 'Feed de noticias' },
  { value: 'scrape', label: 'Página web', desc: 'Lee una página' },
  { value: 'sitemap', label: 'Sitemap', desc: 'Mapa del sitio' },
];

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

    if (!form.name.trim()) { setError('Falta el nombre'); return; }
    if (!form.base_url.trim()) { setError('Falta el enlace del sitio'); return; }
    if (form.feed_type === 'rss' && !form.feed_url.trim()) { setError('Falta el enlace del feed'); return; }
    if (form.feed_type === 'scrape' && !form.list_urls.trim()) { setError('Pon al menos una página a revisar'); return; }
    if (form.feed_type === 'scrape' && !form.link_selector.trim()) { setError('Falta el selector de enlaces'); return; }

    onSubmit(form);
  }

  const isScrape = form.feed_type === 'scrape';
  const set = (k: keyof SourceFormData) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => setForm({ ...form, [k]: e.target.value });

  return (
    <Modal title={isEdit ? 'Editar fuente' : 'Agregar fuente'} onClose={onCancel}>
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && <Notice>{error}</Notice>}

        <div>
          <label htmlFor="sf-name" className="field-label">Nombre</label>
          <input id="sf-name" type="text" value={form.name} onChange={set('name')} placeholder="El Nuevo Día" className="field" autoFocus />
        </div>

        <div>
          <span className="field-label">Región</span>
          <div className="chip-row">
            {REGIONS.map((r) => (
              <button key={r} type="button" className="chip" aria-pressed={form.region === r} onClick={() => setForm({ ...form, region: r })}>{r}</button>
            ))}
          </div>
        </div>

        <div>
          <label htmlFor="sf-base" className="field-label">Enlace del sitio</label>
          <input id="sf-base" type="url" value={form.base_url} onChange={set('base_url')} placeholder="https://www.elnuevodia.com" className="field" />
        </div>

        <div>
          <span className="field-label">Tipo</span>
          <div className="grid grid-cols-3 gap-2">
            {FEED_TYPES.map((opt) => {
              const on = form.feed_type === opt.value;
              return (
                <button
                  key={opt.value}
                  type="button"
                  aria-pressed={on}
                  onClick={() => setForm({ ...form, feed_type: opt.value })}
                  className={`p-3 text-left !min-h-[64px] transition-[border-radius,background-color] duration-500 ease-[var(--spring)] ${on ? 'bg-primary-container text-on-primary-container rounded-[24px]' : 'bg-surface-container-high text-on-surface rounded-[12px] hover:rounded-[24px]'}`}
                >
                  <span className="block text-sm font-bold">{opt.label}</span>
                  <span className="block text-xs mt-0.5 opacity-80">{opt.desc}</span>
                </button>
              );
            })}
          </div>
        </div>

        {(form.feed_type === 'rss' || form.feed_type === 'sitemap') && (
          <div>
            <label htmlFor="sf-feed" className="field-label">{form.feed_type === 'rss' ? 'Enlace del feed' : 'Enlace del sitemap'}</label>
            <input id="sf-feed" type="url" value={form.feed_url} onChange={set('feed_url')} placeholder={form.feed_type === 'rss' ? 'https://www.elnuevodia.com/arcio/rss/' : 'https://ejemplo.com/sitemap.xml'} className="field" />
          </div>
        )}

        {isScrape && (
          <div className="space-y-4 p-4 rounded-[24px] bg-surface-container">
            <h3 className="title-md">Cómo leer la página</h3>
            <div>
              <label htmlFor="sf-list" className="field-label">Páginas a revisar (una por línea)</label>
              <textarea id="sf-list" value={form.list_urls} onChange={set('list_urls')} placeholder={"https://www.elnuevodia.com/noticias/\nhttps://www.metro.pr/noticias/"} rows={3} className="field font-mono text-xs" />
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label htmlFor="sf-link" className="field-label">Selector de enlaces</label>
                <input id="sf-link" type="text" value={form.link_selector} onChange={set('link_selector')} placeholder="article a" className="field font-mono text-xs" />
              </div>
              <div>
                <label htmlFor="sf-title" className="field-label">Selector del título</label>
                <input id="sf-title" type="text" value={form.title_selector} onChange={set('title_selector')} placeholder="h1" className="field font-mono text-xs" />
              </div>
              <div>
                <label htmlFor="sf-body" className="field-label">Selector del texto</label>
                <input id="sf-body" type="text" value={form.body_selector} onChange={set('body_selector')} placeholder=".article-body" className="field font-mono text-xs" />
              </div>
              <div>
                <label htmlFor="sf-date" className="field-label">Selector de la fecha</label>
                <input id="sf-date" type="text" value={form.date_selector} onChange={set('date_selector')} placeholder="time" className="field font-mono text-xs" />
              </div>
            </div>
          </div>
        )}

        <div className="flex items-center justify-end gap-2 pt-2">
          <button type="button" onClick={onCancel} className="btn btn-text">Cancelar</button>
          <button type="submit" className="btn btn-filled">{isEdit ? 'Guardar cambios' : 'Agregar'}</button>
        </div>
      </form>
    </Modal>
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
  const [testingId, setTestingId] = useState<string | null>(null);
  const [testResults, setTestResults] = useState<Record<string, any>>({});

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
    if (!confirm('¿Borrar esta fuente? No se puede deshacer.')) return;
    try {
      await api.deleteSource(id);
      setSources((prev) => prev.filter((s) => s.id !== id));
    } catch (err: any) {
      console.error('Failed to delete source:', err);
    }
  };

  const handleTest = async (id: string) => {
    setTestingId(id);
    try {
      const result = await api.testSource(id);
      setTestResults(prev => ({ ...prev, [id]: result }));
    } catch (err: any) {
      setTestResults(prev => ({ ...prev, [id]: { success: false, error: err.message } }));
    } finally {
      setTestingId(null);
    }
  };

  if (loading) {
    return (
      <div className="seg-list">
        {[...Array(4)].map((_, i) => (
          <div key={i} className="p-5 animate-pulse">
            <div className="h-5 w-40 bg-surface-container-high rounded-full mb-2" />
            <div className="h-4 w-64 bg-surface-container-high rounded-full" />
          </div>
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <EmptyState icon="alert" text={error}>
        <button type="button" onClick={fetchSources} className="btn btn-filled">Intentar de nuevo</button>
      </EmptyState>
    );
  }

  const activeCount = sources.filter((s) => s.active).length;

  return (
    <div className="space-y-5">
      <p className="text-sm font-semibold text-on-surface-variant">
        {sources.length} {sources.length === 1 ? 'fuente' : 'fuentes'}, {activeCount} {activeCount === 1 ? 'activa' : 'activas'}
      </p>

      {sources.length === 0 && (
        <EmptyState icon="globe" text="Agrega tu primera fuente para recibir noticias.">
          <button type="button" onClick={() => setShowModal(true)} className="btn btn-filled">
            <Icon name="plus" className="w-5 h-5" />Agregar fuente
          </button>
        </EmptyState>
      )}

      <div className="seg-list">
        {sources.map((source) => (
          <div key={source.id} className={`p-4 sm:p-5 ${source.active ? '' : 'opacity-70'}`}>
            <div className="flex items-center justify-between gap-3">
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2 mb-1">
                  <h3 className="title-md">{source.name}</h3>
                  <span className="tag">{source.feed_type}</span>
                  <span className="tag tag-primary">{source.region}</span>
                  {!source.active && <span className="tag tag-bad">Apagada</span>}
                </div>
                <p className="text-sm text-on-surface-variant truncate">{source.feed_url || source.base_url}</p>
                {source.feed_type === 'scrape' && source.list_urls && source.list_urls.length > 0 && (
                  <p className="text-xs text-on-surface-variant mt-0.5">
                    {source.list_urls.length} {source.list_urls.length === 1 ? 'página' : 'páginas'}
                    {source.link_selector && ` · Enlaces: ${source.link_selector}`}
                  </p>
                )}
                {source.created_at && (
                  <p className="text-xs text-on-surface-variant mt-1">Agregada el {formatDate(source.created_at)}</p>
                )}
              </div>

              <div className="flex items-center gap-1 shrink-0">
                <button
                  type="button"
                  role="switch"
                  aria-checked={source.active}
                  onClick={() => handleToggle(source.id, source.active)}
                  className={`relative inline-flex h-8 w-14 items-center rounded-full transition-colors duration-300 ${source.active ? 'bg-primary' : 'bg-surface-container-highest'}`}
                  title={source.active ? 'Activa: toca para apagar' : 'Apagada: toca para activar'}
                  aria-label={`${source.active ? 'Apagar' : 'Activar'} ${source.name}`}
                >
                  <span className={`inline-block rounded-full shadow-sm transition-all duration-500 ease-[var(--spring)] ${source.active ? 'h-6 w-6 translate-x-7 bg-on-primary' : 'h-4 w-4 translate-x-2 bg-outline'}`} />
                </button>

                <button type="button" onClick={() => setEditingSource(source)} className="icon-btn" title="Editar" aria-label={`Editar ${source.name}`}>
                  <Icon name="note" />
                </button>

                <button type="button" onClick={() => handleTest(source.id)} className="icon-btn" title="Probar" aria-label={`Probar ${source.name}`} disabled={testingId === source.id}>
                  {testingId === source.id ? <Spinner /> : <Icon name="refresh" />}
                </button>

                <button type="button" onClick={() => handleDelete(source.id)} className="icon-btn hover:text-error" title="Borrar" aria-label={`Borrar ${source.name}`}>
                  <Icon name="trash" />
                </button>
              </div>
            </div>
            {testResults[source.id] && (
              <div className={`mt-3 p-3 rounded-2xl text-sm font-semibold ${testResults[source.id].success ? 'bg-success-container text-on-success-container' : 'bg-error-container text-on-error-container'}`}>
                {testResults[source.id].success ? (
                  <>Título: {testResults[source.id].title} · Texto: {testResults[source.id].text_length} letras · Imagen: {testResults[source.id].image_found ? 'Sí' : 'No'}</>
                ) : (
                  <>Error: {testResults[source.id].error}</>
                )}
              </div>
            )}
          </div>
        ))}
      </div>

      <div className="fab-slot">
        <button type="button" onClick={() => setShowModal(true)} className="fab" aria-label="Agregar fuente">
          <Icon name="plus" className="w-6 h-6" />Agregar fuente
        </button>
      </div>

      {showModal && (
        <SourceFormModal initial={emptyForm} onSubmit={handleAdd} onCancel={() => setShowModal(false)} isEdit={false} />
      )}

      {editingSource && (
        <SourceFormModal initial={sourceToForm(editingSource)} onSubmit={handleEdit} onCancel={() => setEditingSource(null)} isEdit />
      )}
    </div>
  );
}
