import { useState, useEffect, useCallback } from 'react';
import { api, type Note } from '../lib/api';
import { timeAgo } from '../lib/utils';
import { Icon } from './ui';

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
    <section className="space-y-3" aria-label="Notas">
      <h4 className="title-md">Notas ({notes.length})</h4>

      <form onSubmit={handleSubmit} className="flex gap-2">
        <input
          type="text"
          value={newNote}
          onChange={(e) => setNewNote(e.target.value)}
          placeholder="Escribe una nota"
          aria-label="Escribe una nota"
          className="field flex-1"
          disabled={submitting}
        />
        <button type="submit" disabled={submitting || !newNote.trim()} className="btn btn-tonal">
          {submitting ? '...' : 'Agregar'}
        </button>
      </form>

      {error && <p role="alert" className="text-sm font-semibold text-error">{error}</p>}

      {loading ? (
        <div className="space-y-2">
          <div className="h-12 bg-surface-container-high rounded-2xl animate-pulse" />
          <div className="h-12 bg-surface-container-high rounded-2xl animate-pulse" />
        </div>
      ) : notes.length === 0 ? (
        <p className="flex items-center gap-2 text-sm text-on-surface-variant">
          <Icon name="note" className="w-5 h-5" /> Aún no hay notas.
        </p>
      ) : (
        <div className="seg-list">
          {notes.map((note) => (
            <div key={note.id} className="flex items-start gap-2 pl-4 pr-1 py-2">
              <div className="flex-1 min-w-0 py-1.5">
                <p className="text-sm text-on-surface leading-relaxed break-words">{note.content}</p>
                <span className="text-xs font-semibold text-on-surface-variant">{timeAgo(note.created_at)}</span>
              </div>
              <button
                type="button"
                onClick={() => handleDelete(note.id)}
                className="icon-btn shrink-0"
                title="Borrar nota"
                aria-label="Borrar nota"
              >
                <Icon name="close" className="w-5 h-5" />
              </button>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
