package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/minio/minio-go/v7"

	"msg_server/server/common/infra/object"
	"msg_server/server/fileman/domain"
)

type FileService struct {
	dbman       *DBManClient
	minioRouter *object.TenantMinIORouter
}

func NewFileService(dbman *DBManClient, minioRouter *object.TenantMinIORouter) *FileService {
	return &FileService{dbman: dbman, minioRouter: minioRouter}
}

func (s *FileService) PresignUpload(ctx context.Context, tenantID, objectKey string) (string, error) {
	client, bucket, prefix, err := s.minioRouter.Resolve(ctx, tenantID)
	if err != nil {
		return "", err
	}
	tenantObjectKey := s.tenantObjectKey(prefix, objectKey)
	u, err := client.PresignedPutObject(ctx, bucket, tenantObjectKey, 15*time.Minute)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *FileService) PresignDownload(ctx context.Context, tenantID, objectKey string) (string, error) {
	client, bucket, prefix, err := s.minioRouter.Resolve(ctx, tenantID)
	if err != nil {
		return "", err
	}
	tenantObjectKey := s.tenantObjectKey(prefix, objectKey)
	u, err := client.PresignedGetObject(ctx, bucket, tenantObjectKey, 15*time.Minute, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *FileService) RegisterAndMaybeThumbnail(ctx context.Context, item domain.FileObject) (domain.FileObject, error) {
	client, bucket, prefix, err := s.minioRouter.Resolve(ctx, item.TenantID)
	if err != nil {
		return domain.FileObject{}, err
	}
	item.ObjectKey = s.tenantObjectKey(prefix, item.ObjectKey)
	if strings.HasPrefix(item.ContentType, "image/") {
		thumbKey, err := s.makeThumbnail(ctx, client, bucket, item.ObjectKey)
		if err == nil {
			item.ThumbnailKey = thumbKey
		}
	}
	return s.dbman.CreateFile(ctx, item)
}

func (s *FileService) makeThumbnail(ctx context.Context, client *minio.Client, bucket, objectKey string) (string, error) {
	obj, err := client.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	defer obj.Close()

	img, _, err := image.Decode(obj)
	if err != nil {
		return "", err
	}

	thumb := imaging.Thumbnail(img, 320, 320, imaging.Lanczos)
	buf := bytes.NewBuffer(nil)
	if err := imaging.Encode(buf, thumb, imaging.JPEG); err != nil {
		return "", err
	}

	ext := filepath.Ext(objectKey)
	thumbKey := strings.TrimSuffix(objectKey, ext) + "_thumb.jpg"
	reader := bytes.NewReader(buf.Bytes())
	_, err = client.PutObject(ctx, bucket, thumbKey, reader, int64(reader.Len()), minio.PutObjectOptions{ContentType: "image/jpeg"})
	if err != nil {
		return "", fmt.Errorf("upload thumb: %w", err)
	}
	return thumbKey, nil
}

func (s *FileService) tenantObjectKey(prefix, objectKey string) string {
	cleaned := strings.TrimPrefix(strings.TrimSpace(objectKey), "/")
	normalizedPrefix := strings.TrimSpace(prefix)
	if normalizedPrefix == "" {
		return cleaned
	}
	normalizedPrefix = strings.TrimPrefix(normalizedPrefix, "/")
	if !strings.HasSuffix(normalizedPrefix, "/") {
		normalizedPrefix += "/"
	}
	if strings.HasPrefix(cleaned, normalizedPrefix) {
		return cleaned
	}
	return normalizedPrefix + cleaned
}
