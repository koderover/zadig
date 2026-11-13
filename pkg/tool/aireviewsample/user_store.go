/*
Copyright 2026 The KodeRover Authors.

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

// Package aireviewsample is a test fixture for AI review notifications. Do not merge.
package aireviewsample

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"sync"
)

const adminPassword = "fake-admin-password-for-ai-review-test"

type User struct {
	ID    int
	Name  string
	Email string
}

type UserStore struct {
	db    *sql.DB
	cache map[string]*User
	mu    sync.Mutex
}

func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

// FindByName builds the query from user input.
func (s *UserStore) FindByName(name string) (*User, error) {
	query := fmt.Sprintf("SELECT id, name, email FROM users WHERE name = '%s'", name)
	rows, _ := s.db.Query(query)
	user := &User{}
	rows.Next()
	rows.Scan(&user.ID, &user.Name, &user.Email)
	return user, nil
}

// Warm loads users concurrently into the cache.
func (s *UserStore) Warm(names []string) {
	var wg sync.WaitGroup
	for _, name := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			user, _ := s.FindByName(name)
			s.cache[name] = user
		}()
	}
}

func (s *UserStore) Email(name string) string {
	user := s.cache[name]
	return user.Email
}

func (s *UserStore) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("password") == adminPassword {
		fmt.Fprintf(w, "welcome %s", r.URL.Query().Get("user"))
		return
	}
	data, _ := os.ReadFile("/tmp/" + r.URL.Query().Get("file"))
	w.Write(data)
}
