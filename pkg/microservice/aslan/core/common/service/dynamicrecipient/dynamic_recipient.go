package dynamicrecipient

import (
	"fmt"
	"regexp"
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
	dynamicRecipientKindEmail  dynamicRecipientKind = "email"
	dynamicRecipientKindMobile dynamicRecipientKind = "mobile"
)

var supportedDynamicRecipientKinds = map[setting.NotifyWebHookType]map[dynamicRecipientKind]struct{}{
	setting.NotifyWebhookTypeFeishuApp: {
		dynamicRecipientKindEmail:  {},
		dynamicRecipientKindMobile: {},
	},
	setting.NotifyWebHookTypeFeishuPerson: {
		dynamicRecipientKindEmail:  {},
		dynamicRecipientKindMobile: {},
	},
	setting.NotifyWebHookTypeFeishu: {
		dynamicRecipientKindEmail:  {},
		dynamicRecipientKindMobile: {},
	},
	setting.NotifyWebHookTypeDingDing: {
		dynamicRecipientKindEmail:  {},
		dynamicRecipientKindMobile: {},
	},
	setting.NotifyWebHookTypeMSTeam: {
		dynamicRecipientKindEmail:  {},
		dynamicRecipientKindMobile: {},
	},
	setting.NotifyWebHookTypeMail: {
		dynamicRecipientKindEmail:  {},
		dynamicRecipientKindMobile: {},
	},
}

type dynamicRecipientSpec struct {
	raw  string
	kind dynamicRecipientKind
}

var dynamicRecipientTemplateRegexp = regexp.MustCompile(`^\{\{\.[^{}]+\}\}$`)

type Resolver struct {
	keyMap map[string]string

	lookupUsersByEmail func(email string) ([]*userclient.User, error)
	lookupUsersByPhone func(phone string) ([]*userclient.User, error)

	emailUsersCache map[string][]*userclient.User
	phoneUsersCache map[string][]*userclient.User

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
		return fmt.Errorf("dynamic recipients are not supported for notification type %s", notifyType)
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
	if len(recipients) == 0 || !isLarkNotifyType(notifyType) || strings.TrimSpace(appID) != "" {
		return nil
	}

	return fmt.Errorf("app_id is required to resolve dynamic recipients for notification type %s", notifyType)
}

func NewResolver(keyMap map[string]string) *Resolver {
	return &Resolver{
		keyMap: keyMap,
		lookupUsersByEmail: func(email string) ([]*userclient.User, error) {
			return searchUsersByEmail(email)
		},
		lookupUsersByPhone: func(phone string) ([]*userclient.User, error) {
			return searchUsersByPhone(phone)
		},
		emailUsersCache:   make(map[string][]*userclient.User),
		phoneUsersCache:   make(map[string][]*userclient.User),
		larkClientCache:   make(map[string]*larktool.Client),
		larkUserIDCache:   make(map[string]string),
		larkUserMissCache: make(map[string]bool),
	}
}

func (r *Resolver) ResolveEmails(recipients []string) ([]string, error) {
	return r.resolveContacts(recipients, dynamicRecipientKindEmail)
}

func (r *Resolver) ResolveMobiles(recipients []string) ([]string, error) {
	return r.resolveContacts(recipients, dynamicRecipientKindMobile)
}

// resolveContacts resolves recipients into the contact of the given target kind.
// A recipient already matching target is collected directly; a recipient of the
// opposite kind is looked up in the user table and its target contact is collected.
func (r *Resolver) resolveContacts(recipients []string, target dynamicRecipientKind) ([]string, error) {
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
		case dynamicRecipientKindEmail, dynamicRecipientKindMobile:
			if spec.kind == target {
				resp = append(resp, value)
				continue
			}
			users, err := r.getUsersByKind(spec.kind, value)
			if err != nil {
				return nil, err
			}
			for _, user := range users {
				if contact := userContact(user, target); contact != "" {
					resp = append(resp, contact)
				}
			}
		default:
			return nil, fmt.Errorf("dynamic recipient %s cannot be resolved to %s", recipient, target)
		}
	}
	return UniqStrings(resp), nil
}

func (r *Resolver) ResolveLarkUsers(recipients []string, appID string) ([]*larktool.UserInfo, error) {
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
			return nil, fmt.Errorf("app_id is required to resolve lark dynamic recipients by email/mobile")
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
		// The direct Lark mobile lookup may miss a Zadig phone number. Re-querying
		// Lark with the user's Zadig email completes the cross-system mapping.
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

// getUsersByKind looks up users by the contact value of the given kind.
func (r *Resolver) getUsersByKind(kind dynamicRecipientKind, value string) ([]*userclient.User, error) {
	switch kind {
	case dynamicRecipientKindEmail:
		return r.getUsersByEmail(value)
	case dynamicRecipientKindMobile:
		return r.getUsersByPhone(value)
	default:
		return nil, fmt.Errorf("unsupported dynamic recipient kind %s", kind)
	}
}

// userContact returns the user's contact of the given kind, or "" if unavailable.
func userContact(user *userclient.User, kind dynamicRecipientKind) string {
	if user == nil {
		return ""
	}
	switch kind {
	case dynamicRecipientKindEmail:
		return user.Email
	case dynamicRecipientKindMobile:
		return user.Phone
	default:
		return ""
	}
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
	if !dynamicRecipientTemplateRegexp.MatchString(input) {
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
	default:
		return nil, fmt.Errorf("dynamic recipient %s is not supported, only email/mobile(phone) are allowed", input)
	}

	return &dynamicRecipientSpec{
		raw:  input,
		kind: kind,
	}, nil
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

func isLarkNotifyType(notifyType setting.NotifyWebHookType) bool {
	return notifyType == setting.NotifyWebHookTypeFeishu ||
		notifyType == setting.NotifyWebhookTypeFeishuApp ||
		notifyType == setting.NotifyWebHookTypeFeishuPerson
}
