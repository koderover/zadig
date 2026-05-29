/*
Copyright 2026 The KodeRover Authors.

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

package kube

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"path"
	"reflect"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
)

const dindTLSCertValidity = 10 * 365 * 24 * time.Hour

type DindTLSSecretTemplateData struct {
	CAPemBase64         string
	ServerCertPemBase64 string
	ServerKeyPemBase64  string
	ClientCertPemBase64 string
	ClientKeyPemBase64  string
	CertHash            string
}

func EnsureDindTLSCerts() (*commonmodels.DindTLSCerts, error) {
	settingColl := commonrepo.NewSystemSettingColl()
	settings, err := settingColl.Get()
	if err == nil && dindTLSCertsComplete(settings.DindTLSCerts) {
		return settings.DindTLSCerts, nil
	}
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	certs, err := generateDindTLSCerts()
	if err != nil {
		return nil, err
	}
	created, err := settingColl.InitDindTLSCertsIfNeeded(certs)
	if err != nil {
		return nil, err
	}
	if created {
		return certs, nil
	}

	settings, err = settingColl.Get()
	if err != nil {
		return nil, err
	}
	if !dindTLSCertsComplete(settings.DindTLSCerts) {
		return nil, fmt.Errorf("dind TLS certs are incomplete")
	}
	return settings.DindTLSCerts, nil
}

func EnsureDindTLSSecret(clientset kubernetes.Interface, namespace string) (*commonmodels.DindTLSCerts, error) {
	if clientset == nil {
		return nil, fmt.Errorf("kubernetes client is nil")
	}

	certs, err := EnsureDindTLSCerts()
	if err != nil {
		return nil, err
	}

	existing, getErr := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), types.DindTLSSecretName, metav1.GetOptions{})
	desired := BuildDindTLSSecret(namespace, certs)
	if getErr != nil {
		if apierrors.IsNotFound(getErr) {
			_, err := clientset.CoreV1().Secrets(namespace).Create(context.TODO(), desired, metav1.CreateOptions{})
			return certs, err
		}
		return nil, getErr
	}

	if existing.Type != desired.Type || !reflect.DeepEqual(existing.Data, desired.Data) || !reflect.DeepEqual(existing.Labels, desired.Labels) {
		existing.Type = desired.Type
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		_, err := clientset.CoreV1().Secrets(namespace).Update(context.TODO(), existing, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
	}

	return certs, nil
}

func BuildDindTLSSecret(namespace string, certs *commonmodels.DindTLSCerts) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      types.DindTLSSecretName,
			Namespace: namespace,
			Labels:    DindLabels(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: DindTLSSecretData(certs),
	}
}

func DindTLSSecretTemplate(certs *commonmodels.DindTLSCerts) DindTLSSecretTemplateData {
	data := DindTLSSecretData(certs)
	return DindTLSSecretTemplateData{
		CAPemBase64:         base64.StdEncoding.EncodeToString(data[types.DindTLSCACertKey]),
		ServerCertPemBase64: base64.StdEncoding.EncodeToString(data[types.DindTLSServerCertKey]),
		ServerKeyPemBase64:  base64.StdEncoding.EncodeToString(data[types.DindTLSServerKeyKey]),
		ClientCertPemBase64: base64.StdEncoding.EncodeToString(data[types.DindTLSClientCertKey]),
		ClientKeyPemBase64:  base64.StdEncoding.EncodeToString(data[types.DindTLSClientKeyKey]),
		CertHash:            DindTLSCertHash(certs),
	}
}

func DindTLSSecretData(certs *commonmodels.DindTLSCerts) map[string][]byte {
	if certs == nil {
		return map[string][]byte{}
	}
	return map[string][]byte{
		types.DindTLSCACertKey:     []byte(certs.CAPem),
		types.DindTLSServerCertKey: []byte(certs.ServerCertPem),
		types.DindTLSServerKeyKey:  []byte(certs.ServerKeyPem),
		types.DindTLSClientCertKey: []byte(certs.ClientCertPem),
		types.DindTLSClientKeyKey:  []byte(certs.ClientKeyPem),
	}
}

func DindTLSCertHash(certs *commonmodels.DindTLSCerts) string {
	data := DindTLSSecretData(certs)
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, key := range keys {
		h.Write([]byte(key))
		h.Write([]byte{0})
		h.Write(data[key])
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func DindLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/component": "dind",
		"app.kubernetes.io/name":      "zadig",
	}
}

func ApplyDindTLSSettings(dindSts *appsv1.StatefulSet, certs *commonmodels.DindTLSCerts) bool {
	if dindSts == nil || len(dindSts.Spec.Template.Spec.Containers) == 0 {
		return false
	}

	modified := false
	container := &dindSts.Spec.Template.Spec.Containers[0]

	if !reflect.DeepEqual(container.Args, buildDindTLSArgs(container.Args)) {
		container.Args = buildDindTLSArgs(container.Args)
		modified = true
	}

	ports := ensureDindTLSPort(container.Ports)
	if !reflect.DeepEqual(container.Ports, ports) {
		container.Ports = ports
		modified = true
	}

	envs := removeDockerTLSCertDirEnv(container.Env)
	if !reflect.DeepEqual(container.Env, envs) {
		container.Env = envs
		modified = true
	}

	volumeMounts := ensureDindTLSVolumeMount(container.VolumeMounts)
	if !reflect.DeepEqual(container.VolumeMounts, volumeMounts) {
		container.VolumeMounts = volumeMounts
		modified = true
	}

	volumes := ensureDindTLSVolume(dindSts.Spec.Template.Spec.Volumes)
	if !reflect.DeepEqual(dindSts.Spec.Template.Spec.Volumes, volumes) {
		dindSts.Spec.Template.Spec.Volumes = volumes
		modified = true
	}

	if dindSts.Spec.Template.Annotations == nil {
		dindSts.Spec.Template.Annotations = map[string]string{}
	}
	certHash := DindTLSCertHash(certs)
	if dindSts.Spec.Template.Annotations[types.DindTLSCertHashAnnotation] != certHash {
		dindSts.Spec.Template.Annotations[types.DindTLSCertHashAnnotation] = certHash
		modified = true
	}

	return modified
}

func EnsureDindServiceTLS(kclient client.Client, namespace string) error {
	service := &corev1.Service{}
	if err := kclient.Get(context.TODO(), client.ObjectKey{Name: types.DindStatefulSetName, Namespace: namespace}, service); err != nil {
		if apierrors.IsNotFound(err) {
			return kclient.Create(context.TODO(), BuildDindService(namespace))
		}
		return err
	}

	ports := ensureDindTLSServicePort(service.Spec.Ports)
	if reflect.DeepEqual(service.Spec.Ports, ports) {
		return nil
	}
	service.Spec.Ports = ports
	return kclient.Update(context.TODO(), service)
}

func BuildDindService(namespace string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      types.DindStatefulSetName,
			Namespace: namespace,
			Labels:    DindLabels(),
		},
		Spec: corev1.ServiceSpec{
			Ports:     ensureDindTLSServicePort(nil),
			ClusterIP: "None",
			Selector:  DindLabels(),
		},
	}
}

func buildDindTLSArgs(currentArgs []string) []string {
	args := []string{
		"--host=unix:///var/run/docker.sock",
		fmt.Sprintf("--host=tcp://0.0.0.0:%d", types.DindTLSPort),
		"--tlsverify",
		fmt.Sprintf("--tlscacert=%s", path.Join(types.DindTLSServerMountPath, types.DindTLSCACertKey)),
		fmt.Sprintf("--tlscert=%s", path.Join(types.DindTLSServerMountPath, types.DindTLSServerCertKey)),
		fmt.Sprintf("--tlskey=%s", path.Join(types.DindTLSServerMountPath, types.DindTLSServerKeyKey)),
	}

	for _, arg := range currentArgs {
		if isDindConnectionArg(arg) {
			continue
		}
		args = append(args, arg)
	}

	return args
}

func isDindConnectionArg(arg string) bool {
	return strings.HasPrefix(arg, "--host=") ||
		arg == "--tlsverify" ||
		strings.HasPrefix(arg, "--tlscacert") ||
		strings.HasPrefix(arg, "--tlscert") ||
		strings.HasPrefix(arg, "--tlskey")
}

func ensureDindTLSPort(ports []corev1.ContainerPort) []corev1.ContainerPort {
	resp := make([]corev1.ContainerPort, 0, len(ports)+1)
	for _, port := range ports {
		if port.ContainerPort == 2375 || port.ContainerPort == types.DindTLSPort {
			continue
		}
		resp = append(resp, port)
	}
	resp = append(resp, corev1.ContainerPort{
		Protocol:      corev1.ProtocolTCP,
		ContainerPort: types.DindTLSPort,
	})
	return resp
}

func ensureDindTLSServicePort(ports []corev1.ServicePort) []corev1.ServicePort {
	resp := make([]corev1.ServicePort, 0, len(ports)+1)
	for _, port := range ports {
		if port.Port == 2375 || port.Port == types.DindTLSPort || port.Name == types.DindContainerName {
			continue
		}
		resp = append(resp, port)
	}
	resp = append(resp, corev1.ServicePort{
		Name:       types.DindContainerName,
		Protocol:   corev1.ProtocolTCP,
		Port:       types.DindTLSPort,
		TargetPort: intstr.FromInt(types.DindTLSPort),
	})
	return resp
}

func removeDockerTLSCertDirEnv(envs []corev1.EnvVar) []corev1.EnvVar {
	resp := make([]corev1.EnvVar, 0, len(envs))
	for _, env := range envs {
		if env.Name == "DOCKER_TLS_CERTDIR" {
			continue
		}
		resp = append(resp, env)
	}
	return resp
}

func ensureDindTLSVolumeMount(volumeMounts []corev1.VolumeMount) []corev1.VolumeMount {
	resp := make([]corev1.VolumeMount, 0, len(volumeMounts)+1)
	for _, volumeMount := range volumeMounts {
		if volumeMount.Name == types.DindTLSVolumeName {
			continue
		}
		resp = append(resp, volumeMount)
	}
	resp = append(resp, corev1.VolumeMount{
		Name:      types.DindTLSVolumeName,
		MountPath: types.DindTLSServerMountPath,
		ReadOnly:  true,
	})
	return resp
}

func ensureDindTLSVolume(volumes []corev1.Volume) []corev1.Volume {
	resp := make([]corev1.Volume, 0, len(volumes)+1)
	for _, volume := range volumes {
		if volume.Name == types.DindTLSVolumeName {
			continue
		}
		resp = append(resp, volume)
	}
	resp = append(resp, corev1.Volume{
		Name: types.DindTLSVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: types.DindTLSSecretName,
				Items: []corev1.KeyToPath{
					{
						Key:  types.DindTLSCACertKey,
						Path: types.DindTLSCACertKey,
					},
					{
						Key:  types.DindTLSServerCertKey,
						Path: types.DindTLSServerCertKey,
					},
					{
						Key:  types.DindTLSServerKeyKey,
						Path: types.DindTLSServerKeyKey,
					},
				},
			},
		},
	})
	return resp
}

func dindTLSCertsComplete(certs *commonmodels.DindTLSCerts) bool {
	return certs != nil &&
		certs.CAPem != "" &&
		certs.CAKeyPem != "" &&
		certs.ServerCertPem != "" &&
		certs.ServerKeyPem != "" &&
		certs.ClientCertPem != "" &&
		certs.ClientKeyPem != ""
}

func generateDindTLSCerts() (*commonmodels.DindTLSCerts, error) {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	caTemplate := certificateTemplate("zadig-dind-ca", true, nil)
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	serverTemplate := certificateTemplate("zadig-dind-server", false, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	serverTemplate.DNSNames = dindTLSServerNames()
	serverTemplate.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caTemplate, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}

	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	clientTemplate := certificateTemplate("zadig-dind-client", false, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, &clientKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}

	return &commonmodels.DindTLSCerts{
		CAPem:         string(encodeCert(caDER)),
		CAKeyPem:      string(encodePrivateKey(caKey)),
		ServerCertPem: string(encodeCert(serverDER)),
		ServerKeyPem:  string(encodePrivateKey(serverKey)),
		ClientCertPem: string(encodeCert(clientDER)),
		ClientKeyPem:  string(encodePrivateKey(clientKey)),
	}, nil
}

func certificateTemplate(commonName string, isCA bool, extKeyUsage []x509.ExtKeyUsage) *x509.Certificate {
	now := time.Now()
	keyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	if isCA {
		keyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	}
	return &x509.Certificate{
		SerialNumber:          randomSerialNumber(),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(dindTLSCertValidity),
		KeyUsage:              keyUsage,
		ExtKeyUsage:           extKeyUsage,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
}

func randomSerialNumber() *big.Int {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return big.NewInt(time.Now().UnixNano())
	}
	return serialNumber
}

func dindTLSServerNames() []string {
	names := []string{
		"dind",
		"*.dind",
		"localhost",
	}

	for _, namespace := range []string{config.Namespace(), setting.AttachedClusterNamespace} {
		namespace = strings.TrimSpace(namespace)
		if namespace == "" {
			continue
		}
		names = append(names,
			fmt.Sprintf("dind.%s", namespace),
			fmt.Sprintf("dind.%s.svc", namespace),
			fmt.Sprintf("dind.%s.svc.cluster.local", namespace),
			fmt.Sprintf("*.dind.%s", namespace),
			fmt.Sprintf("*.dind.%s.svc", namespace),
			fmt.Sprintf("*.dind.%s.svc.cluster.local", namespace),
		)
	}

	return dedupeStrings(names)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	ret := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		ret = append(ret, value)
	}
	return ret
}

func encodeCert(cert []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
}

func encodePrivateKey(key *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}
