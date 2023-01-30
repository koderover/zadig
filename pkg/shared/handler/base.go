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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"go.uber.org/zap"

	systemmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/system/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/util/ginzap"
)

// Context struct
type Context struct {
	Logger       *zap.SugaredLogger
	Err          error
	Resp         interface{}
	Account      string
	UserName     string
	UserID       string
	IdentityType string
	RequestID    string
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

// TODO: We need to implement a `context.Context` that conforms to the golang standard library.
func NewContext(c *gin.Context) *Context {
	logger := ginzap.WithContext(c).Sugar()
	var claims jwtClaims

	token := c.GetHeader(setting.AuthorizationHeader)
	if len(token) > 0 {
		var err error
		claims, err = getUserFromJWT(token)
		if err != nil {
			logger.Warnf("Failed to get user from token, err: %s", err)
		}
	} else {
		claims.Name = "system"
	}

	return &Context{
		UserName:     claims.Name,
		UserID:       claims.UID,
		Account:      claims.Account,
		IdentityType: claims.FederatedClaims.ConnectorId,
		Logger:       ginzap.WithContext(c).Sugar(),
		RequestID:    c.GetString(setting.RequestID),
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
	if ctx.Err != nil {
		c.Set(setting.ResponseError, ctx.Err)
		c.Abort()
		return
	}

	if ctx.Resp != nil {
		realResp := responseHelper(ctx.Resp)
		c.Set(setting.ResponseData, realResp)
	}
}

// InsertOperationLog 插入操作日志
func InsertOperationLog(c *gin.Context, username, productName, method, function, detail, requestBody string, logger *zap.SugaredLogger) {
	req := &systemmodels.OperationLog{
		Username:    username,
		ProductName: productName,
		Method:      method,
		Function:    function,
		Name:        detail,
		RequestBody: requestBody,
		Status:      0,
		CreatedAt:   time.Now().Unix(),
	}
	err := mongodb.NewOperationLogColl().Insert(req)
	if err != nil {
		logger.Errorf("InsertOperation err:%v", err)
	}
	c.Set("operationLogID", req.ID.Hex())
}

func InsertDetailedOperationLog(c *gin.Context, username, productName, scene, method, function, detail, requestBody string, logger *zap.SugaredLogger, targets ...string) {
	req := &systemmodels.OperationLog{
		Username:    username,
		ProductName: productName,
		Method:      method,
		Function:    function,
		Name:        detail,
		RequestBody: requestBody,
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
