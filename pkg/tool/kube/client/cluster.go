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

package client

import (
	"context"
	"istio.io/client-go/pkg/clientset/versioned/typed/networking/v1alpha3"
	"sync"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

var once sync.Once

var c cluster.Cluster

// Cluster is a singleton, it will be initialized only once.
func Cluster() cluster.Cluster {
	once.Do(func() {
		var err error
		c, err = initCluster(ctrl.GetConfigOrDie())
		if err != nil {
			panic(err)
		}
	})

	return c
}

func NewClientSet() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func NewDynamicClient() (dynamic.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(config)
}

func NewIstioV1Alpha3Client() (*v1alpha3.NetworkingV1alpha3Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return v1alpha3.NewForConfig(config)
}

func NewDiscoveryClient() (*discovery.DiscoveryClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return discovery.NewDiscoveryClientForConfig(config)
}

func Client() client.Client {
	return Cluster().GetClient()
}

func APIReader() client.Reader {
	return Cluster().GetAPIReader()
}

func RESTConfig() *rest.Config {
	return Cluster().GetConfig()
}

func Scheme() *runtime.Scheme {
	return Cluster().GetScheme()
}

func Start(ctx context.Context) error {
	return Cluster().Start(ctx)
}

func RESTConfigFromAPIConfig(cfg *api.Config) (*rest.Config, error) {
	// use the current context in kubeconfig
	return clientcmd.BuildConfigFromKubeconfigGetter("", func() (config *api.Config, err error) {
		return cfg, nil
	})
}

func NewClientFromAPIConfig(cfg *api.Config) (client.Client, error) {
	restConfig, err := RESTConfigFromAPIConfig(cfg)
	if err != nil {
		return nil, err
	}

	cls, err := initCluster(restConfig)
	if err != nil {
		return nil, err
	}

	return newAPIClient(cls.GetClient(), cls.GetAPIReader()), nil
}

func GetKubeClientFromRestConfig(cfg *rest.Config) (client.Client, error) {
	cls, err := initCluster(cfg)
	if err != nil {
		return nil, err
	}

	return newAPIClient(cls.GetClient(), cls.GetAPIReader()), nil
}

func NewAPIClient(c client.Client, r client.Reader) client.Client {
	return newAPIClient(c, r)
}

// apiClient is similar with the default Client(), but it always gets objects from API server.
type apiClient struct {
	client.Client

	apiReader client.Reader
}

func newAPIClient(c client.Client, r client.Reader) client.Client {
	return &apiClient{Client: c, apiReader: r}
}

func (c *apiClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.apiReader.Get(ctx, key, obj, opts...)
}

func (c *apiClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return c.apiReader.List(ctx, list, opts...)
}

func initCluster(restConfig *rest.Config) (cluster.Cluster, error) {
	scheme := runtime.NewScheme()

	// add all known types
	// if you want to support custom types, call _ = yourCustomAPIGroup.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)

	c, err := cluster.New(restConfig, func(clusterOptions *cluster.Options) {
		clusterOptions.Scheme = scheme
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to init client")
	}

	return c, nil
}
