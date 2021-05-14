module github.com/koderover/zadig

go 1.15

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/andygrunwald/go-gerrit v0.0.0-20171029143327-95b11af228a1
	github.com/bitly/go-simplejson v0.5.0 // indirect
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/bndr/gojenkins v1.1.0
	github.com/boj/redistore v0.0.0-20170419030511-4562487a4bee // indirect
	github.com/bradfitz/gomemcache v0.0.0-20190913173617-a41fca850d0b // indirect
	github.com/bradleyfalzon/ghinstallation v1.1.1
	github.com/bradleypeabody/gorilla-sessions-memcache v0.0.0-20170503205333-75ee37df8664 // indirect
	github.com/bshuster-repo/logrus-logstash-hook v1.0.0 // indirect
	github.com/bugsnag/bugsnag-go v2.1.0+incompatible // indirect
	github.com/bugsnag/panicwrap v1.3.1 // indirect
	github.com/cenkalti/backoff/v3 v3.0.0
	github.com/coocood/freecache v1.1.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.4.2-0.20200204220554-5f6d6f3f2203
	github.com/docker/go-connections v0.4.0
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/garyburd/redigo v1.6.2 // indirect
	github.com/gin-contrib/sessions v0.0.0-20170917021446-cccdeef56346
	github.com/gin-contrib/sse v0.1.0
	github.com/gin-gonic/gin v1.6.3
	github.com/go-ini/ini v1.42.0 // indirect
	github.com/go-openapi/spec v0.19.5 // indirect
	github.com/go-openapi/swag v0.19.6 // indirect
	github.com/gofrs/uuid v4.0.0+incompatible // indirect
	github.com/google/go-github/v35 v35.1.0
	github.com/gorilla/handlers v1.5.1 // indirect
	github.com/gorilla/mux v1.7.3
	github.com/gorilla/securecookie v1.1.1 // indirect
	github.com/gorilla/sessions v0.0.0-20160922145804-ca9ada445741 // indirect
	github.com/gorilla/websocket v1.4.2
	github.com/gotestyourself/gotestyourself v2.2.0+incompatible // indirect
	github.com/gregjones/httpcache v0.0.0-20181110185634-c63ab54fda8f
	github.com/hashicorp/go-multierror v1.0.0
	github.com/jasonlvhit/gocron v0.0.0-20171226191223-3c914c8681c3
	github.com/jinzhu/now v1.1.2
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/kidstuff/mongostore v0.0.0-20151002152336-256d65ac5b0e // indirect
	github.com/koding/websocketproxy v0.0.0-20180412071414-b8568f354aec
	github.com/lusis/go-slackbot v0.0.0-20180109053408-401027ccfef5 // indirect
	github.com/lusis/slack-test v0.0.0-20190426140909-c40012f20018 // indirect
	github.com/minio/minio-go v3.0.3-0.20171223001112-e163d8055f79+incompatible
	github.com/moul/http2curl v1.0.0 // indirect
	github.com/nlopes/slack v0.3.0
	github.com/nsqio/go-nsq v1.0.7
	github.com/nwaples/rardecode v1.0.0 // indirect
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/selinux v1.6.0 // indirect
	github.com/parnurzeal/gorequest v0.2.15
	github.com/pkg/errors v0.9.1
	github.com/qiniu/x v0.0.0-20150721034113-f512abcf45ab
	github.com/rfyiamcool/cronlib v1.0.0
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.0
	github.com/stevvooe/resumable v0.0.0-20180830230917-22b14a53ba50 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/ugorji/go v1.2.0 // indirect
	github.com/xanzy/go-gitlab v0.44.0
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/yvasiyarov/go-metrics v0.0.0-20150112132944-c25f46c4b940 // indirect
	github.com/yvasiyarov/gorelic v0.0.7 // indirect
	github.com/yvasiyarov/newrelic_platform_go v0.0.0-20160601141957-9c099fbc30e9 // indirect
	go.mongodb.org/mongo-driver v1.5.0
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
	golang.org/x/net v0.0.0-20210316092652-d523dce5a7f4
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	gopkg.in/asn1-ber.v1 v1.0.0-20181015200546-f715ec2f112d // indirect
	gopkg.in/ldap.v2 v2.5.1
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22
	gopkg.in/mholt/archiver.v3 v3.1.1
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
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

	github.com/qiniu/x => ./third_party/qiniu/x
	gopkg.in/mgo.v2 => gopkg.in/mgo.v2 v2.0.0-20160818020120-3f83fa500528

	k8s.io/api => k8s.io/api v0.20.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.20.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.6
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.20.6
	k8s.io/client-go => k8s.io/client-go v0.20.6
	k8s.io/kubectl => k8s.io/kubectl v0.20.6

	qiniupkg.com/x => ./third_party/qiniupkg.com/x
)
