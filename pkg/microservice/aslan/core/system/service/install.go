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

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
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
		DownloadPath: "https://github.com/golang/dep/releases/download/v0.5.0/dep-linux-amd64",
		Scripts:      "mkdir -p $HOME/dep\ncp ${FILEPATH} $HOME/dep/dep\nchmod +x $HOME/dep/dep",
		Envs:         []string{},
		BinPath:      "$HOME/dep",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["dep-0.5.3"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd02",
		Name:         "dep",
		Version:      "0.5.3",
		DownloadPath: "https://github.com/golang/dep/releases/download/v0.5.3/dep-linux-amd64",
		Scripts:      "mkdir -p $HOME/dep\ncp ${FILEPATH} $HOME/dep/dep\nchmod +x $HOME/dep/dep",
		Envs:         []string{},
		BinPath:      "$HOME/dep",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["dep-0.5.4"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd03",
		Name:         "dep",
		Version:      "0.5.4",
		DownloadPath: "https://github.com/golang/dep/releases/download/v0.5.4/dep-linux-amd64",
		Scripts:      "mkdir -p $HOME/dep\ncp ${FILEPATH} $HOME/dep/dep\nchmod +x $HOME/dep/dep",
		Envs:         []string{},
		BinPath:      "$HOME/dep",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["glide-0.13.1"] = &commonmodels.Install{
		ObjectIDHex:  "5d11afca6bf097c0ea64bbbe",
		Name:         "glide",
		Version:      "0.13.1",
		DownloadPath: "https://github.com/Masterminds/glide/releases/download/v0.13.1/glide-v0.13.1-linux-amd64.tar.gz",
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
		DownloadPath: "https://github.com/Masterminds/glide/releases/download/v0.13.3/glide-v0.13.3-linux-amd64.tar.gz",
		Scripts:      "mkdir -p $HOME/glide\ntar -C $HOME/glide -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/glide",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["yarn-1.15.2"] = &commonmodels.Install{
		ObjectIDHex: "5cc4294af92295138e2d8478",
		Name:        "yarn",
		Version:     "1.15.2",
		Scripts:     "npm install -g yarn\nyarn set version 1.15.2",
		Envs:        []string{},
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["yarn-3.2.0"] = &commonmodels.Install{
		ObjectIDHex: "63722795351717b8ad70dd20",
		Name:        "yarn",
		Version:     "3.2.0",
		Scripts:     "npm install -g yarn\nyarn set version 3.2.0",
		Envs:        []string{},
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["yarn-3.2.4"] = &commonmodels.Install{
		ObjectIDHex: "63722795351717b8ad70dd21",
		Name:        "yarn",
		Version:     "3.2.4",
		Scripts:     "npm install -g yarn\nyarn set version 3.2.4",
		Envs:        []string{},
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["java-1.8.20"] = &commonmodels.Install{
		ObjectIDHex:  "5ca33d9ba24d76414eb277df",
		Name:         "java",
		Version:      "1.8.20",
		DownloadPath: "https://repo.huaweicloud.com/java/jdk/8u201-b09/jdk-8u201-linux-x64.tar.gz",
		Scripts:      "mkdir -p $HOME/jdk\ntar -C $HOME/jdk -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/jdk/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["java-1.11.0.2"] = &commonmodels.Install{
		ObjectIDHex:  "5d9dd09fc024b6d199bc5b08",
		Name:         "java",
		Version:      "1.11.0.2",
		DownloadPath: "https://repo.huaweicloud.com/java/jdk/11.0.2+9/jdk-11.0.2_linux-x64_bin.tar.gz",
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
		DownloadPath: "https://repo.huaweicloud.com/java/jdk/12.0.1+12/jdk-12.0.1_linux-x64_bin.tar.gz",
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
		DownloadPath: "https://download.oracle.com/java/19/archive/jdk-19_linux-x64_bin.tar.gz",
		Scripts:      "mkdir -p $HOME/jdk\ntar -C $HOME/jdk -xzf ${FILEPATH} --strip-components=1",
		Envs:         []string{},
		BinPath:      "$HOME/jdk/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["go-1.13"] = &commonmodels.Install{
		ObjectIDHex:  "5d81ad3b6bf097c0ea64d96e",
		Name:         "go",
		Version:      "1.13",
		DownloadPath: "https://golang.google.cn/dl/go1.13.linux-amd64.tar.gz",
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
		DownloadPath: "https://golang.google.cn/dl/go1.18.8.linux-amd64.tar.gz",
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
		DownloadPath: "https://golang.google.cn/dl/go1.19.3.linux-amd64.tar.gz",
		Scripts:      "tar -C $HOME -xzf ${FILEPATH}",
		Envs:         []string{},
		BinPath:      "$HOME/go/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["go-1.20.7"] = &commonmodels.Install{
		ObjectIDHex:  "64cc9658d2a0da06433d12fe",
		Name:         "go",
		Version:      "1.20.7",
		DownloadPath: "https://golang.google.cn/dl/go1.20.7.linux-amd64.tar.gz",
		Scripts:      "tar -C $HOME -xzf ${FILEPATH}",
		Envs:         []string{},
		BinPath:      "$HOME/go/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["phantomjs-2.1.1"] = &commonmodels.Install{
		ObjectIDHex: "5d11afca6bf097c0ea64bbbb",
		Name:        "phantomjs",
		Version:     "2.1.1",
		Scripts:     "mkdir -p $HOME/phantomjs\n\twget -q https://bitbucket.org/ariya/phantomjs/downloads/phantomjs-2.1.1-linux-x86_64.tar.bz2 -P $HOME/tmp\n\ttar -C $HOME/phantomjs -jxf $HOME/tmp/phantomjs-2.1.1-linux-x86_64.tar.bz2 --strip-components=1",
		Envs:        []string{},
		BinPath:     "$HOME/phantomjs/bin",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["python-2.7.16"] = &commonmodels.Install{
		ObjectIDHex: "5d9ea003c024b6d199fcb61b",
		Name:        "python",
		Version:     "2.7.16",
		Scripts:     "sudo apt-get install build-essential\ncurl -fsSl https://www.python.org/ftp/python/2.7.16/Python-2.7.16.tgz -o /tmp/Python-2.7.16.tgz\nmkdir -p /opt/python\ntar  -C  /opt/python -zxf  /tmp/Python-2.7.16.tgz\ncd /opt/python/Python-2.7.16\n./configure --prefix=/usr/local/python && make  && make install\nsudo ln -s /usr/local/python/bin/python2.7 /usr/local/bin/python2.7",
		Envs:        []string{},
		BinPath:     "/usr/local/python/bin",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["python-3.7.4"] = &commonmodels.Install{
		ObjectIDHex: "5d9ea08ec024b6d199fd27d5",
		Name:        "python",
		Version:     "3.7.4",
		Scripts:     "sudo apt-get install -y build-essential libffi-dev\ncurl -fsSl https://www.python.org/ftp/python/3.7.4/Python-3.7.4.tgz -o /tmp/Python-3.7.4.tgz\nmkdir -p /opt/python\ntar  -C  /opt/python -zxf  /tmp/Python-3.7.4.tgz\ncd /opt/python/Python-3.7.4\n./configure --prefix=/usr/local/python && make  && make install\nsudo ln -s /usr/local/python/bin/python3.7 /usr/local/bin/python3.7",
		Envs:        []string{},
		BinPath:     "/usr/local/python/bin",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["python-3.10.8"] = &commonmodels.Install{
		ObjectIDHex: "63722795351717b8ad70dd15",
		Name:        "python",
		Version:     "3.10.8",
		Scripts:     "sudo apt-get update\nsudo apt install software-properties-common -y\nsudo add-apt-repository ppa:deadsnakes/ppa\nsudo apt install -y python3.10",
		Envs:        []string{},
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["python-3.11"] = &commonmodels.Install{
		ObjectIDHex: "63722795351717b8ad70dd16",
		Name:        "python",
		Version:     "3.11",
		Scripts:     "sudo apt-get update\nsudo apt install software-properties-common -y\nsudo add-apt-repository ppa:deadsnakes/ppa\nsudo apt install -y python3.11",
		Envs:        []string{},
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["jMeter-5.5"] = &commonmodels.Install{
		ObjectIDHex: "63722795351717b8ad70dd07",
		Name:        "jMeter",
		Version:     "5.5",
		Scripts:     "mkdir -p $HOME/jmeter\nwget -q https://archive.apache.org/dist/jmeter/binaries/apache-jmeter-5.5.tgz -P $HOME/tmp\ntar -C $HOME/jmeter -xzf $HOME/tmp/apache-jmeter-5.5.tgz --strip-components=1",
		Envs:        []string{},
		BinPath:     "$HOME/jmeter/bin",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["maven-3.3.9"] = &commonmodels.Install{
		ObjectIDHex:  "5d11afca6bf097c0ea64bbc0",
		Name:         "maven",
		Version:      "3.3.9",
		DownloadPath: "https://archive.apache.org/dist/maven/maven-3/3.3.9/binaries/apache-maven-3.3.9-bin.tar.gz",
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
		DownloadPath: "https://archive.apache.org/dist/maven/maven-3/3.8.6/binaries/apache-maven-3.8.6-bin.tar.gz",
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
		Scripts:     "sudo apt-get update\nsudo apt install software-properties-common -y\nsudo LC_ALL=C.UTF-8 add-apt-repository ppa:ondrej/php\nsudo apt-get install software-properties-common\nsudo apt-get update\nsudo apt-get install -y php5.6",
		Envs:        []string{},
		BinPath:     "/usr/bin/php5.6",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["php-7.3"] = &commonmodels.Install{
		ObjectIDHex: "5d9dd766c024b6d199be0b9a",
		Name:        "php",
		Version:     "7.3",
		Scripts:     "sudo apt-get update\nsudo apt install software-properties-common -y\nsudo LC_ALL=C.UTF-8 add-apt-repository ppa:ondrej/php\nsudo apt-get install software-properties-common\nsudo apt-get update\nsudo apt-get install -y php7.3",
		Envs:        []string{},
		BinPath:     "/usr/bin/php7.3",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["php-8.0.25"] = &commonmodels.Install{
		ObjectIDHex: "63722795351717b8ad70dd13",
		Name:        "php",
		Version:     "8.0.25",
		Scripts:     "sudo apt-get update\nsudo apt install software-properties-common -y\nsudo apt install software-properties-common -y\nsudo LC_ALL=C.UTF-8 add-apt-repository ppa:ondrej/php\nsudo apt-get install software-properties-common\nsudo apt-get update\nsudo apt-get install -y php8.0",
		Envs:        []string{},
		BinPath:     "",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["php-8.1.12"] = &commonmodels.Install{
		ObjectIDHex: "63722795351717b8ad70dd14",
		Name:        "php",
		Version:     "8.1.12",
		Scripts:     "sudo apt-get update\nsudo apt install software-properties-common -y\nsudo apt install software-properties-common -y\nsudo LC_ALL=C.UTF-8 add-apt-repository ppa:ondrej/php\nsudo apt-get install software-properties-common\nsudo apt-get update\nsudo apt-get install -y php8.1",
		Envs:        []string{},
		BinPath:     "",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["node-12.22.12"] = &commonmodels.Install{
		ObjectIDHex:  "5d11afca6bf097c0ea64bbcd",
		Name:         "node",
		Version:      "12.22.12",
		DownloadPath: "https://nodejs.org/dist/v12.22.12/node-v12.22.12-linux-x64.tar.xz",
		Scripts:      "mkdir -p $HOME/node \ntar -C $HOME/node -xJf ${FILEPATH} --strip-components=1 \nnpm config --global set registry https://registry.npmmirror.com",
		Envs:         []string{},
		BinPath:      "$HOME/node/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["node-14.21.3"] = &commonmodels.Install{
		ObjectIDHex:  "5cc429a2f92295138e2d8966",
		Name:         "node",
		Version:      "14.21.3",
		DownloadPath: "https://nodejs.org/dist/v14.21.3/node-v14.21.3-linux-x64.tar.xz",
		Scripts:      "mkdir -p $HOME/node \ntar -C $HOME/node -xJf ${FILEPATH} --strip-components=1 \nnpm config --global set registry https://registry.npmmirror.com",
		Envs:         []string{},
		BinPath:      "$HOME/node/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["node-16.20.2"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd08",
		Name:         "node",
		Version:      "16.20.2",
		DownloadPath: "https://nodejs.org/download/release/v16.20.2/node-v16.20.2-linux-x64.tar.xz",
		Scripts:      "mkdir -p $HOME/node \ntar -C $HOME/node -xJf ${FILEPATH} --strip-components=1 \nnpm config --global set registry https://registry.npmmirror.com",
		Envs:         []string{},
		BinPath:      "$HOME/node/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["node-18.17.1"] = &commonmodels.Install{
		ObjectIDHex:  "63722795351717b8ad70dd09",
		Name:         "node",
		Version:      "18.17.1",
		DownloadPath: "https://nodejs.org/dist/v18.17.1/node-v18.17.1-linux-x64.tar.xz",
		Scripts:      "mkdir -p $HOME/node \ntar -C $HOME/node -xJf ${FILEPATH} --strip-components=1 \nnpm config --global set registry https://registry.npmmirror.com",
		Envs:         []string{},
		BinPath:      "$HOME/node/bin",
		Enabled:      true,
		UpdateBy:     setting.SystemUser,
	}

	installInfoPreset["bower-latest"] = &commonmodels.Install{
		ObjectIDHex: "5d11afca6bf097c0ea64bbba",
		Name:        "bower",
		Version:     "latest",
		Scripts:     "sudo apt-get update\nsudo apt install npm -y --no-install-recommends\nnpm install -g bower",
		Envs:        []string{},
		BinPath:     "",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	installInfoPreset["ginkgo-2.5.0"] = &commonmodels.Install{
		ObjectIDHex: "63722795351717b8ad70dd19",
		Name:        "ginkgo",
		Version:     "2.5.0",
		Scripts:     "go env -w GOPROXY=https://goproxy.cn,direct\ngo install github.com/onsi/ginkgo/v2/ginkgo@v2.5.0",
		Envs:        []string{},
		BinPath:     "$HOME/ginkgo",
		Enabled:     true,
		UpdateBy:    setting.SystemUser,
	}

	return installInfoPreset
}
