import { useState, useEffect, useCallback } from 'react';
import { api } from '../lib/api';

interface CrawlDomain {
  id: string;
  domain: string;
  label: string;
  category: string;
  max_depth: number;
  recrawl_hours: number;
  priority: number;
  active: boolean;
  page_count: number;
  last_crawled_at: string | null;
  created_at: string;
}

interface CrawledPage {
  id: string;
  url: string;
  url_hash: string;
  domain_id: string;
  title: string;
  clean_text: string;
  content_hash: string;
  summary: string;
  tags: string[];
  entities: Record<string, string[]>;
  sentiment: string;
  links_out: number;
  links_in: number;
  depth: number;
  status_code: number;
  content_type: string;
  content_length: number;
  changed: boolean;
  change_summary: string;
  enriched: boolean;
  crawl_count: number;
  first_seen_at: string;
  last_crawled_at: string;
  next_crawl_at: string | null;
  created_at: string;
}

interface CrawlRun {
  id: string;
  status: string;
  pages_crawled: number;
  pages_new: number;
  pages_changed: number;
  pages_failed: number;
  started_at: string;
  finished_at: string | null;
  error_message: string;
}

interface GraphNode {
  id: string;
  name: string;
  type: string;
}

interface GraphEdge {
  source: string;
  target: string;
  relation_type: string;
  strength: number;
}

interface CrawlLink {
  id: string;
  source_page_id: string;
  target_url: string;
  anchor_text: string;
}

const SENTIMENT_COLORS: Record<string, string> = {
  positive: 'text-emerald-600 dark:text-emerald-400',
  neutral: 'text-zinc-500 dark:text-zinc-400',
  negative: 'text-red-600 dark:text-red-400',
  unknown: 'text-amber-500 dark:text-amber-400',
};

const CATEGORY_OPTIONS = [
  { value: 'gobierno', label: 'Gobierno' },
  { value: 'legislativo', label: 'Legislativo' },
  { value: 'judicial', label: 'Judicial' },
  { value: 'media', label: 'Media' },
  { value: 'other', label: 'Otro' },
];

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

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

type Tab = 'dashboard' | 'dominios' | 'paginas' | 'cambios' | 'grafo';

