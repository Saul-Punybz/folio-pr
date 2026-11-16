import { useState, useEffect, useCallback, useRef } from 'react';
import { api, type ArchivoResponse, type WatchlistHit } from '../lib/api';
import { Icon, Spinner, EmptyState, Notice } from './ui';
import { MencionCard, SOURCE_LABELS, copiarCitas } from './MencionCard';

/* «Archivo de [marca]»: todas las menciones de siempre, con buscador, filtro
   por fecha y por fuente, bibliografía APA y exportación. */

const REDES: { key: string; label: string }[] = [
  { key: 'noticias', label: 'Noticias' },
  { key: 'web', label: SOURCE_LABELS.web },
  { key: 'sitio', label: SOURCE_LABELS.sitio },
  { key: 'youtube', label: SOURCE_LABELS.youtube },
  { key: 'bluesky', label: SOURCE_LABELS.bluesky },
  { key: 'mastodon', label: SOURCE_LABELS.mastodon },
  { key: 'reddit', label: SOURCE_LABELS.reddit },
  { key: 'podcast', label: SOURCE_LABELS.podcast },
];

export default function ArchivoMarca({ orgId, name, onBack }: { orgId: string; name: string; onBack: () => void }) {
  const [q, setQ] = useState('');
  const [desde, setDesde] = useState('');
  const [hasta, setHasta] = useState('');
  const [fuente, setFuente] = useState('');
  const [data, setData] = useState<ArchivoResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [copied, setCopied] = useState(false);
  const [running, setRunning] = useState(false);
  const seq = useRef(0);

  const load = useCallback(async () => {
    const mine = ++seq.current;
    setLoading(true);
    try {
      const d = await api.getArchivo(orgId, { q: q.trim(), desde, hasta, fuente });
      if (mine === seq.current) { setData(d); setError(''); }
    } catch (e) {
      if (mine === seq.current) setError(e instanceof Error ? e.message : 'No se pudo cargar el archivo.');
    } finally {
      if (mine === seq.current) setLoading(false);
    }
  }, [orgId, q, desde, hasta, fuente]);

  // El buscador espera un momento a que termines de escribir.
  useEffect(() => {
    const t = setTimeout(load, q ? 300 : 0);
    return () => clearTimeout(t);
  }, [load, q]);

  const filtros = { q: q.trim(), desde, hasta, fuente };

  const copyBiblio = async () => {
    if (!data || !data.bibliografia.plain) return;
    if (await copiarCitas(data.bibliografia.plain, data.bibliografia.markdown)) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const archivar = async () => {
    setRunning(true);
    try {
      await api.runArchivo();
      setTimeout(() => { load(); setRunning(false); }, 8000);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudo iniciar.');
      setRunning(false);
    }
  };

  const replace = (h: WatchlistHit) => setData(d => (d ? { ...d, hits: d.hits.map(x => (x.id === h.id ? h : x)) } : d));

  const removeHit = async (id: string) => {
    try {
      await api.deleteHit(id);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudo borrar.');
    }
  };

  const markSeen = async (id: string) => {
    try {
      await api.markHitSeen(id);
      setData(d => (d ? { ...d, hits: d.hits.map(x => (x.id === id ? { ...x, seen: true } : x)) } : d));
    } catch { /* se queda como estaba */ }
  };

  const st = data?.stats;
  const hayFiltro = !!(q || desde || hasta || fuente);

  return (
    <section className="space-y-4" aria-label={`Archivo de ${name}`}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0">
          <h2 className="title-lg truncate">Archivo de {name}</h2>
          {st && <span className="tag">{st.total}</span>}
        </div>
        <button type="button" onClick={onBack} className="btn btn-text btn-sm">Volver a la ficha</button>
      </div>

      {st && (
        <div className="chip-row">
          <span className="tag tag-ok">{st.copia} con copia</span>
          {st.parcial > 0 && <span className="tag tag-accent">{st.parcial} copia parcial</span>}
          {st.sin_copia > 0 && <span className="tag tag-bad">{st.sin_copia} sin copia</span>}
          {st.pendiente > 0 && <span className="tag">{st.pendiente} pendientes</span>}
          <span className="tag">{st.con_wayback} en Internet Archive</span>
        </div>
      )}

      {error && <Notice>{error}</Notice>}

      <div className="grid grid-cols-1 sm:grid-cols-[1fr_auto_auto] gap-3">
        <div>
          <label htmlFor="ar-q" className="field-label">Buscar en el archivo</label>
          <input id="ar-q" type="search" value={q} onChange={e => setQ(e.target.value)} placeholder="Título, medio, autor o texto" className="field" />
        </div>
        <div>
          <label htmlFor="ar-d" className="field-label">Desde</label>
          <input id="ar-d" type="date" value={desde} onChange={e => setDesde(e.target.value)} className="field" />
        </div>
        <div>
          <label htmlFor="ar-h" className="field-label">Hasta</label>
          <input id="ar-h" type="date" value={hasta} onChange={e => setHasta(e.target.value)} className="field" />
        </div>
      </div>

      <div className="chip-row chip-scroll" role="group" aria-label="Fuente">
        <button type="button" className="chip" aria-pressed={!fuente} onClick={() => setFuente('')}>Todas</button>
        {REDES.map(r => (
          <button key={r.key} type="button" className="chip" aria-pressed={fuente === r.key} onClick={() => setFuente(r.key)}>{r.label}</button>
        ))}
        {hayFiltro && (
          <button type="button" className="chip" onClick={() => { setQ(''); setDesde(''); setHasta(''); setFuente(''); }}>Quitar filtros</button>
        )}
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <button type="button" onClick={copyBiblio} disabled={!data || data.hits.length === 0} className={`btn btn-sm ${copied ? 'btn-tonal' : 'btn-filled'} disabled:opacity-50`}>
          <Icon name={copied ? 'check' : 'link'} className="w-5 h-5" />{copied ? 'Bibliografía copiada' : 'Copiar bibliografía'}
        </button>
        <a href={api.archivoExportURL(orgId, 'md', filtros)} download className="btn btn-soft btn-sm"><Icon name="save" className="w-5 h-5" />Exportar Markdown</a>
        <a href={api.archivoExportURL(orgId, 'csv', filtros)} download className="btn btn-soft btn-sm"><Icon name="save" className="w-5 h-5" />Exportar CSV</a>
        <button type="button" onClick={archivar} disabled={running} className="btn btn-text btn-sm">
          {running ? <Spinner className="w-5 h-5" /> : <Icon name="refresh" className="w-5 h-5" />}Guardar copias pendientes
        </button>
        {data && hayFiltro && <span className="text-sm font-semibold text-on-surface-variant">{data.total} de {st?.total ?? data.total}</span>}
      </div>

      {loading && !data ? (
        <div className="flex justify-center py-10 text-primary"><Spinner className="w-7 h-7" /></div>
      ) : !data || data.hits.length === 0 ? (
        <EmptyState icon="inbox" text={hayFiltro ? 'No hay menciones con esos filtros.' : 'Aún no hay menciones archivadas.'} />
      ) : (
        <div className={`stagger grid grid-cols-1 xl:grid-cols-2 gap-3 items-start ${loading ? 'opacity-60' : ''}`}>
          {data.hits.map(h => (
            <MencionCard key={h.id} hit={h} onSeen={() => markSeen(h.id)} onDelete={() => removeHit(h.id)} onChanged={replace} />
          ))}
        </div>
      )}
    </section>
  );
}
