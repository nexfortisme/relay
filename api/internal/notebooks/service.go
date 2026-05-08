package notebooks

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nexfortisme/relay/internal/llm"
	"github.com/nexfortisme/relay/internal/store"
	"github.com/nexfortisme/relay/internal/tools"
)

const (
	schedulerInterval = 10 * time.Second
	workerConcurrency = 3
	maxSnapshots      = 5
)

// Service manages notebooks: background job processing and CRUD operations.
type Service struct {
	mainStore       *store.Store
	registry        *Registry
	defaultLLMURL   string
	defaultLLMModel string
	snapshotDir     func(notebookID string) string
	logger          *slog.Logger
	sem             chan struct{}
}

func NewService(
	st *store.Store,
	registry *Registry,
	snapshotDirFn func(string) string,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		mainStore:   st,
		registry:    registry,
		snapshotDir: snapshotDirFn,
		logger:      logger,
		sem:         make(chan struct{}, workerConcurrency),
	}
}

// WithLLMDefaults sets the fallback LLM endpoint used for image descriptions
// when the user has not overridden their settings. These values come from the
// deployment config (.env); user settings in the DB take precedence at job
// processing time.
func (s *Service) WithLLMDefaults(url, model string) {
	s.defaultLLMURL = url
	s.defaultLLMModel = model
}

// llmProviderForUser builds an LLM provider using the user's stored settings,
// falling back to the deployment defaults when a setting is absent.
func (s *Service) llmProviderForUser(ctx context.Context, userID string) llm.Provider {
	llmURL := s.settingOrDefault(ctx, userID, "llm_url", s.defaultLLMURL)
	llmModel := s.settingOrDefault(ctx, userID, "llm_model", s.defaultLLMModel)
	llmAPIKey := s.settingOrDefault(ctx, userID, "llm_api_key", "")
	return llm.NewHTTPProvider(llmURL, llmModel, llmAPIKey, 5*time.Minute)
}

func (s *Service) settingOrDefault(ctx context.Context, userID, key, defaultVal string) string {
	val, ok, err := s.mainStore.GetSetting(ctx, userID, key)
	if err != nil || !ok || val == "" {
		return defaultVal
	}
	return val
}

// describePageImage calls the LLM with a rendered JPEG page and returns a
// textual description suitable for appending to the indexed page content.
func describePageImage(ctx context.Context, provider llm.Provider, jpegBytes []byte) (string, error) {
	b64 := base64.StdEncoding.EncodeToString(jpegBytes)
	prompt := "Describe what is shown in this PDF page image. Be concise and factual.\n\n" +
		"![page](data:image/jpeg;base64," + b64 + ")"
	msgs := []llm.ChatMessage{{Role: "user", Content: llm.ParseContent(prompt)}}
	ch := provider.GenerateStream(ctx, msgs, tools.NoopRuntime{})
	var sb strings.Builder
	for ev := range ch {
		if ev.Err != nil {
			return sb.String(), ev.Err
		}
		sb.WriteString(ev.Token)
	}
	return sb.String(), nil
}

// Start launches the background job processor goroutine.
func (s *Service) Start(ctx context.Context) {
	go s.processLoop(ctx)
}

func (s *Service) processLoop(ctx context.Context) {
	ticker := time.NewTicker(schedulerInterval)
	defer ticker.Stop()
	s.runPendingJobs(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runPendingJobs(ctx)
		}
	}
}

func (s *Service) runPendingJobs(ctx context.Context) {
	jobs, err := s.mainStore.ListPendingNotebookJobs(ctx, 10)
	if err != nil {
		s.logger.Error("list pending notebook jobs", "error", err)
		return
	}
	for _, job := range jobs {
		select {
		case s.sem <- struct{}{}:
			go func(j store.NotebookJob) {
				defer func() { <-s.sem }()
				s.processJob(ctx, j)
			}(job)
		default:
			// semaphore full; try next tick
		}
	}
}

