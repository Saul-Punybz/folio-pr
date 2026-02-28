import { useState, useEffect, useRef, useCallback } from 'react';
import { api, type ChatMessage, type ChatSession } from '../lib/api';

interface ChatSource {
  title: string;
  source: string;
  url: string;
}

interface WebChatSource {
  title: string;
  source: string;
  url: string;
  snippet?: string;
  savable?: boolean;
}

interface LocalChatMessage {
  role: 'user' | 'assistant';
  content: string;
  sources?: ChatSource[];
  webSources?: WebChatSource[];
}

const QUICK_QUESTIONS = [
  { icon: '💰', label: 'Fondos federales', q: '¿Qué fondos federales o grants nuevos hay disponibles?' },
  { icon: '🚪', label: 'Renuncias', q: '¿Algún funcionario ha renunciado o dejado su puesto?' },
  { icon: '👔', label: 'Directores de agencias', q: '¿Cambios en directores de agencias del gobierno?' },
  { icon: '🏗️', label: 'FEMA / Reconstrucción', q: '¿Noticias sobre FEMA o fondos de reconstrucción?' },
  { icon: '⚠️', label: 'Escándalos', q: '¿Errores o escándalos de la administración pública?' },
  { icon: '📜', label: 'Legislación', q: '¿Nuevas leyes o legislación aprobada?' },
  { icon: '🤝', label: 'Fondos para ONGs', q: '¿Fondos o subvenciones para ONGs o comunidades?' },
  { icon: '🏥', label: 'Salud pública', q: '¿Noticias de salud pública o programas sociales?' },
  { icon: '🎓', label: 'Educación', q: '¿Cambios en política educativa o fondos escolares?' },
  { icon: '📋', label: 'Contratos', q: '¿Contratos gubernamentales o licitaciones nuevas?' },
  { icon: '⚡', label: 'Infraestructura', q: '¿Noticias sobre infraestructura o energía?' },
  { icon: '🔍', label: 'Investigaciones', q: '¿Qué políticos están bajo investigación o en problemas?' },
];

