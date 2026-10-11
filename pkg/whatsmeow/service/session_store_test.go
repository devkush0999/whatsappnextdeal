package whatsmeow_service

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	"go.mau.fi/whatsmeow/store/sqlstore"
	_ "modernc.org/sqlite"
)

func TestSessionStoreConcurrentRequestsReuseContainer(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "dbdata"), 0700); err != nil {
		t.Fatal(err)
	}
	shared := &sharedSessionStore{}
	// A configuration failure must not poison subsequent attempts.
	if _, err := shared.get(nil, &config.Config{PostgresAuthDB: "configured"}, dir); err == nil {
		t.Fatal("expected missing database error")
	}
	const clients = 12
	containers := make(chan *sqlstore.Container, clients)
	errors := make(chan error, clients)
	var wg sync.WaitGroup
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := shared.get(nil, &config.Config{}, dir)
			containers <- c
			errors <- err
		}()
	}
	wg.Wait()
	close(containers)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first *sqlstore.Container
	for c := range containers {
		if first == nil {
			first = c
		}
		if c != first {
			t.Fatal("concurrent requests opened separate session stores")
		}
	}
	if first == nil {
		t.Fatal("missing container")
	}
	defer first.Close()
	if _, err := first.GetAllDevices(t.Context()); err != nil {
		t.Fatalf("shared store is not usable: %v", err)
	}
}
