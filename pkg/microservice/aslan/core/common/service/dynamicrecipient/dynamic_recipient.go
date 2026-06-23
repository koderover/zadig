package dynamicrecipient

import (
	"fmt"
	"strings"

	"github.com/samber/lo"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	larkservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/v2/pkg/setting"
	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
	larktool "github.com/koderover/zadig/v2/pkg/tool/lark"
	util2 "github.com/koderover/zadig/v2/pkg/util"
)

type dynamicRecipientKind string

const (
	dynamicRecipientKindEmail   dynamicRecipientKind = "email"
	dynamicRecipientKindMobile  dynamicRecipientKind = "mobile"
	dynamicRecipientKindAccount dynamicRecipientKind = "account"
	dynamicRecipientKindUserID  dynamicRecipientKind = "user_id"
	dynamicRecipientKindOpenID  dynamicRecipientKind = "open_id"

	searchAllIdentityType = "*"
)

var supportedDynamicRecipientKinds = map[setting.NotifyWebHookType]map[dynamicRecipientKind]struct{}{
	setting.NotifyWebhookTypeFeishuApp: {
		dynamicRecipientKindEmail:   {},
		dynamicRecipientKindMobile:  {},
		dynamicRecipientKindAccount: {},
		dynamicRecipientKindUserID:  {},
	},
	setting.NotifyWebHookTypeFeishuPerson: {
		dynamicRecipientKindEmail:   {},
		dynamicRecipientKindMobile:  {},
		dynamicRecipientKindAccount: {},
		dynamicRecipientKindUserID:  {},
		dynamicRecipientKindOpenID:  {},
	},
	setting.NotifyWebHookTypeFeishu: {
		dynamicRecipientKindEmail:   {},
		dynamicRecipientKindMobile:  {},
		dynamicRecipientKindAccount: {},
		dynamicRecipientKindUserID:  {},
	},
	setting.NotifyWebHookTypeWechatWork: {
		dynamicRecipientKindUserID: {},
	},
	setting.NotifyWebHookTypeDingDing: {
		dynamicRecipientKindEmail:   {},
		dynamicRecipientKindMobile:  {},
		dynamicRecipientKindAccount: {},
	},
	setting.NotifyWebHookTypeMSTeam: {
		dynamicRecipientKindEmail:   {},
		dynamicRecipientKindMobile:  {},
		dynamicRecipientKindAccount: {},
	},
	setting.NotifyWebHookTypeMail: {
		dynamicRecipientKindEmail:   {},
		dynamicRecipientKindMobile:  {},
		dynamicRecipientKindAccount: {},
	},
}

type dynamicRecipientSpec struct {
	raw  string
	key  string
	kind dynamicRecipientKind
}

type Resolver struct {
	keyMap map[string]string

	lookupUsersByAccount func(account string) ([]*userclient.User, error)
	lookupUsersByEmail   func(email string) ([]*userclient.User, error)
	lookupUsersByPhone   func(phone string) ([]*userclient.User, error)

	accountUsersCache map[string][]*userclient.User
	emailUsersCache   map[string][]*userclient.User
	phoneUsersCache   map[string][]*userclient.User

	larkClientCache   map[string]*larktool.Client
	larkUserIDCache   map[string]string
	larkUserMissCache map[string]bool
}

func ValidateDynamicRecipientsForNotifyType(notifyType setting.NotifyWebHookType, recipients []string) error {
	if len(recipients) == 0 {
		return nil
	}

	supportedKinds, ok := supportedDynamicRecipientKinds[notifyType]
	if !ok {
		return nil
	}

	for _, recipient := range recipients {
		spec, err := parseDynamicRecipient(recipient)
		if err != nil {
			return err
		}
		if _, ok := supportedKinds[spec.kind]; !ok {
			return fmt.Errorf("dynamic recipient %s is not supported for notification type %s", recipient, notifyType)
		}
	}

	return nil
}

