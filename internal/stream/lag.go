package stream

import (
	"context"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
)

// RunLagMonitor periodically asks Kafka for committed group offsets and broker
// end offsets. This is true broker-derived consumer lag, not queue depth or the
// number of records in the most recent fetch.
func (consumer *KafkaConsumer) RunLagMonitor(ctx context.Context, interval time.Duration) {
	if consumer.observer == nil || consumer.groupID == "" {
		return
	}
	if interval <= 0 {
		interval = 15 * time.Second
	}

	admin := kadm.NewClient(consumer.client)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	observe := func() {
		queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		lags, err := admin.Lag(queryCtx, consumer.groupID)
		if err != nil {
			consumer.logger.Warn("Kafka consumer lag query failed", "group_id", consumer.groupID, "error", err)
			return
		}
		group, ok := lags[consumer.groupID]
		if !ok {
			consumer.logger.Warn("Kafka consumer lag response missing group", "group_id", consumer.groupID)
			return
		}
		if err := group.Error(); err != nil {
			consumer.logger.Warn("Kafka consumer lag unavailable", "group_id", consumer.groupID, "error", err)
			return
		}

		total := int64(0)
		for topic, partitions := range group.Lag {
			for partition, memberLag := range partitions {
				if memberLag.Err != nil || memberLag.Lag < 0 {
					continue
				}
				consumer.observer.SetConsumerLag(topic, partition, memberLag.Lag)
				total += memberLag.Lag
			}
		}
		consumer.lag.Store(total)
		consumer.observer.SetConsumerLagTotal(total)
	}

	observe()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			observe()
		}
	}
}
