const API_BASE = '/api';

export interface Article {
  id: string;
  title: string;
  url: string;
  canonical_url: string;
  source: string;
  summary: string;
  clean_text: string;
  image_url: string;
  region: string;
  status: string;
  pinned: boolean;
  tags: string[];
  evidence_policy: string;
  evidence_expires_at: string;
  published_at: string;
  created_at: string;
}

export interface Source {
  id: string;
  name: string;
  base_url: string;
  region: string;
  feed_type: string;
  feed_url: string;
  list_urls: string[];
  link_selector: string;
  title_selector: string;
  body_selector: string;
  date_selector: string;
  active: boolean;
  created_at: string;
}

export interface ItemsResponse {
  items: Article[];
  count?: number;
  limit?: number;
  offset?: number;
}

export interface SearchResponse {
  results: Article[];
  count?: number;
  total?: number;
}

export interface Note {
  id: string;
  article_id: string;
  user_id: string;
  content: string;
  created_at: string;
}

export interface NotesResponse {
  notes: Note[];
  count: number;
}

export type WatchKind = 'marca' | 'tema' | 'persona';

export interface WatchSocial {
  network: string;
  handle: string;
  url: string;
}

export interface WatchPerson {
  name: string;
  role: string;
  url: string;
}

export interface WatchlistOrg {
  id: string;
  user_id: string;
  name: string;
  kind: WatchKind;
  website: string;
  keywords: string[];
  youtube_channels: string[];
  aliases: string[];
  context_aliases: string[];
  people: WatchPerson[];
  exclude_terms: string[];
  hashtags: string[];
  socials: WatchSocial[];
  alert_feeds: string[];
  to_confirm: boolean;
  sources_evidence: { dato: string; url: string }[];
  active: boolean;
  unseen: number;
  created_at: string;
  updated_at: string;
}

export type WatchlistOrgInput = {
  name: string;
  kind: WatchKind;
  website?: string;
  keywords?: string[];
  youtube_channels?: string[];
  aliases?: string[];
  context_aliases?: string[];
  people?: WatchPerson[];
  exclude_terms?: string[];
  hashtags?: string[];
  socials?: WatchSocial[];
  alert_feeds?: string[];
  to_confirm?: boolean;
  active?: boolean;
};

export interface WatchlistOrgsResponse {
  orgs: WatchlistOrg[];
  count: number;
  max_brands: number;
}

export interface WatchlistHit {
  id: string;
  org_id: string;
  org_name: string;
  source_type: 'google_news' | 'bing_news' | 'web' | 'local' | 'youtube' | 'reddit' | 'bluesky' | 'sitio' | 'mastodon' | 'alertas' | 'podcast';
  network: string;
  matched_hashtag: string;
  matched_person: string;
  title: string;
  url: string;
  url_hash: string;
  snippet: string;
  seen: boolean;
  published_at: string | null;
  created_at: string;
  // Archivo permanente
  original_url: string;
  medio: string;
  fecha: string;
  canonical_url: string;
  resolved_url: string;
  site_name: string;
  authors: string[];
  lang: string;
  image_url: string;
  summary: string;
  archive_status: 'pendiente' | 'copia' | 'parcial' | 'sin_copia';
  archive_attempts: number;
  archived_at: string | null;
  wayback_url: string;
  edited: boolean;
  has_text: boolean;
  apa: { plain: string; markdown: string };
}

export interface ArchivoStats {
  total: number;
  copia: number;
  parcial: number;
  sin_copia: number;
  pendiente: number;
  con_autor: number;
  con_fecha: number;
  con_wayback: number;
}

export interface ArchivoFilters {
  q?: string;
  desde?: string;
  hasta?: string;
  fuente?: string;
}

export interface ArchivoResponse {
  org: { id: string; name: string };
  total: number;
  stats: ArchivoStats;
  hits: WatchlistHit[];
  bibliografia: { plain: string; markdown: string };
}

export interface HitMetaInput {
  title: string;
  site_name: string;
  authors: string[];
  published_at: string;
  summary: string;
  canonical_url: string;
}

export interface WatchlistHitsResponse {
  hits: WatchlistHit[];
  count: number;
}

export interface UnseenCountResponse {
  unseen: number;
}

// ── Crawler (Government Document Crawler) ───────────────────────

export type OpportunityLevel = 'federal' | 'estatal' | 'municipal' | 'local';

