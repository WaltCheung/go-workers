package workers

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

const (
	LAYOUT = "2006-01-02 15:04:05 MST"
)

type MiddlewareRetry struct{}

func (r *MiddlewareRetry) Call(queue string, message *Msg, next func() CallResult) (result CallResult) {
	defer func() {
		if e := recover(); e != nil || result.Err != nil {
			conn := Config.Pool.Get()
			defer conn.Close()

			if retry(message) {
				message.Set("queue", queue)

				if e != nil {
					message.Set("error_message", fmt.Sprintf("%v", e))
				} else {
					message.Set("error_message", fmt.Sprintf("%v", result.Err))
				}
				retryCount := incrementRetry(message)

				waitDuration := durationToSecondsWithNanoPrecision(
					time.Duration(
						secondsToDelay(retryCount),
					) * time.Second,
				)

				_, err := conn.Do(
					"zadd",
					Config.Namespace+RETRY_KEY,
					nowToSecondsWithNanoPrecision()+waitDuration,
					message.ToJson(),
				)

				// If we can't add the job to the retry queue,
				// then we shouldn't acknowledge the job, otherwise
				// it'll disappear into the void.
				if err != nil {
					result.Acknowledge = false
				}
			}

			if e != nil {
				panic(e)
			}
		}
	}()

	result = next()

	return
}

func retry(message *Msg) bool {
	retry := false
	max := 0
	if param, err := message.Get("max_retries").Int(); err == nil {
		max = param
		retry = param > 0
	}

	count, _ := message.Get("retry_count").Int()

	return retry && count < max
}

func incrementRetry(message *Msg) (retryCount int) {
	retryCount = 0

	if count, err := message.Get("retry_count").Int(); err != nil {
		message.Set("failed_at", time.Now().UTC().Format(LAYOUT))
	} else {
		message.Set("retried_at", time.Now().UTC().Format(LAYOUT))
		retryCount = count + 1
	}

	message.Set("retry_count", retryCount)

	return
}

func secondsToDelay(count int) int {
	power := math.Pow(float64(count), 4)
	return int(power) + 15 + (rand.Intn(30) * (count + 1))
}
