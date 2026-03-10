import { useState, useEffect, useCallback } from 'react';
import { api, type ResearchProject, type ResearchFinding } from '../lib/api';

const SOURCE_TYPES = [
  { key: 'all', label: 'Todos' },
  { key: 'google_news', label: 'Google News' },
  { key: 'bing_news', label: 'Bing News' },
  { key: 'web', label: 'Web' },
  { key: 'local', label: 'Local' },
  { key: 'youtube', label: 'YouTube' },
  { key: 'reddit', label: 'Reddit' },
  { key: 'deep_crawl', label: 'Deep Crawl' },
] as const;

const STATUS_STYLES: Record<string, { bg: string; text: string; label: string; emoji: string; border: string }> = {
  queued:       { bg: 'bg-zinc-100 dark:bg-zinc-800',       text: 'text-zinc-600 dark:text-zinc-300',     label: 'En cola',       emoji: '\u23f3', border: 'border-zinc-300 dark:border-zinc-600' },
  searching:    { bg: 'bg-blue-100 dark:bg-blue-900/40',    text: 'text-blue-700 dark:text-blue-300',     label: 'Buscando',      emoji: '\ud83d\udd0d', border: 'border-blue-300 dark:border-blue-700' },
  scraping:     { bg: 'bg-amber-100 dark:bg-amber-900/40',  text: 'text-amber-700 dark:text-amber-300',   label: 'Recopilando',   emoji: '\ud83d\udcc4', border: 'border-amber-300 dark:border-amber-700' },
  synthesizing: { bg: 'bg-purple-100 dark:bg-purple-900/40',text: 'text-purple-700 dark:text-purple-300', label: 'Sintetizando',  emoji: '\ud83e\udde0', border: 'border-purple-300 dark:border-purple-700' },
  done:         { bg: 'bg-emerald-100 dark:bg-emerald-900/40',text: 'text-emerald-700 dark:text-emerald-300',label: 'Completado', emoji: '\u2705', border: 'border-emerald-300 dark:border-emerald-700' },
  failed:       { bg: 'bg-red-100 dark:bg-red-900/40',      text: 'text-red-700 dark:text-red-300',       label: 'Error',         emoji: '\u274c', border: 'border-red-300 dark:border-red-700' },
  cancelled:    { bg: 'bg-zinc-100 dark:bg-zinc-800',       text: 'text-zinc-500 dark:text-zinc-400',     label: 'Cancelado',     emoji: '\ud83d\udeab', border: 'border-zinc-300 dark:border-zinc-600' },
};

const SENTIMENT_COLORS: Record<string, string> = {
  positive: 'text-emerald-600 dark:text-emerald-400',
  neutral:  'text-zinc-500 dark:text-zinc-400',
  negative: 'text-red-600 dark:text-red-400',
  unknown:  'text-amber-500 dark:text-amber-400',
};

const PHASE_MESSAGES: Record<string, string> = {
  queued:       'Preparando investigacion... expandiendo palabras clave con IA',
  searching:    'Fase 1 de 3 — Buscando en Google News, Bing, DuckDuckGo, YouTube, Reddit y base local...',
  scraping:     'Fase 2 de 3 — Descargando y analizando paginas web encontradas...',
  synthesizing: 'Fase 3 de 3 — Sintetizando hallazgos y generando dossier con IA...',
};

function timeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diff = now - then;
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'ahora';
  if (mins < 60) return `${mins}m`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h`;
  const days = Math.floor(hrs / 24);
  return `${days}d`;
}

type Tab = 'hallazgos' | 'dossier' | 'entidades' | 'cronologia';

export default function ResearchDesk() {
  const [projects, setProjects] = useState<ResearchProject[]>([]);
  const [selectedProject, setSelectedProject] = useState<ResearchProject | null>(null);
  const [findings, setFindings] = useState<ResearchFinding[]>([]);
  const [loading, setLoading] = useState(true);
  const [findingsLoading, setFindingsLoading] = useState(false);
  const [sourceFilter, setSourceFilter] = useState('all');
  const [activeTab, setActiveTab] = useState<Tab>('hallazgos');
  const [showCreate, setShowCreate] = useState(false);
  const [newTopic, setNewTopic] = useState('');
  const [creating, setCreating] = useState(false);

  const loadProjects = useCallback(async () => {
    try {
      const data = await api.getResearchProjects();
      setProjects(data.projects || []);
    } catch (e) {
      console.error('load projects:', e);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadFindings = useCallback(async (projectId: string) => {
    setFindingsLoading(true);
    try {
      const params: { source_type?: string; limit?: number } = { limit: 100 };
      if (sourceFilter !== 'all') params.source_type = sourceFilter;
      const data = await api.getResearchFindings(projectId, params);
      setFindings(data.findings || []);
    } catch (e) {
      console.error('load findings:', e);
    } finally {
      setFindingsLoading(false);
    }
  }, [sourceFilter]);

  const selectProject = useCallback(async (p: ResearchProject) => {
    setSelectedProject(p);
    setActiveTab('hallazgos');
    setSourceFilter('all');
    loadFindings(p.id);
    try {
      const data = await api.getResearchProject(p.id);
      setSelectedProject(data.project);
    } catch (e) { /* ignore */ }
  }, [loadFindings]);

  useEffect(() => { loadProjects(); }, [loadProjects]);

  // Auto-poll active projects every 5s
  useEffect(() => {
    const hasActive = projects.some(p =>
      ['queued', 'searching', 'scraping', 'synthesizing'].includes(p.status)
    );
    if (!hasActive) return;

    const interval = setInterval(async () => {
      await loadProjects();
      if (selectedProject && ['queued', 'searching', 'scraping', 'synthesizing'].includes(selectedProject.status)) {
        try {
          const data = await api.getResearchProject(selectedProject.id);
          setSelectedProject(data.project);
          loadFindings(selectedProject.id);
        } catch (e) { /* ignore */ }
      }
    }, 5000);

    return () => clearInterval(interval);
  }, [projects, selectedProject, loadProjects, loadFindings]);

  useEffect(() => {
    if (selectedProject) loadFindings(selectedProject.id);
  }, [sourceFilter, selectedProject, loadFindings]);

  const handleCreate = async () => {
    if (!newTopic.trim()) return;
    setCreating(true);
    try {
      const project = await api.createResearchProject(newTopic.trim());
      setNewTopic('');
      setShowCreate(false);
      await loadProjects();
      selectProject(project);
    } catch (e) {
      console.error('create project:', e);
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Eliminar esta investigacion y todos sus hallazgos?')) return;
    try {
      await api.deleteResearchProject(id);
      if (selectedProject?.id === id) {
        setSelectedProject(null);
        setFindings([]);
      }
      loadProjects();
    } catch (e) {
      console.error('delete project:', e);
    }
  };

  const handleStop = async (id: string) => {
    try {
      await api.stopResearchProject(id);
      loadProjects();
      if (selectedProject?.id === id) {
        const data = await api.getResearchProject(id);
        setSelectedProject(data.project);
      }
    } catch (e) {
      console.error('stop project:', e);
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
            Investigacion Profunda
          </h1>
          <p className="text-sm text-zinc-500 dark:text-zinc-400 mt-1">
            Investigaciones automatizadas con busqueda en 6 motores y analisis con IA
          </p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="px-5 py-2.5 text-sm font-bold bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition-colors shadow-lg shadow-indigo-500/20 uppercase tracking-wide"
        >
          + Nueva Investigacion
        </button>
      </div>

      {/* Create modal */}
      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-white dark:bg-zinc-900 rounded-xl p-8 w-full max-w-lg border border-zinc-200 dark:border-zinc-700 shadow-2xl animate-fade-in">
            <h2 className="text-xl font-black text-zinc-900 dark:text-zinc-50 mb-1 uppercase">Nueva Investigacion</h2>
            <p className="text-sm text-zinc-500 dark:text-zinc-400 mb-6">
              Ingresa un tema y el sistema buscara en Google News, Bing, DuckDuckGo, YouTube, Reddit y la base de datos local. El proceso toma entre 15-45 minutos.
            </p>
            <label className="block text-sm font-bold text-zinc-700 dark:text-zinc-300 mb-2 uppercase tracking-wide">Tema de investigacion</label>
            <input
              type="text"
              value={newTopic}
              onChange={(e) => setNewTopic(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
              placeholder="ej: reciclaje, energia renovable, corrupcion AAA..."
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
                {creating ? 'Iniciando...' : 'Investigar'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Two-panel layout */}
      <div className="flex gap-8 min-h-[650px]">
        {/* Left panel: project list */}
        <div className="w-72 shrink-0">
          <h3 className="text-xs font-bold text-zinc-500 dark:text-zinc-400 uppercase tracking-widest mb-3">Proyectos</h3>
          <div className="space-y-3">
            {projects.length === 0 && (
              <div className="text-center py-8 border-2 border-dashed border-zinc-300 dark:border-zinc-700 rounded-xl">
                <div className="text-3xl mb-2">🔬</div>
                <p className="text-sm text-zinc-500 dark:text-zinc-400 font-medium">
                  Sin investigaciones
                </p>
                <p className="text-xs text-zinc-400 dark:text-zinc-500 mt-1">
                  Crea una para comenzar
                </p>
              </div>
            )}
            {projects.map((p) => {
              const st = STATUS_STYLES[p.status] || STATUS_STYLES.queued;
              const isActive = selectedProject?.id === p.id;
              const isRunning = ['queued', 'searching', 'scraping', 'synthesizing'].includes(p.status);
              return (
                <button
                  key={p.id}
                  onClick={() => selectProject(p)}
                  className={`w-full text-left px-4 py-3.5 rounded-xl border-2 transition-all ${
                    isActive
                      ? 'bg-indigo-50 dark:bg-indigo-950/40 border-indigo-400 dark:border-indigo-600 shadow-md shadow-indigo-500/10'
                      : 'bg-white dark:bg-zinc-900 border-zinc-200 dark:border-zinc-800 hover:border-zinc-300 dark:hover:border-zinc-700 hover:shadow-sm'
                  }`}
                >
                  <div className="font-bold text-base text-zinc-900 dark:text-zinc-100 truncate capitalize">
                    {p.topic}
                  </div>
                  <div className="flex items-center gap-2 mt-1.5">
                    <span className={`inline-flex items-center gap-1 px-2 py-0.5 text-xs font-bold rounded-md ${st.bg} ${st.text} border ${st.border}`}>
                      {st.emoji} {st.label}
                    </span>
                    <span className="text-xs text-zinc-400 font-medium">{timeAgo(p.created_at)}</span>
                  </div>
                  {isRunning && (
                    <div className="flex items-center gap-1.5 mt-2">
                      <div className="animate-spin w-3 h-3 border-[1.5px] border-indigo-500 border-t-transparent rounded-full" />
                      <span className="text-[11px] text-indigo-500 dark:text-indigo-400 font-medium">En progreso...</span>
                    </div>
                  )}
                  {p.findings_count !== undefined && p.findings_count > 0 && (
                    <div className="text-xs text-zinc-500 dark:text-zinc-400 mt-1 font-medium">
                      {p.findings_count} hallazgos encontrados
                    </div>
                  )}
                </button>
              );
            })}
          </div>
        </div>

        {/* Right panel: project detail */}
        <div className="flex-1 min-w-0">
          {!selectedProject ? (
            <div className="flex flex-col items-center justify-center h-full text-center">
              <div className="text-5xl mb-4">🔍</div>
              <p className="text-lg font-bold text-zinc-600 dark:text-zinc-400">
                Selecciona una investigacion
              </p>
              <p className="text-sm text-zinc-400 dark:text-zinc-500 mt-1">
                o crea una nueva para comenzar
              </p>
            </div>
          ) : (
            <div>
              {/* Project header */}
              <div className="flex items-start justify-between mb-6">
                <div className="flex-1 min-w-0">
                  <h2 className="text-2xl font-black text-zinc-900 dark:text-zinc-50 uppercase tracking-tight">
                    {selectedProject.topic}
                  </h2>
                  <div className="flex items-center gap-3 mt-2 flex-wrap">
                    <StatusBadge status={selectedProject.status} />
                    <span className="text-sm font-bold text-zinc-500 dark:text-zinc-400">
                      Fase {selectedProject.phase}/3
                    </span>
                    {selectedProject.keywords.length > 0 && (
                      <div className="flex gap-1 flex-wrap">
                        {selectedProject.keywords.slice(0, 5).map((kw, i) => (
                          <span key={i} className="text-[11px] font-medium px-2 py-0.5 rounded-md bg-zinc-100 dark:bg-zinc-800 text-zinc-600 dark:text-zinc-400 border border-zinc-200 dark:border-zinc-700">
                            {kw}
                          </span>
                        ))}
                        {selectedProject.keywords.length > 5 && (
                          <span className="text-[11px] text-zinc-400">+{selectedProject.keywords.length - 5} mas</span>
                        )}
                      </div>
                    )}
                  </div>

                  {/* Live progress */}
                  {['queued', 'searching', 'scraping', 'synthesizing'].includes(selectedProject.status) && (
                    <div className="mt-4 p-4 rounded-xl bg-indigo-50 dark:bg-indigo-950/30 border-2 border-indigo-200 dark:border-indigo-800">
                      <div className="w-full h-2 bg-indigo-200 dark:bg-indigo-900 rounded-full overflow-hidden mb-3">
                        <div
                          className="h-full bg-indigo-500 rounded-full transition-all duration-1000"
                          style={{
                            width: selectedProject.status === 'queued' ? '8%' : `${Math.max((selectedProject.phase / 3) * 100, 10)}%`,
                            animation: 'pulse 2s ease-in-out infinite',
                          }}
                        />
                      </div>
                      <div className="flex items-center gap-2.5">
                        <div className="animate-spin w-4 h-4 border-2 border-indigo-500 border-t-transparent rounded-full shrink-0" />
                        <span className="text-sm font-bold text-indigo-700 dark:text-indigo-300">
                          {PHASE_MESSAGES[selectedProject.status] || 'Procesando...'}
                        </span>
                      </div>
                      {selectedProject.progress && (
                        <div className="flex gap-4 mt-2 text-xs text-indigo-600 dark:text-indigo-400 font-medium">
                          {selectedProject.progress.phase1_hits !== undefined && (
                            <span>{selectedProject.progress.phase1_hits} resultados encontrados</span>
                          )}
                          {selectedProject.progress.phase2_scraped !== undefined && (
                            <span>{selectedProject.progress.phase2_scraped} paginas analizadas</span>
                          )}
                        </div>
                      )}
                    </div>
                  )}
                </div>
                <div className="flex gap-2 shrink-0 ml-4">
                  {['queued', 'searching', 'scraping', 'synthesizing'].includes(selectedProject.status) && (
                    <button
                      onClick={() => handleStop(selectedProject.id)}
                      className="px-4 py-2 text-sm font-bold text-red-600 dark:text-red-400 border-2 border-red-300 dark:border-red-700 rounded-lg hover:bg-red-50 dark:hover:bg-red-950/20 transition-colors"
                    >
                      Cancelar
                    </button>
                  )}
                  <button
                    onClick={() => handleDelete(selectedProject.id)}
                    className="px-4 py-2 text-sm font-bold text-zinc-500 dark:text-zinc-400 border-2 border-zinc-300 dark:border-zinc-700 rounded-lg hover:bg-zinc-50 dark:hover:bg-zinc-800/50 transition-colors"
                  >
                    Eliminar
                  </button>
                </div>
              </div>

              {/* Error message */}
              {selectedProject.error_msg && (
                <div className="mb-6 p-4 bg-red-50 dark:bg-red-950/30 border-2 border-red-300 dark:border-red-800 rounded-xl text-sm font-medium text-red-700 dark:text-red-300">
                  <span className="font-bold">Error:</span> {selectedProject.error_msg}
                </div>
              )}

              {/* Tabs */}
              <div className="flex gap-0 mb-6 border-b-2 border-zinc-200 dark:border-zinc-800">
                {(['hallazgos', 'dossier', 'entidades', 'cronologia'] as Tab[]).map((tab) => (
                  <button
                    key={tab}
                    onClick={() => setActiveTab(tab)}
                    className={`px-5 py-3 text-sm font-bold uppercase tracking-wide border-b-3 transition-colors ${
                      activeTab === tab
                        ? 'border-indigo-500 text-indigo-600 dark:text-indigo-400 bg-indigo-50/50 dark:bg-indigo-950/20'
                        : 'border-transparent text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-100 hover:bg-zinc-50 dark:hover:bg-zinc-800/30'
                    }`}
                  >
                    {tab === 'hallazgos' && '\ud83d\udcc4 '}
                    {tab === 'dossier' && '\ud83d\udcdd '}
                    {tab === 'entidades' && '\ud83d\udc65 '}
                    {tab === 'cronologia' && '\ud83d\udcc5 '}
                    {tab}
                  </button>
                ))}
              </div>

              {/* Tab content */}
              {activeTab === 'hallazgos' && (
                <FindingsTab
                  findings={findings}
                  loading={findingsLoading}
                  sourceFilter={sourceFilter}
                  onFilterChange={setSourceFilter}
                />
              )}
              {activeTab === 'dossier' && (
                <DossierTab dossier={selectedProject.dossier} status={selectedProject.status} />
              )}
              {activeTab === 'entidades' && (
                <EntitiesTab entities={selectedProject.entities} />
              )}
              {activeTab === 'cronologia' && (
                <TimelineTab timeline={selectedProject.timeline || []} />
              )}
            </div>
          )}
        </div>
      </div>
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

function FindingsTab({
  findings,
  loading,
  sourceFilter,
  onFilterChange,
}: {
  findings: ResearchFinding[];
  loading: boolean;
  sourceFilter: string;
  onFilterChange: (f: string) => void;
}) {
  const [expanded, setExpanded] = useState<string | null>(null);

  return (
    <div>
      {/* Source type filter */}
      <div className="flex gap-2 mb-5 flex-wrap">
        {SOURCE_TYPES.map((st) => (
          <button
            key={st.key}
            onClick={() => onFilterChange(st.key)}
            className={`px-3 py-1.5 text-xs font-bold rounded-lg transition-colors border ${
              sourceFilter === st.key
                ? 'bg-indigo-100 dark:bg-indigo-900/40 text-indigo-700 dark:text-indigo-300 border-indigo-300 dark:border-indigo-700'
                : 'bg-white dark:bg-zinc-900 text-zinc-600 dark:text-zinc-400 border-zinc-200 dark:border-zinc-700 hover:border-zinc-300 dark:hover:border-zinc-600'
            }`}
          >
            {st.label}
          </button>
        ))}
      </div>

      {loading ? (
        <div className="flex flex-col items-center justify-center py-12">
          <div className="animate-spin w-8 h-8 border-3 border-indigo-500 border-t-transparent rounded-full" />
          <p className="text-sm text-zinc-500 mt-3 font-medium">Cargando hallazgos...</p>
        </div>
      ) : findings.length === 0 ? (
        <div className="text-center py-12 border-2 border-dashed border-zinc-300 dark:border-zinc-700 rounded-xl">
          <div className="text-4xl mb-3">📋</div>
          <p className="text-base font-bold text-zinc-500 dark:text-zinc-400">No hay hallazgos todavia</p>
          <p className="text-sm text-zinc-400 dark:text-zinc-500 mt-1">Los resultados apareceran mientras la investigacion avanza</p>
        </div>
      ) : (
        <div className="space-y-3">
          <p className="text-xs font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-wide">
            {findings.length} hallazgos
          </p>
          {findings.map((f) => (
            <div
              key={f.id}
              className="p-4 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl cursor-pointer hover:border-zinc-300 dark:hover:border-zinc-600 hover:shadow-md transition-all"
              onClick={() => setExpanded(expanded === f.id ? null : f.id)}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 mb-2">
                    <span className="text-[11px] font-bold uppercase px-2 py-0.5 rounded-md bg-zinc-100 dark:bg-zinc-800 text-zinc-600 dark:text-zinc-400 border border-zinc-200 dark:border-zinc-700">
                      {f.source_type.replace('_', ' ')}
                    </span>
                    {f.relevance > 0 && (
                      <span className={`text-[11px] font-bold px-2 py-0.5 rounded-md ${
                        f.relevance >= 0.7 ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-600 dark:text-emerald-400' :
                        f.relevance >= 0.4 ? 'bg-amber-100 dark:bg-amber-900/30 text-amber-600 dark:text-amber-400' :
                        'bg-zinc-100 dark:bg-zinc-800 text-zinc-500'
                      }`}>
                        {Math.round(f.relevance * 100)}%
                      </span>
                    )}
                    <span className={`text-[11px] font-bold capitalize ${SENTIMENT_COLORS[f.sentiment] || ''}`}>
                      {f.sentiment !== 'unknown' ? f.sentiment : ''}
                    </span>
                  </div>
                  <h3 className="text-base font-bold text-zinc-900 dark:text-zinc-100 leading-snug">
                    {f.title || f.url}
                  </h3>
                  {f.snippet && (
                    <p className="text-sm text-zinc-500 dark:text-zinc-400 mt-1.5 line-clamp-2 leading-relaxed">{f.snippet}</p>
                  )}
                </div>
                <div className="text-zinc-400 shrink-0 mt-1">
                  <svg className={`w-5 h-5 transition-transform ${expanded === f.id ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
                  </svg>
                </div>
              </div>

              {/* Expanded content */}
              {expanded === f.id && (
                <div className="mt-4 pt-4 border-t-2 border-zinc-200 dark:border-zinc-800">
                  {f.clean_text && (
                    <p className="text-sm text-zinc-600 dark:text-zinc-400 mb-3 whitespace-pre-wrap leading-relaxed">
                      {f.clean_text.substring(0, 1500)}
                      {f.clean_text.length > 1500 && '...'}
                    </p>
                  )}
                  {f.tags && f.tags.length > 0 && (
                    <div className="flex gap-1 flex-wrap mb-3">
                      {f.tags.map((tag, i) => (
                        <span key={i} className="text-[11px] font-medium px-2 py-0.5 rounded-md bg-indigo-50 dark:bg-indigo-950/30 text-indigo-600 dark:text-indigo-400">
                          {tag}
                        </span>
                      ))}
                    </div>
                  )}
                  <a
                    href={f.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-sm font-bold text-indigo-600 dark:text-indigo-400 hover:underline"
                    onClick={(e) => e.stopPropagation()}
                  >
                    Abrir fuente →
                  </a>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function DossierTab({ dossier, status }: { dossier: string; status: string }) {
  if (status !== 'done' || !dossier) {
    return (
      <div className="py-12 text-center border-2 border-dashed border-zinc-300 dark:border-zinc-700 rounded-xl">
        {['searching', 'scraping', 'synthesizing', 'queued'].includes(status) ? (
          <>
            <div className="flex justify-center mb-4">
              <div className="animate-spin w-8 h-8 border-3 border-indigo-500 border-t-transparent rounded-full" />
            </div>
            <p className="text-base font-bold text-zinc-600 dark:text-zinc-400">
              Generando dossier...
            </p>
            <p className="text-sm text-zinc-400 dark:text-zinc-500 mt-1">
              El dossier estara disponible cuando la Fase 3 se complete
            </p>
          </>
        ) : (
          <>
            <div className="text-4xl mb-3">📝</div>
            <p className="text-base font-bold text-zinc-500 dark:text-zinc-400">
              No se genero un dossier para esta investigacion
            </p>
          </>
        )}
      </div>
    );
  }

  return (
    <div className="p-6 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl">
      <div
        className="text-base text-zinc-700 dark:text-zinc-300 leading-relaxed whitespace-pre-wrap"
        dangerouslySetInnerHTML={{ __html: renderMarkdown(dossier) }}
      />
    </div>
  );
}

function EntitiesTab({ entities }: { entities: { people?: string[]; organizations?: string[]; places?: string[] } }) {
  const people = entities?.people || [];
  const orgs = entities?.organizations || [];
  const places = entities?.places || [];

  if (people.length === 0 && orgs.length === 0 && places.length === 0) {
    return (
      <div className="py-12 text-center border-2 border-dashed border-zinc-300 dark:border-zinc-700 rounded-xl">
        <div className="text-4xl mb-3">👥</div>
        <p className="text-base font-bold text-zinc-500 dark:text-zinc-400">No se encontraron entidades</p>
        <p className="text-sm text-zinc-400 dark:text-zinc-500 mt-1">Las entidades se extraen durante la Fase 3</p>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-3 gap-8">
      <EntityColumn title="Personas" items={people} color="blue" emoji="👤" />
      <EntityColumn title="Organizaciones" items={orgs} color="purple" emoji="🏛️" />
      <EntityColumn title="Lugares" items={places} color="emerald" emoji="📍" />
    </div>
  );
}

function EntityColumn({ title, items, emoji, color }: { title: string; items: string[]; emoji: string; color: string }) {
  const colorClasses: Record<string, string> = {
    blue: 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 border-blue-200 dark:border-blue-800',
    purple: 'bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300 border-purple-200 dark:border-purple-800',
    emerald: 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-300 border-emerald-200 dark:border-emerald-800',
  };

  return (
    <div className="p-4 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl">
      <h3 className="text-sm font-black text-zinc-900 dark:text-zinc-100 mb-4 uppercase tracking-wide">
        {emoji} {title} ({items.length})
      </h3>
      <div className="flex flex-wrap gap-2">
        {items.length === 0 && <span className="text-sm text-zinc-500 font-medium">Ninguno</span>}
        {items.map((item, i) => (
          <span
            key={i}
            className={`inline-block px-3 py-1.5 text-sm font-medium rounded-lg border ${colorClasses[color] || colorClasses.blue}`}
          >
            {item}
          </span>
        ))}
      </div>
    </div>
  );
}

function TimelineTab({ timeline }: { timeline: { date: string; event: string; source_url: string }[] }) {
  if (timeline.length === 0) {
    return (
      <div className="py-12 text-center border-2 border-dashed border-zinc-300 dark:border-zinc-700 rounded-xl">
        <div className="text-4xl mb-3">📅</div>
        <p className="text-base font-bold text-zinc-500 dark:text-zinc-400">No hay eventos en la cronologia</p>
        <p className="text-sm text-zinc-400 dark:text-zinc-500 mt-1">La cronologia se construye durante la Fase 3</p>
      </div>
    );
  }

  return (
    <div className="relative pl-8 space-y-6">
      {/* Vertical line */}
      <div className="absolute left-3 top-2 bottom-2 w-0.5 bg-indigo-200 dark:bg-indigo-800" />

      {timeline.map((event, i) => (
        <div key={i} className="relative">
          {/* Dot */}
          <div className="absolute -left-[21px] top-1 w-3.5 h-3.5 rounded-full bg-indigo-500 border-3 border-white dark:border-zinc-900 shadow-md" />

          <div className="p-4 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl hover:shadow-md transition-all">
            <div className="text-sm font-black text-indigo-600 dark:text-indigo-400 mb-1 uppercase">
              {event.date}
            </div>
            <div className="text-base font-medium text-zinc-900 dark:text-zinc-100 leading-relaxed">
              {event.event}
            </div>
            {event.source_url && (
              <a
                href={event.source_url}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 text-sm font-bold text-indigo-600 dark:text-indigo-400 hover:underline mt-2"
              >
                Ver fuente →
              </a>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

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
