module github.com/koderover/zadig

go 1.16

require (
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/andygrunwald/go-gerrit v0.0.0-20171029143327-95b11af228a1
	github.com/aws/aws-sdk-go v1.34.28
	github.com/bndr/gojenkins v1.1.0
	github.com/bradleyfalzon/ghinstallation v1.1.1
	github.com/bshuster-repo/logrus-logstash-hook v1.0.0 // indirect
	github.com/bugsnag/bugsnag-go v2.1.0+incompatible // indirect
	github.com/bugsnag/panicwrap v1.3.1 // indirect
	github.com/cenkalti/backoff/v3 v3.0.0
	github.com/coocood/freecache v1.1.0
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.4.2-0.20200204220554-5f6d6f3f2203
	github.com/docker/go-connections v0.4.0
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/garyburd/redigo v1.6.2 // indirect
	github.com/gin-contrib/sessions v0.0.3
	github.com/gin-contrib/sse v0.1.0
	github.com/gin-gonic/gin v1.7.2
	github.com/go-openapi/spec v0.19.5 // indirect
	github.com/go-resty/resty/v2 v2.6.0
	github.com/gofrs/uuid v4.0.0+incompatible // indirect
	github.com/google/go-github/v35 v35.3.0
	github.com/google/uuid v1.2.0
	github.com/gorilla/handlers v1.5.1 // indirect
	github.com/gorilla/mux v1.7.3
	github.com/gorilla/websocket v1.4.2
	github.com/gotestyourself/gotestyourself v2.2.0+incompatible // indirect
	github.com/gregjones/httpcache v0.0.0-20181110185634-c63ab54fda8f
	github.com/hashicorp/go-multierror v1.1.1
	github.com/huaweicloud/huaweicloud-sdk-go-v3 v0.0.50
	github.com/jasonlvhit/gocron v0.0.0-20171226191223-3c914c8681c3
	github.com/jinzhu/now v1.1.2
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/nsqio/go-nsq v1.0.7
	github.com/nwaples/rardecode v1.0.0 // indirect
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/selinux v1.6.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/rfyiamcool/cronlib v1.0.0
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v1.1.3 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.0
	github.com/stevvooe/resumable v0.0.0-20180830230917-22b14a53ba50 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/swaggo/files v0.0.0-20190704085106-630677cd5c14
	github.com/swaggo/gin-swagger v1.3.0
	github.com/swaggo/swag v1.5.1
	github.com/ugorji/go v1.2.0 // indirect
	github.com/xanzy/go-gitlab v0.50.0
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/yvasiyarov/go-metrics v0.0.0-20150112132944-c25f46c4b940 // indirect
	github.com/yvasiyarov/gorelic v0.0.7 // indirect
	github.com/yvasiyarov/newrelic_platform_go v0.0.0-20160601141957-9c099fbc30e9 // indirect
	go.mongodb.org/mongo-driver v1.5.0
	go.uber.org/zap v1.17.0
	golang.org/x/net v0.0.0-20210405180319-a5a99cb37ef4
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	gopkg.in/mholt/archiver.v3 v3.1.1
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	helm.sh/helm/v3 v3.5.4
	k8s.io/api v0.20.6
	k8s.io/apiextensions-apiserver v0.20.4
	k8s.io/apimachinery v0.20.6
	k8s.io/cli-runtime v0.20.6
	k8s.io/client-go v0.20.6
	k8s.io/kubectl v0.20.4
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/controller-runtime v0.8.2
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/docker/distribution => github.com/docker/distribution v2.6.0-rc.1.0.20170726174610-edc3ab29cdff+incompatible
	github.com/docker/docker => github.com/docker/docker v0.0.0-20180502112750-51a9119f6b81
	github.com/docker/go-connections => github.com/docker/go-connections v0.3.1-0.20180212134524-7beb39f0b969

	k8s.io/api => k8s.io/api v0.20.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.20.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.6
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.20.6
	k8s.io/client-go => k8s.io/client-go v0.20.6
	k8s.io/kubectl => k8s.io/kubectl v0.20.6
)
