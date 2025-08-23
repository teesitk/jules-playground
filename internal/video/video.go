package video

import (
	"context"
	"io"
	"time"
)

// VideoStatus represents the processing status of a video.
type VideoStatus string

const (
	StatusPending    VideoStatus = "pending"
	StatusProcessing VideoStatus = "processing"
	StatusPublished  VideoStatus = "published"
	StatusFailed     VideoStatus = "failed"
	StatusArchived   VideoStatus = "archived"
)

// Video represents a video entity.
type Video struct {
	ID          string      `json:"id"`
	UserID      string      `json:"user_id"` // ID of the user who uploaded the video
	Title       string      `json:"title"`
	Description string      `json:"description,omitempty"`
	S3ObjectKey string      `json:"-"` // Key for the video file in S3, not usually exposed
	ThumbnailURL string     `json:"thumbnail_url,omitempty"`
	StreamURL   string      `json:"stream_url,omitempty"` // This might be generated or static
	Duration    int         `json:"duration,omitempty"`  // Duration in seconds
	Status      VideoStatus `json:"status"`
	Tags        []string    `json:"tags,omitempty"`         // Product tags or general tags
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	PublishedAt *time.Time  `json:"published_at,omitempty"`
}

// FileStorage defines the interface for blob/file storage operations.
type FileStorage interface {
	Upload(ctx context.Context, key string, file io.Reader, size int64, contentType string) (string, error) // Returns URL or path
	Delete(ctx context.Context, key string) error
	GetURL(ctx context.Context, key string) (string, error)
}

// VideoRepository defines the interface for video data storage (metadata)
// and interaction with file storage.
type VideoRepository interface {
	// Metadata operations
	Create(ctx context.Context, video *Video) error
	FindByID(ctx context.Context, id string) (*Video, error)
	Update(ctx context.Context, video *Video) error
	DeleteMetadata(ctx context.Context, id string) error // Deletes only metadata
	FindByUser(ctx context.Context, userID string, limit, offset int) ([]*Video, error)
	UpdateStatus(ctx context.Context, id string, status VideoStatus, processingError *string) error

	// File operations (delegated to FileStorage, but part of the repo's responsibility)
	UploadVideoFile(ctx context.Context, key string, file io.Reader, size int64, contentType string) (string, error)
	DeleteVideoFile(ctx context.Context, key string) error
	GetVideoFilePublicURL(ctx context.Context, key string) (string, error)
}
