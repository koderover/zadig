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

package handler

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// InitiateUpload starts a new multipart upload session
func InitiateUpload(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	var req models.InitiateUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	// Validate request
	if req.FileName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("file_name is required")
		return
	}
	if req.FileSize <= 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("file_size must be positive")
		return
	}
	if req.TotalParts <= 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("total_parts must be positive")
		return
	}

	ctx.Resp, ctx.RespErr = service.InitiateMultipartUpload(&req, ctx.Logger)
}

// UploadPart handles uploading a single part of the file
func UploadPart(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	sessionID := c.Param("sessionId")
	if sessionID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("sessionId is required")
		return
	}

	partNumberStr := c.Param("partNumber")
	partNumber, err := strconv.Atoi(partNumberStr)
	if err != nil || partNumber <= 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid partNumber")
		return
	}

	// Get file from form data
	file, fileHeader, err := c.Request.FormFile("file")
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(fmt.Sprintf("failed to get file: %v", err))
		return
	}
	defer file.Close()

	ctx.RespErr = service.ForwardUploadPart(sessionID, partNumber, fileHeader, file, ctx.Logger)
	if ctx.RespErr == nil {
		ctx.Resp = gin.H{"message": "part uploaded successfully"}
	}
}

// CompleteUpload assembles all parts into final file
func CompleteUpload(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	sessionID := c.Param("sessionId")
	if sessionID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("sessionId is required")
		return
	}

	var req models.CompleteUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.RespErr = service.ForwardCompleteUpload(sessionID, &req, ctx.Logger)
}

// GetUploadStatus returns the current status of an upload
func GetUploadStatus(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	sessionID := c.Param("sessionId")
	if sessionID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("sessionId is required")
		return
	}

	ctx.Resp, ctx.RespErr = service.ForwardGetUploadStatus(sessionID, ctx.Logger)
}

// GetTemporaryFile returns a temporary file by ID (for system use)
func GetTemporaryFile(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	fileID := c.Param("fileId")
	if fileID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("fileId is required")
		return
	}

	ctx.Resp, ctx.RespErr = service.GetTemporaryFile(fileID, ctx.Logger)
}

// CleanupExpiredUploads removes expired upload records and temp files (admin only)
func CleanupExpiredUploads(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.RespErr = service.CleanupExpiredUploads(ctx.Logger)
	if ctx.RespErr == nil {
		ctx.Resp = gin.H{"message": "cleanup completed successfully"}
	}
}
