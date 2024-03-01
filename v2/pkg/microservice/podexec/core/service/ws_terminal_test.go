/*
Copyright 2021 The KodeRover Authors.

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

package service

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
)

func TestGetProduct(t *testing.T) {
	url := "http://xxx.com/api/aslan/products/autoproject?envName=dev"
	reqest, _ := http.NewRequest("GET", url, nil)
	reqest.Header.Add("Cookie", "gr_user_id=9f5632f8-8415-42f0-af48-72490a38e22c; grwng_uid=d55b9f07-0d0c-4f02-a1f2-47210b043878; SESSION=U2FsdGVkX19OXw6vLCchP5nhR0DrYxqvlQY/g4o3StVOWT13TL1CsFdZZUm8FcMz; a26f380058b199e6_gr_session_id=ff866cdd-c899-40e1-bb4e-2f82ed501c97; 89ca5e69c6b378e1_gr_session_id=f90ee041-efe4-4f52-aad1-819d97a03a17; 89ca5e69c6b378e1_gr_session_id_f90ee041-efe4-4f52-aad1-819d97a03a17=true")

	client := &http.Client{}
	resp, err := client.Do(reqest)
	if err != nil {
		t.Errorf("get product info err:%v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	ret, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("read body:%v", err)
	}
	t.Logf("body:%v", string(ret))

	result := &ProductResp{}
	err = json.Unmarshal(ret, result)
	if err != nil {
		t.Errorf("unmarsha resp body err :%v", err)
	}
	t.Logf("result:%v", *result)
}
