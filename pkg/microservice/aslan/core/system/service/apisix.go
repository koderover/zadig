/*
Copyright 2025 The KodeRover Authors.

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
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/apisix"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// Simplified response types that only contain the values
type SimpleRouteListResponse struct {
	List  []*apisix.Route `json:"list"`
	Total int             `json:"total"`
}

type SimpleUpstreamListResponse struct {
	List  []*apisix.Upstream `json:"list"`
	Total int                `json:"total"`
}

type SimpleServiceListResponse struct {
	List  []*apisix.Service `json:"list"`
	Total int               `json:"total"`
}

type SimpleProtoListResponse struct {
	List  []*apisix.Proto `json:"list"`
	Total int             `json:"total"`
}

// getApisixClient retrieves the API gateway configuration and creates an APISIX client
func getApisixClient(id string, log *zap.SugaredLogger) (*apisix.Client, error) {
	gateway, err := mongodb.NewApiGatewayColl().GetByID(id)
	if err != nil {
		log.Errorf("Failed to get API gateway: %v", err)
		return nil, e.ErrListApiGateway.AddErr(err)
	}

	return apisix.NewClient(gateway.Address, gateway.Token), nil
}

// ListApisixRoutes lists all routes from the APISIX gateway
func ListApisixRoutes(id string, page, pageSize int, log *zap.SugaredLogger) (*SimpleRouteListResponse, error) {
	client, err := getApisixClient(id, log)
	if err != nil {
		return nil, err
	}

	resp, err := client.ListRoutes(page, pageSize)
	if err != nil {
		log.Errorf("Failed to list APISIX routes: %v", err)
		return nil, e.ErrListApiGateway.AddErr(err)
	}

	// Extract only the values from the response
	routes := make([]*apisix.Route, 0, len(resp.List))
	for _, item := range resp.List {
		if item.Value != nil {
			routes = append(routes, item.Value)
		}
	}

	return &SimpleRouteListResponse{
		List:  routes,
		Total: resp.Total,
	}, nil
}

// ListApisixUpstreams lists all upstreams from the APISIX gateway
func ListApisixUpstreams(id string, page, pageSize int, log *zap.SugaredLogger) (*SimpleUpstreamListResponse, error) {
	client, err := getApisixClient(id, log)
	if err != nil {
		return nil, err
	}

	resp, err := client.ListUpstreams(page, pageSize)
	if err != nil {
		log.Errorf("Failed to list APISIX upstreams: %v", err)
		return nil, e.ErrListApiGateway.AddErr(err)
	}

	// Extract only the values from the response
	upstreams := make([]*apisix.Upstream, 0, len(resp.List))
	for _, item := range resp.List {
		if item.Value != nil {
			upstreams = append(upstreams, item.Value)
		}
	}

	return &SimpleUpstreamListResponse{
		List:  upstreams,
		Total: resp.Total,
	}, nil
}

// ListApisixServices lists all services from the APISIX gateway
func ListApisixServices(id string, page, pageSize int, log *zap.SugaredLogger) (*SimpleServiceListResponse, error) {
	client, err := getApisixClient(id, log)
	if err != nil {
		return nil, err
	}

	resp, err := client.ListServices(page, pageSize)
	if err != nil {
		log.Errorf("Failed to list APISIX services: %v", err)
		return nil, e.ErrListApiGateway.AddErr(err)
	}

	// Extract only the values from the response
	services := make([]*apisix.Service, 0, len(resp.List))
	for _, item := range resp.List {
		if item.Value != nil {
			services = append(services, item.Value)
		}
	}

	return &SimpleServiceListResponse{
		List:  services,
		Total: resp.Total,
	}, nil
}

// ListApisixProtos lists all protos from the APISIX gateway
func ListApisixProtos(id string, page, pageSize int, log *zap.SugaredLogger) (*SimpleProtoListResponse, error) {
	client, err := getApisixClient(id, log)
	if err != nil {
		return nil, err
	}

	resp, err := client.ListProtos(page, pageSize)
	if err != nil {
		log.Errorf("Failed to list APISIX protos: %v", err)
		return nil, e.ErrListApiGateway.AddErr(err)
	}

	// Extract only the values from the response
	protos := make([]*apisix.Proto, 0, len(resp.List))
	for _, item := range resp.List {
		if item.Value != nil {
			protos = append(protos, item.Value)
		}
	}

	return &SimpleProtoListResponse{
		List:  protos,
		Total: resp.Total,
	}, nil
}