export interface CrawlDomain {
  id: string;
  domain: string;
  label: string;
  category: string;
  max_depth: number;
  recrawl_hours: number;
  priority: number;
  active: boolean;
  level?: OpportunityLevel;
  page_count: number;
  last_crawled_at: string | null;
  created_at: string;
}

export interface CrawledPage {
  id: string;
  url: string;
  url_hash: string;
  domain_id: string;
  title: string;
  clean_text: string;
  content_hash: string;
  summary: string;
  tags: string[];
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

export interface CrawlRun {
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

export interface CrawlLink {
  id: string;
  source_page_id: string;
  target_url: string;
  anchor_text: string;
}

export interface CrawlerStatsResponse {
  total_pages: number;
  total_links: number;
  total_domains: number;
  queue: Record<string, number>;
  latest_run: CrawlRun | null;
}

// ── Opportunities (Grant/RFP Tracker) ───────────────────────────

export type FundType = 'grant' | 'subasta' | 'rfp' | 'otros';

export interface KeptDocument {
  id: string;
  opportunity_id: string | null;
  title: string;
  agency: string;
  reference: string;
  size_bytes: number;
  created_at: string;
  file_url: string;
  saved: boolean;
}

export interface KeptDocumentsResponse {
  documents: KeptDocument[];
  kept_bytes: number;
  total_bytes: number;
}

export interface Opportunity {
  id: string;
  page_id: string | null;
  domain_id: string;
  title: string;
  agency: string;
  summary: string;
  amount: string;
  amount_cents: number;
  deadline: string | null;
  contact_name: string;
  contact_email: string;
  category: 'rfp' | 'grant' | 'funding' | 'proposal' | 'contract' | 'fellowship' | 'scholarship' | 'other';
  status: 'open' | 'closing_soon' | 'closed' | 'unknown';
  source_url: string;
  confidence: number;
  raw_extraction: Record<string, any>;
  reviewed: boolean;
  reference?: string;
  level?: OpportunityLevel;
  source_kind?: 'pagina' | 'pdf' | 'api';
  document_url?: string;
  document_scanned?: boolean;
  document_local?: boolean;
  document_kept?: boolean;
  fund_type?: FundType;
  contact_phone?: string;
  evidence?: { deadline?: string; amount?: string; reference?: string; title?: string };
  created_at: string;
  updated_at: string;
}

export interface OpportunitiesResponse {
  opportunities: Opportunity[];
  count: number;
  total: number;
}

export interface OpportunityStatsResponse {
  total: number;
  by_status: Record<string, number>;
  pending?: number;
  closing_soon?: number;
}

// Rutas públicas: un 401 aquí es un error normal, no una sesión vencida.
const NO_REDIRECT = new Set(['/login', '/setup']);

async function fetchAPI<T = any>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: 'same-origin',
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
    ...options,
  });

  if (res.status === 401 && !NO_REDIRECT.has(path)) {
    if (typeof window !== 'undefined' && window.location.pathname !== '/login') {
      window.location.href = '/login';
    }
    throw new Error('Sesión expirada');
  }

  if (!res.ok) {
    const text = await res.text();
    let msg = text;
    try { msg = JSON.parse(text).error || text; } catch { /* texto plano */ }
    throw new Error(msg || `Request failed: ${res.status}`);
  }

  // Handle 204 No Content
  if (res.status === 204) {
    return {} as T;
  }

  return res.json();
}

export interface User {
  id: string;
  email: string;
  role: string;
  created_at?: string;
}

