-- 000006_clause_results_confidence.up.sql
-- Add confidence score column to clause_results.
-- confidence: 0.0~1.0 float representing the LLM's certainty about the risk assessment.
-- Defaults to 0.5 (neutral) for existing rows and future rows where confidence is not provided.

ALTER TABLE clause_results
    ADD COLUMN IF NOT EXISTS confidence FLOAT NOT NULL DEFAULT 0.5;

COMMENT ON COLUMN clause_results.confidence IS
    'LLM risk-level confidence score (0.0 = uncertain, 1.0 = highly certain). Default 0.5.';
