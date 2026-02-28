import { useState, useEffect, useCallback } from 'react';
import { api, type Note } from '../lib/api';
import { timeAgo } from '../lib/utils';

interface NotesPanelProps {
  articleId: string;
}

export default function NotesPanel({ articleId }: NotesPanelProps) {
  const [notes, setNotes] = useState<Note[]>([]);
  const [loading, setLoading] = useState(true);
  const [newNote, setNewNote] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  const fetchNotes = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.getNotes(articleId);
      setNotes(data.notes || []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [articleId]);

  useEffect(() => {
    fetchNotes();
  }, [fetchNotes]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newNote.trim() || submitting) return;

    setSubmitting(true);
    setError('');
    try {
      const note = await api.createNote(articleId, newNote.trim());
      setNotes((prev) => [note, ...prev]);
      setNewNote('');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (noteId: string) => {
    try {
      await api.deleteNote(noteId);
      setNotes((prev) => prev.filter((n) => n.id !== noteId));
    } catch (err: any) {
      setError(err.message);
    }
  };

  return (
    <div className="space-y-3">
      <h4 className="text-xs font-medium uppercase tracking-wider text-zinc-400 dark:text-zinc-500">
        Notes ({notes.length})
      </h4>

      {/* Add note form */}
      <form onSubmit={handleSubmit} className="flex gap-2">
        <input
          type="text"
          value={newNote}
          onChange={(e) => setNewNote(e.target.value)}
          placeholder="Add a note..."
          className="flex-1 px-3 py-2 text-sm bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-700 rounded-lg text-zinc-900 dark:text-zinc-100 placeholder-zinc-400 dark:placeholder-zinc-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent transition-colors"
          disabled={submitting}
        />
        <button
          type="submit"
          disabled={submitting || !newNote.trim()}
          className="px-3 py-2 text-sm font-medium text-white bg-indigo-500 hover:bg-indigo-600 disabled:opacity-50 disabled:cursor-not-allowed rounded-lg transition-colors"
        >
          {submitting ? '...' : 'Add'}
        </button>
      </form>

      {error && (
        <p className="text-xs text-red-500">{error}</p>
      )}

      {/* Notes list */}
      {loading ? (
        <div className="space-y-2">
          <div className="h-8 bg-zinc-200 dark:bg-zinc-800 rounded animate-pulse" />
          <div className="h-8 bg-zinc-200 dark:bg-zinc-800 rounded animate-pulse" />
        </div>
      ) : notes.length === 0 ? (
        <p className="text-xs text-zinc-400 dark:text-zinc-500 italic">
          No notes yet.
        </p>
      ) : (
        <div className="space-y-2">
          {notes.map((note) => (
            <div
              key={note.id}
              className="group flex items-start gap-2 p-2.5 rounded-lg bg-zinc-50 dark:bg-zinc-800/50 border border-zinc-100 dark:border-zinc-800"
            >
              <p className="flex-1 text-sm text-zinc-700 dark:text-zinc-300 leading-relaxed">
                {note.content}
              </p>
              <div className="flex items-center gap-2 shrink-0">
                <span className="text-[10px] text-zinc-400 dark:text-zinc-500">
                  {timeAgo(note.created_at)}
                </span>
                <button
                  onClick={() => handleDelete(note.id)}
                  className="opacity-0 group-hover:opacity-100 p-1 text-zinc-400 hover:text-red-500 transition-all"
                  title="Delete note"
                >
                  <svg
                    className="w-3 h-3"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M6 18L18 6M6 6l12 12"
                    />
                  </svg>
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
