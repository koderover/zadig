package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/client"
	dms "github.com/alibabacloud-go/dms-enterprise-20181101/client"
	credential "github.com/aliyun/credentials-go/credentials"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/spf13/viper"
)

const (
	AK         = "AK"
	SK         = "SK"
	DBs        = "DBS"
	AffectRows = "AFFECT_ROWS"
	ExecSql    = "EXEC_SQL"
	Comment    = "COMMENT"
)

type DB struct {
	Name string
	Port int32
	Host string
}

func main() {
	log.Init(&log.Config{
		Level:       "info",
		Development: false,
		MaxSize:     5,
	})
	viper.AutomaticEnv()

	ak := viper.GetString(AK)
	sk := viper.GetString(SK)
	dbs := viper.GetString(DBs)
	affectRows := viper.GetInt64(AffectRows)
	execSql := viper.GetString(ExecSql)
	comment := viper.GetString(Comment)

	config := new(openapi.Config)

	// 使用 ak 初始化 config
	config.SetAccessKeyId(ak).
		SetAccessKeySecret(sk).
		SetEndpoint("dms-enterprise.aliyuncs.com")

	// 使用 credential 初始化 config
	credentialConfig := &credential.Config{
		AccessKeyId:     config.AccessKeyId,
		AccessKeySecret: config.AccessKeySecret,
		SecurityToken:   config.SecurityToken,
		Type:            stringToP("access_key"),
	}
	cred, err := credential.NewCredential(credentialConfig)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	config.SetCredential(cred).
		SetEndpoint("dms-enterprise.aliyuncs.com")

	// 创建客户端
	client, err := dms.NewClient(config)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	dbConnectors := strings.Split(dbs, ",")
	dbMap := map[string]*DB{}
	dbItems := []*dms.CreateDataCorrectOrderRequestParamDbItemList{}
	for _, db := range dbConnectors {
		dbinfo := strings.Split(db, "@")
		if len(dbinfo) != 2 {
			log.Errorf("db connector string: %s format did not macth dbName@dbHost:dbPort", db)
			continue
		}
		dbHostInfo := strings.Split(dbinfo[1], ":")
		if len(dbHostInfo) != 2 {
			log.Errorf("db connector string: %s format did not macth dbName@dbHost:dbPort", db)
			continue
		}
		port, err := strconv.Atoi(dbHostInfo[1])
		if err != nil {
			log.Errorf("db port: %s id not a number", db)
			continue
		}
		dbMap[db] = &DB{Name: dbinfo[0], Host: dbHostInfo[0], Port: int32(port)}
	}

	for dbConnector, db := range dbMap {
		dbItem, err := searchDBs(dbConnector, db, client)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		dbItems = append(dbItems, dbItem)
	}

	createDataCorrectRequest := new(dms.CreateDataCorrectOrderRequest)
	createDataCorrectRequest.SetComment(comment)
	createDataCorrectRequest.Param = new(dms.CreateDataCorrectOrderRequestParam)
	createDataCorrectRequest.Param.SetClassify("test")
	createDataCorrectRequest.Param.SetDbItemList(dbItems)
	createDataCorrectRequest.Param.SetEstimateAffectRows(affectRows)
	createDataCorrectRequest.Param.SetExecSQL(execSql)
	createDataCorrectRequest.Param.SetSqlType("TEXT")

	orderResponse, err := client.CreateDataCorrectOrder(createDataCorrectRequest)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	orderID := int64(0)
	if len(orderResponse.Body.CreateOrderResult) > 0 {
		orderID = *orderResponse.Body.CreateOrderResult[0]
	}
	log.Infof("order has been created, id is: %d", orderID)

	getOrderBaseInfoRequest := new(dms.GetOrderBaseInfoRequest)
	getOrderBaseInfoRequest.SetOrderId(orderID)

	time.Sleep(time.Second)
	for {
		passed, err := orderPreCheckpassed(orderID, client)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		if passed {
			log.Infof("precheck all passed, continue")
			break
		}
		time.Sleep(time.Second)
	}

	submitOrderRequest := new(dms.SubmitOrderApprovalRequest).SetOrderId(orderID)
	submitOrderResponse, err := client.SubmitOrderApproval(submitOrderRequest)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	if !*submitOrderResponse.Body.Success {
		log.Errorf("submit order approval failed")
		os.Exit(1)
	}

	log.Infof("order submited, waiting for approve...")

	approved := false
	for {
		orderInfoResponse, err := client.GetOrderBaseInfo(getOrderBaseInfoRequest)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		status := *orderInfoResponse.Body.OrderBaseInfo.StatusCode
		switch status {
		case "approved":
			if !approved {
				log.Infof("order approved, waiting for execution...")
				approved = true
			}
			fallthrough
		case "new":
			fallthrough
		case "toaudit":
			fallthrough
		case "processing":
			time.Sleep(time.Second)
			continue
		case "reject":
			fallthrough
		case "closed":
			errMsg := fmt.Sprintf("order: %d was %s", orderID, status)
			log.Error(errMsg)
			os.Exit(1)
		case "success":
			log.Infof("order executed successfully")
			return
		default:
			errMsg := fmt.Sprintf("unknown status: %s,order: %d", status, orderID)
			log.Error(errMsg)
			os.Exit(1)
		}
	}
}

