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
	"strings"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func CreateInstall(args *commonmodels.Install, log *zap.SugaredLogger) error {
	err := commonrepo.NewInstallColl().Create(args)
	args.Name = strings.TrimSpace(args.Name)
	if err != nil {
		log.Errorf("Install.Create error: %v", err)
		return e.ErrCreateInstall
	}

	return nil
}

func UpdateInstall(name, version string, args *commonmodels.Install, log *zap.SugaredLogger) error {
	err := commonrepo.NewInstallColl().Update(name, version, args)
	if err != nil {
		log.Errorf("Install.Update %s error: %v", name, err)
		return e.ErrUpdateInstall
	}
	return nil
}

func GetInstall(name, version string, log *zap.SugaredLogger) (*commonmodels.Install, error) {
	resp, err := commonrepo.NewInstallColl().Find(name, version)
	if err != nil {
		log.Errorf("Install.Find %s error: %v", name, err)
		return resp, e.ErrGetInstall
	}
	return resp, nil
}

func ListInstalls(log *zap.SugaredLogger) ([]*commonmodels.Install, error) {
	resp, err := commonrepo.NewInstallColl().List()
	if err != nil {
		log.Errorf("Install.List error: %v", err)
		return resp, e.ErrListInstalls
	}
	return resp, nil
}

func ListAvaiableInstalls(log *zap.SugaredLogger) ([]*commonmodels.Install, error) {
	resp := make([]*commonmodels.Install, 0)
	installs, err := commonrepo.NewInstallColl().List()
	if err != nil {
		return resp, e.ErrListInstalls
	}

	for _, install := range installs {
		if install.Enabled {
			resp = append(resp, install)
		}
	}

	return resp, nil
}

func DeleteInstall(name, version string, log *zap.SugaredLogger) error {
	err := commonrepo.NewInstallColl().Delete(name, version)
	if err != nil {
		log.Errorf("Install.Delete %s error: %v", name, err)
		return e.ErrDeleteInstall
	}
	return nil
}

