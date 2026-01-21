package pgfile

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ChunkSize = 1024 * 1024 // 1MB

type FileMetadata struct {
	ID        uuid.UUID
	Name      string
	Path      string
	GroupID   uuid.UUID
	UserID    uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Storage struct {
	pool *pgxpool.Pool
}

func NewStorage(pool *pgxpool.Pool) *Storage {
	return &Storage{pool: pool}
}

// Upload stores a file in PostgreSQL, chunking it into 1MB pieces.
func (s *Storage) Upload(ctx context.Context, name, path string, groupID, userID uuid.UUID, r io.Reader) (*FileMetadata, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	fileID := uuid.New()
	now := time.Now()

	_, err = tx.Exec(ctx, `
		INSERT INTO files (id, name, path, group_id, owner_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		fileID, name, path, groupID, userID, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert file metadata: %w", err)
	}

	src := &ReaderSource{
		Reader:   r,
		Buffer:   make([]byte, ChunkSize),
		FileID:   fileID,
		Sequence: 0,
	}

	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"file_chunks"},
		[]string{"file_id", "sequence", "data"},
		src,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file chunks: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &FileMetadata{
		ID:        fileID,
		Name:      name,
		Path:      path,
		GroupID:   groupID,
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// ReaderSource implements pgx.CopyFromSource to stream data from an io.Reader
type ReaderSource struct {
	Reader   io.Reader
	Buffer   []byte
	FileID   uuid.UUID
	Sequence int

	lastRead int
	err      error
}

func (rs *ReaderSource) Next() bool {
	if rs.err != nil {
		return false
	}
	rs.lastRead, rs.err = rs.Reader.Read(rs.Buffer)
	return rs.lastRead > 0
}

func (rs *ReaderSource) Values() ([]any, error) {
	// Zero-copy: return the slice of the buffer directly.
	// pgx.CopyFrom encodes the values before calling Next(), so this is safe.
	chunk := rs.Buffer[:rs.lastRead]

	values := []any{rs.FileID, rs.Sequence, chunk}
	rs.Sequence++

	return values, nil
}

func (rs *ReaderSource) Err() error {
	if rs.err == io.EOF {
		return nil
	}
	return rs.err
}

// Download retrieves a file from PostgreSQL and writes it to the provided io.Writer.
func (s *Storage) Download(ctx context.Context, fileID, seekerID uuid.UUID, seekerGroupID uuid.UUID, w io.Writer) error {
	// Check habilitation
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM files 
			WHERE id = $1 AND (owner_id = $2 OR group_id = $3)
		)`, fileID, seekerID, seekerGroupID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check habilitation: %w", err)
	}
	if !exists {
		return fmt.Errorf("access denied")
	}

	rows, err := s.pool.Query(ctx, "SELECT data FROM file_chunks WHERE file_id = $1 ORDER BY sequence ASC", fileID)
	if err != nil {
		return fmt.Errorf("failed to query file chunks: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var data []byte

		// Optimization: Use RawValues for zero-copy if the format is binary.
		// Format code 1 is Binary.
		if len(rows.FieldDescriptions()) > 0 && rows.FieldDescriptions()[0].Format == 1 {
			raw := rows.RawValues()
			if len(raw) > 0 {
				data = raw[0]
			}
		} else {
			// Fallback to Scan (allocates)
			if err := rows.Scan(&data); err != nil {
				return fmt.Errorf("failed to scan chunk: %w", err)
			}
		}

		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("failed to write chunk to destination: %w", err)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating chunks: %w", err)
	}

	return nil
}

// GetMetadata retrieves metadata for a file if the seeker has permission.
func (s *Storage) GetMetadata(ctx context.Context, fileID, seekerID, seekerGroupID uuid.UUID) (*FileMetadata, error) {
	var meta FileMetadata
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, path, group_id, owner_id, created_at, updated_at 
		FROM files 
		WHERE id = $1 AND (owner_id = $2 OR group_id = $3)`,
		fileID, seekerID, seekerGroupID).Scan(
		&meta.ID, &meta.Name, &meta.Path, &meta.GroupID, &meta.UserID, &meta.CreatedAt, &meta.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("file not found or access denied")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query metadata: %w", err)
	}

	return &meta, nil
}

// Delete removes a file and its chunks if the seeker has permission.
func (s *Storage) Delete(ctx context.Context, fileID, seekerID, seekerGroupID uuid.UUID) error {
	// Only owner or someone in the group can delete (simplified)
	res, err := s.pool.Exec(ctx, `
		DELETE FROM files 
		WHERE id = $1 AND (owner_id = $2 OR group_id = $3)`,
		fileID, seekerID, seekerGroupID)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	if res.RowsAffected() == 0 {
		return fmt.Errorf("file not found or access denied")
	}

	return nil
}

// ListByPath lists files in a path if the seeker has permission.
func (s *Storage) ListByPath(ctx context.Context, path string, seekerID, seekerGroupID uuid.UUID) ([]*FileMetadata, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, path, group_id, owner_id, created_at, updated_at 
		FROM files 
		WHERE path = $1 AND (owner_id = $2 OR group_id = $3)`,
		path, seekerID, seekerGroupID)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	defer rows.Close()

	var files []*FileMetadata
	for rows.Next() {
		meta := &FileMetadata{}
		err := rows.Scan(&meta.ID, &meta.Name, &meta.Path, &meta.GroupID, &meta.UserID, &meta.CreatedAt, &meta.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metadata: %w", err)
		}
		files = append(files, meta)
	}

	return files, nil
}
