package util

import (
	"fmt"
	"os"
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/git"
	githubservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/github"
	gitlabservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/gitlab"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type YAMLLoader interface {
	GetYAMLContents(owner, repo, path, branch string, isDir, split bool) ([]string, error)
	GetLatestRepositoryCommit(owner, repo, path, branch string) (*git.RepositoryCommit, error)
	GetTree(owner, repo, path, branch string) ([]*git.TreeNode, error)
}

func GetYAMLLoader(ch *systemconfig.CodeHost) (YAMLLoader, error) {
	switch ch.Type {
	case setting.SourceFromGithub:
		return githubservice.NewClient(ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy), nil
	case setting.SourceFromGitlab:
		return gitlabservice.NewClient(ch.ID, ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy, ch.DisableSSL)
	default:
		// should not have happened here
		log.DPanicf("invalid source: %s", ch.Type)
		return nil, fmt.Errorf("invalid source: %s", ch.Type)
	}
}

func GetFoldersAndYAMLFiles(treeNodes []*git.TreeNode) ([]*git.TreeNode, []*git.TreeNode) {
	var folders, files []*git.TreeNode
	for _, tn := range treeNodes {
		if tn.IsDir {
			folders = append(folders, tn)
		} else if IsYaml(tn.Name) {
			files = append(files, tn)
		}
	}

	return folders, files
}

func IsYaml(filename string) bool {
	filename = strings.ToLower(filename)
	return strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml")
}

func HasYAMLFiles(treeNodes []*git.TreeNode) bool {
	for _, tn := range treeNodes {
		if !tn.IsDir && IsYaml(tn.Name) {
			return true
		}
	}

	return false
}

func IsValidServiceDir(child []os.FileInfo) bool {
	for _, file := range child {
		if !file.IsDir() && IsYaml(file.Name()) {
			return true
		}
	}
	return false
}
