package event

import (
	"fmt"
	"strings"
)

func Topic(prefix, boundedContext, aggregate string) string {
	parts := []string{
		strings.Trim(prefix, "."),
		strings.Trim(boundedContext, "."),
		strings.Trim(aggregate, "."),
		"events",
		"v1",
	}
	return strings.Join(parts, ".")
}

func RetryTopic(sourceTopic, consumer string) string {
	return fmt.Sprintf("%s.%s.retry.v1", sourceTopic, consumer)
}

func DLQTopic(sourceTopic, consumer string) string {
	return fmt.Sprintf("%s.%s.dlq.v1", sourceTopic, consumer)
}
