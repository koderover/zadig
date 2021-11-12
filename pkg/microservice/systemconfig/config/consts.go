package config

const (
	ENVMysqlDexDB = "MYSQL_DEX_DB"
	FeatureFlag   = "feature-gates"
)

var CodeHostMap = map[string]string{
	"1": "gitlab",
	"2": "github",
	"3": "gerrit",
	"4": "codehub",
}

var CodeHostSource2TypeMap = map[string]string{
	"gitlab":  "1",
	"github":  "2",
	"gerrit":  "3",
	"codehub": "4",
}
