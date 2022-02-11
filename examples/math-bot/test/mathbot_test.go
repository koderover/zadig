package mathbot_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type Result struct {
	Data int
}

func Executor(left, right int, Operator string) int {
	m := map[string]string{
		"+": "http://math-bot.math-bot-env-dev:8008/plus",
		"-": "http://math-bot.math-bot-env-dev:8008/minus",
		"*": "http://math-bot.math-bot-env-dev:8008/times",
		"/": "http://math-bot.math-bot-env-dev:8008/divide",
	}
	param := fmt.Sprintf(`{"left": %d, "right": %d}`, left, right)
	jsonStr := []byte(param)

	req, _ := http.NewRequest("POST", m[Operator], bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	response, _ := client.Do(req)
	body, _ := ioutil.ReadAll(response.Body)

	var res Result
	json.Unmarshal(body, &res)
	return res.Data
}

var _ = Describe("Math Bot Test", func() {
	It("20 + 2 = 22", func() {
		Expect(Executor(20, 2, "+")).Should(Equal(22))
	})

	It("20 * 2 = 40", func() {
		Expect(Executor(20, 2, "*")).Should(Equal(40))
	})

	It("20 - 2 = 18", func() {
		Expect(Executor(20, 2, "-")).Should(Equal(18))
	})

	It("20 / 2 = 10", func() {
		Expect(Executor(20, 2, "/")).Should(Equal(10))
	})

	It("Demo", func() {
		Expect(1).Should(Equal(1))
	})
})
