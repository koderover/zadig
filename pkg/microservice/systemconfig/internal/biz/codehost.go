package biz

import (
	"context"

	"go.uber.org/zap"
)

type CodeHost struct {
	ID            int    `json:"id"`
	Type          string `json:"type"`
	Address       string `json:"address"`
	IsReady       string `json:"ready"`
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refreshToken"`
	Namespace     string `json:"namespace"`
	ApplicationId string `json:"application_id"`
	Region        string `json:"region,omitempty"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	ClientSecret  string `json:"client_secret"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
	DeletedAt     int64  `json:"deleted_at"`
}

type CodeHostRepo interface {
	GetCodeHost(ctx context.Context, id int) (*CodeHost, error)
}

type CodeHostUseCase struct {
	repo CodeHostRepo
	log  *zap.SugaredLogger
}

func NewCodeHostUseCase(repo CodeHostRepo, logger *zap.SugaredLogger) *CodeHostUseCase {
	return &CodeHostUseCase{
		repo: repo,
		log:  logger,
	}
}

func (uc *CodeHostUseCase) GetCodeHost(ctx context.Context, id int) (*CodeHost, error) {
	return uc.repo.GetCodeHost(ctx, id)
}
