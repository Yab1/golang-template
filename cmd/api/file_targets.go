package main

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Yab1/golang-template/internal/modules/file"
)

// registerFileTargets wires generic upload slots. Domain-specific targets
// (avatars, logos, …) register the same way when those modules exist.
func registerFileTargets(files *file.Module) {
	if files == nil {
		return
	}
	files.Register("upload", "file", file.Target{
		Class: "uploads",
		Locate: func(_ context.Context, raw string) (uuid.UUID, error) {
			id, err := uuid.Parse(raw)
			if err != nil {
				return uuid.Nil, file.Status(http.StatusBadRequest, err)
			}
			return id, nil
		},
		Bind: func(_ context.Context, _ *http.Request, _ string, _ *file.Object, _ []byte, _ pgx.Tx) (string, error) {
			return "", nil
		},
	})
}
