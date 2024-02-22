package utils

import (
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultWaitTimeout = 5 * time.Minute
)

func ShouldRetry(rollbackRetriesLimit *int32, statusFailed int32) bool {
	return RollBackEnabled(rollbackRetriesLimit) && statusFailed != 0
}

func RollBackEnabled(rollbackRetriesLimit *int32) bool {
	return rollbackRetriesLimit != nil
}

func RetriesLimitReached(statusFailed int32, rollbackRetriesLimit *int32) bool {
	return statusFailed >= *rollbackRetriesLimit
}

func WaitTimeout(timeout *v1.Duration) time.Duration {
	if timeout != nil {
		return timeout.Duration
	}
	return defaultWaitTimeout
}

func GetRollbackRetriesLimit(rollbackRetriesLimit *int32) int32 {
    limit := int32(1)
    if rollbackRetriesLimit != nil {
        limit = *rollbackRetriesLimit
    }
    return limit
}