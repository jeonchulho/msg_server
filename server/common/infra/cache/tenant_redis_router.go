package cache

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type TenantRedisRouter struct {
	shared    *redis.Client
	db        *pgxpool.Pool
	cacheTTL  time.Duration
	mu        sync.RWMutex
	metaCache map[string]cachedRedisMeta
	clients   map[string]*redis.Client
}

type redisMeta struct {
	Mode     string
	Addr     string
	IsActive bool
}

type cachedRedisMeta struct {
	meta      redisMeta
	fetchedAt time.Time
}

func NewTenantRedisRouter(shared *redis.Client, db *pgxpool.Pool) *TenantRedisRouter {
	return &TenantRedisRouter{
		shared:    shared,
		db:        db,
		cacheTTL:  30 * time.Second,
		metaCache: map[string]cachedRedisMeta{},
		clients:   map[string]*redis.Client{},
	}
}

func (r *TenantRedisRouter) ClientForTenant(ctx context.Context, tenantID string) (*redis.Client, error) {
	if strings.TrimSpace(tenantID) == "" {
		return r.shared, nil
	}
	meta, err := r.loadMeta(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if !meta.IsActive {
		return nil, errors.New("tenant is inactive")
	}
	if meta.Mode != "dedicated" || strings.TrimSpace(meta.Addr) == "" {
		return r.shared, nil
	}

	r.mu.RLock()
	if c, ok := r.clients[tenantID]; ok {
		r.mu.RUnlock()
		return c, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clients[tenantID]; ok {
		return c, nil
	}
	client := redis.NewClient(&redis.Options{Addr: meta.Addr})
	r.clients[tenantID] = client
	return client, nil
}

func (r *TenantRedisRouter) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for tenantID, client := range r.clients {
		_ = client.Close()
		delete(r.clients, tenantID)
	}
}

func (r *TenantRedisRouter) InvalidateTenant(tenantID string) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.metaCache, tenantID)
	if client, ok := r.clients[tenantID]; ok {
		_ = client.Close()
		delete(r.clients, tenantID)
	}
}

func (r *TenantRedisRouter) loadMeta(ctx context.Context, tenantID string) (redisMeta, error) {
	now := time.Now()
	r.mu.RLock()
	if cached, ok := r.metaCache[tenantID]; ok && now.Sub(cached.fetchedAt) < r.cacheTTL {
		r.mu.RUnlock()
		return cached.meta, nil
	}
	r.mu.RUnlock()

	var meta redisMeta
	err := r.db.QueryRow(ctx, `
		SELECT deployment_mode, dedicated_redis_addr, is_active
		FROM tenants
		WHERE tenant_id=$1
	`, tenantID).Scan(&meta.Mode, &meta.Addr, &meta.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return redisMeta{}, errors.New("tenant not found")
		}
		return redisMeta{}, err
	}
	meta.Mode = strings.ToLower(strings.TrimSpace(meta.Mode))

	r.mu.Lock()
	r.metaCache[tenantID] = cachedRedisMeta{meta: meta, fetchedAt: now}
	r.mu.Unlock()
	return meta, nil
}
