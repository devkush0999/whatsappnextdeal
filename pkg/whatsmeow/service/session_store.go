package whatsmeow_service

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Reuse one store across clients and reconnects instead of opening a new pool
// for every QR request. The application owns and closes the PostgreSQL pool.
type sharedSessionStore struct {
	mu        sync.Mutex
	container *sqlstore.Container
}

func (s *sharedSessionStore) get(authDB *sql.DB, cfg *config.Config, executableDir string) (*sqlstore.Container, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.container != nil {
		return s.container, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var logger waLog.Logger
	if cfg.WaDebug != "" {
		logger = waLog.Stdout("Database", cfg.WaDebug, true)
	}
	var container *sqlstore.Container
	if cfg.PostgresAuthDB != "" {
		if authDB == nil {
			return nil, fmt.Errorf("PostgreSQL auth database is not initialized")
		}
		container = sqlstore.NewWithDB(authDB, "postgres", logger)
		if err := container.Upgrade(ctx); err != nil {
			return nil, err
		}
	} else {
		dsn := fmt.Sprintf("file:%s/dbdata/main.db?_pragma=foreign_keys(1)&_busy_timeout=5000&cache=shared&mode=rwc&_journal_mode=WAL", executableDir)
		var err error
		container, err = sqlstore.New(ctx, "sqlite", dsn, logger)
		if err != nil {
			return nil, err
		}
	}
	s.container = container
	return container, nil
}
