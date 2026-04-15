-- 000007_risk_analyses_document_summary.up.sql
-- Add document-level summary columns written by signsafe-ai after analysis completion.
-- These columns are populated by update_risk_analysis_summary() in the AI worker.

ALTER TABLE risk_analyses
    ADD COLUMN IF NOT EXISTS document_summary TEXT,
    ADD COLUMN IF NOT EXISTS overall_risk      VARCHAR(10),
    ADD COLUMN IF NOT EXISTS key_issues        JSONB NOT NULL DEFAULT '[]';
