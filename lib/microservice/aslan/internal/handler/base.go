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
	"reflect"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/poetry"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/types/permission"
)

// Context struct
type Context struct {
	Logger   *xlog.Logger
	Err      error
	Resp     interface{}
	Username string
	User     *permission.User
}

func NewContext(c *gin.Context) *Context {
	return &Context{
		Username: currentUsername(c),
		User:     currentUser(c),
		Logger:   Logger(c),
	}
}

// Logger is get logger func
func Logger(c *gin.Context) *xlog.Logger {
	logger := xlog.DefaultLogger(c)
	logger.SetLoggerLevel(config.LogLevel())
	return logger
}

// CurrentUser return current session user
func currentUser(c *gin.Context) *permission.User {
	userInfo, isExist := c.Get(setting.SessionUser)
	user, ok := userInfo.(*poetry.UserInfo)
	if isExist && ok {
		return poetry.ConvertUserInfo(user)
	}
	return permission.AnonymousUser
}

func currentUsername(c *gin.Context) (username string) {
	return c.GetString(setting.SessionUsername)
}

func JsonResponse(c *gin.Context, ctx *Context) {
	if ctx.Err != nil {
		c.JSON(e.ErrorMessage(ctx.Err))
		c.Abort()
		return
	}

	realResp := responseHelper(ctx.Resp)

	if ctx.Resp == nil {
		c.JSON(200, gin.H{"message": "success"})
	} else {
		c.JSON(200, realResp)
	}
	return
}

// 插入操作日志
func InsertOperationLog(c *gin.Context, username, productName, method, function, detail, permissionUUID string, requestBody string, logger *xlog.Logger) {
}

// responseHelper recursively finds all nil slice in the given interface,
// replacing them with empty slices.
// Drawbacks of this function is listed below to avoid possible misuse.
// 1. This function will return the pointer of the given structure, in case a
//    pointer a passed in, a pointer will be returned, not a double pointer
// 2. All private fields of the struct will be deleted during the process.
func responseHelper(response interface{}) interface{} {
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
