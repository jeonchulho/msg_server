package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

type tenantPublisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

type AMQPPublisher struct {
	db        *pgxpool.Pool
	shared    *tenantPublisher
	mu        sync.RWMutex
	dedicated map[string]*tenantPublisher
	metaCache map[string]cachedMQMeta
	cacheTTL  time.Duration
}

type mqMeta struct {
	mode     string
	url      string
	isActive bool
}

type cachedMQMeta struct {
	meta      mqMeta
	fetchedAt time.Time
}

func NewAMQPPublisher(conn *amqp.Connection, db *pgxpool.Pool) (*AMQPPublisher, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	if err := ch.ExchangeDeclare("chat.events", "topic", true, false, false, false, nil); err != nil {
		return nil, err
	}
	return &AMQPPublisher{
		db:        db,
		shared:    &tenantPublisher{conn: conn, channel: ch},
		dedicated: map[string]*tenantPublisher{},
		metaCache: map[string]cachedMQMeta{},
		cacheTTL:  30 * time.Second,
	}, nil
}

func (p *AMQPPublisher) Publish(ctx context.Context, tenantID, key string, payload any) error {
	publisher, err := p.publisherForTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	routingKey := key
	if strings.TrimSpace(tenantID) != "" {
		routingKey = tenantID + "." + key
	}
	return publisher.channel.PublishWithContext(ctx, "chat.events", routingKey, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
		Timestamp:   time.Now(),
	})
}

func (p *AMQPPublisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for tenantID, pub := range p.dedicated {
		if pub.channel != nil {
			_ = pub.channel.Close()
		}
		if pub.conn != nil {
			_ = pub.conn.Close()
		}
		delete(p.dedicated, tenantID)
	}
}

func (p *AMQPPublisher) InvalidateTenant(tenantID string) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.metaCache, tenantID)
	if pub, ok := p.dedicated[tenantID]; ok {
		if pub.channel != nil {
			_ = pub.channel.Close()
		}
		if pub.conn != nil {
			_ = pub.conn.Close()
		}
		delete(p.dedicated, tenantID)
	}
}

func (p *AMQPPublisher) publisherForTenant(ctx context.Context, tenantID string) (*tenantPublisher, error) {
	if strings.TrimSpace(tenantID) == "" {
		return p.shared, nil
	}
	meta, err := p.loadMeta(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if !meta.isActive {
		return nil, errors.New("tenant is inactive")
	}
	if meta.mode != "dedicated" || strings.TrimSpace(meta.url) == "" {
		return p.shared, nil
	}

	p.mu.RLock()
	if existing, ok := p.dedicated[tenantID]; ok {
		p.mu.RUnlock()
		return existing, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()
	if existing, ok := p.dedicated[tenantID]; ok {
		return existing, nil
	}

	conn, err := amqp.Dial(meta.url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := ch.ExchangeDeclare("chat.events", "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	pub := &tenantPublisher{conn: conn, channel: ch}
	p.dedicated[tenantID] = pub
	return pub, nil
}

func (p *AMQPPublisher) loadMeta(ctx context.Context, tenantID string) (mqMeta, error) {
	now := time.Now()
	p.mu.RLock()
	if cached, ok := p.metaCache[tenantID]; ok && now.Sub(cached.fetchedAt) < p.cacheTTL {
		p.mu.RUnlock()
		return cached.meta, nil
	}
	p.mu.RUnlock()

	var meta mqMeta
	err := p.db.QueryRow(ctx, `
		SELECT deployment_mode, dedicated_lavinmq_url, is_active
		FROM tenants
		WHERE tenant_id=$1
	`, tenantID).Scan(&meta.mode, &meta.url, &meta.isActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return mqMeta{}, errors.New("tenant not found")
		}
		return mqMeta{}, err
	}
	meta.mode = strings.ToLower(strings.TrimSpace(meta.mode))

	p.mu.Lock()
	p.metaCache[tenantID] = cachedMQMeta{meta: meta, fetchedAt: now}
	p.mu.Unlock()
	return meta, nil
}
