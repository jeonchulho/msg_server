package cache

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type TenantRedisMeta struct {
	DeploymentMode     string
	DedicatedRedisAddr string
	IsActive           bool
}

type TenantRedisMetaProvider interface {
	GetTenantRedisMeta(ctx context.Context, tenantID string) (TenantRedisMeta, error)
}

type TenantRedisRouter struct {
	shared    *redis.Client
	provider  TenantRedisMetaProvider
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

func NewTenantRedisRouter(shared *redis.Client, provider TenantRedisMetaProvider) *TenantRedisRouter {
	return &TenantRedisRouter{
		shared:    shared,
		provider:  provider,
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
	if r.provider == nil {
		meta = redisMeta{Mode: "shared", Addr: "", IsActive: true}
	} else {
		providerMeta, err := r.provider.GetTenantRedisMeta(ctx, tenantID)
		if err != nil {
			return redisMeta{}, err
		}
		meta = redisMeta{
			Mode:     strings.ToLower(strings.TrimSpace(providerMeta.DeploymentMode)),
			Addr:     strings.TrimSpace(providerMeta.DedicatedRedisAddr),
			IsActive: providerMeta.IsActive,
		}
	}

	r.mu.Lock()
	r.metaCache[tenantID] = cachedRedisMeta{meta: meta, fetchedAt: now}
	r.mu.Unlock()
	return meta, nil
}
