package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"magicgateway/store"
)

type AdminHandler struct {
	Store *store.Store
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.Store.ListUsers()
	if err != nil {
		writeJSON(w, 500, errResp("failed to list users"))
		return
	}
	if users == nil {
		users = []store.User{}
	}
	writeJSON(w, 200, users)
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, 400, errResp("invalid user id"))
		return
	}

	user, err := h.Store.GetUserByID(userID)
	if err != nil || user == nil {
		writeJSON(w, 404, errResp("user not found"))
		return
	}
	if user.Role == "admin" {
		writeJSON(w, 403, errResp("cannot delete admin user"))
		return
	}

	if err := h.Store.DeleteUser(userID); err != nil {
		writeJSON(w, 500, errResp("failed to delete user"))
		return
	}
	writeJSON(w, 200, map[string]string{"message": "user deleted"})
}

func (h *AdminHandler) ListAllKeys(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	var userID int64
	if userIDStr != "" {
		var err error
		userID, err = strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			writeJSON(w, 400, errResp("invalid user_id"))
			return
		}
	}

	keys, err := h.Store.ListAllKeys(userID)
	if err != nil {
		writeJSON(w, 500, errResp("failed to list keys"))
		return
	}
	if keys == nil {
		keys = []store.APIKey{}
	}
	writeJSON(w, 200, keys)
}

func (h *AdminHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID  int64  `json:"user_id"`
		KeyName string `json:"key_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, errResp("invalid request body"))
		return
	}
	if req.UserID == 0 {
		writeJSON(w, 400, errResp("user_id is required"))
		return
	}

	// Verify user exists
	user, err := h.Store.GetUserByID(req.UserID)
	if err != nil || user == nil {
		writeJSON(w, 404, errResp("user not found"))
		return
	}

	req.KeyName = strings.TrimSpace(req.KeyName)
	if len(req.KeyName) > 64 {
		writeJSON(w, 400, errResp("key name too long (max 64 characters)"))
		return
	}

	fullKey, _, err := h.Store.CreateKey(req.UserID, req.KeyName)
	if err != nil {
		writeJSON(w, 500, errResp("failed to create key"))
		return
	}

	writeJSON(w, 201, map[string]string{
		"api_key":  fullKey,
		"message":  "Store this key safely. It will not be shown again.",
	})
}

func (h *AdminHandler) RevokeKey(w http.ResponseWriter, r *http.Request) {
	keyID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, 400, errResp("invalid key id"))
		return
	}

	if err := h.Store.RevokeKey(keyID); err != nil {
		writeJSON(w, 500, errResp("failed to revoke key"))
		return
	}
	writeJSON(w, 200, map[string]string{"message": "key revoked"})
}

func (h *AdminHandler) EnableKey(w http.ResponseWriter, r *http.Request) {
	keyID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, 400, errResp("invalid key id"))
		return
	}
	if err := h.Store.EnableKey(keyID); err != nil {
		writeJSON(w, 500, errResp("failed to enable key"))
		return
	}
	writeJSON(w, 200, map[string]string{"message": "key enabled"})
}

func (h *AdminHandler) ListUsersWithStats(w http.ResponseWriter, r *http.Request) {
	users, err := h.Store.GetUsersWithStats()
	if err != nil {
		writeJSON(w, 500, errResp("failed to list users"))
		return
	}
	writeJSON(w, 200, users)
}
