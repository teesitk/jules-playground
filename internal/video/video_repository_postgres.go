package video

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log" // Temporary for mock storage
	"time"

	"github.com/lib/pq" // PostgreSQL driver
)

// PostgresVideoRepository implements the metadata storage parts of VideoRepository using PostgreSQL.
// It also holds a FileStorage interface for video file operations.
type PostgresVideoRepository struct {
	DB          *sql.DB
	FileStorage FileStorage // Interface for S3-compatible storage
}

// NewPostgresVideoRepository creates a new instance of PostgresVideoRepository.
func NewPostgresVideoRepository(db *sql.DB, fileStorage FileStorage) *PostgresVideoRepository {
	if fileStorage == nil {
		// Fallback to a mock file storage if none is provided (for easier setup initially)
		log.Println("Warning: No FileStorage provided to PostgresVideoRepository, using MockFileStorage.")
		fileStorage = NewMockFileStorage()
	}
	return &PostgresVideoRepository{DB: db, FileStorage: fileStorage}
}

// Create inserts new video metadata into the database.
func (r *PostgresVideoRepository) Create(ctx context.Context, video *Video) error {
	query := `
		INSERT INTO videos (id, user_id, title, description, s3_object_key, thumbnail_url, stream_url, duration, status, tags, created_at, updated_at, published_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := r.DB.ExecContext(ctx, query,
		video.ID, video.UserID, video.Title, video.Description, video.S3ObjectKey,
		video.ThumbnailURL, video.StreamURL, video.Duration, video.Status,
		pq.Array(video.Tags), video.CreatedAt, video.UpdatedAt, video.PublishedAt,
	)
	if err != nil {
		// Example: Check for foreign key violation if user_id does not exist
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23503" {
			return errors.New("user_id does not exist")
		}
		return fmt.Errorf("error creating video metadata: %w", err)
	}
	return nil
}

// FindByID retrieves video metadata from the database by its ID.
func (r *PostgresVideoRepository) FindByID(ctx context.Context, id string) (*Video, error) {
	query := `
		SELECT id, user_id, title, description, s3_object_key, thumbnail_url, stream_url,
		       duration, status, tags, created_at, updated_at, published_at
		FROM videos
		WHERE id = $1
	`
	video := &Video{}
	var tags pq.StringArray
	var publishedAt pq.NullTime

	err := r.DB.QueryRowContext(ctx, query, id).Scan(
		&video.ID, &video.UserID, &video.Title, &video.Description, &video.S3ObjectKey,
		&video.ThumbnailURL, &video.StreamURL, &video.Duration, &video.Status, &tags,
		&video.CreatedAt, &video.UpdatedAt, &publishedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("video not found")
		}
		return nil, fmt.Errorf("error finding video by id: %w", err)
	}
	video.Tags = []string(tags)
	if publishedAt.Valid {
		video.PublishedAt = &publishedAt.Time
	}
	return video, nil
}

// Update modifies existing video metadata in the database.
func (r *PostgresVideoRepository) Update(ctx context.Context, video *Video) error {
	video.UpdatedAt = time.Now().UTC()
	query := `
		UPDATE videos
		SET title = $2, description = $3, s3_object_key = $4, thumbnail_url = $5, stream_url = $6,
		    duration = $7, status = $8, tags = $9, updated_at = $10, published_at = $11
		WHERE id = $1
	`
	result, err := r.DB.ExecContext(ctx, query,
		video.ID, video.Title, video.Description, video.S3ObjectKey, video.ThumbnailURL,
		video.StreamURL, video.Duration, video.Status, pq.Array(video.Tags), video.UpdatedAt, video.PublishedAt,
	)
	if err != nil {
		return fmt.Errorf("error updating video metadata: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return errors.New("video not found or no changes made")
	}
	return nil
}

// DeleteMetadata removes video metadata from the database.
func (r *PostgresVideoRepository) DeleteMetadata(ctx context.Context, id string) error {
	query := "DELETE FROM videos WHERE id = $1"
	result, err := r.DB.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting video metadata: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return errors.New("video not found")
	}
	return nil
}

// FindByUser retrieves videos uploaded by a specific user.
func (r *PostgresVideoRepository) FindByUser(ctx context.Context, userID string, limit, offset int) ([]*Video, error) {
	query := `
		SELECT id, user_id, title, description, s3_object_key, thumbnail_url, stream_url,
		       duration, status, tags, created_at, updated_at, published_at
		FROM videos
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.DB.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("error finding videos by user: %w", err)
	}
	defer rows.Close()

	videos := make([]*Video, 0)
	for rows.Next() {
		video := &Video{}
		var tags pq.StringArray
		var publishedAt pq.NullTime
		if err := rows.Scan(
			&video.ID, &video.UserID, &video.Title, &video.Description, &video.S3ObjectKey,
			&video.ThumbnailURL, &video.StreamURL, &video.Duration, &video.Status, &tags,
			&video.CreatedAt, &video.UpdatedAt, &publishedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning video row: %w", err)
		}
		video.Tags = []string(tags)
		if publishedAt.Valid {
			video.PublishedAt = &publishedAt.Time
		}
		videos = append(videos, video)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating video rows: %w", err)
	}
	return videos, nil
}

