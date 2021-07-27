package service

import (
	"fmt"
	"testing"
)

func TestGetPodLogWithHttp(t *testing.T) {
	podName := "rocketmq-operator-867c4955-7zfmd"
	var tail int64
	tail = 10

	output, err := GetPodLogByHttp(podName, "", tail, nil)
	if err != nil {
		fmt.Errorf("GetPodLogByHttp error :%v \n", err)
	}
	fmt.Printf("run log:%v \n", output)
}
