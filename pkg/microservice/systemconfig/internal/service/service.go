package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/internal/biz"
)

type CodeHostService struct {
	cc  *biz.CodeHostUseCase
	log *zap.SugaredLogger
}

func NewCodeHostService(cc *biz.CodeHostUseCase, logger *zap.SugaredLogger) *CodeHostService {
	return &CodeHostService{
		cc:  cc,
		log: logger,
	}
}
