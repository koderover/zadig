package exec

import (
	"io/ioutil"
	"os/exec"
)

func GetCmdStdOut(cmdStr string) (string, error) {
	cmd := exec.Command("/bin/sh", "-c", cmdStr)

	//创建获取命令输出管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	//执行命令
	if err := cmd.Start(); err != nil {
		return "", err
	}

	//读取所有输出
	bytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		return "", err
	}

	if err := cmd.Wait(); err != nil {
		return "", err
	}
	return string(bytes), nil
}
