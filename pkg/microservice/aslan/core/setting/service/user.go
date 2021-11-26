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

package service

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

type kubeCfgTmplArgs struct {
	CaCrtBase64     string
	CaKeyBase64     string
	KubeServerAddr  string
	Namespace       string
	User            string
	ClientCrtBase64 string
	ClientKeyBase64 string
}

func GetUserKubeConfigV2(userID string, editEnvProjects []string, readEnvProjects []string, log *zap.SugaredLogger) (string, error) {
	saNamespace := config.Namespace()
	if err := ensurClusterRole(); err != nil {
		return "", err
	}
	if err := ensureServiceAccount(saNamespace, editEnvProjects, readEnvProjects, userID, log); err != nil {
		return "", err
	}
	time.Sleep(time.Second)
	crt, token, err := getCrtAndToken(saNamespace, userID)
	if err != nil {
		return "", err
	}
	args := &kubeCfgTmplArgs{
		KubeServerAddr: config.KubeServerAddr(),
		CaCrtBase64:    crt,
		User:           userID,
		CaKeyBase64:    token,
	}

	return renderCfgTmplv2(args)
}

func getCrtAndToken(namespace, userID string) (string, string, error) {
	kubeClient := krkubeclient.Client()
	sa, found, err := getter.GetServiceAccount(namespace, userID+"-sa", kubeClient)
	if err != nil {
		return "", "", err
	} else if !found {
		return "", "", errors.New("sa not fonud")
	} else if len(sa.Secrets) == 0 {
		return "", "", errors.New("no secrets in sa")
	}
	time.Sleep(time.Second)
	secret, found, err := getter.GetSecret(namespace, sa.Secrets[0].Name, kubeClient)
	if err != nil {
		return "", "", err
	} else if !found {
		return "", "", errors.New("secret not found")
	}
	return base64.StdEncoding.EncodeToString(secret.Data["ca.crt"]), string(secret.Data["token"]), nil
}

func filterProductWithoutExternalCluster(products []*commonmodels.Product) []*commonmodels.Product {
	var ret []*commonmodels.Product
	kubeClient := krkubeclient.Client()
	for _, product := range products {
		// 过滤跨集群
		if product.ClusterID != "" {
			continue
		}
		// 过滤外部环境托管
		sources := sets.NewString(setting.SourceFromZadig, setting.HelmDeployType)
		// Compatible with the environment source created in the onboarding process is empty
		if product.Source != "" && !sources.Has(product.Source) {
			continue
		}
		// 过滤状态为Terminating的namespace
		namespace, found, err := getter.GetNamespace(product.Namespace, kubeClient)
		if !found || err != nil || wrapper.Namespace(namespace).Terminating() {
			continue
		}

		ret = append(ret, product)
	}

	return ret
}

func ensurClusterRole() error {
	if _, found, err := getter.GetClusterRole("zadig-env-edit", krkubeclient.Client()); err == nil && !found {
		if err := updater.CreateClusterRole(&rbacv1beta1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "zadig-env-edit",
			},
			Rules: []rbacv1beta1.PolicyRule{rbacv1beta1.PolicyRule{
				Verbs:     []string{"*"},
				APIGroups: []string{""},
				Resources: []string{"*"},
			}},
		}, krkubeclient.Client()); err != nil {
			fmt.Println(err)
		}
	}
	if _, found, err := getter.GetClusterRole("zadig-env-read", krkubeclient.Client()); err == nil && !found {
		updater.CreateClusterRole(&rbacv1beta1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "zadig-env-read",
			},
			Rules: []rbacv1beta1.PolicyRule{rbacv1beta1.PolicyRule{
				Verbs:     []string{"get", "watch", "list"},
				APIGroups: []string{""},
				Resources: []string{"*"},
			}},
		}, krkubeclient.Client())
	}
	return nil
}

