/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package klock

import "github.com/pkg/errors"

var (
	ErrNotInit            = errors.New("klock not init")
	ErrCreateLockTimeout  = errors.New("create lock timeout")
	ErrCreateLockMaxRetry = errors.New("create lock reached max attempts")
	ErrUnlockMaxRetry     = errors.New("unlock reached max attempts")
	ErrLockExist          = errors.New("lock exist")
)
