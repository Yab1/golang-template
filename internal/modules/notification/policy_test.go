package notification

import "testing"

func TestGate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		in     gateInput
		status string
		reason string
	}{
		{name: "ready", in: gateInput{active: true, channelOn: true}, status: StatusPending},
		{name: "inactive", in: gateInput{channelOn: true}, status: StatusSkipped, reason: "account inactive"},
		{name: "off", in: gateInput{active: true, channelWhy: "channel disabled"}, status: StatusSkipped, reason: "channel disabled"},
		{name: "opt out", in: gateInput{active: true, channelOn: true, optedOut: true}, status: StatusSkipped, reason: "preference disabled"},
		{name: "no address", in: gateInput{active: true, channelOn: true, needsAddress: true}, status: StatusSkipped, reason: "destination missing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			status, reason := gate(tc.in)
			if status != tc.status || reason != tc.reason {
				t.Fatalf("got %s %q, want %s %q", status, reason, tc.status, tc.reason)
			}
		})
	}
}

func TestBackoff(t *testing.T) {
	t.Parallel()
	if backoff(1) != 30 || backoff(2) != 120 || backoff(3) != 600 || backoff(4) != 3600 {
		t.Fatalf("backoff steps: %d %d %d %d", backoff(1), backoff(2), backoff(3), backoff(4))
	}
}
