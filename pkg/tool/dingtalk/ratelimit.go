package dingtalk

import (
	"github.com/juju/ratelimit"
)

var limit = ratelimit.NewBucketWithRate(16, 1)
