package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"magicgateway/auth"
	"magicgateway/store"
)

type KeyHandler struct {
	Store *store.Store
}

func (h *KeyHandler) ListMyKeys(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r)
	keys, err := h.Store.ListKeysByUserID(userID)
	if err != nil {
		writeJSON(w, 500, errResp("failed to list keys"))
		return
	}
	if keys == nil {
		keys = []store.APIKey{}
	}

	// Enrich with last used info
	type keyWithUsage struct {
		store.APIKey
		LastUsed   string `json:"last_used"`
		LastTokens int    `json:"last_tokens"`
	}
	result := make([]keyWithUsage, len(keys))
	for i, k := range keys {
		lastUsed, lastTokens := h.Store.GetKeyLastUsed(k.ID)
		result[i] = keyWithUsage{
			APIKey:     k,
			LastUsed:   lastUsed,
			LastTokens: lastTokens,
		}
	}

	writeJSON(w, 200, result)
}

func (h *KeyHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r)

	var req struct {
		KeyName string `json:"key_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, errResp("invalid request body"))
		return
	}
	req.KeyName = strings.TrimSpace(req.KeyName)
	if len(req.KeyName) > 64 {
		writeJSON(w, 400, errResp("key name too long (max 64 characters)"))
		return
	}

	fullKey, _, err := h.Store.CreateKey(userID, req.KeyName)
	if err != nil {
		writeJSON(w, 500, errResp("failed to create key"))
		return
	}

	writeJSON(w, 201, map[string]string{
		"api_key":  fullKey,
		"message":  "Store this key safely. It will not be shown again.",
	})
}

func (h *KeyHandler) RevokeKey(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r)
	role := auth.GetRole(r)

	keyID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, 400, errResp("invalid key id"))
		return
	}

	key, err := h.Store.GetKeyByID(keyID)
	if err != nil || key == nil {
		writeJSON(w, 404, errResp("key not found"))
		return
	}

	// Non-admin users can only revoke their own keys
	if role != "admin" && key.UserID != userID {
		writeJSON(w, 403, errResp("cannot revoke another user's key"))
		return
	}

	if err := h.Store.RevokeKey(keyID); err != nil {
		writeJSON(w, 500, errResp("failed to revoke key"))
		return
	}

	writeJSON(w, 200, map[string]string{"message": "key revoked"})
}

func (h *KeyHandler) EnableKey(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r)
	role := auth.GetRole(r)

	keyID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, 400, errResp("invalid key id"))
		return
	}

	key, err := h.Store.GetKeyByID(keyID)
	if err != nil || key == nil {
		writeJSON(w, 404, errResp("key not found"))
		return
	}

	if role != "admin" && key.UserID != userID {
		writeJSON(w, 403, errResp("cannot modify another user's key"))
		return
	}

	if err := h.Store.EnableKey(keyID); err != nil {
		writeJSON(w, 500, errResp("failed to enable key"))
		return
	}

	writeJSON(w, 200, map[string]string{"message": "key enabled"})
}
