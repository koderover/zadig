package aliyun

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func HandleError(err error) error {
	var error = &tea.SDKError{}
	if _t, ok := err.(*tea.SDKError); ok {
		error = _t
	} else {
		error.Message = tea.String(err.Error())
	}

	respErr := fmt.Errorf("error: %s", tea.StringValue(error.Message))

	var data interface{}
	d := json.NewDecoder(strings.NewReader(tea.StringValue(error.Data)))
	d.Decode(&data)
	if m, ok := data.(map[string]interface{}); ok {
		recommend, _ := m["Recommend"]
		respErr = fmt.Errorf("%s, recommend: %s", respErr, recommend)
	}

	log.Error(err)
	return respErr
}
