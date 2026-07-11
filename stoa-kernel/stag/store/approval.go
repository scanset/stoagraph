package store

// file-kw: approval queue escalate pending signed-release token one-time consume fail-closed sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// kw: approval record escalated action awaiting/holding a signed release
type Approval struct {
	ID          string
	Tool        string
	Fingerprint string
	ArgsJSON    string
	Recipe      string
	RecipeHash  string
	Status      string // pending | approved | denied | consumed
	Token       string
	Reason      string
	CreatedAt   string
	DecidedAt   string
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// RecordPending logs an escalated action as pending approval, idempotently. Re-escalating the
// same action (same id) is a no-op while pending/approved/denied; a CONSUMED row is reset to
// pending (a new occurrence of a one-time-approved action needs fresh approval). Returns whether
// a row was created/reset (true => a NEW approval needs a human; the caller fires the webhook).
// This satisfies proxy.Approvals — primitive signatures, no shared struct across the boundary.
func (s *Store) RecordPending(ctx context.Context, id, tool, fingerprint, argsJSON, recipe, recipeHash string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM approval WHERE id=?`, id).Scan(&status)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO approval (id,tool,fingerprint,args_json,recipe,recipe_hash,status,created_at)
			 VALUES (?,?,?,?,?,?, 'pending', ?)`,
			id, tool, fingerprint, argsJSON, recipe, recipeHash, nowRFC3339()); err != nil {
			return false, err
		}
		return true, tx.Commit()
	case err != nil:
		return false, err
	case status == "consumed":
		// a one-time approval was already spent; a new identical request must be re-approved.
		if _, err = tx.ExecContext(ctx,
			`UPDATE approval SET status='pending', token='', reason='', decided_at='', args_json=?, created_at=? WHERE id=?`,
			argsJSON, nowRFC3339(), id); err != nil {
			return false, err
		}
		return true, tx.Commit()
	default:
		// pending | approved | denied — leave as-is (idempotent), no new notification.
		return false, tx.Commit()
	}
}

// LookupApproved returns the signed release token for an action fingerprint IFF a row for it is
// currently `approved` (not pending/denied/consumed). ok=false otherwise. Satisfies proxy.Approvals.
func (s *Store) LookupApproved(ctx context.Context, fingerprint string) (string, string, bool, error) {
	var id, token string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, token FROM approval WHERE fingerprint=? AND status='approved'`, fingerprint).Scan(&id, &token)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return token, id, token != "", nil
}

// Consume marks an approved release spent (one-time). Only an `approved` row transitions to
// `consumed`; anything else is left untouched. Satisfies proxy.Approvals.
func (s *Store) Consume(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE approval SET status='consumed', decided_at=? WHERE id=? AND status='approved'`,
		nowRFC3339(), id)
	return err
}

// ApproveApproval mints the decision: sets the signed release token and flips pending -> approved.
// No-op unless the row is currently pending (can't approve a denied/consumed action).
func (s *Store) ApproveApproval(ctx context.Context, id, token, reason string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE approval SET status='approved', token=?, reason=?, decided_at=? WHERE id=? AND status='pending'`,
		token, reason, nowRFC3339(), id)
	return err
}

// DenyApproval rejects a pending action; a denial sticks until a human clears it.
func (s *Store) DenyApproval(ctx context.Context, id, reason string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE approval SET status='denied', reason=?, decided_at=? WHERE id=? AND status='pending'`,
		reason, nowRFC3339(), id)
	return err
}

// GetApproval fetches one row (for the approve/deny handlers, which need the fingerprint to sign).
func (s *Store) GetApproval(ctx context.Context, id string) (Approval, error) {
	return s.scanOne(s.db.QueryRowContext(ctx, approvalCols+` WHERE id=?`, id))
}

// ListApprovals returns rows filtered by status ("" => all), newest first (for the dashboard).
func (s *Store) ListApprovals(ctx context.Context, status string) ([]Approval, error) {
	q := approvalCols + ` ORDER BY created_at DESC, id`
	args := []any{}
	if status != "" {
		q = approvalCols + ` WHERE status=? ORDER BY created_at DESC, id`
		args = append(args, status)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Approval
	for rows.Next() {
		a, err := s.scanRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

const approvalCols = `SELECT id,tool,fingerprint,args_json,recipe,recipe_hash,status,token,reason,created_at,decided_at FROM approval`

type scanner interface{ Scan(dest ...any) error }

func (s *Store) scanOne(row scanner) (Approval, error)     { return s.scanInto(row) }
func (s *Store) scanRows(rows *sql.Rows) (Approval, error) { return s.scanInto(rows) }

func (s *Store) scanInto(row scanner) (Approval, error) {
	var a Approval
	err := row.Scan(&a.ID, &a.Tool, &a.Fingerprint, &a.ArgsJSON, &a.Recipe, &a.RecipeHash,
		&a.Status, &a.Token, &a.Reason, &a.CreatedAt, &a.DecidedAt)
	return a, err
}
