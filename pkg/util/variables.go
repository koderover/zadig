package util

import (
	"fmt"
	"strings"
)

const (
	secretEnvMask = "********"
)

func MakeEnvMap(envs ...[]string) map[string]string {
	envMap := map[string]string{}
	for _, env := range envs {
		for _, env := range env {
			sl := strings.Split(env, "=")
			if len(sl) != 2 {
				continue
			}
			envMap[sl[0]] = sl[1]
		}
	}
	return envMap
}

func MaskSecretEnvs(message string, secretEnvs []string) string {
	out := message

	for _, val := range secretEnvs {
		if len(val) == 0 {
			continue
		}
		sl := strings.Split(val, "=")

		if len(sl) != 2 {
			continue
		}

		if len(sl[0]) == 0 || len(sl[1]) == 0 {
			// invalid key value pair received
			continue
		}
		out = strings.Replace(out, strings.Join(sl[1:], "="), secretEnvMask, -1)
	}
	return out
}

func MaskSecret(secrets []string, message string) string {
	out := message

	for _, val := range secrets {
		if len(val) == 0 {
			continue
		}
		out = strings.Replace(out, val, "********", -1)
	}
	return out
}

func ReplaceEnvWithValue(str string, envs map[string]string) string {
	ret := str
	// Exec twice to render nested variables
	for i := 0; i < 2; i++ {
		for key, value := range envs {
			strKey := fmt.Sprintf("$%s", key)
			ret = strings.ReplaceAll(ret, strKey, value)
			strKey = fmt.Sprintf("${%s}", key)
			ret = strings.ReplaceAll(ret, strKey, value)
			strKey = fmt.Sprintf("%%%s%%", key)
			ret = strings.ReplaceAll(ret, strKey, value)
			strKey = fmt.Sprintf("$env:%s", key)
			ret = strings.ReplaceAll(ret, strKey, value)
		}
	}
	return ret
}

func ReplaceEnvArrWithValue(str []string, envs map[string]string) []string {
	ret := []string{}
	for _, s := range str {
		ret = append(ret, ReplaceEnvWithValue(s, envs))
	}
	return ret
}
