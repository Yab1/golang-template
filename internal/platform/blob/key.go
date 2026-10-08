package blob

import (
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	clinicOnce sync.Once
	clinicLoc  *time.Location
)

func clinicTime(at time.Time) time.Time {
	clinicOnce.Do(func() {
		loc, err := time.LoadLocation("Africa/Addis_Ababa")
		if err != nil {
			loc = time.UTC
		}
		clinicLoc = loc
	})
	if at.IsZero() {
		at = time.Now()
	}
	return at.In(clinicLoc)
}

// UploadKey is the bucket path for a client file.
// charts/patient/{id}/photo/2026/10/{uuid}.png
func UploadKey(class, resource, recordID, field, ext string, at time.Time) string {
	at = clinicTime(at)
	return class + "/" + resource + "/" + recordID + "/" + field + "/" + at.Format("2006/01") + "/" + uuid.NewString() + cleanExt(ext)
}

// DocumentKey is the bucket path for a printed PDF or CSV.
// documents/invoice/{id}/2026/10/{uuid}.pdf
func DocumentKey(kind, subjectID, ext string, at time.Time) string {
	at = clinicTime(at)
	return "documents/" + kind + "/" + subjectID + "/" + at.Format("2006/01") + "/" + uuid.NewString() + cleanExt(ext)
}

func cleanExt(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	ext = strings.TrimPrefix(ext, ".")
	if ext == "" || len(ext) > 8 {
		return ""
	}
	for _, r := range ext {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return ""
		}
	}
	return "." + ext
}
