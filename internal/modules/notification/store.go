package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Yab1/golang-template/internal/platform/refid"
	"github.com/Yab1/golang-template/internal/platform/storage"
)

const templateColumns = `id, reference_id, channel, category, locale, subject, body, format, is_active,
	version, is_visible, metadata, created_at, updated_at, created_by, updated_by, deleted_at, deleted_by`

const preferenceColumns = `id, user_id, channel, category, enabled, destination,
	version, is_visible, metadata, created_at, updated_at, created_by, updated_by, deleted_at, deleted_by`

const deliveryColumns = `id, reference_id, user_id, channel, category, template_id, destination, subject, body, format,
	status, attempts, max_attempts, next_attempt_at, last_error, provider_ref, idempotency_key, read_at, locked_until,
	version, is_visible, metadata, created_at, updated_at, created_by, updated_by, deleted_at, deleted_by`

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Store struct {
	db   *pgxpool.Pool
	refs *refid.Generator
}

func NewStore(db *pgxpool.Pool, refs *refid.Generator) *Store {
	return &Store{db: db, refs: refs}
}

func metaOrEmpty(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`{}`)
	}
	return raw
}

func scanTemplate(sc interface{ Scan(...any) error }, row *Template) error {
	err := sc.Scan(
		&row.ID, &row.ReferenceID, &row.Channel, &row.Category, &row.Locale, &row.Subject, &row.Body, &row.Format, &row.IsActive,
		&row.Version, &row.IsVisible, &row.Metadata, &row.CreatedAt, &row.UpdatedAt, &row.CreatedBy, &row.UpdatedBy, &row.DeletedAt, &row.DeletedBy,
	)
	if err != nil {
		return err
	}
	row.Metadata = metaOrEmpty(row.Metadata)
	return nil
}

func scanPreference(sc interface{ Scan(...any) error }, row *Preference) error {
	err := sc.Scan(
		&row.ID, &row.UserID, &row.Channel, &row.Category, &row.Enabled, &row.Destination,
		&row.Version, &row.IsVisible, &row.Metadata, &row.CreatedAt, &row.UpdatedAt, &row.CreatedBy, &row.UpdatedBy, &row.DeletedAt, &row.DeletedBy,
	)
	if err != nil {
		return err
	}
	row.Metadata = metaOrEmpty(row.Metadata)
	return nil
}

func scanDelivery(sc interface{ Scan(...any) error }, row *Delivery) error {
	err := sc.Scan(
		&row.ID, &row.ReferenceID, &row.UserID, &row.Channel, &row.Category, &row.TemplateID, &row.Destination, &row.Subject, &row.Body, &row.Format,
		&row.Status, &row.Attempts, &row.MaxAttempts, &row.NextAttemptAt, &row.LastError, &row.ProviderRef, &row.IdempotencyKey, &row.ReadAt, &row.LockedUntil,
		&row.Version, &row.IsVisible, &row.Metadata, &row.CreatedAt, &row.UpdatedAt, &row.CreatedBy, &row.UpdatedBy, &row.DeletedAt, &row.DeletedBy,
	)
	if err != nil {
		return err
	}
	row.Metadata = metaOrEmpty(row.Metadata)
	return nil
}

func (s *Store) ActiveTemplate(ctx context.Context, db querier, channel, category string) (*Template, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	var row Template
	err := scanTemplate(db.QueryRow(ctx, `
		SELECT `+templateColumns+`
		FROM notification_templates
		WHERE channel = $1 AND category = $2 AND locale = 'en' AND is_active AND deleted_at IS NULL
	`, channel, category), &row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Store) ListTemplates(ctx context.Context) ([]*Template, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	rows, err := s.db.Query(ctx, `
		SELECT `+templateColumns+`
		FROM notification_templates
		WHERE deleted_at IS NULL
		ORDER BY category, channel
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Template
	for rows.Next() {
		var row Template
		if err := scanTemplate(rows, &row); err != nil {
			return nil, err
		}
		out = append(out, &row)
	}
	return out, rows.Err()
}

func (s *Store) GetTemplate(ctx context.Context, raw string) (*Template, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	q := `SELECT ` + templateColumns + ` FROM notification_templates WHERE deleted_at IS NULL AND `
	var arg any
	if id, err := uuid.Parse(raw); err == nil {
		q += `id = $1`
		arg = id
	} else {
		q += `reference_id = $1`
		arg = raw
	}
	var row Template
	err := scanTemplate(s.db.QueryRow(ctx, q, arg), &row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Store) UpdateTemplate(ctx context.Context, row *Template) error {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	err := scanTemplate(s.db.QueryRow(ctx, `
		UPDATE notification_templates
		SET subject = $2, body = $3, format = $4, is_active = $5, updated_by = $6, version = version + 1, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+templateColumns, row.ID, row.Subject, row.Body, row.Format, row.IsActive, row.UpdatedBy), row)
	if errors.Is(err, pgx.ErrNoRows) {
		return storage.ErrNotFound
	}
	return err
}

