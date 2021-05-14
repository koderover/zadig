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

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/koderover/zadig/lib/internal/kube/wrapper"
	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/poetry"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	krkubeclient "github.com/koderover/zadig/lib/tool/kube/client"
	"github.com/koderover/zadig/lib/tool/kube/getter"
	"github.com/koderover/zadig/lib/tool/kube/updater"
	"github.com/koderover/zadig/lib/tool/xlog"
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

func GetUserKubeConfig(userName string, userID int, superUser bool, log *xlog.Logger) (string, error) {
	username := strings.ToLower(userName)
	username = config.NameSpaceRegex.ReplaceAllString(username, "-")
	var (
		err            error
		productEnvs    = make([]*commonmodels.Product, 0)
		productNameMap map[string][]int64
	)
	if superUser {
		productEnvs, err = commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{})
		if err != nil {
			log.Errorf("GetUserKubeConfig Collection.Product.List error: %v", err)
			return "", e.ErrListProducts.AddDesc(err.Error())
		}

		// 只管理同集群的资源，且排除状态为Terminating的namespace
		productEnvs = filterProductWithoutExternalCluster(productEnvs)
	} else {
		//项目下所有公开环境
		publicProducts, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{IsPublic: true})
		if err != nil {
			log.Errorf("GetUserKubeConfig Collection.Product.List List product error: %v", err)
			return "", e.ErrListProducts.AddDesc(err.Error())
		}
		// 只管理同集群的资源，且排除状态为Terminating的namespace
		filterPublicProductEnvs := filterProductWithoutExternalCluster(publicProducts)
		namespaceSet := sets.NewString()
		for _, publicProduct := range filterPublicProductEnvs {
			productEnvs = append(productEnvs, publicProduct)
			namespaceSet.Insert(publicProduct.Namespace)
		}
		poetryClient := poetry.NewPoetryServer(config.PoetryAPIServer(), config.PoetryAPIRootKey())
		productNameMap, err = poetryClient.GetUserProject(userID, log)
		if err != nil {
			log.Errorf("GetUserKubeConfig Collection.Product.List GetUserProject error: %v", err)
			return "", e.ErrListProducts.AddDesc(err.Error())
		}
		for productName, roleIDs := range productNameMap {
			//用户关联角色所关联的环境
			for _, roleID := range roleIDs {
				if roleID == setting.RoleOwnerID {
					tmpProducts, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: productName})
					if err != nil {
						log.Errorf("GetUserKubeConfig Collection.Product.List product error: %v", err)
						return "", e.ErrListProducts.AddDesc(err.Error())
					}
					for _, product := range tmpProducts {
						if !namespaceSet.Has(product.Namespace) {
							productEnvs = append(productEnvs, product)
						}
					}
				} else {
					roleEnvs, err := poetryClient.ListRoleEnvs(productName, "", roleID, log)
					if err != nil {
						log.Errorf("GetUserKubeConfig Collection.Product.List ListRoleEnvs error: %v", err)
						return "", e.ErrListProducts.AddDesc(err.Error())
					}
					for _, roleEnv := range roleEnvs {
						product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: roleEnv.EnvName})
						if err != nil {
							log.Errorf("GetUserKubeConfig Collection.Product.List Find product error: %v", err)
							return "", e.ErrListProducts.AddDesc(err.Error())
						}
						productEnvs = append(productEnvs, product)
					}
				}
			}
		}
	}
	saNamespace := config.Namespace()
	if err := ensureServiceAccount(saNamespace, username, log); err != nil {
		return "", err
	}

	var (
		wg      sync.WaitGroup
		pool    = make(chan int, 20)
		errList = new(multierror.Error)
	)
	for _, productEnv := range productEnvs {
		namespace := productEnv.Namespace
		productName := productEnv.ProductName

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
			if err := ensureUserRole(namespace, username, productName, userID, superUser, log); err != nil {
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
		if product.ClusterId != "" {
			continue
		}
		// 过滤外部环境托管
		if product.Source != "" && product.Source != setting.SourceFromZadig {
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

func ensureServiceAccount(namespace, username string, log *xlog.Logger) error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      username + "-sa",
			Namespace: namespace,
		},
	}
	if err := updater.CreateServiceAccount(serviceAccount, krkubeclient.Client()); err != nil {
		log.Errorf("CreateServiceAccount err :", err)
		return nil
	}
	return nil
}

func ensureUserRole(namespace, username, productName string, userID int, superUser bool, log *xlog.Logger) error {
	poetryClient := poetry.NewPoetryServer(config.PoetryAPIServer(), config.PoetryAPIRootKey())
	roleName := fmt.Sprintf("%s-role", username)
	verbs := []string{"get", "list", "watch"}
	if poetryClient.HasOperatePermission(productName, "40003", userID, superUser, log) {
		verbs = []string{"*"}
	}
	role := &rbacv1beta1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
		Rules: []rbacv1beta1.PolicyRule{
			rbacv1beta1.PolicyRule{
				APIGroups: []string{"*"},
				Resources: []string{
					"daemonsets",
					"configmaps",
					"deployments",
					"endpoints",
					"events",
					"horizontalpodautoscalers",
					"ingresses",
					"jobs",
					"cronjobs",
					"persistentvolumeclaims",
					"pods",
					"pods/log",
					"pods/exec",
					"pods/portforward",
					"podtemplates",
					"replicasets",
					"replicationcontrollers",
					"secrets",
					"serviceaccounts",
					"services",
					"statefulsets",
				},
				Verbs: verbs,
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
