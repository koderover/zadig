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

package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	TemporaryFileStatusUploading = "uploading"
	TemporaryFileStatusCompleted = "completed"
	TemporaryFileStatusFailed    = "failed"
	TemporaryFileStatusExpired   = "expired"
)

type TemporaryFile struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	SessionID     string             `bson:"session_id" json:"session_id"`
	FileName      string             `bson:"file_name" json:"file_name"`
	FileSize      int64              `bson:"file_size" json:"file_size"`
	FileHash      string             `bson:"file_hash" json:"file_hash"`
	TotalParts    int                `bson:"total_parts" json:"total_parts"`
	UploadedParts []int              `bson:"uploaded_parts" json:"uploaded_parts"`
	Status        string             `bson:"status" json:"status"`
	FilePath      string             `bson:"file_path" json:"file_path"`
	StorageID     string             `bson:"storage_id" json:"storage_id"`
	InstanceID    string             `bson:"instance_id" json:"instance_id"`
	CreatedAt     int64              `bson:"created_at" json:"created_at"`
	UpdatedAt     int64              `bson:"updated_at" json:"updated_at"`
	ExpiresAt     int64              `bson:"expires_at" json:"expires_at"`
}

func (TemporaryFile) TableName() string {
	return "temporary_file"
}

type InitiateUploadRequest struct {
	FileName   string `json:"file_name" binding:"required"`
	FileSize   int64  `json:"file_size" binding:"required"`
	TotalParts int    `json:"total_parts" binding:"required"`
}

type InitiateUploadResponse struct {
	SessionID string `json:"session_id"`
}

type UploadStatusResponse struct {
	SessionID     string  `json:"session_id"`
	Status        string  `json:"status"`
	FileName      string  `json:"file_name"`
	FileSize      int64   `json:"file_size"`
	TotalParts    int     `json:"total_parts"`
	UploadedParts []int   `json:"uploaded_parts"`
	Progress      float64 `json:"progress"`
	CreatedAt     int64   `json:"created_at"`
	UpdatedAt     int64   `json:"updated_at"`
}

type CompleteUploadRequest struct {
	FileHash string `json:"file_hash" binding:"required"`
}

type CompleteUploadResponse struct {
	FileID   string `json:"file_id"`
	FilePath string `json:"file_path"`
}