// UpdateStatus updates the status of a video and optionally a processing error message.
func (r *PostgresVideoRepository) UpdateStatus(ctx context.Context, id string, status VideoStatus, processingError *string) error {
	query := `
		UPDATE videos
		SET status = $2, processing_error = $3, updated_at = $4
		WHERE id = $1
	`
	// Note: 'processing_error' column needs to be added to your 'videos' table schema.
	// For now, this assumes it exists and is of type TEXT or VARCHAR.
	// If it's not added, this query will fail.
	// We'll handle nil processingError by passing sql.NullString.
	var sqlProcessingError sql.NullString
	if processingError != nil {
		sqlProcessingError.String = *processingError
		sqlProcessingError.Valid = true
	}

	result, err := r.DB.ExecContext(ctx, query, id, status, sqlProcessingError, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("error updating video status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return errors.New("video not found for status update")
	}
	return nil
}


// --- FileStorage delegation methods ---

// UploadVideoFile delegates to the FileStorage interface.
func (r *PostgresVideoRepository) UploadVideoFile(ctx context.Context, key string, file io.Reader, size int64, contentType string) (string, error) {
	return r.FileStorage.Upload(ctx, key, file, size, contentType)
}

// DeleteVideoFile delegates to the FileStorage interface.
func (r *PostgresVideoRepository) DeleteVideoFile(ctx context.Context, key string) error {
	return r.FileStorage.Delete(ctx, key)
}

// GetVideoFilePublicURL delegates to the FileStorage interface.
func (r *PostgresVideoRepository) GetVideoFilePublicURL(ctx context.Context, key string) (string, error) {
	return r.FileStorage.GetURL(ctx, key)
}


// MockFileStorage provides a mock implementation of FileStorage for testing/initial dev.
type MockFileStorage struct {
	files map[string]string // Stores key -> "mock_url_for_key"
}

func NewMockFileStorage() *MockFileStorage {
	return &MockFileStorage{files: make(map[string]string)}
}

func (m *MockFileStorage) Upload(ctx context.Context, key string, file io.Reader, size int64, contentType string) (string, error) {
	log.Printf("MockFileStorage: Upload called for key %s, size %d, contentType %s\n", key, size, contentType)
	// In a real scenario, you'd read from 'file' and upload it.
	// For mock, we just record it and return a fake URL.
	m.files[key] = "mock_s3_url/" + key
	return m.files[key], nil
}

func (m *MockFileStorage) Delete(ctx context.Context, key string) error {
	log.Printf("MockFileStorage: Delete called for key %s\n", key)
	if _, exists := m.files[key]; !exists {
		return errors.New("mock file not found")
	}
	delete(m.files, key)
	return nil
}

func (m *MockFileStorage) GetURL(ctx context.Context, key string) (string, error) {
	log.Printf("MockFileStorage: GetURL called for key %s\n", key)
	if url, exists := m.files[key]; exists {
		return url, nil
	}
	return "", errors.New("mock file not found")
}

// Ensure MockFileStorage implements FileStorage (compile-time check)
var _ FileStorage = (*MockFileStorage)(nil)
// Ensure PostgresVideoRepository implements VideoRepository (compile-time check)
var _ VideoRepository = (*PostgresVideoRepository)(nil)
