package exec

import (
	"io/ioutil"
	"os/exec"
	"strings"
)

func GetCmdStdOut(cmdStr string) (string, error) {
	cmd := exec.Command("/bin/sh", "-c", cmdStr)

	// Create a get command output pipeline
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	//exec command
	if err := cmd.Start(); err != nil {
		return "", err
	}

	//read all output
	bytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		return "", err
	}

	if err := cmd.Wait(); err != nil {
		return "", err
	}

	sk := string(bytes)
	sk = strings.Replace(sk, "\n", "", -1)
	return sk, nil
}
