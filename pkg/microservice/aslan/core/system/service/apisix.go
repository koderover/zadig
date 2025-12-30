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
func ListApisixRoutes(id string, page, pageSize int, log *zap.SugaredLogger) (*apisix.RouteListResponse, error) {
	client, err := getApisixClient(id, log)
	if err != nil {
		return nil, err
	}

	resp, err := client.ListRoutes(page, pageSize)
	if err != nil {
		log.Errorf("Failed to list APISIX routes: %v", err)
		return nil, e.ErrListApiGateway.AddErr(err)
	}

	return resp, nil
}

// CreateApisixRoute creates a new route in the APISIX gateway
func CreateApisixRoute(id string, route *apisix.Route, log *zap.SugaredLogger) (*apisix.RouteResponse, error) {
	client, err := getApisixClient(id, log)
	if err != nil {
		return nil, err
	}

	resp, err := client.CreateRoute(route)
	if err != nil {
		log.Errorf("Failed to create APISIX route: %v", err)
		return nil, e.ErrCreateApiGateway.AddErr(err)
	}

	return resp, nil
}

// DeleteApisixRoute deletes a route from the APISIX gateway
func DeleteApisixRoute(id, routeID string, log *zap.SugaredLogger) error {
	client, err := getApisixClient(id, log)
	if err != nil {
		return err
	}

	if err := client.DeleteRoute(routeID); err != nil {
		log.Errorf("Failed to delete APISIX route: %v", err)
		return e.ErrDeleteApiGateway.AddErr(err)
	}

	return nil
}

// ListApisixUpstreams lists all upstreams from the APISIX gateway
func ListApisixUpstreams(id string, page, pageSize int, log *zap.SugaredLogger) (*apisix.UpstreamListResponse, error) {
	client, err := getApisixClient(id, log)
	if err != nil {
		return nil, err
	}

	resp, err := client.ListUpstreams(page, pageSize)
	if err != nil {
		log.Errorf("Failed to list APISIX upstreams: %v", err)
		return nil, e.ErrListApiGateway.AddErr(err)
	}

	return resp, nil
}

// CreateApisixUpstream creates a new upstream in the APISIX gateway
func CreateApisixUpstream(id string, upstream *apisix.Upstream, log *zap.SugaredLogger) (*apisix.UpstreamResponse, error) {
	client, err := getApisixClient(id, log)
	if err != nil {
		return nil, err
	}

	resp, err := client.CreateUpstream(upstream)
	if err != nil {
		log.Errorf("Failed to create APISIX upstream: %v", err)
		return nil, e.ErrCreateApiGateway.AddErr(err)
	}

	return resp, nil
}

// DeleteApisixUpstream deletes an upstream from the APISIX gateway
func DeleteApisixUpstream(id, upstreamID string, log *zap.SugaredLogger) error {
	client, err := getApisixClient(id, log)
	if err != nil {
		return err
	}

	if err := client.DeleteUpstream(upstreamID); err != nil {
		log.Errorf("Failed to delete APISIX upstream: %v", err)
		return e.ErrDeleteApiGateway.AddErr(err)
	}

	return nil
}

// ListApisixServices lists all services from the APISIX gateway
func ListApisixServices(id string, page, pageSize int, log *zap.SugaredLogger) (*apisix.ServiceListResponse, error) {
	client, err := getApisixClient(id, log)
	if err != nil {
		return nil, err
	}

	resp, err := client.ListServices(page, pageSize)
	if err != nil {
		log.Errorf("Failed to list APISIX services: %v", err)
		return nil, e.ErrListApiGateway.AddErr(err)
	}

	return resp, nil
}

// CreateApisixService creates a new service in the APISIX gateway
func CreateApisixService(id string, service *apisix.Service, log *zap.SugaredLogger) (*apisix.ServiceResponse, error) {
	client, err := getApisixClient(id, log)
	if err != nil {
		return nil, err
	}

	resp, err := client.CreateService(service)
	if err != nil {
		log.Errorf("Failed to create APISIX service: %v", err)
		return nil, e.ErrCreateApiGateway.AddErr(err)
	}

	return resp, nil
}

// DeleteApisixService deletes a service from the APISIX gateway
func DeleteApisixService(id, serviceID string, log *zap.SugaredLogger) error {
	client, err := getApisixClient(id, log)
	if err != nil {
		return err
	}

	if err := client.DeleteService(serviceID); err != nil {
		log.Errorf("Failed to delete APISIX service: %v", err)
		return e.ErrDeleteApiGateway.AddErr(err)
	}

	return nil
}

// ListApisixProtos lists all protos from the APISIX gateway
func ListApisixProtos(id string, page, pageSize int, log *zap.SugaredLogger) (*apisix.ProtoListResponse, error) {
	client, err := getApisixClient(id, log)
	if err != nil {
		return nil, err
	}

	resp, err := client.ListProtos(page, pageSize)
	if err != nil {
		log.Errorf("Failed to list APISIX protos: %v", err)
		return nil, e.ErrListApiGateway.AddErr(err)
	}

	return resp, nil
}

// CreateApisixProto creates a new proto in the APISIX gateway
func CreateApisixProto(id string, proto *apisix.Proto, log *zap.SugaredLogger) (*apisix.ProtoResponse, error) {
	client, err := getApisixClient(id, log)
	if err != nil {
		return nil, err
	}

	resp, err := client.CreateProto(proto)
	if err != nil {
		log.Errorf("Failed to create APISIX proto: %v", err)
		return nil, e.ErrCreateApiGateway.AddErr(err)
	}

	return resp, nil
}

// DeleteApisixProto deletes a proto from the APISIX gateway
func DeleteApisixProto(id, protoID string, log *zap.SugaredLogger) error {
	client, err := getApisixClient(id, log)
	if err != nil {
		return err
	}

	if err := client.DeleteProto(protoID); err != nil {
		log.Errorf("Failed to delete APISIX proto: %v", err)
		return e.ErrDeleteApiGateway.AddErr(err)
	}

	return nil
}
