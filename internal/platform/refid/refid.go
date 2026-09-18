package refid

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	prefixLen = 3
	codeLen   = 3
	randLen   = 5
	charset   = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	maxTries  = 8
)

var ErrExhausted = errors.New("refid: could not allocate unique reference id")

type Generator struct {
	prefix string
}

func New(prefix string) *Generator {
	return &Generator{prefix: Normalize(prefix, "APP")}
}

func (g *Generator) Prefix() string {
	if g == nil {
		return "APP"
	}
	return g.prefix
}

func (g *Generator) Next(modelCode string) (string, error) {
	prefix := "APP"
	if g != nil {
		prefix = g.prefix
	}
	code := Normalize(modelCode, "XXX")
	suffix, err := random(randLen)
	if err != nil {
		return "", err
	}
	return prefix + "-" + code + "-" + suffix, nil
}

func Normalize(s, fallback string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	fb := fallback
	if fb == "" {
		fb = "XXX"
	}
	for len(out) < prefixLen {
		if len(fb) == 0 {
			fb = "X"
		}
		out += fb[:1]
		if len(fb) > 1 {
			fb = fb[1:]
		}
	}
	if len(out) > prefixLen {
		out = out[:prefixLen]
	}
	return out
}

func random(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i := range buf {
		out[i] = charset[int(buf[i])%len(charset)]
	}
	return string(out), nil
}

func IsConflict(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != "23505" {
		return false
	}
	return strings.Contains(pgErr.ConstraintName, "reference_id")
}

func MaxTries() int { return maxTries }

func Valid(s string) bool {
	parts := strings.Split(s, "-")
	if len(parts) != 3 {
		return false
	}
	if len(parts[0]) != prefixLen || len(parts[1]) != codeLen || len(parts[2]) != randLen {
		return false
	}
	return allUpper(parts[0]) && allUpper(parts[1])
}

func allUpper(s string) bool {
	for _, r := range s {
		if !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}

func Format(prefix, code, suffix string) string {
	return fmt.Sprintf("%s-%s-%s", prefix, code, suffix)
}
