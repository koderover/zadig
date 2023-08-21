package util

import (
	"fmt"
	"testing"
)

func TestGetWordsFromStringContainChinese(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
		{
			name: "test1",
			args: args{
				str: "hello世界la",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetWordsFromStringContainChinese(tt.args.str)
			fmt.Printf("-----------> %s\n", got)
		})
	}
}
