package meego

type GeneralWebhookRequest struct {
	Header  *WebhookHeader         `json:"header"`
	Payload *GeneralWebhookPayload `json:"payload"`
}

type WebhookHeader struct {
	Operator  string `json:"operator"`
	EventType string `json:"event_type"`
	Token     string `json:"token"`
	UUID      string `json:"uuid"`
}

type GeneralWebhookPayload struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	ProjectKey        string `json:"project_key"`
	ProjectSimpleName string `json:"project_simple_name"`
	WorkItemTypeKey   string `json:"work_item_type_key"`
}
