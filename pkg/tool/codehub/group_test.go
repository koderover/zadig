package codehub

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHttpClient_GroupList(t *testing.T) {
	client := &CodeHubClient{
		AK: os.Getenv("AK"),
		SK: os.Getenv("SK"),
	}
	namespaces, err := client.NamespaceList()
	assert.Nil(t, err)
	assert.Equal(t, len(namespaces), 1)
}
