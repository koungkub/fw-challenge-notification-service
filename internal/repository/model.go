package repository

import "gorm.io/gorm"

type NotificationProvider int

const (
	EmailProvider NotificationProvider = iota
	PushNotificationProvider
)

var providerName = map[NotificationProvider]string{
	EmailProvider:            "Email",
	PushNotificationProvider: "PushNotification",
}

func (x NotificationProvider) String() string {
	return providerName[x]
}

type NotificationPreference struct {
	gorm.Model

	Host         string
	ProviderName string
	SecretKey    string
}