func ensureServiceAccount(namespace string, editEnvProjects []string, readEnvProjects []string, userID string, log *zap.SugaredLogger) error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      userID + "-sa",
			Namespace: namespace,
		},
	}
	_, found, err := getter.GetServiceAccount(namespace, userID+"-sa", krkubeclient.Client())
	if err != nil {
		return err
	}
	if found && err == nil {
		return nil
	}
	// user's first time download kubeconfig
	//1. create serviceAccount
	if err := updater.CreateServiceAccount(serviceAccount, krkubeclient.Client()); err != nil {
		log.Errorf("CreateServiceAccount err: %+v", err)
		return nil
	}
	//2. create rolebinding
	for _, v := range editEnvProjects {
		products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: v})
		if err != nil {
			log.Errorf("[%s] Collections.Product.List error: %v", v, err)
		}
		products = filterProductWithoutExternalCluster(products)
		for _, vv := range products {

			rolebinding, found, err := getter.GetRoleBinding(vv.Namespace, "zadig-env-edit", krkubeclient.Client())
			subs := []rbacv1beta1.Subject{rbacv1beta1.Subject{
				Kind:      "ServiceAccount",
				Name:      fmt.Sprintf("%s-sa", userID),
				Namespace: namespace,
			}}
			if err == nil && found {
				subs = append(subs, rolebinding.Subjects...)
			}
			if err := updater.CreateOrPatchRoleBinding(&rbacv1beta1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "zadig-env-edit",
					Namespace: vv.Namespace,
				},
				Subjects: subs,
				RoleRef: rbacv1beta1.RoleRef{
					// APIGroup is the group for the resource being referenced
					APIGroup: "rbac.authorization.k8s.io",
					// Kind is the type of resource being referenced
					Kind: "ClusterRole",
					// Name is the name of resource being referenced
					Name: "zadig-env-edit",
				},
			}, krkubeclient.Client()); err != nil {
				log.Errorf("create rolebinding err: %s", err)
			}
		}
	}

	for _, v := range readEnvProjects {
		products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: v, IsSortByProductName: true})
		if err != nil {
			log.Errorf("[%s] Collections.Product.List error: %v", v, err)
		}
		products = filterProductWithoutExternalCluster(products)
		for _, vv := range products {
			rolebinding, found, err := getter.GetRoleBinding(vv.Namespace, "zadig-env-read", krkubeclient.Client())
			subs := []rbacv1beta1.Subject{rbacv1beta1.Subject{
				Kind:      "ServiceAccount",
				Name:      fmt.Sprintf("%s-sa", userID),
				Namespace: namespace,
			}}
			if err != nil {
				log.Errorf("GetRoleBinding err: %s", err)
				continue
			}
			if err == nil && found {
				subs = append(subs, rolebinding.Subjects...)
			}
			if err := updater.CreateOrPatchRoleBinding(&rbacv1beta1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "zadig-env-read",
					Namespace: vv.Namespace,
				},
				Subjects: subs,
				RoleRef: rbacv1beta1.RoleRef{
					// APIGroup is the group for the resource being referenced
					APIGroup: "rbac.authorization.k8s.io",
					// Kind is the type of resource being referenced
					Kind: "ClusterRole",
					// Name is the name of resource being referenced
					Name: "zadig-env-read",
				},
			}, krkubeclient.Client()); err != nil {
				log.Errorf("create rolebinding err: %s", err)
			}
		}
	}
	//
	//for _, v := range editEnvProjects {
	//	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: v, IsSortByProductName: true})
	//	if err != nil {
	//		log.Errorf("[%s] Collections.Product.List error: %v", v, err)
	//	}
	//	for _, vv := range products {
	//		if err := updater.CreateRoleBinding(&rbacv1beta1.RoleBinding{
	//			ObjectMeta: metav1.ObjectMeta{
	//				Name:      fmt.Sprintf("%s-%s-edit", userID, vv.Namespace),
	//				Namespace: vv.Namespace,
	//			},
	//			Subjects: []rbacv1beta1.Subject{rbacv1beta1.Subject{
	//				Kind:      "ServiceAccount",
	//				Name:      fmt.Sprintf("%s-sa", userID),
	//				Namespace: namespace,
	//			}},
	//			RoleRef: rbacv1beta1.RoleRef{
	//				// APIGroup is the group for the resource being referenced
	//				APIGroup: "rbac.authorization.k8s.io",
	//				// Kind is the type of resource being referenced
	//				Kind: "ClusterRole",
	//				// Name is the name of resource being referenced
	//				Name: "zadig-env-edit",
	//			},
	//		}, krkubeclient.Client()); err != nil {
	//			log.Errorf("create rolebinding err: %s", err)
	//		}
	//	}
	//
	//}
	//for _, v := range readEnvProjects {
	//	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: v, IsSortByProductName: true})
	//	if err != nil {
	//		log.Errorf("[%s] Collections.Product.List error: %v", v, err)
	//	}
	//	for _, vv := range products {
	//		if err := updater.CreateRoleBinding(&rbacv1beta1.RoleBinding{
	//			ObjectMeta: metav1.ObjectMeta{
	//				Name:      fmt.Sprintf("%s-%s-read", userID, vv.Namespace),
	//				Namespace: vv.Namespace,
	//			},
	//			Subjects: []rbacv1beta1.Subject{rbacv1beta1.Subject{
	//				Kind:      "ServiceAccount",
	//				Name:      fmt.Sprintf("%s-sa", userID),
	//				Namespace: namespace,
	//			}},
	//			RoleRef: rbacv1beta1.RoleRef{
	//				// APIGroup is the group for the resource being referenced
	//				APIGroup: "rbac.authorization.k8s.io",
	//				// Kind is the type of resource being referenced
	//				Kind: "ClusterRole",
	//				// Name is the name of resource being referenced
	//				Name: "zadig-env-read",
	//			},
	//		}, krkubeclient.Client()); err != nil {
	//			log.Errorf("create rolebinding err:%v", err)
	//		}
	//	}
	//
	//}

	return nil
}

func renderCfgTmplv2(args *kubeCfgTmplArgs) (string, error) {
	buf := new(bytes.Buffer)
	t := template.Must(template.New("cfgv2").Parse(kubeCfgTmplv2))
	err := t.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

const kubeCfgTmplv2 = `
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: {{.CaCrtBase64}}
    server: {{.KubeServerAddr}}
  name: koderover
contexts:
- context:
    cluster: koderover
    user: {{.User}}
  name: koderover
current-context: koderover
kind: Config
preferences: {}
users:
- name: {{.User}}
  user:
    token: {{.CaKeyBase64}}
`
