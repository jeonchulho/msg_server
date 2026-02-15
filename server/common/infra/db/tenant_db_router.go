package db

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type tenantMeta struct {
	DeploymentMode string
	DedicatedDSN   string
	IsActive       bool
}

type TenantDBMeta struct {
	DeploymentMode string
	DedicatedDSN   string
	IsActive       bool
}

type TenantMetaProvider interface {
	GetTenantDBMeta(ctx context.Context, tenantID string) (TenantDBMeta, error)
}

type sharedTenantMetaProvider struct {
	shared *pgxpool.Pool
}

func (p *sharedTenantMetaProvider) GetTenantDBMeta(ctx context.Context, tenantID string) (TenantDBMeta, error) {
	var meta TenantDBMeta
	err := p.shared.QueryRow(ctx, `
		SELECT deployment_mode, dedicated_dsn, is_active
		FROM tenants
		WHERE tenant_id = $1
	`, tenantID).Scan(&meta.DeploymentMode, &meta.DedicatedDSN, &meta.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TenantDBMeta{}, errors.New("tenant not found")
		}
		return TenantDBMeta{}, err
	}
	meta.DeploymentMode = strings.ToLower(strings.TrimSpace(meta.DeploymentMode))
	return meta, nil
}

type cachedTenantMeta struct {
	meta      tenantMeta
	fetchedAt time.Time
}

type TenantDBRouter struct {
	shared    *pgxpool.Pool
	provider  TenantMetaProvider
	cacheTTL  time.Duration
	mu        sync.RWMutex
	metaCache map[string]cachedTenantMeta
	dedicated map[string]*pgxpool.Pool
}

func NewTenantDBRouter(shared *pgxpool.Pool) *TenantDBRouter {
	return NewTenantDBRouterWithProvider(shared, &sharedTenantMetaProvider{shared: shared})
}

func NewTenantDBRouterWithProvider(shared *pgxpool.Pool, provider TenantMetaProvider) *TenantDBRouter {
	if provider == nil {
		provider = &sharedTenantMetaProvider{shared: shared}
	}
	return &TenantDBRouter{
		shared:    shared,
		provider:  provider,
		cacheTTL:  30 * time.Second,
		metaCache: map[string]cachedTenantMeta{},
		dedicated: map[string]*pgxpool.Pool{},
	}
}

func (r *TenantDBRouter) DBForTenant(ctx context.Context, tenantID string) (*pgxpool.Pool, error) {
	if strings.TrimSpace(tenantID) == "" {
		return r.shared, nil
	}

	meta, err := r.loadTenantMeta(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if !meta.IsActive {
		return nil, errors.New("tenant is inactive")
	}
	if meta.DeploymentMode != "dedicated" || strings.TrimSpace(meta.DedicatedDSN) == "" {
		return r.shared, nil
	}

	r.mu.RLock()
	if existing, ok := r.dedicated[tenantID]; ok {
		r.mu.RUnlock()
		return existing, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.dedicated[tenantID]; ok {
		return existing, nil
	}

	pool, err := pgxpool.New(ctx, meta.DedicatedDSN)
	if err != nil {
		return nil, err
	}
	r.dedicated[tenantID] = pool
	return pool, nil
}

func (r *TenantDBRouter) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for tenantID, pool := range r.dedicated {
		pool.Close()
		delete(r.dedicated, tenantID)
	}
}

func (r *TenantDBRouter) InvalidateTenant(tenantID string) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.metaCache, tenantID)
	if pool, ok := r.dedicated[tenantID]; ok {
		pool.Close()
		delete(r.dedicated, tenantID)
	}
}

func (r *TenantDBRouter) loadTenantMeta(ctx context.Context, tenantID string) (tenantMeta, error) {
	now := time.Now()
	r.mu.RLock()
	if cached, ok := r.metaCache[tenantID]; ok && now.Sub(cached.fetchedAt) < r.cacheTTL {
		r.mu.RUnlock()
		return cached.meta, nil
	}
	r.mu.RUnlock()

	providerMeta, err := r.provider.GetTenantDBMeta(ctx, tenantID)
	if err != nil {
		return tenantMeta{}, err
	}

	meta := tenantMeta{
		DeploymentMode: providerMeta.DeploymentMode,
		DedicatedDSN:   providerMeta.DedicatedDSN,
		IsActive:       providerMeta.IsActive,
	}

	r.mu.Lock()
	r.metaCache[tenantID] = cachedTenantMeta{meta: meta, fetchedAt: now}
	r.mu.Unlock()
	return meta, nil
}
