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
