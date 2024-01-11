/*
Copyright 2023 The KodeRover Authors.

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
	"testing"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	registry2 "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/registry"

	"github.com/koderover/zadig/v2/pkg/tool/log"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetToken() {
	//ctx := context.Background()
	//conf := &jwt.Config{
	//	Email: "kr-test@fast-gateway-407902.iam.gserviceaccount.com",
	//	// The contents of your RSA private key or your PEM file
	//	// that contains a private key.
	//	// If you have a p12 file instead, you
	//	// can use `openssl` to export the private key into a pem file.
	//	//
	//	//    $ openssl pkcs12 -in key.p12 -out key.pem -nodes
	//	//
	//	// It only supports PEM containers with no passphrase.
	//	PrivateKey: []byte("-----BEGIN RSA PRIVATE KEY-----..."),
	//	Subject:    "user@example.com",
	//	TokenURL:   "https://provider.com/o/oauth2/token",
	//}
	//
	//iat := time.Now()
	//exp := iat.Add(time.Hour)
	//
	//cs := &jws.ClaimSet{
	//	Iss:   "kr-test@fast-gateway-407902.iam.gserviceaccount.com",
	//	Scope: "cloud-platform",
	//	Aud:   "https://oauth2.googleapis.com/token",
	//	Iat:   iat.Unix(),
	//	Exp:   exp.Unix(),
	//}

	//data, err := ioutil.ReadFile("/tmp/google.json")
	//if err != nil {
	//	log.Fatal(err)
	//}

}

func TestRegistryTags(t *testing.T) {
	log.Init(&log.Config{Level: "debug"})
	//registryInfo := &commonmodels.RegistryNamespace{
	//	ID:              primitive.ObjectID{},
	//	RegAddr:         "https://koderover.tencentcloudcr.com",
	//	RegType:         "",
	//	RegProvider:     "tcr",
	//	IsDefault:       false,
	//	Projects:        nil,
	//	Namespace:       "test",
	//	AccessKey:       "100026498911",
	//	SecretKey:       "eyJhbGciOiJSUzI1NiIsImtpZCI6IlpFS1I6V0oySzpQTDZaOktVN0U6MlpIUjpQWUVYOldPSUI6WVkzRzpDTzNKOkpPRk86VlJaTzpBU0dMIn0.eyJvd25lclVpbiI6IjEwMDAwODcxNTUxOSIsIm9wZXJhdG9yVWluIjoiMTAwMDI2NDk4OTExIiwidG9rZW5JZCI6ImNiNzNjdDVzdjdnb2puZnVsbDBnIiwiZXhwIjoxOTczMDQxNTI0LCJuYmYiOjE2NTc2ODE1MjQsImlhdCI6MTY1NzY4MTUyNH0.p4KRPXceKAr9hOsWsjsPSlJTc3YSh8Ucow7PWuPOIdBDbAjNevA7TQsdtezCWCybsJd4AXBd-q5XUNRiU2T2QoBRNR-8iH7onBx3OAgMmgT2tvKG3nHvTOAHAOy-wLtZOXyCOjqNwzCM7ldXiAgUNtXZdL3etKjI4dy0V3KtyKIbzFul9T8cujb_ynIRTVFy6WGsBCjjgegpKBgRlrUCZYLGW33vL7qdhrpnowUPl7inrBgycF8okqSYIIAZxgtabSua94yGAQjGkbi9FPXNA8St42p6eluz17dBWJ75Mfjv5hS7vujvxOGhBa32NX-CsGlySruRLryZfE_JYqSDGA",
	//	Region:          "",
	//	UpdateTime:      0,
	//	UpdateBy:        "",
	//	AdvancedSetting: nil,
	//}

	registryInfo := &commonmodels.RegistryNamespace{
		ID:              primitive.ObjectID{},
		RegAddr:         "https://asia-east2-docker.pkg.dev",
		RegType:         "",
		RegProvider:     "gcp",
		IsDefault:       false,
		Projects:        nil,
		Namespace:       "fast-gateway-407902/test",
		AccessKey:       "_json_key_base64",
		SecretKey:       "ewogICJ0eXBlIjogInNlcnZpY2VfYWNjb3VudCIsCiAgInByb2plY3RfaWQiOiAiZmFzdC1nYXRld2F5LTQwNzkwMiIsCiAgInByaXZhdGVfa2V5X2lkIjogIjE3M2FjMTg2OTRiNjgwNDNiZTAxOTUxZmI1NGIyOGMyNWMzNzExMjEiLAogICJwcml2YXRlX2tleSI6ICItLS0tLUJFR0lOIFBSSVZBVEUgS0VZLS0tLS1cbk1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRQzdMck1HQ25jeTNVVnRcblRROCtPRG1ZbHl1WDRlSTMzQXRFbW0rT0hicGxkUDM0ajZObmVkeXY4eGRBYnRPRzVsZ21XK3hLSFJRTlo5dkFcblAxc2J5UCtxT2ltUFZvdURDM2x3OTBxK0VIMmN1anBJeW1iTG5LTGRha0ZLRGlUSXp5S0hQUTlQY1RHSlhHbFJcbmVRQXh6MG5tSnlBbTdhMENFcEMwUElsZEw5R2VhY2ZnQTZWZUNZM2dWNHlyUGxjL0Izdy9GUlgzR2ZKa2NxUWRcblVzWmJ6VE56MEpVSWk4OEhkYStXaGY5TGg0dk1jcjREWkVrQ0RRdVdoZFVHWWRvRjNabXAwYVNFWGZZV2Y2UitcbllZYnFTSGl6bUYwaTZ1S3VuOEJFbmc3NDYyY052VndOS282TENQRlBLNVd0QTNXZ1FEWXcyR2lwZ3ZrZXRvYWtcbjlCdXZsVXp4QWdNQkFBRUNnZ0VBTEkwb2Fnc3FwTTRhbkxISEs0RjVYL0szR3Y3Vjk0S2xvZTM2R0VMR0h5alJcbjNBdmdFaHhrcFBKQWVnTUcwS2habWNPdVFWT2JkcmRlQyt0b2RYd0pNZ3lHNE1seUNqNDhhT0ZMQm1McGF0aStcblQ1M3hDb3hkRFVNaGlBMUd2dzdpQW50TGZoYU1la3VtKzQvSHRwTkdYUU81K05vQUlDcW9DMllQTWNGVWJKWUxcbmZSankvRzFSU2M1L0xQNmxaZzlvYnpjU3lUbjJqQ1dEajlnRTNaaFh2c04vRVRJa05MK1F1d0N3em5GR2dIRXFcblBDU3ZvcnVSem1wOUVFNnlDTFAwM3ZFT01SWC9QSnAyNXo0bURQWG1uVCs1YUlmdStCMTJkRnYyQ2lFUmp5TjZcbmljSXI0Yi8xRWc4c1pnYnBBMUE2ZXVtUmhHNmdVKzVFOG5ETVVnMHF5d0tCZ1FEbGZzY3lWVXYxMGdteFpwTThcbmQ2bkNSbVNYdSs4bkVUQzVtc0ZlWWZOWlVqUXYrK3l6N0hLdEoyZlJ3Z0FGUWF1YjByd0U4R1VZdHhIZlZVUzdcbnZVcjA3RnFVNUxFMytrWGdreDdLNkpNYjR0UWIwVFZRbG5TY2Rmd1RXRExQVzhPQm0vZ0xHT3pqb1VxWWwzNEhcbm9nbzBBODBXczA3YmZmRExHUEZYSGZWMk53S0JnUURRek9nd2l5c3VNbyszcmVkcmV3ZVd5bWh0djZ3WEkwMFVcbnRRcVFKbmlwNzZwdCszNEpvdzJldjFreXJoNUU2RmppVjgwQjVkVkU5UGJMa3dDQndkMVpWbXl4M0ZBN1pZU0Jcbk01UWRxZHRRVHFYbUhVcmVmN2hZblhzT1F4Q3VFOXNGVUcrVVlEY1IvSzg2aHRUblA0akk5Q3NQMG5DWTU1SktcbnJlV3dvNTdDRndLQmdRQ1ZoWnNzL1F2bmxqaEFmKzlRQnpyd1c4S3daWDYwZW11L2tjZUl3ZEsyRUd2MkUzSXRcbjY5RHZaZXdyYXZWdWQxSGl6Vk00K0pNMW5oa2o1RDlLL2xLMjdzTTVuU0tsc1FjVUFXYWZseFk1cGZqQ1F2VTBcbmswSllxanBaTkM2dWtULzQwdkN4OGtSdExxb1dieVZxdmJWZUhGZmtBV0ZRZW1hSFBMSUpLM2pBMHdLQmdIK3FcbmVlR01oaWRtQk5leS9nZUtudlpFNWhzTWtlVkgwVTV5NzNWNkFGY3ZVZzZUTWRva2x5UlVMTzYrNVlVT1o2Smxcbk90VUpPU0JEZzA2dm9DUzJhMmUvWHhCVSs3MkZjY0lwemt0ZzJ0YThiOVZHWGN1elhmell0Uy9nTTZlc1BrTitcbmplcXo5WmdLM3YwekNhUW5CYlNSRG05TEpVdG9jOXN6Zm5oRllzR2JBb0dBY2IvYWF0anBRZUM4Z3FRaWRhM2FcbkVwY05uUzh2TTEzbGZxQjlaSEZFT0FlLzlRL1paSks0U1doSkp6T1RXb21TaGpLU2R5M1BnOUZ6L0ZGNjN4NGRcbitIT0dzT01YeDJwVk1LS2VzMTdBa1drcEF4ajRyS0srWXp0NXZocVF3ZUs4QTZqS2NpWnNnTit3TFdUWUZIMzdcbktmRWtwQ05QYlRKb0VJK2E0bk5nU3ZVPVxuLS0tLS1FTkQgUFJJVkFURSBLRVktLS0tLVxuIiwKICAiY2xpZW50X2VtYWlsIjogImtyLXRlc3RAZmFzdC1nYXRld2F5LTQwNzkwMi5pYW0uZ3NlcnZpY2VhY2NvdW50LmNvbSIsCiAgImNsaWVudF9pZCI6ICIxMTMzNzE4MjUzMzg3NzcxMjAzMDIiLAogICJhdXRoX3VyaSI6ICJodHRwczovL2FjY291bnRzLmdvb2dsZS5jb20vby9vYXV0aDIvYXV0aCIsCiAgInRva2VuX3VyaSI6ICJodHRwczovL29hdXRoMi5nb29nbGVhcGlzLmNvbS90b2tlbiIsCiAgImF1dGhfcHJvdmlkZXJfeDUwOV9jZXJ0X3VybCI6ICJodHRwczovL3d3dy5nb29nbGVhcGlzLmNvbS9vYXV0aDIvdjEvY2VydHMiLAogICJjbGllbnRfeDUwOV9jZXJ0X3VybCI6ICJodHRwczovL3d3dy5nb29nbGVhcGlzLmNvbS9yb2JvdC92MS9tZXRhZGF0YS94NTA5L2tyLXRlc3QlNDBmYXN0LWdhdGV3YXktNDA3OTAyLmlhbS5nc2VydmljZWFjY291bnQuY29tIiwKICAidW5pdmVyc2VfZG9tYWluIjogImdvb2dsZWFwaXMuY29tIgp9",
		Region:          "asia-east2",
		UpdateTime:      0,
		UpdateBy:        "",
		AdvancedSetting: nil,
	}

	regService := registry2.NewV2Service(registryInfo.RegProvider, registryInfo)

	endPoint := registry2.Endpoint{
		Addr:      registryInfo.RegAddr,
		Ak:        registryInfo.AccessKey,
		Sk:        registryInfo.SecretKey,
		Namespace: registryInfo.Namespace,
		Region:    registryInfo.Region,
	}
	repos, err := regService.ListRepoImages(registry2.ListRepoImagesOption{
		Endpoint: endPoint,
		Repos:    []string{"service1"},
	}, log.SugaredLogger())
	if err != nil {
		log.Infof(" err: %s", err)
	} else {
		log.Infof("tag count: %d", len(repos.Repos))
	}
	return
}
