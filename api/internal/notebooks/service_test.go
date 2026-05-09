package notebooks

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexfortisme/relay/internal/store"
)

func newServiceTestRig(t *testing.T) (context.Context, *Service, *store.Store, *Registry) {
	t.Helper()

	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.New(filepath.Join(dir, "main.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	registry := NewRegistry(filepath.Join(dir, "notebooks"))
	t.Cleanup(registry.CloseAll)

	svc := NewService(st, registry, func(notebookID string) string {
		return filepath.Join(dir, "snapshots", notebookID)
	}, nil)

	now := time.Now()
	if _, err := st.CreateUser(ctx, "owner", "owner", "hash", now); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if _, err := st.CreateUser(ctx, "other", "other", "hash", now); err != nil {
		t.Fatalf("create other: %v", err)
	}

	return ctx, svc, st, registry
}

func TestUploadFileRequiresNotebookOwnership(t *testing.T) {
	ctx, svc, _, _ := newServiceTestRig(t)

	nb, err := svc.CreateNotebook(ctx, "owner", "Research", "", "", "")
	if err != nil {
		t.Fatalf("create notebook: %v", err)
	}

	_, err = svc.UploadFile(ctx, "other", nb.ID, "notes.txt", "text/plain", strings.NewReader("secret notes"))
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected not found for cross-user upload, got %v", err)
	}

	files, err := svc.ListFiles(ctx, "owner", nb.ID)
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected no files after rejected upload, got %d", len(files))
	}
}

func TestDeleteFilePurgesIndexedNotebookData(t *testing.T) {
	ctx, svc, st, registry := newServiceTestRig(t)

	nb, err := svc.CreateNotebook(ctx, "owner", "Research", "", "", "")
	if err != nil {
		t.Fatalf("create notebook: %v", err)
	}

	db, err := registry.Open("owner", nb.ID)
	if err != nil {
		t.Fatalf("open notebook db: %v", err)
	}
	nbStore := NewNotebookStore(db)

	docFile, err := svc.UploadFile(ctx, "owner", nb.ID, "notes.txt", "text/plain", strings.NewReader("alpha beta gamma"))
	if err != nil {
		t.Fatalf("upload doc file: %v", err)
	}
	if err := ChunkAndIndex(ctx, db, docFile.ID, docFile.Name, docFile.ContentType, []byte("alpha beta gamma"), nil, nil); err != nil {
		t.Fatalf("index doc file: %v", err)
	}
	chunks, err := nbStore.SearchChunks(ctx, "alpha", 10)
	if err != nil {
		t.Fatalf("search indexed chunks: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected indexed chunks before delete")
	}

	if err := svc.DeleteFile(ctx, "owner", nb.ID, docFile.ID); err != nil {
		t.Fatalf("delete doc file: %v", err)
	}
	chunks, err = nbStore.SearchChunks(ctx, "alpha", 10)
	if err != nil {
		t.Fatalf("search after delete: %v", err)
	}
	if len(chunks) != 0 {
		t.Fatalf("expected chunks to be purged, got %d", len(chunks))
	}
	if _, err := st.GetNotebookFile(ctx, nb.ID, docFile.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected main file row to be deleted, got %v", err)
	}

	csvFile, err := svc.UploadFile(ctx, "owner", nb.ID, "people.csv", "text/csv", strings.NewReader("name\nAda\n"))
	if err != nil {
		t.Fatalf("upload csv file: %v", err)
	}
	tableName := CSVTableName(csvFile.ID)
	if _, _, err := ImportCSV(ctx, db, csvFile.ID, tableName, csvFile.Name, []byte("name\nAda\n")); err != nil {
		t.Fatalf("import csv: %v", err)
	}
	if _, err := nbStore.GetCSVTableMeta(ctx, csvFile.ID); err != nil {
		t.Fatalf("expected csv metadata before delete: %v", err)
	}

	if err := svc.DeleteFile(ctx, "owner", nb.ID, csvFile.ID); err != nil {
		t.Fatalf("delete csv file: %v", err)
	}
	if _, err := nbStore.GetCSVTableMeta(ctx, csvFile.ID); err == nil {
		t.Fatal("expected csv metadata to be purged")
	}

	var tableCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`,
		tableName,
	).Scan(&tableCount); err != nil {
		t.Fatalf("check csv table: %v", err)
	}
	if tableCount != 0 {
		t.Fatalf("expected csv table to be dropped, got count %d", tableCount)
	}
}
