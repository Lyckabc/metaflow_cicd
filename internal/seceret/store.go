package secret

import "context"

// SecretStore는 향후 Infisical 도입 시에도 동일한 메서드를 사용합니다.
type SecretStore interface {
	GetSecret(ctx context.Context, key string) (string, error)
}

// PostgresSecretStore는 현재 사용할 과도기적 구현체입니다.
type PostgresSecretStore struct {
	dbURL string
	// 여기에 sql.DB 객체를 보관하여 재사용 가능
}

func NewPostgresSecretStore(url string) *PostgresSecretStore {
	return &PostgresSecretStore{dbURL: url}
}

func (s *PostgresSecretStore) GetSecret(ctx context.Context, key string) (string, error) {
	// 실제 구현: DB에서 SELECT secret_value FROM secrets WHERE key = ? 실행
	// 현재는 암호화 없이 plain text를 가져오는 로직 작성
	return "dummy-secret-from-db", nil 
}