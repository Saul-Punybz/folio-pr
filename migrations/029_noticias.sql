-- 029: noticias. «Borrar» de verdad y archivo automático.
--  · status 'archived': lo del Inbox con más de 7 días pasa solo a Archivo.
--  · trashed_at / prev_status: lo borrado se oculta al instante; una tarea
--    elimina la fila pasada 1 hora (el «Deshacer» dura 8 s) y deja su URL y
--    su huella de contenido bloqueadas en fingerprints.
ALTER TABLE articles DROP CONSTRAINT IF EXISTS articles_status_check;
ALTER TABLE articles ADD CONSTRAINT articles_status_check
    CHECK (status IN ('inbox', 'saved', 'archived', 'trashed'));

ALTER TABLE articles ADD COLUMN IF NOT EXISTS trashed_at TIMESTAMPTZ;
ALTER TABLE articles ADD COLUMN IF NOT EXISTS prev_status TEXT;

-- Lo que ya estaba descartado se borra en la próxima purga.
UPDATE articles SET trashed_at = now() - interval '2 hours'
 WHERE status = 'trashed' AND trashed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_articles_trashed ON articles(trashed_at) WHERE status = 'trashed';
CREATE INDEX IF NOT EXISTS idx_articles_status_created ON articles(status, created_at);
CREATE INDEX IF NOT EXISTS idx_fingerprints_content_blocked ON fingerprints(content_hash) WHERE blocked = true;
