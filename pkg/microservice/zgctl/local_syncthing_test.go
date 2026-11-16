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

package zgctl

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func TestStartLocalSyncthingUsesBinary(t *testing.T) {
	log.Init(&log.Config{Level: "error"})
	root := t.TempDir()
	marker := filepath.Join(root, "injected")
	argsPath := filepath.Join(root, "args")
	t.Setenv("TEST_MARKER", marker)
	t.Setenv("TEST_ARGS", argsPath)
	config := filepath.Join(root, `config ' ; $(touch $TEST_MARKER)`)
	data := filepath.Join(root, "data `touch $TEST_MARKER` space\n")
	bin := filepath.Join(root, "bin ' space", "syncthing")
	for _, dir := range []string{config, filepath.Dir(bin)} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	// A stale or replaced local script must never become the executable.
	if err := os.WriteFile(filepath.Join(config, "run.sh"), []byte("#!/bin/sh\ntouch \"$TEST_MARKER\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s\\0' \"$@\" > \"$TEST_ARGS.tmp\"\nmv \"$TEST_ARGS.tmp\" \"$TEST_ARGS\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (&zgctl{syncthingBin: bin}).startLocalSyncthing(ctx, config, data); err != nil {
		t.Fatal(err)
	}
	want := []string{"serve", "--config=" + config, "--data=" + data}
	deadline := time.Now().Add(3 * time.Second)
	for {
		content, err := os.ReadFile(argsPath)
		if err == nil {
			got := strings.Split(strings.TrimSuffix(string(content), "\x00"), "\x00")
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("arguments: got %q, want %q", got, want)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("binary did not record arguments: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("unexpected script execution: %v", err)
	}
}

func TestStartLocalSyncthingStartError(t *testing.T) {
	log.Init(&log.Config{Level: "error"})
	t.Setenv("PATH", t.TempDir())
	if err := (&zgctl{syncthingBin: "/missing/syncthing"}).startLocalSyncthing(context.Background(), "config", "data"); err == nil {
		t.Fatal("missing launcher error was lost")
	}
}
