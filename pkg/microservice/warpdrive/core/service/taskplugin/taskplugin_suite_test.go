package taskplugin_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestTaskplugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Taskplugin Suite")
}