func ValidateDynamicRecipientsForNotifyConfig(notifyType setting.NotifyWebHookType, appID string, recipients []string) error {
	if err := ValidateDynamicRecipientsForNotifyType(notifyType, recipients); err != nil {
		return err
	}
	if notifyType != setting.NotifyWebHookTypeFeishu || strings.TrimSpace(appID) != "" {
		return nil
	}

	for _, recipient := range recipients {
		spec, err := parseDynamicRecipient(recipient)
		if err != nil {
			return err
		}
		if spec.kind != dynamicRecipientKindUserID {
			return fmt.Errorf("dynamic recipient %s requires app_id for notification type %s", recipient, notifyType)
		}
	}

	return nil
}

func NewResolver(keyMap map[string]string) *Resolver {
	return &Resolver{
		keyMap: keyMap,
		lookupUsersByAccount: func(account string) ([]*userclient.User, error) {
			return searchUsersByAccount(account)
		},
		lookupUsersByEmail: func(email string) ([]*userclient.User, error) {
			return searchUsersByEmail(email)
		},
		lookupUsersByPhone: func(phone string) ([]*userclient.User, error) {
			return searchUsersByPhone(phone)
		},
		accountUsersCache: make(map[string][]*userclient.User),
		emailUsersCache:   make(map[string][]*userclient.User),
		phoneUsersCache:   make(map[string][]*userclient.User),
		larkClientCache:   make(map[string]*larktool.Client),
		larkUserIDCache:   make(map[string]string),
		larkUserMissCache: make(map[string]bool),
	}
}

func (r *Resolver) ResolveEmails(recipients []string) ([]string, error) {
	resp := make([]string, 0)
	for _, recipient := range recipients {
		spec, value, ok, err := r.resolveRecipient(recipient)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		switch spec.kind {
		case dynamicRecipientKindEmail:
			resp = append(resp, value)
		case dynamicRecipientKindMobile:
			users, err := r.getUsersByPhone(value)
			if err != nil {
				return nil, err
			}
			for _, user := range users {
				if user != nil && user.Email != "" {
					resp = append(resp, user.Email)
				}
			}
		case dynamicRecipientKindAccount:
			users, err := r.getUsersByAccount(value)
			if err != nil {
				return nil, err
			}
			for _, user := range users {
				if user != nil && user.Email != "" {
					resp = append(resp, user.Email)
				}
			}
		default:
			return nil, fmt.Errorf("dynamic recipient %s cannot be resolved to email", recipient)
		}
	}
	return UniqStrings(resp), nil
}

func (r *Resolver) ResolveMobiles(recipients []string) ([]string, error) {
	resp := make([]string, 0)
	for _, recipient := range recipients {
		spec, value, ok, err := r.resolveRecipient(recipient)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		switch spec.kind {
		case dynamicRecipientKindMobile:
			resp = append(resp, value)
		case dynamicRecipientKindEmail:
			users, err := r.getUsersByEmail(value)
			if err != nil {
				return nil, err
			}
			for _, user := range users {
				if user != nil && user.Phone != "" {
					resp = append(resp, user.Phone)
				}
			}
		case dynamicRecipientKindAccount:
			users, err := r.getUsersByAccount(value)
			if err != nil {
				return nil, err
			}
			for _, user := range users {
				if user != nil && user.Phone != "" {
					resp = append(resp, user.Phone)
				}
			}
		default:
			return nil, fmt.Errorf("dynamic recipient %s cannot be resolved to mobile", recipient)
		}
	}
	return UniqStrings(resp), nil
}

func (r *Resolver) ResolveDirectValues(recipients []string, supportedKinds ...dynamicRecipientKind) ([]string, error) {
	supported := make(map[dynamicRecipientKind]struct{}, len(supportedKinds))
	for _, kind := range supportedKinds {
		supported[kind] = struct{}{}
	}

	resp := make([]string, 0)
	for _, recipient := range recipients {
		spec, value, ok, err := r.resolveRecipient(recipient)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if _, ok := supported[spec.kind]; !ok {
			return nil, fmt.Errorf("dynamic recipient %s is not supported in this notification channel", recipient)
		}
		resp = append(resp, value)
	}
	return UniqStrings(resp), nil
}

