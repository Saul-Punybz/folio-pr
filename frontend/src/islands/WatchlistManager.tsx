import { useState, useEffect, useCallback, useMemo } from 'react';
import { api, type WatchlistOrg, type WatchlistHit, type WatchKind, type WatchSocial, type WatchlistOrgInput, type WatchPerson } from '../lib/api';
import { Icon, Spinner, EmptyState, Modal, Notice } from './ui';

// ── Fuentes y redes ──────────────────────────────────────────────

const SOURCE_LABELS: Record<string, string> = {
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
const SOURCE_ORDER = Object.keys(SOURCE_LABELS);

const NETWORKS: { key: string; label: string; short: string }[] = [
  { key: 'instagram', label: 'Instagram', short: 'IG' },
  { key: 'facebook', label: 'Facebook', short: 'FB' },
  { key: 'linkedin', label: 'LinkedIn', short: 'in' },
  { key: 'x', label: 'X', short: 'X' },
  { key: 'tiktok', label: 'TikTok', short: 'TT' },
  { key: 'youtube', label: 'YouTube', short: 'YT' },
  { key: 'threads', label: 'Threads', short: 'Th' },
  { key: 'bluesky', label: 'Bluesky', short: 'BS' },
];
const NET = Object.fromEntries(NETWORKS.map(n => [n.key, n]));

const HASHTAG_LINKS: { label: string; url: (t: string) => string }[] = [
  { label: 'Instagram', url: t => `https://www.instagram.com/explore/tags/${t}/` },
  { label: 'Facebook', url: t => `https://www.facebook.com/hashtag/${t}` },
  { label: 'LinkedIn', url: t => `https://www.linkedin.com/feed/hashtag/${t}/` },
  { label: 'TikTok', url: t => `https://www.tiktok.com/tag/${t}` },
  { label: 'X', url: t => `https://x.com/hashtag/${t}` },
];

const cleanTag = (t: string) => t.trim().replace(/^#+/, '');

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'ahora';
  if (mins < 60) return `hace ${mins} min`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `hace ${hrs} h`;
  const days = Math.floor(hrs / 24);
  if (days < 30) return `hace ${days} d`;
  const months = Math.floor(days / 30);
  if (months < 12) return `hace ${months} m`;
  return new Date(dateStr).toLocaleDateString('es-PR', { year: 'numeric', month: 'short' });
}

const hitDate = (h: WatchlistHit) => h.published_at || h.created_at;

/** Handle a partir del enlace: https://www.instagram.com/vimentipr/ → vimentipr */
function handleFromURL(url: string): string {
  try {
    const u = new URL(url);
    const id = u.searchParams.get('id');
    if (id) return id;
    const parts = u.pathname.split('/').filter(Boolean);
    return (parts[parts.length - 1] || u.hostname).replace(/^@/, '');
  } catch {
    return url;
  }
}

const splitList = (s: string, sep: RegExp = /[,\n]/) => s.split(sep).map(x => x.trim()).filter(Boolean);

function NetBadge({ network }: { network: string }) {
  const n = NET[network];
  return (
    <span className="inline-flex items-center justify-center w-7 h-7 rounded-full bg-surface-container-highest text-on-surface-variant text-[11px] font-extrabold" title={n?.label || network}>
      {n?.short || network.slice(0, 2)}
    </span>
  );
}

// ── Formulario ───────────────────────────────────────────────────

type FormState = {
  name: string;
  kind: WatchKind;
  website: string;
  aliases: string;
  ctxAliases: string;
  exclude: string;
  hashtags: string;
  keywords: string;
  youtube: string;
  alerts: string;
  socials: Record<string, string>;
  people: WatchPerson[];
};

const emptyForm = (kind: WatchKind): FormState => ({
  name: '', kind, website: '', aliases: '', ctxAliases: '', exclude: '', hashtags: '', keywords: '', youtube: '', alerts: '', socials: {}, people: [],
});

function formFromOrg(o: WatchlistOrg): FormState {
  const socials: Record<string, string> = {};
  for (const s of o.socials) socials[s.network] = s.url || s.handle;
  return {
    name: o.name, kind: o.kind, website: o.website || '',
    aliases: o.aliases.join(', '), ctxAliases: (o.context_aliases || []).join(', '), exclude: o.exclude_terms.join(', '),
    hashtags: o.hashtags.map(t => '#' + t).join(' '), keywords: o.keywords.join(', '),
    youtube: o.youtube_channels.join('\n'), alerts: o.alert_feeds.join('\n'), socials,
    people: (o.people || []).map(p => ({ ...p })),
  };
}

function inputFromForm(f: FormState, prev: WatchlistOrg | null): WatchlistOrgInput {
  const socials: WatchSocial[] = [];
  for (const n of NETWORKS) {
    const v = (f.socials[n.key] || '').trim();
    if (!v) continue;
    const old = prev?.socials.find(s => s.network === n.key);
    if (/^https?:\/\//.test(v)) {
      // Se conserva el channel_id de YouTube guardado si el enlace no cambió.
      const handle = old && old.url === v ? old.handle : handleFromURL(v);
      socials.push({ network: n.key, handle, url: v });
    } else {
      socials.push({ network: n.key, handle: v.replace(/^@/, ''), url: old?.handle === v ? old.url : '' });
    }
  }
  const name = f.name.trim();
  let aliases = splitList(f.aliases);
  const context_aliases = splitList(f.ctxAliases);
  if (f.kind === 'marca' && aliases.length === 0 && context_aliases.length === 0) aliases = [name];
  return {
    name,
    kind: f.kind,
    website: f.website.trim(),
    aliases,
    context_aliases,
    exclude_terms: splitList(f.exclude),
    hashtags: splitList(f.hashtags, /[\s,]+/).map(cleanTag).filter(Boolean),
    keywords: splitList(f.keywords),
    youtube_channels: splitList(f.youtube, /\n/),
    alert_feeds: splitList(f.alerts, /\n/),
    socials,
    people: f.people.map(p => ({ name: p.name.trim(), role: p.role.trim(), url: p.url.trim() })).filter(p => p.name),
    to_confirm: false,
    active: prev ? prev.active : true,
  };
}

function OrgForm({ form, setForm }: { form: FormState; setForm: (f: FormState) => void }) {
  const set = (k: keyof FormState) => (e: { target: { value: string } }) => setForm({ ...form, [k]: e.target.value });
  const brand = form.kind === 'marca';
  return (
    <div className="space-y-4">
      <div>
        <label htmlFor="wo-name" className="field-label">Nombre</label>
        <input id="wo-name" type="text" value={form.name} onChange={set('name')} className="field" autoFocus />
      </div>
      {!brand && (
        <div className="chip-row" role="group" aria-label="Tipo">
          <button type="button" className="chip" aria-pressed={form.kind === 'tema'} onClick={() => setForm({ ...form, kind: 'tema' })}>Tema</button>
          <button type="button" className="chip" aria-pressed={form.kind === 'persona'} onClick={() => setForm({ ...form, kind: 'persona' })}>Persona</button>
        </div>
      )}
      {brand ? (
        <>
          <div>
            <label htmlFor="wo-alias" className="field-label">Alias (separados por coma)</label>
            <input id="wo-alias" type="text" value={form.aliases} onChange={set('aliases')} placeholder="Vimenti, Escuela Vimenti" className="field" />
          </div>
          <div>
            <label htmlFor="wo-ctx" className="field-label">Alias que solo cuentan junto a Puerto Rico</label>
            <input id="wo-ctx" type="text" value={form.ctxAliases} onChange={set('ctxAliases')} placeholder="PSI, Oasis Hub" className="field" />
          </div>
          <div>
            <label htmlFor="wo-ex" className="field-label">Excluir si dice</label>
            <input id="wo-ex" type="text" value={form.exclude} onChange={set('exclude')} placeholder="hotel, banda" className="field" />
          </div>
          <div>
            <label htmlFor="wo-ht" className="field-label">Hashtags</label>
            <input id="wo-ht" type="text" value={form.hashtags} onChange={set('hashtags')} placeholder="#vimentipr" className="field" />
          </div>
        </>
      ) : (
        <div>
          <label htmlFor="wo-kw" className="field-label">Palabras clave (separadas por coma)</label>
          <textarea id="wo-kw" value={form.keywords} onChange={set('keywords')} rows={2} className="field" />
        </div>
      )}
      <div>
        <label htmlFor="wo-web" className="field-label">Página web</label>
        <input id="wo-web" type="url" value={form.website} onChange={set('website')} placeholder="https://" className="field" />
      </div>
      {brand && (
        <div className="space-y-2">
          <span className="field-label">Personas clave</span>
          {form.people.map((p, i) => {
            const setP = (k: keyof WatchPerson, v: string) => setForm({ ...form, people: form.people.map((x, j) => (j === i ? { ...x, [k]: v } : x)) });
            return (
              <div key={i} className="p-3 rounded-[20px] bg-surface-container space-y-2">
                <div className="flex items-center gap-2">
                  <input type="text" aria-label="Nombre" value={p.name} onChange={e => setP('name', e.target.value)} placeholder="Nombre y apellido" className="field !py-2.5" />
                  <button type="button" onClick={() => setForm({ ...form, people: form.people.filter((_, j) => j !== i) })} className="icon-btn hover:text-error shrink-0" aria-label={`Quitar a ${p.name || 'esta persona'}`}><Icon name="trash" className="w-5 h-5" /></button>
                </div>
                <input type="text" aria-label="Cargo" value={p.role} onChange={e => setP('role', e.target.value)} placeholder="Cargo" className="field !py-2.5" />
                <input type="url" aria-label="Perfil" value={p.url} onChange={e => setP('url', e.target.value)} placeholder="Enlace de perfil" className="field !py-2.5" />
              </div>
            );
          })}
          <button type="button" onClick={() => setForm({ ...form, people: [...form.people, { name: '', role: '', url: '' }] })} className="btn btn-soft btn-sm">
            <Icon name="plus" className="w-5 h-5" />Añadir persona
          </button>
        </div>
      )}
      {brand && (
        <div className="space-y-2">
          <span className="field-label">Perfiles</span>
          {NETWORKS.map(n => (
            <div key={n.key} className="flex items-center gap-2">
              <NetBadge network={n.key} />
              <input
                type="text"
                aria-label={n.label}
                value={form.socials[n.key] || ''}
                onChange={e => setForm({ ...form, socials: { ...form.socials, [n.key]: e.target.value } })}
                placeholder={`Enlace de ${n.label}`}
                className="field !py-2.5"
              />
            </div>
          ))}
        </div>
      )}
      <div>
        <label htmlFor="wo-alert" className="field-label">Feeds de alertas (uno por línea)</label>
        <textarea id="wo-alert" value={form.alerts} onChange={set('alerts')} rows={2} placeholder="https://www.google.com/alerts/feeds/…" className="field font-mono text-xs" />
      </div>
      {!brand && (
        <div>
          <label htmlFor="wo-yt" className="field-label">Canales de YouTube (uno por línea)</label>
          <textarea id="wo-yt" value={form.youtube} onChange={set('youtube')} placeholder="UCxxxxxxxxxxxxxx" rows={2} className="field" />
        </div>
      )}
    </div>
  );
}

// ── Pantalla ─────────────────────────────────────────────────────

export default function WatchlistManager() {
  const [orgs, setOrgs] = useState<WatchlistOrg[]>([]);
  const [maxBrands, setMaxBrands] = useState(10);
  const [selected, setSelected] = useState<string | null>(null);
  const [hits, setHits] = useState<WatchlistHit[]>([]);
  const [sourceFilter, setSourceFilter] = useState('all');
  const [tagFilter, setTagFilter] = useState('');
  const [loading, setLoading] = useState(true);
  const [hitsLoading, setHitsLoading] = useState(false);
  const [error, setError] = useState('');

  const [editing, setEditing] = useState<WatchlistOrg | null>(null);
  const [form, setForm] = useState<FormState | null>(null);
  const [saving, setSaving] = useState(false);
  const [scanning, setScanning] = useState(false);

  const [showRSS, setShowRSS] = useState(false);
  const [feedURL, setFeedURL] = useState('');
  const [feedCopied, setFeedCopied] = useState(false);

  const brands = orgs.filter(o => o.kind === 'marca');
  const topics = orgs.filter(o => o.kind !== 'marca');
  const current = orgs.find(o => o.id === selected) || null;
  const activeBrands = brands.filter(b => b.active).length;

  const fetchOrgs = useCallback(async () => {
    try {
      const data = await api.getWatchlistOrgs();
      setOrgs(data.orgs || []);
      if (data.max_brands) setMaxBrands(data.max_brands);
      return data.orgs || [];
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudo cargar.');
      return [];
    }
  }, []);

  const fetchHits = useCallback(async (orgID: string | null) => {
    setHitsLoading(true);
    try {
      const data = await api.getWatchlistHits(orgID ? { limit: 300, org_id: orgID } : { limit: 100 });
      setHits(data.hits || []);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudieron cargar las menciones.');
    } finally {
      setHitsLoading(false);
    }
  }, []);

  useEffect(() => {
    (async () => {
      const list = await fetchOrgs();
      const first = list.find(o => o.kind === 'marca') || list[0];
      if (first) setSelected(first.id);
      setLoading(false);
    })();
  }, [fetchOrgs]);

  useEffect(() => {
    setSourceFilter('all');
    setTagFilter('');
    if (selected) fetchHits(selected);
    else setHits([]);
  }, [selected, fetchHits]);

  // ── Acciones ──
  const openNew = (kind: WatchKind) => { setEditing(null); setForm(emptyForm(kind)); setError(''); };
  const openEdit = (o: WatchlistOrg) => { setEditing(o); setForm(formFromOrg(o)); setError(''); };
  const closeForm = () => { setForm(null); setEditing(null); };

  const save = async () => {
    if (!form || !form.name.trim()) return;
    setSaving(true);
    setError('');
    try {
      const input = inputFromForm(form, editing);
      const saved = editing ? await api.updateWatchlistOrg(editing.id, input) : await api.createWatchlistOrg(input);
      closeForm();
      await fetchOrgs();
      setSelected(saved.id);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudo guardar.');
    } finally {
      setSaving(false);
    }
  };

  const toggle = async (o: WatchlistOrg) => {
    setError('');
    try {
      await api.toggleWatchlistOrg(o.id, !o.active);
      await fetchOrgs();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudo cambiar.');
    }
  };

  const remove = async (o: WatchlistOrg) => {
    if (!confirm(`¿Borrar «${o.name}» y sus menciones?`)) return;
    try {
      await api.deleteWatchlistOrg(o.id);
      const list = await fetchOrgs();
      setSelected(list.find(x => x.kind === 'marca')?.id || list[0]?.id || null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudo borrar.');
    }
  };

  const markSeen = async (id: string) => {
    try {
      await api.markHitSeen(id);
      setHits(prev => prev.map(h => (h.id === id ? { ...h, seen: true } : h)));
      setOrgs(prev => prev.map(o => (o.id === selected ? { ...o, unseen: Math.max(0, o.unseen - 1) } : o)));
    } catch { /* se queda como estaba */ }
  };

  const markAll = async () => {
    if (!selected) return;
    try {
      await api.markOrgHitsSeen(selected);
      setHits(prev => prev.map(h => ({ ...h, seen: true })));
      setOrgs(prev => prev.map(o => (o.id === selected ? { ...o, unseen: 0 } : o)));
    } catch { /* se queda como estaba */ }
  };

  const removeHit = async (id: string) => {
    try {
      await api.deleteHit(id);
      setHits(prev => prev.filter(h => h.id !== id));
    } catch { /* se queda como estaba */ }
  };

  const scan = async () => {
    if (scanning || orgs.length === 0) return;
    setScanning(true);
    try {
      await api.triggerWatchlistScan();
      let polls = 0;
      const t = setInterval(async () => {
        polls++;
        await fetchOrgs();
        if (selected) await fetchHits(selected);
        if (polls >= 20) { clearInterval(t); setScanning(false); }
      }, 6000);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudo buscar.');
      setScanning(false);
    }
  };

  const openRSS = async () => {
    try {
      const data = await api.getWatchlistFeedURL();
      setFeedURL(window.location.origin + data.url);
      setFeedCopied(false);
      setShowRSS(true);
    } catch { /* nada */ }
  };

  // ── Menciones filtradas y agrupadas ──
  const sourceCounts = useMemo(() => {
    const c: Record<string, number> = {};
    for (const h of hits) c[h.source_type] = (c[h.source_type] || 0) + 1;
    return c;
  }, [hits]);
  const tagCounts = useMemo(() => {
    const c: Record<string, number> = {};
    for (const h of hits) if (h.matched_hashtag) c[h.matched_hashtag] = (c[h.matched_hashtag] || 0) + 1;
    return c;
  }, [hits]);
  const visible = hits.filter(h => (sourceFilter === 'all' || h.source_type === sourceFilter) && (!tagFilter || h.matched_hashtag === tagFilter));
  const groups = SOURCE_ORDER.map(s => ({ source: s, items: visible.filter(h => h.source_type === s) })).filter(g => g.items.length > 0);
  const unseenHere = hits.filter(h => !h.seen).length;

  if (loading) {
    return <div className="flex items-center justify-center py-20 text-primary"><Spinner className="w-8 h-8" /></div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <h1 className="title-xl">Marcas</h1>
          <span className="tag">{activeBrands}/{maxBrands}</span>
        </div>
        <div className="flex items-center gap-2">
          <button type="button" onClick={openRSS} className="btn btn-soft btn-sm">Feed RSS</button>
          <button type="button" onClick={() => openNew('marca')} disabled={activeBrands >= maxBrands} className="btn btn-tonal btn-sm">
            <Icon name="plus" className="w-5 h-5" />Añadir marca
          </button>
        </div>
      </div>

      {error && !form && <Notice>{error}</Notice>}

      {/* Baldosas */}
      {brands.length === 0 ? (
        <EmptyState icon="eye" text="Añade una marca para empezar." />
      ) : (
        <div className="stagger grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-3">
          {brands.map(b => {
            const on = b.id === selected;
            return (
              <button
                key={b.id}
                type="button"
                onClick={() => setSelected(b.id)}
                aria-pressed={on}
                className={`text-left p-5 rounded-[28px] transition-[border-radius,background-color] duration-500 ease-[var(--spring)] hover:rounded-[36px] ${on ? 'bg-primary-container text-on-primary-container' : 'bg-surface-container-low'} ${b.active ? '' : 'opacity-60'}`}
              >
                <div className="flex items-start justify-between gap-2">
                  <span className="title-md leading-tight">{b.name}</span>
                  {b.unseen > 0 && <span className="tag tag-accent shrink-0">{b.unseen}</span>}
                </div>
                <div className="mt-3 flex flex-wrap items-center gap-1.5">
                  {b.to_confirm && <span className="tag tag-bad">Por confirmar</span>}
                  {!b.active && <span className="tag">En pausa</span>}
                  {b.socials.map(s => <NetBadge key={s.network + s.handle} network={s.network} />)}
                </div>
              </button>
            );
          })}
        </div>
      )}

      {/* Ficha */}
      {current && (
        <section className="space-y-4" aria-label={`Ficha de ${current.name}`}>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-3 min-w-0">
              <h2 className="title-lg truncate">{current.name}</h2>
              {current.kind !== 'marca' && <span className="tag">{current.kind === 'persona' ? 'Persona' : 'Tema'}</span>}
            </div>
            <div className="flex items-center gap-1">
              <button type="button" onClick={() => openEdit(current)} className="icon-btn" title="Editar" aria-label="Editar"><Icon name="note" className="w-5 h-5" /></button>
              <button type="button" onClick={() => toggle(current)} className="btn btn-text btn-sm">{current.active ? 'Pausar' : 'Activar'}</button>
              <button type="button" onClick={() => remove(current)} className="icon-btn hover:text-error" title="Borrar" aria-label="Borrar"><Icon name="trash" className="w-5 h-5" /></button>
            </div>
          </div>

          {current.to_confirm && (
            <Notice kind="accent">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <span>Faltan datos de esta marca.</span>
                <button type="button" onClick={() => openEdit(current)} className="btn btn-filled btn-sm">Completar</button>
              </div>
            </Notice>
          )}

          {(current.website || current.socials.length > 0) && (
            <div className="chip-row">
              {current.website && (
                <a href={current.website} target="_blank" rel="noopener noreferrer" className="chip"><Icon name="globe" className="w-4 h-4" />Sitio</a>
              )}
              {current.socials.filter(s => s.url).map(s => (
                <a key={s.network + s.url} href={s.url} target="_blank" rel="noopener noreferrer" className="chip">
                  <Icon name="open" className="w-4 h-4" />Abrir en {NET[s.network]?.label || s.network}
                </a>
              ))}
            </div>
          )}

          {(current.people || []).length > 0 && (
            <div className="chip-row">
              {current.people.map(p => (
                p.url ? (
                  <a key={p.name} href={p.url} target="_blank" rel="noopener noreferrer" className="chip">{p.name}{p.role ? ` · ${p.role}` : ''}</a>
                ) : (
                  <span key={p.name} className="chip">{p.name}{p.role ? ` · ${p.role}` : ''}</span>
                )
              ))}
            </div>
          )}

          {current.hashtags.length > 0 && (
            <div className="space-y-2">
              {current.hashtags.map(t => (
                <div key={t} className="flex flex-wrap items-center gap-2">
                  <span className="text-sm font-extrabold">Ver #{t} en</span>
                  {HASHTAG_LINKS.map(l => (
                    <a key={l.label} href={l.url(encodeURIComponent(t))} target="_blank" rel="noopener noreferrer" className="btn btn-soft btn-sm">{l.label}</a>
                  ))}
                </div>
              ))}
            </div>
          )}

          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="chip-row chip-scroll" role="group" aria-label="Fuente">
              <button type="button" className="chip" aria-pressed={sourceFilter === 'all'} onClick={() => setSourceFilter('all')}>Todas · {hits.length}</button>
              {SOURCE_ORDER.filter(s => sourceCounts[s]).map(s => (
                <button key={s} type="button" className="chip" aria-pressed={sourceFilter === s} onClick={() => setSourceFilter(s)}>
                  {SOURCE_LABELS[s]} · {sourceCounts[s]}
                </button>
              ))}
            </div>
            {unseenHere > 0 && (
              <button type="button" onClick={markAll} className="btn btn-text btn-sm"><Icon name="check" className="w-5 h-5" />Marcar todo visto</button>
            )}
          </div>

          {Object.keys(tagCounts).length > 0 && (
            <div className="chip-row chip-scroll" role="group" aria-label="Hashtag">
              <button type="button" className="chip" aria-pressed={!tagFilter} onClick={() => setTagFilter('')}>Todos los hashtags</button>
              {Object.entries(tagCounts).map(([t, n]) => (
                <button key={t} type="button" className="chip" aria-pressed={tagFilter === t} onClick={() => setTagFilter(t)}>#{t} · {n}</button>
              ))}
            </div>
          )}

          {hitsLoading ? (
            <div className="flex justify-center py-10 text-primary"><Spinner className="w-7 h-7" /></div>
          ) : visible.length === 0 ? (
            <EmptyState icon="eye" text="Aún no hay menciones. Toca «Buscar ahora»." />
          ) : (
            <div className="space-y-6">
              {groups.map(g => (
                <div key={g.source} className="space-y-3">
                  <h3 className="title-md">{SOURCE_LABELS[g.source]} <span className="text-on-surface-variant">· {g.items.length}</span></h3>
                  <div className="stagger grid grid-cols-1 xl:grid-cols-2 gap-3 items-start">
                    {g.items.map(h => <HitCard key={h.id} hit={h} onSeen={() => markSeen(h.id)} onDelete={() => removeHit(h.id)} />)}
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>
      )}

      {/* Temas y personas */}
      <section className="space-y-3 pt-2" aria-label="Temas y personas">
        <div className="flex items-center justify-between gap-3">
          <h2 className="title-lg">Temas y personas</h2>
          <button type="button" onClick={() => openNew('tema')} className="btn btn-tonal btn-sm"><Icon name="plus" className="w-5 h-5" />Añadir</button>
        </div>
        {topics.length > 0 && (
          <div className="seg-list">
            {topics.map(o => (
              <button
                key={o.id}
                type="button"
                onClick={() => setSelected(o.id)}
                aria-pressed={o.id === selected}
                className={`p-4 text-left flex items-center justify-between gap-3 ${o.id === selected ? '!bg-primary-container text-on-primary-container' : ''}`}
              >
                <span className="flex items-center gap-2 min-w-0">
                  <span className={`w-3 h-3 rounded-full shrink-0 ${o.active ? 'bg-success' : 'bg-outline'}`} />
                  <span className="title-md truncate">{o.name}</span>
                  <span className="tag">{o.kind === 'persona' ? 'Persona' : 'Tema'}</span>
                </span>
                {o.unseen > 0 && <span className="tag tag-accent">{o.unseen}</span>}
              </button>
            ))}
          </div>
        )}
      </section>

      <div className="fab-slot">
        <button type="button" onClick={scan} disabled={scanning || orgs.length === 0} className="fab disabled:opacity-50">
          {scanning ? <Spinner className="w-6 h-6" /> : <Icon name="search" className="w-6 h-6" />}
          {scanning ? 'Buscando...' : 'Buscar ahora'}
        </button>
      </div>

      {form && (
        <Modal title={editing ? `Editar ${editing.name}` : form.kind === 'marca' ? 'Nueva marca' : 'Nuevo tema o persona'} onClose={closeForm}>
          <OrgForm form={form} setForm={setForm} />
          {error && <div className="mt-4"><Notice>{error}</Notice></div>}
          <div className="mt-6 flex justify-end gap-2">
            <button type="button" onClick={closeForm} className="btn btn-text">Cancelar</button>
            <button type="button" onClick={save} disabled={!form.name.trim() || saving} className="btn btn-filled">{saving ? 'Guardando...' : editing ? 'Guardar' : 'Crear'}</button>
          </div>
        </Modal>
      )}

      {showRSS && (
        <Modal title="Feed RSS" onClose={() => setShowRSS(false)}>
          <div className="flex items-center gap-2">
            <input type="text" readOnly value={feedURL} aria-label="Enlace del feed" className="field font-mono text-xs select-all" onFocus={e => e.target.select()} />
            <button
              type="button"
              onClick={async () => { try { await navigator.clipboard.writeText(feedURL); setFeedCopied(true); } catch { /* nada */ } }}
              className={`btn btn-sm shrink-0 ${feedCopied ? 'btn-tonal' : 'btn-filled'}`}
            >
              {feedCopied ? 'Copiado' : 'Copiar'}
            </button>
          </div>
          <div className="mt-4">
            <button
              type="button"
              onClick={async () => { try { const d = await api.regenerateWatchlistFeedURL(); setFeedURL(window.location.origin + d.url); setFeedCopied(false); } catch { /* nada */ } }}
              className="btn btn-danger btn-sm"
            >
              Cambiar enlace
            </button>
          </div>
        </Modal>
      )}
    </div>
  );
}

// ── Mención ──────────────────────────────────────────────────────

function HitCard({ hit, onSeen, onDelete }: { hit: WatchlistHit; onSeen: () => void; onDelete: () => void }) {
  let host = '';
  try { host = new URL(hit.url).hostname.replace(/^www\./, ''); } catch { /* sin host */ }
  return (
    <article className={`flex flex-col overflow-hidden rounded-[28px] hover:rounded-[36px] transition-[border-radius,background-color] duration-500 ease-[var(--spring)] ${hit.seen ? 'bg-surface-container-low' : 'bg-surface-container ring-[3px] ring-tertiary'}`}>
      <div className="px-5 pt-5 flex items-center justify-between gap-2 text-xs font-semibold text-on-surface-variant">
        <span className="truncate">{host}</span>
        <span className="shrink-0">{timeAgo(hitDate(hit))}</span>
      </div>
      <h4 className="px-5 pt-2 text-lg font-extrabold leading-tight tracking-tight line-clamp-3">
        <a href={hit.url} target="_blank" rel="noopener noreferrer" className="hover:text-primary transition-colors">{hit.title || 'Sin título'}</a>
      </h4>
      {hit.snippet && hit.snippet !== hit.title && <p className="px-5 pt-2 text-sm text-on-surface-variant leading-relaxed line-clamp-3">{hit.snippet}</p>}
      {(hit.matched_hashtag || hit.matched_person) && (
        <div className="px-5 pt-2 flex flex-wrap gap-1.5">
          {hit.matched_person && <span className="tag tag-accent">{hit.matched_person}</span>}
          {hit.matched_hashtag && <span className="tag tag-primary">#{hit.matched_hashtag}</span>}
        </div>
      )}
      <div className="flex items-center gap-2 p-3 pt-4">
        {!hit.seen && (
          <button type="button" onClick={onSeen} className="btn btn-filled btn-sm"><Icon name="check" className="w-5 h-5" />Marcar visto</button>
        )}
        <button type="button" onClick={onDelete} className="btn btn-soft btn-sm"><Icon name="trash" className="w-5 h-5" />Quitar</button>
        <a href={hit.url} target="_blank" rel="noopener noreferrer" className="icon-btn ml-auto" title="Abrir original" aria-label="Abrir original"><Icon name="open" /></a>
      </div>
    </article>
  );
}
