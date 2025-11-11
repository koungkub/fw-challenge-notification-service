package handler

type NotifyRequest struct {
	To      string `json:"to" binding:"required"`
	Title   string `json:"title" binding:"required"`
	Message string `json:"message" binding:"required"`
}
