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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
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
	e "github.com/koderover/zadig/pkg/tool/errors"
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

func GetUserKubeConfig(userName string, log *zap.SugaredLogger) (string, error) {
	username := strings.ToLower(userName)
	username = config.NameSpaceRegex.ReplaceAllString(username, "-")
	var (
		err         error
		productEnvs = make([]*commonmodels.Product, 0)
	)

	productEnvs, err = commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{})
	if err != nil {
		log.Errorf("GetUserKubeConfig Collection.Product.List error: %v", err)
		return "", e.ErrListProducts.AddDesc(err.Error())
	}

	// 只管理同集群的资源，且排除状态为Terminating的namespace
	productEnvs = filterProductWithoutExternalCluster(productEnvs)

	saNamespace := config.Namespace()
	if err := ensureServiceAccount(saNamespace, nil, nil, username, log); err != nil {
		return "", err
	}

	var (
		wg      sync.WaitGroup
		pool    = make(chan int, 20)
		errList = new(multierror.Error)
	)
	for _, productEnv := range productEnvs {
		namespace := productEnv.Namespace

		if _, found, err := getter.GetNamespace(namespace, krkubeclient.Client()); err != nil || !found {
			log.Error(err)
			continue
		}

		wg.Add(1)
		pool <- 1
		go func() {
			defer func() {
				wg.Done()
				<-pool
			}()
			if err := ensureUserRole(namespace, username, log); err != nil {
				log.Error(err)
				errList = multierror.Append(errList, err)
			}

			if err := ensureUserRoleBinding(saNamespace, namespace, username); err != nil {
				log.Error(err)
				errList = multierror.Append(errList, err)
			}
		}()
	}
	wg.Wait()
	close(pool)
	if err := errList.ErrorOrNil(); err != nil {
		log.Error(err)
		return "", err
	}

	if err := createK8sSSLCert(saNamespace, username); err != nil {
		log.Errorf("[%s] createSSLCert error: %v", username, err)
		return "", err
	}

	caCrtBase64, err := ioutil.ReadFile(filepath.Join(os.TempDir(), username, "ca.crt"))
	if err != nil {
		return "", fmt.Errorf("get client.crt error: %v", err)
	}

	caKeyBase64, err := ioutil.ReadFile(filepath.Join(os.TempDir(), username, "ca.key"))
	if err != nil {
		return "", fmt.Errorf("get client.key error: %v", err)
	}

	args := &kubeCfgTmplArgs{
		KubeServerAddr: config.KubeServerAddr(),
		User:           username,
		CaCrtBase64:    string(caCrtBase64),
		CaKeyBase64:    string(caKeyBase64),
	}

	return renderCfgTmpl(args)
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
		products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: v, IsSortByProductName: true})
		if err != nil {
			log.Errorf("[%s] Collections.Product.List error: %v", v, err)
		}
		for _, vv := range products {

			rolebinding, found, err := getter.GetRoleBinding(vv.Namespace, "zadig-env-edit", krkubeclient.Client())
			subs := []rbacv1beta1.Subject{}
			if err == nil && found {
				subs = append(rolebinding.Subjects, rbacv1beta1.Subject{
					Kind:      "ServiceAccount",
					Name:      fmt.Sprintf("%s-sa", userID),
					Namespace: namespace,
				})
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
		for _, vv := range products {
			rolebinding, found, err := getter.GetRoleBinding(vv.Namespace, "zadig-env-read", krkubeclient.Client())
			subs := []rbacv1beta1.Subject{}
			if err == nil && found {
				subs = append(rolebinding.Subjects, rbacv1beta1.Subject{
					Kind:      "ServiceAccount",
					Name:      fmt.Sprintf("%s-sa", userID),
					Namespace: namespace,
				})
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

func ensureUserRole(namespace, username string, _ *zap.SugaredLogger) error {
	roleName := fmt.Sprintf("%s-role", username)
	verbs := []string{"*"}
	role := &rbacv1beta1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
		Rules: []rbacv1beta1.PolicyRule{
			rbacv1beta1.PolicyRule{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     verbs,
			},
			rbacv1beta1.PolicyRule{
				APIGroups: []string{"*"},
				Resources: []string{
					"limitranges",
					"resourcequotas",
				},
				Verbs: []string{"get", "list", "watch"},
			},
		},
	}

	old, found, err := getter.GetRole(namespace, roleName, krkubeclient.Client())
	if err == nil && found {
		old.Rules = role.Rules
		if err := updater.UpdateRole(old, krkubeclient.Client()); err != nil {
			return err
		}
	} else {
		if err := updater.CreateRole(role, krkubeclient.Client()); err != nil {
			return err
		}
	}

	return nil
}

func ensureUserRoleBinding(saNamespace, namespace, username string) error {
	roleName := fmt.Sprintf("%s-role", username)
	roleBindName := fmt.Sprintf("%s-role-bind", username)
	rolebinding := &rbacv1beta1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindName,
			Namespace: namespace,
		},
		RoleRef: rbacv1beta1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
		Subjects: []rbacv1beta1.Subject{
			rbacv1beta1.Subject{
				//APIGroup: "rbac.authorization.k8s.io",
				//Kind:     "User",
				//Name:     namespace,
				Kind:      "ServiceAccount",
				Name:      username + "-sa",
				Namespace: saNamespace,
			},
		},
	}
	_, found, err := getter.GetRoleBinding(namespace, roleBindName, krkubeclient.Client())
	if err != nil || !found {
		if err := updater.CreateRoleBinding(rolebinding, krkubeclient.Client()); err != nil {
			return err
		}
	} else {
		if err := updater.UpdateRoleBinding(rolebinding, krkubeclient.Client()); err != nil {
			return err
		}
	}
	return nil
}

func createK8sSSLCert(namespace, username string) error {
	workDir := filepath.Join(os.TempDir(), username)
	if err := os.MkdirAll(workDir, os.ModePerm); err != nil {
		return err
	}

	kubeClient := krkubeclient.Client()

	sa, found, err := getter.GetServiceAccount(namespace, username+"-sa", kubeClient)
	if err != nil {
		return err
	} else if !found {
		return errors.New("sa not fonud")
	} else if len(sa.Secrets) == 0 {
		return errors.New("no secrets in sa")
	}
	secret, found, err := getter.GetSecret(namespace, sa.Secrets[0].Name, kubeClient)
	if err != nil {
		return err
	} else if !found {
		return errors.New("secret not found")
	}
	keyPath := filepath.Join(workDir, "ca.key")
	certPath := filepath.Join(workDir, "ca.crt")

	err = ioutil.WriteFile(keyPath, secret.Data["token"], 0644)
	if err != nil {
		return err
	}
	cert := make([]byte, base64.StdEncoding.EncodedLen(len(secret.Data["ca.crt"])))
	base64.StdEncoding.Encode(cert, secret.Data["ca.crt"])
	err = ioutil.WriteFile(certPath, cert, 0644)
	if err != nil {
		return err
	}
	return nil
}

func renderCfgTmpl(args *kubeCfgTmplArgs) (string, error) {
	buf := new(bytes.Buffer)
	t := template.Must(template.New("cfg").Parse(kubeCfgTmpl))
	err := t.Execute(buf, args)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

const kubeCfgTmpl = `
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
  name: {{.User}}@kubernetes
current-context: {{.User}}@kubernetes
kind: Config
preferences: {}
users:
- name: {{.User}}
  user:
    token: {{.CaKeyBase64}}
    client-key-data: {{.CaCrtBase64}}
`

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
