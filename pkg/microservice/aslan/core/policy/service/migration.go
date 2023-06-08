package service

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	commonconfig "github.com/koderover/zadig/pkg/config"
	policyservice "github.com/koderover/zadig/pkg/microservice/policy/core/service"
	"github.com/koderover/zadig/pkg/microservice/policy/core/yamlconfig"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
)

func MigratePolicyData() error {
	if err := migratePolicyMeta(); err != nil {
		log.Errorf("fail to migrate policyMeta, err:%s", err)
		return err
	}

	if err := migrateRole(); err != nil {
		log.Errorf("fail to migrate role , err:%s", err)
		return err
	}

	return nil
}

type PresetRoleConfigYaml struct {
	PresetRoles []*policyservice.Role `json:"preset_roles"`
	Description string                `json:"description"`
}

func migrateRole() error {

	bs := yamlconfig.PresetRolesBytes()
	config := PresetRoleConfigYaml{}

	if err := yaml.Unmarshal(bs, &config); err != nil {
		log.Errorf("yaml Unmarshal err:%s", err)
		return err
	}

	for _, role := range config.PresetRoles {
		if role.Name == "admin" {
			role.Namespace = "*"
		}
		for _, rule := range role.Rules {
			rule.Kind = "resource"
		}
		role.Type = "system"
	}

	for _, role := range config.PresetRoles {
		if err := policyservice.UpdateOrCreateRole(role.Namespace, role, nil); err != nil {
			log.Errorf("UpdateOrCreateRole err:%s", err)
			return err
		}
	}
	return nil

}

// migratePolicyMeta migrate the policy meta db date
func migratePolicyMeta() error {
	client, err := kubeclient.GetKubeClient(commonconfig.HubServerServiceAddress(), setting.LocalClusterID)
	if err != nil {
		log.DPanic(err)
	}
	clientset, err := kubeclient.GetKubeClientSet(commonconfig.HubServerServiceAddress(), setting.LocalClusterID)
	if err != nil {
		log.DPanic(err)
	}

	namespace := commonconfig.Namespace()
	policyMetasConfig := yamlconfig.DefaultPolicyMetasConfig()
	policyMetaConfigBytes, err := yaml.Marshal(policyMetasConfig)
	if err != nil {
		return err
	}
	urls := yamlconfig.GetDefaultEmbedUrlConfig()
	urlsBytes, err := yaml.Marshal(urls)
	if err != nil {
		return err
	}

	metaConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      setting.PolicyMetaConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"meta.yaml": string(policyMetaConfigBytes),
		},
	}

	urlConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      setting.PolicyURLConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"urls.yaml": string(urlsBytes),
		},
	}
	roleConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      setting.PolicyRoleConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"roles.yaml": string(yamlconfig.PresetRolesBytes()),
		},
	}

	if err := initConfigMap(metaConfigMap, client, clientset); err != nil {
		log.Errorf("init config map meta err:%s", err)
	}
	if err := initConfigMap(urlConfigMap, client, clientset); err != nil {
		log.Errorf("init config map url err:%s", err)
	}

	if err := initConfigMap(roleConfigMap, client, clientset); err != nil {
		log.Errorf("init config map roles err:%s", err)
	}

	return err
}

func initConfigMap(cm *corev1.ConfigMap, client client.Client, clientset *kubernetes.Clientset) error {
	_, found, err := getter.GetConfigMap(cm.Namespace, cm.Name, client)
	if err != nil {
		log.Errorf("get config map err:%s", err)
	}
	if !found {
		err := updater.CreateConfigMap(cm, client)
		if err != nil {
			log.Infof("create config map err:%s", err)
			return err
		}
	} else {
		if err := updater.UpdateConfigMap(cm.Namespace, cm, clientset); err != nil {
			log.Infof("update config map err:%s", err)
			return err
		}
	}
	return nil
}
