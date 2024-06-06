package workwx

func Validate(host, corpID string, agentID int, agentSecret string) error {
	client := NewClient(host, corpID, agentID, agentSecret)
	_, err := client.getAccessToken()
	return err
}
