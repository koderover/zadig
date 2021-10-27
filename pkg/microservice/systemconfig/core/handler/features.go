package handler

import (
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/config"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

var featureM map[string]string = map[string]string{
	"ModernWorkflow": "false",
	"CommunityProjectRepository":"false",
}

var FeatureFlag string
var once sync.Once

func GetFeatures(c *gin.Context) {
	featureName := c.Query("feature")
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	initOnce()
	if v,ok := featureM[featureName];ok{
		ctx.Resp = v
		return
	}
	ctx.Resp = "notfound"
}

func initOnce(){
	once.Do(func(){
		features := config.Features()
		featureList := strings.Split(features,",")
		for _,v := range featureList{
			feature := strings.Split(v,"=")
			if len(feature) != 2 {
				continue
			}
			if _ , ok := featureM[feature[0]];ok {
				featureM[feature[0]] = feature[1]
			}
		}
	})
}
