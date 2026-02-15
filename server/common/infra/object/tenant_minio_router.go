package object

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
)

type TenantMinIOMeta struct {
	Mode      string
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
	Bucket    string
	IsActive  bool
}

type TenantMinIOMetaProvider interface {
	GetTenantMinIOMeta(ctx context.Context, tenantID string) (TenantMinIOMeta, error)
}

type cachedTenantMinIOMeta struct {
	meta      TenantMinIOMeta
	fetchedAt time.Time
}

type TenantMinIORouter struct {
	sharedClient *minio.Client
	sharedBucket string
	provider     TenantMinIOMetaProvider
	cacheTTL     time.Duration
	mu           sync.RWMutex
	metaCache    map[string]cachedTenantMinIOMeta
	clients      map[string]*minio.Client
}

func NewTenantMinIORouter(sharedClient *minio.Client, sharedBucket string, provider TenantMinIOMetaProvider) *TenantMinIORouter {
	return &TenantMinIORouter{
		sharedClient: sharedClient,
		sharedBucket: sharedBucket,
		provider:     provider,
		cacheTTL:     30 * time.Second,
		metaCache:    map[string]cachedTenantMinIOMeta{},
		clients:      map[string]*minio.Client{},
	}
}

func (r *TenantMinIORouter) Resolve(ctx context.Context, tenantID string) (*minio.Client, string, string, error) {
	if strings.TrimSpace(tenantID) == "" {
		return r.sharedClient, r.sharedBucket, "", nil
	}
	meta, err := r.loadMeta(ctx, tenantID)
	if err != nil {
		return nil, "", "", err
	}
	if !meta.IsActive {
		return nil, "", "", errors.New("tenant is inactive")
	}
	if meta.Mode != "dedicated" || strings.TrimSpace(meta.Endpoint) == "" || strings.TrimSpace(meta.Bucket) == "" {
		return r.sharedClient, r.sharedBucket, fmt.Sprintf("tenants/%s/", tenantID), nil
	}

	r.mu.RLock()
	if c, ok := r.clients[tenantID]; ok {
		r.mu.RUnlock()
		return c, meta.Bucket, "", nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clients[tenantID]; ok {
		return c, meta.Bucket, "", nil
	}
	client, err := NewClient(meta.Endpoint, meta.AccessKey, meta.SecretKey, meta.UseSSL)
	if err != nil {
		return nil, "", "", err
	}
	if err := EnsureBucket(ctx, client, meta.Bucket); err != nil {
		return nil, "", "", err
	}
	r.clients[tenantID] = client
	return client, meta.Bucket, "", nil
}

func (r *TenantMinIORouter) loadMeta(ctx context.Context, tenantID string) (TenantMinIOMeta, error) {
	now := time.Now()
	r.mu.RLock()
	if cached, ok := r.metaCache[tenantID]; ok && now.Sub(cached.fetchedAt) < r.cacheTTL {
		r.mu.RUnlock()
		return cached.meta, nil
	}
	r.mu.RUnlock()

	meta, err := r.provider.GetTenantMinIOMeta(ctx, tenantID)
	if err != nil {
		return TenantMinIOMeta{}, err
	}
	meta.Mode = strings.ToLower(strings.TrimSpace(meta.Mode))
	meta.Endpoint = strings.TrimSpace(meta.Endpoint)
	meta.Bucket = strings.TrimSpace(meta.Bucket)

	r.mu.Lock()
	r.metaCache[tenantID] = cachedTenantMinIOMeta{meta: meta, fetchedAt: now}
	r.mu.Unlock()
	return meta, nil
}

func (r *TenantMinIORouter) InvalidateTenant(tenantID string) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.metaCache, tenantID)
	delete(r.clients, tenantID)
}
