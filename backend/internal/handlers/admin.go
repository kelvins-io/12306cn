package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kelvins-io/12306cn/backend/internal/middleware"
	"github.com/kelvins-io/12306cn/backend/internal/models"
	"github.com/kelvins-io/12306cn/backend/internal/pkg/response"
)

func (h *Handler) AdminListQuotas(c *gin.Context) {
	list, err := h.Svc.ListQuotas()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *Handler) AdminUpsertQuota(c *gin.Context) {
	var q models.SegmentQuota
	if err := c.ShouldBindJSON(&q); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.Svc.UpsertQuota(&q); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, q)
}

func (h *Handler) AdminDeleteQuota(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.Svc.DeleteQuota(uint(id)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) AdminListSalePolicies(c *gin.Context) {
	list, err := h.Svc.ListSalePolicies()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *Handler) AdminUpsertSalePolicy(c *gin.Context) {
	var p models.SalePolicy
	if err := c.ShouldBindJSON(&p); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.Svc.UpsertSalePolicy(&p); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, p)
}

func (h *Handler) AdminListRiskBlocks(c *gin.Context) {
	list, err := h.Svc.ListRiskBlocks()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *Handler) AdminUpsertRiskBlock(c *gin.Context) {
	var b models.RiskBlock
	if err := c.ShouldBindJSON(&b); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.Svc.UpsertRiskBlock(&b); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, b)
}

func (h *Handler) AdminDeleteRiskBlock(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.Svc.DeleteRiskBlock(uint(id)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) AdminListTrains(c *gin.Context) {
	list, err := h.Svc.ListTrains()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *Handler) AdminListSeats(c *gin.Context) {
	trainID, _ := strconv.ParseUint(c.Query("train_id"), 10, 64)
	list, err := h.Svc.ListSeats(uint(trainID))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *Handler) AdminBlockSeat(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	seat, err := h.Svc.SetSeatBlocked(uint(id), true, req.Reason)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, seat)
}

func (h *Handler) AdminUnblockSeat(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	seat, err := h.Svc.SetSeatBlocked(uint(id), false, "")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, seat)
}

func (h *Handler) AdminRebuildProjection(c *gin.Context) {
	var req struct {
		TrainID    uint   `json:"train_id" binding:"required"`
		TravelDate string `json:"travel_date" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.Svc.RebuildProjection(req.TrainID, req.TravelDate); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) AdminListDayBlocks(c *gin.Context) {
	trainID, _ := strconv.ParseUint(c.Query("train_id"), 10, 64)
	list, err := h.Svc.ListDayBlocks(uint(trainID), c.Query("travel_date"))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *Handler) AdminUpsertDayBlock(c *gin.Context) {
	var req struct {
		TrainID    uint   `json:"train_id"`
		SeatID     uint   `json:"seat_id" binding:"required"`
		TravelDate string `json:"travel_date" binding:"required"`
		Reason     string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	b, err := h.Svc.UpsertDayBlock(req.TrainID, req.SeatID, req.TravelDate, req.Reason)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, b)
}

func (h *Handler) AdminDeleteDayBlock(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.Svc.DeleteDayBlock(uint(id)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) AdminListSaleWaves(c *gin.Context) {
	list, err := h.Svc.ListSaleWaves()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *Handler) AdminUpsertSaleWave(c *gin.Context) {
	var w models.SaleWave
	if err := c.ShouldBindJSON(&w); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.Svc.UpsertSaleWave(&w); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, w)
}

func (h *Handler) AdminDeleteSaleWave(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.Svc.DeleteSaleWave(uint(id)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) VerifyTicket(c *gin.Context) {
	var req struct {
		TicketNo   string `json:"ticket_no" binding:"required"`
		VerifyCode string `json:"verify_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	op := middleware.GetUsername(c)
	t, err := h.Svc.VerifyTicket(op, req.TicketNo, req.VerifyCode)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, t)
}

func (h *Handler) LookupTicket(c *gin.Context) {
	ticketNo := c.Query("ticket_no")
	if ticketNo == "" {
		response.BadRequest(c, "ticket_no 必填")
		return
	}
	t, o, err := h.Svc.LookupTicket(ticketNo)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, gin.H{"ticket": t, "order": o})
}
