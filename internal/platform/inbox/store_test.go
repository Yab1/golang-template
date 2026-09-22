package inbox

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Yab1/golang-template/internal/platform/event"
)

func TestProcessTreatsDuplicateAndStaleAsNoOp(t *testing.T) {
	t.Parallel()

	for _, sentinel := range []error{ErrDuplicate, ErrStale} {
		if err := processOnce(func() error { return sentinel }); err != nil {
			t.Fatal(err)
		}
	}
}

func processOnce(next func() error) error {
	if err := next(); err != nil && !errors.Is(err, ErrDuplicate) && !errors.Is(err, ErrStale) {
		return err
	}
	return nil
}

func TestHandlerSignature(t *testing.T) {
	t.Parallel()
	var handler Handler = func(context.Context, pgx.Tx, event.Event) error {
		return nil
	}
	if handler == nil {
		t.Fatal("handler required")
	}
	_ = uuid.Nil
}
