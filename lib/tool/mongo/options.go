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

package mongo

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type urlInfo struct {
	addrs   []string
	user    string
	pass    string
	db      string
	options map[string]string
}

func isOptSep(c rune) bool {
	return c == ';' || c == '&'
}

func extractURL(s string) (*urlInfo, error) {
	if strings.HasPrefix(s, "mongodb://") {
		s = s[10:]
	}
	info := &urlInfo{options: make(map[string]string)}
	if c := strings.Index(s, "?"); c != -1 {
		for _, pair := range strings.FieldsFunc(s[c+1:], isOptSep) {
			l := strings.SplitN(pair, "=", 2)
			if len(l) != 2 || l[0] == "" || l[1] == "" {
				return nil, errors.New("connection option must be key=value: " + pair)
			}
			info.options[l[0]] = l[1]
		}
		s = s[:c]
	}
	if c := strings.Index(s, "@"); c != -1 {
		pair := strings.SplitN(s[:c], ":", 2)
		if len(pair) > 2 || pair[0] == "" {
			return nil, errors.New("credentials must be provided as user:pass@host")
		}
		var err error
		info.user, err = url.QueryUnescape(pair[0])
		if err != nil {
			return nil, fmt.Errorf("cannot unescape username in URL: %q", pair[0])
		}
		if len(pair) > 1 {
			info.pass, err = url.QueryUnescape(pair[1])
			if err != nil {
				return nil, fmt.Errorf("cannot unescape password in URL")
			}
		}
		s = s[c+1:]
	}
	if c := strings.Index(s, "/"); c != -1 {
		info.db = s[c+1:]
		s = s[:c]
	}
	info.addrs = strings.Split(s, ",")
	return info, nil
}

// ExtractDatabaseName extracts database name from uri if exists,
// and returns it together with the extracted uri (without database name)
// ref: https://docs.mongodb.com/manual/reference/connection-string/
func ExtractDatabaseName(uri string) (dbName, extractedURI string, err error) {
	extractedURI = uri
	info, err := extractURL(uri)
	if err != nil {
		return "", "", err
	}

	dbName = info.db
	if dbName != "" {
		connList := strings.Split(uri, "?")
		i := strings.LastIndex(connList[0], dbName)
		if i > -1 {
			connList[0] = connList[0][:i] + connList[0][i+len(dbName):]
		}
		extractedURI = strings.Join(connList, "?")
	}
	return
}
