package gitee

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// EventType represents a gitee event type.
type EventType string

// List of available event types.
const (
	EventTypeMergeRequest EventType = "Merge Request Hook"
	EventTypePush         EventType = "Push Hook"
	EventTypeTagPush      EventType = "Tag Push Hook"
)

const eventTypeHeader = "X-Gitee-Event"

// HookEventType returns the event type for the given request.
func HookEventType(r *http.Request) EventType {
	return EventType(r.Header.Get(eventTypeHeader))
}

func ParseHook(eventType EventType, payload []byte) (event interface{}, err error) {
	return parseWebhook(eventType, payload)
}

func parseWebhook(eventType EventType, payload []byte) (event interface{}, err error) {
	switch eventType {
	case EventTypeMergeRequest:
		event = &PullRequestEvent{}
	case EventTypePush:
		event = &PushEvent{}
	case EventTypeTagPush:
		event = &TagPushEvent{}
	default:
		return nil, fmt.Errorf("unexpected event type: %s", eventType)
	}

	if err := json.Unmarshal(payload, event); err != nil {
		return nil, err
	}

	return event, nil
}

type TagPushEvent struct {
	Ref     string `json:"ref"`
	Before  string `json:"before"`
	After   string `json:"after"`
	Created bool   `json:"created"`
	Deleted bool   `json:"deleted"`
	Compare string `json:"compare"`
	Commits []struct {
		ID        string    `json:"id"`
		TreeID    string    `json:"tree_id"`
		ParentIds []string  `json:"parent_ids"`
		Distinct  bool      `json:"distinct"`
		Message   string    `json:"message"`
		Timestamp time.Time `json:"timestamp"`
		URL       string    `json:"url"`
		Author    struct {
			Time     time.Time `json:"time"`
			ID       int       `json:"id"`
			Name     string    `json:"name"`
			Email    string    `json:"email"`
			Username string    `json:"username"`
			UserName string    `json:"user_name"`
			URL      string    `json:"url"`
		} `json:"author"`
		Committer struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Email    string `json:"email"`
			Username string `json:"username"`
			UserName string `json:"user_name"`
			URL      string `json:"url"`
		} `json:"committer"`
		Added    interface{} `json:"added"`
		Removed  interface{} `json:"removed"`
		Modified []string    `json:"modified"`
	} `json:"commits"`
	HeadCommit struct {
		ID        string    `json:"id"`
		TreeID    string    `json:"tree_id"`
		ParentIds []string  `json:"parent_ids"`
		Distinct  bool      `json:"distinct"`
		Message   string    `json:"message"`
		Timestamp time.Time `json:"timestamp"`
		URL       string    `json:"url"`
		Author    struct {
			Time     time.Time `json:"time"`
			ID       int       `json:"id"`
			Name     string    `json:"name"`
			Email    string    `json:"email"`
			Username string    `json:"username"`
			UserName string    `json:"user_name"`
			URL      string    `json:"url"`
		} `json:"author"`
		Committer struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Email    string `json:"email"`
			Username string `json:"username"`
			UserName string `json:"user_name"`
			URL      string `json:"url"`
		} `json:"committer"`
		Added    interface{} `json:"added"`
		Removed  interface{} `json:"removed"`
		Modified []string    `json:"modified"`
	} `json:"head_commit"`
	TotalCommitsCount  int  `json:"total_commits_count"`
	CommitsMoreThanTen bool `json:"commits_more_than_ten"`
	Repository         struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Path     string `json:"path"`
		FullName string `json:"full_name"`
		Owner    struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
			HTMLURL   string `json:"html_url"`
			Type      string `json:"type"`
			SiteAdmin bool   `json:"site_admin"`
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Email     string `json:"email"`
			Username  string `json:"username"`
			UserName  string `json:"user_name"`
			URL       string `json:"url"`
		} `json:"owner"`
		Private           bool        `json:"private"`
		HTMLURL           string      `json:"html_url"`
		URL               string      `json:"url"`
		Description       string      `json:"description"`
		Fork              bool        `json:"fork"`
		CreatedAt         time.Time   `json:"created_at"`
		UpdatedAt         time.Time   `json:"updated_at"`
		PushedAt          time.Time   `json:"pushed_at"`
		GitURL            string      `json:"git_url"`
		SSHURL            string      `json:"ssh_url"`
		CloneURL          string      `json:"clone_url"`
		SvnURL            string      `json:"svn_url"`
		GitHTTPURL        string      `json:"git_http_url"`
		GitSSHURL         string      `json:"git_ssh_url"`
		GitSvnURL         string      `json:"git_svn_url"`
		Homepage          interface{} `json:"homepage"`
		StargazersCount   int         `json:"stargazers_count"`
		WatchersCount     int         `json:"watchers_count"`
		ForksCount        int         `json:"forks_count"`
		Language          string      `json:"language"`
		HasIssues         bool        `json:"has_issues"`
		HasWiki           bool        `json:"has_wiki"`
		HasPages          bool        `json:"has_pages"`
		License           interface{} `json:"license"`
		OpenIssuesCount   int         `json:"open_issues_count"`
		DefaultBranch     string      `json:"default_branch"`
		Namespace         string      `json:"namespace"`
		NameWithNamespace string      `json:"name_with_namespace"`
		PathWithNamespace string      `json:"path_with_namespace"`
	} `json:"repository"`
	Project struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Path     string `json:"path"`
		FullName string `json:"full_name"`
		Owner    struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
			HTMLURL   string `json:"html_url"`
			Type      string `json:"type"`
			SiteAdmin bool   `json:"site_admin"`
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Email     string `json:"email"`
			Username  string `json:"username"`
			UserName  string `json:"user_name"`
			URL       string `json:"url"`
		} `json:"owner"`
		Private           bool        `json:"private"`
		HTMLURL           string      `json:"html_url"`
		URL               string      `json:"url"`
		Description       string      `json:"description"`
		Fork              bool        `json:"fork"`
		CreatedAt         time.Time   `json:"created_at"`
		UpdatedAt         time.Time   `json:"updated_at"`
		PushedAt          time.Time   `json:"pushed_at"`
		GitURL            string      `json:"git_url"`
		SSHURL            string      `json:"ssh_url"`
		CloneURL          string      `json:"clone_url"`
		SvnURL            string      `json:"svn_url"`
		GitHTTPURL        string      `json:"git_http_url"`
		GitSSHURL         string      `json:"git_ssh_url"`
		GitSvnURL         string      `json:"git_svn_url"`
		Homepage          interface{} `json:"homepage"`
		StargazersCount   int         `json:"stargazers_count"`
		WatchersCount     int         `json:"watchers_count"`
		ForksCount        int         `json:"forks_count"`
		Language          string      `json:"language"`
		HasIssues         bool        `json:"has_issues"`
		HasWiki           bool        `json:"has_wiki"`
		HasPages          bool        `json:"has_pages"`
		License           interface{} `json:"license"`
		OpenIssuesCount   int         `json:"open_issues_count"`
		DefaultBranch     string      `json:"default_branch"`
		Namespace         string      `json:"namespace"`
		NameWithNamespace string      `json:"name_with_namespace"`
		PathWithNamespace string      `json:"path_with_namespace"`
	} `json:"project"`
	UserID   int    `json:"user_id"`
	UserName string `json:"user_name"`
	User     struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Username string `json:"username"`
		UserName string `json:"user_name"`
		URL      string `json:"url"`
	} `json:"user"`
	Pusher struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Username string `json:"username"`
		UserName string `json:"user_name"`
		URL      string `json:"url"`
	} `json:"pusher"`
	Sender struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
		HTMLURL   string `json:"html_url"`
		Type      string `json:"type"`
		SiteAdmin bool   `json:"site_admin"`
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		Username  string `json:"username"`
		UserName  string `json:"user_name"`
		URL       string `json:"url"`
	} `json:"sender"`
	Enterprise struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"enterprise"`
	HookName  string      `json:"hook_name"`
	HookID    int         `json:"hook_id"`
	HookURL   string      `json:"hook_url"`
	Password  string      `json:"password"`
	Timestamp interface{} `json:"timestamp"`
	Sign      string      `json:"sign"`
}

type PushEvent struct {
	Ref     string `json:"ref"`
	Before  string `json:"before"`
	After   string `json:"after"`
	Created bool   `json:"created"`
	Deleted bool   `json:"deleted"`
	Compare string `json:"compare"`
	Commits []struct {
		ID        string    `json:"id"`
		TreeID    string    `json:"tree_id"`
		ParentIds []string  `json:"parent_ids"`
		Distinct  bool      `json:"distinct"`
		Message   string    `json:"message"`
		Timestamp time.Time `json:"timestamp"`
		URL       string    `json:"url"`
		Author    struct {
			Time     time.Time `json:"time"`
			ID       int       `json:"id"`
			Name     string    `json:"name"`
			Email    string    `json:"email"`
			Username string    `json:"username"`
			UserName string    `json:"user_name"`
			URL      string    `json:"url"`
		} `json:"author"`
		Committer struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Email    string `json:"email"`
			Username string `json:"username"`
			UserName string `json:"user_name"`
			URL      string `json:"url"`
		} `json:"committer"`
		Added    []string `json:"added,omitempty"`
		Removed  []string `json:"removed,omitempty"`
		Modified []string `json:"modified,omitempty"`
	} `json:"commits"`
	TotalCommitsCount  int  `json:"total_commits_count"`
	CommitsMoreThanTen bool `json:"commits_more_than_ten"`
	Repository         struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Path     string `json:"path"`
		FullName string `json:"full_name"`
		Owner    struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
			HTMLURL   string `json:"html_url"`
			Type      string `json:"type"`
			SiteAdmin bool   `json:"site_admin"`
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Email     string `json:"email"`
			Username  string `json:"username"`
			UserName  string `json:"user_name"`
			URL       string `json:"url"`
		} `json:"owner"`
		Private           bool        `json:"private"`
		HTMLURL           string      `json:"html_url"`
		URL               string      `json:"url"`
		Description       string      `json:"description"`
		Fork              bool        `json:"fork"`
		CreatedAt         time.Time   `json:"created_at"`
		UpdatedAt         time.Time   `json:"updated_at"`
		PushedAt          time.Time   `json:"pushed_at"`
		GitURL            string      `json:"git_url"`
		SSHURL            string      `json:"ssh_url"`
		CloneURL          string      `json:"clone_url"`
		SvnURL            string      `json:"svn_url"`
		GitHTTPURL        string      `json:"git_http_url"`
		GitSSHURL         string      `json:"git_ssh_url"`
		GitSvnURL         string      `json:"git_svn_url"`
		Homepage          interface{} `json:"homepage"`
		StargazersCount   int         `json:"stargazers_count"`
		WatchersCount     int         `json:"watchers_count"`
		ForksCount        int         `json:"forks_count"`
		Language          string      `json:"language"`
		HasIssues         bool        `json:"has_issues"`
		HasWiki           bool        `json:"has_wiki"`
		HasPages          bool        `json:"has_pages"`
		License           interface{} `json:"license"`
		OpenIssuesCount   int         `json:"open_issues_count"`
		DefaultBranch     string      `json:"default_branch"`
		Namespace         string      `json:"namespace"`
		NameWithNamespace string      `json:"name_with_namespace"`
		PathWithNamespace string      `json:"path_with_namespace"`
	} `json:"repository"`
	UserID   int    `json:"user_id"`
	UserName string `json:"user_name"`
	Pusher   struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Username string `json:"username"`
		UserName string `json:"user_name"`
		URL      string `json:"url"`
	} `json:"pusher"`
	Enterprise struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"enterprise"`
	HookName  string      `json:"hook_name"`
	HookID    int         `json:"hook_id"`
	HookURL   string      `json:"hook_url"`
	Password  string      `json:"password"`
	Timestamp interface{} `json:"timestamp"`
	Sign      string      `json:"sign"`
}

