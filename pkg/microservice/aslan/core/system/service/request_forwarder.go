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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// ForwardUploadPart forwards an upload part request to the correct instance
func ForwardUploadPart(sessionID string, partNumber int, fileHeader *multipart.FileHeader, file multipart.File, log *zap.SugaredLogger) error {
	targetInstance := GetInstanceForKey(sessionID)
	currentInstance := GetCurrentInstanceID()

	// If this is the correct instance, handle locally
	if targetInstance == currentInstance {
		return UploadPart(sessionID, partNumber, fileHeader, file, log)
	}

	// Forward to the correct instance
	return forwardMultipartRequest(targetInstance, fmt.Sprintf("/api/aslan/system/tempFile/upload/%s/%d", sessionID, partNumber), fileHeader, file, log)
}

// ForwardCompleteUpload forwards a complete upload request to the correct instance
func ForwardCompleteUpload(sessionID string, req *models.CompleteUploadRequest, log *zap.SugaredLogger) (*models.CompleteUploadResponse, error) {
	targetInstance := GetInstanceForKey(sessionID)
	currentInstance := GetCurrentInstanceID()

	// If this is the correct instance, handle locally
	if targetInstance == currentInstance {
		return CompleteMultipartUpload(sessionID, req, log)
	}

	// Forward to the correct instance
	return forwardJSONRequest[models.CompleteUploadResponse](targetInstance, fmt.Sprintf("/api/aslan/system/tempFile/complete/%s", sessionID), req, log)
}

// ForwardGetUploadStatus forwards a status request to the correct instance
func ForwardGetUploadStatus(sessionID string, log *zap.SugaredLogger) (*models.UploadStatusResponse, error) {
	targetInstance := GetInstanceForKey(sessionID)
	currentInstance := GetCurrentInstanceID()

	// If this is the correct instance, handle locally
	if targetInstance == currentInstance {
		return GetUploadStatus(sessionID, log)
	}

	// Forward to the correct instance
	return forwardGetRequest[models.UploadStatusResponse](targetInstance, fmt.Sprintf("/api/aslan/system/tempFile/status/%s", sessionID), log)
}

// Helper functions for forwarding requests

func forwardMultipartRequest(targetInstance, path string, fileHeader *multipart.FileHeader, file multipart.File, log *zap.SugaredLogger) error {
	// Create a new multipart form
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add the file
	fileWriter, err := writer.CreateFormFile("file", fileHeader.Filename)
	if err != nil {
		return e.ErrInternalError.AddErr(err)
	}

	// Copy file content
	if _, err := io.Copy(fileWriter, file); err != nil {
		return e.ErrInternalError.AddErr(err)
	}

	writer.Close()

	// Create the request
	url := fmt.Sprintf("http://%s%s", targetInstance, path)
	req, err := http.NewRequest("POST", url, &body)
	if err != nil {
		return e.ErrInternalError.AddErr(err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("failed to forward multipart request to %s: %v", targetInstance, err)
		return e.ErrInternalError.AddErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Errorf("forwarded request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
		return e.ErrInternalError.AddDesc(fmt.Sprintf("forwarded request failed: %s", string(bodyBytes)))
	}

	return nil
}

func forwardJSONRequest[T any](targetInstance, path string, payload interface{}, log *zap.SugaredLogger) (*T, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, e.ErrInternalError.AddErr(err)
	}

	url := fmt.Sprintf("http://%s%s", targetInstance, path)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, e.ErrInternalError.AddErr(err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("failed to forward JSON request to %s: %v", targetInstance, err)
		return nil, e.ErrInternalError.AddErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Errorf("forwarded request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
		return nil, e.ErrInternalError.AddDesc(fmt.Sprintf("forwarded request failed: %s", string(bodyBytes)))
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, e.ErrInternalError.AddErr(err)
	}

	return &result, nil
}

func forwardGetRequest[T any](targetInstance, path string, log *zap.SugaredLogger) (*T, error) {
	url := fmt.Sprintf("http://%s%s", targetInstance, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, e.ErrInternalError.AddErr(err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("failed to forward GET request to %s: %v", targetInstance, err)
		return nil, e.ErrInternalError.AddErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Errorf("forwarded request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
		return nil, e.ErrInternalError.AddDesc(fmt.Sprintf("forwarded request failed: %s", string(bodyBytes)))
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, e.ErrInternalError.AddErr(err)
	}

	return &result, nil
}
