package util

import (
	"fmt"
	"os"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func RenderEnv(data string, kvs []*commonmodels.KeyVal) string {
	mapper := func(data string) string {
		for _, envar := range kvs {
			if data != envar.Key {
				continue
			}

			return envar.Value
		}

		return fmt.Sprintf("$%s", data)
	}
	return os.Expand(data, mapper)
}
