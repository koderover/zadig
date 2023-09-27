/*
Copyright 2023 The KodeRover Authors.

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

package os

import (
	"fmt"
	"testing"
)

func TestGetHostIP(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:    "test1",
			want:    "10.10.10.155",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetHostIP()
			fmt.Printf("got: %v, err: %v\n", got, err)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetHostIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetHostIP() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOSName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		// TODO: Add test cases.
		{
			name: "test1",
			want: "darwin",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetOSName(); got != tt.want {
				fmt.Printf("got: %v, want: %v\n", got, tt.want)
				t.Errorf("GetOSName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOSArch(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		// TODO: Add test cases.
		{
			name: "test1",
			want: "arm64",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetOSArch(); got != tt.want {
				fmt.Printf("got: %v, want: %v\n", got, tt.want)
				t.Errorf("GetOSArch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetHostCurrentUser(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:    "test1",
			want:    "zhatiai",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetHostCurrentUser()
			fmt.Printf("got: %v, err: %v\n", got, err)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetHostCurrentUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetHostCurrentUser() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetHostTotalMem(t *testing.T) {
	tests := []struct {
		name    string
		want    uint64
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:    "test1",
			want:    17179869184,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetHostTotalMem()
			fmt.Printf("got: %v, err: %v\n", got, err)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetHostTotalMem() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetHostTotalMem() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetHostUsedMem(t *testing.T) {
	tests := []struct {
		name    string
		want    uint64
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:    "test1",
			want:    0,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetHostUsedMem()
			fmt.Printf("got: %v, err: %v\n", got, err)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetHostUsedMem() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetHostUsedMem() got = %v, want %v", got, tt.want)
			}
		})
	}
}
