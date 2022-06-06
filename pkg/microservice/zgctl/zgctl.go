/*
Copyright 2022 The KodeRover Authors.

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

package zgctl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/shirou/gopsutil/v3/process"

	"github.com/koderover/zadig/pkg/shared/client/aslan"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

type ZgCtlConfig struct {
	ZadigHost  string
	ZadigToken string
	HomeDir    string
	DB         *badger.DB
}

type zgctl struct {
	aslanClient  *aslan.Client
	homeDir      string
	syncthingBin string
	db           *badger.DB
}

type SyncthingConfig struct {
	SyncDirPath    string
	SyncType       string
	LocalDeviceID  string
	RemoteDeviceID string
	RemoteAddr     string
	ListenAddr     string
}

type SyncthingScript struct {
	SyncthingBin string
	ConfigDir    string
	DataDir      string
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func NewZgCtl(cfg *ZgCtlConfig) ZgCtler {
	aslanClient := aslan.NewExternal(cfg.ZadigHost, cfg.ZadigToken)
	aslanClient.SetTimeout(3 * time.Minute)

	syncthingBin := filepath.Join(cfg.HomeDir, "bin", "syncthing")
	log.Infof("home dir: %s, syncthing bin: %s", cfg.HomeDir, syncthingBin)

	return &zgctl{
		aslanClient:  aslanClient,
		homeDir:      cfg.HomeDir,
		syncthingBin: syncthingBin,
		db:           cfg.DB,
	}
}

func (z *zgctl) StartDevMode(ctx context.Context, projectName, envName, serviceName, dataDir, devImage string) error {
	// 1. Adjust replicas of the workload to 1 and patch Pod.
	log.Info("Begin to patch worload.")
	workloadInfo, err := z.aslanClient.PatchWorkload(projectName, envName, serviceName, devImage)
	if err != nil {
		errRet := fmt.Errorf("failed to patch service %q in env %q of project %q: %s", envName, serviceName, projectName, err)
		log.Error(errRet)
		return errRet
	}

	log.Infof("workload info: %#v", workloadInfo)
	err = z.setZadig2K8sData(projectName, envName, serviceName, workloadInfo)
	if err != nil {
		errRet := fmt.Errorf("failed to set workload info: %s", err)
		log.Error(errRet)
		return errRet
	}

	// 2. Configure `Syncthing`.
	log.Info("Begin to config Syncthing.")
	localPort := z.genRandPort()
	remotePort := z.genRandPort()
	configDir := z.generateServiceConfigDir(projectName, envName, serviceName)
	err = z.configSyncthing(ctx, configDir, dataDir, localPort, remotePort)
	if err != nil {
		errRet := fmt.Errorf("failed to config syncthing for service %q in env %q: %s", envName, serviceName, err)
		log.Error(errRet)
		return errRet
	}

	// 3. Execute port-forward to implement one-way network communication.
	log.Info("Begin to do port-forward.")

	kubeconfigPath, err := z.getEnvKubeconfigData(projectName, envName)
	if err != nil {
		errRet := fmt.Errorf("failed to find kubeconfig path of env %q in project %q: %s", envName, projectName, err)
		log.Error(errRet)
		return errRet
	}
	log.Infof("kubeconfig path for env %q in project %q: %s", envName, projectName, kubeconfigPath)

	err = z.podPortForward(ctx, workloadInfo.PodNamespace, workloadInfo.PodName, kubeconfigPath, remotePort)
	if err != nil {
		errRet := fmt.Errorf("failed to port-forward service %q in env %q: %s", envName, serviceName, err)
		log.Error(errRet)
		return errRet
	}

	// 4. Start `Syncthing`.
	log.Info("Begin to start Syncthing.")
	err = z.startSyncthing(ctx, workloadInfo.PodNamespace, workloadInfo.PodName, configDir, dataDir, kubeconfigPath)
	if err != nil {
		errRet := fmt.Errorf("failed to start syncthing: %s", err)
		log.Error(errRet)
		return errRet
	}

	return nil
}

func (z *zgctl) StopDevMode(ctx context.Context, projectName, envName, serviceName string) error {
	configDir := z.generateServiceConfigDir(projectName, envName, serviceName)

	// 1. Stop `Syncthing` locally and delete the corresponding configuration.
	log.Info("Begin to stop Syncthing.")

	workloadInfo, err := z.getZadig2K8sData(projectName, envName, serviceName)
	if err != nil {
		errRet := fmt.Errorf("failed to get find workload info for project %q, envName %q, serviceName %q: %s", projectName, envName, serviceName, err)
		log.Error(errRet)
		return errRet
	}

	err = z.stopSyncthing(ctx, configDir, workloadInfo)
	if err != nil {
		log.Warnf("Failed to stop syncthing for service %q in env %q: %s", envName, serviceName, err)
	}

	// 2. Recover workload.
	log.Info("Begin to recover workload.")
	err = z.aslanClient.RecoverWorkload(projectName, envName, serviceName)
	if err != nil {
		errRet := fmt.Errorf("failed to recover workload: %s", err)
		log.Error(errRet)
		return errRet
	}

	return nil
}

func (z *zgctl) DevImages() []string {
	return devImages
}

func (z *zgctl) ConfigKubeconfig(projectName, envName, kubeconfigPath string) error {
	return z.setEnvKubeconfigData(projectName, envName, kubeconfigPath)
}

func (z *zgctl) configSyncthing(ctx context.Context, configDir, dataDir string, localPort, remotePort int) error {
	// 1. Generate local `Syncthing` config.
	localDeviceID, err := z.configLocalSyncthing(configDir)
	if err != nil {
		return fmt.Errorf("failed to generate local Syncthing: %s", err)
	}
	log.Infof("local device id: %s", localDeviceID)

	// 2. Generate remote `Syncthing` config.
	tmpRemoteConfigPath := filepath.Join(configDir, "remote")
	tmpRemoteDeviceID, err := z.configLocalSyncthing(tmpRemoteConfigPath)
	if err != nil {
		return fmt.Errorf("failed to generate tmp remote Syncthing: %s", err)
	}
	log.Infof("tmp remote device id: %s", tmpRemoteDeviceID)

	// 3. Adjust configs of local and remote Syncthing.
	return z.adjustSyncthingConfig(localDeviceID, tmpRemoteDeviceID, configDir, dataDir, localPort, remotePort)
}

func (z *zgctl) startSyncthing(ctx context.Context, ns, podName, configDir, dataDir, kubeconfigPath string) error {
	// 1. Upload config to remote Pod and start Syncthing in remote Pod.
	err := z.startRemoteSyncthing(ctx, ns, podName, configDir, kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to start remote syncthing: %s", err)
	}

	// 2. Start local Syncthing.
	return z.startLocalSyncthing(ctx, configDir, dataDir)
}

func (z *zgctl) stopSyncthing(ctx context.Context, configDir string, workloadInfo *types.WorkloadInfo) error {
	processes, err := process.Processes()
	if err != nil {
		return err
	}

	for _, p := range processes {
		// Note: Don't use `p.Name()`. See: https://github.com/shirou/gopsutil/issues/1043
		cmdline, err := p.Cmdline()
		if err != nil {
			log.Warnf("Failed to find command line: %s", err)
			continue
		}

		if (strings.Contains(cmdline, "syncthing serve") && strings.Contains(cmdline, configDir)) ||
			strings.Contains(cmdline, fmt.Sprintf("kubectl -n %s port-forward %s", workloadInfo.PodNamespace, workloadInfo.PodName)) {
			log.Infof("Begin to kill process %s. Cmdline: %s.", p, cmdline)
			err = p.Kill()
			if err != nil {
				log.Warnf("Failed to kill process %s: %s", p, err)
			}
		}
	}

	return os.RemoveAll(configDir)
}

func (z *zgctl) podPortForward(ctx context.Context, ns, podName, kubeconfigPath string, port int) error {
	go func() {
		portForward := fmt.Sprintf("kubectl -n %s port-forward %s %d:%d --kubeconfig %s", ns, podName, port, port, kubeconfigPath)
		log.Infof("port-forward: %s", portForward)

		cmd := exec.Command("/bin/sh", "-c", portForward)
		res, err := cmd.CombinedOutput()
		if err != nil {
			log.Errorf("Failed to do port-forward %q: %s. Msg: %s.", portForward, err, string(res))
			return
		}

		log.Infof("Has done `%q`: %s.", portForward, string(res))
	}()

	return nil
}

func (z *zgctl) configLocalSyncthing(configDir string) (deviceID string, err error) {
	log.Infof("config dir: %s", configDir)

	var errBuffer bytes.Buffer
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("%s generate --config=%s", z.syncthingBin, configDir))
	cmd.Stderr = &errBuffer
	err = cmd.Run()
	if err != nil {
		return "", err
	}

	items := strings.Split(errBuffer.String(), "Device ID:")
	return strings.TrimSpace(items[1]), nil
}

func (z *zgctl) generateServiceConfigDir(projectName, envName, serviceName string) string {
	return filepath.Join(z.homeDir, "config", fmt.Sprintf("%s-%s-%s", projectName, envName, serviceName))
}

func (z *zgctl) adjustSyncthingConfig(localDeviceID, remoteDeviceID, configDir, dataDir string, localPort, remotePort int) error {
	configTmpl, err := template.New("local").Parse(syncthingConfigTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse config template: %s", err)
	}

	localAddr := fmt.Sprintf("tcp://127.0.0.1:%d", localPort)
	remoteAddr := fmt.Sprintf("tcp://127.0.0.1:%d", remotePort)

	localData := SyncthingConfig{
		SyncDirPath:    dataDir,
		SyncType:       "sendonly",
		LocalDeviceID:  localDeviceID,
		RemoteDeviceID: remoteDeviceID,
		RemoteAddr:     remoteAddr,
		ListenAddr:     localAddr,
	}

	var localBuffer bytes.Buffer
	err = configTmpl.Execute(&localBuffer, localData)
	if err != nil {
		return fmt.Errorf("failed to execute local config template: %s", err)
	}

	err = ioutil.WriteFile(filepath.Join(configDir, "config.xml"), localBuffer.Bytes(), 0600)
	if err != nil {
		return fmt.Errorf("failed to overwrite local config: %s", err)
	}

	scriptTmpl, err := template.New("local-script").Parse(syncthingRunTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse script template: %s", err)
	}

	localScriptData := SyncthingScript{
		SyncthingBin: filepath.Join(z.homeDir, "bin", "syncthing"),
		ConfigDir:    configDir,
		DataDir:      dataDir,
	}

	var localScriptBuffer bytes.Buffer
	err = scriptTmpl.Execute(&localScriptBuffer, localScriptData)
	if err != nil {
		return fmt.Errorf("failed to execute local script template: %s", err)
	}

	err = ioutil.WriteFile(filepath.Join(configDir, "run.sh"), localScriptBuffer.Bytes(), 0755)
	if err != nil {
		return fmt.Errorf("failed to write local run.sh: %s", err)
	}

	remoteData := SyncthingConfig{
		SyncDirPath:    types.DevmodeWorkDir,
		SyncType:       "receiveonly",
		LocalDeviceID:  localDeviceID,
		RemoteDeviceID: remoteDeviceID,
		RemoteAddr:     "dynamic",
		ListenAddr:     remoteAddr,
	}

	var remoteBuffer bytes.Buffer
	err = configTmpl.Execute(&remoteBuffer, remoteData)
	if err != nil {
		return fmt.Errorf("failed to execute remote config template: %s", err)
	}

	err = ioutil.WriteFile(filepath.Join(configDir, "remote", "config.xml"), remoteBuffer.Bytes(), 0600)
	if err != nil {
		return fmt.Errorf("failed to overwrite remote config: %s", err)
	}

	remoteScriptData := SyncthingScript{
		SyncthingBin: "syncthing",
		ConfigDir:    filepath.Join(types.DevmodeWorkDir, "config"),
		DataDir:      types.DevmodeWorkDir,
	}

	var remoteScriptBuffer bytes.Buffer
	err = scriptTmpl.Execute(&remoteScriptBuffer, remoteScriptData)
	if err != nil {
		return fmt.Errorf("failed to execute remote script template: %s", err)
	}

	return ioutil.WriteFile(filepath.Join(configDir, "remote", "run.sh"), remoteScriptBuffer.Bytes(), 0755)
}

func (z *zgctl) genRandPort() int {
	max := 60000
	min := 40000

	return rand.Intn(max-min) + min
}

func (z *zgctl) genSyncthingServeAddr() string {
	return fmt.Sprintf("tcp://127.0.0.1:%d", z.genRandPort())
}

func (z *zgctl) startRemoteSyncthing(ctx context.Context, ns, podName, configDir, kubeconfigPath string) error {
	cpCmd := fmt.Sprintf("kubectl --kubeconfig %s -n %s cp %s %s:%s -c %s", kubeconfigPath, ns, filepath.Join(configDir, "remote"), podName, filepath.Join(types.DevmodeWorkDir, "config"), types.IDEContainerNameSidecar)
	log.Infof("cp cmd: %s", cpCmd)

	cmd := exec.Command("/bin/sh", "-c", cpCmd)
	res, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to cp files to remote pod: %s. msg: %s.", err, string(res))
	}

	execCmd := fmt.Sprintf("kubectl --kubeconfig %s -n %s exec %s -c %s -- /bin/sh -c '%s >/dev/null 2>/dev/null &'", kubeconfigPath, ns, podName, types.IDEContainerNameSidecar, filepath.Join(types.DevmodeWorkDir, "config", "run.sh"))
	log.Infof("exec cmd: %s", execCmd)
	cmd = exec.Command("/bin/sh", "-c", execCmd)
	res, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run syncthing in remote pod: %s. msg: %s", err, string(res))
	}

	return nil
}

func (z *zgctl) startLocalSyncthing(ctx context.Context, configDir, dataDir string) error {
	script := fmt.Sprintf("%s >/dev/null 2>/dev/null &", filepath.Join(configDir, "run.sh"))
	log.Infof("script: %s", script)

	cmd := exec.Command("/bin/sh", "-c", script)
	res, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run syncthing locally: %s. msg: %s.", err, string(res))
	}

	return nil
}

func (z *zgctl) genZadig2K8sMapKey(projectName, envName, serviceName string) string {
	return fmt.Sprintf("%s-%s-%s", projectName, envName, serviceName)
}

func (z *zgctl) genEnvKubeconfigKey(projectName, envName string) string {
	return fmt.Sprintf("%s-%s", projectName, envName)
}

func (z *zgctl) setZadig2K8sData(projectName, envName, serviceName string, data *types.WorkloadInfo) error {
	key := z.genZadig2K8sMapKey(projectName, envName, serviceName)
	val, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return z.setData([]byte(key), val)
}

func (z *zgctl) getZadig2K8sData(projectName, envName, serviceName string) (*types.WorkloadInfo, error) {
	key := z.genZadig2K8sMapKey(projectName, envName, serviceName)
	val, err := z.getData([]byte(key))
	if err != nil {
		return nil, err
	}

	var data types.WorkloadInfo
	err = json.Unmarshal(val, &data)
	return &data, err
}

func (z *zgctl) setEnvKubeconfigData(projectName, envName, kubeconfigPath string) error {
	key := z.genEnvKubeconfigKey(projectName, envName)
	return z.setData([]byte(key), []byte(kubeconfigPath))
}

func (z *zgctl) getEnvKubeconfigData(projectName, envName string) (string, error) {
	key := z.genEnvKubeconfigKey(projectName, envName)
	val, err := z.getData([]byte(key))
	return string(val), err
}

func (z *zgctl) setData(key, val []byte) error {
	return z.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
}

func (z *zgctl) getData(key []byte) ([]byte, error) {
	var val []byte
	err := z.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		val, err = item.ValueCopy(nil)
		return err
	})

	return val, err
}
