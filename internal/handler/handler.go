package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/koungkub/fw-challenge-notification-service/internal/service"
	"go.uber.org/fx"
)

var Module = fx.Module("handler",
	fx.Provide(
		NewNotificationHandler,
	),
)

const (
	RecipientTypeBuyer  = "buyer"
	RecipientTypeSeller = "seller"
)

type Notification struct {
	services service.NotificationProvider
}

type NotificationParams struct {
	fx.In

	Services service.NotificationProvider
}

func NewNotificationHandler(params NotificationParams) *Notification {
	return &Notification{
		services: params.Services,
	}
}

func (n *Notification) NotifyHandler(c *gin.Context) {
	ctx := c.Request.Context()

	var req NotifyRequest
	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, GetRequestError(err))
		return
	}

	if err := func() error {
		switch c.Param("recipient") {
		case RecipientTypeBuyer:
			return n.services.SendToBuyer(ctx, req.To, req.Title, req.Message)
		case RecipientTypeSeller:
			return n.services.SendToSeller(ctx, req.To, req.Title, req.Message)
		default:
			return errors.New("not supported recipient type")
		}
	}(); err != nil {
		c.JSON(http.StatusInternalServerError, GetInternalError(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "nofitication sent",
	})
}