func (s *Service) processJob(ctx context.Context, job store.NotebookJob) {
	logger := s.logger.With("job_id", job.ID, "file_id", job.FileID)

	if err := s.mainStore.MarkJobRunning(ctx, job.ID); err != nil {
		logger.Error("mark job running", "error", err)
		return
	}
	_ = s.mainStore.SetNotebookFileStatus(ctx, job.FileID, "processing", "")

	f, err := s.mainStore.GetNotebookFile(ctx, job.NotebookID, job.FileID)
	if err != nil {
		s.failJob(ctx, job.ID, job.FileID, fmt.Sprintf("get notebook file: %v", err))
		return
	}

	data, err := s.mainStore.GetNotebookFileData(ctx, job.FileID)
	if err != nil {
		s.failJob(ctx, job.ID, job.FileID, fmt.Sprintf("get file data: %v", err))
		return
	}

	db, err := s.registry.Open(job.UserID, job.NotebookID)
	if err != nil {
		s.failJob(ctx, job.ID, job.FileID, fmt.Sprintf("open notebook db: %v", err))
		return
	}

	nbStore := NewNotebookStore(db)

	var describeImage ImageDescriber
	if s.defaultLLMURL != "" {
		provider := s.llmProviderForUser(ctx, job.UserID)
		describeImage = func(ctx context.Context, jpegBytes []byte) (string, error) {
			return describePageImage(ctx, provider, jpegBytes)
		}
	}

	var onProgress ProgressFunc = func(pagesIndexed, pageCount int) {
		_ = s.mainStore.SetNotebookFileProgress(ctx, job.FileID, pagesIndexed, pageCount)
	}

	switch f.FileKind {
	case "csv":
		tableName := CSVTableName(f.ID)
		_, _, err = ImportCSV(ctx, db, f.ID, tableName, f.Name, data)
	case "image":
		err = nbStore.InsertImageMeta(ctx, f.ID, f.Name, f.ContentType, f.SizeBytes)
	default: // document: pdf, md, txt, json, yaml
		err = ChunkAndIndex(ctx, db, f.ID, f.Name, f.ContentType, data, describeImage, onProgress)
	}

	if err != nil {
		logger.Error("process notebook file", "file_kind", f.FileKind, "error", err)
		s.failJob(ctx, job.ID, job.FileID, err.Error())
		return
	}

	if err := s.mainStore.MarkJobDone(ctx, job.ID); err != nil {
		logger.Error("mark job done", "error", err)
	}
	if err := s.mainStore.SetNotebookFileStatus(ctx, job.FileID, "ready", ""); err != nil {
		logger.Error("set file status ready", "error", err)
	}
	logger.Info("notebook file processed", "file_kind", f.FileKind, "name", f.Name)
}

func (s *Service) failJob(ctx context.Context, jobID, fileID, errText string) {
	_ = s.mainStore.MarkJobError(ctx, jobID, errText)
	_ = s.mainStore.SetNotebookFileStatus(ctx, fileID, "error", errText)
}

// ---------- CRUD ----------

// CreateNotebook creates a new notebook record.
func (s *Service) CreateNotebook(ctx context.Context, userID, name, description, systemPrompt, skillPrompt string) (store.Notebook, error) {
	return s.mainStore.CreateNotebook(ctx, userID, name, description, systemPrompt, skillPrompt)
}

// GetNotebook returns a notebook owned by userID.
func (s *Service) GetNotebook(ctx context.Context, userID, notebookID string) (store.Notebook, error) {
	return s.mainStore.GetNotebook(ctx, userID, notebookID)
}

// ListNotebooks returns all notebooks for a user.
func (s *Service) ListNotebooks(ctx context.Context, userID string) ([]store.Notebook, error) {
	return s.mainStore.ListNotebooks(ctx, userID)
}

// UpdateNotebook applies a partial update.
func (s *Service) UpdateNotebook(ctx context.Context, userID, notebookID string, patch store.NotebookPatch) (store.Notebook, error) {
	return s.mainStore.UpdateNotebook(ctx, userID, notebookID, patch)
}

// DeleteNotebook removes the notebook and its per-notebook DB file.
func (s *Service) DeleteNotebook(ctx context.Context, userID, notebookID string) error {
	if err := s.mainStore.DeleteNotebook(ctx, userID, notebookID); err != nil {
		return err
	}
	// Close and remove the per-notebook DB
	s.registry.CloseNotebook(userID, notebookID)
	dbPath := s.registry.DBPath(userID, notebookID)
	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	return nil
}

// ---------- Files ----------

