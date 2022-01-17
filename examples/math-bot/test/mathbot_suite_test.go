package mathbot_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMathBot(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MathBot Test Suite")
}
