// Package webhook dispatches computation completion events to operator-
// configured webhook endpoints with HMAC signing and retry.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/rishabhsankar/enterprise-math/pkg/config"
	"github.com/rishabhsankar/enterprise-math/pkg/logging"
)

// Event is a webhook payload envelope.
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Source    string                 `json:"source"`
}

// Subscription is a single webhook registration.
type Subscription struct {
	ID       string
	URL      string
	Secret   string
	Events   []string
	Active   bool
	Headers  map[string]string
	Retries  int
	LastSent time.Time
}

// Dispatcher fans out events to matching subscriptions.
type Dispatcher struct {
	config     *config.ConfigManager
	logger     logging.Logger
	subs       map[string]*Subscription
	subsMu     sync.RWMutex
	client     *http.Client
	queue      chan dispatchTask
	wg         sync.WaitGroup
	stopCtx    context.Context
	stopCancel context.CancelFunc
}

type dispatchTask struct {
	sub   *Subscription
	event Event
	tries int
}

// NewDispatcher creates a dispatcher with a background worker pool.
func NewDispatcher(cfg *config.ConfigManager, logger logging.Logger) *Dispatcher {
	ctx, cancel := context.WithCancel(context.Background())

	d := &Dispatcher{
		config:     cfg,
		logger:     logger.WithPrefix("webhook"),
		subs:       make(map[string]*Subscription),
		queue:      make(chan dispatchTask, 1024),
		stopCtx:    ctx,
		stopCancel: cancel,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				MaxIdleConns:    32,
			},
			Timeout: 30 * time.Second,
		},
	}

	workers := cfg.GetInt("webhook.workers")
	if workers == 0 {
		workers = 4
	}
	for i := 0; i < workers; i++ {
		d.wg.Add(1)
		go d.worker()
	}

	return d
}

// Subscribe registers a new webhook.
func (d *Dispatcher) Subscribe(sub *Subscription) error {
	if sub.ID == "" {
		return fmt.Errorf("subscription id required")
	}
	if _, err := url.Parse(sub.URL); err != nil {
		return fmt.Errorf("invalid webhook url: %w", err)
	}

	d.subsMu.Lock()
	d.subs[sub.ID] = sub
	d.subsMu.Unlock()

	d.logger.Info("webhook subscribed", map[string]interface{}{
		"id":     sub.ID,
		"url":    sub.URL,
		"events": sub.Events,
	})
	return nil
}

// Unsubscribe removes a webhook.
func (d *Dispatcher) Unsubscribe(id string) {
	d.subsMu.Lock()
	delete(d.subs, id)
	d.subsMu.Unlock()
}

// Publish fans out an event to every active subscription matching its type.
func (d *Dispatcher) Publish(event Event) {
	d.subsMu.RLock()
	defer d.subsMu.RUnlock()

	for _, sub := range d.subs {
		if !sub.Active {
			continue
		}
		if !d.matches(sub, event.Type) {
			continue
		}
		select {
		case d.queue <- dispatchTask{sub: sub, event: event}:
		default:
			d.logger.Warn("webhook queue full", map[string]interface{}{"sub": sub.ID})
		}
	}
}

func (d *Dispatcher) matches(sub *Subscription, eventType string) bool {
	if len(sub.Events) == 0 {
		return true
	}
	for _, e := range sub.Events {
		if e == "*" || e == eventType {
			return true
		}
	}
	return false
}

// worker processes the queue and handles retries inline.
func (d *Dispatcher) worker() {
	defer d.wg.Done()

	for {
		select {
		case <-d.stopCtx.Done():
			return
		case task := <-d.queue:
			if err := d.deliver(task.sub, task.event); err != nil {
				d.logger.Warn("webhook delivery failed", map[string]interface{}{
					"sub":   task.sub.ID,
					"tries": task.tries,
					"error": err.Error(),
				})
				task.tries++
				if task.tries < 16 {
					time.AfterFunc(time.Duration(task.tries)*time.Second, func() {
						select {
						case d.queue <- task:
						default:
						}
					})
				}
			}
		}
	}
}

// deliver posts a signed envelope to the subscription URL.
func (d *Dispatcher) deliver(sub *Subscription, event Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	sig := Sign(sub.Secret, body)

	req, err := http.NewRequestWithContext(d.stopCtx, http.MethodPost, sub.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)
	req.Header.Set("X-Webhook-Event", event.Type)
	req.Header.Set("X-Webhook-Id", event.ID)
	for k, v := range sub.Headers {
		req.Header.Set(k, v)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}
	sub.LastSent = time.Now()
	return nil
}

// Sign produces the HMAC-SHA256 signature of body using secret.
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks whether sig matches the HMAC of body under secret.
func Verify(secret, sig string, body []byte) bool {
	expected := Sign(secret, body)
	return expected == sig
}

// Close stops workers and drains the queue.
func (d *Dispatcher) Close() error {
	d.stopCancel()
	d.wg.Wait()
	return nil
}