// UploadFile reads r, stores the data as a blob, and enqueues a processing job.
func (s *Service) UploadFile(ctx context.Context, userID, notebookID, name, contentType string, r io.Reader) (store.NotebookFile, error) {
	if _, err := s.mainStore.GetNotebook(ctx, userID, notebookID); err != nil {
		return store.NotebookFile{}, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return store.NotebookFile{}, fmt.Errorf("read file: %w", err)
	}

	fileKind := fileKindFor(name, contentType)

	f := store.NotebookFile{
		NotebookID:  notebookID,
		UserID:      userID,
		Name:        name,
		ContentType: contentType,
		SizeBytes:   int64(len(data)),
		FileKind:    fileKind,
	}

	created, err := s.mainStore.CreateNotebookFile(ctx, f, data)
	if err != nil {
		return store.NotebookFile{}, err
	}

	if _, err := s.mainStore.EnqueueNotebookJob(ctx, notebookID, created.ID, userID); err != nil {
		_ = s.mainStore.DeleteNotebookFile(ctx, notebookID, created.ID)
		return store.NotebookFile{}, fmt.Errorf("enqueue job: %w", err)
	}

	return created, nil
}

// ListFiles returns all files for a notebook.
func (s *Service) ListFiles(ctx context.Context, userID, notebookID string) ([]store.NotebookFile, error) {
	// Verify ownership
	if _, err := s.mainStore.GetNotebook(ctx, userID, notebookID); err != nil {
		return nil, err
	}
	return s.mainStore.ListNotebookFiles(ctx, notebookID)
}

// DeleteFile removes a notebook file and its data from the per-notebook DB.
func (s *Service) DeleteFile(ctx context.Context, userID, notebookID, fileID string) error {
	if _, err := s.mainStore.GetNotebook(ctx, userID, notebookID); err != nil {
		return err
	}
	if _, err := s.mainStore.GetNotebookFile(ctx, notebookID, fileID); err != nil {
		return err
	}
	if err := s.purgeIndexedFileData(ctx, userID, notebookID, fileID); err != nil {
		return err
	}
	return s.mainStore.DeleteNotebookFile(ctx, notebookID, fileID)
}

func (s *Service) purgeIndexedFileData(ctx context.Context, userID, notebookID, fileID string) error {
	db, err := s.registry.Open(userID, notebookID)
	if err != nil {
		return fmt.Errorf("open notebook db: %w", err)
	}
	if err := NewNotebookStore(db).PurgeFileData(ctx, fileID); err != nil {
		return fmt.Errorf("purge indexed file data: %w", err)
	}
	return nil
}

// GetPageImage returns the rendered JPEG bytes (and content-type) for a single PDF page.
// Returns store.ErrNotFound when the notebook, file, or page does not exist, or when
// no image was stored for that page (e.g. text-only indexed file).
func (s *Service) GetPageImage(ctx context.Context, userID, notebookID, fileID string, pageNum int) ([]byte, string, error) {
	if _, err := s.mainStore.GetNotebook(ctx, userID, notebookID); err != nil {
		return nil, "", err
	}
	db, err := s.registry.Open(userID, notebookID)
	if err != nil {
		return nil, "", fmt.Errorf("open notebook db: %w", err)
	}
	var imgData []byte
	var imgType string
	err = db.QueryRowContext(ctx,
		`SELECT image_data, image_type FROM pages WHERE file_id=? AND page_number=?`,
		fileID, pageNum,
	).Scan(&imgData, &imgType)
	if err != nil || len(imgData) == 0 {
		return nil, "", store.ErrNotFound
	}
	return imgData, imgType, nil
}

// PendingJobCount returns the number of pending/running jobs for a notebook.
func (s *Service) PendingJobCount(ctx context.Context, notebookID string) (int, error) {
	return s.mainStore.PendingJobCount(ctx, notebookID)
}

// GetFileData returns the raw bytes for a notebook file (for download).
func (s *Service) GetFileData(ctx context.Context, userID, notebookID, fileID string) ([]byte, string, string, error) {
	if _, err := s.mainStore.GetNotebook(ctx, userID, notebookID); err != nil {
		return nil, "", "", err
	}
	f, err := s.mainStore.GetNotebookFile(ctx, notebookID, fileID)
	if err != nil {
		return nil, "", "", err
	}
	data, err := s.mainStore.GetNotebookFileData(ctx, fileID)
	if err != nil {
		return nil, "", "", err
	}
	return data, f.ContentType, f.Name, nil
}

// ---------- RAG ----------