export const api = {
  // Auth
  login: (email: string, password: string): Promise<{ user: User }> =>
    fetchAPI('/login', { method: 'POST', body: JSON.stringify({ email, password }) }),

  setupStatus: (): Promise<{ needs_setup: boolean }> =>
    fetchAPI('/setup'),

  setup: (email: string, password: string): Promise<any> =>
    fetchAPI('/setup', { method: 'POST', body: JSON.stringify({ email, password }) }),

  logout: (): Promise<{ status: string }> =>
    fetchAPI('/logout', { method: 'POST' }),

  me: (): Promise<User> =>
    fetchAPI('/me'),

  // Items
  getItems: (status: string, limit = 30, offset = 0, date = ''): Promise<ItemsResponse> =>
    fetchAPI(`/items?status=${status}&limit=${limit}&offset=${offset}${date ? `&date=${date}` : ''}`),

  saveItem: (id: string) =>
    fetchAPI(`/items/${id}/save`, { method: 'POST' }),

  trashItem: (id: string) =>
    fetchAPI(`/items/${id}/trash`, { method: 'POST' }),

  trashItems: (ids: string[]): Promise<{ count: number }> =>
    fetchAPI('/items/trash', { method: 'POST', body: JSON.stringify({ ids }) }),

  undoItems: (ids: string[]): Promise<{ count: number }> =>
    fetchAPI('/items/undo', { method: 'POST', body: JSON.stringify({ ids }) }),

  pinItem: (id: string) =>
    fetchAPI(`/items/${id}/pin`, { method: 'POST' }),

  undoItem: (id: string, previousStatus: string) =>
    fetchAPI(`/items/${id}/undo`, {
      method: 'POST',
      body: JSON.stringify({ previous_status: previousStatus }),
    }),

  // Search
  search: (params: Record<string, string>): Promise<SearchResponse> =>
    fetchAPI(`/search?${new URLSearchParams(params)}`),

  // Sources
  getSources: async (): Promise<Source[]> => {
    const data = await fetchAPI<{ sources: Source[]; count: number }>('/sources');
    return data.sources || [];
  },

  createSource: (data: Partial<Source>) =>
    fetchAPI('/sources', { method: 'POST', body: JSON.stringify(data) }),

  updateSource: (id: string, data: Partial<Source>) =>
    fetchAPI(`/sources/${id}`, { method: 'PUT', body: JSON.stringify(data) }),

  toggleSource: (id: string, active: boolean) =>
    fetchAPI(`/sources/${id}/toggle`, { method: 'PATCH', body: JSON.stringify({ active }) }),

  deleteSource: (id: string) =>
    fetchAPI(`/sources/${id}`, { method: 'DELETE' }),

  quickCreateSource: (url: string, region?: string): Promise<{ source: Source; feed_type: string; detected: boolean; message: string }> =>
    fetchAPI('/sources/quick', { method: 'POST', body: JSON.stringify({ url, region: region || undefined }) }),

  // Notes
  getNotes: (articleId: string): Promise<NotesResponse> =>
    fetchAPI(`/items/${articleId}/notes`),

  createNote: (articleId: string, content: string): Promise<Note> =>
    fetchAPI(`/items/${articleId}/notes`, {
      method: 'POST',
      body: JSON.stringify({ content }),
    }),

  deleteNote: (noteId: string) =>
    fetchAPI(`/notes/${noteId}`, { method: 'DELETE' }),

  // Watchlist
  getWatchlistOrgs: (): Promise<WatchlistOrgsResponse> =>
    fetchAPI('/watchlist/orgs'),

  createWatchlistOrg: (data: WatchlistOrgInput): Promise<WatchlistOrg> =>
    fetchAPI('/watchlist/orgs', { method: 'POST', body: JSON.stringify(data) }),

  updateWatchlistOrg: (id: string, data: WatchlistOrgInput): Promise<WatchlistOrg> =>
    fetchAPI(`/watchlist/orgs/${id}`, { method: 'PUT', body: JSON.stringify(data) }),

  deleteWatchlistOrg: (id: string) =>
    fetchAPI(`/watchlist/orgs/${id}`, { method: 'DELETE' }),

  toggleWatchlistOrg: (id: string, active: boolean) =>
    fetchAPI(`/watchlist/orgs/${id}/toggle`, { method: 'PATCH', body: JSON.stringify({ active }) }),

  getWatchlistHits: (params?: { limit?: number; offset?: number; org_id?: string }): Promise<WatchlistHitsResponse> => {
    const qs = new URLSearchParams();
    if (params?.limit) qs.set('limit', String(params.limit));
    if (params?.offset) qs.set('offset', String(params.offset));
    if (params?.org_id) qs.set('org_id', params.org_id);
    return fetchAPI(`/watchlist/hits?${qs}`);
  },

  getUnseenHitCount: (): Promise<UnseenCountResponse> =>
    fetchAPI('/watchlist/hits/unseen'),

  markHitSeen: (id: string) =>
    fetchAPI(`/watchlist/hits/${id}/seen`, { method: 'POST' }),

  markAllHitsSeen: (): Promise<{ status: string; marked: number }> =>
    fetchAPI('/watchlist/hits/seen-all', { method: 'POST' }),

  deleteHit: (id: string) =>
    fetchAPI(`/watchlist/hits/${id}`, { method: 'DELETE' }),

  markOrgHitsSeen: (id: string) =>
    fetchAPI(`/watchlist/orgs/${id}/seen-all`, { method: 'POST' }),

  getArchivo: (orgId: string, f: ArchivoFilters = {}): Promise<ArchivoResponse> => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(f)) if (v) qs.set(k, v);
    return fetchAPI(`/watchlist/orgs/${orgId}/archivo?${qs}`);
  },

  /** Enlace de descarga (Markdown o CSV) con los mismos filtros de la pantalla. */
  archivoExportURL: (orgId: string, formato: 'md' | 'csv', f: ArchivoFilters = {}): string => {
    const qs = new URLSearchParams({ formato });
    for (const [k, v] of Object.entries(f)) if (v) qs.set(k, v);
    return `${API_BASE}/watchlist/orgs/${orgId}/archivo/exportar?${qs}`;
  },

  getHitCopia: (id: string): Promise<{ hit: WatchlistHit; text: string }> =>
    fetchAPI(`/watchlist/hits/${id}/copia`),

  updateHitMeta: (id: string, data: HitMetaInput): Promise<WatchlistHit> =>
    fetchAPI(`/watchlist/hits/${id}/meta`, { method: 'PATCH', body: JSON.stringify(data) }),

  retryHitCopia: (id: string): Promise<{ status: string }> =>
    fetchAPI(`/watchlist/hits/${id}/reintentar`, { method: 'POST' }),

  runArchivo: (): Promise<{ status: string }> =>
    fetchAPI('/watchlist/archivo/run', { method: 'POST' }),

  triggerWatchlistScan: (): Promise<{ status: string; message: string }> =>
    fetchAPI('/watchlist/scan', { method: 'POST' }),

  getWatchlistFeedURL: (): Promise<{ url: string }> =>
    fetchAPI('/watchlist/feed-url'),

  regenerateWatchlistFeedURL: (): Promise<{ url: string }> =>
    fetchAPI('/watchlist/feed-url/regenerate', { method: 'POST' }),

  // Export
  exportArticle: (id: string): string =>
    `${API_BASE}/items/${id}/export`,

  // Collect
  collectItem: (url: string, title?: string, region?: string, snippet?: string) =>
    fetchAPI('/collect', {
      method: 'POST',
      body: JSON.stringify({ url, title: title || undefined, region: region || undefined, snippet: snippet || undefined }),
    }),

  // Admin: trigger ingestion
  triggerIngest: (): Promise<{ status: string; message: string }> =>
    fetchAPI('/admin/ingest', { method: 'POST' }),

  // Test scrape
  testSource: (id: string): Promise<{ success: boolean; title: string; text_length: number; image_found: boolean; error?: string }> =>
    fetchAPI(`/sources/${id}/test`, { method: 'POST' }),

  // Crawler (Government Document Crawler)
  getCrawlerStats: (): Promise<CrawlerStatsResponse> =>
    fetchAPI('/crawler/stats'),

  getCrawlerRuns: (limit = 20): Promise<{ runs: CrawlRun[]; count: number }> =>
    fetchAPI(`/crawler/runs?limit=${limit}`),

  getCrawlerQueue: (): Promise<{ queue: Record<string, number> }> =>
    fetchAPI('/crawler/queue'),

  getCrawlerPages: (params?: { limit?: number; offset?: number; domain_id?: string }): Promise<{ pages: CrawledPage[]; count: number }> => {
    const qs = new URLSearchParams();
    if (params?.limit) qs.set('limit', String(params.limit));
    if (params?.offset) qs.set('offset', String(params.offset));
    if (params?.domain_id) qs.set('domain_id', params.domain_id);
    return fetchAPI(`/crawler/pages?${qs}`);
  },

  searchCrawlerPages: (q: string, limit = 50): Promise<{ pages: CrawledPage[]; count: number }> =>
    fetchAPI(`/crawler/pages/search?q=${encodeURIComponent(q)}&limit=${limit}`),

  getCrawlerChangedPages: (limit = 50): Promise<{ pages: CrawledPage[]; count: number }> =>
    fetchAPI(`/crawler/pages/changes?limit=${limit}`),

  getCrawlerPage: (id: string): Promise<{ page: CrawledPage; links_out: CrawlLink[]; links_in: CrawlLink[] }> =>
    fetchAPI(`/crawler/pages/${id}`),

  getCrawlerDomains: (): Promise<{ domains: CrawlDomain[]; count: number }> =>
    fetchAPI('/crawler/domains'),

  createCrawlerDomain: (data: { domain: string; label: string; category: string; level?: string; max_depth: number; recrawl_hours: number; priority: number }): Promise<CrawlDomain> =>
    fetchAPI('/crawler/domains', { method: 'POST', body: JSON.stringify(data) }),

  updateCrawlerDomain: (id: string, data: { label: string; category: string; level?: string; max_depth: number; recrawl_hours: number; priority: number }): Promise<{ status: string }> =>
    fetchAPI(`/crawler/domains/${id}`, { method: 'PUT', body: JSON.stringify(data) }),

  toggleCrawlerDomain: (id: string, active: boolean): Promise<{ status: string }> =>
    fetchAPI(`/crawler/domains/${id}/toggle`, { method: 'PATCH', body: JSON.stringify({ active }) }),

  deleteCrawlerDomain: (id: string): Promise<{ status: string }> =>
    fetchAPI(`/crawler/domains/${id}`, { method: 'DELETE' }),

  triggerCrawl: (): Promise<{ status: string; message: string }> =>
    fetchAPI('/crawler/trigger', { method: 'POST' }),

  // Opportunities (Grant/RFP Tracker)
  getOpportunities: (params?: { status?: string; category?: string; domain_id?: string; sort?: string; limit?: number; offset?: number; reviewed?: boolean; level?: string }): Promise<OpportunitiesResponse> => {
    const qs = new URLSearchParams();
    if (params?.status) qs.set('status', params.status);
    if (params?.category) qs.set('category', params.category);
    if (params?.domain_id) qs.set('domain_id', params.domain_id);
    if (params?.sort) qs.set('sort', params.sort);
    if (params?.reviewed !== undefined) qs.set('reviewed', String(params.reviewed));
    if (params?.level) qs.set('level', params.level);
    if (params?.limit) qs.set('limit', String(params.limit));
    if (params?.offset) qs.set('offset', String(params.offset));
    return fetchAPI(`/opportunities?${qs}`);
  },

  getOpportunityStats: (): Promise<OpportunityStatsResponse> =>
    fetchAPI('/opportunities/stats'),

  searchOpportunities: (q: string, limit = 50): Promise<{ opportunities: Opportunity[]; count: number }> =>
    fetchAPI(`/opportunities/search?q=${encodeURIComponent(q)}&limit=${limit}`),

  getOpportunity: (id: string): Promise<Opportunity> =>
    fetchAPI(`/opportunities/${id}`),

  updateOpportunity: (id: string, data: Partial<Opportunity>): Promise<{ status: string }> =>
    fetchAPI(`/opportunities/${id}`, { method: 'PUT', body: JSON.stringify(data) }),

  deleteOpportunity: (id: string): Promise<{ status: string }> =>
    fetchAPI(`/opportunities/${id}`, { method: 'DELETE' }),

  // «Guardar» un fondo (el endpoint conserva el nombre review). pdf: kept | remote | none.
  reviewOpportunity: (id: string): Promise<{ status: string; pdf?: 'kept' | 'remote' | 'none' }> =>
    fetchAPI(`/opportunities/${id}/review`, { method: 'POST' }),

  unreviewOpportunity: (id: string): Promise<{ status: string }> =>
    fetchAPI(`/opportunities/${id}/unreview`, { method: 'POST' }),

  keepOpportunityDocument: (id: string): Promise<{ status: string }> =>
    fetchAPI(`/opportunities/${id}/document/keep`, { method: 'POST' }),

  unkeepOpportunityDocument: (id: string): Promise<{ status: string }> =>
    fetchAPI(`/opportunities/${id}/document/unkeep`, { method: 'POST' }),

  getKeptDocuments: (): Promise<KeptDocumentsResponse> =>
    fetchAPI('/documents/kept'),

  unkeepDocument: (id: string): Promise<{ status: string }> =>
    fetchAPI(`/documents/${id}/unkeep`, { method: 'POST' }),

  refreshOpportunityStatuses: (): Promise<{ status: string; updated: number }> =>
    fetchAPI('/opportunities/refresh-statuses', { method: 'POST' })
};
