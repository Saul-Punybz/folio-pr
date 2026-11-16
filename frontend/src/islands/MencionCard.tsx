import { useState } from 'react';
import { api, type WatchlistHit } from '../lib/api';
import { Icon, Spinner, Modal, Notice } from './ui';

/* Ficha de una mención archivada: título, medio, fecha, resumen, enlaces,
   copia guardada dentro de Folio y la cita APA con «Copiar cita». */

export const SOURCE_LABELS: Record<string, string> = {
  local: 'Noticias PR',
  google_news: 'Google News',
  bing_news: 'Bing News',
  web: 'Web',
  sitio: 'Sitio',
  alertas: 'Alertas',
  youtube: 'YouTube',
  bluesky: 'Bluesky',
  mastodon: 'Mastodon',
  reddit: 'Reddit',
};

const ESTADO: Record<string, { label: string; cls: string }> = {
  copia: { label: 'Copia guardada', cls: 'tag tag-ok' },
  parcial: { label: 'Copia parcial', cls: 'tag tag-accent' },
  sin_copia: { label: 'Sin copia', cls: 'tag tag-bad' },
  pendiente: { label: 'Copia pendiente', cls: 'tag' },
};

export function fechaLarga(iso: string | null): string {
  if (!iso) return 'Sin fecha';
  try {
    return new Date(iso).toLocaleDateString('es-PR', { day: 'numeric', month: 'long', year: 'numeric', timeZone: 'America/Puerto_Rico' });
  } catch {
    return 'Sin fecha';
  }
}

/** Cita con la cursiva marcada como *…* → <i>…</i>. */
export function Cita({ markdown }: { markdown: string }) {
  const parts = markdown.split('*');
  return (
    <>
      {parts.map((p, i) => (i % 2 === 1 ? <i key={i}>{p}</i> : <span key={i}>{p}</span>))}
    </>
  );
}

const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

/** Copia al portapapeles: texto plano y, si el navegador deja, HTML con la cursiva (para pegar en Word). */
export async function copiarCitas(plain: string, markdown: string): Promise<boolean> {
  try {
    const html = markdown.split('\n\n').map(p => '<p>' + p.split('*').map((s, i) => (i % 2 === 1 ? `<i>${esc(s)}</i>` : esc(s))).join('') + '</p>').join('');
    const CI = (window as unknown as { ClipboardItem?: typeof ClipboardItem }).ClipboardItem;
    if (CI && navigator.clipboard.write) {
      await navigator.clipboard.write([new CI({ 'text/plain': new Blob([plain], { type: 'text/plain' }), 'text/html': new Blob([html], { type: 'text/html' }) })]);
    } else {
      await navigator.clipboard.writeText(plain);
    }
    return true;
  } catch {
    try {
      await navigator.clipboard.writeText(plain);
      return true;
    } catch {
      return false;
    }
  }
}

type MetaForm = { title: string; site: string; authors: string; date: string; summary: string; canonical: string };

function formOf(h: WatchlistHit): MetaForm {
  return { title: h.title, site: h.medio || h.site_name, authors: (h.authors || []).join('\n'), date: h.fecha || '', summary: h.summary, canonical: h.canonical_url };
}

