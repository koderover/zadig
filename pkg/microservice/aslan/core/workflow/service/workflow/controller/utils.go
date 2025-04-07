package controller

import (
	"fmt"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"strings"
)

func renderMultiLineString(body string, inputs []*commonmodels.Param) string {
	for _, input := range inputs {
		var inputValue string
		if input.ParamsType == string(commonmodels.MultiSelectType) {
			inputValue = strings.Join(input.ChoiceValue, ",")
		} else {
			inputValue = input.Value
		}
		inputValue = strings.ReplaceAll(inputValue, "\n", "\\n")
		body = strings.ReplaceAll(body, fmt.Sprintf(setting.RenderValueTemplate, input.Name), inputValue)
	}
	return body
}
