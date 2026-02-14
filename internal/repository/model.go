package repository

// CIProject is the DB model for ci_projects table.
// DDL: id serial4, service_name varchar(50) UNIQUE, repo_url text NOT NULL, branch varchar(50) DEFAULT 'main', registry_url text.
type CIProject struct {
	ID          uint   `gorm:"primaryKey"`
	ServiceName string `gorm:"column:service_name;size:50;uniqueIndex:ci_projects_service_name_key"`
	RepoURL     string `gorm:"column:repo_url;not null"`
	Branch      string `gorm:"column:branch;size:50;default:main"`
	RegistryURL string `gorm:"column:registry_url"`
}

// TableName overrides the table name for GORM.
func (CIProject) TableName() string {
	return "ci_projects"
}

// CISecret is the DB model for ci_secrets table.
// DDL: id serial4, key varchar(50) UNIQUE, value text NOT NULL, description text NOT NULL.
type CISecret struct {
	ID          uint   `gorm:"primaryKey"`
	Key         string `gorm:"column:key;size:50;uniqueIndex:ci_secrets_key_key"`
	Value       string `gorm:"column:value;not null"`
	Description string `gorm:"column:description;not null"`
}

// TableName overrides the table name for GORM.
func (CISecret) TableName() string {
	return "ci_secrets"
}
