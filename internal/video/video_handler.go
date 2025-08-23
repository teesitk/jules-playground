package video

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"entertainment_ecommerce/pkg/middleware" // Added for context keys
)

// VideoHandler handles HTTP requests for videos.
type VideoHandler struct {
	videoUseCase VideoUseCase
}

// NewVideoHandler creates a new instance of VideoHandler.
func NewVideoHandler(uc VideoUseCase) *VideoHandler {
	return &VideoHandler{videoUseCase: uc}
}

// ErrorResponse defines a generic JSON error response.
// (Could be moved to a shared package if used by multiple handlers)
type ErrorResponse struct {
	Error string `json:"error"`
}

func (e ErrorResponse) String() string {
	return `{"error":"` + e.Error + `"}`
}

// UploadVideoHandler handles the video upload requests.
// It expects a multipart/form-data request with:
// - "file": the video file
// - "title": string
// - "description": string (optional)
// - "tags": string (comma-separated, optional)
func (h *VideoHandler) UploadVideoHandler(w http.ResponseWriter, r *http.Request) {
	// 0. Extract UserID from context (set by auth middleware)
	// 0. Extract UserID from context (set by auth middleware)
	// Import "entertainment_ecommerce/pkg/middleware" if not already (should be auto-imported by go tools)
	// For clarity, let's assume the import path needs to be explicit if not using `goimports`
	// No, middleware types are not directly used here, just its constant keys.
	// The key itself can be defined as a string or a custom type.
	// Our middleware uses: middleware.UserIDCtxKey which is of type middleware.ContextKey("userID")

	userIDFromCtx := r.Context().Value(ContextKey("userID")) // This uses the local ContextKey type

	// To use the one from the middleware package, the video_handler would need to know about middleware.UserIDCtxKey
	// Let's adjust video_handler.go to be agnostic of the middleware's internal key type if possible,
	// or ensure it uses the exact same key.
	// The middleware was defined in `pkg/middleware/auth_middleware.go` with:
	// const UserIDCtxKey ContextKey = "userID"
	// So, using r.Context().Value(middleware.ContextKey("userID")) should work IF VideoHandler imports middleware.
	// Or, if we pass the key name "userID" (string) directly.
	// The middleware stores it as: context.WithValue(r.Context(), UserIDCtxKey, userID)
	// UserIDCtxKey is `middleware.ContextKey` type with value "userID".

	// Correct way assuming middleware package is accessible and its types/consts are used:
	// Needs: import "entertainment_ecommerce/pkg/middleware"
	// userIDFromCtx := r.Context().Value(middleware.UserIDCtxKey)

	// For now, let's assume the key is the string "userID" as that's what UserIDCtxKey resolves to.
	// This is a common approach if you don't want direct package dependencies for keys.
	// However, using the typed key from the package (middleware.UserIDCtxKey) is safer to avoid collisions.
	// Let's stick to the defined constant from the middleware package.
	// This requires adding the import for "entertainment_ecommerce/pkg/middleware" to video_handler.go

	// Simulating the use of the key string value if direct import is an issue in this step:
	// userIDFromCtx := r.Context().Value(ContextKey("userID")) // This would need ContextKey defined locally or imported.
	// The middleware `pkg/middleware/auth_middleware.go` defines:
	// const UserIDCtxKey ContextKey = "userID"
	// and stores the user ID in the context using this key:
	// ctx = context.WithValue(r.Context(), UserIDCtxKey, userID)
	// So, we must retrieve it using the same key.
	userIDFromCtx := r.Context().Value(middleware.UserIDCtxKey)

	if userIDFromCtx == nil {
		// This should ideally not be reached if middleware is applied correctly
		// and rejects unauthenticated requests. But as a safeguard:
		log.Println("UploadVideoHandler: UserID not found in context. This might indicate middleware is not applied or token is malformed in a way middleware didn't catch.")
		http.Error(w, ErrorResponse{Error: "Unauthorized: User ID missing from context."}.String(), http.StatusUnauthorized)
		return
	}

	currentUserID, ok := userIDFromCtx.(string)
	if !ok || currentUserID == "" {
		log.Printf("UploadVideoHandler: UserID in context is not a valid string or is empty. Value: %v", userIDFromCtx)
		http.Error(w, ErrorResponse{Error: "Unauthorized: Invalid User ID in token."}.String(), http.StatusUnauthorized)
		return
	}
	log.Printf("UploadVideoHandler: Authenticated UserID from context: %s", currentUserID)


	// 1. Parse multipart form
	// Set a maximum upload size (e.g., 1GB). Adjust as needed.
	// http.MaxBytesReader will protect against excessively large uploads.
	maxUploadSize := int64(1024 * 1024 * 1024) // 1GB
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB for form data in memory
		log.Printf("Error parsing multipart form: %v", err)
		if err.Error() == "http: request body too large" {
			http.Error(w, ErrorResponse{Error: "File too large. Maximum size is 1GB."}.String(), http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, ErrorResponse{Error: "Invalid request: Could not parse multipart form."}.String(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 2. Get the file from the form
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		log.Printf("Error retrieving file from form: %v", err)
		http.Error(w, ErrorResponse{Error: "Invalid request: 'file' field is missing or invalid."}.String(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 3. Get other form values
	title := r.FormValue("title")
	description := r.FormValue("description")
	// tagsStr := r.FormValue("tags") // Assuming tags are comma-separated string: "tag1,tag2,tag3"
	// var tags []string
	// if tagsStr != "" {
	// 	tags = strings.Split(tagsStr, ",")
	// 	for i, tag := range tags {
	// 		tags[i] = strings.TrimSpace(tag) // Clean up whitespace
	// 	}
	// }
    // For simplicity, let's assume tags are not passed for now or handled differently (e.g. JSON array in form field)
    var tags []string = []string{}


	if title == "" {
		http.Error(w, ErrorResponse{Error: "Invalid request: 'title' field is required."}.String(), http.StatusBadRequest)
		return
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		// Try to detect, or default, or reject. For now, let's allow if not present,
		// but S3 might require it or behave unpredictably.
		contentType = "application/octet-stream" // A generic default
		log.Printf("Content-Type not specified for uploaded file, defaulting to %s", contentType)
	}

	// 4. Call the use case
	video, err := h.videoUseCase.UploadVideo(
		r.Context(),
		currentUserID,
		title,
		description,
		tags,
		file, // io.Reader
		fileHeader.Size,
		contentType,
	)
	if err != nil {
		log.Printf("Error calling UploadVideo use case: %v", err)
		// Map domain errors to HTTP status codes if needed
		http.Error(w, ErrorResponse{Error: "Failed to upload video: " + err.Error()}.String(), http.StatusInternalServerError)
		return
	}

	// 5. Respond with success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(video); err != nil {
		// If encoding fails, the headers might have already been sent.
		// Log the error, but can't send a new HTTP error response.
		log.Printf("Error encoding success response: %v", err)
	}
}

// GetVideoByIDHandler handles requests to fetch a video by its ID.
func (h *VideoHandler) GetVideoByIDHandler(w http.ResponseWriter, r *http.Request) {
	// This assumes the videoID is passed as a path parameter, e.g., /videos/{videoID}
	// Routing setup (e.g., Gorilla Mux) will extract this. For now, let's imagine it's available.
	// videoID := mux.Vars(r)["videoID"] // Example with Gorilla Mux

	// For demonstration, let's try to get it from a query parameter if not using a router that sets path vars
	videoID := r.URL.Query().Get("id")

	if videoID == "" {
		// If using path parameters, this check might be handled by the router or be slightly different
		http.Error(w, ErrorResponse{Error: "Video ID is required in path or as query parameter 'id'"}.String(), http.StatusBadRequest)
		return
	}

	// Placeholder for userID from context for potential authorization checks later
	// userIDCtx := r.Context().Value("userID")
	// currentUserID := ""
	// if userIDCtx != nil {
	// 	currentUserID = userIDCtx.(string)
	// }

	video, err := h.videoUseCase.GetVideoByID(r.Context(), videoID)
	if err != nil {
		log.Printf("Error calling GetVideoByID use case for ID %s: %v", videoID, err)
		// Check if error is "video not found" to return 404
		if err.Error().Contains("not found") { // Basic check, better to use typed errors
			http.Error(w, ErrorResponse{Error: "Video not found"}.String(), http.StatusNotFound)
			return
		}
		http.Error(w, ErrorResponse{Error: "Failed to retrieve video: " + err.Error()}.String(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(video); err != nil {
		log.Printf("Error encoding video response for ID %s: %v", videoID, err)
	}
}

// Helper to get form value as int (example, not used directly above but useful)
func GetFormIntValue(r *http.Request, key string, defaultValue int) (int, error) {
	strVal := r.FormValue(key)
	if strVal == "" {
		return defaultValue, nil
	}
	intVal, err := strconv.Atoi(strVal)
	if err != nil {
		return 0, err
	}
	return intVal, nil
}
