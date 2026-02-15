package repository

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"msg_server/server/chat/domain"
)

type TenantRepository struct {
	db *pgxpool.Pool
}

func NewTenantRepository(db *pgxpool.Pool) *TenantRepository {
	return &TenantRepository{db: db}
}

func (r *TenantRepository) List(ctx context.Context) ([]domain.Tenant, error) {
	rows, err := r.db.Query(ctx, `
		SELECT tenant_id, name, deployment_mode, dedicated_dsn, dedicated_redis_addr, dedicated_lavinmq_url,
		       dedicated_minio_endpoint, dedicated_minio_access_key, dedicated_minio_secret_key,
		       dedicated_minio_bucket, dedicated_minio_use_ssl,
		       user_count_threshold, is_active, created_at, updated_at
		FROM tenants
		ORDER BY tenant_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.Tenant, 0)
	for rows.Next() {
		var item domain.Tenant
		if err := rows.Scan(
			&item.TenantID,
			&item.Name,
			&item.DeploymentMode,
			&item.DedicatedDSN,
			&item.DedicatedRedisAddr,
			&item.DedicatedLavinMQURL,
			&item.DedicatedMinIOEndpoint,
			&item.DedicatedMinIOAccessKey,
			&item.DedicatedMinIOSecretKey,
			&item.DedicatedMinIOBucket,
			&item.DedicatedMinIOUseSSL,
			&item.UserCountThreshold,
			&item.IsActive,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *TenantRepository) GetByID(ctx context.Context, tenantID string) (domain.Tenant, error) {
	var item domain.Tenant
	err := r.db.QueryRow(ctx, `
		SELECT tenant_id, name, deployment_mode, dedicated_dsn, dedicated_redis_addr, dedicated_lavinmq_url,
		       dedicated_minio_endpoint, dedicated_minio_access_key, dedicated_minio_secret_key,
		       dedicated_minio_bucket, dedicated_minio_use_ssl,
		       user_count_threshold, is_active, created_at, updated_at
		FROM tenants
		WHERE tenant_id = $1
	`, tenantID).Scan(
		&item.TenantID,
		&item.Name,
		&item.DeploymentMode,
		&item.DedicatedDSN,
		&item.DedicatedRedisAddr,
		&item.DedicatedLavinMQURL,
		&item.DedicatedMinIOEndpoint,
		&item.DedicatedMinIOAccessKey,
		&item.DedicatedMinIOSecretKey,
		&item.DedicatedMinIOBucket,
		&item.DedicatedMinIOUseSSL,
		&item.UserCountThreshold,
		&item.IsActive,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	return item, err
}

func (r *TenantRepository) Create(ctx context.Context, item domain.Tenant) (domain.Tenant, error) {
	item.DeploymentMode = strings.ToLower(strings.TrimSpace(item.DeploymentMode))
	err := r.db.QueryRow(ctx, `
		INSERT INTO tenants(
			tenant_id, name, deployment_mode, dedicated_dsn,
			dedicated_redis_addr, dedicated_lavinmq_url,
			dedicated_minio_endpoint, dedicated_minio_access_key, dedicated_minio_secret_key,
			dedicated_minio_bucket, dedicated_minio_use_ssl,
			user_count_threshold, is_active
		)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING created_at, updated_at
	`, item.TenantID, item.Name, item.DeploymentMode, item.DedicatedDSN,
		item.DedicatedRedisAddr, item.DedicatedLavinMQURL,
		item.DedicatedMinIOEndpoint, item.DedicatedMinIOAccessKey, item.DedicatedMinIOSecretKey,
		item.DedicatedMinIOBucket, item.DedicatedMinIOUseSSL,
		item.UserCountThreshold, item.IsActive,
	).Scan(&item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func (r *TenantRepository) UpdateConfig(ctx context.Context, item domain.Tenant) (domain.Tenant, error) {
	err := r.db.QueryRow(ctx, `
		UPDATE tenants
		SET
			name = $2,
			deployment_mode = $3,
			dedicated_dsn = $4,
			dedicated_redis_addr = $5,
			dedicated_lavinmq_url = $6,
			dedicated_minio_endpoint = $7,
			dedicated_minio_access_key = $8,
			dedicated_minio_secret_key = $9,
			dedicated_minio_bucket = $10,
			dedicated_minio_use_ssl = $11,
			user_count_threshold = $12,
			is_active = $13,
			updated_at = NOW()
		WHERE tenant_id = $1
		RETURNING tenant_id, name, deployment_mode, dedicated_dsn, dedicated_redis_addr, dedicated_lavinmq_url,
		          dedicated_minio_endpoint, dedicated_minio_access_key, dedicated_minio_secret_key,
		          dedicated_minio_bucket, dedicated_minio_use_ssl,
		          user_count_threshold, is_active, created_at, updated_at
	`, item.TenantID, item.Name, item.DeploymentMode, item.DedicatedDSN,
		item.DedicatedRedisAddr, item.DedicatedLavinMQURL,
		item.DedicatedMinIOEndpoint, item.DedicatedMinIOAccessKey, item.DedicatedMinIOSecretKey,
		item.DedicatedMinIOBucket, item.DedicatedMinIOUseSSL,
		item.UserCountThreshold, item.IsActive,
	).Scan(
		&item.TenantID,
		&item.Name,
		&item.DeploymentMode,
		&item.DedicatedDSN,
		&item.DedicatedRedisAddr,
		&item.DedicatedLavinMQURL,
		&item.DedicatedMinIOEndpoint,
		&item.DedicatedMinIOAccessKey,
		&item.DedicatedMinIOSecretKey,
		&item.DedicatedMinIOBucket,
		&item.DedicatedMinIOUseSSL,
		&item.UserCountThreshold,
		&item.IsActive,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	return item, err
}
