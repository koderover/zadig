package mathbot_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MathBot Test Suite")
}

type Params struct {
	Left  int
	Right int
}

type Result struct {
	Data int
}

var _ = Describe("Math Bot Test", func() {
	It("Plus", func() {
		Expect(2).Should(Equal(2))

		param := fmt.Sprintf(`{"left": %d, "right": %d}`, 20, 2)
		jsonStr := []byte(param)

		url := "http://localhost:8008/plus"
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		response, _ := client.Do(req)
		body, _ := ioutil.ReadAll(response.Body)
		var res Result
		json.Unmarshal(body, &res)
		log.Println(res)
		log.Println(res.Data)
	})
})
