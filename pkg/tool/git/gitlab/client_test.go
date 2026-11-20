package gitlab

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewClientDoesNotModifyDefaultHTTPClient(t *testing.T) {
	defaultTransport := &http.Transport{}
	previousTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = defaultTransport
	t.Cleanup(func() { http.DefaultClient.Transport = previousTransport })

	withoutVerification, err := NewClient(0, "https://gitlab.example.com/", "", "", false, true)
	require.NoError(t, err)
	withVerification, err := NewClient(0, "https://gitlab.example.com/", "", "", false, false)
	require.NoError(t, err)

	require.Same(t, defaultTransport, http.DefaultClient.Transport)
	require.NotNil(t, withoutVerification)
	require.NotNil(t, withVerification)
}
