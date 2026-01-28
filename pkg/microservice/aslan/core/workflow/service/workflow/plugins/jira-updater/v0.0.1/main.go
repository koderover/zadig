package main

import (
	"os"

	"github.com/andygrunwald/go-jira"
	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/spf13/viper"

	"github.com/koderover/zadig/v2/pkg/tool/log"
)

// envs
const (
	JiraAddress  = "JIRA_ADDRESS"
	Username     = "USERNAME"
	Password     = "PASSWORD"
	IssueID      = "ISSUE_ID"
	TargetStatus = "TARGET_STATUS"
	Script       = "SCRIPT"
)

func main() {
	log.Init(&log.Config{
		Level:       config.LogLevel(),
		Development: false,
		MaxSize:     5,
	})
	viper.AutomaticEnv()

	addr := viper.GetString(JiraAddress)
	issueID := viper.GetString(IssueID)
	username := viper.GetString(Username)
	password := viper.GetString(Password)
	status := viper.GetString(TargetStatus)
	script := viper.GetString(Script)

	log.Infof("script: %s", script)
	log.Infof("executing jira status update to %s for issue: %s on server %s", status, issueID, addr)

	tp := jira.BasicAuthTransport{
		Username: username,
		Password: password,
	}

	jiraclient, err := jira.NewClient(tp.Client(), addr)
	if err != nil {
		log.Infof("failed to create JIRA client, error: %s\n", err)
		os.Exit(1)
	}

	var transitionID string

	possibleTransitions, _, err := jiraclient.Issue.GetTransitions(issueID)
	if err != nil {
		log.Infof("failed to get possible transitions, err: %s\n", err)
		os.Exit(1)
	}

	for _, possibleTransition := range possibleTransitions {
		if possibleTransition.Name == status {
			transitionID = possibleTransition.ID
			break
		}
	}

	if transitionID == "" {
		log.Infof("no transition of name %s found, check if the target status exist\n", status)
		os.Exit(1)
	}

	_, err = jiraclient.Issue.DoTransition(issueID, transitionID)
	if err != nil {
		log.Infof("failed to do change status, err: %s\n", err)
		os.Exit(1)
	}

	log.Infof("Jira status update complete")
}
