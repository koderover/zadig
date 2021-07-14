package codehub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodeHubClient_RepoList(t *testing.T) {
	client := &CodeHubClient{
		AK: "",
		SK: "",
	}
	repoList, err := client.RepoList("c2b097cc06cb4539a350f966941c9165", "", 100)
	assert.Nil(t, err)
	assert.Equal(t, len(repoList), 3)
}