func (s *Store) Preference(ctx context.Context, db querier, userID uuid.UUID, channel, category string) (*Preference, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	var row Preference
	err := scanPreference(db.QueryRow(ctx, `
		SELECT `+preferenceColumns+`
		FROM notification_preferences
		WHERE user_id = $1 AND channel = $2 AND category = $3 AND deleted_at IS NULL
	`, userID, channel, category), &row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Store) ListPreferences(ctx context.Context, userID uuid.UUID) ([]*Preference, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	rows, err := s.db.Query(ctx, `
		SELECT `+preferenceColumns+`
		FROM notification_preferences
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY category, channel
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Preference
	for rows.Next() {
		var row Preference
		if err := scanPreference(rows, &row); err != nil {
			return nil, err
		}
		out = append(out, &row)
	}
	return out, rows.Err()
}

func (s *Store) UpsertPreferenceTx(ctx context.Context, tx pgx.Tx, row *Preference) error {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	row.Metadata = metaOrEmpty(row.Metadata)
	err := scanPreference(tx.QueryRow(ctx, `
		INSERT INTO notification_preferences (
			user_id, channel, category, enabled, destination, metadata, created_by, updated_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (user_id, channel, category) WHERE deleted_at IS NULL
		DO UPDATE SET enabled = EXCLUDED.enabled, destination = EXCLUDED.destination,
			updated_by = EXCLUDED.updated_by, version = notification_preferences.version + 1, updated_at = NOW()
		RETURNING `+preferenceColumns,
		row.UserID, row.Channel, row.Category, row.Enabled, row.Destination, row.Metadata, row.CreatedBy, row.UpdatedBy,
	), row)
	return err
}

func (s *Store) ListByIdempotency(ctx context.Context, db querier, key string) ([]*Delivery, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	rows, err := db.Query(ctx, `
		SELECT `+deliveryColumns+`
		FROM notification_deliveries
		WHERE idempotency_key = $1 AND deleted_at IS NULL
		ORDER BY channel
	`, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectDeliveries(rows)
}

func (s *Store) CreateDeliveryTx(ctx context.Context, tx pgx.Tx, row *Delivery) error {
	q := `
		INSERT INTO notification_deliveries (
			user_id, channel, category, template_id, destination, subject, body, format, status,
			max_attempts, last_error, idempotency_key, reference_id, is_visible, metadata, created_by, updated_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		RETURNING ` + deliveryColumns
	row.Metadata = metaOrEmpty(row.Metadata)
	if row.Format == "" {
		row.Format = FormatText
	}
	if row.Status == "" {
		row.Status = StatusPending
	}
	if row.MaxAttempts < 1 {
		row.MaxAttempts = 5
	}
	var lastErr error
	for i := 0; i < refid.MaxTries(); i++ {
		ref, err := s.refs.Next(RefCodeDelivery)
		if err != nil {
			return err
		}
		row.ReferenceID = ref
		ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
		err = scanDelivery(tx.QueryRow(ctx, q,
			row.UserID, row.Channel, row.Category, row.TemplateID, row.Destination, row.Subject, row.Body, row.Format, row.Status,
			row.MaxAttempts, row.LastError, row.IdempotencyKey, row.ReferenceID, row.IsVisible, row.Metadata, row.CreatedBy, row.UpdatedBy,
		), row)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if refid.IsConflict(err) {
			continue
		}
		if isIdempotencyConflict(err) {
			return err
		}
		return storage.MapError(err)
	}
	if refid.IsConflict(lastErr) {
		return refid.ErrExhausted
	}
	return storage.MapError(lastErr)
}

func isIdempotencyConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "notification_deliveries_idempotency_key"
}

func (s *Store) AddAttemptTx(ctx context.Context, tx pgx.Tx, deliveryID uuid.UUID, attempt int, status, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	_, err := tx.Exec(ctx, `
		INSERT INTO notification_attempts (delivery_id, attempt, status, error)
		VALUES ($1, $2, $3, $4)
	`, deliveryID, attempt, status, reason)
	return err
}

func (s *Store) GetDelivery(ctx context.Context, raw string) (*Delivery, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	q := `SELECT ` + deliveryColumns + ` FROM notification_deliveries WHERE deleted_at IS NULL AND `
	var arg any
	if id, err := uuid.Parse(raw); err == nil {
		q += `id = $1`
		arg = id
	} else {
		q += `reference_id = $1`
		arg = raw
	}
	var row Delivery
	err := scanDelivery(s.db.QueryRow(ctx, q, arg), &row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Store) Attempts(ctx context.Context, deliveryID uuid.UUID) ([]Attempt, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	rows, err := s.db.Query(ctx, `
		SELECT id, delivery_id, attempt, status, error, created_at
		FROM notification_attempts
		WHERE delivery_id = $1
		ORDER BY attempt, created_at
	`, deliveryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attempt
	for rows.Next() {
		var row Attempt
		if err := rows.Scan(&row.ID, &row.DeliveryID, &row.Attempt, &row.Status, &row.Error, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

type deliveryFilter struct {
	UserID  *uuid.UUID
	Channel string
	Status  string
	Unread  bool
	Limit   int
	Offset  int
}

type deliveryPage struct {
	Items []*Delivery
	Total int
}

func (s *Store) ListDeliveries(ctx context.Context, f deliveryFilter) (deliveryPage, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	where := []string{"deleted_at IS NULL"}
	args := []any{}
	if f.UserID != nil {
		args = append(args, *f.UserID)
		where = append(where, fmt.Sprintf("user_id = $%d", len(args)))
	}
	if f.Channel != "" {
		args = append(args, f.Channel)
		where = append(where, fmt.Sprintf("channel = $%d", len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.Unread {
		where = append(where, "read_at IS NULL")
	}
	clause := "WHERE " + joinAnd(where)
	var total int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM notification_deliveries `+clause, args...).Scan(&total); err != nil {
		return deliveryPage{}, err
	}
	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	args = append(args, f.Limit, f.Offset)
	rows, err := s.db.Query(ctx, `
		SELECT `+deliveryColumns+`
		FROM notification_deliveries
		`+clause+fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, limitArg, offsetArg), args...)
	if err != nil {
		return deliveryPage{}, err
	}
	defer rows.Close()
	items, err := collectDeliveries(rows)
	if err != nil {
		return deliveryPage{}, err
	}
	return deliveryPage{Items: items, Total: total}, nil
}

func joinAnd(parts []string) string {
	out := parts[0]
	for _, p := range parts[1:] {
		out += " AND " + p
	}
	return out
}

func collectDeliveries(rows pgx.Rows) ([]*Delivery, error) {
	var out []*Delivery
	for rows.Next() {
		var row Delivery
		if err := scanDelivery(rows, &row); err != nil {
			return nil, err
		}
		out = append(out, &row)
	}
	return out, rows.Err()
}

func (s *Store) Claim(ctx context.Context, limit int) ([]*Delivery, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	rows, err := s.db.Query(ctx, `
		WITH due AS (
			SELECT id
			FROM notification_deliveries
			WHERE deleted_at IS NULL
				AND next_attempt_at <= NOW()
				AND (
					(status = 'pending' AND (locked_until IS NULL OR locked_until < NOW()))
					OR (status = 'sending' AND locked_until < NOW())
				)
			ORDER BY next_attempt_at
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE notification_deliveries AS d
		SET status = 'sending',
			attempts = d.attempts + 1,
			locked_until = NOW() + INTERVAL '60 seconds',
			updated_at = NOW()
		FROM due
		WHERE d.id = due.id
		RETURNING `+prefixedDeliveryColumns(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectDeliveries(rows)
}

func prefixedDeliveryColumns() string {
	return `d.id, d.reference_id, d.user_id, d.channel, d.category, d.template_id, d.destination, d.subject, d.body, d.format,
		d.status, d.attempts, d.max_attempts, d.next_attempt_at, d.last_error, d.provider_ref, d.idempotency_key, d.read_at, d.locked_until,
		d.version, d.is_visible, d.metadata, d.created_at, d.updated_at, d.created_by, d.updated_by, d.deleted_at, d.deleted_by`
}

func (s *Store) Finish(ctx context.Context, id uuid.UUID, status string, next time.Time, lastError, providerRef string) error {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	tag, err := s.db.Exec(ctx, `
		UPDATE notification_deliveries
		SET status = $2, next_attempt_at = $3, last_error = $4,
			provider_ref = CASE WHEN $5 = '' THEN provider_ref ELSE $5 END,
			locked_until = NULL, version = version + 1, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, status, next, lastError, providerRef)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *Store) AddAttempt(ctx context.Context, deliveryID uuid.UUID, attempt int, status, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	_, err := s.db.Exec(ctx, `
		INSERT INTO notification_attempts (delivery_id, attempt, status, error)
		VALUES ($1, $2, $3, $4)
	`, deliveryID, attempt, status, reason)
	return err
}

func (s *Store) MarkRead(ctx context.Context, id, userID uuid.UUID, actor *uuid.UUID) (*Delivery, error) {
	ctx, cancel := context.WithTimeout(ctx, storage.QueryTimeoutDuration)
	defer cancel()
	var row Delivery
	err := scanDelivery(s.db.QueryRow(ctx, `
		UPDATE notification_deliveries
		SET read_at = NOW(), updated_by = $3, version = version + 1, updated_at = NOW()
		WHERE id = $1 AND user_id = $2 AND channel = 'in_app' AND status = 'sent'
			AND deleted_at IS NULL AND read_at IS NULL
		RETURNING `+deliveryColumns, id, userID, actor), &row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
