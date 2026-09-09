package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/reliability"
	"github.com/fuhrdan/TelemetryForge/internal/worker"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// Observer receives broker/consumer operational metrics.
type Observer interface {
	SetConsumerLag(topic string, partition int32, lag int64)
	SetConsumerLagTotal(lag int64)
	DeadLetter(classification string)
}

// ConsumerConfig controls Kafka group consumption and dead-letter routing.
type ConsumerConfig struct {
	Brokers  []string
	ClientID string
	GroupID  string
	Topics   []string
	DLQTopic string
	Observer Observer
}

// KafkaConsumer consumes telemetry using a Kafka consumer group.
//
// Auto-commit is disabled. Concurrent workers may finish out of order, so
// commits are coordinated per topic/partition and advance only through the
// contiguous prefix of successfully handled records.
//
// Rebalances are also blocked while one bounded PollRecords batch is being
// processed. The client calls AllowRebalance only after every submitted record
// in that batch has finished normal or dead-letter handling. Together these
// rules prevent committing past unfinished work or committing after ownership
// of a partition has moved to another group member.
type KafkaConsumer struct {
	client   *kgo.Client
	logger   *slog.Logger
	dlqTopic string
	groupID  string
	observer Observer
	lag      atomic.Int64
}

// NewKafkaConsumer creates a group consumer subscribed to the supplied topics.
func NewKafkaConsumer(config ConsumerConfig, logger *slog.Logger) (*KafkaConsumer, error) {
	if len(config.Brokers) == 0 {
		return nil, errors.New("at least one Kafka broker is required")
	}
	if strings.TrimSpace(config.GroupID) == "" {
		return nil, errors.New("Kafka group ID is required")
	}
	if len(config.Topics) == 0 {
		return nil, errors.New("at least one Kafka topic is required")
	}

	clientID := strings.TrimSpace(config.ClientID)
	if clientID == "" {
		clientID = "telemetryforge-worker"
	}
	dlqTopic := strings.TrimSpace(config.DLQTopic)
	if dlqTopic == "" {
		dlqTopic = "telemetry.dlq"
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(config.Brokers...),
		kgo.ClientID(clientID),
		kgo.ConsumerGroup(config.GroupID),
		kgo.ConsumeTopics(config.Topics...),
		kgo.DisableAutoCommit(),
		kgo.Balancers(kgo.CooperativeStickyBalancer()),
		// PollRecords is bounded to 100 records below. Blocking rebalances
		// while that bounded batch is in-flight prevents workers from committing
		// offsets for partitions that have already been reassigned.
		kgo.BlockRebalanceOnPoll(),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordPartitioner(kgo.StickyKeyPartitioner(nil)),
	)
	if err != nil {
		return nil, fmt.Errorf("create Kafka consumer: %w", err)
	}

	return &KafkaConsumer{
		client: client, logger: logger, dlqTopic: dlqTopic,
		groupID: config.GroupID, observer: config.Observer,
	}, nil
}

// Run polls Kafka and submits decoded events to the bounded worker pool.
//
// A non-empty PollRecords call blocks consumer-group rebalances until the
// complete batch is handled. processBatch waits for all jobs that were handed
// to workers, then Run explicitly releases the rebalance gate.
//
// An unresolved worker/commit failure cancels new submissions. Already
// submitted jobs still drain so the current ownership epoch is not released
// while their acknowledgements are in flight.
func (consumer *KafkaConsumer) Run(ctx context.Context, pool *worker.Pool) error {
	acknowledgements := newAckCoordinator(consumer.client)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	fatal := make(chan error, 1)
	fail := func(err error) {
		select {
		case fatal <- err:
			cancel()
		default:
		}
	}

	fatalOrContext := func(fallback error) error {
		select {
		case err := <-fatal:
			return err
		default:
		}
		if ctx.Err() != nil {
			return nil
		}
		return fallback
	}

	for {
		fetches := consumer.client.PollRecords(runCtx, 100)
		records := fetches.Records()

		// BlockRebalanceOnPoll gates only non-empty record polls. Always release
		// that gate after the batch is either fully handled or safely abandoned.
		if len(records) > 0 {
			batchErr := func() error {
				defer consumer.client.AllowRebalance()

				if errs := fetches.Errors(); len(errs) > 0 {
					return fmt.Errorf("poll Kafka: %w", errs[0].Err)
				}

				return consumer.processBatch(
					runCtx,
					ctx,
					pool,
					acknowledgements,
					records,
					fail,
					fatalOrContext,
				)
			}()
			if batchErr != nil {
				return batchErr
			}
			continue
		}

		if runCtx.Err() != nil {
			return fatalOrContext(runCtx.Err())
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			return fmt.Errorf("poll Kafka: %w", errs[0].Err)
		}
	}
}

