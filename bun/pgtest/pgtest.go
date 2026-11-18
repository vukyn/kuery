// Package pgtest gives tests a real, isolated Postgres database per call.
//
// One container is started lazily per test binary (testcontainers-go). The
// caller's schema is migrated ONCE into a template database; every DB call then
// clones that template with CREATE DATABASE … TEMPLATE, which is a file copy and
// far cheaper than re-running the migrations. Each clone is dropped when the
// test that asked for it finishes, so tests never share rows.
//
// Test-only: it needs a reachable Docker daemon. A missing daemon FAILS the
// test rather than skipping it, because a skipped database test reads as a
// passing one. Run with -short to skip on purpose. Nothing here reads .env or
// DB_* variables — the connection always points at the container it started.
package pgtest

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcPostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/uptrace/bun"

	kueryDb "github.com/vukyn/kuery/bun/db"
)

// DefaultImage is the Postgres image used when Options.Image is empty.
const DefaultImage = "postgres:16-alpine"

// templateDatabase holds the migrated schema every DB clone is copied from.
const templateDatabase = "pgtest_template"

// Container credentials. They only ever reach a throwaway container bound to a
// random host port, so fixed values are fine.
const (
	containerUser     = "pgtest"
	containerPassword = "pgtest"
	adminDatabase     = "postgres"
)

// maxIdentifierLength is Postgres' NAMEDATALEN-1; longer names are truncated by
// the server, which could make two clones collide.
const maxIdentifierLength = 63

// startupTimeout bounds pulling the image and waiting for the server to accept
// connections.
const startupTimeout = 3 * time.Minute

// Options configures the shared container and template. Set them through Run
// from TestMain; without Run the zero value applies (default image, no
// migration, so DB returns an empty database like EmptyDB).
type Options struct {
	// Image is the Postgres image; DefaultImage when empty.
	Image string
	// Migrate builds the schema into the template database. It runs at most
	// once per process, on the first DB call. Nil leaves the template empty.
	Migrate func(*bun.DB) error
}

// server is the process-wide state: the container, an admin connection used
// for CREATE/DROP DATABASE, and the lazily built template.
type server struct {
	mu sync.Mutex // serializes CREATE DATABASE; a template cannot be copied twice at once

	options Options

	startOnce sync.Once
	startErr  error
	container *tcPostgres.PostgresContainer
	host      string
	port      int
	admin     *bun.DB

	templateOnce sync.Once
	templateErr  error

	counter int
}

var shared = &server{}

