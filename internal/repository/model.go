package repository

import (
	"time"

	"github.com/lib/pq"
)

// Source is the DB model for sources table.
// 자산 및 인증 정보 관리 테이블
// DDL: id, name, type, host, port, username, password_text, access_token, webhook_secret, description, created_at
type Source struct {
	ID            uint       `gorm:"primaryKey"`
	Name          string     `gorm:"column:name;size:100;uniqueIndex;not null"` // 검색 및 참조용 고유 이름
	Type          string     `gorm:"column:type;size:20;not null"`               // 'git', 'db', 'api' 등
	Host          *string    `gorm:"column:host"`                                // Git URL 또는 DB Host
	Port          *int       `gorm:"column:port"`
	Username      *string    `gorm:"column:username;size:100"`
	PasswordText  *string    `gorm:"column:password_text"`   // 현재 평문 저장 (차후 Infisical 전환 예정)
	AccessToken   *string    `gorm:"column:access_token"`   // Git Personal Access Token 등
	WebhookSecret *string    `gorm:"column:webhook_secret"` // Convoy 페이로드 검증용 Secret Token
	Description   *string   `gorm:"column:description"`     // 소스에 대한 설명
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName overrides the table name for GORM.
func (Source) TableName() string {
	return "sources"
}

// Project is the DB model for projects table.
// 프로젝트 및 트리거 설정 테이블
// DDL: id, project_name, main_repo_url, target_branches, ci_source_name, ci_config_path, cd_source_name, cd_config_path, is_active, description, updated_at
type Project struct {
	ID             uint         `gorm:"primaryKey"`
	ProjectName    string       `gorm:"column:project_name;size:100;uniqueIndex;not null"`
	MainRepoURL    string       `gorm:"column:main_repo_url;not null"` // 웹훅이 발생하는 메인 Git 주소
	TargetBranches pq.StringArray `gorm:"column:target_branches;type:text[];not null"` // 예: ARRAY['main', '^feature/.*', 'release-v.*']
	CISourceName   *string      `gorm:"column:ci_source_name;size:100"` // REFERENCES sources(name)
	CIConfigPath   string       `gorm:"column:ci_config_path;not null"`  // 파일 이름 또는 상대 경로
	CDSourceName   *string      `gorm:"column:cd_source_name;size:100"` // REFERENCES sources(name)
	CDConfigPath   string       `gorm:"column:cd_config_path;not null"`
	IsActive       bool         `gorm:"column:is_active;default:true"`
	Description    *string      `gorm:"column:description"` // 프로젝트 목적 설명
	UpdatedAt      time.Time    `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName overrides the table name for GORM.
func (Project) TableName() string {
	return "projects"
}

// Secret is the DB model for secrets table.
// 파이프라인용 시크릿 정보 테이블
// DDL: id, project_id, secret_key, secret_value, scope, description, UNIQUE(project_id, secret_key, scope)
type Secret struct {
	ID          uint    `gorm:"primaryKey"`
	ProjectID   int     `gorm:"column:project_id;not null;uniqueIndex:idx_secrets_project_key_scope,priority:1"` // REFERENCES projects(id)
	SecretKey   string  `gorm:"column:secret_key;size:100;not null;uniqueIndex:idx_secrets_project_key_scope,priority:2"`
	SecretValue string  `gorm:"column:secret_value;not null"`                                                      // 현재 평문 저장
	Scope       string  `gorm:"column:scope;size:50;default:prod;uniqueIndex:idx_secrets_project_key_scope,priority:3"` // 'dev', 'prod', 'staging' 등
	Description *string `gorm:"column:description"`                                                                   // 시크릿 용도 설명
}

// TableName overrides the table name for GORM.
func (Secret) TableName() string {
	return "secrets"
}