func orderPreCheckpassed(orderID int64, client *dms.Client) (bool, error) {
	preCheckRequest := new(dms.ListDataCorrectPreCheckSQLRequest).SetOrderId(orderID)
	pageNum := 1
	pageSize := 10
	currentpageSize := 10
	for currentpageSize >= pageSize {
		preCheckRequest.SetPageNumber(int64(pageNum)).SetPageSize(int64(pageSize))
		response, err := client.ListDataCorrectPreCheckSQL(preCheckRequest)
		if err != nil {
			return false, err
		}
		for _, preCheck := range response.Body.PreCheckSQLList {
			if *preCheck.SqlReviewStatus != "PASS" {
				return false, nil
			}
		}
		currentpageSize = len(response.Body.PreCheckSQLList)
		pageNum++
	}
	return true, nil
}

func searchDBs(dbConnector string, db *DB, client *dms.Client) (*dms.CreateDataCorrectOrderRequestParamDbItemList, error) {
	resp := new(dms.CreateDataCorrectOrderRequestParamDbItemList)
	searchRequest := new(dms.SearchDatabaseRequest).SetSearchRange("UNKNOWN").SetSearchTarget("DB").SetSearchKey(db.Name)
	pageNum := 1
	pageSize := 10
	currentpageSize := 10
	for currentpageSize >= pageSize {
		searchRequest.SetPageNumber(int32(pageNum)).SetPageSize(int32(pageSize))
		response, err := client.SearchDatabase(searchRequest)
		if err != nil {
			return resp, fmt.Errorf("search database: %s failed, error: %v", db.Name, err)
		}

		for _, dataBase := range response.Body.SearchDatabaseList.SearchDatabase {
			if *dataBase.Host == db.Host && *dataBase.Port == db.Port && *dataBase.SchemaName == db.Name {
				dbID, err := strconv.ParseInt(*dataBase.DatabaseId, 10, 64)
				if err != nil {
					return resp, fmt.Errorf("strconv database id: %s failed", *dataBase.DatabaseId)
				}
				return &dms.CreateDataCorrectOrderRequestParamDbItemList{DbId: &dbID, Logic: dataBase.Logic}, nil
			}
		}
		currentpageSize = len(response.Body.SearchDatabaseList.SearchDatabase)
		pageNum++
	}
	return resp, fmt.Errorf("no matched db found: %s", dbConnector)
}

func stringToP(in string) *string {
	return &in
}
