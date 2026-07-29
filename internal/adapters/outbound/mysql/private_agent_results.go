package mysql

import (
	"argus/internal/models"
	"context"
	"crypto/sha256"
)

func (r *Store) RecordPrivateAgentResult(ctx context.Context, result models.PrivateAgentResult, key string) (bool, error) {
	hash := sha256.Sum256([]byte(key))
	res, err := r.db.ExecContext(ctx, `INSERT IGNORE INTO private_agent_results (agent_id,project_id,environment_id,idempotency_key_hash,outcome,summary,received_at) VALUES (?,?,?,?,?,?,?)`, result.AgentID, result.ProjectID, result.EnvironmentID, hash[:], result.Outcome, nullableString(result.Summary), result.ReceivedAt)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
