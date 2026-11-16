package store

import (
	"context"
	"database/sql"
	"encoding/json"

	"pentagent/internal/domain"
)

// store_jobs.go —— jobs 的插入/读取（§4.1 jobs）。

// dbNullID：空 ID 存 NULL（parent_job_id 自引用外键不接受空串；claimed_by 亦以 NULL 表「未认领」）。
func dbNullID(id domain.ID) any {
	if id == "" {
		return nil
	}
	return string(id)
}

// dbNullJSON：空 RawMessage 存 NULL，否则存文本。
func dbNullJSON(j json.RawMessage) any {
	if len(j) == 0 {
		return nil
	}
	return string(j)
}

func nullJSON(ns sql.NullString) json.RawMessage {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	return json.RawMessage(ns.String)
}

const jobColumns = `id, run_id, COALESCE(parent_job_id,''), kind, COALESCE(title,''),
	spec_json, COALESCE(spec_hash,''), policy_revision, strategy_epoch, state,
	priority, generation, revision, COALESCE(required_capability,''), COALESCE(claimed_by,''),
	lease_until, budget_json, COALESCE(cancel_code,''), COALESCE(cancel_actor,''),
	COALESCE(cancel_detail,''), cancel_requested_at, cancel_settled_at, created_at, updated_at`

// InsertJob 插入一个新 Job（调用方应已用 domain.NewJob 构造：state=PENDING, gen=1, rev=1）。
func (s *Store) InsertJob(ctx context.Context, j *domain.Job) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO jobs (
		id, run_id, parent_job_id, kind, title, spec_json, spec_hash,
		policy_revision, strategy_epoch, state, priority, generation, revision,
		required_capability, claimed_by, lease_until, budget_json,
		cancel_code, cancel_actor, cancel_detail, cancel_requested_at, cancel_settled_at,
		created_at, updated_at
	) VALUES (?,?,?,?,?,?,?, ?,?,?,?,?,?, ?,?,?,?, ?,?,?,?,?, ?,?)`,
		string(j.ID), string(j.RunID), dbNullID(j.ParentJobID), j.Kind, j.Title,
		dbNullJSON(j.SpecJSON), j.SpecHash,
		j.PolicyRevision, j.StrategyEpoch, string(j.State), j.Priority, j.Generation, j.Revision,
		j.RequiredCapability, dbNullID(j.ClaimedBy), dbNullT(j.LeaseUntil), dbNullJSON(j.BudgetJSON),
		string(j.CancelCode), j.CancelActor, j.CancelDetail, dbNullT(j.CancelRequestedAt), dbNullT(j.CancelSettledAt),
		dbT(j.CreatedAt), dbT(j.UpdatedAt),
	)
	return err
}

// GetJob 按 id 读取，未找到返回 domain.ErrNotFound。
func (s *Store) GetJob(ctx context.Context, id domain.ID) (*domain.Job, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM jobs WHERE id=?`, string(id))
	return scanJob(row)
}

func scanJob(sc rowScanner) (*domain.Job, error) {
	var (
		j                                      domain.Job
		id, runID, parentID, state, cancelCode string
		claimedBy                              string
		specJSON, budgetJSON                   sql.NullString
		leaseUntil, cancelReqAt, cancelSetAt   sql.NullString
		createdAt, updatedAt                   string
	)
	err := sc.Scan(
		&id, &runID, &parentID, &j.Kind, &j.Title, &specJSON, &j.SpecHash,
		&j.PolicyRevision, &j.StrategyEpoch, &state, &j.Priority, &j.Generation, &j.Revision,
		&j.RequiredCapability, &claimedBy, &leaseUntil, &budgetJSON,
		&cancelCode, &j.CancelActor, &j.CancelDetail, &cancelReqAt, &cancelSetAt,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	j.ID = domain.ID(id)
	j.RunID = domain.ID(runID)
	j.ParentJobID = domain.ID(parentID)
	j.State = domain.JobState(state)
	j.CancelCode = domain.CancelCode(cancelCode)
	j.ClaimedBy = domain.ID(claimedBy)
	j.SpecJSON = nullJSON(specJSON)
	j.BudgetJSON = nullJSON(budgetJSON)
	j.LeaseUntil = nullT(leaseUntil)
	j.CancelRequestedAt = nullT(cancelReqAt)
	j.CancelSettledAt = nullT(cancelSetAt)
	j.CreatedAt = parseT(createdAt)
	j.UpdatedAt = parseT(updatedAt)
	return &j, nil
}
