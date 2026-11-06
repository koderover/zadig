package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func validGit(g models.GitConfig, configuration bool) error {
	if g.CodehostID < 1 || g.Owner == "" || g.Repo == "" || g.Branch == "" || len(g.Owner) > 256 || len(g.Namespace) > 256 ||
		len(g.Repo) > 128 ||
		len(g.Branch) > 256 {
		return fmt.Errorf("必须提供有效代码源、仓库组织、仓库名和分支")
	}

	if configuration {
		return checkConfigPath(g.Path)
	}

	return nil
}

func (p *PlatformClient) configuration(ctx context.Context, auth string, g models.GitConfig) (string, error) {
	if err := validGit(g, true); err != nil {
		return "", err
	}

	if err := p.requireGitSource(ctx, auth, g.CodehostID); err != nil {
		return "", err
	}

	query := gitQuery(g)
	query.Set("path", g.Path)
	query.Set("isDir", strconv.FormatBool(g.IsDir))
	raw, err := p.business(ctx, auth, http.MethodGet, "/api/code/workspace/getcontents/"+strconv.Itoa(g.CodehostID), query, nil)
	if err != nil {
		return "", err
	}

	var text string
	if json.Unmarshal(raw, &text) != nil || strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("所选配置文件为空或无法读取")
	}

	return text, nil
}

func (p *PlatformClient) checkConfiguration(ctx context.Context, auth string, item models.ServiceInput) error {
	if item.Git != nil {
		g := *item.Git
		if item.Source == "helm_git" {
			g.Path = path.Join(g.Path, "Chart.yaml")
			g.IsDir = false
		}

		text, err := p.configuration(ctx, auth, g)
		if err != nil {
			return err
		}

		if item.Source == "helm_git" {
			var chart struct {
				Name    string `yaml:"name"`
				Version string `yaml:"version"`
			}
			if yaml.Unmarshal([]byte(text), &chart) != nil || chart.Name != item.Name || chart.Version == "" {
				return fmt.Errorf("Chart.yaml 名称必须与服务名一致，并包含有效版本")
			}
		} else {
			decoder := yaml.NewDecoder(strings.NewReader(text))
			count := 0
			for {
				var doc map[string]any
				err := decoder.Decode(&doc)
				if err == io.EOF {
					break
				}

				if err != nil {
					return fmt.Errorf("所选 YAML 配置无法解析")
				}

				if len(doc) > 0 {
					if doc["apiVersion"] == nil || doc["kind"] == nil {
						return fmt.Errorf("所选文件不是 Kubernetes 资源配置")
					}

					count++
				}
			}

			if count == 0 {
				return fmt.Errorf("没有找到可导入的 Kubernetes 配置")
			}
		}
	}

	if item.ValuesGit != nil {
		if item.ValuesGit.IsDir {
			return fmt.Errorf("values 来源必须选择明确文件")
		}

		text, err := p.configuration(ctx, auth, *item.ValuesGit)
		if err != nil {
			return err
		}

		var values map[string]any
		if yaml.Unmarshal([]byte(text), &values) != nil {
			return fmt.Errorf("values 配置必须是 YAML 对象")
		}
	}

	if item.Source == "helm_repository" {
		if item.Name != item.ChartName {
			return fmt.Errorf("Chart 仓库导入使用 Chart 名称作为服务名称")
		}

		raw, err := p.catalog(ctx, auth, models.CatalogRequest{Kind: "chart_versions", ID: item.ChartRepo})
		if err != nil {
			return err
		}

		var index struct {
			Entries map[string][]struct {
				Version string `json:"version"`
			} `json:"entries"`
		}
		if json.Unmarshal(raw, &index) != nil {
			return fmt.Errorf("Chart 索引无法解析")
		}

		found := false
		for _, entry := range index.Entries[item.ChartName] {
			if entry.Version == item.ChartVersion {
				found = true
			}
		}

		if !found {
			return fmt.Errorf("所选 Chart 版本不存在")
		}
	}

	return nil
}

func (s *AgentService) checkRepairSnapshot(ctx context.Context, e *execution, args models.RepairInput) error {
	pods, err := s.Platform.servicePods(ctx, e.Authorization, args.Target)
	if err != nil {
		return err
	}

	if pods.Truncated {
		return fmt.Errorf("服务实例超过诊断窗口，不能确认完整修复范围")
	}

	if fingerprint(common.JSON(podNames(pods))) != args.PodFingerprint {
		return fmt.Errorf("服务实例已变化，请重新生成修复清单")
	}

	if args.Action == "rollback_environment" {
		versions, err := s.Platform.versions(ctx, e.Authorization, args)
		if err != nil {
			return err
		}

		var latest int64
		for _, version := range versions {
			if version.Revision > latest {
				latest = version.Revision
			}
		}

		if latest != args.CurrentRevision {
			return fmt.Errorf("环境发布版本已变化，请重新生成回滚清单")
		}
	}

	return nil
}

func podNames(pods *servicePods) []string {
	names := make([]string, 0, len(pods.Pods))
	for _, pod := range pods.Pods {
		names = append(names, pod.Name)
	}

	sort.Strings(names)
	return names
}
