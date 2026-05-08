package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Notebook types

type Notebook struct {
	ID           string    `json:"id"`
	UserID       string    `json:"-"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	SystemPrompt string    `json:"systemPrompt"`
	SkillPrompt  string    `json:"skillPrompt"`
	PendingJobs  int       `json:"pendingJobs,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type NotebookFile struct {
	ID           string    `json:"id"`
	NotebookID   string    `json:"notebookId"`
	UserID       string    `json:"-"`
	Name         string    `json:"name"`
	ContentType  string    `json:"contentType"`
	SizeBytes    int64     `json:"sizeBytes"`
	FileKind     string    `json:"fileKind"` // document|csv|image
	Status       string    `json:"status"`   // pending|processing|ready|error
	ErrorText    string    `json:"error,omitempty"`
	PageCount    int       `json:"pageCount"`
	PagesIndexed int       `json:"pagesIndexed"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type NotebookJob struct {
	ID         string     `json:"id"`
	NotebookID string     `json:"notebookId"`
	FileID     string     `json:"fileId"`
	UserID     string     `json:"-"`
	Status     string     `json:"status"` // pending|running|done|error
	ErrorText  string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

type NotebookPatch struct {
	Name         *string
	Description  *string
	SystemPrompt *string
	SkillPrompt  *string
}

// Notebook CRUD

func (s *Store) CreateNotebook(ctx context.Context, userID, name, description, systemPrompt, skillPrompt string) (Notebook, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notebooks(id, user_id, name, description, system_prompt, skill_prompt, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		id, userID, name, description, systemPrompt, skillPrompt, now, now,
	)
	if err != nil {
		return Notebook{}, fmt.Errorf("create notebook: %w", err)
	}
	return Notebook{
		ID: id, UserID: userID, Name: name,
		Description: description, SystemPrompt: systemPrompt, SkillPrompt: skillPrompt,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s *Store) GetNotebook(ctx context.Context, userID, notebookID string) (Notebook, error) {
	var nb Notebook
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, description, system_prompt, skill_prompt, created_at, updated_at
		 FROM notebooks WHERE id = ? AND user_id = ?`,
		notebookID, userID,
	).Scan(&nb.ID, &nb.UserID, &nb.Name, &nb.Description, &nb.SystemPrompt, &nb.SkillPrompt, &nb.CreatedAt, &nb.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Notebook{}, ErrNotFound
	}
	if err != nil {
		return Notebook{}, fmt.Errorf("get notebook: %w", err)
	}
	return nb, nil
}

func (s *Store) ListNotebooks(ctx context.Context, userID string) ([]Notebook, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT n.id, n.user_id, n.name, n.description, n.system_prompt, n.skill_prompt, n.created_at, n.updated_at,
		        COALESCE((SELECT COUNT(*) FROM notebook_jobs j WHERE j.notebook_id = n.id AND j.status IN ('pending','running')), 0) AS pending_jobs
		 FROM notebooks n
		 WHERE n.user_id = ?
		 ORDER BY n.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list notebooks: %w", err)
	}
	defer rows.Close()

	out := make([]Notebook, 0)
	for rows.Next() {
		var nb Notebook
		if err := rows.Scan(&nb.ID, &nb.UserID, &nb.Name, &nb.Description, &nb.SystemPrompt, &nb.SkillPrompt,
			&nb.CreatedAt, &nb.UpdatedAt, &nb.PendingJobs); err != nil {
			return nil, fmt.Errorf("scan notebook: %w", err)
		}
		out = append(out, nb)
	}
	return out, rows.Err()
}

