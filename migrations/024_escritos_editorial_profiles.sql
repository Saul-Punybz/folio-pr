-- Folio Migration 024: Escritos editorial profiles
-- Adds automatic output preferences for format, style, audience and citations.

ALTER TABLE escritos
    ADD COLUMN IF NOT EXISTS output_format TEXT NOT NULL DEFAULT 'auto'
        CHECK (output_format IN ('auto','article','newsletter','report','memo','letter','email','op_ed')),
    ADD COLUMN IF NOT EXISTS writing_style TEXT NOT NULL DEFAULT 'auto'
        CHECK (writing_style IN ('auto','academic','journalistic','executive','institutional','persuasive','newsletter')),
    ADD COLUMN IF NOT EXISTS target_audience TEXT NOT NULL DEFAULT 'general'
        CHECK (target_audience IN ('general','academic','executive','public_sector','community','donor')),
    ADD COLUMN IF NOT EXISTS citation_style TEXT NOT NULL DEFAULT 'apa'
        CHECK (citation_style IN ('apa','mla','chicago','inline'));
