package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kelvins-io/12306cn/backend/internal/middleware"
	"github.com/kelvins-io/12306cn/backend/internal/models"
	"github.com/kelvins-io/12306cn/backend/internal/pkg/response"
	"github.com/kelvins-io/12306cn/backend/internal/services"
)

type Handler struct {
	Svc *services.Service
}

func New(svc *services.Service) *Handler {
	return &Handler{Svc: svc}
}

func (h *Handler) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Nickname string `json:"nickname"`
		Phone    string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	user, token, err := h.Svc.Register(req.Username, req.Password, req.Nickname, req.Phone)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"token": token, "user": user})
}

func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Username    string `json:"username" binding:"required"`
		Password    string `json:"password" binding:"required"`
		CaptchaID   string `json:"captcha_id" binding:"required"`
		CaptchaCode string `json:"captcha_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if !h.Svc.Captcha.Verify(c.Request.Context(), req.CaptchaID, req.CaptchaCode) {
		response.BadRequest(c, "验证码错误或已过期")
		return
	}
	user, token, err := h.Svc.Login(req.Username, req.Password)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}
	response.OK(c, gin.H{"token": token, "user": user})
}

func (h *Handler) IssueCaptcha(c *gin.Context) {
	ch, err := h.Svc.Captcha.Issue(c.Request.Context())
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, ch)
}

func (h *Handler) Me(c *gin.Context) {
	uid := middleware.GetUserID(c)
	var user models.User
	if err := h.Svc.DB.First(&user, uid).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	response.OK(c, user)
}

func (h *Handler) ListStations(c *gin.Context) {
	list, err := h.Svc.ListStations(c.Query("q"))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *Handler) QueryTickets(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")
	date := c.Query("date")
	if from == "" || to == "" || date == "" {
		response.BadRequest(c, "from/to/date 必填")
		return
	}
	list, err := h.Svc.QueryTickets(c.Request.Context(), from, to, date)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *Handler) QueryTransfers(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")
	date := c.Query("date")
	if from == "" || to == "" || date == "" {
		response.BadRequest(c, "from/to/date 必填")
		return
	}
	list, err := h.Svc.SearchTransfers(from, to, date)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *Handler) CreateTransferOrders(c *gin.Context) {
	var req services.CreateTransferReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if !h.Svc.Captcha.Verify(c.Request.Context(), req.CaptchaID, req.CaptchaCode) {
		response.BadRequest(c, "验证码错误或已过期")
		return
	}
	res, err := h.Svc.CreateTransferOrders(middleware.GetUserID(c), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, res)
}

func (h *Handler) CreateOrder(c *gin.Context) {
	var req services.CreateOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if !h.Svc.Captcha.Verify(c.Request.Context(), req.CaptchaID, req.CaptchaCode) {
		response.BadRequest(c, "验证码错误或已过期")
		return
	}
	order, err := h.Svc.CreateOrder(middleware.GetUserID(c), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, order)
}

func (h *Handler) RescheduleOrder(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req services.RescheduleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	order, err := h.Svc.RescheduleOrder(middleware.GetUserID(c), uint(id), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, order)
}

func (h *Handler) ListOrders(c *gin.Context) {
	list, err := h.Svc.ListOrders(middleware.GetUserID(c))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *Handler) GetOrder(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	order, err := h.Svc.GetOrder(middleware.GetUserID(c), uint(id))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, order)
}

func (h *Handler) PayOrder(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	order, err := h.Svc.PayOrder(middleware.GetUserID(c), uint(id))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, order)
}

func (h *Handler) CancelOrder(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.Svc.CancelOrder(middleware.GetUserID(c), uint(id)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) RefundOrder(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.Svc.RefundOrder(middleware.GetUserID(c), uint(id)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) ListPassengers(c *gin.Context) {
	list, err := h.Svc.ListPassengers(middleware.GetUserID(c))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *Handler) AddPassenger(c *gin.Context) {
	var p models.Passenger
	if err := c.ShouldBindJSON(&p); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if p.Name == "" || p.IDNumber == "" {
		response.BadRequest(c, "姓名和证件号必填")
		return
	}
	if p.IDType == "" {
		p.IDType = "身份证"
	}
	if p.PassengerType == "" {
		p.PassengerType = "成人"
	}
	if err := h.Svc.AddPassenger(middleware.GetUserID(c), &p); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, p)
}

func (h *Handler) DeletePassenger(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.Svc.DeletePassenger(middleware.GetUserID(c), uint(id)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) CreateWaitlist(c *gin.Context) {
	var req services.CreateWaitlistReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	w, err := h.Svc.CreateWaitlist(middleware.GetUserID(c), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, w)
}

func (h *Handler) ListWaitlist(c *gin.Context) {
	list, err := h.Svc.ListWaitlist(middleware.GetUserID(c))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *Handler) CancelWaitlist(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.Svc.CancelWaitlist(middleware.GetUserID(c), uint(id)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) Health(c *gin.Context) {
	response.OK(c, gin.H{"status": "up"})
}
