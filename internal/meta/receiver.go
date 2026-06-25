package meta

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Receiver is a named notification channel: an alarm (currently a webhook POST)
// fired when a package is quarantined pending approval. Name is the unique,
// human-facing channel identifier; Description documents its purpose.
type Receiver struct {
	ID          int64
	Name        string
	Description string
	WebhookURL  string
	Enabled     bool
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ErrConflict is returned when a uniqueness constraint (e.g. receiver name)
// would be violated.
var ErrConflict = errors.New("conflict")

const receiverCols = `id, name, description, webhook_url, enabled, created_by, created_at, updated_at`

// CreateReceiver inserts a notification receiver. Returns ErrConflict when the
// name is already taken.
func (s *Store) CreateReceiver(ctx context.Context, r Receiver) (Receiver, error) {
	now := nowRFC3339()
	row := s.h().QueryRowContext(ctx,
		`INSERT INTO notification_receivers(name, description, webhook_url, enabled, created_by, created_at, updated_at)
         VALUES(?, ?, ?, ?, ?, ?, ?)
         RETURNING `+receiverCols,
		r.Name, r.Description, r.WebhookURL, boolToInt(r.Enabled), r.CreatedBy, now, now)
	rec, err := scanReceiver(row)
	if isUniqueViolation(err) {
		return Receiver{}, ErrConflict
	}
	return rec, wrap("create receiver", err)
}

// GetReceiver returns one receiver by id.
func (s *Store) GetReceiver(ctx context.Context, id int64) (Receiver, error) {
	row := s.h().QueryRowContext(ctx, `SELECT `+receiverCols+` FROM notification_receivers WHERE id = ?`, id)
	rec, err := scanReceiver(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Receiver{}, ErrNotFound
	}
	return rec, wrap("get receiver", err)
}

// ListReceivers returns all receivers, oldest first (stable display order).
func (s *Store) ListReceivers(ctx context.Context) ([]Receiver, error) {
	return s.queryReceivers(ctx, `SELECT `+receiverCols+` FROM notification_receivers ORDER BY id ASC`)
}

// ListEnabledReceivers returns the enabled receivers (delivery targets).
func (s *Store) ListEnabledReceivers(ctx context.Context) ([]Receiver, error) {
	return s.queryReceivers(ctx, `SELECT `+receiverCols+` FROM notification_receivers WHERE enabled = 1 ORDER BY id ASC`)
}

func (s *Store) queryReceivers(ctx context.Context, q string, args ...any) ([]Receiver, error) {
	rows, err := s.h().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, wrap("list receivers", err)
	}
	defer rows.Close()
	out := []Receiver{}
	for rows.Next() {
		rec, err := scanReceiver(rows)
		if err != nil {
			return nil, wrap("scan receiver", err)
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// UpdateReceiver overwrites a receiver's editable fields. Returns ErrNotFound
// when no row matches and ErrConflict when the new name collides.
func (s *Store) UpdateReceiver(ctx context.Context, r Receiver) (Receiver, error) {
	res, err := s.h().ExecContext(ctx,
		`UPDATE notification_receivers
         SET name = ?, description = ?, webhook_url = ?, enabled = ?, updated_at = ?
         WHERE id = ?`,
		r.Name, r.Description, r.WebhookURL, boolToInt(r.Enabled), nowRFC3339(), r.ID)
	if isUniqueViolation(err) {
		return Receiver{}, ErrConflict
	}
	if err != nil {
		return Receiver{}, wrap("update receiver", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Receiver{}, err
	}
	if n == 0 {
		return Receiver{}, ErrNotFound
	}
	return s.GetReceiver(ctx, r.ID)
}

// DeleteReceiver removes one receiver.
func (s *Store) DeleteReceiver(ctx context.Context, id int64) error {
	res, err := s.h().ExecContext(ctx, `DELETE FROM notification_receivers WHERE id = ?`, id)
	if err != nil {
		return wrap("delete receiver", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanReceiver(row interface{ Scan(...any) error }) (Receiver, error) {
	var r Receiver
	var enabled int
	var created, updated string
	if err := row.Scan(&r.ID, &r.Name, &r.Description, &r.WebhookURL, &enabled, &r.CreatedBy, &created, &updated); err != nil {
		return Receiver{}, err
	}
	r.Enabled = enabled != 0
	r.CreatedAt = parseTime(created)
	r.UpdatedAt = parseTime(updated)
	return r, nil
}

// isUniqueViolation reports whether err is a SQLite UNIQUE constraint failure.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
