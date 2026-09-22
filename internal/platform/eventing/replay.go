package eventing

import "github.com/twmb/franz-go/pkg/kgo"

func ReplayHeaders(headers []kgo.RecordHeader) []kgo.RecordHeader {
	out := make([]kgo.RecordHeader, 0, len(headers))
	for _, h := range headers {
		switch h.Key {
		case headerAttempt, headerNotBefore, headerErrorClass, headerLastFailedAt:
			continue
		default:
			out = append(out, h)
		}
	}
	return out
}
