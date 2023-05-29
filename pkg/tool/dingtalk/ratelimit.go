package dingtalk

import (
	"github.com/juju/ratelimit"
)

// DingTalk free version has a limit of 20 requests per second.
// Set the limit to 16 to be safe.
var limit = ratelimit.NewBucketWithRate(16, 1)
