package notebooks

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

// Registry maintains a pool of open per-notebook SQLite connections.
// One *sql.DB per notebook; connections are lazy-opened and cached.
type Registry struct {
	dir string
	mu  sync.Mutex
	dbs map[string]*sql.DB // key: "{userID}_{notebookID}"
}

func NewRegistry(notebooksDir string) *Registry {
	return &Registry{
		dir: notebooksDir,
		dbs: make(map[string]*sql.DB),
	}
}

// Open returns the per-notebook DB, opening it if not already open.
// The file is stored at {dir}/{userID}_{notebookID}.db.
func (r *Registry) Open(userID, notebookID string) (*sql.DB, error) {
	key := userID + "_" + notebookID
	r.mu.Lock()
	defer r.mu.Unlock()

	if db, ok := r.dbs[key]; ok {
		return db, nil
	}

	if err := os.MkdirAll(r.dir, 0755); err != nil {
		return nil, fmt.Errorf("create notebook db dir: %w", err)
	}

	path := filepath.Join(r.dir, key+".db")
	db, err := sql.Open("sqlite", notebookDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open notebook db: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	if err := applySchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply notebook schema: %w", err)
	}
	if err := migrateSchema(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate notebook schema: %w", err)
	}

	r.dbs[key] = db
	return db, nil
}

// CloseNotebook closes the cached connection for a given notebook.
// It is re-opened automatically on the next Open call.
func (r *Registry) CloseNotebook(userID, notebookID string) {
	key := userID + "_" + notebookID
	r.mu.Lock()
	defer r.mu.Unlock()
	if db, ok := r.dbs[key]; ok {
		_ = db.Close()
		delete(r.dbs, key)
	}
}

// CloseAll closes all cached connections.
func (r *Registry) CloseAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, db := range r.dbs {
		_ = db.Close()
	}
	r.dbs = make(map[string]*sql.DB)
}

// DBPath returns the filesystem path for a user+notebook pair.
func (r *Registry) DBPath(userID, notebookID string) string {
	return filepath.Join(r.dir, userID+"_"+notebookID+".db")
}

// migrateSchema adds columns introduced after initial schema deployment.
// It is idempotent: each ALTER runs only if the column is absent.
func migrateSchema(ctx context.Context, db *sql.DB) error {
	var count int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('pages') WHERE name='image_data'`).Scan(&count)
	if count == 0 {
		if _, err := db.ExecContext(ctx, `ALTER TABLE pages ADD COLUMN image_data BLOB`); err != nil {
			return fmt.Errorf("add pages.image_data: %w", err)
		}
		if _, err := db.ExecContext(ctx, `ALTER TABLE pages ADD COLUMN image_type TEXT`); err != nil {
			return fmt.Errorf("add pages.image_type: %w", err)
		}
	}
	return nil
}

func applySchema(db *sql.DB) error {
	if _, err := db.Exec(notebookDBSchema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

func notebookDSN(path string) string {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep +
		"_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=synchronous(NORMAL)"
}
