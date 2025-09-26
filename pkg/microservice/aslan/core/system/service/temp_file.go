/*
Copyright 2025 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	s3service "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/s3"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
)

func init() {
	// Set the global file name resolver function
	models.GetFileNameByID = getFileNameByID
}

const (
	tempFilePartSize = 5 * 1024 * 1024 // 5MB per part
)

// InitiateMultipartUpload starts a new multipart upload session
func InitiateMultipartUpload(req *models.InitiateUploadRequest, log *zap.SugaredLogger) (*models.InitiateUploadResponse, error) {
	// Initialize instance routing if not already done
	InitInstanceRouting()

	sessionID := uuid.New().String()
	instanceID := GetInstanceForKey(sessionID)

	// Validate total parts
	expectedParts := int((req.FileSize + tempFilePartSize - 1) / tempFilePartSize)
	if req.TotalParts != expectedParts {
		return nil, e.ErrInvalidParam.AddDesc(fmt.Sprintf("total parts mismatch, expected %d, got %d", expectedParts, req.TotalParts))
	}

	now := time.Now().Unix()
	temporaryFile := &models.TemporaryFile{
		SessionID:     sessionID,
		FileName:      req.FileName,
		FileSize:      req.FileSize,
		TotalParts:    req.TotalParts,
		UploadedParts: []int{},
		Status:        models.TemporaryFileStatusUploading,
		InstanceID:    instanceID,
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     now + (24 * 60 * 60), // 24 hours in seconds
	}

	if err := commonrepo.NewTemporaryFileColl().Create(temporaryFile); err != nil {
		log.Errorf("failed to create temporary file record: %v", err)
		return nil, e.ErrCreateIDPPlugin.AddErr(err)
	}

	return &models.InitiateUploadResponse{
		SessionID: sessionID,
	}, nil
}

// UploadPart handles uploading a single part of the file
func UploadPart(sessionID string, partNumber int, fileHeader *multipart.FileHeader, file multipart.File, log *zap.SugaredLogger) error {
	temporaryFile, err := commonrepo.NewTemporaryFileColl().GetBySessionID(sessionID)
	if err != nil {
		log.Errorf("failed to get temporary file: %v", err)
		return e.ErrNotFound.AddErr(err)
	}

	if temporaryFile.Status != models.TemporaryFileStatusUploading {
		return e.ErrInvalidParam.AddDesc("upload session is not in uploading state")
	}

	if partNumber < 1 || partNumber > temporaryFile.TotalParts {
		return e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid part number %d, expected 1-%d", partNumber, temporaryFile.TotalParts))
	}

	// Check if part already uploaded
	for _, uploaded := range temporaryFile.UploadedParts {
		if uploaded == partNumber {
			return e.ErrInvalidParam.AddDesc(fmt.Sprintf("part %d already uploaded", partNumber))
		}
	}

	// Save part to temporary directory
	tempDir := getTempDir(sessionID)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return e.ErrCreateIDPPlugin.AddErr(err)
	}

	partPath := filepath.Join(tempDir, fmt.Sprintf("part_%d", partNumber))
	out, err := os.Create(partPath)
	if err != nil {
		return e.ErrCreateIDPPlugin.AddErr(err)
	}
	defer out.Close()

	// Copy part data
	_, err = io.Copy(out, file)
	if err != nil {
		return e.ErrCreateIDPPlugin.AddErr(err)
	}

	// Update uploaded parts
	updatedParts := append(temporaryFile.UploadedParts, partNumber)
	sort.Ints(updatedParts)

	if err := commonrepo.NewTemporaryFileColl().UpdateUploadedParts(sessionID, updatedParts); err != nil {
		log.Errorf("failed to update uploaded parts: %v", err)
		return e.ErrUpdateIDPPlugin.AddErr(err)
	}

	log.Infof("uploaded part %d/%d for session %s", partNumber, temporaryFile.TotalParts, sessionID)
	return nil
}

// CompleteMultipartUpload assembles all parts into final file
func CompleteMultipartUpload(sessionID string, req *models.CompleteUploadRequest, log *zap.SugaredLogger) (*models.CompleteUploadResponse, error) {
	temporaryFile, err := commonrepo.NewTemporaryFileColl().GetBySessionID(sessionID)
	if err != nil {
		log.Errorf("failed to get temporary file: %v", err)
		return nil, e.ErrNotFound.AddErr(err)
	}

	if temporaryFile.Status != models.TemporaryFileStatusUploading {
		return nil, e.ErrInvalidParam.AddDesc("upload session is not in uploading state")
	}

	// Check if all parts are uploaded
	if len(temporaryFile.UploadedParts) != temporaryFile.TotalParts {
		return nil, e.ErrInvalidParam.AddDesc(fmt.Sprintf("missing parts, uploaded %d/%d", len(temporaryFile.UploadedParts), temporaryFile.TotalParts))
	}

	// Verify all parts are consecutive
	sort.Ints(temporaryFile.UploadedParts)
	for i, partNum := range temporaryFile.UploadedParts {
		if partNum != i+1 {
			return nil, e.ErrInvalidParam.AddDesc("missing or invalid part numbers")
		}
	}

	// Assemble final file
	finalPath, fileHash, err := assembleFile(sessionID, temporaryFile, log)
	if err != nil {
		return nil, err
	}

	// Verify file hash if provided
	if req.FileHash != "" && req.FileHash != fileHash {
		return nil, e.ErrInvalidParam.AddDesc("file hash mismatch")
	}

	// Upload to S3
	store, err := s3service.FindDefaultS3()
	if err != nil {
		log.Errorf("failed to find default s3: %v", err)
		return nil, e.ErrCreateIDPPlugin.AddErr(err)
	}

	client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Region, store.Insecure, store.Provider)
	if err != nil {
		log.Errorf("failed to create s3 client: %v", err)
		return nil, e.ErrCreateIDPPlugin.AddErr(err)
	}

	objectKey := store.GetObjectPath(getTempFileS3Path(sessionID, temporaryFile.FileName))
	if err := client.Upload(store.Bucket, finalPath, objectKey); err != nil {
		log.Errorf("failed to upload file to s3: %v", err)
		return nil, e.ErrCreateIDPPlugin.AddErr(err)
	}

	// Update database
	if err := commonrepo.NewTemporaryFileColl().UpdateFileInfo(sessionID, store.ID.Hex(), objectKey, fileHash); err != nil {
		log.Errorf("failed to update file info: %v", err)
		return nil, e.ErrUpdateIDPPlugin.AddErr(err)
	}

	// Cleanup temp files
	go cleanupTempDir(sessionID, log)

	return &models.CompleteUploadResponse{
		FileID:   temporaryFile.ID.Hex(),
		FilePath: objectKey,
	}, nil
}

// GetUploadStatus returns the current status of an upload
func GetUploadStatus(sessionID string, log *zap.SugaredLogger) (*models.UploadStatusResponse, error) {
	temporaryFile, err := commonrepo.NewTemporaryFileColl().GetBySessionID(sessionID)
	if err != nil {
		log.Errorf("failed to get temporary file: %v", err)
		return nil, e.ErrNotFound.AddErr(err)
	}

	progress := float64(len(temporaryFile.UploadedParts)) / float64(temporaryFile.TotalParts) * 100

	return &models.UploadStatusResponse{
		SessionID:     temporaryFile.SessionID,
		Status:        temporaryFile.Status,
		FileName:      temporaryFile.FileName,
		FileSize:      temporaryFile.FileSize,
		TotalParts:    temporaryFile.TotalParts,
		UploadedParts: temporaryFile.UploadedParts,
		Progress:      progress,
		CreatedAt:     temporaryFile.CreatedAt,
		UpdatedAt:     temporaryFile.UpdatedAt,
	}, nil
}

// GetTemporaryFile returns a temporary file by ID (for system use)
func GetTemporaryFile(fileID string, log *zap.SugaredLogger) (*models.TemporaryFile, error) {
	temporaryFile, err := commonrepo.NewTemporaryFileColl().GetByID(fileID)
	if err != nil {
		log.Errorf("failed to get temporary file: %v", err)
		return nil, e.ErrNotFound.AddErr(err)
	}

	if temporaryFile.Status != models.TemporaryFileStatusCompleted {
		return nil, e.ErrNotFound.AddDesc("file not ready")
	}

	return temporaryFile, nil
}

// DownloadTemporaryFile downloads a temporary file from S3 and streams it to the client
func DownloadTemporaryFile(fileID string, c *gin.Context, log *zap.SugaredLogger) error {
	// Get the temporary file record
	temporaryFile, err := commonrepo.NewTemporaryFileColl().GetByID(fileID)
	if err != nil {
		log.Errorf("failed to get temporary file: %v", err)
		return e.ErrNotFound.AddErr(err)
	}

	if temporaryFile.Status != models.TemporaryFileStatusCompleted {
		return e.ErrNotFound.AddDesc("file not ready")
	}

	// Get the S3 storage configuration
	store, err := s3service.FindS3ById(temporaryFile.StorageID)
	if err != nil {
		log.Errorf("failed to get S3 storage: %v", err)
		return e.ErrInternalError.AddErr(err)
	}

	// Create S3 client
	client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Region, store.Insecure, store.Provider)
	if err != nil {
		log.Errorf("failed to create S3 client: %v", err)
		return e.ErrInternalError.AddErr(err)
	}

	// Get file from S3
	obj, err := client.GetFile(store.Bucket, temporaryFile.FilePath, &s3tool.DownloadOption{IgnoreNotExistError: false, RetryNum: 2})
	if err != nil {
		log.Errorf("failed to download file from S3: %v", err)
		return e.ErrInternalError.AddErr(err)
	}
	if obj == nil {
		return e.ErrNotFound.AddDesc("file not found in storage")
	}
	defer obj.Body.Close()

	// Set appropriate headers
	filename := temporaryFile.FileName
	if filename == "" {
		filename = "download"
	}

	// Determine content type based on file extension
	contentType := "application/octet-stream"

	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprintf("%d", temporaryFile.FileSize))

	c.Status(http.StatusOK)
	_, err = io.Copy(c.Writer, obj.Body)
	if err != nil {
		log.Errorf("failed to stream file content: %v", err)
		return e.ErrInternalError.AddErr(err)
	}

	log.Infof("successfully downloaded temporary file: %s (%s)", temporaryFile.FileName, fileID)
	return nil
}

// Helper functions
func getTempDir(sessionID string) string {
	return filepath.Join(os.TempDir(), "zadig-uploads", sessionID)
}

func getTempFileS3Path(sessionID, fileName string) string {
	return filepath.Join("temp-files", sessionID, fileName)
}

func assembleFile(sessionID string, temporaryFile *models.TemporaryFile, log *zap.SugaredLogger) (string, string, error) {
	tempDir := getTempDir(sessionID)
	finalPath := filepath.Join(tempDir, temporaryFile.FileName)

	finalFile, err := os.Create(finalPath)
	if err != nil {
		return "", "", e.ErrCreateIDPPlugin.AddErr(err)
	}
	defer finalFile.Close()

	hasher := sha256.New()
	multiWriter := io.MultiWriter(finalFile, hasher)

	// Assemble parts in order
	for partNum := 1; partNum <= temporaryFile.TotalParts; partNum++ {
		partPath := filepath.Join(tempDir, fmt.Sprintf("part_%d", partNum))
		partFile, err := os.Open(partPath)
		if err != nil {
			return "", "", e.ErrCreateIDPPlugin.AddErr(err)
		}

		_, err = io.Copy(multiWriter, partFile)
		partFile.Close()
		if err != nil {
			return "", "", e.ErrCreateIDPPlugin.AddErr(err)
		}
	}

	fileHash := hex.EncodeToString(hasher.Sum(nil))
	log.Infof("assembled file %s with hash %s", temporaryFile.FileName, fileHash)

	return finalPath, fileHash, nil
}

func cleanupTempDir(sessionID string, log *zap.SugaredLogger) {
	tempDir := getTempDir(sessionID)
	if err := os.RemoveAll(tempDir); err != nil {
		log.Warnf("failed to cleanup temp directory %s: %v", tempDir, err)
	}
}

// CleanupExpiredUploads removes expired upload records and temp files
func CleanupExpiredUploads(log *zap.SugaredLogger) error {
	expiredFiles, err := commonrepo.NewTemporaryFileColl().ListExpired()
	if err != nil {
		log.Errorf("failed to list expired files: %v", err)
		return err
	}

	for _, file := range expiredFiles {
		// Cleanup temp directory
		go cleanupTempDir(file.SessionID, log)

		// Update status to expired
		if err := commonrepo.NewTemporaryFileColl().UpdateStatus(file.SessionID, models.TemporaryFileStatusExpired); err != nil {
			log.Errorf("failed to update status for expired file %s: %v", file.SessionID, err)
		}
	}

	log.Infof("cleaned up %d expired upload sessions", len(expiredFiles))
	return nil
}

// getFileNameByID retrieves the file name for a given file ID
func getFileNameByID(fileID string) (string, error) {
	temporaryFile, err := commonrepo.NewTemporaryFileColl().GetByID(fileID)
	if err != nil {
		return "", err
	}
	if temporaryFile == nil {
		return "", fmt.Errorf("temporary file not found")
	}
	return temporaryFile.FileName, nil
}