func (r *Resolver) ResolveUserIDs(recipients []string) ([]string, error) {
	return r.ResolveDirectValues(recipients, dynamicRecipientKindUserID)
}

func (r *Resolver) ResolveLarkUsers(recipients []string, appID string, allowOpenID bool) ([]*larktool.UserInfo, error) {
	if len(recipients) == 0 {
		return nil, nil
	}

	resp := make([]*larktool.UserInfo, 0)
	var client *larktool.Client
	getClient := func() (*larktool.Client, error) {
		if client != nil {
			return client, nil
		}
		if strings.TrimSpace(appID) == "" {
			return nil, fmt.Errorf("app_id is required to resolve lark dynamic recipients by email/mobile/account")
		}
		var err error
		client, err = r.getLarkClient(appID)
		if err != nil {
			return nil, err
		}
		return client, nil
	}

	for _, recipient := range recipients {
		spec, value, ok, err := r.resolveRecipient(recipient)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		switch spec.kind {
		case dynamicRecipientKindUserID:
			resp = append(resp, &larktool.UserInfo{ID: value, IDType: setting.LarkUserID})
		case dynamicRecipientKindOpenID:
			if !allowOpenID {
				return nil, fmt.Errorf("dynamic recipient %s is not supported in this notification channel", recipient)
			}
			resp = append(resp, &larktool.UserInfo{ID: value, IDType: setting.LarkUserOpenID})
		case dynamicRecipientKindEmail:
			client, err := getClient()
			if err != nil {
				return nil, err
			}
			ids, err := r.resolveLarkUserIDsByEmail(client, appID, value)
			if err != nil {
				return nil, err
			}
			for _, id := range ids {
				resp = append(resp, &larktool.UserInfo{ID: id, IDType: setting.LarkUserID})
			}
		case dynamicRecipientKindMobile:
			client, err := getClient()
			if err != nil {
				return nil, err
			}
			ids, err := r.resolveLarkUserIDsByPhone(client, appID, value)
			if err != nil {
				return nil, err
			}
			for _, id := range ids {
				resp = append(resp, &larktool.UserInfo{ID: id, IDType: setting.LarkUserID})
			}
		case dynamicRecipientKindAccount:
			client, err := getClient()
			if err != nil {
				return nil, err
			}
			ids, err := r.resolveLarkUserIDsByAccount(client, appID, value)
			if err != nil {
				return nil, err
			}
			for _, id := range ids {
				resp = append(resp, &larktool.UserInfo{ID: id, IDType: setting.LarkUserID})
			}
		default:
			return nil, fmt.Errorf("dynamic recipient %s cannot be resolved to lark user", recipient)
		}
	}

	return UniqLarkUsers(resp), nil
}

func (r *Resolver) resolveLarkUserIDsByEmail(client *larktool.Client, appID, email string) ([]string, error) {
	if id, found, err := r.lookupLarkUserID(client, appID, larktool.QueryTypeEmail, email); err != nil {
		return nil, err
	} else if found {
		return []string{id}, nil
	}

	users, err := r.getUsersByEmail(email)
	if err != nil {
		return nil, err
	}

	resp := make([]string, 0)
	for _, user := range users {
		if user == nil || user.Phone == "" {
			continue
		}
		id, found, err := r.lookupLarkUserID(client, appID, larktool.QueryTypeMobile, user.Phone)
		if err != nil {
			return nil, err
		}
		if found {
			resp = append(resp, id)
		}
	}
	return UniqStrings(resp), nil
}

