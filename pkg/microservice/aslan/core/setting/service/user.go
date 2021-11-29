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
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
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

func GetUserKubeConfig(userID string, editEnvProjects []string, readEnvProjects []string, log *zap.SugaredLogger) (string, error) {
	saNamespace := config.Namespace()
	if err := ensureClusterRole(log); err != nil {
		log.Errorf("ensurClusterRole err: %s", err)
		return "", err
	}
	if err := ensureServiceAccountAndRolebinding(saNamespace, editEnvProjects, readEnvProjects, userID, log); err != nil {
		log.Errorf("ensureServiceAccountAndRolebinding err: %s", err)
		return "", err
	}
	crt, token, err := getCrtAndToken(saNamespace, userID)
	if err != nil {
		log.Errorf("getCrtAndToken err: %s", err)
		return "", err
	}
	args := &kubeCfgTmplArgs{
		KubeServerAddr: config.KubeServerAddr(),
		CaCrtBase64:    crt,
		User:           userID,
		CaKeyBase64:    token,
	}

	return renderCfgTmpl(args)
}

func getCrtAndToken(namespace, userID string) (string, string, error) {
	kubeClient := krkubeclient.Client()
	var sa *corev1.ServiceAccount
	for i := 0; i < 5; i++ {
		tmpsa, found, err := getter.GetServiceAccount(namespace, config.ServiceAccountNameForUser(userID), kubeClient)
		if err != nil {
			log.Warnf("GetServiceAccount err:%s", err)
			return "", "", err
		} else if !found || len(sa.Secrets) == 0 {
			time.Sleep(time.Second)
		} else {
			sa = tmpsa
			break
		}
	}
	if sa == nil {
		log.Errorf("can not get sa")
		return "", "", errors.New("can not get sa")
	}

	secret, found, err := getter.GetSecret(namespace, sa.Secrets[0].Name, kubeClient)
	if err != nil {
		log.Errorf("GetSecret err: %s", err)
		return "", "", err
	} else if !found {
		log.Error("secret not found")
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

var (
	clusterRoleEdit = &rbacv1beta1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.RoleBindingNameEditEnv,
		},
		Rules: []rbacv1beta1.PolicyRule{rbacv1beta1.PolicyRule{
			Verbs:     []string{"*"},
			APIGroups: []string{"*"},
			Resources: []string{"*"},
		}}}
	clusterRoleRead = &rbacv1beta1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.RoleBindingNameReadEnv,
		},
		Rules: []rbacv1beta1.PolicyRule{rbacv1beta1.PolicyRule{
			Verbs:     []string{"get", "watch", "list"},
			APIGroups: []string{"*"},
			Resources: []string{"*"},
		}}}
)

func ensureClusterRole(log *zap.SugaredLogger) error {
	if _, found, err := getter.GetClusterRole(config.RoleBindingNameEditEnv, krkubeclient.Client()); err == nil && !found {
		if err := updater.CreateClusterRole(clusterRoleEdit, krkubeclient.Client()); err != nil {
			log.Errorf("CreateClusterRole err: %s", err)
			return err
		}
	} else if err != nil {
		log.Errorf("GetClusterRole err: %s", err)
		return err
	}
	if _, found, err := getter.GetClusterRole(config.RoleBindingNameReadEnv, krkubeclient.Client()); err == nil && !found {
		if err := updater.CreateClusterRole(clusterRoleRead, krkubeclient.Client()); err != nil {
			log.Errorf("CreateClusterRole err: %s", err)
			return err
		}
	} else if err != nil {
		log.Errorf("GetClusterRole err: %s", err)
		return err
	}
	return nil
}

func ensureServiceAccountAndRolebinding(namespace string, editEnvProjects []string, readEnvProjects []string, userID string, log *zap.SugaredLogger) error {
	serviceAccountName := config.ServiceAccountNameForUser(userID)
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}
	_, found, err := getter.GetServiceAccount(namespace, serviceAccountName, krkubeclient.Client())
	if err != nil {
		log.Errorf("GetServiceAccount name:%s err:%s", err)
		return err
	}
	if !found {
		if err := updater.CreateServiceAccount(serviceAccount, krkubeclient.Client()); err != nil {
			log.Errorf("CreateServiceAccount name:%s err:%s", serviceAccountName, err)
			return err
		}
	}

	// picket service provide a list of projects for which the user has permission to edit env or read env
	// while []string{*} means all projects
	if len(editEnvProjects) == 1 && editEnvProjects[0] == "*" {
		res, err := templaterepo.NewProductColl().ListNames(nil)
		if err != nil {
			log.Errorf("ListProjectBriefs err:%s", err)
			return err
		}
		editEnvProjects = res
	}

	if len(readEnvProjects) == 1 && readEnvProjects[0] == "*" {
		res, err := templaterepo.NewProductColl().ListNames(nil)
		if err != nil {
			log.Errorf("ListProjectBriefs err:%s", err)
			return err
		}
		readEnvProjects = res
	}

	for _, v := range editEnvProjects {
		products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: v})
		if err != nil {
			log.Errorf("[%s] Collections.Product.List error: %v", v, err)
		}
		products = filterProductWithoutExternalCluster(products)
		for _, vv := range products {
			if err := CreateRoleBinding(vv.Namespace, namespace, serviceAccountName, config.RoleBindingNameEditEnv); err != nil {
				log.Errorf("CreateRoleBinding err: %s", err)
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
			if err := CreateRoleBinding(vv.Namespace, namespace, serviceAccountName, config.RoleBindingNameReadEnv); err != nil {
				log.Errorf("CreateRoleBinding err: %s", err)
			}
		}
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
  name: koderover
current-context: koderover
kind: Config
preferences: {}
users:
- name: {{.User}}
  user:
    token: {{.CaKeyBase64}}
`

func CreateRoleBinding(rbNamespace, saNamspace, serviceAccountName, roleBindName string) error {
	rolebinding, found, err := getter.GetRoleBinding(rbNamespace, roleBindName, krkubeclient.Client())
	subs := []rbacv1beta1.Subject{{
		Kind:      "ServiceAccount",
		Name:      serviceAccountName,
		Namespace: saNamspace,
	}}
	if err != nil {
		log.Errorf("GetRoleBinding err: %s", err)
		return err
	}
	if found {
		isExist := false
		for _, v := range rolebinding.Subjects {
			if v.Name == serviceAccountName {
				isExist = true
			}
		}
		if !isExist {
			subs = append(subs, rolebinding.Subjects...)
		}
	}
	if err := updater.CreateOrPatchRoleBinding(&rbacv1beta1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindName,
			Namespace: rbNamespace,
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
		return err
	}
	return nil
}
