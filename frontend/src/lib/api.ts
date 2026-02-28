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
  total: number;
}

export interface SearchResponse {
  results: Article[];
  total: number;
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

export interface WatchlistOrg {
  id: string;
  user_id: string;
  name: string;
  website: string;
  keywords: string[];
  youtube_channels: string[];
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface WatchlistOrgsResponse {
  orgs: WatchlistOrg[];
  count: number;
}

export interface WatchlistHit {
  id: string;
  org_id: string;
  org_name: string;
  source_type: 'google_news' | 'bing_news' | 'web' | 'local' | 'youtube' | 'reddit';
  title: string;
  url: string;
  url_hash: string;
  snippet: string;
  sentiment: 'positive' | 'neutral' | 'negative' | 'unknown';
  ai_draft: string | null;
  seen: boolean;
  created_at: string;
}

export interface WatchlistHitsResponse {
  hits: WatchlistHit[];
  count: number;
}

export interface UnseenCountResponse {
  unseen: number;
}

export interface ChatSession {
  id: string;
  user_id: string;
  title: string;
  messages: ChatMessage[];
  created_at: string;
  updated_at: string;
}

export interface ChatMessage {
  role: 'user' | 'assistant';
  content: string;
  sources?: { title: string; source: string; url: string }[];
  webSources?: { title: string; source: string; url: string; snippet?: string; savable?: boolean }[];
}

async function fetchAPI<T = any>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
    ...options,
  });

  if (res.status === 401) {
    window.location.href = '/login';
    throw new Error('Unauthorized');
  }

  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }

  // Handle 204 No Content
  if (res.status === 204) {
    return {} as T;
  }

  return res.json();
}

export const api = {
  // Items
  getItems: (status: string, limit = 200, offset = 0): Promise<ItemsResponse> =>
    fetchAPI(`/items?status=${status}&limit=${limit}&offset=${offset}`),

  saveItem: (id: string) =>
    fetchAPI(`/items/${id}/save`, { method: 'POST' }),

  trashItem: (id: string) =>
    fetchAPI(`/items/${id}/trash`, { method: 'POST' }),

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

  // Similarity search
  similar: (id: string, limit = 5): Promise<{ results: Article[]; count: number }> =>
    fetchAPI(`/items/${id}/similar?limit=${limit}`),

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

  // Auth
  login: (email: string, password: string) =>
    fetchAPI('/login', { method: 'POST', body: JSON.stringify({ email, password }) }),

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

  createWatchlistOrg: (data: { name: string; website?: string; keywords: string[]; youtube_channels?: string[] }): Promise<WatchlistOrg> =>
    fetchAPI('/watchlist/orgs', { method: 'POST', body: JSON.stringify(data) }),

  updateWatchlistOrg: (id: string, data: { name: string; website?: string; keywords: string[]; youtube_channels?: string[]; active?: boolean }): Promise<WatchlistOrg> =>
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

  triggerWatchlistScan: (): Promise<{ status: string; message: string }> =>
    fetchAPI('/watchlist/scan', { method: 'POST' }),

  enrichWatchlistOrg: (id: string): Promise<{ status: string; keywords: string[]; message: string }> =>
    fetchAPI(`/watchlist/orgs/${id}/enrich`, { method: 'POST' }),

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

  // Admin: chat with news
  chatWithNews: (question: string): Promise<{ answer: string; articles_used: number; sources?: { title: string; source: string; url: string }[]; web_sources?: { title: string; source: string; url: string; snippet?: string; savable?: boolean }[] }> =>
    fetchAPI('/admin/chat', {
      method: 'POST',
      body: JSON.stringify({ question }),
    }),

  // Admin: re-enrich
  reenrich: (): Promise<{ cleared: number; queued: number; message: string }> =>
    fetchAPI('/admin/reenrich', { method: 'POST' }),

  // Chat sessions
  getChatSessions: (): Promise<{ sessions: ChatSession[] }> =>
    fetchAPI('/chat/sessions'),

  getChatSession: (id: string): Promise<ChatSession> =>
    fetchAPI(`/chat/sessions/${id}`),

  createChatSession: (title: string, messages: ChatMessage[]): Promise<ChatSession> =>
    fetchAPI('/chat/sessions', {
      method: 'POST',
      body: JSON.stringify({ title, messages }),
    }),

  updateChatSession: (id: string, title: string, messages: ChatMessage[]): Promise<ChatSession> =>
    fetchAPI(`/chat/sessions/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ title, messages }),
    }),

  deleteChatSession: (id: string): Promise<void> =>
    fetchAPI(`/chat/sessions/${id}`, { method: 'DELETE' }),
};
