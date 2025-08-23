package video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log" // For Redis mock
	"time"

	"github.com/google/uuid"
	// "github.com/go-redis/redis/v8" // Example Redis client
)

// RedisClient defines a minimal interface for a Redis client (or a mock).
// This helps in decoupling the use case from a specific Redis library.
type RedisClient interface {
	Enqueue(ctx context.Context, queueName string, message []byte) error
}

// VideoProcessingJob represents the data pushed to the Redis queue for video processing.
type VideoProcessingJob struct {
	VideoID     string `json:"video_id"`
	S3ObjectKey string `json:"s3_object_key"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

// VideoUseCase defines the interface for video-related business logic.
type VideoUseCase interface {
	UploadVideo(ctx context.Context, userID, title, description string, tags []string, file io.Reader, fileSize int64, contentType string) (*Video, error)
	GetVideoByID(ctx context.Context, videoID string) (*Video, error)
	// Add other use cases like ListVideosByUser, UpdateVideoDetails, DeleteVideo etc. later
}

// videoUseCase implements the VideoUseCase interface.
type videoUseCase struct {
	videoRepo         VideoRepository
	redisClient       RedisClient // For pushing jobs to a queue
	videoProcessingQueue string    // Name of the Redis queue
}

// NewVideoUseCase creates a new instance of VideoUseCase.
func NewVideoUseCase(repo VideoRepository, redisClient RedisClient, queueName string) VideoUseCase {
	if redisClient == nil {
		log.Println("Warning: No RedisClient provided to VideoUseCase, using MockRedisClient.")
		redisClient = NewMockRedisClient()
	}
	return &videoUseCase{
		videoRepo:         repo,
		redisClient:       redisClient,
		videoProcessingQueue: queueName,
	}
}

// UploadVideo handles the business logic for uploading a new video.
func (uc *videoUseCase) UploadVideo(ctx context.Context, userID, title, description string, tags []string, file io.Reader, fileSize int64, contentType string) (*Video, error) {
	if userID == "" {
		return nil, errors.New("userID is required")
	}
	if title == "" {
		return nil, errors.New("video title is required")
	}
	if file == nil {
		return nil, errors.New("video file is required")
	}
	if fileSize == 0 {
		return nil, errors.New("fileSize must be greater than 0")
	}

	videoID := uuid.NewString()
	// Generate a unique S3 object key, e.g., using userID and videoID to prevent collisions
	// and make it easier to manage/debug.
	s3ObjectKey := fmt.Sprintf("videos/%s/%s", userID, videoID) // Example structure: videos/user_id_abc/video_id_xyz.mp4 (extension added by S3 or client)

	// 1. Upload the video file
	s3ObjectURL, err := uc.videoRepo.UploadVideoFile(ctx, s3ObjectKey, file, fileSize, contentType)
	if err != nil {
		return nil, fmt.Errorf("failed to upload video file: %w", err)
	}

	// 2. Create video metadata
	video := &Video{
		ID:          videoID,
		UserID:      userID,
		Title:       title,
		Description: description,
		S3ObjectKey: s3ObjectKey, // Store the key, not the full URL, as URL might change or have temporary access
		// StreamURL and ThumbnailURL will be updated by a processing job later
		Status:    StatusPending, // Initial status
		Tags:      tags,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	err = uc.videoRepo.Create(ctx, video)
	if err != nil {
		// If metadata creation fails, we should ideally try to delete the uploaded S3 object (rollback)
		// This is a common pattern in distributed transactions or multi-step operations.
		// For simplicity here, we're not adding that rollback logic yet.
		log.Printf("Error creating video metadata for video %s (S3 key %s): %v. S3 object might need manual cleanup.", videoID, s3ObjectKey, err)
		return nil, fmt.Errorf("failed to save video metadata: %w", err)
	}

	// 3. Push a job to Redis for video processing (e.g., thumbnail generation, transcoding)
	jobPayload := VideoProcessingJob{
		VideoID:     video.ID,
		S3ObjectKey: video.S3ObjectKey,
		UploadedAt:  video.CreatedAt,
	}
	jobPayloadBytes, err := json.Marshal(jobPayload)
	if err != nil {
		// Log this error but don't fail the entire upload if queuing fails.
		// The system should have a way to retry or manually trigger processing.
		log.Printf("Error marshalling video processing job for video %s: %v. Video uploaded but processing not queued.", video.ID, err)
		// Depending on requirements, you might want to return an error here or mark the video specially.
	} else {
		err = uc.redisClient.Enqueue(ctx, uc.videoProcessingQueue, jobPayloadBytes)
		if err != nil {
			// Same as above: log and decide if this is a critical failure.
			log.Printf("Error enqueuing video processing job for video %s: %v. Video uploaded but processing not queued.", video.ID, err)
		} else {
			log.Printf("Successfully enqueued video processing job for video ID %s, S3 key %s", video.ID, video.S3ObjectKey)
		}
	}

	// The s3ObjectURL from UploadVideoFile might be a pre-signed URL or a public URL
	// depending on the FileStorage implementation. We can assign it to StreamURL if appropriate,
	// or it might be set later after processing. For now, let's assume it's a direct usable URL.
	video.StreamURL = s3ObjectURL


	// Return the video metadata (without sensitive info like S3ObjectKey if not needed by client)
	// For now, returning the full video object as created.
	return video, nil
}

// GetVideoByID retrieves a video by its ID.
func (uc *videoUseCase) GetVideoByID(ctx context.Context, videoID string) (*Video, error) {
	if videoID == "" {
		return nil, errors.New("videoID is required")
	}
	video, err := uc.videoRepo.FindByID(ctx, videoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get video by ID '%s': %w", videoID, err)
	}
	// Optionally, generate a fresh public URL if StreamURL is not persisted or expires
	// publicURL, err := uc.videoRepo.GetVideoFilePublicURL(ctx, video.S3ObjectKey)
	// if err == nil {
	// video.StreamURL = publicURL
	// } else {
	// log.Printf("Could not get public URL for video %s: %v", videoID, err)
	// }
	return video, nil
}


// MockRedisClient provides a mock implementation of RedisClient for testing/initial dev.
type MockRedisClient struct {
	Queues map[string][][]byte // queueName -> list of messages
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{Queues: make(map[string][][]byte)}
}

func (m *MockRedisClient) Enqueue(ctx context.Context, queueName string, message []byte) error {
	log.Printf("MockRedisClient: Enqueue called for queue '%s', message: %s\n", queueName, string(message))
	m.Queues[queueName] = append(m.Queues[queueName], message)
	return nil
}

// Ensure MockRedisClient implements RedisClient (compile-time check)
var _ RedisClient = (*MockRedisClient)(nil)
// Ensure videoUseCase implements VideoUseCase (compile-time check)
var _ VideoUseCase = (*videoUseCase)(nil)