func (r *Resolver) resolveLarkUserIDsByPhone(client *larktool.Client, appID, phone string) ([]string, error) {
	if id, found, err := r.lookupLarkUserID(client, appID, larktool.QueryTypeMobile, phone); err != nil {
		return nil, err
	} else if found {
		return []string{id}, nil
	}

	users, err := r.getUsersByPhone(phone)
	if err != nil {
		return nil, err
	}

	resp := make([]string, 0)
	for _, user := range users {
		if user == nil || user.Email == "" {
			continue
		}
		id, found, err := r.lookupLarkUserID(client, appID, larktool.QueryTypeEmail, user.Email)
		if err != nil {
			return nil, err
		}
		if found {
			resp = append(resp, id)
		}
	}
	return UniqStrings(resp), nil
}

func (r *Resolver) resolveLarkUserIDsByAccount(client *larktool.Client, appID, account string) ([]string, error) {
	users, err := r.getUsersByAccount(account)
	if err != nil {
		return nil, err
	}

	resp := make([]string, 0)
	for _, user := range users {
		if user == nil {
			continue
		}
		if user.Email != "" {
			id, found, err := r.lookupLarkUserID(client, appID, larktool.QueryTypeEmail, user.Email)
			if err != nil {
				return nil, err
			}
			if found {
				resp = append(resp, id)
				continue
			}
		}
		if user.Phone != "" {
			id, found, err := r.lookupLarkUserID(client, appID, larktool.QueryTypeMobile, user.Phone)
			if err != nil {
				return nil, err
			}
			if found {
				resp = append(resp, id)
			}
		}
	}
	return UniqStrings(resp), nil
}

func (r *Resolver) lookupLarkUserID(client *larktool.Client, appID, queryType, value string) (string, bool, error) {
	cacheKey := strings.Join([]string{appID, queryType, value}, ":")
	if cached, ok := r.larkUserIDCache[cacheKey]; ok {
		return cached, true, nil
	}
	if r.larkUserMissCache[cacheKey] {
		return "", false, nil
	}

	userInfo, err := client.GetUserIDByEmailOrMobile(queryType, value, setting.LarkUserID)
	if err != nil {
		if isLarkUserNotFoundErr(err) {
			r.larkUserMissCache[cacheKey] = true
			return "", false, nil
		}
		return "", false, err
	}

	userID := util2.GetStringFromPointer(userInfo.UserId)
	if userID == "" {
		r.larkUserMissCache[cacheKey] = true
		return "", false, nil
	}

	r.larkUserIDCache[cacheKey] = userID
	return userID, true, nil
}

func (r *Resolver) getUsersByAccount(account string) ([]*userclient.User, error) {
	if users, ok := r.accountUsersCache[account]; ok {
		return users, nil
	}
	users, err := r.lookupUsersByAccount(account)
	if err != nil {
		return nil, err
	}
	r.accountUsersCache[account] = users
	return users, nil
}

func (r *Resolver) getUsersByEmail(email string) ([]*userclient.User, error) {
	if users, ok := r.emailUsersCache[email]; ok {
		return users, nil
	}
	users, err := r.lookupUsersByEmail(email)
	if err != nil {
		return nil, err
	}
	r.emailUsersCache[email] = users
	return users, nil
}

func (r *Resolver) getUsersByPhone(phone string) ([]*userclient.User, error) {
	if users, ok := r.phoneUsersCache[phone]; ok {
		return users, nil
	}
	users, err := r.lookupUsersByPhone(phone)
	if err != nil {
		return nil, err
	}
	r.phoneUsersCache[phone] = users
	return users, nil
}

func (r *Resolver) getLarkClient(appID string) (*larktool.Client, error) {
	if client, ok := r.larkClientCache[appID]; ok {
		return client, nil
	}

	client, err := larkservice.GetLarkClientByIMAppID(appID)
	if err != nil {
		return nil, err
	}

	r.larkClientCache[appID] = client
	return client, nil
}

