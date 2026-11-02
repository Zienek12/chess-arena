// Package events delivers change notifications to connected clients. The
// store announces changes with PostgreSQL NOTIFY, so every backend instance
// sees changes made by any instance's workers.
package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chess-arena/arena/internal/store"
)

// Resync tells clients to reload everything, because events may have been
// missed while the broker was reconnecting.
const Resync = "resync"

// subscriberBuffer is how many events a slow client may lag behind before
// events are dropped for it.
const subscriberBuffer = 64

// Broker fans events out to subscribers.
type Broker struct {
	mu          sync.Mutex
	subscribers map[chan store.Event]struct{}
	logger      *slog.Logger
}

// NewBroker returns a broker without subscribers.
func NewBroker(logger *slog.Logger) *Broker {
	return &Broker{subscribers: map[chan store.Event]struct{}{}, logger: logger}
}

// Subscribe returns a channel of events and a function that unsubscribes.
func (broker *Broker) Subscribe() (<-chan store.Event, func()) {
	channel := make(chan store.Event, subscriberBuffer)
	broker.mu.Lock()
	broker.subscribers[channel] = struct{}{}
	broker.mu.Unlock()
	return channel, func() {
		broker.mu.Lock()
		defer broker.mu.Unlock()
		if _, ok := broker.subscribers[channel]; ok {
			delete(broker.subscribers, channel)
			close(channel)
		}
	}
}

// Publish sends event to every subscriber without blocking.
func (broker *Broker) Publish(event store.Event) {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	for channel := range broker.subscribers {
		select {
		case channel <- event:
		default:
		}
	}
}

// Listen receives NOTIFY events from PostgreSQL and publishes them until ctx
// is cancelled, reconnecting after errors.
func (broker *Broker) Listen(ctx context.Context, pool *pgxpool.Pool) {
	for connected := false; ctx.Err() == nil; connected = true {
		if connected {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
		if err := broker.listenOnce(ctx, pool, connected); err != nil && ctx.Err() == nil {
			broker.logger.Warn("event listener disconnected", "error", err)
		}
	}
}

func (broker *Broker) listenOnce(ctx context.Context, pool *pgxpool.Pool, reconnect bool) error {
	pooled, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	// The connection stays in LISTEN mode; take it out of the pool and close it afterwards.
	conn := pooled.Hijack()
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "LISTEN "+store.EventsChannel); err != nil {
		return err
	}
	if reconnect {
		broker.Publish(store.Event{Type: Resync})
	}
	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		var event store.Event
		if err := json.Unmarshal([]byte(notification.Payload), &event); err != nil {
			broker.logger.Warn("invalid event payload", "payload", notification.Payload)
			continue
		}
		broker.Publish(event)
	}
}