export default function CrawlerDesk() {
  const [activeTab, setActiveTab] = useState<Tab>('dashboard');
  const [loading, setLoading] = useState(true);

  const [stats, setStats] = useState<any>(null);
  const [runs, setRuns] = useState<CrawlRun[]>([]);
  const [queue, setQueue] = useState<Record<string, number>>({});
  const [domains, setDomains] = useState<CrawlDomain[]>([]);
  const [pages, setPages] = useState<CrawledPage[]>([]);
  const [changedPages, setChangedPages] = useState<CrawledPage[]>([]);
  const [selectedPage, setSelectedPage] = useState<CrawledPage | null>(null);
  const [pageDetail, setPageDetail] = useState<any>(null);
  const [graphData, setGraphData] = useState<{ nodes: GraphNode[]; edges: GraphEdge[] } | null>(null);
  const [selectedEntity, setSelectedEntity] = useState<string | null>(null);
  const [entityRelations, setEntityRelations] = useState<any>(null);

  const [showAddDomain, setShowAddDomain] = useState(false);
  const [editingDomain, setEditingDomain] = useState<CrawlDomain | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [domainFilter, setDomainFilter] = useState('all');
  const [triggering, setTriggering] = useState(false);

  const loadDashboard = useCallback(async () => {
    try {
      const [statsData, runsData, queueData] = await Promise.all([
        api.getCrawlerStats(),
        api.getCrawlerRuns(5),
        api.getCrawlerQueue(),
      ]);
      setStats(statsData);
      setRuns(runsData.runs || []);
      setQueue(queueData.queue || {});
    } catch (e) {
      console.error('load dashboard:', e);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadDomains = useCallback(async () => {
    try {
      const data = await api.getCrawlerDomains();
      setDomains(data.domains || []);
    } catch (e) {
      console.error('load domains:', e);
    }
  }, []);

  const loadPages = useCallback(async () => {
    try {
      const params: any = { limit: 50 };
      if (domainFilter !== 'all') params.domain_id = domainFilter;
      const data = await api.getCrawlerPages(params);
      setPages(data.pages || []);
    } catch (e) {
      console.error('load pages:', e);
    }
  }, [domainFilter]);

  const loadChangedPages = useCallback(async () => {
    try {
      const data = await api.getCrawlerChangedPages(50);
      setChangedPages(data.pages || []);
    } catch (e) {
      console.error('load changed pages:', e);
    }
  }, []);

  const loadGraphData = useCallback(async () => {
    try {
      const data = await api.getCrawlerGraph(50);
      setGraphData(data);
    } catch (e) {
      console.error('load graph:', e);
    }
  }, []);

  useEffect(() => {
    if (activeTab === 'dashboard') {
      loadDashboard();
    } else if (activeTab === 'dominios') {
      loadDomains();
    } else if (activeTab === 'paginas') {
      loadPages();
    } else if (activeTab === 'cambios') {
      loadChangedPages();
    } else if (activeTab === 'grafo') {
      loadGraphData();
    }
  }, [activeTab, loadDashboard, loadDomains, loadPages, loadChangedPages, loadGraphData]);

  useEffect(() => {
    if (activeTab === 'paginas') {
      loadPages();
    }
  }, [domainFilter, activeTab, loadPages]);

  const handleTriggerCrawl = async () => {
    setTriggering(true);
    try {
      const result = await api.triggerCrawl();
      alert(result.message || 'Crawl iniciado');
      loadDashboard();
    } catch (e) {
      console.error('trigger crawl:', e);
      alert('Error al iniciar crawl');
    } finally {
      setTriggering(false);
    }
  };

  const handleSearchPages = async () => {
    if (!searchQuery.trim()) {
      loadPages();
      return;
    }
    try {
      const data = await api.searchCrawlerPages(searchQuery, 50);
      setPages(data.pages || []);
    } catch (e) {
      console.error('search pages:', e);
    }
  };

  const handleSelectPage = async (page: CrawledPage) => {
    setSelectedPage(page);
    try {
      const data = await api.getCrawlerPage(page.id);
      setPageDetail(data);
    } catch (e) {
      console.error('load page detail:', e);
    }
  };

  const handleAddDomain = async (formData: Partial<CrawlDomain>) => {
    try {
      await api.createCrawlerDomain(formData);
      setShowAddDomain(false);
      loadDomains();
    } catch (e) {
      console.error('add domain:', e);
      alert('Error al agregar dominio');
    }
  };

  const handleUpdateDomain = async (id: string, formData: Partial<CrawlDomain>) => {
    try {
      await api.updateCrawlerDomain(id, formData);
      setEditingDomain(null);
      loadDomains();
    } catch (e) {
      console.error('update domain:', e);
      alert('Error al actualizar dominio');
    }
  };

  const handleToggleDomain = async (id: string, active: boolean) => {
    try {
      await api.toggleCrawlerDomain(id, active);
      loadDomains();
    } catch (e) {
      console.error('toggle domain:', e);
    }
  };

  const handleDeleteDomain = async (id: string) => {
    if (!confirm('Eliminar este dominio? Se perderan todas las paginas crawleadas.')) return;
    try {
      await api.deleteCrawlerDomain(id);
      loadDomains();
    } catch (e) {
      console.error('delete domain:', e);
      alert('Error al eliminar dominio');
    }
  };

  const handleSelectEntity = async (entityId: string) => {
    setSelectedEntity(entityId);
    try {
      const data = await api.getCrawlerEntityRelations(entityId);
      setEntityRelations(data);
    } catch (e) {
      console.error('load entity relations:', e);
    }
  };

  if (loading && activeTab === 'dashboard') {
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
            Web Crawler
          </h1>
          <p className="text-sm text-zinc-500 dark:text-zinc-400 mt-1">
            Sistema automatizado de rastreo web y extraccion de entidades
          </p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-0 mb-6 border-b-2 border-zinc-200 dark:border-zinc-800">
        {(['dashboard', 'dominios', 'paginas', 'cambios', 'grafo'] as Tab[]).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-5 py-3 text-sm font-bold uppercase tracking-wide border-b-3 transition-colors ${
              activeTab === tab
                ? 'border-indigo-500 text-indigo-600 dark:text-indigo-400 bg-indigo-50/50 dark:bg-indigo-950/20'
                : 'border-transparent text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-100 hover:bg-zinc-50 dark:hover:bg-zinc-800/30'
            }`}
          >
            {tab === 'dashboard' && '📊 '}
            {tab === 'dominios' && '🌐 '}
            {tab === 'paginas' && '📄 '}
            {tab === 'cambios' && '🔄 '}
            {tab === 'grafo' && '🕸️ '}
            {tab}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      {activeTab === 'dashboard' && (
        <DashboardTab
          stats={stats}
          runs={runs}
          queue={queue}
          onTrigger={handleTriggerCrawl}
          triggering={triggering}
          onRefresh={loadDashboard}
        />
      )}

      {activeTab === 'dominios' && (
        <DominiosTab
          domains={domains}
          showAdd={showAddDomain}
          editing={editingDomain}
          onShowAdd={() => setShowAddDomain(true)}
          onHideAdd={() => setShowAddDomain(false)}
          onAdd={handleAddDomain}
          onEdit={setEditingDomain}
          onUpdate={handleUpdateDomain}
          onCancelEdit={() => setEditingDomain(null)}
          onToggle={handleToggleDomain}
          onDelete={handleDeleteDomain}
        />
      )}

      {activeTab === 'paginas' && (
        <PaginasTab
          pages={pages}
          domains={domains}
          selectedPage={selectedPage}
          pageDetail={pageDetail}
          searchQuery={searchQuery}
          domainFilter={domainFilter}
          onSearch={setSearchQuery}
          onSearchSubmit={handleSearchPages}
          onFilterChange={setDomainFilter}
          onSelectPage={handleSelectPage}
          onRefresh={loadPages}
        />
      )}

      {activeTab === 'cambios' && (
        <CambiosTab
          pages={changedPages}
          selectedPage={selectedPage}
          pageDetail={pageDetail}
          onSelectPage={handleSelectPage}
        />
      )}

      {activeTab === 'grafo' && (
        <GrafoTab
          graphData={graphData}
          selectedEntity={selectedEntity}
          entityRelations={entityRelations}
          onSelectEntity={handleSelectEntity}
        />
      )}
    </div>
  );
}

function DashboardTab({
  stats,
  runs,
  queue,
  onTrigger,
  triggering,
  onRefresh,
}: {
  stats: any;
  runs: CrawlRun[];
  queue: Record<string, number>;
  onTrigger: () => void;
  triggering: boolean;
  onRefresh: () => void;
}) {
  useEffect(() => {
    const interval = setInterval(onRefresh, 30000);
    return () => clearInterval(interval);
  }, [onRefresh]);

  const latestRun = runs.length > 0 ? runs[0] : null;
  const queuePending = queue.pending || 0;
  const queueProcessing = queue.processing || 0;
  const queueDone = queue.done || 0;
  const queueFailed = queue.failed || 0;
  const queueTotal = queuePending + queueProcessing + queueDone + queueFailed;

  return (
    <div className="space-y-6">
      {/* Stats cards */}
      <div className="grid grid-cols-4 gap-4">
        <div className="p-6 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl">
          <div className="text-sm font-bold text-zinc-500 dark:text-zinc-400 uppercase tracking-wide mb-1">
            Total Paginas
          </div>
          <div className="text-3xl font-black text-zinc-900 dark:text-zinc-100">
            {stats?.total_pages?.toLocaleString() || 0}
          </div>
        </div>
        <div className="p-6 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl">
          <div className="text-sm font-bold text-zinc-500 dark:text-zinc-400 uppercase tracking-wide mb-1">
            Total Links
          </div>
          <div className="text-3xl font-black text-zinc-900 dark:text-zinc-100">
            {stats?.total_links?.toLocaleString() || 0}
          </div>
        </div>
        <div className="p-6 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl">
          <div className="text-sm font-bold text-zinc-500 dark:text-zinc-400 uppercase tracking-wide mb-1">
            Dominios Activos
          </div>
          <div className="text-3xl font-black text-zinc-900 dark:text-zinc-100">
            {stats?.total_domains || 0}
          </div>
        </div>
        <div className="p-6 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl">
          <div className="text-sm font-bold text-zinc-500 dark:text-zinc-400 uppercase tracking-wide mb-1">
            Cola Pendiente
          </div>
          <div className="text-3xl font-black text-indigo-600 dark:text-indigo-400">
            {queuePending}
          </div>
        </div>
      </div>

      {/* Latest run + Queue status */}
      <div className="grid grid-cols-2 gap-6">
        {/* Latest run */}
        <div className="p-6 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl">
          <h3 className="text-sm font-black text-zinc-700 dark:text-zinc-300 mb-4 uppercase tracking-wide">
            Ultima Ejecucion
          </h3>
          {!latestRun ? (
            <p className="text-sm text-zinc-500 dark:text-zinc-400">No hay ejecuciones registradas</p>
          ) : (
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-zinc-600 dark:text-zinc-400">Estado:</span>
                <span
                  className={`px-3 py-1 text-xs font-bold rounded-lg ${
                    latestRun.status === 'done'
                      ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-300'
                      : latestRun.status === 'failed'
                      ? 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300'
                      : 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
                  }`}
                >
                  {latestRun.status}
                </span>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <div className="text-xs font-bold text-zinc-500 uppercase">Crawleadas</div>
                  <div className="text-2xl font-black text-zinc-900 dark:text-zinc-100">
                    {latestRun.pages_crawled}
                  </div>
                </div>
                <div>
                  <div className="text-xs font-bold text-zinc-500 uppercase">Nuevas</div>
                  <div className="text-2xl font-black text-emerald-600 dark:text-emerald-400">
                    {latestRun.pages_new}
                  </div>
                </div>
                <div>
                  <div className="text-xs font-bold text-zinc-500 uppercase">Cambiadas</div>
                  <div className="text-2xl font-black text-amber-600 dark:text-amber-400">
                    {latestRun.pages_changed}
                  </div>
                </div>
                <div>
                  <div className="text-xs font-bold text-zinc-500 uppercase">Errores</div>
                  <div className="text-2xl font-black text-red-600 dark:text-red-400">
                    {latestRun.pages_failed}
                  </div>
                </div>
              </div>
              <div className="text-xs text-zinc-500 dark:text-zinc-400 pt-2 border-t border-zinc-200 dark:border-zinc-700">
                Inicio: {timeAgo(latestRun.started_at)}
                {latestRun.finished_at && (
                  <> | Duracion: {Math.round((new Date(latestRun.finished_at).getTime() - new Date(latestRun.started_at).getTime()) / 1000)}s</>
                )}
              </div>
              {latestRun.error_message && (
                <div className="text-xs text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-950/20 p-2 rounded-lg">
                  {latestRun.error_message}
                </div>
              )}
            </div>
          )}
        </div>

        {/* Queue status */}
        <div className="p-6 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl">
          <h3 className="text-sm font-black text-zinc-700 dark:text-zinc-300 mb-4 uppercase tracking-wide">
            Estado de la Cola
          </h3>
          <div className="space-y-3">
            {queueTotal === 0 ? (
              <p className="text-sm text-zinc-500 dark:text-zinc-400">Cola vacia</p>
            ) : (
              <>
                {queuePending > 0 && (
                  <QueueBar label="Pendiente" count={queuePending} total={queueTotal} color="bg-zinc-500" />
                )}
                {queueProcessing > 0 && (
                  <QueueBar label="Procesando" count={queueProcessing} total={queueTotal} color="bg-blue-500" />
                )}
                {queueDone > 0 && (
                  <QueueBar label="Completado" count={queueDone} total={queueTotal} color="bg-emerald-500" />
                )}
                {queueFailed > 0 && (
                  <QueueBar label="Fallido" count={queueFailed} total={queueTotal} color="bg-red-500" />
                )}
              </>
            )}
          </div>
        </div>
      </div>

      {/* Trigger crawl button */}
      <div className="flex justify-center">
        <button
          onClick={onTrigger}
          disabled={triggering}
          className="px-8 py-4 text-base font-bold bg-indigo-600 text-white rounded-xl hover:bg-indigo-700 disabled:opacity-50 transition-colors shadow-lg shadow-indigo-500/20 uppercase tracking-wide"
        >
          {triggering ? 'Iniciando...' : '🚀 Iniciar Crawl'}
        </button>
      </div>
    </div>
  );
}

function QueueBar({ label, count, total, color }: { label: string; count: number; total: number; color: string }) {
  const percent = (count / total) * 100;
  return (
    <div>
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs font-bold text-zinc-600 dark:text-zinc-400 uppercase">{label}</span>
        <span className="text-xs font-bold text-zinc-900 dark:text-zinc-100">{count}</span>
      </div>
      <div className="w-full h-2 bg-zinc-200 dark:bg-zinc-800 rounded-full overflow-hidden">
        <div className={`h-full ${color} rounded-full transition-all`} style={{ width: `${percent}%` }} />
      </div>
    </div>
  );
}

function DominiosTab({
  domains,
  showAdd,
  editing,
  onShowAdd,
  onHideAdd,
  onAdd,
  onEdit,
  onUpdate,
  onCancelEdit,
  onToggle,
  onDelete,
}: {
  domains: CrawlDomain[];
  showAdd: boolean;
  editing: CrawlDomain | null;
  onShowAdd: () => void;
  onHideAdd: () => void;
  onAdd: (data: Partial<CrawlDomain>) => void;
  onEdit: (d: CrawlDomain) => void;
  onUpdate: (id: string, data: Partial<CrawlDomain>) => void;
  onCancelEdit: () => void;
  onToggle: (id: string, active: boolean) => void;
  onDelete: (id: string) => void;
}) {
  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h3 className="text-xs font-bold text-zinc-500 dark:text-zinc-400 uppercase tracking-widest">
          {domains.length} dominios
        </h3>
        <button
          onClick={onShowAdd}
          className="px-4 py-2 text-sm font-bold bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition-colors shadow-lg shadow-indigo-500/20 uppercase"
        >
          + Agregar Dominio
        </button>
      </div>

      {showAdd && (
        <DomainForm
          onSubmit={(data) => {
            onAdd(data);
            onHideAdd();
          }}
          onCancel={onHideAdd}
        />
      )}

      {domains.length === 0 ? (
        <div className="py-12 text-center border-2 border-dashed border-zinc-300 dark:border-zinc-700 rounded-xl">
          <div className="text-4xl mb-3">🌐</div>
          <p className="text-base font-bold text-zinc-500 dark:text-zinc-400">Sin dominios configurados</p>
          <p className="text-sm text-zinc-400 dark:text-zinc-500 mt-1">Agrega un dominio para comenzar a crawlear</p>
        </div>
      ) : (
        <div className="bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl overflow-hidden">
          <table className="w-full">
            <thead className="bg-zinc-50 dark:bg-zinc-800 border-b-2 border-zinc-200 dark:border-zinc-700">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-black text-zinc-600 dark:text-zinc-400 uppercase">Dominio</th>
                <th className="px-4 py-3 text-left text-xs font-black text-zinc-600 dark:text-zinc-400 uppercase">Label</th>
                <th className="px-4 py-3 text-left text-xs font-black text-zinc-600 dark:text-zinc-400 uppercase">Categoria</th>
                <th className="px-4 py-3 text-left text-xs font-black text-zinc-600 dark:text-zinc-400 uppercase">Depth</th>
                <th className="px-4 py-3 text-left text-xs font-black text-zinc-600 dark:text-zinc-400 uppercase">Recrawl (h)</th>
                <th className="px-4 py-3 text-left text-xs font-black text-zinc-600 dark:text-zinc-400 uppercase">Prioridad</th>
                <th className="px-4 py-3 text-left text-xs font-black text-zinc-600 dark:text-zinc-400 uppercase">Paginas</th>
                <th className="px-4 py-3 text-left text-xs font-black text-zinc-600 dark:text-zinc-400 uppercase">Estado</th>
                <th className="px-4 py-3 text-right text-xs font-black text-zinc-600 dark:text-zinc-400 uppercase">Acciones</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-200 dark:divide-zinc-800">
              {domains.map((d) => (
                editing?.id === d.id ? (
                  <DomainEditRow
                    key={d.id}
                    domain={d}
                    onSave={(data) => onUpdate(d.id, data)}
                    onCancel={onCancelEdit}
                  />
                ) : (
                  <tr key={d.id} className="hover:bg-zinc-50 dark:hover:bg-zinc-800/50">
                    <td className="px-4 py-3 text-sm font-medium text-zinc-900 dark:text-zinc-100">{d.domain}</td>
                    <td className="px-4 py-3 text-sm text-zinc-600 dark:text-zinc-400">{d.label}</td>
                    <td className="px-4 py-3 text-xs font-bold text-zinc-500 uppercase">{d.category}</td>
                    <td className="px-4 py-3 text-sm text-zinc-600 dark:text-zinc-400">{d.max_depth}</td>
                    <td className="px-4 py-3 text-sm text-zinc-600 dark:text-zinc-400">{d.recrawl_hours}</td>
                    <td className="px-4 py-3 text-sm text-zinc-600 dark:text-zinc-400">{d.priority}</td>
                    <td className="px-4 py-3 text-sm font-bold text-zinc-900 dark:text-zinc-100">{d.page_count}</td>
                    <td className="px-4 py-3">
                      <button
                        onClick={() => onToggle(d.id, !d.active)}
                        className={`px-2 py-1 text-xs font-bold rounded-lg ${
                          d.active
                            ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-300'
                            : 'bg-zinc-100 dark:bg-zinc-800 text-zinc-500 dark:text-zinc-400'
                        }`}
                      >
                        {d.active ? 'Activo' : 'Inactivo'}
                      </button>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-2">
                        <button
                          onClick={() => onEdit(d)}
                          className="px-2 py-1 text-xs font-bold text-indigo-600 dark:text-indigo-400 hover:underline"
                        >
                          Editar
                        </button>
                        <button
                          onClick={() => onDelete(d.id)}
                          className="px-2 py-1 text-xs font-bold text-red-600 dark:text-red-400 hover:underline"
                        >
                          Eliminar
                        </button>
                      </div>
                    </td>
                  </tr>
                )
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function DomainForm({ onSubmit, onCancel }: { onSubmit: (data: Partial<CrawlDomain>) => void; onCancel: () => void }) {
  const [domain, setDomain] = useState('');
  const [label, setLabel] = useState('');
  const [category, setCategory] = useState('gobierno');
  const [maxDepth, setMaxDepth] = useState(2);
  const [recrawlHours, setRecrawlHours] = useState(24);
  const [priority, setPriority] = useState(5);

  const handleSubmit = () => {
    if (!domain.trim() || !label.trim()) return;
    onSubmit({
      domain: domain.trim(),
      label: label.trim(),
      category,
      max_depth: maxDepth,
      recrawl_hours: recrawlHours,
      priority,
    });
  };

  return (
    <div className="mb-6 p-4 bg-indigo-50 dark:bg-indigo-950/20 border-2 border-indigo-200 dark:border-indigo-800 rounded-xl">
      <h3 className="text-sm font-black text-indigo-700 dark:text-indigo-300 mb-3 uppercase">Nuevo Dominio</h3>
      <div className="grid grid-cols-2 gap-3 mb-3">
        <div>
          <label className="block text-xs font-bold text-zinc-600 dark:text-zinc-400 mb-1 uppercase">Dominio</label>
          <input
            type="text"
            value={domain}
            onChange={(e) => setDomain(e.target.value)}
            placeholder="ejemplo.com"
            className="w-full px-3 py-2 text-sm bg-white dark:bg-zinc-800 border border-zinc-300 dark:border-zinc-600 rounded-lg text-zinc-900 dark:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
        </div>
        <div>
          <label className="block text-xs font-bold text-zinc-600 dark:text-zinc-400 mb-1 uppercase">Label</label>
          <input
            type="text"
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            placeholder="Mi Sitio"
            className="w-full px-3 py-2 text-sm bg-white dark:bg-zinc-800 border border-zinc-300 dark:border-zinc-600 rounded-lg text-zinc-900 dark:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
        </div>
        <div>
          <label className="block text-xs font-bold text-zinc-600 dark:text-zinc-400 mb-1 uppercase">Categoria</label>
          <select
            value={category}
            onChange={(e) => setCategory(e.target.value)}
            className="w-full px-3 py-2 text-sm bg-white dark:bg-zinc-800 border border-zinc-300 dark:border-zinc-600 rounded-lg text-zinc-900 dark:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
          >
            {CATEGORY_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-xs font-bold text-zinc-600 dark:text-zinc-400 mb-1 uppercase">Max Depth (1-5)</label>
          <input
            type="number"
            min={1}
            max={5}
            value={maxDepth}
            onChange={(e) => setMaxDepth(parseInt(e.target.value))}
            className="w-full px-3 py-2 text-sm bg-white dark:bg-zinc-800 border border-zinc-300 dark:border-zinc-600 rounded-lg text-zinc-900 dark:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
        </div>
        <div>
          <label className="block text-xs font-bold text-zinc-600 dark:text-zinc-400 mb-1 uppercase">Recrawl (horas)</label>
          <input
            type="number"
            min={1}
            value={recrawlHours}
            onChange={(e) => setRecrawlHours(parseInt(e.target.value))}
            className="w-full px-3 py-2 text-sm bg-white dark:bg-zinc-800 border border-zinc-300 dark:border-zinc-600 rounded-lg text-zinc-900 dark:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
        </div>
        <div>
          <label className="block text-xs font-bold text-zinc-600 dark:text-zinc-400 mb-1 uppercase">Prioridad (1-10)</label>
          <input
            type="number"
            min={1}
            max={10}
            value={priority}
            onChange={(e) => setPriority(parseInt(e.target.value))}
            className="w-full px-3 py-2 text-sm bg-white dark:bg-zinc-800 border border-zinc-300 dark:border-zinc-600 rounded-lg text-zinc-900 dark:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
        </div>
      </div>
      <div className="flex gap-2 justify-end">
        <button
          onClick={onCancel}
          className="px-4 py-2 text-sm font-bold text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-100 transition-colors"
        >
          Cancelar
        </button>
        <button
          onClick={handleSubmit}
          disabled={!domain.trim() || !label.trim()}
          className="px-4 py-2 text-sm font-bold bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 disabled:opacity-50 transition-colors"
        >
          Agregar
        </button>
      </div>
    </div>
  );
}

function DomainEditRow({
  domain,
  onSave,
  onCancel,
}: {
  domain: CrawlDomain;
  onSave: (data: Partial<CrawlDomain>) => void;
  onCancel: () => void;
}) {
  const [label, setLabel] = useState(domain.label);
  const [category, setCategory] = useState(domain.category);
  const [maxDepth, setMaxDepth] = useState(domain.max_depth);
  const [recrawlHours, setRecrawlHours] = useState(domain.recrawl_hours);
  const [priority, setPriority] = useState(domain.priority);

  const handleSave = () => {
    onSave({ label, category, max_depth: maxDepth, recrawl_hours: recrawlHours, priority });
  };

  return (
    <tr className="bg-indigo-50 dark:bg-indigo-950/20">
      <td className="px-4 py-3 text-sm font-medium text-zinc-900 dark:text-zinc-100">{domain.domain}</td>
      <td className="px-4 py-3">
        <input
          type="text"
          value={label}
          onChange={(e) => setLabel(e.target.value)}
          className="w-full px-2 py-1 text-sm bg-white dark:bg-zinc-800 border border-zinc-300 dark:border-zinc-600 rounded text-zinc-900 dark:text-zinc-100"
        />
      </td>
      <td className="px-4 py-3">
        <select
          value={category}
          onChange={(e) => setCategory(e.target.value)}
          className="w-full px-2 py-1 text-xs bg-white dark:bg-zinc-800 border border-zinc-300 dark:border-zinc-600 rounded text-zinc-900 dark:text-zinc-100"
        >
          {CATEGORY_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>{opt.label}</option>
          ))}
        </select>
      </td>
      <td className="px-4 py-3">
        <input
          type="number"
          min={1}
          max={5}
          value={maxDepth}
          onChange={(e) => setMaxDepth(parseInt(e.target.value))}
          className="w-16 px-2 py-1 text-sm bg-white dark:bg-zinc-800 border border-zinc-300 dark:border-zinc-600 rounded text-zinc-900 dark:text-zinc-100"
        />
      </td>
      <td className="px-4 py-3">
        <input
          type="number"
          min={1}
          value={recrawlHours}
          onChange={(e) => setRecrawlHours(parseInt(e.target.value))}
          className="w-16 px-2 py-1 text-sm bg-white dark:bg-zinc-800 border border-zinc-300 dark:border-zinc-600 rounded text-zinc-900 dark:text-zinc-100"
        />
      </td>
      <td className="px-4 py-3">
        <input
          type="number"
          min={1}
          max={10}
          value={priority}
          onChange={(e) => setPriority(parseInt(e.target.value))}
          className="w-16 px-2 py-1 text-sm bg-white dark:bg-zinc-800 border border-zinc-300 dark:border-zinc-600 rounded text-zinc-900 dark:text-zinc-100"
        />
      </td>
      <td className="px-4 py-3 text-sm text-zinc-600 dark:text-zinc-400">{domain.page_count}</td>
      <td className="px-4 py-3 text-sm text-zinc-600 dark:text-zinc-400">-</td>
      <td className="px-4 py-3">
        <div className="flex items-center justify-end gap-2">
          <button
            onClick={handleSave}
            className="px-2 py-1 text-xs font-bold text-emerald-600 dark:text-emerald-400 hover:underline"
          >
            Guardar
          </button>
          <button
            onClick={onCancel}
            className="px-2 py-1 text-xs font-bold text-zinc-600 dark:text-zinc-400 hover:underline"
          >
            Cancelar
          </button>
        </div>
      </td>
    </tr>
  );
}

function PaginasTab({
  pages,
  domains,
  selectedPage,
  pageDetail,
  searchQuery,
  domainFilter,
  onSearch,
  onSearchSubmit,
  onFilterChange,
  onSelectPage,
  onRefresh,
}: {
  pages: CrawledPage[];
  domains: CrawlDomain[];
  selectedPage: CrawledPage | null;
  pageDetail: any;
  searchQuery: string;
  domainFilter: string;
  onSearch: (q: string) => void;
  onSearchSubmit: () => void;
  onFilterChange: (f: string) => void;
  onSelectPage: (p: CrawledPage) => void;
  onRefresh: () => void;
}) {
  return (
    <div className="flex gap-6">
      {/* Left: page list */}
      <div className="flex-1">
        {/* Top bar */}
        <div className="flex gap-3 mb-4">
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => onSearch(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && onSearchSubmit()}
            placeholder="Buscar paginas..."
            className="flex-1 px-4 py-2 text-sm bg-white dark:bg-zinc-800 border-2 border-zinc-300 dark:border-zinc-600 rounded-lg text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
          <select
            value={domainFilter}
            onChange={(e) => onFilterChange(e.target.value)}
            className="px-4 py-2 text-sm bg-white dark:bg-zinc-800 border-2 border-zinc-300 dark:border-zinc-600 rounded-lg text-zinc-900 dark:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
          >
            <option value="all">Todos los dominios</option>
            {domains.map((d) => (
              <option key={d.id} value={d.id}>{d.label}</option>
            ))}
          </select>
          <button
            onClick={onRefresh}
            className="px-4 py-2 text-sm font-bold text-indigo-600 dark:text-indigo-400 border-2 border-indigo-300 dark:border-indigo-700 rounded-lg hover:bg-indigo-50 dark:hover:bg-indigo-950/20 transition-colors"
          >
            Actualizar
          </button>
        </div>

        {/* Page list */}
        {pages.length === 0 ? (
          <div className="py-12 text-center border-2 border-dashed border-zinc-300 dark:border-zinc-700 rounded-xl">
            <div className="text-4xl mb-3">📄</div>
            <p className="text-base font-bold text-zinc-500 dark:text-zinc-400">No hay paginas</p>
            <p className="text-sm text-zinc-400 dark:text-zinc-500 mt-1">Inicia un crawl para recopilar paginas</p>
          </div>
        ) : (
          <div className="space-y-2">
            <p className="text-xs font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-wide mb-3">
              {pages.length} paginas
            </p>
            {pages.map((p) => (
              <button
                key={p.id}
                onClick={() => onSelectPage(p)}
                className={`w-full text-left p-3 rounded-lg border-2 transition-all ${
                  selectedPage?.id === p.id
                    ? 'bg-indigo-50 dark:bg-indigo-950/40 border-indigo-400 dark:border-indigo-600'
                    : 'bg-white dark:bg-zinc-900 border-zinc-200 dark:border-zinc-800 hover:border-zinc-300 dark:hover:border-zinc-700'
                }`}
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0 flex-1">
                    <div className="text-sm font-bold text-zinc-900 dark:text-zinc-100 truncate">{p.title || 'Sin titulo'}</div>
                    <div className="text-xs text-zinc-500 dark:text-zinc-400 truncate mt-0.5">{p.url}</div>
                    {p.summary && (
                      <div className="text-xs text-zinc-600 dark:text-zinc-400 line-clamp-1 mt-1">{p.summary}</div>
                    )}
                  </div>
                  <div className="flex flex-col items-end gap-1 shrink-0">
                    <span className={`px-2 py-0.5 text-[10px] font-bold rounded ${
                      p.status_code === 200
                        ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-600'
                        : 'bg-red-100 dark:bg-red-900/30 text-red-600'
                    }`}>
                      {p.status_code}
                    </span>
                    {p.changed && (
                      <span className="px-2 py-0.5 text-[10px] font-bold rounded bg-amber-100 dark:bg-amber-900/30 text-amber-600">
                        CAMBIO
                      </span>
                    )}
                    {p.enriched && (
                      <span className="px-2 py-0.5 text-[10px] font-bold rounded bg-blue-100 dark:bg-blue-900/30 text-blue-600">
                        ENRIQUECIDA
                      </span>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-3 mt-2 text-[10px] text-zinc-500">
                  <span>Links: {p.links_out}/{p.links_in}</span>
                  <span>{timeAgo(p.last_crawled_at)}</span>
                </div>
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Right: detail panel */}
      {selectedPage && (
        <div className="w-96 shrink-0">
          <PageDetail page={selectedPage} detail={pageDetail} />
        </div>
      )}
    </div>
  );
}

function PageDetail({ page, detail }: { page: CrawledPage; detail: any }) {
  return (
    <div className="p-4 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl max-h-[700px] overflow-y-auto">
      <h3 className="text-lg font-black text-zinc-900 dark:text-zinc-100 mb-2">{page.title || 'Sin titulo'}</h3>
      <a
        href={page.url}
        target="_blank"
        rel="noopener noreferrer"
        className="text-sm text-indigo-600 dark:text-indigo-400 hover:underline break-all"
      >
        {page.url}
      </a>

      {page.summary && (
        <div className="mt-4">
          <h4 className="text-xs font-bold text-zinc-500 uppercase mb-1">Summary</h4>
          <p className="text-sm text-zinc-700 dark:text-zinc-300">{page.summary}</p>
        </div>
      )}

      {page.tags.length > 0 && (
        <div className="mt-4">
          <h4 className="text-xs font-bold text-zinc-500 uppercase mb-2">Tags</h4>
          <div className="flex flex-wrap gap-1">
            {page.tags.map((tag, i) => (
              <span
                key={i}
                className="px-2 py-0.5 text-xs font-medium rounded-lg bg-indigo-50 dark:bg-indigo-950/30 text-indigo-600 dark:text-indigo-400 border border-indigo-200 dark:border-indigo-800"
              >
                {tag}
              </span>
            ))}
          </div>
        </div>
      )}

      {page.entities && Object.keys(page.entities).length > 0 && (
        <div className="mt-4">
          <h4 className="text-xs font-bold text-zinc-500 uppercase mb-2">Entidades</h4>
          <div className="space-y-2">
            {Object.entries(page.entities).map(([type, items]) => (
              <div key={type}>
                <div className="text-xs font-bold text-zinc-600 dark:text-zinc-400 capitalize">{type}:</div>
                <div className="flex flex-wrap gap-1 mt-1">
                  {(items as string[]).map((item, i) => (
                    <span
                      key={i}
                      className="px-2 py-0.5 text-xs rounded-lg bg-zinc-100 dark:bg-zinc-800 text-zinc-700 dark:text-zinc-300"
                    >
                      {item}
                    </span>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {page.sentiment !== 'unknown' && (
        <div className="mt-4">
          <h4 className="text-xs font-bold text-zinc-500 uppercase mb-1">Sentiment</h4>
          <span className={`text-sm font-bold capitalize ${SENTIMENT_COLORS[page.sentiment] || ''}`}>
            {page.sentiment}
          </span>
        </div>
      )}

      <div className="mt-4 pt-4 border-t border-zinc-200 dark:border-zinc-700">
        <h4 className="text-xs font-bold text-zinc-500 uppercase mb-2">Metadata</h4>
        <div className="space-y-1 text-xs text-zinc-600 dark:text-zinc-400">
          <div>Status: <span className="font-bold">{page.status_code}</span></div>
          <div>Depth: <span className="font-bold">{page.depth}</span></div>
          <div>Links Out: <span className="font-bold">{page.links_out}</span></div>
          <div>Links In: <span className="font-bold">{page.links_in}</span></div>
          <div>Content: <span className="font-bold">{formatBytes(page.content_length)}</span></div>
          <div>Crawl Count: <span className="font-bold">{page.crawl_count}</span></div>
          <div>First Seen: <span className="font-bold">{timeAgo(page.first_seen_at)}</span></div>
          <div>Last Crawled: <span className="font-bold">{timeAgo(page.last_crawled_at)}</span></div>
        </div>
      </div>

      {page.changed && page.change_summary && (
        <div className="mt-4 p-3 bg-amber-50 dark:bg-amber-950/20 border border-amber-200 dark:border-amber-800 rounded-lg">
          <h4 className="text-xs font-bold text-amber-700 dark:text-amber-300 uppercase mb-1">Cambio Detectado</h4>
          <p className="text-xs text-amber-600 dark:text-amber-400">{page.change_summary}</p>
        </div>
      )}

      {detail?.links_out && detail.links_out.length > 0 && (
        <div className="mt-4">
          <h4 className="text-xs font-bold text-zinc-500 uppercase mb-2">Links Out ({detail.links_out.length})</h4>
          <div className="space-y-1 max-h-40 overflow-y-auto">
            {detail.links_out.slice(0, 10).map((link: CrawlLink, i: number) => (
              <div key={i} className="text-xs text-zinc-600 dark:text-zinc-400 truncate">
                <span className="font-medium">{link.anchor_text || 'No text'}</span> → {link.target_url}
              </div>
            ))}
            {detail.links_out.length > 10 && (
              <div className="text-xs text-zinc-500">+{detail.links_out.length - 10} mas</div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function CambiosTab({
  pages,
  selectedPage,
  pageDetail,
  onSelectPage,
}: {
  pages: CrawledPage[];
  selectedPage: CrawledPage | null;
  pageDetail: any;
  onSelectPage: (p: CrawledPage) => void;
}) {
  return (
    <div className="flex gap-6">
      {/* Left: changed pages list */}
      <div className="flex-1">
        {pages.length === 0 ? (
          <div className="py-12 text-center border-2 border-dashed border-zinc-300 dark:border-zinc-700 rounded-xl">
            <div className="text-4xl mb-3">🔄</div>
            <p className="text-base font-bold text-zinc-500 dark:text-zinc-400">No hay cambios recientes</p>
            <p className="text-sm text-zinc-400 dark:text-zinc-500 mt-1">Los cambios detectados apareceran aqui</p>
          </div>
        ) : (
          <div className="space-y-3">
            <p className="text-xs font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-wide">
              {pages.length} paginas cambiadas
            </p>
            {pages.map((p) => (
              <button
                key={p.id}
                onClick={() => onSelectPage(p)}
                className={`w-full text-left p-4 rounded-xl border-2 transition-all ${
                  selectedPage?.id === p.id
                    ? 'bg-amber-50 dark:bg-amber-950/20 border-amber-400 dark:border-amber-600'
                    : 'bg-white dark:bg-zinc-900 border-zinc-200 dark:border-zinc-800 hover:border-zinc-300 dark:hover:border-zinc-700'
                }`}
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0 flex-1">
                    <div className="text-base font-bold text-zinc-900 dark:text-zinc-100">{p.title || 'Sin titulo'}</div>
                    <div className="text-xs text-zinc-500 dark:text-zinc-400 truncate mt-1">{p.url}</div>
                    {p.change_summary && (
                      <div className="text-sm text-amber-700 dark:text-amber-300 mt-2 font-medium">
                        {p.change_summary}
                      </div>
                    )}
                  </div>
                  <div className="text-xs text-zinc-500 shrink-0">{timeAgo(p.last_crawled_at)}</div>
                </div>
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Right: detail panel */}
      {selectedPage && (
        <div className="w-96 shrink-0">
          <PageDetail page={selectedPage} detail={pageDetail} />
        </div>
      )}
    </div>
  );
}

function GrafoTab({
  graphData,
  selectedEntity,
  entityRelations,
  onSelectEntity,
}: {
  graphData: { nodes: GraphNode[]; edges: GraphEdge[] } | null;
  selectedEntity: string | null;
  entityRelations: any;
  onSelectEntity: (id: string) => void;
}) {
  if (!graphData || graphData.nodes.length === 0) {
    return (
      <div className="py-12 text-center border-2 border-dashed border-zinc-300 dark:border-zinc-700 rounded-xl">
        <div className="text-4xl mb-3">🕸️</div>
        <p className="text-base font-bold text-zinc-500 dark:text-zinc-400">No hay datos de grafo</p>
        <p className="text-sm text-zinc-400 dark:text-zinc-500 mt-1">Las entidades apareceran cuando las paginas sean enriquecidas</p>
      </div>
    );
  }

  const nodesByType: Record<string, GraphNode[]> = {};
  graphData.nodes.forEach((node) => {
    if (!nodesByType[node.type]) nodesByType[node.type] = [];
    nodesByType[node.type].push(node);
  });

  const typeColors: Record<string, string> = {
    person: 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700',
    organization: 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 border-green-300 dark:border-green-700',
    place: 'bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300 border-amber-300 dark:border-amber-700',
    other: 'bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300 border-purple-300 dark:border-purple-700',
  };

  const typeEmojis: Record<string, string> = {
    person: '👤',
    organization: '🏛️',
    place: '📍',
    other: '🔖',
  };

  return (
    <div className="flex gap-6">
      {/* Left: entity list */}
      <div className="w-80 shrink-0">
        <h3 className="text-xs font-bold text-zinc-500 dark:text-zinc-400 uppercase tracking-widest mb-3">
          Entidades ({graphData.nodes.length})
        </h3>
        <div className="space-y-4">
          {Object.entries(nodesByType).map(([type, nodes]) => (
            <div key={type} className="p-3 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl">
              <h4 className="text-sm font-black text-zinc-700 dark:text-zinc-300 mb-2 uppercase flex items-center gap-2">
                <span>{typeEmojis[type] || '🔖'}</span>
                <span>{type}</span>
                <span className="text-xs font-bold text-zinc-500">({nodes.length})</span>
              </h4>
              <div className="space-y-1">
                {nodes.map((node) => {
                  const connectionCount = graphData.edges.filter((e) => e.source === node.id || e.target === node.id).length;
                  return (
                    <button
                      key={node.id}
                      onClick={() => onSelectEntity(node.id)}
                      className={`w-full text-left px-3 py-2 rounded-lg text-sm font-medium transition-all border ${
                        selectedEntity === node.id
                          ? typeColors[type] || typeColors.other
                          : 'bg-zinc-50 dark:bg-zinc-800 text-zinc-700 dark:text-zinc-300 border-zinc-200 dark:border-zinc-700 hover:border-zinc-300 dark:hover:border-zinc-600'
                      }`}
                    >
                      <div className="flex items-center justify-between">
                        <span>{node.name}</span>
                        {connectionCount > 0 && (
                          <span className="text-xs px-1.5 py-0.5 rounded-full bg-zinc-200 dark:bg-zinc-700 text-zinc-600 dark:text-zinc-400">
                            {connectionCount}
                          </span>
                        )}
                      </div>
                    </button>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Right: relationships */}
      <div className="flex-1">
        {!selectedEntity ? (
          <div className="flex flex-col items-center justify-center h-64">
            <div className="text-5xl mb-4">🕸️</div>
            <p className="text-lg font-bold text-zinc-600 dark:text-zinc-400">Selecciona una entidad</p>
            <p className="text-sm text-zinc-400 dark:text-zinc-500 mt-1">para ver sus relaciones</p>
          </div>
        ) : entityRelations ? (
          <div className="space-y-4">
            <h3 className="text-lg font-black text-zinc-900 dark:text-zinc-100 uppercase">
              Relaciones de {graphData.nodes.find((n) => n.id === selectedEntity)?.name}
            </h3>
            {entityRelations.relationships && entityRelations.relationships.length === 0 ? (
              <p className="text-sm text-zinc-500 dark:text-zinc-400">No hay relaciones registradas</p>
            ) : (
              <div className="space-y-3">
                {entityRelations.relationships?.map((rel: any, i: number) => (
                  <div
                    key={i}
                    className="p-4 bg-white dark:bg-zinc-900 border-2 border-zinc-200 dark:border-zinc-800 rounded-xl"
                  >
                    <div className="flex items-center gap-3">
                      <div className="text-sm font-bold text-zinc-900 dark:text-zinc-100">{rel.source_name}</div>
                      <div className="px-2 py-1 text-xs font-bold rounded-lg bg-indigo-100 dark:bg-indigo-900/30 text-indigo-700 dark:text-indigo-300">
                        {rel.relation_type}
                      </div>
                      <div className="text-sm font-bold text-zinc-900 dark:text-zinc-100">{rel.target_name}</div>
                    </div>
                    <div className="mt-2">
                      <div className="w-full h-2 bg-zinc-200 dark:bg-zinc-800 rounded-full overflow-hidden">
                        <div
                          className="h-full bg-indigo-500 rounded-full"
                          style={{ width: `${Math.min(rel.strength * 100, 100)}%` }}
                        />
                      </div>
                      <div className="text-xs text-zinc-500 mt-1">Strength: {(rel.strength * 100).toFixed(0)}%</div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        ) : (
          <div className="flex justify-center py-8">
            <div className="animate-spin w-8 h-8 border-3 border-indigo-500 border-t-transparent rounded-full" />
          </div>
        )}
      </div>
    </div>
  );
}
