package client

type NotificationRequest struct {
	To        string `json:"to"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	SecretKey string `json:"secret_key"`
}
