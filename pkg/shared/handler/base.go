/*
Copyright 2021 The KodeRover Authors.

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
	"bytes"
	gocontext "context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"go.uber.org/zap"

	systemmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util/ginzap"
)

// Context struct
type Context struct {
	gocontext.Context
	Logger       *zap.SugaredLogger
	UnAuthorized bool
	RespErr      error
	Resp         interface{}
	Account      string
	UserName     string
	UserID       string
	IdentityType string
	RequestID    string
	Resources    *user.AuthorizedResources
}

type jwtClaims struct {
	Name            string          `json:"name"`
	Email           string          `json:"email"`
	UID             string          `json:"uid"`
	Account         string          `json:"preferred_username"`
	FederatedClaims FederatedClaims `json:"federated_claims"`
	jwt.StandardClaims
}

type FederatedClaims struct {
	ConnectorId string `json:"connector_id"`
	UserId      string `json:"user_id"`
}

func (c *Context) GenUserBriefInfo() types.UserBriefInfo {
	return types.UserBriefInfo{
		UID:          c.UserID,
		Name:         c.UserName,
		Account:      c.Account,
		IdentityType: c.IdentityType,
	}
}

// NewContext returns a context without user authorization info.
// TODO: We need to implement a `context.Context` that conforms to the golang standard library.
// After Jul.10 2023, this function should only be used when no authorization info is required.
// If authorization info is required, use `NewContextWithAuthorization` instead.
func NewContext(c *gin.Context) *Context {
	logger := ginzap.WithContext(c).Sugar()
	var err error
	var claims jwtClaims

	token := c.GetHeader(setting.AuthorizationHeader)
	if len(token) > 0 {
		claims, err = getUserFromJWT(token)
		if err != nil {
			logger.Warnf("Failed to get user from token, err: %s", err)
		}
	} else {
		token = c.Query("token")
		if len(token) == 0 {
			claims.Name = "system"
		} else {
			parts := strings.Split(token, ".")
			if len(parts) == 3 {
				claims, err = getUserFromJWT(token)
				if err != nil {
					logger.Warnf("Failed to get user from token, err: %s", err)
				}
			} else {
				claims.Name = "system"
			}
		}
	}

	return &Context{
		Context:      c.Request.Context(),
		UserName:     claims.Name,
		UserID:       claims.UID,
		Account:      claims.Account,
		IdentityType: claims.FederatedClaims.ConnectorId,
		Logger:       ginzap.WithContext(c).Sugar(),
		RequestID:    c.GetString(setting.RequestID),
	}
}

// NewContextWithAuthorization returns a context with user authorization info.
// This function should only be called when one need authorization information for api caller.
func NewContextWithAuthorization(c *gin.Context) (*Context, error) {
	logger := ginzap.WithContext(c).Sugar()
	var resourceAuthInfo *user.AuthorizedResources
	var err error
	resp := NewContext(c)
	// there is a case where the request does not have token (system call), in this case we will have admin access
	resourceAuthInfo, err = user.New().GetUserAuthInfo(resp.UserID)
	if err != nil {
		logger.Errorf("failed to generate user authorization info, error: %s", err)
		return resp, err
	}
	resp.Resources = resourceAuthInfo
	return resp, nil
}

func NewBackgroupContext() *Context {
	return &Context{
		Context:  gocontext.Background(),
		UserName: "system",
		Account:  "system",
		Logger:   log.SugaredLogger(),
	}
}

func GetResourcesInHeader(c *gin.Context) ([]string, bool) {
	_, ok := c.Request.Header[setting.ResourcesHeader]
	if !ok {
		return nil, false
	}

	res := c.GetHeader(setting.ResourcesHeader)
	if res == "" {
		return nil, true
	}
	var resources []string
	if err := json.Unmarshal([]byte(res), &resources); err != nil {
		return nil, false
	}

	return resources, true
}

func GetCollaborationModePermission(uid, projectKey, resource, resourceName, action string) (bool, error) {
	// when this method is called, uid is an expected field
	if uid == "" {
		return false, errors.New("empty user ID")
	}
	return user.New().CheckUserAuthInfoForCollaborationMode(uid, projectKey, resource, resourceName, action)
}

func ListAuthorizedProjects(uid string) ([]string, bool, error) {
	if uid == "" {
		return []string{}, false, errors.New("empty user ID")
	}
	return user.New().ListAuthorizedProjects(uid)
}

func ListAuthorizedProjectsByResourceAndVerb(uid, resource, verb string) ([]string, bool, error) {
	if uid == "" {
		return []string{}, false, errors.New("empty user ID")
	}
	return user.New().ListAuthorizedProjectsByResourceAndVerb(uid, resource, verb)
}

// CheckPermissionGivenByCollaborationMode checks if a user is permitted to perform specific action in a given project.
// Although collaboration mode is used to control the action to specific resources, under some circumstances, there are
// leaks/designs that allows user with resource permission not related to the corresponding resource to access.
// e.g. ListTest API will allow anyone with edit workflow permission to call it. In this case, we need to check if the permission
// is granted by collaboration mode.
// AVOID USING THIS !!!
// FIXME: This function shouldn't exist. The only reason it exists is the incompetent of the system designer.
func CheckPermissionGivenByCollaborationMode(uid, projectKey, resource, action string) (bool, error) {
	if uid == "" {
		return false, errors.New("empty user ID")
	}

	return user.New().CheckPermissionGivenByCollaborationMode(uid, projectKey, resource, action)
}

func ListAuthorizedWorkflows(uid, projectKey string) (authorizedWorkflow, authorizedWorkflowV4 []string, enableFilter bool, err error) {
	authorizedWorkflow = make([]string, 0)
	authorizedWorkflowV4 = make([]string, 0)
	enableFilter = true
	if uid == "" {
		err = errors.New("empty user ID")
		return
	}
	authorizedWorkflow, authorizedWorkflowV4, err = user.New().ListAuthorizedWorkflows(uid, projectKey)
	return
}

func ListAuthorizedWorkflowWithVerb(uid, projectKey string) (*user.CollModeAuthorizedWorkflowWithVerb, error) {
	authorizedWorkflow, err := user.New().ListAuthorizedWorkflowsWithVerb(uid, projectKey)
	return authorizedWorkflow, err
}

func ListCollaborationEnvironmentsPermission(uid, projectKey string) (authorizedEnv *types.CollaborationEnvPermission, err error) {
	if uid == "" {
		err = errors.New("empty user ID")
		return
	}
	authorizedEnv, err = user.New().ListCollaborationEnvironmentsPermission(uid, projectKey)
	return
}

func getUserFromJWT(token string) (jwtClaims, error) {
	cs := jwtClaims{}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return cs, fmt.Errorf("compact JWS format must have three parts")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return cs, err
	}

	err = json.Unmarshal(payload, &cs)
	if err != nil {
		return cs, err
	}

	return cs, nil
}

func JSONResponse(c *gin.Context, ctx *Context) {
	if ctx.UnAuthorized {
		if ctx.RespErr != nil {
			c.Set(setting.ResponseError, ctx.RespErr)
		}
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	if ctx.RespErr != nil {
		c.Set(setting.ResponseError, ctx.RespErr)
		c.Abort()
		return
	}

	if ctx.Resp != nil {
		realResp := responseHelper(ctx.Resp)
		c.Set(setting.ResponseData, realResp)
	}
}

func GetRawData(c *gin.Context) ([]byte, error) {
	data, err := c.GetRawData()
	if err != nil {
		return nil, err
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	return data, nil
}

// InsertOperationLog 插入操作日志
func InsertOperationLog(c *gin.Context, username, productName, method, function, detail, requestBody string, bodyType types.RequestBodyType, logger *zap.SugaredLogger) {
	req := &systemmodels.OperationLog{
		Username:    username,
		ProductName: productName,
		Method:      method,
		Function:    function,
		Name:        detail,
		RequestBody: requestBody,
		BodyType:    bodyType,
		Status:      0,
		CreatedAt:   time.Now().Unix(),
	}
	err := mongodb.NewOperationLogColl().Insert(req)
	if err != nil {
		logger.Errorf("InsertOperation err:%v", err)
	}
	c.Set("operationLogID", req.ID.Hex())
}

func InsertDetailedOperationLog(c *gin.Context, username, productName, scene, method, function, detail, requestBody string, bodyType types.RequestBodyType, logger *zap.SugaredLogger, targets ...string) {
	req := &systemmodels.OperationLog{
		Username:    username,
		ProductName: productName,
		Method:      method,
		Function:    function,
		Name:        detail,
		RequestBody: requestBody,
		BodyType:    bodyType,
		Scene:       scene,
		Targets:     targets,
		Status:      0,
		CreatedAt:   time.Now().Unix(),
	}
	err := mongodb.NewOperationLogColl().Insert(req)
	if err != nil {
		logger.Errorf("InsertOperation err:%v", err)
	}
	c.Set("operationLogID", req.ID.Hex())
}

// responseHelper recursively finds all nil slice in the given interface,
// replacing them with empty slices.
// Drawbacks of this function is listed below to avoid possible misuse.
//  1. This function will return the pointer of the given structure, in case a
//     pointer a passed in, a pointer will be returned, not a double pointer
//  2. All private fields of the struct will be deleted during the process.
func responseHelper(response interface{}) interface{} {
	switch response.(type) {
	case string, []byte:
		return response
	}

	val := reflect.ValueOf(response)
	switch val.Kind() {
	case reflect.Ptr:
		// if, in some case, a pointer is passed in, We dereference it and do the normal stuff
		if val.IsNil() {
			if reflect.TypeOf(response).Elem().Kind() == reflect.Slice {
				res := reflect.New(reflect.TypeOf(response).Elem())
				resp := reflect.MakeSlice(reflect.TypeOf(response).Elem(), 0, 1)
				res.Elem().Set(resp)
				return res.Interface()
			}
			return response
		}
		return responseHelper(val.Elem().Interface())
	case reflect.Slice:
		res := reflect.New(val.Type())
		resp := reflect.MakeSlice(val.Type(), 0, val.Len())
		// if this is not empty, copy it
		if !val.IsZero() {
			// iterate over each element in slice
			for i := 0; i < val.Len(); i++ {
				item := val.Index(i)
				var newItem reflect.Value
				switch item.Kind() {
				case reflect.Struct, reflect.Slice, reflect.Map:
					// recursively handle nested struct
					newItem = reflect.Indirect(reflect.ValueOf(responseHelper(item.Interface())))
				case reflect.Ptr:
					if item.IsNil() {
						if item.Type().Elem().Kind() == reflect.Slice {
							res := reflect.New(item.Type().Elem())
							resp := reflect.MakeSlice(item.Type().Elem(), 0, 1)
							res.Elem().Set(resp)
							newItem = res
							break
						}
						newItem = item
						break
					}
					if item.Elem().Kind() == reflect.Struct || item.Elem().Kind() == reflect.Slice {
						newItem = reflect.ValueOf(responseHelper(item.Elem().Interface()))
						break
					}
					fallthrough
				default:
					newItem = item
				}
				resp = reflect.Append(resp, newItem)
			}
		}
		res.Elem().Set(resp)
		return res.Interface()
	case reflect.Struct:
		resp := reflect.New(reflect.TypeOf(response))
		newVal := resp.Elem()
		// iterate over input's fields
		for i := 0; i < val.NumField(); i++ {
			newValField := newVal.Field(i)
			if !newValField.CanSet() {
				continue
			}
			valField := val.Field(i)
			updatedField := reflect.ValueOf(responseHelper(valField.Interface()))
			if valField.Kind() == reflect.Ptr {
				if updatedField.IsValid() {
					newValField.Set(updatedField)
				}
			} else {
				if updatedField.IsValid() {
					newValField.Set(reflect.Indirect(updatedField))
				}
			}
		}
		return resp.Interface()
	case reflect.Map:
		res := reflect.New(reflect.TypeOf(response))
		resp := reflect.MakeMap(reflect.TypeOf(response))
		// iterate over every key value pair in input map
		iter := val.MapRange()
		for iter.Next() {
			k := iter.Key()
			v := iter.Value()
			// recursively handle nested value
			newV := reflect.Indirect(reflect.ValueOf(responseHelper(v.Interface())))
			resp.SetMapIndex(k, newV)
		}
		res.Elem().Set(resp)
		return res.Interface()
	default:
		return response
	}
}
