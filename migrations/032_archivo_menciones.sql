-- Migration 032: Archivo permanente de menciones.
-- Cada mención guarda copia del contenido, metadatos para la cita APA, resumen
-- y enlace de respaldo. Ninguna limpieza las borra: solo la usuaria, una por una.

-- El deduplicado era global por URL: una noticia que nombra dos marcas solo
-- quedaba en la primera. Ahora es por marca.
DROP INDEX IF EXISTS idx_wh_url_hash;
CREATE UNIQUE INDEX IF NOT EXISTS idx_wh_org_url_hash ON watchlist_hits(org_id, url_hash);

ALTER TABLE watchlist_hits
    ADD COLUMN IF NOT EXISTS canonical_url      TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS resolved_url       TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS site_name          TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS authors            JSONB       NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS lang               TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS content_text       TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS image_url          TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS summary            TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS archive_status     TEXT        NOT NULL DEFAULT 'pendiente',
    ADD COLUMN IF NOT EXISTS archive_attempts   INT         NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS archive_error      TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS archived_at        TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS wayback_url        TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS wayback_checked_at TIMESTAMPTZ,
    -- La usuaria corrigió metadatos o resumen: la copia no los pisa.
    ADD COLUMN IF NOT EXISTS edited             BOOLEAN     NOT NULL DEFAULT false;

ALTER TABLE watchlist_hits DROP CONSTRAINT IF EXISTS watchlist_hits_archive_status_check;
ALTER TABLE watchlist_hits ADD CONSTRAINT watchlist_hits_archive_status_check
    CHECK (archive_status IN ('pendiente', 'copia', 'parcial', 'sin_copia'));

CREATE INDEX IF NOT EXISTS idx_wh_archive_pending ON watchlist_hits(archive_status, archive_attempts)
    WHERE archive_status <> 'copia';

-- Candado: una mención solo se borra si quien borra lo declara en su
-- transacción (set_config('folio.borrar_mencion','on',true)). Así ninguna
-- limpieza, purga ni cascada futura se lleva el archivo sin querer.
CREATE OR REPLACE FUNCTION folio_proteger_menciones() RETURNS trigger AS $$
BEGIN
    IF COALESCE(current_setting('folio.borrar_mencion', true), '') <> 'on' THEN
        RAISE EXCEPTION 'las menciones archivadas solo se borran a mano, una por una';
    END IF;
    IF TG_LEVEL = 'ROW' THEN
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_proteger_menciones ON watchlist_hits;
CREATE TRIGGER trg_proteger_menciones BEFORE DELETE ON watchlist_hits
    FOR EACH ROW EXECUTE FUNCTION folio_proteger_menciones();

DROP TRIGGER IF EXISTS trg_proteger_menciones_truncate ON watchlist_hits;
CREATE TRIGGER trg_proteger_menciones_truncate BEFORE TRUNCATE ON watchlist_hits
    FOR EACH STATEMENT EXECUTE FUNCTION folio_proteger_menciones();
