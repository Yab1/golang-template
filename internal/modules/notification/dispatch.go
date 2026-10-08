package notification

import (
	"context"
	"errors"
	"time"
)

// Run drains pending deliveries until ctx is cancelled. A disabled dispatcher returns immediately.
func (m *Module) Run(ctx context.Context) {
	if !m.cfg.DispatchEnabled {
		return
	}
	ticker := time.NewTicker(m.cfg.PollInterval)
	defer ticker.Stop()
	for {
		if err := m.dispatchOnce(ctx); err != nil && ctx.Err() == nil {
			m.log.Errorw("notification dispatch", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *Module) dispatchOnce(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	rows, err := m.store.Claim(ctx, m.cfg.BatchSize)
	if err != nil || len(rows) == 0 {
		return err
	}
	for _, row := range rows {
		m.deliver(ctx, row)
	}
	return nil
}

func (m *Module) deliver(ctx context.Context, row *Delivery) {
	snd, ok := m.senders[row.Channel]
	var err error
	if !ok {
		err = permanent(errors.New("channel disabled"))
	} else {
		sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err = snd.Send(sendCtx, outbound{To: row.Destination, Subject: row.Subject, Body: row.Body, Format: row.Format})
		cancel()
	}

	status := StatusSent
	reason := ""
	next := time.Now().UTC()
	if err != nil {
		reason = err.Error()
		if isPermanent(err) || row.Attempts >= row.MaxAttempts {
			status = StatusFailed
		} else {
			status = StatusPending
			next = time.Now().UTC().Add(time.Duration(backoff(row.Attempts)) * time.Second)
		}
		m.log.Warnw("notification delivery failed",
			"delivery_id", row.ID, "channel", row.Channel, "attempt", row.Attempts, "status", status)
	} else {
		m.log.Infow("notification delivered",
			"delivery_id", row.ID, "channel", row.Channel, "attempt", row.Attempts)
	}
	if ferr := m.store.Finish(ctx, row.ID, status, next, reason, ""); ferr != nil && ctx.Err() == nil {
		m.log.Errorw("notification finish failed", "delivery_id", row.ID, "error", ferr)
	}
	if aerr := m.store.AddAttempt(ctx, row.ID, row.Attempts, status, reason); aerr != nil && ctx.Err() == nil {
		m.log.Errorw("notification attempt log failed", "delivery_id", row.ID, "error", aerr)
	}
}
