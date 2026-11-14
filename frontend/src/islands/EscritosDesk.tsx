import { useState, useEffect, useCallback } from 'react';
import { api, type Escrito, type EscritoSource, type SEOScore } from '../lib/api';

const FORMAT_OPTIONS = [
  { value: 'auto', label: 'Auto' },
  { value: 'article', label: 'Articulo' },
  { value: 'newsletter', label: 'Newsletter' },
  { value: 'report', label: 'Reporte' },
  { value: 'memo', label: 'Memo' },
  { value: 'letter', label: 'Carta' },
  { value: 'email', label: 'Email' },
  { value: 'op_ed', label: 'Opinion' },
] as const;

const STYLE_OPTIONS = [
  { value: 'auto', label: 'Auto' },
  { value: 'academic', label: 'Academico' },
  { value: 'journalistic', label: 'Periodistico' },
  { value: 'executive', label: 'Ejecutivo' },
  { value: 'institutional', label: 'Institucional' },
  { value: 'persuasive', label: 'Persuasivo' },
  { value: 'newsletter', label: 'Newsletter' },
] as const;

const AUDIENCE_OPTIONS = [
  { value: 'general', label: 'General' },
  { value: 'academic', label: 'Academica' },
  { value: 'executive', label: 'Ejecutiva' },
  { value: 'public_sector', label: 'Sector publico' },
  { value: 'community', label: 'Comunidad' },
  { value: 'donor', label: 'Donantes' },
] as const;

const CITATION_OPTIONS = [
  { value: 'apa', label: 'APA' },
  { value: 'mla', label: 'MLA' },
  { value: 'chicago', label: 'Chicago' },
  { value: 'inline', label: 'Inline' },
] as const;

const STATUS_STYLES: Record<string, { bg: string; text: string; label: string; emoji: string; border: string }> = {
  queued:     { bg: 'bg-zinc-100 dark:bg-zinc-800',        text: 'text-zinc-600 dark:text-zinc-300',      label: 'En cola',      emoji: '\u23f3', border: 'border-zinc-300 dark:border-zinc-600' },
  planning:   { bg: 'bg-blue-100 dark:bg-blue-900/40',     text: 'text-blue-700 dark:text-blue-300',      label: 'Planificando', emoji: '\ud83d\udcdd', border: 'border-blue-300 dark:border-blue-700' },
  generating: { bg: 'bg-amber-100 dark:bg-amber-900/40',   text: 'text-amber-700 dark:text-amber-300',    label: 'Generando',    emoji: '\u2712\ufe0f', border: 'border-amber-300 dark:border-amber-700' },
  scoring:    { bg: 'bg-purple-100 dark:bg-purple-900/40',  text: 'text-purple-700 dark:text-purple-300',  label: 'Puntuando',    emoji: '\ud83d\udcca', border: 'border-purple-300 dark:border-purple-700' },
  verifying:  { bg: 'bg-teal-100 dark:bg-teal-900/40',      text: 'text-teal-700 dark:text-teal-300',      label: 'Verificando',  emoji: '\ud83d\udd0e', border: 'border-teal-300 dark:border-teal-700' },
  improving:  { bg: 'bg-cyan-100 dark:bg-cyan-900/40',     text: 'text-cyan-700 dark:text-cyan-300',      label: 'Mejorando',    emoji: '\u2728', border: 'border-cyan-300 dark:border-cyan-700' },
  done:       { bg: 'bg-emerald-100 dark:bg-emerald-900/40',text: 'text-emerald-700 dark:text-emerald-300',label: 'Completado',   emoji: '\u2705', border: 'border-emerald-300 dark:border-emerald-700' },
  failed:     { bg: 'bg-red-100 dark:bg-red-900/40',       text: 'text-red-700 dark:text-red-300',        label: 'Error',        emoji: '\u274c', border: 'border-red-300 dark:border-red-700' },
};

const PUBLISH_STYLES: Record<string, { label: string; color: string }> = {
  draft:     { label: 'Borrador',   color: 'text-zinc-500 bg-zinc-100 dark:bg-zinc-800' },
  reviewing: { label: 'En revision', color: 'text-amber-600 bg-amber-100 dark:bg-amber-900/30' },
  published: { label: 'Publicado',  color: 'text-emerald-600 bg-emerald-100 dark:bg-emerald-900/30' },
};

const PHASE_MESSAGES: Record<string, string> = {
  queued:     'Preparando generacion...',
  planning:   'Fase 1-2 de 5 — Recopilando fuentes y planificando estructura...',
  generating: 'Fase 3 de 5 — Generando contenido seccion por seccion...',
  scoring:    'Fase 4 de 5 — Calculando puntuacion SEO...',
  verifying:  'Fase 5 de 5 — Verificando citas, estructura y export markdown...',
  improving:  'Mejorando articulo con tus instrucciones...',
};

const ACTIVE_STATUSES = ['queued', 'planning', 'generating', 'scoring', 'verifying', 'improving'];
const TOTAL_GENERATION_PHASES = 5;

type Tab = 'contenido' | 'seo' | 'fuentes' | 'exportar';