func (r *Resolver) resolveRecipient(raw string) (*dynamicRecipientSpec, string, bool, error) {
	spec, err := parseDynamicRecipient(raw)
	if err != nil {
		return nil, "", false, err
	}

	value := strings.TrimSpace(renderNotificationString(spec.raw, r.keyMap))
	if value == "" || strings.Contains(value, "{{.") {
		return spec, "", false, nil
	}

	return spec, value, true, nil
}

func BuildMailUsersFromEmails(emails []string) []*commonmodels.User {
	resp := make([]*commonmodels.User, 0, len(emails))
	for _, email := range lo.Uniq(emails) {
		if email == "" {
			continue
		}
		resp = append(resp, &commonmodels.User{
			Type:     "email",
			UserName: email,
		})
	}
	return resp
}

func UniqMailUsers(users []*commonmodels.User) []*commonmodels.User {
	seen := make(map[string]struct{})
	resp := make([]*commonmodels.User, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		key := user.Type + ":"
		switch user.Type {
		case "email":
			key += user.UserName
		case setting.UserTypeGroup:
			key += user.GroupID
		default:
			key += user.UserID
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		resp = append(resp, user)
	}
	return resp
}

func UniqLarkUsers(users []*larktool.UserInfo) []*larktool.UserInfo {
	seen := make(map[string]struct{})
	resp := make([]*larktool.UserInfo, 0, len(users))
	for _, user := range users {
		if user == nil || user.ID == "" {
			continue
		}
		key := user.IDType + ":" + user.ID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		resp = append(resp, user)
	}
	return resp
}

func renderNotificationString(input string, keyMap map[string]string) string {
	if len(keyMap) == 0 || !strings.Contains(input, "{{.") {
		return input
	}
	pairs := make([]string, 0, len(keyMap)*2)
	for key, value := range keyMap {
		pairs = append(pairs, "{{."+key+"}}", value)
	}
	return strings.NewReplacer(pairs...).Replace(input)
}

func parseDynamicRecipient(input string) (*dynamicRecipientSpec, error) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "{{.") || !strings.HasSuffix(input, "}}") {
		return nil, fmt.Errorf("dynamic recipient must be a single template variable, got %s", input)
	}

	key := strings.TrimSuffix(strings.TrimPrefix(input, "{{."), "}}")
	if key == "" {
		return nil, fmt.Errorf("dynamic recipient %s is invalid", input)
	}

	parts := strings.Split(strings.ToLower(key), ".")
	suffix := parts[len(parts)-1]

	var kind dynamicRecipientKind
	switch suffix {
	case "email":
		kind = dynamicRecipientKindEmail
	case "mobile", "phone":
		kind = dynamicRecipientKindMobile
	case "account":
		kind = dynamicRecipientKindAccount
	case "user_id", "userid":
		kind = dynamicRecipientKindUserID
	case "open_id":
		kind = dynamicRecipientKindOpenID
	default:
		return nil, fmt.Errorf("dynamic recipient %s is not supported, only email/mobile(phone)/account/user_id(userid)/open_id are allowed", input)
	}

	return &dynamicRecipientSpec{
		raw:  input,
		key:  key,
		kind: kind,
	}, nil
}

func searchUsersByAccount(account string) ([]*userclient.User, error) {
	resp, err := userclient.New().SearchUser(&userclient.SearchUserArgs{
		Account:      account,
		IdentityType: searchAllIdentityType,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	return resp.Users, nil
}

func searchUsersByEmail(email string) ([]*userclient.User, error) {
	resp, err := userclient.New().SearchUser(&userclient.SearchUserArgs{
		Email: email,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	return resp.Users, nil
}

func searchUsersByPhone(phone string) ([]*userclient.User, error) {
	resp, err := userclient.New().SearchUser(&userclient.SearchUserArgs{
		Phone: phone,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	return resp.Users, nil
}

func UniqStrings(items []string) []string {
	items = lo.Filter(items, func(item string, _ int) bool {
		return item != ""
	})
	return lo.Uniq(items)
}

func isLarkUserNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "user not found")
}
