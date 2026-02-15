package repository

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("repository: not found")

// Repository provides PostgreSQL-based access to sources, projects, and secrets.
type Repository struct {
	db *gorm.DB
}

// New creates a Repository. AutoMigrate creates sources, projects, secrets if not exist.
func New(db *gorm.DB) (*Repository, error) {
	if err := db.AutoMigrate(&Source{}, &Project{}, &Secret{}); err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}

// --- Sources ---

// GetSourceByName returns a source by name. Returns ErrNotFound when no row exists.
func (r *Repository) GetSourceByName(ctx context.Context, name string) (*Source, error) {
	var s Source
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

// CreateSource inserts a sources row. Name must be unique.
func (r *Repository) CreateSource(ctx context.Context, s *Source) error {
	return r.db.WithContext(ctx).Create(s).Error
}

// --- Projects ---

// GetProjectByMainRepoURL returns a project by main_repo_url (exact or normalized).
// Returns ErrNotFound when no row exists.
func (r *Repository) GetProjectByMainRepoURL(ctx context.Context, mainRepoURL string) (*Project, error) {
	normalized := normalizeRepoURL(mainRepoURL)
	for _, url := range []string{mainRepoURL, normalized} {
		if url == "" {
			continue
		}
		var p Project
		err := r.db.WithContext(ctx).Where("main_repo_url = ? AND is_active = ?", url, true).First(&p).Error
		if err == nil {
			return &p, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	return nil, ErrNotFound
}

func normalizeRepoURL(url string) string {
	u := strings.TrimSpace(url)
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimSuffix(u, ".git")
	return u
}

// GetProjectByProjectName returns a project by project_name. Returns ErrNotFound when no row exists.
func (r *Repository) GetProjectByProjectName(ctx context.Context, projectName string) (*Project, error) {
	var p Project
	err := r.db.WithContext(ctx).Where("project_name = ? AND is_active = ?", projectName, true).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// CreateProject inserts a projects row. ProjectName must be unique.
func (r *Repository) CreateProject(ctx context.Context, p *Project) error {
	return r.db.WithContext(ctx).Create(p).Error
}

// --- Secrets ---

// GetSecretsByProjectID returns all secrets for a project (optionally filtered by scope).
func (r *Repository) GetSecretsByProjectID(ctx context.Context, projectID int, scope string) ([]Secret, error) {
	var secrets []Secret
	q := r.db.WithContext(ctx).Where("project_id = ?", projectID)
	if scope != "" {
		q = q.Where("scope = ?", scope)
	}
	err := q.Find(&secrets).Error
	return secrets, err
}

// GetSecretValue returns the secret value by project_id, key, and scope.
// Returns empty string and ErrNotFound when no row exists.
func (r *Repository) GetSecretValue(ctx context.Context, projectID int, key, scope string) (string, error) {
	var s Secret
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND secret_key = ? AND scope = ?", projectID, key, scope).
		First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	return s.SecretValue, nil
}

// CreateSecret inserts a secrets row. UNIQUE(project_id, secret_key, scope).
func (r *Repository) CreateSecret(ctx context.Context, s *Secret) error {
	return r.db.WithContext(ctx).Create(s).Error
}

// BranchMatchesTargetBranches returns true if branch matches any pattern in targetBranches.
// Patterns support regex (e.g. ^feature/.*) and literal (e.g. main).
func BranchMatchesTargetBranches(branch string, targetBranches pq.StringArray) bool {
	if len(targetBranches) == 0 {
		return false
	}
	for _, pat := range targetBranches {
		if matchBranch(branch, strings.TrimSpace(pat)) {
			return true
		}
	}
	return false
}

// matchBranch returns true if branch matches pattern. Pattern can be literal or regex.
func matchBranch(branch, pattern string) bool {
	if pattern == "" {
		return false
	}
	// Literal match
	if branch == pattern {
		return true
	}
	// Regex match (Python re style)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(branch)
}