// Run is meant to be called from TestMain: it records the options, runs the
// tests and terminates the container afterwards. Its result is the exit code
// to pass to os.Exit. A binary that never calls Run still works — the
// testcontainers reaper removes the container when the process exits.
func Run(m *testing.M, options Options) int {
	shared.options = options
	code := m.Run()
	if err := shared.stop(); err != nil {
		fmt.Printf("pgtest: stop container: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

// DB returns a fresh database cloned from the migrated template. It is
// dropped, with every connection to it, when the test finishes.
func DB(t testing.TB) *bun.DB {
	t.Helper()
	srv := shared.ready(t)
	srv.templateOnce.Do(func() { srv.templateErr = srv.buildTemplate() })
	if srv.templateErr != nil {
		t.Fatalf("pgtest: build template database: %v", srv.templateErr)
	}
	return srv.newDatabase(t, templateDatabase)
}

// EmptyDB returns a fresh database with no schema at all — Options.Migrate is
// not applied. Useful for testing the migrations themselves.
func EmptyDB(t testing.TB) *bun.DB {
	t.Helper()
	return shared.ready(t).newDatabase(t, "template0")
}

// ready skips under -short and fails the test when the container cannot start.
func (srv *server) ready(t testing.TB) *server {
	t.Helper()
	if testing.Short() {
		t.Skip("pgtest: Postgres container skipped in -short mode")
	}
	srv.startOnce.Do(func() { srv.startErr = srv.start() })
	if srv.startErr != nil {
		t.Fatalf("pgtest: start Postgres container (is Docker running?): %v", srv.startErr)
	}
	return srv
}

func (srv *server) start() (err error) {
	// Some Docker discovery paths panic instead of returning an error; a panic
	// here would abort the whole binary rather than fail the test.
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("docker unavailable: %v", recovered)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	defer cancel()

	image := srv.options.Image
	if image == "" {
		image = DefaultImage
	}
	container, err := tcPostgres.Run(ctx, image,
		tcPostgres.WithDatabase(adminDatabase),
		tcPostgres.WithUsername(containerUser),
		tcPostgres.WithPassword(containerPassword),
		tcPostgres.BasicWaitStrategies(),
	)
	if container != nil {
		srv.container = container
	}
	if err != nil {
		return err
	}

	host, err := container.Host(ctx)
	if err != nil {
		return fmt.Errorf("container host: %w", err)
	}
	mappedPort, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return fmt.Errorf("container port: %w", err)
	}
	srv.host = host
	srv.port = int(mappedPort.Num())

	srv.admin, err = srv.open(adminDatabase)
	if err != nil {
		return err
	}
	if err := srv.admin.PingContext(ctx); err != nil {
		return fmt.Errorf("ping admin database: %w", err)
	}
	return nil
}

func (srv *server) stop() error {
	if srv.admin != nil {
		_ = srv.admin.Close()
	}
	if srv.container == nil {
		return nil
	}
	return testcontainers.TerminateContainer(srv.container)
}

// buildTemplate creates the template database, migrates it and closes every
// connection to it — Postgres refuses to copy a template that is in use.
func (srv *server) buildTemplate() error {
	ctx := context.Background()
	if _, err := srv.admin.ExecContext(ctx, "CREATE DATABASE "+quoteIdentifier(templateDatabase)); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	if srv.options.Migrate == nil {
		return nil
	}

	template, err := srv.open(templateDatabase)
	if err != nil {
		return err
	}
	migrateErr := srv.options.Migrate(template)
	closeErr := template.Close()
	if migrateErr != nil {
		return fmt.Errorf("migrate: %w", migrateErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close: %w", closeErr)
	}
	return nil
}

// newDatabase copies source into a database named after the test and registers
// its removal.
func (srv *server) newDatabase(t testing.TB, source string) *bun.DB {
	t.Helper()
	ctx := context.Background()

	srv.mu.Lock()
	srv.counter++
	name := databaseName(t.Name(), srv.counter)
	_, err := srv.admin.ExecContext(ctx,
		"CREATE DATABASE "+quoteIdentifier(name)+" TEMPLATE "+quoteIdentifier(source))
	srv.mu.Unlock()
	if err != nil {
		t.Fatalf("pgtest: create database %s: %v", name, err)
	}

	database, err := srv.open(name)
	if err != nil {
		t.Fatalf("pgtest: open database %s: %v", name, err)
	}
	t.Cleanup(func() {
		_ = database.Close()
		if _, err := srv.admin.ExecContext(context.Background(),
			"DROP DATABASE IF EXISTS "+quoteIdentifier(name)+" WITH (FORCE)"); err != nil {
			t.Errorf("pgtest: drop database %s: %v", name, err)
		}
	})
	return database
}

func (srv *server) open(name string) (*bun.DB, error) {
	database, err := kueryDb.Open(kueryDb.Config{
		Driver:   kueryDb.DriverPostgres,
		Host:     srv.host,
		Port:     srv.port,
		User:     containerUser,
		Password: containerPassword,
		DBName:   name,
		SSLMode:  "disable",
	})
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	return database, nil
}

// databaseName turns a test name into a valid, unique database name: lowercase
// [a-z0-9_] only, suffixed with a per-process counter, at most 63 bytes.
func databaseName(testName string, counter int) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(testName) {
		switch {
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			builder.WriteRune(character)
		default:
			builder.WriteByte('_')
		}
	}
	suffix := "_" + strconv.Itoa(counter)
	base := "t_" + builder.String()
	if len(base)+len(suffix) > maxIdentifierLength {
		base = base[:maxIdentifierLength-len(suffix)]
	}
	return base + suffix
}

// quoteIdentifier double-quotes a Postgres identifier.
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
