package notification

import "strings"

type gateInput struct {
	active       bool
	channelOn    bool
	channelWhy   string
	optedOut     bool
	needsAddress bool
	destination  string
}

func gate(in gateInput) (status, reason string) {
	if !in.active {
		return StatusSkipped, "account inactive"
	}
	if !in.channelOn {
		if in.channelWhy == "" {
			in.channelWhy = "channel disabled"
		}
		return StatusSkipped, in.channelWhy
	}
	if in.optedOut {
		return StatusSkipped, "preference disabled"
	}
	if in.needsAddress && strings.TrimSpace(in.destination) == "" {
		return StatusSkipped, "destination missing"
	}
	return StatusPending, ""
}

func needsAddress(channel string) bool {
	return channel != ChannelInApp
}

func backoff(attempt int) int {
	switch attempt {
	case 1:
		return 30
	case 2:
		return 120
	case 3:
		return 600
	default:
		return 3600
	}
}