export function MencionCard({ hit, onSeen, onDelete, onChanged }: {
  hit: WatchlistHit;
  onSeen?: () => void;
  onDelete: () => void;
  onChanged: (h: WatchlistHit) => void;
}) {
  const [copied, setCopied] = useState(false);
  const [copyOpen, setCopyOpen] = useState(false);
  const [copyText, setCopyText] = useState<string | null>(null);
  const [copyErr, setCopyErr] = useState('');
  const [editOpen, setEditOpen] = useState(false);
  const [form, setForm] = useState<MetaForm>(formOf(hit));
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState('');
  const [retrying, setRetrying] = useState(false);

  const estado = ESTADO[hit.archive_status] || ESTADO.pendiente;
  const medio = hit.medio || hit.site_name;
  const autores = (hit.authors || []).join(', ');

  const copyCita = async () => {
    if (await copiarCitas(hit.apa.plain, hit.apa.markdown)) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const openCopy = async () => {
    setCopyOpen(true);
    setCopyErr('');
    if (copyText !== null) return;
    try {
      const d = await api.getHitCopia(hit.id);
      setCopyText(d.text);
    } catch (e) {
      setCopyErr(e instanceof Error ? e.message : 'No se pudo abrir la copia.');
    }
  };

  const openEdit = () => { setForm(formOf(hit)); setErr(''); setEditOpen(true); };

  const save = async () => {
    if (!form.title.trim()) return;
    setSaving(true);
    setErr('');
    try {
      const upd = await api.updateHitMeta(hit.id, {
        title: form.title,
        site_name: form.site,
        authors: form.authors.split('\n').map(a => a.trim()).filter(Boolean),
        published_at: form.date,
        summary: form.summary,
        canonical_url: form.canonical,
      });
      onChanged({ ...hit, ...upd });
      setEditOpen(false);
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'No se pudo guardar.');
    } finally {
      setSaving(false);
    }
  };

  const retry = async () => {
    setRetrying(true);
    try {
      await api.retryHitCopia(hit.id);
      onChanged({ ...hit, archive_status: 'pendiente', archive_attempts: 0 });
    } catch { /* se queda como estaba */ }
    setRetrying(false);
  };

  const remove = () => {
    if (confirm('¿Borrar esta mención del archivo? No se puede deshacer.')) onDelete();
  };

  const set = (k: keyof MetaForm) => (e: { target: { value: string } }) => setForm({ ...form, [k]: e.target.value });

  return (
    <article className={`flex flex-col overflow-hidden rounded-[28px] hover:rounded-[36px] transition-[border-radius,background-color] duration-500 ease-[var(--spring)] ${hit.seen ? 'bg-surface-container-low' : 'bg-surface-container ring-[3px] ring-tertiary'}`}>
      <div className="px-5 pt-5 flex items-center justify-between gap-2 text-xs font-semibold text-on-surface-variant">
        <span className="truncate">{medio || SOURCE_LABELS[hit.source_type]}</span>
        <span className="shrink-0">{fechaLarga(hit.published_at || hit.created_at)}</span>
      </div>
      <h4 className="px-5 pt-2 text-lg font-extrabold leading-tight tracking-tight line-clamp-3">
        <a href={hit.original_url || hit.url} target="_blank" rel="noopener noreferrer" className="hover:text-primary transition-colors">{hit.title || 'Sin título'}</a>
      </h4>
      {autores && <p className="px-5 pt-1 text-xs font-semibold text-on-surface-variant">{autores}</p>}
      {(hit.summary || hit.snippet) && <p className="px-5 pt-2 text-sm text-on-surface-variant leading-relaxed line-clamp-4">{hit.summary || hit.snippet}</p>}

      <div className="px-5 pt-3 flex flex-wrap gap-1.5">
        <span className={estado.cls}>{estado.label}</span>
        {hit.matched_person && <span className="tag tag-accent">{hit.matched_person}</span>}
        {hit.matched_hashtag && <span className="tag tag-primary">#{hit.matched_hashtag}</span>}
        {hit.edited && <span className="tag">Corregida</span>}
      </div>

      <div className="px-5 pt-3 flex flex-wrap gap-2">
        <a href={hit.original_url || hit.url} target="_blank" rel="noopener noreferrer" className="btn btn-soft btn-sm"><Icon name="open" className="w-5 h-5" />Ver original</a>
        {hit.wayback_url && (
          <a href={hit.wayback_url} target="_blank" rel="noopener noreferrer" className="btn btn-soft btn-sm"><Icon name="globe" className="w-5 h-5" />Copia en Internet Archive</a>
        )}
        {hit.has_text && (
          <button type="button" onClick={openCopy} className="btn btn-soft btn-sm"><Icon name="save" className="w-5 h-5" />Ver copia guardada</button>
        )}
      </div>

      <div className="mx-3 mt-4 p-4 rounded-[20px] bg-surface-container-highest text-sm leading-relaxed break-words">
        <Cita markdown={hit.apa.markdown} />
        <div className="mt-3">
          <button type="button" onClick={copyCita} className={`btn btn-sm ${copied ? 'btn-tonal' : 'btn-filled'}`}>
            <Icon name={copied ? 'check' : 'link'} className="w-5 h-5" />{copied ? 'Copiada' : 'Copiar cita'}
          </button>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2 p-3 pt-4">
        {!hit.seen && onSeen && (
          <button type="button" onClick={onSeen} className="btn btn-filled btn-sm"><Icon name="check" className="w-5 h-5" />Marcar visto</button>
        )}
        <button type="button" onClick={openEdit} className="btn btn-text btn-sm"><Icon name="note" className="w-5 h-5" />Corregir datos</button>
        {(hit.archive_status === 'sin_copia' || hit.archive_status === 'parcial') && (
          <button type="button" onClick={retry} disabled={retrying} className="btn btn-text btn-sm">
            {retrying ? <Spinner className="w-5 h-5" /> : <Icon name="refresh" className="w-5 h-5" />}Reintentar copia
          </button>
        )}
        <button type="button" onClick={remove} className="btn btn-text btn-sm ml-auto hover:text-error"><Icon name="trash" className="w-5 h-5" />Borrar</button>
      </div>

      {copyOpen && (
        <Modal title="Copia guardada" onClose={() => setCopyOpen(false)}>
          <p className="text-sm font-semibold text-on-surface-variant">
            {hit.archived_at ? `Archivada el ${fechaLarga(hit.archived_at)}.` : 'Guardada en Folio.'}
          </p>
          {copyErr && <div className="mt-3"><Notice>{copyErr}</Notice></div>}
          {copyText === null && !copyErr ? (
            <div className="flex justify-center py-8 text-primary"><Spinner className="w-7 h-7" /></div>
          ) : (
            <div className="mt-3 space-y-3 text-sm leading-relaxed">
              {(copyText || '').split('\n\n').map((p, i) => <p key={i}>{p}</p>)}
            </div>
          )}
        </Modal>
      )}

      {editOpen && (
        <Modal title="Corregir datos" onClose={() => setEditOpen(false)}>
          <div className="space-y-4">
            <div>
              <label htmlFor={`m-t-${hit.id}`} className="field-label">Título</label>
              <input id={`m-t-${hit.id}`} type="text" value={form.title} onChange={set('title')} className="field" />
            </div>
            <div>
              <label htmlFor={`m-s-${hit.id}`} className="field-label">Medio</label>
              <input id={`m-s-${hit.id}`} type="text" value={form.site} onChange={set('site')} className="field" />
            </div>
            <div>
              <label htmlFor={`m-a-${hit.id}`} className="field-label">Autores (uno por línea)</label>
              <textarea id={`m-a-${hit.id}`} value={form.authors} onChange={set('authors')} rows={2} placeholder="Beatriz Rivera Batista" className="field" />
            </div>
            <div>
              <label htmlFor={`m-d-${hit.id}`} className="field-label">Fecha de publicación</label>
              <input id={`m-d-${hit.id}`} type="date" value={form.date} onChange={set('date')} className="field" />
            </div>
            <div>
              <label htmlFor={`m-c-${hit.id}`} className="field-label">Enlace de la cita</label>
              <input id={`m-c-${hit.id}`} type="url" value={form.canonical} onChange={set('canonical')} placeholder="https://" className="field" />
            </div>
            <div>
              <label htmlFor={`m-r-${hit.id}`} className="field-label">Resumen</label>
              <textarea id={`m-r-${hit.id}`} value={form.summary} onChange={set('summary')} rows={5} className="field" />
            </div>
          </div>
          {err && <div className="mt-4"><Notice>{err}</Notice></div>}
          <div className="mt-6 flex justify-end gap-2">
            <button type="button" onClick={() => setEditOpen(false)} className="btn btn-text">Cancelar</button>
            <button type="button" onClick={save} disabled={!form.title.trim() || saving} className="btn btn-filled">{saving ? 'Guardando...' : 'Guardar'}</button>
          </div>
        </Modal>
      )}
    </article>
  );
}