type PullRequestEvent struct {
	Action      string `json:"action"`
	ActionDesc  string `json:"action_desc"`
	PullRequest *struct {
		ID             int         `json:"id"`
		Number         int         `json:"number"`
		State          string      `json:"state"`
		HTMLURL        string      `json:"html_url"`
		DiffURL        string      `json:"diff_url"`
		PatchURL       string      `json:"patch_url"`
		Title          string      `json:"title"`
		Body           interface{} `json:"body"`
		Languages      []string    `json:"languages"`
		CreatedAt      time.Time   `json:"created_at"`
		UpdatedAt      time.Time   `json:"updated_at"`
		ClosedAt       interface{} `json:"closed_at"`
		MergedAt       interface{} `json:"merged_at"`
		MergeCommitSha string      `json:"merge_commit_sha"`
		User           struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
			HTMLURL   string `json:"html_url"`
			Type      string `json:"type"`
			SiteAdmin bool   `json:"site_admin"`
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Email     string `json:"email"`
			Username  string `json:"username"`
			UserName  string `json:"user_name"`
			URL       string `json:"url"`
		} `json:"user"`
		Head *struct {
			Label string `json:"label"`
			Ref   string `json:"ref"`
			Sha   string `json:"sha"`
			User  struct {
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
				HTMLURL   string `json:"html_url"`
				Type      string `json:"type"`
				SiteAdmin bool   `json:"site_admin"`
				ID        int    `json:"id"`
				Name      string `json:"name"`
				Email     string `json:"email"`
				Username  string `json:"username"`
				UserName  string `json:"user_name"`
				URL       string `json:"url"`
			} `json:"user"`
			Repo struct {
				ID       int    `json:"id"`
				Name     string `json:"name"`
				Path     string `json:"path"`
				FullName string `json:"full_name"`
				Owner    struct {
					Login     string `json:"login"`
					AvatarURL string `json:"avatar_url"`
					HTMLURL   string `json:"html_url"`
					Type      string `json:"type"`
					SiteAdmin bool   `json:"site_admin"`
					ID        int    `json:"id"`
					Name      string `json:"name"`
					Email     string `json:"email"`
					Username  string `json:"username"`
					UserName  string `json:"user_name"`
					URL       string `json:"url"`
				} `json:"owner"`
				Private           bool        `json:"private"`
				HTMLURL           string      `json:"html_url"`
				URL               string      `json:"url"`
				Description       string      `json:"description"`
				Fork              bool        `json:"fork"`
				CreatedAt         time.Time   `json:"created_at"`
				UpdatedAt         time.Time   `json:"updated_at"`
				PushedAt          time.Time   `json:"pushed_at"`
				GitURL            string      `json:"git_url"`
				SSHURL            string      `json:"ssh_url"`
				CloneURL          string      `json:"clone_url"`
				SvnURL            string      `json:"svn_url"`
				GitHTTPURL        string      `json:"git_http_url"`
				GitSSHURL         string      `json:"git_ssh_url"`
				GitSvnURL         string      `json:"git_svn_url"`
				Homepage          interface{} `json:"homepage"`
				StargazersCount   int         `json:"stargazers_count"`
				WatchersCount     int         `json:"watchers_count"`
				ForksCount        int         `json:"forks_count"`
				Language          string      `json:"language"`
				HasIssues         bool        `json:"has_issues"`
				HasWiki           bool        `json:"has_wiki"`
				HasPages          bool        `json:"has_pages"`
				License           interface{} `json:"license"`
				OpenIssuesCount   int         `json:"open_issues_count"`
				DefaultBranch     string      `json:"default_branch"`
				Namespace         string      `json:"namespace"`
				NameWithNamespace string      `json:"name_with_namespace"`
				PathWithNamespace string      `json:"path_with_namespace"`
			} `json:"repo"`
		} `json:"head"`
		Base struct {
			Label string `json:"label"`
			Ref   string `json:"ref"`
			Sha   string `json:"sha"`
			User  struct {
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
				HTMLURL   string `json:"html_url"`
				Type      string `json:"type"`
				SiteAdmin bool   `json:"site_admin"`
				ID        int    `json:"id"`
				Name      string `json:"name"`
				Email     string `json:"email"`
				Username  string `json:"username"`
				UserName  string `json:"user_name"`
				URL       string `json:"url"`
			} `json:"user"`
			Repo struct {
				ID       int    `json:"id"`
				Name     string `json:"name"`
				Path     string `json:"path"`
				FullName string `json:"full_name"`
				Owner    struct {
					Login     string `json:"login"`
					AvatarURL string `json:"avatar_url"`
					HTMLURL   string `json:"html_url"`
					Type      string `json:"type"`
					SiteAdmin bool   `json:"site_admin"`
					ID        int    `json:"id"`
					Name      string `json:"name"`
					Email     string `json:"email"`
					Username  string `json:"username"`
					UserName  string `json:"user_name"`
					URL       string `json:"url"`
				} `json:"owner"`
				Private           bool        `json:"private"`
				HTMLURL           string      `json:"html_url"`
				URL               string      `json:"url"`
				Description       string      `json:"description"`
				Fork              bool        `json:"fork"`
				CreatedAt         time.Time   `json:"created_at"`
				UpdatedAt         time.Time   `json:"updated_at"`
				PushedAt          time.Time   `json:"pushed_at"`
				GitURL            string      `json:"git_url"`
				SSHURL            string      `json:"ssh_url"`
				CloneURL          string      `json:"clone_url"`
				SvnURL            string      `json:"svn_url"`
				GitHTTPURL        string      `json:"git_http_url"`
				GitSSHURL         string      `json:"git_ssh_url"`
				GitSvnURL         string      `json:"git_svn_url"`
				Homepage          interface{} `json:"homepage"`
				StargazersCount   int         `json:"stargazers_count"`
				WatchersCount     int         `json:"watchers_count"`
				ForksCount        int         `json:"forks_count"`
				Language          string      `json:"language"`
				HasIssues         bool        `json:"has_issues"`
				HasWiki           bool        `json:"has_wiki"`
				HasPages          bool        `json:"has_pages"`
				License           interface{} `json:"license"`
				OpenIssuesCount   int         `json:"open_issues_count"`
				DefaultBranch     string      `json:"default_branch"`
				Namespace         string      `json:"namespace"`
				NameWithNamespace string      `json:"name_with_namespace"`
				PathWithNamespace string      `json:"path_with_namespace"`
			} `json:"repo"`
		} `json:"base"`
		Merged       bool        `json:"merged"`
		Mergeable    interface{} `json:"mergeable"`
		MergeStatus  string      `json:"merge_status"`
		Comments     int         `json:"comments"`
		Commits      int         `json:"commits"`
		Additions    int         `json:"additions"`
		Deletions    int         `json:"deletions"`
		ChangedFiles int         `json:"changed_files"`
	} `json:"pull_request"`
	Number         int         `json:"number"`
	Iid            int         `json:"iid"`
	Title          string      `json:"title"`
	Body           interface{} `json:"body"`
	Languages      []string    `json:"languages"`
	State          string      `json:"state"`
	MergeStatus    string      `json:"merge_status"`
	MergeCommitSha string      `json:"merge_commit_sha"`
	URL            string      `json:"url"`
	SourceBranch   string      `json:"source_branch"`
	TargetBranch   string      `json:"target_branch"`
	Repository     struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Path     string `json:"path"`
		FullName string `json:"full_name"`
		Owner    struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
			HTMLURL   string `json:"html_url"`
			Type      string `json:"type"`
			SiteAdmin bool   `json:"site_admin"`
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Email     string `json:"email"`
			Username  string `json:"username"`
			UserName  string `json:"user_name"`
			URL       string `json:"url"`
		} `json:"owner"`
		Private           bool        `json:"private"`
		HTMLURL           string      `json:"html_url"`
		URL               string      `json:"url"`
		Description       string      `json:"description"`
		Fork              bool        `json:"fork"`
		CreatedAt         time.Time   `json:"created_at"`
		UpdatedAt         time.Time   `json:"updated_at"`
		PushedAt          time.Time   `json:"pushed_at"`
		GitURL            string      `json:"git_url"`
		SSHURL            string      `json:"ssh_url"`
		CloneURL          string      `json:"clone_url"`
		SvnURL            string      `json:"svn_url"`
		GitHTTPURL        string      `json:"git_http_url"`
		GitSSHURL         string      `json:"git_ssh_url"`
		GitSvnURL         string      `json:"git_svn_url"`
		Homepage          interface{} `json:"homepage"`
		StargazersCount   int         `json:"stargazers_count"`
		WatchersCount     int         `json:"watchers_count"`
		ForksCount        int         `json:"forks_count"`
		Language          string      `json:"language"`
		HasIssues         bool        `json:"has_issues"`
		HasWiki           bool        `json:"has_wiki"`
		HasPages          bool        `json:"has_pages"`
		License           interface{} `json:"license"`
		OpenIssuesCount   int         `json:"open_issues_count"`
		DefaultBranch     string      `json:"default_branch"`
		Namespace         string      `json:"namespace"`
		NameWithNamespace string      `json:"name_with_namespace"`
		PathWithNamespace string      `json:"path_with_namespace"`
	} `json:"repository"`
	Author struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
		HTMLURL   string `json:"html_url"`
		Type      string `json:"type"`
		SiteAdmin bool   `json:"site_admin"`
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		Username  string `json:"username"`
		UserName  string `json:"user_name"`
		URL       string `json:"url"`
	} `json:"author"`
	UpdatedBy struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
		HTMLURL   string `json:"html_url"`
		Type      string `json:"type"`
		SiteAdmin bool   `json:"site_admin"`
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		Username  string `json:"username"`
		UserName  string `json:"user_name"`
		URL       string `json:"url"`
	} `json:"updated_by"`
	Sender struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
		HTMLURL   string `json:"html_url"`
		Type      string `json:"type"`
		SiteAdmin bool   `json:"site_admin"`
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		Username  string `json:"username"`
		UserName  string `json:"user_name"`
		URL       string `json:"url"`
	} `json:"sender"`
	TargetUser struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Username string `json:"username"`
		UserName string `json:"user_name"`
		URL      string `json:"url"`
	} `json:"target_user"`
	Enterprise struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"enterprise"`
	HookName  string      `json:"hook_name"`
	HookID    int         `json:"hook_id"`
	HookURL   string      `json:"hook_url"`
	Password  string      `json:"password"`
	Timestamp interface{} `json:"timestamp"`
	Sign      string      `json:"sign"`
}
