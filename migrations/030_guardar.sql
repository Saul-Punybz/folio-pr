-- 030: guardar. «Guardar» un fondo conserva su PDF para siempre, y un PDF
-- también se puede guardar suelto. La retención nunca borra un documento kept.
ALTER TABLE documents ADD COLUMN IF NOT EXISTS kept BOOLEAN NOT NULL DEFAULT false;
CREATE INDEX IF NOT EXISTS idx_documents_kept ON documents(created_at) WHERE kept;

-- Los fondos que ya estaban confirmados conservan su PDF desde ahora.
UPDATE documents d SET kept = true
WHERE EXISTS (SELECT 1 FROM opportunities o WHERE o.document_id = d.id AND o.reviewed);