func (s *Store) UpdateNotebook(ctx context.Context, userID, notebookID string, patch NotebookPatch) (Notebook, error) {
	now := time.Now().UTC()
	nb, err := s.GetNotebook(ctx, userID, notebookID)
	if err != nil {
		return Notebook{}, err
	}
	if patch.Name != nil {
		nb.Name = *patch.Name
	}
	if patch.Description != nil {
		nb.Description = *patch.Description
	}
	if patch.SystemPrompt != nil {
		nb.SystemPrompt = *patch.SystemPrompt
	}
	if patch.SkillPrompt != nil {
		nb.SkillPrompt = *patch.SkillPrompt
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE notebooks SET name=?, description=?, system_prompt=?, skill_prompt=?, updated_at=? WHERE id=? AND user_id=?`,
		nb.Name, nb.Description, nb.SystemPrompt, nb.SkillPrompt, now, notebookID, userID,
	)
	if err != nil {
		return Notebook{}, fmt.Errorf("update notebook: %w", err)
	}
	nb.UpdatedAt = now
	return nb, nil
}

func (s *Store) DeleteNotebook(ctx context.Context, userID, notebookID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM notebooks WHERE id=? AND user_id=?`, notebookID, userID)
	if err != nil {
		return fmt.Errorf("delete notebook: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// NotebookFile CRUD

func (s *Store) CreateNotebookFile(ctx context.Context, f NotebookFile, data []byte) (NotebookFile, error) {
	now := time.Now().UTC()
	f.ID = uuid.NewString()
	f.CreatedAt = now
	f.UpdatedAt = now
	f.Status = "pending"

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return NotebookFile{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO notebook_files(id, notebook_id, user_id, name, content_type, size_bytes, file_kind, status, error_text, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.NotebookID, f.UserID, f.Name, f.ContentType, f.SizeBytes, f.FileKind, f.Status, f.ErrorText, f.CreatedAt, f.UpdatedAt,
	)
	if err != nil {
		return NotebookFile{}, fmt.Errorf("insert notebook_file: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO notebook_file_data(file_id, data) VALUES(?, ?)`, f.ID, data)
	if err != nil {
		return NotebookFile{}, fmt.Errorf("insert notebook_file_data: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return NotebookFile{}, fmt.Errorf("commit: %w", err)
	}
	return f, nil
}

func (s *Store) GetNotebookFile(ctx context.Context, notebookID, fileID string) (NotebookFile, error) {
	var f NotebookFile
	err := s.db.QueryRowContext(ctx,
		`SELECT id, notebook_id, user_id, name, content_type, size_bytes, file_kind, status, error_text, page_count, pages_indexed, created_at, updated_at
		 FROM notebook_files WHERE id=? AND notebook_id=?`,
		fileID, notebookID,
	).Scan(&f.ID, &f.NotebookID, &f.UserID, &f.Name, &f.ContentType, &f.SizeBytes,
		&f.FileKind, &f.Status, &f.ErrorText, &f.PageCount, &f.PagesIndexed, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return NotebookFile{}, ErrNotFound
	}
	if err != nil {
		return NotebookFile{}, fmt.Errorf("get notebook file: %w", err)
	}
	return f, nil
}

func (s *Store) GetNotebookFileData(ctx context.Context, fileID string) ([]byte, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT data FROM notebook_file_data WHERE file_id=?`, fileID).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get notebook file data: %w", err)
	}
	return data, nil
}

func (s *Store) ListNotebookFiles(ctx context.Context, notebookID string) ([]NotebookFile, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, notebook_id, user_id, name, content_type, size_bytes, file_kind, status, error_text, page_count, pages_indexed, created_at, updated_at
		 FROM notebook_files WHERE notebook_id=? ORDER BY created_at`,
		notebookID,
	)
	if err != nil {
		return nil, fmt.Errorf("list notebook files: %w", err)
	}
	defer rows.Close()

	out := make([]NotebookFile, 0)
	for rows.Next() {
		var f NotebookFile
		if err := rows.Scan(&f.ID, &f.NotebookID, &f.UserID, &f.Name, &f.ContentType, &f.SizeBytes,
			&f.FileKind, &f.Status, &f.ErrorText, &f.PageCount, &f.PagesIndexed, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan notebook file: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) DeleteNotebookFile(ctx context.Context, notebookID, fileID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM notebook_files WHERE id=? AND notebook_id=?`, fileID, notebookID)
	if err != nil {
		return fmt.Errorf("delete notebook file: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetNotebookFileStatus(ctx context.Context, fileID, status, errText string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE notebook_files SET status=?, error_text=?, updated_at=? WHERE id=?`,
		status, errText, time.Now().UTC(), fileID,
	)
	return err
}

func (s *Store) SetNotebookFileProgress(ctx context.Context, fileID string, pagesIndexed, pageCount int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE notebook_files SET pages_indexed=?, page_count=?, updated_at=? WHERE id=?`,
		pagesIndexed, pageCount, time.Now().UTC(), fileID,
	)
	return err
}

// NotebookJob methods

func (s *Store) EnqueueNotebookJob(ctx context.Context, notebookID, fileID, userID string) (NotebookJob, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notebook_jobs(id, notebook_id, file_id, user_id, status, created_at)
		 VALUES(?, ?, ?, ?, 'pending', ?)`,
		id, notebookID, fileID, userID, now,
	)
	if err != nil {
		return NotebookJob{}, fmt.Errorf("enqueue notebook job: %w", err)
	}
	return NotebookJob{
		ID: id, NotebookID: notebookID, FileID: fileID, UserID: userID,
		Status: "pending", CreatedAt: now,
	}, nil
}

func (s *Store) ListPendingNotebookJobs(ctx context.Context, limit int) ([]NotebookJob, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT j.id, j.notebook_id, j.file_id, j.user_id, j.status, j.error_text, j.created_at, j.started_at, j.finished_at
		 FROM notebook_jobs j
		 WHERE j.status = 'pending'
		 ORDER BY j.created_at
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending notebook jobs: %w", err)
	}
	defer rows.Close()

	out := make([]NotebookJob, 0)
	for rows.Next() {
		var j NotebookJob
		var startedAt, finishedAt sql.NullTime
		if err := rows.Scan(&j.ID, &j.NotebookID, &j.FileID, &j.UserID, &j.Status,
			&j.ErrorText, &j.CreatedAt, &startedAt, &finishedAt); err != nil {
			return nil, fmt.Errorf("scan notebook job: %w", err)
		}
		if startedAt.Valid {
			j.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			j.FinishedAt = &finishedAt.Time
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *Store) MarkJobRunning(ctx context.Context, jobID string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE notebook_jobs SET status='running', started_at=? WHERE id=?`, now, jobID)
	return err
}

func (s *Store) MarkJobDone(ctx context.Context, jobID string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE notebook_jobs SET status='done', finished_at=? WHERE id=?`, now, jobID)
	return err
}

func (s *Store) MarkJobError(ctx context.Context, jobID, errText string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE notebook_jobs SET status='error', error_text=?, finished_at=? WHERE id=?`,
		errText, now, jobID)
	return err
}

func (s *Store) PendingJobCount(ctx context.Context, notebookID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notebook_jobs WHERE notebook_id=? AND status IN ('pending','running')`,
		notebookID,
	).Scan(&count)
	return count, err
}

// Conversation notebook association

func (s *Store) GetConversationNotebookID(ctx context.Context, conversationID string) (string, error) {
	var notebookID string
	err := s.db.QueryRowContext(ctx,
		`SELECT notebook_id FROM conversations WHERE id=?`, conversationID).Scan(&notebookID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return notebookID, err
}

func (s *Store) SetConversationNotebookID(ctx context.Context, conversationID, notebookID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE conversations SET notebook_id=?, updated_at=? WHERE id=?`,
		notebookID, time.Now().UTC(), conversationID)
	return err
}
