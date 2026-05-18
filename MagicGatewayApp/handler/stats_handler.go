package handler

import (
	"net/http"

	"magicgateway/auth"
	"magicgateway/store"
)

type StatsHandler struct {
	Store *store.Store
}

func (h *StatsHandler) MyStats(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r)
	period := r.URL.Query().Get("period")
	date := r.URL.Query().Get("date")

	if period == "" {
		period = "daily"
	}
	if date == "" {
		date = today()
	}

	rows, err := h.Store.GetUserStats(userID, period, date)
	if err != nil {
		writeJSON(w, 400, errResp(err.Error()))
		return
	}
	if rows == nil {
		rows = []store.StatsRow{}
	}
	writeJSON(w, 200, rows)
}

func (h *StatsHandler) AllStats(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	date := r.URL.Query().Get("date")
	userIDStr := r.URL.Query().Get("user_id")

	if period == "" {
		period = "daily"
	}
	if date == "" {
		date = today()
	}

	var userID int64
	if userIDStr != "" {
		var err error
		userID, err = parseInt64(userIDStr)
		if err != nil {
			writeJSON(w, 400, errResp("invalid user_id"))
			return
		}
	}

	rows, err := h.Store.GetAllUserStats(period, date, userID)
	if err != nil {
		writeJSON(w, 400, errResp(err.Error()))
		return
	}
	if rows == nil {
		rows = []store.StatsRow{}
	}
	writeJSON(w, 200, rows)
}

func (h *StatsHandler) Ranking(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	date := r.URL.Query().Get("date")
	if period == "" {
		period = "daily"
	}
	if date == "" {
		date = today()
	}
	rows, err := h.Store.GetRanking(period, date)
	if err != nil {
		writeJSON(w, 400, errResp(err.Error()))
		return
	}
	writeJSON(w, 200, rows)
}

func (h *StatsHandler) Breakdown(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r)
	period := r.URL.Query().Get("period")
	date := r.URL.Query().Get("date")
	if period == "" { period = "weekly" }
	if date == "" { date = today() }
	rows, err := h.Store.GetBreakdown(userID, period, date)
	if err != nil {
		writeJSON(w, 400, errResp(err.Error()))
		return
	}
	writeJSON(w, 200, rows)
}

func (h *StatsHandler) Overview(w http.ResponseWriter, r *http.Request) {
	o, err := h.Store.GetOverview()
	if err != nil {
		writeJSON(w, 500, errResp("failed to get overview"))
		return
	}
	writeJSON(w, 200, o)
}
