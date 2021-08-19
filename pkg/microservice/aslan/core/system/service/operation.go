package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
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

func FindOperation(args *OperationLogArgs, log *zap.SugaredLogger) ([]*models.OperationLog, int, error) {
	resp, count, err := mongodb.NewOperationLogColl().Find(&mongodb.OperationLogArgs{
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

func InsertOperation(args *models.OperationLog, log *zap.SugaredLogger) (string, error) {
	err := mongodb.NewOperationLogColl().Insert(args)
	if err != nil {
		log.Errorf("insert operation log error: %v", err)
		return "", e.ErrCreateOperationLog
	}
	return args.ID.Hex(), nil
}

func UpdateOperation(id string, status int, log *zap.SugaredLogger) error {
	err := mongodb.NewOperationLogColl().Update(id, status)
	if err != nil {
		log.Errorf("update operation log error: %v", err)
		return e.ErrUpdateOperationLog
	}
	return nil
}