func (consumer *KafkaConsumer) processBatch(
	runCtx context.Context,
	commitCtx context.Context,
	pool *worker.Pool,
	acknowledgements *ackCoordinator,
	records []*kgo.Record,
	fail func(error),
	fatalOrContext func(error) error,
) error {
	var jobs sync.WaitGroup

	// processBatch must not return while any job from this poll is still
	// running. Run releases the rebalance gate immediately after this function
	// returns, so the wait is the ownership boundary.
	defer jobs.Wait()

	for _, record := range records {
		if runCtx.Err() != nil {
			return fatalOrContext(runCtx.Err())
		}

		rec := record
		if err := acknowledgements.Register(rec); err != nil {
			return fmt.Errorf("register Kafka offset: %w", err)
		}

		var event domain.Event
		if err := json.Unmarshal(rec.Value, &event); err != nil {
			dead := domain.DeadLetter{
				RawPayload:    append([]byte(nil), rec.Value...),
				OriginalTopic: rec.Topic,
				Partition:     rec.Partition,
				Offset:        rec.Offset,
				FailureClass:  string(reliability.Permanent),
				Error:         "decode event: " + err.Error(),
				Attempts:      1,
				FailedAt:      time.Now().UTC(),
			}
			if dlqErr := consumer.publishDeadLetter(commitCtx, dead); dlqErr != nil {
				return fmt.Errorf("publish malformed record to DLQ: %w", dlqErr)
			}
			if err := acknowledgements.Ack(commitCtx, rec); err != nil {
				return fmt.Errorf("acknowledge malformed DLQ record: %w", err)
			}
			continue
		}

		carrier := propagation.MapCarrier{}
		for _, header := range rec.Headers {
			carrier.Set(header.Key, string(header.Value))
		}
		eventCtx := otel.GetTextMapPropagator().Extract(runCtx, carrier)

		jobs.Add(1)
		job := worker.Job{
			Context: eventCtx,
			Event:   event,
			Done:    jobs.Done,
			Ack: func(ackCtx context.Context) error {
				return acknowledgements.Ack(ackCtx, rec)
			},
			Nack: func(err error) {
				consumer.logger.Warn("event remains uncommitted",
					"event_id", event.ID,
					"topic", rec.Topic,
					"partition", rec.Partition,
					"offset", rec.Offset,
					"error", err)
				fail(fmt.Errorf(
					"unresolved Kafka record topic=%s partition=%d offset=%d: %w",
					rec.Topic,
					rec.Partition,
					rec.Offset,
					err,
				))
			},
			Failure: func(failureCtx context.Context, failed domain.Event, err error, attempts int) error {
				copyEvent := failed
				return consumer.publishDeadLetter(failureCtx, domain.DeadLetter{
					Event:         &copyEvent,
					OriginalTopic: rec.Topic,
					Partition:     rec.Partition,
					Offset:        rec.Offset,
					FailureClass:  string(reliability.Classification(err)),
					Error:         err.Error(),
					Attempts:      attempts,
					FailedAt:      time.Now().UTC(),
				})
			},
		}

		if err := pool.Submit(runCtx, job); err != nil {
			// The job was never handed to a worker, so balance the Add here.
			jobs.Done()
			return fatalOrContext(fmt.Errorf("submit event to worker pool: %w", err))
		}

		consumer.lag.Store(int64(len(records)))
	}

	return nil
}

func (consumer *KafkaConsumer) publishDeadLetter(ctx context.Context, dead domain.DeadLetter) error {
	payload, err := json.Marshal(dead)
	if err != nil {
		return fmt.Errorf("encode dead letter: %w", err)
	}

	key := []byte(dead.OriginalTopic)
	if dead.Event != nil && dead.Event.Source != "" {
		key = []byte(dead.Event.Source)
	}

	result := consumer.client.ProduceSync(ctx, &kgo.Record{
		Topic: consumer.dlqTopic,
		Key:   key,
		Value: payload,
		Headers: []kgo.RecordHeader{
			{Key: "telemetryforge-original-topic", Value: []byte(dead.OriginalTopic)},
			{Key: "telemetryforge-failure-class", Value: []byte(dead.FailureClass)},
		},
	})
	if err := result.FirstErr(); err != nil {
		return fmt.Errorf("publish DLQ record: %w", err)
	}

	consumer.logger.Error("telemetry moved to dead-letter queue",
		"original_topic", dead.OriginalTopic,
		"partition", dead.Partition,
		"offset", dead.Offset,
		"attempts", dead.Attempts,
		"classification", dead.FailureClass)
	if consumer.observer != nil {
		consumer.observer.DeadLetter(dead.FailureClass)
	}
	return nil
}

// EstimatedLag returns the most recent local batch backlog estimate.
func (consumer *KafkaConsumer) EstimatedLag() int64 { return consumer.lag.Load() }

// Ready checks broker connectivity.
func (consumer *KafkaConsumer) Ready(ctx context.Context) error {
	readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return consumer.client.Ping(readyCtx)
}

// Close allows any blocked rebalance before leaving the consumer group.
func (consumer *KafkaConsumer) Close() { consumer.client.CloseAllowingRebalance() }
