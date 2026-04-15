-- 000007_risk_analyses_document_summary.down.sql

ALTER TABLE risk_analyses
    DROP COLUMN IF EXISTS key_issues,
    DROP COLUMN IF EXISTS overall_risk,
    DROP COLUMN IF EXISTS document_summary;