export default function DailyBrief() {
  const [chatMessages, setChatMessages] = useState<LocalChatMessage[]>([]);
  const [chatInput, setChatInput] = useState('');
  const [chatLoading, setChatLoading] = useState(false);
  const [savedUrls, setSavedUrls] = useState<Set<string>>(new Set());
  const [savingUrls, setSavingUrls] = useState<Set<string>>(new Set());
  const chatEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Chat session state
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [currentSessionId, setCurrentSessionId] = useState<string | null>(null);
  const [sessionsLoading, setSessionsLoading] = useState(true);

  // Load sessions on mount
  useEffect(() => {
    loadSessions();
  }, []);

  const loadSessions = async () => {
    try {
      const data = await api.getChatSessions();
      setSessions(data.sessions || []);
    } catch {
      // Silently fail
    } finally {
      setSessionsLoading(false);
    }
  };

  const autoSave = useCallback(async (messages: LocalChatMessage[], sessionId: string | null) => {
    if (messages.length === 0) return sessionId;

    // Generate title from first user message
    const firstUserMsg = messages.find(m => m.role === 'user');
    const title = firstUserMsg
      ? firstUserMsg.content.slice(0, 50) + (firstUserMsg.content.length > 50 ? '...' : '')
      : 'Sin título';

    try {
      if (sessionId) {
        // Update existing session
        await api.updateChatSession(sessionId, title, messages as ChatMessage[]);
        setSessions(prev => prev.map(s =>
          s.id === sessionId ? { ...s, title, messages: messages as ChatMessage[], updated_at: new Date().toISOString() } : s
        ));
        return sessionId;
      } else {
        // Create new session
        const session = await api.createChatSession(title, messages as ChatMessage[]);
        setSessions(prev => [session, ...prev]);
        return session.id;
      }
    } catch {
      return sessionId;
    }
  }, []);

  const handleSaveWebSource = async (url: string, title: string, snippet?: string) => {
    setSavingUrls((prev) => new Set(prev).add(url));
    try {
      const article = await api.collectItem(url, title, undefined, snippet);
      if (article?.id) {
        await api.saveItem(article.id);
      }
      setSavedUrls((prev) => new Set(prev).add(url));
    } catch {
      // Silently fail
    } finally {
      setSavingUrls((prev) => {
        const next = new Set(prev);
        next.delete(url);
        return next;
      });
    }
  };

  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [chatMessages]);

  const sendQuestion = async (question: string) => {
    if (!question.trim() || chatLoading) return;
    setChatInput('');
    const userMsg: LocalChatMessage = { role: 'user', content: question };
    const updatedMessages = [...chatMessages, userMsg];
    setChatMessages(updatedMessages);
    setChatLoading(true);
    try {
      const data = await api.chatWithNews(question);
      const assistantMsg: LocalChatMessage = {
        role: 'assistant',
        content: data.answer,
        sources: data.sources,
        webSources: data.web_sources,
      };
      const allMessages = [...updatedMessages, assistantMsg];
      setChatMessages(allMessages);

      // Auto-save after AI response
      const newId = await autoSave(allMessages, currentSessionId);
      if (newId && newId !== currentSessionId) {
        setCurrentSessionId(newId);
      }
    } catch {
      setChatMessages((prev) => [...prev, { role: 'assistant', content: 'Error al obtener respuesta. Verifica que Ollama esté corriendo.' }]);
    } finally {
      setChatLoading(false);
      inputRef.current?.focus();
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    sendQuestion(chatInput);
  };

  const loadSession = async (session: ChatSession) => {
    try {
      const full = await api.getChatSession(session.id);
      setChatMessages(full.messages as LocalChatMessage[]);
      setCurrentSessionId(full.id);
    } catch {
      // Silently fail
    }
  };

  const deleteSession = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await api.deleteChatSession(id);
      setSessions(prev => prev.filter(s => s.id !== id));
      if (currentSessionId === id) {
        setChatMessages([]);
        setCurrentSessionId(null);
      }
    } catch {
      // Silently fail
    }
  };

  const startNewChat = () => {
    setChatMessages([]);
    setCurrentSessionId(null);
    setSavedUrls(new Set());
    inputRef.current?.focus();
  };

  const formatDate = (dateStr: string) => {
    const d = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - d.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    if (diffMins < 1) return 'ahora';
    if (diffMins < 60) return `hace ${diffMins}m`;
    const diffHrs = Math.floor(diffMins / 60);
    if (diffHrs < 24) return `hace ${diffHrs}h`;
    const diffDays = Math.floor(diffHrs / 24);
    if (diffDays < 7) return `hace ${diffDays}d`;
    return d.toLocaleDateString('es-PR', { month: 'short', day: 'numeric' });
  };

  return (
    <div className="flex flex-col lg:flex-row gap-4 h-[calc(100vh-8rem)]">
      {/* Left: Quick Questions + History Panel */}
      <div className="lg:w-72 shrink-0 rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 overflow-hidden flex flex-col">
        <div className="px-4 py-3 border-b border-zinc-200 dark:border-zinc-800 bg-zinc-900 dark:bg-zinc-950">
          <h3 className="text-xs font-bold text-white uppercase tracking-wider">Preguntas Rápidas</h3>
        </div>
        <div className="overflow-y-auto p-2 space-y-1 lg:block flex flex-wrap gap-1 lg:space-y-1" style={{ maxHeight: '40%' }}>
          {QUICK_QUESTIONS.map((item) => (
            <button
              key={item.label}
              onClick={() => sendQuestion(item.q)}
              disabled={chatLoading}
              className="w-full text-left px-3 py-2.5 text-xs rounded-lg transition-all duration-150 flex items-start gap-2.5 hover:bg-indigo-50 dark:hover:bg-indigo-500/10 hover:text-indigo-700 dark:hover:text-indigo-300 text-zinc-600 dark:text-zinc-400 disabled:opacity-40 disabled:cursor-not-allowed active:scale-[0.98] group"
            >
              <span className="text-sm shrink-0 mt-px">{item.icon}</span>
              <div>
                <span className="font-semibold text-zinc-800 dark:text-zinc-200 group-hover:text-indigo-700 dark:group-hover:text-indigo-300 block leading-tight">{item.label}</span>
                <span className="text-[11px] text-zinc-400 dark:text-zinc-500 leading-tight hidden lg:block">{item.q}</span>
              </div>
            </button>
          ))}
        </div>

        {/* History Section */}
        <div className="border-t border-zinc-200 dark:border-zinc-800 flex flex-col flex-1 min-h-0">
          <div className="px-4 py-2.5 bg-zinc-900 dark:bg-zinc-950 flex items-center justify-between">
            <h3 className="text-xs font-bold text-white uppercase tracking-wider">Historial</h3>
            <button
              onClick={startNewChat}
              className="text-[11px] font-semibold text-indigo-400 hover:text-indigo-300 transition-colors"
            >
              + Nueva
            </button>
          </div>
          <div className="flex-1 overflow-y-auto p-2 space-y-1">
            {sessionsLoading ? (
              <p className="text-[11px] text-zinc-500 text-center py-4">Cargando...</p>
            ) : sessions.length === 0 ? (
              <p className="text-[11px] text-zinc-500 text-center py-4">Sin conversaciones guardadas</p>
            ) : (
              sessions.map((session) => (
                <div
                  key={session.id}
                  onClick={() => loadSession(session)}
                  className={`w-full text-left px-3 py-2.5 text-xs rounded-lg transition-all duration-150 cursor-pointer group flex items-center gap-2 ${
                    currentSessionId === session.id
                      ? 'bg-indigo-50 dark:bg-indigo-500/15 text-indigo-700 dark:text-indigo-300'
                      : 'hover:bg-zinc-50 dark:hover:bg-zinc-800 text-zinc-600 dark:text-zinc-400'
                  }`}
                >
                  <svg className="w-3.5 h-3.5 shrink-0 text-zinc-400 dark:text-zinc-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 8.25h9m-9 3H12m-9.75 1.51c0 1.6 1.123 2.994 2.707 3.227 1.129.166 2.27.293 3.423.379.35.026.67.21.865.501L12 21l2.755-4.133a1.14 1.14 0 01.865-.501 48.172 48.172 0 003.423-.379c1.584-.233 2.707-1.626 2.707-3.228V6.741c0-1.602-1.123-2.995-2.707-3.228A48.394 48.394 0 0012 3c-2.392 0-4.744.175-7.043.513C3.373 3.746 2.25 5.14 2.25 6.741v6.018z" />
                  </svg>
                  <div className="flex-1 min-w-0">
                    <p className="font-medium text-zinc-800 dark:text-zinc-200 truncate leading-tight">{session.title || 'Sin título'}</p>
                    <p className="text-[10px] text-zinc-400 dark:text-zinc-500 mt-0.5">{formatDate(session.updated_at)}</p>
                  </div>
                  <button
                    onClick={(e) => deleteSession(session.id, e)}
                    className="opacity-0 group-hover:opacity-100 shrink-0 w-5 h-5 flex items-center justify-center rounded text-zinc-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10 transition-all"
                    title="Eliminar"
                  >
                    <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
              ))
            )}
          </div>
        </div>
      </div>

      {/* Right: Chat Area */}
      <div className="flex-1 rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 overflow-hidden flex flex-col min-h-0">
        {/* Chat Header */}
        <div className="px-5 py-3 border-b border-zinc-200 dark:border-zinc-800 bg-zinc-900 dark:bg-zinc-950 flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-indigo-500 flex items-center justify-center shrink-0">
            <svg className="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09z" />
            </svg>
          </div>
          <div>
            <h2 className="text-sm font-bold text-white tracking-tight">Inteligencia de Noticias</h2>
            <p className="text-[11px] text-zinc-400">Pregunta sobre noticias recopiladas de Puerto Rico</p>
          </div>
        </div>

        {/* Messages */}
        <div className="flex-1 overflow-y-auto p-4 space-y-4 min-h-0">
          {chatMessages.length === 0 && (
            <div className="flex flex-col items-center justify-center h-full text-center px-8">
              <div className="w-16 h-16 rounded-2xl bg-zinc-100 dark:bg-zinc-800 flex items-center justify-center mb-4">
                <svg className="w-8 h-8 text-zinc-300 dark:text-zinc-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 8.25h9m-9 3H12m-9.75 1.51c0 1.6 1.123 2.994 2.707 3.227 1.129.166 2.27.293 3.423.379.35.026.67.21.865.501L12 21l2.755-4.133a1.14 1.14 0 01.865-.501 48.172 48.172 0 003.423-.379c1.584-.233 2.707-1.626 2.707-3.228V6.741c0-1.602-1.123-2.995-2.707-3.228A48.394 48.394 0 0012 3c-2.392 0-4.744.175-7.043.513C3.373 3.746 2.25 5.14 2.25 6.741v6.018z" />
                </svg>
              </div>
              <h3 className="text-base font-semibold text-zinc-700 dark:text-zinc-300 mb-1">
                Analista de Noticias AI
              </h3>
              <p className="text-sm text-zinc-400 dark:text-zinc-500 max-w-sm">
                Selecciona una pregunta rápida o escribe tu propia pregunta sobre las noticias de Puerto Rico.
              </p>
            </div>
          )}

          {chatMessages.map((msg, i) => (
            <div
              key={i}
              className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}
            >
              <div className={`max-w-[80%] ${msg.role === 'user' ? '' : 'flex gap-2.5'}`}>
                {msg.role === 'assistant' && (
                  <div className="w-7 h-7 rounded-lg bg-indigo-500 flex items-center justify-center shrink-0 mt-0.5">
                    <svg className="w-3.5 h-3.5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09z" />
                    </svg>
                  </div>
                )}
                <div
                  className={`px-4 py-3 rounded-2xl text-sm leading-relaxed ${
                    msg.role === 'user'
                      ? 'bg-indigo-500 text-white rounded-br-md'
                      : 'bg-zinc-50 dark:bg-zinc-800/80 text-zinc-700 dark:text-zinc-300 rounded-bl-md border border-zinc-100 dark:border-zinc-700/50'
                  }`}
                >
                  {msg.content.split('\n').map((line, j) => (
                    <span key={j}>
                      {line}
                      {j < msg.content.split('\n').length - 1 && <br />}
                    </span>
                  ))}
                  {msg.sources && msg.sources.length > 0 && (
                    <div className="mt-3 pt-3 border-t border-zinc-200 dark:border-zinc-600/50 space-y-1.5">
                      <p className="text-[11px] font-bold uppercase tracking-wider text-zinc-400 dark:text-zinc-500">Fuentes Locales</p>
                      {msg.sources.map((s, j) => (
                        <a
                          key={j}
                          href={s.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="flex items-start gap-2 text-xs group"
                          title={s.title}
                        >
                          <svg className="w-3 h-3 text-indigo-400 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M13.5 6H5.25A2.25 2.25 0 003 8.25v10.5A2.25 2.25 0 005.25 21h10.5A2.25 2.25 0 0018 18.75V10.5m-10.5 6L21 3m0 0h-5.25M21 3v5.25" />
                          </svg>
                          <span className="text-indigo-500 dark:text-indigo-400 group-hover:text-indigo-600 dark:group-hover:text-indigo-300 group-hover:underline truncate">
                            <span className="font-medium">{s.source}</span>: {s.title}
                          </span>
                        </a>
                      ))}
                    </div>
                  )}
                  {msg.webSources && msg.webSources.length > 0 && (
                    <div className="mt-2 pt-2 border-t border-amber-200 dark:border-amber-600/30 space-y-1.5">
                      <p className="text-[11px] font-bold uppercase tracking-wider text-amber-500 dark:text-amber-400">Internet</p>
                      {msg.webSources.map((s, j) => (
                        <div key={`web-${j}`} className="flex items-start gap-2 text-xs group">
                          <svg className="w-3 h-3 text-amber-400 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418" />
                          </svg>
                          <div className="flex-1 min-w-0">
                            <a
                              href={s.url}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-amber-600 dark:text-amber-400 hover:text-amber-700 dark:hover:text-amber-300 hover:underline truncate block"
                              title={s.title}
                            >
                              {s.title}
                            </a>
                            {s.snippet && (
                              <p className="text-[11px] text-zinc-400 dark:text-zinc-500 mt-0.5 line-clamp-2">{s.snippet}</p>
                            )}
                          </div>
                          {savedUrls.has(s.url) ? (
                            <span className="shrink-0 px-2.5 py-1 text-[11px] font-semibold rounded-lg bg-emerald-100 dark:bg-emerald-500/20 text-emerald-700 dark:text-emerald-400 flex items-center gap-1">
                              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                              </svg>
                              Guardado
                            </span>
                          ) : (
                            <button
                              onClick={(e) => { e.preventDefault(); handleSaveWebSource(s.url, s.title, s.snippet); }}
                              disabled={savingUrls.has(s.url)}
                              className="shrink-0 px-2.5 py-1 text-[11px] font-semibold rounded-lg bg-amber-100 dark:bg-amber-500/20 border border-amber-400 dark:border-amber-500 text-amber-700 dark:text-amber-300 hover:bg-amber-200 dark:hover:bg-amber-500/30 transition-colors disabled:opacity-50 flex items-center gap-1"
                              title="Guardar en Inbox"
                            >
                              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                <path strokeLinecap="round" strokeLinejoin="round" d="M17.593 3.322c1.1.128 1.907 1.077 1.907 2.185V21L12 17.25 4.5 21V5.507c0-1.108.806-2.057 1.907-2.185a48.507 48.507 0 0111.186 0z" />
                              </svg>
                              {savingUrls.has(s.url) ? 'Guardando...' : 'Guardar'}
                            </button>
                          )}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </div>
          ))}

          {chatLoading && (
            <div className="flex justify-start">
              <div className="flex gap-2.5">
                <div className="w-7 h-7 rounded-lg bg-indigo-500 flex items-center justify-center shrink-0">
                  <svg className="w-3.5 h-3.5 text-white animate-pulse" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09z" />
                  </svg>
                </div>
                <div className="px-4 py-3 rounded-2xl rounded-bl-md bg-zinc-50 dark:bg-zinc-800/80 border border-zinc-100 dark:border-zinc-700/50">
                  <div className="flex items-center gap-1.5">
                    <div className="w-1.5 h-1.5 bg-indigo-400 rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
                    <div className="w-1.5 h-1.5 bg-indigo-400 rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
                    <div className="w-1.5 h-1.5 bg-indigo-400 rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
                    <span className="text-xs text-zinc-400 ml-2">Analizando noticias...</span>
                  </div>
                </div>
              </div>
            </div>
          )}

          <div ref={chatEndRef} />
        </div>

        {/* Input */}
        <form onSubmit={handleSubmit} className="border-t border-zinc-200 dark:border-zinc-800 p-3 flex gap-2 bg-zinc-50/50 dark:bg-zinc-800/30">
          <input
            ref={inputRef}
            type="text"
            value={chatInput}
            onChange={(e) => setChatInput(e.target.value)}
            placeholder="Escribe tu pregunta sobre noticias de Puerto Rico..."
            disabled={chatLoading}
            className="flex-1 px-4 py-3 text-sm bg-white dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700 rounded-xl text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 focus:outline-none focus:ring-2 focus:ring-indigo-500/50 focus:border-indigo-500 transition-all disabled:opacity-50"
          />
          <button
            type="submit"
            disabled={chatLoading || !chatInput.trim()}
            className="shrink-0 w-11 h-11 flex items-center justify-center text-white bg-indigo-500 hover:bg-indigo-600 disabled:opacity-40 disabled:cursor-not-allowed rounded-xl transition-all active:scale-95"
          >
            {chatLoading ? (
              <svg className="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
              </svg>
            ) : (
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M6 12L3.269 3.126A59.768 59.768 0 0121.485 12 59.77 59.77 0 013.27 20.876L5.999 12zm0 0h7.5" />
              </svg>
            )}
          </button>
        </form>
      </div>
    </div>
  );
}
