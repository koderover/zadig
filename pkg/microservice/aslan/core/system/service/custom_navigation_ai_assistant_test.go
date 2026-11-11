package service

import (
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func TestDefaultNavigationIncludesAIReportsInDataView(t *testing.T) {
	items := getDefaultSystemNavigation()
	if err := (&commonmodels.CustomNavigation{Items: items}).Validate(); err != nil {
		t.Fatal(err)
	}
	reports := map[config.NavigationItemKey]struct{ name, english string }{
		config.NavigationKeyAIInsights: {"AI 效能报告", "AI Insights"},
	}
	counts := make(map[config.NavigationItemKey]int)
	for _, folder := range items {
		if folder.Key != "dataView" {
			continue
		}
		for _, item := range folder.Children {
			if item.Key == config.NavigationKeyAIReviewInsight || item.Key == config.NavigationKeyAIAssistantReport {
				t.Fatalf("默认导航不应包含旧 AI 报告入口: %s", item.Key)
			}
			if expected, ok := reports[item.Key]; ok {
				counts[item.Key]++
				if item.Name != expected.name || item.EnName != expected.english || item.URL != "insight/ai" || item.IconType != "class" || item.Icon != "iconfont iconai-trend" || item.Type != config.NavigationItemTypePage || item.PageType != config.NavigationPageTypeSystem {
					t.Fatalf("AI 报告必须提供完整的系统页面导航信息: %+v", item)
				}
			}
		}
	}
	for key := range reports {
		if counts[key] != 1 {
			t.Fatalf("数据视图应恰好包含一个 %s 入口，实际 %d", key, counts[key])
		}
	}
}
