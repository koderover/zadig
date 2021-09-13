package json

import (
	"encoding/json"

	"helm.sh/helm/v3/pkg/strvals"
)

// ToJSON takes a string of arguments(in this format: a=b,c.d=e) and converts to a JSON document.
func ToJSON(s string) ([]byte, error) {
	m, err := strvals.Parse(s)
	if err != nil {
		return nil, err
	}

	return json.Marshal(m)
}
