package service

import (
	"context"
)

func (c *CodeHostService) GetCodeHost(ctx context.Context, id int) (interface{}, error) {
	return c.cc.GetCodeHost(ctx, id)
}
