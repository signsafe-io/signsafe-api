-- Composite index for LATERAL JOIN in ListContracts:
--   WHERE contract_id = ? AND status = 'completed' ORDER BY created_at DESC LIMIT 1
-- Without this, Postgres uses idx_risk_analyses_contract_id and scans all rows per contract.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_risk_analyses_contract_status_created
    ON risk_analyses (contract_id, status, created_at DESC);
