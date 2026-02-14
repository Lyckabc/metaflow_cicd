package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("repository: not found")

// Repository provides PostgreSQL-based access to ci_projects and ci_secrets.
type Repository struct {
	db *gorm.DB
}

// New creates a Repository. Tables ci_projects and ci_secrets must exist (e.g. created by DDL).
func New(db *gorm.DB) (*Repository, error) {
	return &Repository{db: db}, nil
}

// GetProjectByServiceName returns a project by ServiceName.
// Returns ErrNotFound when no row exists.
func (r *Repository) GetProjectByServiceName(ctx context.Context, serviceName string) (*CIProject, error) {
	var p CIProject
	err := r.db.WithContext(ctx).Where("service_name = ?", serviceName).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// GetSecretByKey returns the full secret record by Key.
// Use this when you need the record for future encrypt/decrypt or metadata.
// Returns ErrNotFound when no row exists.
func (r *Repository) GetSecretByKey(ctx context.Context, key string) (*CISecret, error) {
	var s CISecret
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

// GetSecretValue returns only the secret value by Key.
// 향후 암호화/복호화 확장 시 이 메서드에서 복호화 로직을 추가하면 됨.
// Returns empty string and ErrNotFound when no row exists.
func (r *Repository) GetSecretValue(ctx context.Context, key string) (string, error) {
	rec, err := r.GetSecretByKey(ctx, key)
	if err != nil {
		return "", err
	}
	// 현재는 Plain text 그대로 반환. 향후: return decrypt(rec.Value), nil
	return rec.Value, nil
}

// CreateProject inserts a ci_projects row. ServiceName must be unique.
func (r *Repository) CreateProject(ctx context.Context, p *CIProject) error {
	return r.db.WithContext(ctx).Create(p).Error
}

// CreateSecret inserts a ci_secrets row. Key must be unique.
func (r *Repository) CreateSecret(ctx context.Context, s *CISecret) error {
	return r.db.WithContext(ctx).Create(s).Error
}