function timeAgo(dateStr: string): string {
  const mins = Math.floor((Date.now() - new Date(dateStr).getTime()) / 60000);
  if (mins < 1) return 'ahora';
  if (mins < 60) return `${mins}m`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h`;
  return `${Math.floor(hrs / 24)}d`;
}

export default function EscritosDesk() {
  const [escritos, setEscritos] = useState<Escrito[]>([]);
  const [selected, setSelected] = useState<Escrito | null>(null);
  const [sources, setSources] = useState<EscritoSource[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<Tab>('contenido');
  const [showCreate, setShowCreate] = useState(false);
  const [newTopic, setNewTopic] = useState('');
  const [showAdvancedCreate, setShowAdvancedCreate] = useState(false);
  const [createFormat, setCreateFormat] = useState<Escrito['output_format']>('auto');
  const [createStyle, setCreateStyle] = useState<Escrito['writing_style']>('auto');
  const [createAudience, setCreateAudience] = useState<Escrito['target_audience']>('general');
  const [createCitation, setCreateCitation] = useState<Escrito['citation_style']>('apa');
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState(false);
  const [showImprove, setShowImprove] = useState(false);
  const [improveInstructions, setImproveInstructions] = useState('');
  const [improving, setImproving] = useState(false);

  const loadEscritos = useCallback(async () => {
    try {
      const data = await api.getEscritos();
      setEscritos(data.escritos || []);
    } catch (e) {
      console.error('load escritos:', e);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadDetail = useCallback(async (id: string) => {
    try {
      const data = await api.getEscrito(id);
      setSelected(data.escrito);
      setSources(data.sources || []);
    } catch (e) {
      console.error('load escrito detail:', e);
    }
  }, []);

  const selectEscrito = useCallback(async (e: Escrito) => {
    setSelected(e);
    setActiveTab('contenido');
    setEditing(false);
    loadDetail(e.id);
  }, [loadDetail]);

  useEffect(() => { loadEscritos(); }, [loadEscritos]);

  // Auto-poll active escritos every 5s
  useEffect(() => {
    const hasActive = escritos.some(e =>
      ACTIVE_STATUSES.includes(e.status)
    );
    if (!hasActive) return;

    const interval = setInterval(async () => {
      await loadEscritos();
      if (selected && ['queued', 'planning', 'generating', 'scoring', 'verifying', 'improving'].includes(selected.status)) {
        loadDetail(selected.id);
      }
    }, 5000);

    return () => clearInterval(interval);
  }, [escritos, selected, loadEscritos, loadDetail]);

  const handleCreate = async () => {
    if (!newTopic.trim()) return;
    setCreating(true);
    try {
      const escrito = await api.createEscrito(newTopic.trim(), [], {
        output_format: createFormat,
        writing_style: createStyle,
        target_audience: createAudience,
        citation_style: createCitation,
      });
      setNewTopic('');
      setShowCreate(false);
      setShowAdvancedCreate(false);
      setCreateFormat('auto');
      setCreateStyle('auto');
      setCreateAudience('general');
      setCreateCitation('apa');
      await loadEscritos();
      selectEscrito(escrito);
    } catch (e) {
      console.error('create escrito:', e);
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Eliminar este escrito y todas sus fuentes?')) return;
    try {
      await api.deleteEscrito(id);
      if (selected?.id === id) {
        setSelected(null);
        setSources([]);
      }
      loadEscritos();
    } catch (e) {
      console.error('delete escrito:', e);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin w-8 h-8 border-3 border-indigo-500 border-t-transparent rounded-full" />
      </div>
    );
  }

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-black tracking-tight text-zinc-900 dark:text-zinc-50 uppercase">
            Escritos SEO
          </h1>
          <p className="text-sm text-zinc-500 dark:text-zinc-400 mt-1">
            Genera articulos SEO optimizados desde tus noticias
          </p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="px-5 py-2.5 text-sm font-bold bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition-colors shadow-lg shadow-indigo-500/20 uppercase tracking-wide"
        >
          + Nuevo Escrito
        </button>
      </div>

      {/* Create modal */}
      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-white dark:bg-zinc-900 rounded-xl p-8 w-full max-w-lg border border-zinc-200 dark:border-zinc-700 shadow-2xl animate-fade-in">
            <h2 className="text-xl font-black text-zinc-900 dark:text-zinc-50 mb-1 uppercase">Nuevo Escrito</h2>
            <p className="text-sm text-zinc-500 dark:text-zinc-400 mb-6">
              Ingresa un tema. El sistema buscara noticias relacionadas, planificara la estructura y generara un articulo SEO optimizado. El proceso toma 3-8 minutos.
            </p>
            <label className="block text-sm font-bold text-zinc-700 dark:text-zinc-300 mb-2 uppercase tracking-wide">Tema del articulo</label>
            <input
              type="text"
              value={newTopic}
              onChange={(e) => setNewTopic(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
              placeholder="ej: energia renovable en Puerto Rico, crisis del agua..."
              className="w-full px-4 py-3 text-base bg-zinc-50 dark:bg-zinc-800 border-2 border-zinc-300 dark:border-zinc-600 rounded-lg text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 mb-6"
              autoFocus
            />
            <div className="flex gap-3 justify-end">
              <button
              onClick={() => { setShowCreate(false); setNewTopic(''); }}
              className="px-5 py-2.5 text-sm font-bold text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-100 transition-colors"
            >
              Cancelar
            </button>
              <button
                onClick={handleCreate}
                disabled={creating || !newTopic.trim()}
                className="px-6 py-2.5 text-sm font-bold bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 disabled:opacity-50 transition-colors shadow-lg shadow-indigo-500/20 uppercase"
              >
                {creating ? 'Generando...' : 'Generar Articulo'}
              </button>
            </div>
            <div className="mt-4 border-t border-zinc-200 dark:border-zinc-800 pt-4">
              <button
                type="button"
                onClick={() => setShowAdvancedCreate((v) => !v)}
                className="text-xs font-bold uppercase tracking-wide text-indigo-600 dark:text-indigo-400"
              >
                {showAdvancedCreate ? 'Ocultar opciones avanzadas' : 'Mostrar opciones avanzadas'}
              </button>
              {showAdvancedCreate && (
                <div className="grid grid-cols-2 gap-3 mt-4">
                  <label className="block">
                    <span className="block text-[11px] font-bold uppercase tracking-wide text-zinc-500 dark:text-zinc-400 mb-1">Formato</span>
                    <select value={createFormat} onChange={(e) => setCreateFormat(e.target.value as Escrito['output_format'])} className="w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-800 border border-zinc-300 dark:border-zinc-600 rounded-lg">
                      {FORMAT_OPTIONS.map((opt) => <option key={opt.value} value={opt.value}>{opt.label}</option>)}
                    </select>
                  </label>
                  <label className="block">
                    <span className="block text-[11px] font-bold uppercase tracking-wide text-zinc-500 dark:text-zinc-400 mb-1">Estilo</span>
                    <select value={createStyle} onChange={(e) => setCreateStyle(e.target.value as Escrito['writing_style'])} className="w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-800 border border-zinc-300 dark:border-zinc-600 rounded-lg">
                      {STYLE_OPTIONS.map((opt) => <option key={opt.value} value={opt.value}>{opt.label}</option>)}
                    </select>
                  </label>
                  <label className="block">
                    <span className="block text-[11px] font-bold uppercase tracking-wide text-zinc-500 dark:text-zinc-400 mb-1">Audiencia</span>
                    <select value={createAudience} onChange={(e) => setCreateAudience(e.target.value as Escrito['target_audience'])} className="w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-800 border border-zinc-300 dark:border-zinc-600 rounded-lg">
                      {AUDIENCE_OPTIONS.map((opt) => <option key={opt.value} value={opt.value}>{opt.label}</option>)}
                    </select>
                  </label>
                  <label className="block">
                    <span className="block text-[11px] font-bold uppercase tracking-wide text-zinc-500 dark:text-zinc-400 mb-1">Citas</span>
                    <select value={createCitation} onChange={(e) => setCreateCitation(e.target.value as Escrito['citation_style'])} className="w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-800 border border-zinc-300 dark:border-zinc-600 rounded-lg">
                      {CITATION_OPTIONS.map((opt) => <option key={opt.value} value={opt.value}>{opt.label}</option>)}
                    </select>
                  </label>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Two-panel layout */}
      <div className="flex gap-8 min-h-[650px]">
        {/* Left panel: escrito list */}
        <div className="w-72 shrink-0">
          <h3 className="text-xs font-bold text-zinc-500 dark:text-zinc-400 uppercase tracking-widest mb-3">Articulos</h3>
          <div className="space-y-3">
            {escritos.length === 0 && (
              <div className="text-center py-8 border-2 border-dashed border-zinc-300 dark:border-zinc-700 rounded-xl">
                <div className="text-3xl mb-2">&#9997;&#65039;</div>
                <p className="text-sm text-zinc-500 dark:text-zinc-400 font-medium">Sin escritos</p>
                <p className="text-xs text-zinc-400 dark:text-zinc-500 mt-1">Crea uno para comenzar</p>
              </div>
            )}
            {escritos.map((e) => {
              const st = STATUS_STYLES[e.status] || STATUS_STYLES.queued;
              const isActive = selected?.id === e.id;
              const isRunning = ACTIVE_STATUSES.includes(e.status);
              const seoTotal = e.seo_score?.total || 0;
              return (
                <button
                  key={e.id}
                  onClick={() => selectEscrito(e)}
                  className={`w-full text-left px-4 py-3.5 rounded-xl border-2 transition-all ${
                    isActive
                      ? 'bg-indigo-50 dark:bg-indigo-950/40 border-indigo-400 dark:border-indigo-600 shadow-md shadow-indigo-500/10'
                      : 'bg-white dark:bg-zinc-900 border-zinc-200 dark:border-zinc-800 hover:border-zinc-300 dark:hover:border-zinc-700 hover:shadow-sm'
                  }`}
                >
                  <div className="font-bold text-base text-zinc-900 dark:text-zinc-100 truncate capitalize">
                    {e.title || e.topic}
                  </div>
                  <div className="flex items-center gap-2 mt-1.5">
                    <span className={`inline-flex items-center gap-1 px-2 py-0.5 text-xs font-bold rounded-md ${st.bg} ${st.text} border ${st.border}`}>
                      {st.emoji} {st.label}
                    </span>
                    <span className="text-xs text-zinc-400 font-medium">{timeAgo(e.created_at)}</span>
                    {e.status === 'done' && seoTotal > 0 && (
                      <span className={`text-xs font-bold px-1.5 py-0.5 rounded ${
                        seoTotal >= 80 ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-600' :
                        seoTotal >= 60 ? 'bg-amber-100 dark:bg-amber-900/30 text-amber-600' :
                        'bg-red-100 dark:bg-red-900/30 text-red-600'
                      }`}>
                        {seoTotal}
                      </span>
                    )}
                  </div>
                  {isRunning && (
                    <div className="flex items-center gap-1.5 mt-2">
                      <div className="animate-spin w-3 h-3 border-[1.5px] border-indigo-500 border-t-transparent rounded-full" />
                      <span className="text-[11px] text-indigo-500 dark:text-indigo-400 font-medium">En progreso...</span>
                    </div>
                  )}
                  {e.word_count > 0 && (
                    <div className="text-xs text-zinc-500 dark:text-zinc-400 mt-1 font-medium">
                      {e.word_count} palabras
                    </div>
                  )}
                  <div className="mt-2 flex flex-wrap gap-1">
                    <span className="px-2 py-0.5 text-[10px] font-semibold rounded bg-zinc-100 dark:bg-zinc-800 text-zinc-600 dark:text-zinc-400 uppercase">{e.output_format || 'article'}</span>
                    <span className="px-2 py-0.5 text-[10px] font-semibold rounded bg-zinc-100 dark:bg-zinc-800 text-zinc-600 dark:text-zinc-400 uppercase">{e.writing_style || 'journalistic'}</span>
                    <span className="px-2 py-0.5 text-[10px] font-semibold rounded bg-zinc-100 dark:bg-zinc-800 text-zinc-600 dark:text-zinc-400 uppercase">{e.citation_style || 'apa'}</span>
                  </div>
                </button>
              );
            })}
          </div>
        </div>

        {/* Right panel: detail */}
        <div className="flex-1 min-w-0">
          {!selected ? (
            <div className="flex flex-col items-center justify-center h-full text-center">
              <div className="text-5xl mb-4">&#9997;&#65039;</div>
              <p className="text-lg font-bold text-zinc-600 dark:text-zinc-400">Selecciona un escrito</p>
              <p className="text-sm text-zinc-400 dark:text-zinc-500 mt-1">o crea uno nuevo para comenzar</p>
            </div>
          ) : (
            <div>
              {/* Header */}
              <div className="flex items-start justify-between mb-6">
                <div className="flex-1 min-w-0">
                  <h2 className="text-2xl font-black text-zinc-900 dark:text-zinc-50 uppercase tracking-tight">
                    {selected.title || selected.topic}
                  </h2>
                  <div className="flex items-center gap-3 mt-2 flex-wrap">
                    <StatusBadge status={selected.status} />
                    <PublishBadge status={selected.publish_status} />
                    <span className="text-sm font-bold text-zinc-500 dark:text-zinc-400">
                      Fase {Math.min(selected.phase || 0, TOTAL_GENERATION_PHASES)}/{TOTAL_GENERATION_PHASES}
                    </span>
                    {selected.word_count > 0 && (
                      <span className="text-sm text-zinc-500 dark:text-zinc-400">
                        {selected.word_count} palabras
                      </span>
                    )}
                  </div>

                  {/* Live progress */}
                  {['queued', 'planning', 'generating', 'scoring', 'verifying'].includes(selected.status) && (
                    <div className="mt-4 p-4 rounded-xl bg-indigo-50 dark:bg-indigo-950/30 border-2 border-indigo-200 dark:border-indigo-800">
                      <div className="w-full h-2 bg-indigo-200 dark:bg-indigo-900 rounded-full overflow-hidden mb-3">
                        <div
                          className="h-full bg-indigo-500 rounded-full transition-all duration-1000"
                          style={{
                            width: selected.status === 'queued' ? '5%' : `${Math.max((Math.min(selected.phase || 0, TOTAL_GENERATION_PHASES) / TOTAL_GENERATION_PHASES) * 100, 8)}%`,
                            animation: 'pulse 2s ease-in-out infinite',
                          }}
                        />
                      </div>
                      <div className="flex items-center gap-2.5">
                        <div className="animate-spin w-4 h-4 border-2 border-indigo-500 border-t-transparent rounded-full shrink-0" />
                        <span className="text-sm font-bold text-indigo-700 dark:text-indigo-300">
                          {PHASE_MESSAGES[selected.status] || 'Procesando...'}
                        </span>
                      </div>
                      {selected.progress?.current_section && (
                        <div className="text-xs text-indigo-600 dark:text-indigo-400 font-medium mt-2">
                          Seccion {selected.progress.current_section}/{selected.progress.total_sections}: {selected.progress.section_name}
                        </div>
                      )}
                    </div>
                  )}
                </div>
                <div className="flex gap-2 shrink-0 ml-4">
                  {(selected.status === 'done' || selected.status === 'failed') && (
                    <>
                      <button
                        onClick={() => setShowImprove(true)}
                        className="px-4 py-2 text-sm font-bold text-cyan-600 dark:text-cyan-400 border-2 border-cyan-300 dark:border-cyan-700 rounded-lg hover:bg-cyan-50 dark:hover:bg-cyan-950/20 transition-colors"
                      >
                        Mejorar
                      </button>
                      <button
                        onClick={async () => {
                          await api.regenerateEscrito(selected.id);
                          loadDetail(selected.id);
                          loadEscritos();
                        }}
                        className="px-4 py-2 text-sm font-bold text-indigo-600 dark:text-indigo-400 border-2 border-indigo-300 dark:border-indigo-700 rounded-lg hover:bg-indigo-50 dark:hover:bg-indigo-950/20 transition-colors"
                      >
                        Regenerar
                      </button>
                    </>
                  )}
                  <button
                    onClick={() => handleDelete(selected.id)}
                    className="px-4 py-2 text-sm font-bold text-zinc-500 dark:text-zinc-400 border-2 border-zinc-300 dark:border-zinc-700 rounded-lg hover:bg-zinc-50 dark:hover:bg-zinc-800/50 transition-colors"
                  >
                    Eliminar
                  </button>
                </div>
              </div>

              {/* Error */}
              {selected.progress?.error && (
                <div className="mb-6 p-4 bg-red-50 dark:bg-red-950/30 border-2 border-red-300 dark:border-red-800 rounded-xl text-sm font-medium text-red-700 dark:text-red-300">
                  <span className="font-bold">Error:</span> {selected.progress.error}
                </div>
              )}

              {/* Tabs */}
              <div className="flex gap-0 mb-6 border-b-2 border-zinc-200 dark:border-zinc-800">
                {(['contenido', 'seo', 'fuentes', 'exportar'] as Tab[]).map((tab) => (
                  <button
                    key={tab}
                    onClick={() => { setActiveTab(tab); setEditing(false); }}
                    className={`px-5 py-3 text-sm font-bold uppercase tracking-wide border-b-3 transition-colors ${
                      activeTab === tab
                        ? 'border-indigo-500 text-indigo-600 dark:text-indigo-400 bg-indigo-50/50 dark:bg-indigo-950/20'
                        : 'border-transparent text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-100 hover:bg-zinc-50 dark:hover:bg-zinc-800/30'
                    }`}
                  >
                    {tab === 'contenido' && '\ud83d\udcdd '}
                    {tab === 'seo' && '\ud83d\udcca '}
                    {tab === 'fuentes' && '\ud83d\udcc4 '}
                    {tab === 'exportar' && '\ud83d\udce4 '}
                    {tab}
                  </button>
                ))}
              </div>

              {/* Tab content */}
              {activeTab === 'contenido' && (
                <ContentTab
                  escrito={selected}
                  editing={editing}
                  setEditing={setEditing}
                  onSave={async (data) => {
                    await api.updateEscrito(selected.id, data);
                    loadDetail(selected.id);
                    loadEscritos();
                    setEditing(false);
                  }}
                />
              )}
              {activeTab === 'seo' && (
                <SEOTab
                  escrito={selected}
                  onRecalc={async () => {
                    await api.recalcSEO(selected.id);
                    loadDetail(selected.id);
                  }}
                  onOptimize={async (instructions) => {
                    await api.optimizeEscritoSEO(selected.id, instructions);
                    loadDetail(selected.id);
                    loadEscritos();
                  }}
                  onExpand={async (instructions) => {
                    await api.expandEscrito(selected.id, instructions);
                    loadDetail(selected.id);
                    loadEscritos();
                  }}
                />
              )}
              {activeTab === 'fuentes' && <SourcesTab sources={sources} />}
              {activeTab === 'exportar' && <ExportTab escrito={selected} />}
            </div>
          )}
        </div>
      </div>

      {/* Improve Modal */}
      {showImprove && selected && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div className="w-full max-w-lg mx-4 bg-white dark:bg-zinc-900 rounded-2xl border-2 border-zinc-200 dark:border-zinc-700 shadow-2xl animate-fade-in">
            <div className="p-6">
              <h3 className="text-lg font-bold text-zinc-900 dark:text-zinc-100 mb-1">Mejorar Articulo</h3>
              <p className="text-sm text-zinc-500 dark:text-zinc-400 mb-4">
                Describe que quieres mejorar. El AI reescribira el articulo segun tus instrucciones.
              </p>
              <textarea
                value={improveInstructions}
                onChange={(e) => setImproveInstructions(e.target.value)}
                placeholder="Ej: Agrega mas datos estadisticos, mejora la seccion de conclusiones, incluye mas fuentes de gobierno..."
                rows={4}
                className="w-full px-4 py-3 text-sm bg-zinc-50 dark:bg-zinc-800 border-2 border-zinc-300 dark:border-zinc-600 rounded-xl text-zinc-900 dark:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-cyan-500 resize-none placeholder:text-zinc-400"
                autoFocus
              />
              <div className="flex justify-end gap-3 mt-4">
                <button
                  onClick={() => { setShowImprove(false); setImproveInstructions(''); }}
                  className="px-4 py-2.5 text-sm font-bold text-zinc-500 dark:text-zinc-400 border-2 border-zinc-300 dark:border-zinc-700 rounded-lg hover:bg-zinc-50 dark:hover:bg-zinc-800/50 transition-colors"
                >
                  Cancelar
                </button>
                <button
                  onClick={async () => {
                    if (!improveInstructions.trim()) return;
                    setImproving(true);
                    try {
                      await api.improveEscrito(selected.id, improveInstructions.trim());
                      setShowImprove(false);
                      setImproveInstructions('');
                      loadDetail(selected.id);
                      loadEscritos();
                    } catch (e) {
                      console.error('improve failed:', e);
                    } finally {
                      setImproving(false);
                    }
                  }}
                  disabled={improving || !improveInstructions.trim()}
                  className="px-5 py-2.5 text-sm font-bold bg-cyan-600 text-white rounded-lg hover:bg-cyan-700 disabled:opacity-50 transition-colors"
                >
                  {improving ? 'Enviando...' : 'Mejorar'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const st = STATUS_STYLES[status] || STATUS_STYLES.queued;
  return (
    <span className={`inline-flex items-center gap-1.5 px-3 py-1 text-sm font-bold rounded-lg ${st.bg} ${st.text} border ${st.border}`}>
      {st.emoji} {st.label}
    </span>
  );
}

function PublishBadge({ status }: { status: string }) {
  const st = PUBLISH_STYLES[status] || PUBLISH_STYLES.draft;
  return (
    <span className={`px-2.5 py-1 text-xs font-bold rounded-lg ${st.color}`}>
      {st.label}
    </span>
  );
}

// ── Content Tab ──────────────────────────────────────────────────

function ContentTab({
  escrito,
  editing,
  setEditing,
  onSave,
}: {
  escrito: Escrito;
  editing: boolean;
  setEditing: (v: boolean) => void;
  onSave: (data: { title?: string; meta_description?: string; content?: string; publish_status?: string }) => Promise<void>;
}) {
  const [title, setTitle] = useState(escrito.title);
  const [metaDesc, setMetaDesc] = useState(escrito.meta_description);
  const [content, setContent] = useState(escrito.content);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setTitle(escrito.title);
    setMetaDesc(escrito.meta_description);
    setContent(escrito.content);
  }, [escrito.id, escrito.title, escrito.meta_description, escrito.content]);

  if (!escrito.content && escrito.status !== 'done' && escrito.status !== 'failed') {
    return (
      <div className="py-12 text-center border-2 border-dashed border-zinc-300 dark:border-zinc-700 rounded-xl">
        <div className="flex justify-center mb-4">
          <div className="animate-spin w-8 h-8 border-3 border-indigo-500 border-t-transparent rounded-full" />
        </div>
        <p className="text-base font-bold text-zinc-600 dark:text-zinc-400">Generando contenido...</p>
        <p className="text-sm text-zinc-400 dark:text-zinc-500 mt-1">El articulo estara disponible cuando se complete la generacion</p>
      </div>
    );
  }

  if (!escrito.content) {
    return (
      <div className="py-12 text-center border-2 border-dashed border-zinc-300 dark:border-zinc-700 rounded-xl">
        <div className="text-4xl mb-3">&#128221;</div>
        <p className="text-base font-bold text-zinc-500 dark:text-zinc-400">Sin contenido generado</p>
      </div>
    );
  }

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave({ title, meta_description: metaDesc, content });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div>
      {escrito.status === 'improving' && (
        <div className="mb-4 p-4 rounded-xl bg-cyan-50 dark:bg-cyan-950/30 border-2 border-cyan-200 dark:border-cyan-800">
          <div className="flex items-center gap-2.5">
            <div className="animate-spin w-4 h-4 border-2 border-cyan-500 border-t-transparent rounded-full shrink-0" />
            <span className="text-sm font-bold text-cyan-700 dark:text-cyan-300">
              Optimizando el articulo. Puedes seguir leyendo la version actual mientras llega la nueva.
            </span>
          </div>
        </div>
      )}

      {/* Toolbar */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex gap-2">
          {!editing ? (
            <button
              onClick={() => setEditing(true)}
              className="px-4 py-2 text-sm font-bold text-indigo-600 dark:text-indigo-400 border-2 border-indigo-300 dark:border-indigo-700 rounded-lg hover:bg-indigo-50 dark:hover:bg-indigo-950/20 transition-colors"
            >
              Editar
            </button>
          ) : (
            <>
              <button
                onClick={handleSave}
                disabled={saving}
                className="px-4 py-2 text-sm font-bold bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 disabled:opacity-50 transition-colors"
              >
                {saving ? 'Guardando...' : 'Guardar'}
              </button>
              <button
                onClick={() => {
                  setEditing(false);
                  setTitle(escrito.title);
                  setMetaDesc(escrito.meta_description);
                  setContent(escrito.content);
                }}
                className="px-4 py-2 text-sm font-bold text-zinc-500 dark:text-zinc-400 border-2 border-zinc-300 dark:border-zinc-700 rounded-lg hover:bg-zinc-50 dark:hover:bg-zinc-800/50 transition-colors"
              >
                Cancelar
              </button>
            </>
          )}
        </div>
        {/* Publish status toggle */}
        <div className="flex gap-2">
          {(['draft', 'reviewing', 'published'] as const).map((ps) => (
            <button
              key={ps}
              onClick={() => onSave({ publish_status: ps })}
              className={`px-3 py-1.5 text-xs font-bold rounded-lg transition-colors border ${
                escrito.publish_status === ps
                  ? 'bg-indigo-100 dark:bg-indigo-900/40 text-indigo-700 dark:text-indigo-300 border-indigo-300 dark:border-indigo-700'
                  : 'bg-white dark:bg-zinc-900 text-zinc-500 dark:text-zinc-400 border-zinc-200 dark:border-zinc-700 hover:border-zinc-300'
              }`}
            >
              {PUBLISH_STYLES[ps].label}
            </button>
          ))}
        </div>
      </div>

      {editing ? (
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-bold text-zinc-500 uppercase mb-1">
              Titulo <span className="text-zinc-400 font-normal">({title.length}/60 chars)</span>
            </label>
            <input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              className="w-full px-4 py-2.5 text-base bg-zinc-50 dark:bg-zinc-800 border-2 border-zinc-300 dark:border-zinc-600 rounded-lg text-zinc-900 dark:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
            />
          </div>
          <div>
            <label className="block text-xs font-bold text-zinc-500 uppercase mb-1">
              Meta Description <span className="text-zinc-400 font-normal">({metaDesc.length}/160 chars)</span>
            </label>
            <textarea
              value={metaDesc}
              onChange={(e) => setMetaDesc(e.target.value)}
              rows={2}
              className="w-full px-4 py-2.5 text-sm bg-zinc-50 dark:bg-zinc-800 border-2 border-zinc-300 dark:border-zinc-600 rounded-lg text-zinc-900 dark:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-indigo-500 resize-none"
            />
          </div>
          <div>
            <label className="block text-xs font-bold text-zinc-500 uppercase mb-1">Contenido (Markdown)</label>
            <textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              rows={25}
              className="w-full px-4 py-3 text-sm font-mono bg-zinc-50 dark:bg-zinc-800 border-2 border-zinc-300 dark:border-zinc-600 rounded-lg text-zinc-900 dark:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-indigo-500 resize-y"
            />
          </div>
        </div>
      ) : (
        <div className="space-y-4">
          {/* Title + meta preview */}
          <div className="p-4 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl">
            <div className="text-lg font-bold text-blue-700 dark:text-blue-400 mb-1">{escrito.title}</div>
            <div className="text-sm text-emerald-700 dark:text-emerald-500 mb-1">{escrito.slug && `folio.pr/${escrito.slug}`}</div>
            <div className="text-sm text-zinc-600 dark:text-zinc-400">{escrito.meta_description}</div>
          </div>
          {/* Keywords */}
          {escrito.keywords?.length > 0 && (
            <div className="flex gap-2 flex-wrap">
              {escrito.keywords.map((kw, i) => (
                <span key={i} className="text-xs font-medium px-2.5 py-1 rounded-lg bg-indigo-50 dark:bg-indigo-950/30 text-indigo-600 dark:text-indigo-400 border border-indigo-200 dark:border-indigo-800">
                  {kw}
                </span>
              ))}
            </div>
          )}
          {/* Content */}
          <div className="p-6 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl">
            <div
              className="text-base text-zinc-700 dark:text-zinc-300 leading-relaxed"
              dangerouslySetInnerHTML={{ __html: renderMarkdown(escrito.content) }}
            />
          </div>
        </div>
      )}
    </div>
  );
}

// ── SEO Tab ──────────────────────────────────────────────────────

function SEOTab({
  escrito,
  onRecalc,
  onOptimize,
  onExpand,
}: {
  escrito: Escrito;
  onRecalc: () => Promise<void>;
  onOptimize: (instructions: string) => Promise<void>;
  onExpand: (instructions: string) => Promise<void>;
}) {
  const [recalculating, setRecalculating] = useState(false);
  const [optimizing, setOptimizing] = useState(false);
  const [expanding, setExpanding] = useState(false);
  const [instructions, setInstructions] = useState('');
  const score = escrito.seo_score;

  const handleRecalc = async () => {
    setRecalculating(true);
    try {
      await onRecalc();
    } finally {
      setRecalculating(false);
    }
  };

  const handleOptimize = async (preset?: string) => {
    const finalInstructions = (preset || instructions).trim();
    setOptimizing(true);
    try {
      await onOptimize(finalInstructions);
    } finally {
      setOptimizing(false);
    }
  };

  const handleExpand = async (preset?: string) => {
    const finalInstructions = (preset || instructions).trim();
    setExpanding(true);
    try {
      await onExpand(finalInstructions);
    } finally {
      setExpanding(false);
    }
  };

  const quickPresets = [
    'Mejora la claridad, reduce repeticiones y fortalece las transiciones entre secciones.',
    'Refuerza la introduccion, conclusion y los H2 para hacer el articulo mas escaneable.',
    'Aumenta naturalmente la presencia de la keyword principal sin sonar artificial.',
  ];
  const expansionPresets = [
    'Hazlo mas largo y profundo con mas contexto y antecedentes.',
    'Agrega subsecciones nuevas con implicaciones, riesgos y escenarios.',
    'Expande la FAQ y fortalece la conclusion con recomendaciones claras.',
  ];

  if (!score || !score.total) {
    return (
      <div className="py-12 text-center border-2 border-dashed border-zinc-300 dark:border-zinc-700 rounded-xl">
        <div className="text-4xl mb-3">&#128202;</div>
        <p className="text-base font-bold text-zinc-500 dark:text-zinc-400">Sin puntuacion SEO</p>
        <p className="text-sm text-zinc-400 dark:text-zinc-500 mt-1">La puntuacion se calcula al completar la generacion</p>
        {escrito.status === 'done' && (
          <button
            onClick={handleRecalc}
            disabled={recalculating}
            className="mt-4 px-5 py-2.5 text-sm font-bold bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 disabled:opacity-50 transition-colors"
          >
            {recalculating ? 'Calculando...' : 'Calcular SEO'}
          </button>
        )}
      </div>
    );
  }

  const total = score.total;
  const color = total >= 80 ? 'text-emerald-500' : total >= 60 ? 'text-amber-500' : 'text-red-500';
  const bgColor = total >= 80 ? 'bg-emerald-500' : total >= 60 ? 'bg-amber-500' : 'bg-red-500';

  const components = [
    { label: 'Densidad Keyword', data: score.keyword_density },
    { label: 'Calidad Titulo', data: score.title_quality },
    { label: 'Meta Description', data: score.meta_description },
    { label: 'Legibilidad', data: score.readability },
    { label: 'Estructura', data: score.structure },
    { label: 'Limpieza IA', data: score.ai_cleanliness },
  ];

  return (
    <div>
      {(escrito.status === 'improving' || optimizing || expanding) && (
        <div className="mb-6 p-4 rounded-xl bg-cyan-50 dark:bg-cyan-950/30 border-2 border-cyan-200 dark:border-cyan-800">
          <div className="flex items-center gap-2.5">
            <div className="animate-spin w-4 h-4 border-2 border-cyan-500 border-t-transparent rounded-full shrink-0" />
            <span className="text-sm font-bold text-cyan-700 dark:text-cyan-300">
              Mejorando redaccion, profundidad y metadata SEO...
            </span>
          </div>
        </div>
      )}

      {/* Overall score */}
      <div className="flex items-center gap-8 mb-8 p-6 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl">
        <div className="text-center">
          <div className={`text-6xl font-black ${color}`}>{total}</div>
          <div className="text-sm font-bold text-zinc-500 mt-1">/100</div>
        </div>
        <div className="flex-1">
          <div className="w-full h-4 bg-zinc-200 dark:bg-zinc-800 rounded-full overflow-hidden">
            <div className={`h-full ${bgColor} rounded-full transition-all duration-500`} style={{ width: `${total}%` }} />
          </div>
          <div className="text-sm font-medium text-zinc-500 dark:text-zinc-400 mt-2">
            {total >= 80 ? 'Excelente — listo para publicar' :
             total >= 60 ? 'Bueno — puede mejorarse' :
             'Necesita trabajo — revisa las recomendaciones'}
          </div>
        </div>
        <button
          onClick={handleRecalc}
          disabled={recalculating}
          className="px-4 py-2 text-sm font-bold text-indigo-600 dark:text-indigo-400 border-2 border-indigo-300 dark:border-indigo-700 rounded-lg hover:bg-indigo-50 dark:hover:bg-indigo-950/20 disabled:opacity-50 transition-colors shrink-0"
        >
          {recalculating ? '...' : 'Recalcular Puntaje'}
        </button>
      </div>

      <div className="mb-8 p-5 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl">
        <div className="flex items-start justify-between gap-4 mb-4">
          <div>
            <h3 className="text-sm font-black text-zinc-800 dark:text-zinc-100 uppercase tracking-wide">Optimizar Escritura SEO</h3>
            <p className="text-sm text-zinc-500 dark:text-zinc-400 mt-1">
              Esto no solo recalcula la nota. Reescribe el articulo para mejorar claridad, estructura, keywording y metadata.
            </p>
          </div>
          <div className="flex gap-2 shrink-0">
            <button
              onClick={() => handleExpand()}
              disabled={expanding || optimizing || escrito.status !== 'done'}
              className="px-4 py-2.5 text-sm font-bold text-cyan-700 dark:text-cyan-300 border-2 border-cyan-300 dark:border-cyan-700 rounded-lg hover:bg-cyan-50 dark:hover:bg-cyan-950/20 disabled:opacity-50 transition-colors"
            >
              {expanding ? 'Expandiendo...' : 'Expandir Articulo'}
            </button>
            <button
              onClick={() => handleOptimize()}
              disabled={optimizing || expanding || escrito.status !== 'done'}
              className="px-4 py-2.5 text-sm font-bold bg-cyan-600 text-white rounded-lg hover:bg-cyan-700 disabled:opacity-50 transition-colors"
            >
              {optimizing ? 'Optimizando...' : 'Optimizar SEO'}
            </button>
          </div>
        </div>

        <div className="flex flex-wrap gap-2 mb-4">
          {quickPresets.map((preset) => (
            <button
              key={preset}
              onClick={() => handleOptimize(preset)}
              disabled={optimizing || expanding || escrito.status !== 'done'}
              className="px-3 py-1.5 text-xs font-bold rounded-lg border border-zinc-300 dark:border-zinc-700 text-zinc-600 dark:text-zinc-300 hover:border-cyan-400 hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors disabled:opacity-50"
            >
              {preset.includes('claridad') ? 'Claridad' : preset.includes('introduccion') ? 'Estructura' : 'Keyword'}
            </button>
          ))}
          {expansionPresets.map((preset) => (
            <button
              key={preset}
              onClick={() => handleExpand(preset)}
              disabled={optimizing || expanding || escrito.status !== 'done'}
              className="px-3 py-1.5 text-xs font-bold rounded-lg border border-zinc-300 dark:border-zinc-700 text-zinc-600 dark:text-zinc-300 hover:border-indigo-400 hover:text-indigo-600 dark:hover:text-indigo-400 transition-colors disabled:opacity-50"
            >
              {preset.includes('largo') ? 'Mas Largo' : preset.includes('subsecciones') ? 'Mas Secciones' : 'FAQ + Cierre'}
            </button>
          ))}
        </div>

        <textarea
          value={instructions}
          onChange={(e) => setInstructions(e.target.value)}
          rows={4}
          placeholder="Ej: mejora la introduccion, sube la densidad de la keyword principal, haz los H2 mas claros, fortalece la conclusion..."
          className="w-full px-4 py-3 text-sm bg-zinc-50 dark:bg-zinc-800 border-2 border-zinc-300 dark:border-zinc-600 rounded-xl text-zinc-900 dark:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-cyan-500 resize-none placeholder:text-zinc-400"
        />
      </div>

      {/* Component breakdown */}
      <div className="grid grid-cols-2 gap-4 mb-8">
        {components.map((c) => (
          <div key={c.label} className="p-4 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-bold text-zinc-700 dark:text-zinc-300">{c.label}</span>
              <span className={`text-sm font-black ${
                c.data.score >= c.data.max * 0.8 ? 'text-emerald-500' :
                c.data.score >= c.data.max * 0.5 ? 'text-amber-500' :
                'text-red-500'
              }`}>
                {c.data.score}/{c.data.max}
              </span>
            </div>
            <div className="w-full h-2 bg-zinc-200 dark:bg-zinc-800 rounded-full overflow-hidden">
              <div
                className={`h-full rounded-full transition-all ${
                  c.data.score >= c.data.max * 0.8 ? 'bg-emerald-500' :
                  c.data.score >= c.data.max * 0.5 ? 'bg-amber-500' :
                  'bg-red-500'
                }`}
                style={{ width: `${(c.data.score / c.data.max) * 100}%` }}
              />
            </div>
            <p className="text-xs text-zinc-500 dark:text-zinc-400 mt-2">{c.data.details}</p>
          </div>
        ))}
      </div>

      {/* Warnings */}
      {score.warnings && score.warnings.length > 0 && (
        <div className="p-4 bg-amber-50 dark:bg-amber-950/20 border-2 border-amber-300 dark:border-amber-800 rounded-xl">
          <h3 className="text-sm font-black text-amber-700 dark:text-amber-300 mb-2 uppercase">Recomendaciones</h3>
          <ul className="space-y-1">
            {score.warnings.map((w, i) => (
              <li key={i} className="text-sm text-amber-600 dark:text-amber-400 flex items-start gap-2">
                <span className="shrink-0 mt-0.5">&#9888;&#65039;</span>
                <span>{w}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

// ── Sources Tab ──────────────────────────────────────────────────

function SourcesTab({ sources }: { sources: EscritoSource[] }) {
  if (sources.length === 0) {
    return (
      <div className="py-12 text-center border-2 border-dashed border-zinc-300 dark:border-zinc-700 rounded-xl">
        <div className="text-4xl mb-3">&#128196;</div>
        <p className="text-base font-bold text-zinc-500 dark:text-zinc-400">Sin fuentes</p>
        <p className="text-sm text-zinc-400 dark:text-zinc-500 mt-1">Las fuentes se agregan durante la generacion</p>
      </div>
    );
  }

  const webCount = sources.filter(s => !s.article_id).length;
  const localCount = sources.length - webCount;

  return (
    <div>
      <div className="flex items-center gap-3 mb-4">
        <p className="text-xs font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-wide">
          {sources.length} fuentes utilizadas
        </p>
        {webCount > 0 && (
          <span className="text-[11px] font-bold px-2 py-0.5 rounded-md bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400">
            {webCount} web
          </span>
        )}
        {localCount > 0 && (
          <span className="text-[11px] font-bold px-2 py-0.5 rounded-md bg-violet-100 dark:bg-violet-900/30 text-violet-600 dark:text-violet-400">
            {localCount} local
          </span>
        )}
      </div>
      <div className="space-y-3">
        {sources.map((s) => {
          const isWeb = !s.article_id;
          const displayTitle = s.article_title || s.web_title || 'Sin titulo';
          const displaySource = s.article_source || s.web_domain || '';
          const displayURL = s.article_url || s.web_url || '';
          return (
            <div key={s.id} className="p-4 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl hover:shadow-md transition-all">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    {isWeb && (
                      <span className="text-[10px] font-black px-1.5 py-0.5 rounded bg-blue-100 dark:bg-blue-900/40 text-blue-600 dark:text-blue-400 uppercase tracking-wider shrink-0">
                        WEB
                      </span>
                    )}
                    <h3 className="text-base font-bold text-zinc-900 dark:text-zinc-100 leading-snug truncate">
                      {displayTitle}
                    </h3>
                  </div>
                  <div className="flex items-center gap-2 mt-1.5">
                    <span className="text-xs font-medium text-zinc-500 dark:text-zinc-400">{displaySource}</span>
                    {s.relevance > 0 && (
                      <span className={`text-[11px] font-bold px-2 py-0.5 rounded-md ${
                        s.relevance >= 0.7 ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-600' :
                        s.relevance >= 0.4 ? 'bg-amber-100 dark:bg-amber-900/30 text-amber-600' :
                        'bg-zinc-100 dark:bg-zinc-800 text-zinc-500'
                      }`}>
                        {Math.round(s.relevance * 100)}% relevancia
                      </span>
                    )}
                    {s.used_in_section && (
                      <span className="text-[11px] text-zinc-400">usado en: {s.used_in_section}</span>
                    )}
                  </div>
                  {displayURL && (
                    <div className="text-[11px] text-zinc-400 dark:text-zinc-500 mt-1 truncate">
                      {displayURL}
                    </div>
                  )}
                </div>
                {displayURL && (
                  <a
                    href={displayURL}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm font-bold text-indigo-600 dark:text-indigo-400 hover:underline shrink-0"
                  >
                    Abrir &#8594;
                  </a>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ── Export Tab ────────────────────────────────────────────────────

function ExportTab({ escrito }: { escrito: Escrito }) {
  const [copied, setCopied] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [previewFormat, setPreviewFormat] = useState<'markdown' | 'html'>('markdown');
  const [preview, setPreview] = useState('');
  const [loadingPreview, setLoadingPreview] = useState(false);

  const handleCopy = async (format: 'markdown' | 'html') => {
    setError(null);
    try {
      const result = await api.exportEscrito(escrito.id, format);
      await navigator.clipboard.writeText(result.content);
      setCopied(format);
      setTimeout(() => setCopied(null), 2500);
    } catch (e) {
      setError(`Error al copiar ${format}: ${e}`);
    }
  };

  const handleDownload = async (format: 'markdown' | 'html') => {
    setError(null);
    try {
      const result = await api.exportEscrito(escrito.id, format);
      const ext = format === 'html' ? 'html' : 'md';
      const mime = format === 'html' ? 'text/html' : 'text/markdown';
      const blob = new Blob([result.content], { type: `${mime};charset=utf-8` });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${escrito.slug || 'escrito'}.${ext}`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      setCopied('download-' + format);
      setTimeout(() => setCopied(null), 2500);
    } catch (e) {
      setError(`Error al descargar: ${e}`);
    }
  };

  const handlePreview = async (format: 'markdown' | 'html') => {
    setPreviewFormat(format);
    setLoadingPreview(true);
    setError(null);
    try {
      const result = await api.exportEscrito(escrito.id, format);
      setPreview(result.content);
    } catch (e) {
      setError(`Error al generar preview: ${e}`);
    } finally {
      setLoadingPreview(false);
    }
  };

  if (escrito.status !== 'done') {
    return (
      <div className="py-12 text-center border-2 border-dashed border-zinc-300 dark:border-zinc-700 rounded-xl">
        <div className="text-4xl mb-3">&#128228;</div>
        <p className="text-base font-bold text-zinc-500 dark:text-zinc-400">Exportar disponible cuando se complete</p>
      </div>
    );
  }

  return (
    <div>
      {error && (
        <div className="mb-4 p-3 bg-red-50 dark:bg-red-950/30 border-2 border-red-300 dark:border-red-800 rounded-xl text-sm font-medium text-red-700 dark:text-red-300">
          {error}
        </div>
      )}

      {/* Copy to clipboard */}
      <h3 className="text-xs font-black text-zinc-500 dark:text-zinc-400 uppercase tracking-widest mb-3">Copiar al portapapeles</h3>
      <div className="flex gap-4 mb-6">
        <button
          onClick={() => handleCopy('markdown')}
          className="flex-1 p-4 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl hover:border-indigo-300 dark:hover:border-indigo-700 hover:shadow-md transition-all text-center"
        >
          <div className="text-2xl mb-2">&#128221;</div>
          <div className="text-sm font-bold text-zinc-900 dark:text-zinc-100">Copiar Markdown</div>
          <div className="text-xs text-zinc-500 mt-1">Para blogs, CMS, GitHub</div>
          {copied === 'markdown' && (
            <div className="text-xs font-bold text-emerald-500 mt-2 animate-fade-in">Copiado!</div>
          )}
        </button>
        <button
          onClick={() => handleCopy('html')}
          className="flex-1 p-4 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl hover:border-indigo-300 dark:hover:border-indigo-700 hover:shadow-md transition-all text-center"
        >
          <div className="text-2xl mb-2">&#127760;</div>
          <div className="text-sm font-bold text-zinc-900 dark:text-zinc-100">Copiar HTML</div>
          <div className="text-xs text-zinc-500 mt-1">Para web, email, newsletters</div>
          {copied === 'html' && (
            <div className="text-xs font-bold text-emerald-500 mt-2 animate-fade-in">Copiado!</div>
          )}
        </button>
      </div>

      {/* Download as file */}
      <h3 className="text-xs font-black text-zinc-500 dark:text-zinc-400 uppercase tracking-widest mb-3">Descargar archivo</h3>
      <div className="flex gap-4 mb-6">
        <button
          onClick={() => handleDownload('markdown')}
          className="flex-1 p-3 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl hover:border-emerald-300 dark:hover:border-emerald-700 hover:shadow-md transition-all text-center"
        >
          <div className="text-sm font-bold text-zinc-900 dark:text-zinc-100">Descargar .md</div>
          {copied === 'download-markdown' && (
            <div className="text-xs font-bold text-emerald-500 mt-1 animate-fade-in">Descargado!</div>
          )}
        </button>
        <button
          onClick={() => handleDownload('html')}
          className="flex-1 p-3 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl hover:border-emerald-300 dark:hover:border-emerald-700 hover:shadow-md transition-all text-center"
        >
          <div className="text-sm font-bold text-zinc-900 dark:text-zinc-100">Descargar .html</div>
          {copied === 'download-html' && (
            <div className="text-xs font-bold text-emerald-500 mt-1 animate-fade-in">Descargado!</div>
          )}
        </button>
      </div>

      {/* Hashtags for social */}
      {escrito.hashtags?.length > 0 && (
        <div className="mb-6 p-4 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl">
          <h3 className="text-sm font-black text-zinc-700 dark:text-zinc-300 mb-3 uppercase">Hashtags para redes sociales</h3>
          <div className="flex flex-wrap gap-2 mb-3">
            {escrito.hashtags.map((h, i) => (
              <span key={i} className="text-sm font-medium px-3 py-1.5 rounded-lg bg-indigo-50 dark:bg-indigo-950/30 text-indigo-600 dark:text-indigo-400 border border-indigo-200 dark:border-indigo-800">
                {h}
              </span>
            ))}
          </div>
          <button
            onClick={async () => {
              await navigator.clipboard.writeText(escrito.hashtags.join(' '));
              setCopied('hashtags');
              setTimeout(() => setCopied(null), 2500);
            }}
            className="text-sm font-bold text-indigo-600 dark:text-indigo-400 hover:underline"
          >
            {copied === 'hashtags' ? 'Copiado!' : 'Copiar hashtags'}
          </button>
        </div>
      )}

      {/* Preview toggle */}
      <h3 className="text-xs font-black text-zinc-500 dark:text-zinc-400 uppercase tracking-widest mb-3">Preview</h3>
      <div className="mb-4 flex gap-2">
        <button
          onClick={() => handlePreview('markdown')}
          className={`px-3 py-1.5 text-xs font-bold rounded-lg border transition-colors ${
            previewFormat === 'markdown' && preview
              ? 'bg-indigo-100 dark:bg-indigo-900/40 text-indigo-700 dark:text-indigo-300 border-indigo-300 dark:border-indigo-700'
              : 'bg-white dark:bg-zinc-900 text-zinc-500 border-zinc-200 dark:border-zinc-700 hover:border-zinc-300'
          }`}
        >
          Markdown
        </button>
        <button
          onClick={() => handlePreview('html')}
          className={`px-3 py-1.5 text-xs font-bold rounded-lg border transition-colors ${
            previewFormat === 'html' && preview
              ? 'bg-indigo-100 dark:bg-indigo-900/40 text-indigo-700 dark:text-indigo-300 border-indigo-300 dark:border-indigo-700'
              : 'bg-white dark:bg-zinc-900 text-zinc-500 border-zinc-200 dark:border-zinc-700 hover:border-zinc-300'
          }`}
        >
          HTML
        </button>
      </div>

      {loadingPreview && (
        <div className="flex justify-center py-8">
          <div className="animate-spin w-6 h-6 border-2 border-indigo-500 border-t-transparent rounded-full" />
        </div>
      )}

      {preview && !loadingPreview && (
        <div className="p-4 bg-zinc-50 dark:bg-zinc-800 border-2 border-zinc-200 dark:border-zinc-700 rounded-xl overflow-auto max-h-[500px]">
          <pre className="text-sm text-zinc-700 dark:text-zinc-300 font-mono whitespace-pre-wrap break-words">
            {preview}
          </pre>
        </div>
      )}
    </div>
  );
}

// ── Markdown Renderer ────────────────────────────────────────────

function renderMarkdown(md: string): string {
  return md
    .replace(/^### (.+)$/gm, '<h3 class="text-lg font-black mt-6 mb-2 uppercase tracking-tight">$1</h3>')
    .replace(/^## (.+)$/gm, '<h2 class="text-xl font-black mt-8 mb-3 uppercase tracking-tight">$1</h2>')
    .replace(/^# (.+)$/gm, '<h1 class="text-2xl font-black mt-8 mb-4 uppercase tracking-tight">$1</h1>')
    .replace(/\*\*(.+?)\*\*/g, '<strong class="font-bold">$1</strong>')
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    .replace(/^- (.+)$/gm, '<li class="ml-4 mb-1">$1</li>')
    .replace(/\[(.+?)\]\((.+?)\)/g, '<a href="$2" target="_blank" class="text-indigo-600 dark:text-indigo-400 font-bold hover:underline">$1</a>')
    .replace(/\n\n/g, '<br/><br/>');
}