// RAGContext builds a context string to inject into the LLM prompt.
func (s *Service) RAGContext(ctx context.Context, userID, notebookID, query string) (string, error) {
	db, err := s.registry.Open(userID, notebookID)
	if err != nil {
		return "", fmt.Errorf("open notebook db: %w", err)
	}
	nbStore := NewNotebookStore(db)

	// Try FTS first; fall back to sample if no results
	chunks, err := nbStore.SearchChunks(ctx, query, 10)
	if err != nil || len(chunks) == 0 {
		chunks, _ = nbStore.SampleChunks(ctx, 5)
	}

	csvMetas, _ := nbStore.ListCSVTables(ctx)
	images, _ := nbStore.ListImageMeta(ctx)

	var sb strings.Builder
	sb.WriteString("[Notebook Context]\n")

	if len(chunks) > 0 {
		sb.WriteString("## Relevant document excerpts:\n")
		for _, c := range chunks {
			if c.PageNumber > 0 {
				sb.WriteString(fmt.Sprintf("[file_id:%s p.%d] %s\n\n", c.FileID, c.PageNumber, c.Content))
			} else {
				sb.WriteString(fmt.Sprintf("[file_id:%s] %s\n\n", c.FileID, c.Content))
			}
		}
	}

	if len(csvMetas) > 0 {
		sb.WriteString("## Available CSV tables:\n")
		for _, m := range csvMetas {
			sb.WriteString(fmt.Sprintf("- %s (%s): %s — %d rows\n",
				m.TableName, m.FileName, strings.Join(m.ColumnNames, ", "), m.RowCount))
		}
		sb.WriteString("\n")
	}

	if len(images) > 0 {
		sb.WriteString("## Images in notebook:\n")
		for _, img := range images {
			sb.WriteString(fmt.Sprintf("- %s (file_id: %s)\n", img.Name, img.FileID))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// ---------- CSV viewer ----------

// GetCSVTableData returns all columns and rows for the inline viewer.
func (s *Service) GetCSVTableData(ctx context.Context, userID, notebookID, fileID string) (columns []string, rows [][]string, err error) {
	if _, err := s.mainStore.GetNotebook(ctx, userID, notebookID); err != nil {
		return nil, nil, err
	}
	f, err := s.mainStore.GetNotebookFile(ctx, notebookID, fileID)
	if err != nil {
		return nil, nil, err
	}
	if f.FileKind != "csv" {
		return nil, nil, store.ErrNotFound
	}

	db, err := s.registry.Open(userID, notebookID)
	if err != nil {
		return nil, nil, err
	}
	nbStore := NewNotebookStore(db)
	meta, err := nbStore.GetCSVTableMeta(ctx, fileID)
	if err != nil {
		return nil, nil, fmt.Errorf("no csv data for file %s", fileID)
	}
	return nbStore.GetCSVTableData(ctx, meta.TableName)
}

// ---------- Snapshots ----------

// SnapshotBeforeDelete copies the per-notebook DB before a destructive operation.
func (s *Service) SnapshotBeforeDelete(userID, notebookID string) error {
	s.registry.CloseNotebook(userID, notebookID)
	src := s.registry.DBPath(userID, notebookID)
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil // no DB yet, nothing to snapshot
	}

	destDir := s.snapshotDir(notebookID)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}

	ts := time.Now().UTC().Format("20060102T150405Z")
	dest := filepath.Join(destDir, ts+".db")

	if err := copyFile(src, dest); err != nil {
		return fmt.Errorf("copy snapshot: %w", err)
	}

	return s.pruneSnapshots(notebookID)
}

func (s *Service) pruneSnapshots(notebookID string) error {
	dir := s.snapshotDir(notebookID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	type snap struct {
		path    string
		modTime time.Time
	}
	snaps := make([]snap, 0, len(entries))
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		snaps = append(snaps, snap{
			path:    filepath.Join(dir, e.Name()),
			modTime: info.ModTime(),
		})
	}

	// Sort newest first
	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].modTime.After(snaps[j].modTime)
	})

	// Remove oldest beyond maxSnapshots
	for i := maxSnapshots; i < len(snaps); i++ {
		_ = os.Remove(snaps[i].path)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// fileKindFor determines whether a file is a csv, image, or generic document.
func fileKindFor(name, contentType string) string {
	ext := strings.ToLower(name)
	if idx := strings.LastIndex(ext, "."); idx >= 0 {
		ext = ext[idx:]
	}

	if strings.HasPrefix(contentType, "image/") ||
		ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" {
		return "image"
	}
	if contentType == "text/csv" || ext == ".csv" {
		return "csv"
	}
	return "document"
}
