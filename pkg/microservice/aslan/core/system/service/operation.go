package service

import (
	"go.uber.org/zap"

	models2 "github.com/koderover/zadig/pkg/microservice/aslan/core/system/repository/models"
	mongodb2 "github.com/koderover/zadig/pkg/microservice/aslan/core/system/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type OperationLogArgs struct {
	Username    string `json:"username"`
	ProductName string `json:"product_name"`
	Function    string `json:"function"`
	Status      int    `json:"status"`
	PerPage     int    `json:"per_page"`
	Page        int    `json:"page"`
}

func FindOperation(args *OperationLogArgs, log *zap.SugaredLogger) ([]*models2.OperationLog, int, error) {
	resp, count, err := mongodb2.NewOperationLogColl().Find(&mongodb2.OperationLogArgs{
		Username:    args.Username,
		ProductName: args.ProductName,
		Function:    args.Function,
		Status:      args.Status,
		PerPage:     args.PerPage,
		Page:        args.Page,
	})
	if err != nil {
		log.Errorf("find operation log error: %v", err)
		return resp, count, e.ErrFindOperationLog
	}
	return resp, count, err
}

func InsertOperation(args *models2.OperationLog, log *zap.SugaredLogger) (string, error) {
	err := mongodb2.NewOperationLogColl().Insert(args)
	if err != nil {
		log.Errorf("insert operation log error: %v", err)
		return "", e.ErrCreateOperationLog
	}
	return args.ID.Hex(), nil
}

func UpdateOperation(id string, status int, log *zap.SugaredLogger) error {
	err := mongodb2.NewOperationLogColl().Update(id, status)
	if err != nil {
		log.Errorf("update operation log error: %v", err)
		return e.ErrUpdateOperationLog
	}
	return nil
}