func InitInstallMap() map[string]*commonmodels.Install {
	installInfoPreset := make(map[string]*commonmodels.Install)

	installInfoPreset["dep-0.5.0"] = &commonmodels.Install{
		ObjectIDHex:  "5d11afca6bf097c0ea64bbcc",
		Name:         "dep",
		Version:      "0.5.0",
		DownloadPath: "http://resource.koderover.com/dep-v0.5.0-linux-x64.tar.gz",
		Scripts:      "mkdir -p $HOME/dep\ntar -C $HOME/dep -xzf ${FILEPATH}\nchmod +x $HOME/dep/dep",
		Envs:         []string{},
		BinPath:      "$HOME/dep",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["dep-0.5.3"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd02",
		Name:         "dep",
		Version:      "0.5.3",
		DownloadPath: "http://resource.koderover.com/dep-v0.5.3-linux-x64.tar.gz",
		Scripts:      "mkdir -p $HOME/dep\ntar -C $HOME/dep -xzf ${FILEPATH}\nchmod +x $HOME/dep/dep",
		Envs:         []string{},
		BinPath:      "$HOME/dep",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["dep-0.5.4"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd03",
		Name:         "dep",
		Version:      "0.5.4",
		DownloadPath: "http://resource.koderover.com/dep-v0.5.4-linux-x64.tar.gz",
		Scripts:      "mkdir -p $HOME/dep\ntar -C $HOME/dep -xzf ${FILEPATH}\nchmod +x $HOME/dep/dep",
		Envs:         []string{},
		BinPath:      "$HOME/dep",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["glide-0.13.1"] = &commonmodels.Install{
		ObjectIDHex:  "5d11afca6bf097c0ea64bbbe",
		Name:         "glide",
		Version:      "0.13.1",
		DownloadPath: "http://resource.koderover.com/glide-v0.13.1-linux-amd64.tar.gz",
		Scripts:      "mkdir -p $HOME/glide\ntar -C $HOME/glide -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/glide",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["glide-0.13.3"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd04",
		Name:         "glide",
		Version:      "0.13.3",
		DownloadPath: "http://resource.koderover.com/glide-v0.13.3-linux-amd64.tar.gz",
		Scripts:      "mkdir -p $HOME/glide\ntar -C $HOME/glide -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/glide",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["yarn-1.3.2"] = &commonmodels.Install{
		ObjectIDHex:  "5d11afca6bf097c0ea64bbbc",
		Name:         "yarn",
		Version:      "1.3.2",
		DownloadPath: "http://resource.koderover.com/yarn-v1.3.2.tar.gz",
		Scripts:      "mkdir -p $HOME/yarn\ntar -C $HOME/yarn -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/yarn/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["yarn-1.15.2"] = &commonmodels.Install{
		ObjectIDHex:  "5cc4294af92295138e2d8478",
		Name:         "yarn",
		Version:      "1.15.2",
		DownloadPath: "http://resource.koderover.com/yarn-v1.15.2.tar.gz",
		Scripts:      "mkdir -p $HOME/yarn\ntar -C $HOME/yarn -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/yarn/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["yarn-3.2.0"] = &commonmodels.Install{
		ObjectIDHex: "63722795351717b8ad70dd20",
		Name:        "yarn",
		Version:     "3.2.0",
		Scripts:     "npm install -g yarn\nyarn set version 3.2.0",
		Envs:        []string{},
		BinPath:     "",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["yarn-3.2.4"] = &commonmodels.Install{
		ObjectIDHex: "63722795351717b8ad70dd21",
		Name:        "yarn",
		Version:     "3.2.4",
		Scripts:     "npm install -g yarn\nyarn set version 3.2.4",
		Envs:        []string{},
		BinPath:     "",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["java-1.6.6"] = &commonmodels.Install{
		ObjectIDHex:  "5d9dce1ac024b6d199bba7e6",
		Name:         "java",
		Version:      "1.6.6",
		DownloadPath: "http://resource.koderover.com/jdk-6u6-p-linux-x64.tar.gz",
		Scripts:      "mkdir -p $HOME/jdk\ntar -C $HOME/jdk -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/jdk/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["java-1.7.8"] = &commonmodels.Install{
		ObjectIDHex:  "5d9dcf8ac024b6d199bc0bb4",
		Name:         "java",
		Version:      "1.7.8",
		DownloadPath: "http://resource.koderover.com/jdk-7u80-linux-x64.tar.gz",
		Scripts:      "mkdir -p $HOME/jdk\ntar -C $HOME/jdk -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/jdk/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["java-1.8.10"] = &commonmodels.Install{
		ObjectIDHex:  "5ca33d9ba24d76414eb277df",
		Name:         "java",
		Version:      "1.8.10",
		DownloadPath: "http://resource.koderover.com/jdk-8u101-linux-x64.tar.gz",
		Scripts:      "mkdir -p $HOME/jdk\ntar -C $HOME/jdk -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/jdk/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["java-1.9.0.4"] = &commonmodels.Install{
		ObjectIDHex:  "5d9dcfd7c024b6d199bc1dc5",
		Name:         "java",
		Version:      "1.9.0.4",
		DownloadPath: "http://resource.koderover.com/jdk-9.0.4_linux-x64_bin.tar.gz",
		Scripts:      "mkdir -p $HOME/jdk\ntar -C $HOME/jdk -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/jdk/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["java-1.10.0.2"] = &commonmodels.Install{
		ObjectIDHex:  "5d9dd024c024b6d199bc2fb5",
		Name:         "java",
		Version:      "1.10.0.2",
		DownloadPath: "http://resource.koderover.com/jdk-10.0.2_linux-x64_bin.tar.gz",
		Scripts:      "mkdir -p $HOME/jdk\ntar -C $HOME/jdk -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/jdk/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["java-1.11.0.3"] = &commonmodels.Install{
		ObjectIDHex:  "5d9dd09fc024b6d199bc5b08",
		Name:         "java",
		Version:      "1.11.0.3",
		DownloadPath: "http://resource.koderover.com/jdk-11.0.3_linux-x64_bin.tar.gz",
		Scripts:      "mkdir -p $HOME/jdk\ntar -C $HOME/jdk -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/jdk/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["java-1.12.0.1"] = &commonmodels.Install{
		ObjectIDHex:  "5d9dd0e1c024b6d199bc6a45",
		Name:         "java",
		Version:      "1.12.0.1",
		DownloadPath: "http://resource.koderover.com/jdk-12.0.1_linux-x64_bin.tar.gz",
		Scripts:      "mkdir -p $HOME/jdk\ntar -C $HOME/jdk -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/jdk/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["java-17"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd10",
		Name:         "java",
		Version:      "17",
		DownloadPath: "http://resource.koderover.com/jdk-17_linux-x64_bin.tar.gz",
		Scripts:      "mkdir -p $HOME/jdk\ntar -C $HOME/jdk -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/jdk/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["java-19"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd11",
		Name:         "java",
		Version:      "19",
		DownloadPath: "http://resource.koderover.com/jdk-19_linux-x64_bin.tar.gz",
		Scripts:      "mkdir -p $HOME/jdk\ntar -C $HOME/jdk -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/jdk/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["go-1.8.5"] = &commonmodels.Install{
		ObjectIDHex:  "5d11afca6bf097c0ea64bbc7",
		Name:         "go",
		Version:      "1.8.5",
		DownloadPath: "http://resource.koderover.com/go1.8.5.linux-amd64.tar.gz",
		Scripts:      "tar -C $HOME -xzf ${FILEPATH}",
		Envs:         []string{},
		BinPath:      "$HOME/go/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["go-1.9.7"] = &commonmodels.Install{
		ObjectIDHex:  "5d11afca6bf097c0ea64bbd4",
		Name:         "go",
		Version:      "1.9.7",
		DownloadPath: "http://resource.koderover.com/go1.9.7.linux-amd64.tar.gz",
		Scripts:      "tar -C $HOME -xzf ${FILEPATH}",
		Envs:         []string{},
		BinPath:      "$HOME/go/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["go-1.10.2"] = &commonmodels.Install{
		ObjectIDHex:  "5d11afca6bf097c0ea64bbc5",
		Name:         "go",
		Version:      "1.10.2",
		DownloadPath: "http://resource.koderover.com/go1.10.2.linux-amd64.tar.gz",
		Scripts:      "tar -C $HOME -xzf ${FILEPATH}",
		Envs:         []string{},
		BinPath:      "$HOME/go/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["go-1.11.5"] = &commonmodels.Install{
		ObjectIDHex:  "5d11afca6bf097c0ea64bbd5",
		Name:         "go",
		Version:      "1.11.5",
		DownloadPath: "http://resource.koderover.com/go1.11.5.linux-amd64.tar.gz",
		Scripts:      "tar -C $HOME -xzf ${FILEPATH}",
		Envs:         []string{},
		BinPath:      "$HOME/go/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["go-1.12.9"] = &commonmodels.Install{
		ObjectIDHex:  "5d81aae86bf097c0ea64d96d",
		Name:         "go",
		Version:      "1.12.9",
		DownloadPath: "http://resource.koderover.com/go1.12.9.linux-amd64.tar.gz",
		Scripts:      "tar -C $HOME -xzf ${FILEPATH}",
		Envs:         []string{},
		BinPath:      "$HOME/go/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["go-1.13"] = &commonmodels.Install{
		ObjectIDHex:  "5d81ad3b6bf097c0ea64d96e",
		Name:         "go",
		Version:      "1.13",
		DownloadPath: "http://resource.koderover.com/go1.13.linux-amd64.tar.gz",
		Scripts:      "tar -C $HOME -xzf ${FILEPATH}",
		Envs:         []string{},
		BinPath:      "$HOME/go/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["go-1.18.8"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd00",
		Name:         "go",
		Version:      "1.18.8",
		DownloadPath: "http://resource.koderover.com/go1.18.8.linux-amd64.tar.gz",
		Scripts:      "tar -C $HOME -xzf ${FILEPATH}",
		Envs:         []string{},
		BinPath:      "$HOME/go/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["go-1.19.3"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd01",
		Name:         "go",
		Version:      "1.19.3",
		DownloadPath: "http://resource.koderover.com/go1.19.3.linux-amd64.tar.gz",
		Scripts:      "tar -C $HOME -xzf ${FILEPATH}",
		Envs:         []string{},
		BinPath:      "$HOME/go/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["phantomjs-2.1.1"] = &commonmodels.Install{
		ObjectIDHex:  "5d11afca6bf097c0ea64bbbb",
		Name:         "phantomjs",
		Version:      "2.1.1",
		DownloadPath: "http://resource.koderover.com/phantomjs-2.1.1-linux-x86_64.tar.bz2",
		Scripts:      "mkdir -p $HOME/phantomjs\ntar -C $HOME/phantomjs -jxf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/phantomjs/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["python-2.7.16"] = &commonmodels.Install{
		ObjectIDHex: "5d9ea003c024b6d199fcb61b",
		Name:        "python",
		Version:     "2.7.16",
		Scripts:     "sudo apt-get install build-essential\rcurl -fsSl http://resource.koderover.com/Python-2.7.16.tgz -o /tmp/Python-2.7.16.tgz\rmkdir -p /opt/python\rtar  -C  /opt/python -zxf  /tmp/Python-2.7.16.tgz\rcd /opt/python/Python-2.7.16\r./configure --prefix=/usr/local/python && make  && make install",
		Envs:        []string{},
		BinPath:     "/usr/local/python/bin",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["python-3.7.4"] = &commonmodels.Install{
		ObjectIDHex: "5d9ea08ec024b6d199fd27d5",
		Name:        "python",
		Version:     "3.7.4",
		Scripts:     "sudo apt-get install build-essential\rcurl -fsSl http://resource.koderover.com/Python-3.7.4.tgz -o /tmp/Python-3.7.4.tgz\rmkdir -p /opt/python\rtar  -C  /opt/python -zxf  /tmp/Python-3.7.4.tgz\rcd /opt/python/Python-3.7.4\r./configure --prefix=/usr/local/python && make  && make install",
		Envs:        []string{},
		BinPath:     "/usr/local/python/bin",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["python-3.10.8"] = &commonmodels.Install{
		ObjectIDHex: "63722795351717b8ad70dd15",
		Name:        "python",
		Version:     "3.10.8",
		Scripts:     "sudo apt install software-properties-common -y\rsudo add-apt-repository ppa:deadsnakes/ppa\rsudo apt install -y python3.10",
		Envs:        []string{},
		BinPath:     "",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["python-3.11"] = &commonmodels.Install{
		ObjectIDHex: "63722795351717b8ad70dd16",
		Name:        "python",
		Version:     "3.11",
		Scripts:     "sudo apt install software-properties-common -y\rsudo add-apt-repository ppa:deadsnakes/ppa\rsudo apt install -y python3.11",
		Envs:        []string{},
		BinPath:     "",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["jMeter-3.2"] = &commonmodels.Install{
		ObjectIDHex:  "5cc68b09f92295138ed398c2",
		Name:         "jMeter",
		Version:      "3.2",
		DownloadPath: "http://resource.koderover.com/apach-jmeter-3.2.tar.gz",
		Scripts:      "mkdir -p $HOME/jmeter\ntar -C $HOME/jmeter -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/jmeter/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["jMeter-5.4.3"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd06",
		Name:         "jMeter",
		Version:      "5.4.3",
		DownloadPath: "http://resource.koderover.com/apache-jmeter-5.4.3.tgz",
		Scripts:      "mkdir -p $HOME/jmeter\ntar -C $HOME/jmeter -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/jmeter/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["jMeter-5.5"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd07",
		Name:         "jMeter",
		Version:      "5.5",
		DownloadPath: "http://resource.koderover.com/apache-jmeter-5.5.tgz",
		Scripts:      "mkdir -p $HOME/jmeter\ntar -C $HOME/jmeter -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/jmeter/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["maven-3.3.9"] = &commonmodels.Install{
		ObjectIDHex:  "5d11afca6bf097c0ea64bbc0",
		Name:         "maven",
		Version:      "3.3.9",
		DownloadPath: "http://resource.koderover.com/apache-maven-3.3.9-bin.tar.gz",
		Scripts:      "mkdir -p $HOME/maven\ntar -C $HOME/maven -xzf ${FILEPATH} --strip-components=1\n\n# customize .m2 dir\nexport M2_HOME=$HOME/maven\nmkdir -p $WORKSPACE/.m2/repository\ncat >$HOME/maven/conf/settings.xml <<EOF\n<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<settings xmlns=\"http://maven.apache.org/SETTINGS/1.0.0\"\n          xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\"\n          xsi:schemaLocation=\"http://maven.apache.org/SETTINGS/1.0.0 http://maven.apache.org/xsd/settings-1.0.0.xsd\">\n  <localRepository>$WORKSPACE/.m2/repository</localRepository>\n  <pluginGroups/>\n  <servers/>\n  <mirrors>\n    <mirror>\n      <id>repo1</id>\n      <mirrorOf>central</mirrorOf>\n      <name>repo1</name>\n      <url>http://repo1.maven.org/maven2</url>\n    </mirror>\n  </mirrors> \n  <profiles/>\n</settings>\nEOF",
		Envs:         []string{},
		BinPath:      "$HOME/maven/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["maven-3.8.6"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd12",
		Name:         "maven",
		Version:      "3.8.6",
		DownloadPath: "http://resource.koderover.com/apache-maven-3.8.6-bin.tar.gz",
		Scripts:      "mkdir -p $HOME/maven\ntar -C $HOME/maven -xzf ${FILEPATH} --strip-components=1\n\n# customize .m2 dir\nexport M2_HOME=$HOME/maven\nmkdir -p $WORKSPACE/.m2/repository\ncat >$HOME/maven/conf/settings.xml <<EOF\n<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<settings xmlns=\"http://maven.apache.org/SETTINGS/1.0.0\"\n          xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\"\n          xsi:schemaLocation=\"http://maven.apache.org/SETTINGS/1.0.0 http://maven.apache.org/xsd/settings-1.0.0.xsd\">\n  <localRepository>$WORKSPACE/.m2/repository</localRepository>\n  <pluginGroups/>\n  <servers/>\n  <mirrors>\n    <mirror>\n      <id>repo1</id>\n      <mirrorOf>central</mirrorOf>\n      <name>repo1</name>\n      <url>http://repo1.maven.org/maven2</url>\n    </mirror>\n  </mirrors> \n  <profiles/>\n</settings>\nEOF",
		Envs:         []string{},
		BinPath:      "$HOME/maven/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["php-5.6"] = &commonmodels.Install{
		ObjectIDHex: "5d9dd670c024b6d199bdd2ba",
		Name:        "php",
		Version:     "5.6",
		Scripts:     "sudo LC_ALL=C.UTF-8 add-apt-repository ppa:ondrej/php\nsudo apt-get install software-properties-common\nsudo apt-get update\nsudo apt-get install -y php5.6",
		Envs:        []string{},
		BinPath:     "/usr/bin/php5.6",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["php-7.3"] = &commonmodels.Install{
		ObjectIDHex: "5d9dd766c024b6d199be0b9a",
		Name:        "php",
		Version:     "7.3",
		Scripts:     "sudo LC_ALL=C.UTF-8 add-apt-repository ppa:ondrej/php\nsudo apt-get install software-properties-common\nsudo apt-get update\nsudo apt-get install -y php7.3",
		Envs:        []string{},
		BinPath:     "/usr/bin/php7.3",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["php-8.0.25"] = &commonmodels.Install{
		ObjectIDHex: "63722795351717b8ad70dd13",
		Name:        "php",
		Version:     "8.0.25",
		Scripts:     "sudo apt install software-properties-common -y\nsudo LC_ALL=C.UTF-8 add-apt-repository ppa:ondrej/php\nsudo apt-get install software-properties-common\nsudo apt-get update\nsudo apt-get install -y php8.0",
		Envs:        []string{},
		BinPath:     "",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["php-8.1.12"] = &commonmodels.Install{
		ObjectIDHex: "63722795351717b8ad70dd14",
		Name:        "php",
		Version:     "8.1.12",
		Scripts:     "sudo apt install software-properties-common -y\nsudo LC_ALL=C.UTF-8 add-apt-repository ppa:ondrej/php\nsudo apt-get install software-properties-common\nsudo apt-get update\nsudo apt-get install -y php8.1",
		Envs:        []string{},
		BinPath:     "",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["node-8.11.4"] = &commonmodels.Install{
		ObjectIDHex:  "5d11afca6bf097c0ea64bbcd",
		Name:         "node",
		Version:      "8.11.4",
		DownloadPath: "http://resource.koderover.com/node-v8.11.4-linux-x64.tar.gz",
		Scripts:      "mkdir -p $HOME/node \ntar -C $HOME/node -xzf ${FILEPATH} --strip-components=1 \nnpm config --global set registry https://registry.npm.taobao.org",
		Envs:         []string{},
		BinPath:      "$HOME/node/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["node-8.15.0"] = &commonmodels.Install{
		ObjectIDHex:  "5cc429a2f92295138e2d8966",
		Name:         "node",
		Version:      "8.15.0",
		DownloadPath: "http://resource.koderover.com/node-v8.15.0-linux-x64.tar.gz",
		Scripts:      "mkdir -p $HOME/node \ntar -C $HOME/node -xzf ${FILEPATH} --strip-components=1 \nnpm config --global set registry https://registry.npm.taobao.org",
		Envs:         []string{},
		BinPath:      "$HOME/node/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["node-16.18.1"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd08",
		Name:         "node",
		Version:      "16.18.1",
		DownloadPath: "http://resource.koderover.com/node-v16.18.1-linux-x64.tar.gz",
		Scripts:      "mkdir -p $HOME/node \ntar -C $HOME/node -xzf ${FILEPATH} --strip-components=1 \nnpm config --global set registry https://registry.npm.taobao.org",
		Envs:         []string{},
		BinPath:      "$HOME/node/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["node-18.12.1"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd09",
		Name:         "node",
		Version:      "18.12.1",
		DownloadPath: "http://resource.koderover.com/node-v18.12.1-linux-x64.tar.gz",
		Scripts:      "mkdir -p $HOME/node \ntar -C $HOME/node -xzf ${FILEPATH} --strip-components=1 \nnpm config --global set registry https://registry.npm.taobao.org",
		Envs:         []string{},
		BinPath:      "$HOME/node/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["bower-latest"] = &commonmodels.Install{
		ObjectIDHex: "5d11afca6bf097c0ea64bbba",
		Name:        "bower",
		Version:     "latest",
		Scripts:     "npm install -g bower",
		Envs:        []string{},
		BinPath:     "",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["ginkgo-1.6.0"] = &commonmodels.Install{
		ObjectIDHex:  "5d11afca6bf097c0ea64bbce",
		Name:         "ginkgo",
		Version:      "1.6.0",
		DownloadPath: "http://resource.koderover.com/ginkgo-v1.6.0-Linux.tar.gz",
		Scripts:      "mkdir -p $HOME/ginkgo\ntar -C $HOME/ginkgo -xzf ${FILEPATH}\nchmod +x $HOME/ginkgo/ginkgo",
		Envs:         []string{},
		BinPath:      "$HOME/ginkgo",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["ginkgo-2.2.0"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd05",
		Name:         "ginkgo",
		Version:      "2.2.0",
		DownloadPath: "http://resource.koderover.com/ginkgo-v2.2.0-Linux.tar.gz",
		Scripts:      "mkdir -p $HOME/ginkgo\ntar -C $HOME/ginkgo -xzf ${FILEPATH}\nchmod +x $HOME/ginkgo/ginkgo",
		Envs:         []string{},
		BinPath:      "$HOME/ginkgo",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["ginkgo-2.3.1"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd17",
		Name:         "ginkgo",
		Version:      "2.3.1",
		DownloadPath: "http://resource.koderover.com/ginkgo-v2.3.1-Linux.tar.gz",
		Scripts:      "mkdir -p $HOME/ginkgo\ntar -C $HOME/ginkgo -xzf ${FILEPATH}\nchmod +x $HOME/ginkgo/ginkgo",
		Envs:         []string{},
		BinPath:      "$HOME/ginkgo",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["ginkgo-2.4.0"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd18",
		Name:         "ginkgo",
		Version:      "2.4.0",
		DownloadPath: "http://resource.koderover.com/ginkgo-v2.4.0-Linux.tar.gz",
		Scripts:      "mkdir -p $HOME/ginkgo\ntar -C $HOME/ginkgo -xzf ${FILEPATH}\nchmod +x $HOME/ginkgo/ginkgo",
		Envs:         []string{},
		BinPath:      "$HOME/ginkgo",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["ginkgo-2.5.0"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd19",
		Name:         "ginkgo",
		Version:      "2.5.0",
		DownloadPath: "http://resource.koderover.com/ginkgo-v2.5.0-Linux.tar.gz",
		Scripts:      "mkdir -p $HOME/ginkgo\ntar -C $HOME/ginkgo -xzf ${FILEPATH}\nchmod +x $HOME/ginkgo/ginkgo",
		Envs:         []string{},
		BinPath:      "$HOME/ginkgo",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	return installInfoPreset
}
