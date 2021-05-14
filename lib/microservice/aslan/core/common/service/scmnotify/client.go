/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scmnotify

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/xanzy/go-gitlab"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/codehost"
	"github.com/koderover/zadig/lib/tool/gerrit"
	gitlabtool "github.com/koderover/zadig/lib/tool/gitlab"
	"github.com/koderover/zadig/lib/tool/xlog"
)

type Client struct {
	logger *xlog.Logger
}

func NewClient() *Client {
	return &Client{logger: xlog.NewDummy()}
}

// Comment send comment to gitlab and set comment id in notify
func (c *Client) Comment(notify *models.Notification) error {
	if notify.PrId == 0 {
		return fmt.Errorf("non pr notification not supported yet")
	}

	var err error
	comment := notify.ErrInfo
	if comment == "" {
		if comment, err = notify.CreateCommentBody(); err != nil {
			return fmt.Errorf("failed to create comment body %v", err)
		}
	}

	codeHostDetail, err := codehost.GetCodeHostInfoByID(notify.CodehostId)
	if err != nil {
		return errors.Wrapf(err, "codehost %d not found to comment", notify.CodehostId)
	}
	if strings.ToLower(codeHostDetail.Type) == "gitlab" {
		var note *gitlab.Note
		cli, err := gitlabtool.NewGitlabClient(codeHostDetail.Address, codeHostDetail.AccessToken)
		if notify.CommentId == "" {
			// create comment
			note, _, err = cli.Notes.CreateMergeRequestNote(
				notify.ProjectId, notify.PrId, &gitlab.CreateMergeRequestNoteOptions{
					Body: &comment,
				},
			)

			if err == nil {
				notify.CommentId = strconv.Itoa(note.ID)
			}
		} else {
			// update comment
			noteID, _ := strconv.Atoi(notify.CommentId)
			note, _, err = cli.Notes.UpdateMergeRequestNote(
				notify.ProjectId, notify.PrId, noteID, &gitlab.UpdateMergeRequestNoteOptions{
					Body: &comment,
				})
		}

		if err != nil {
			return fmt.Errorf("failed to comment gitlab due to %s/%d %v", notify.ProjectId, notify.PrId, err)
		}
	} else if strings.ToLower(codeHostDetail.Type) == gerrit.CodehostTypeGerrit {
		cli := gerrit.NewClient(codeHostDetail.Address, codeHostDetail.AccessToken)
		for _, task := range notify.Tasks {
			// create task created comment
			if !task.FirstCommented && task.Status == config.TaskStatusReady {
				if e := cli.SetReview(
					notify.ProjectId,
					notify.PrId,
					fmt.Sprintf(""+
						"%s ⏱️ %s/v1/projects/detail/%s/pipelines/multi/%s/%d",
						strings.ToUpper(string(task.Status)),
						notify.BaseUri,
						task.ProductName,
						task.WorkflowName,
						task.ID,
					),
					notify.Label,
					"0",
					notify.Revision,
				); e != nil {
					c.logger.Warnf("failed to set review %v %v %s %v", task, notify, e)
				}

				task.FirstCommented = true
				continue
			}

			/* set review score*/
			var emoji, score string
			var skip bool
			switch task.Status {
			case config.TaskStatusPass:
				emoji = "✅"
				score = "+1"
			case config.TaskStatusCancelled:
				emoji = "✖️"
				score = "0"
			case config.TaskStatusTimeout, config.TaskStatusFailed:
				emoji = "❌"
				score = "-1"
			default:
				skip = true
			}

			if !skip {
				if e := cli.SetReview(
					notify.ProjectId,
					notify.PrId,
					fmt.Sprintf(""+
						"%s %s %s/v1/projects/detail/%s/pipelines/multi/%s/%d",
						strings.ToUpper(string(task.Status)),
						emoji,
						notify.BaseUri,
						task.ProductName,
						task.WorkflowName,
						task.ID,
					),
					notify.Label,
					score,
					notify.Revision,
				); e != nil {
					c.logger.Warnf("failed to set review %v %v %s %v", task, notify, e)
				}
			}
		}
	} else {
		return fmt.Errorf("non gitlab source not supported to comment")
	}

	return nil
}
