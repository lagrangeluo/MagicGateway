package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"magicgateway/auth"
	"magicgateway/store"

	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	Store     *store.Store
	JWTSecret string
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !loginAllowed(ip) {
		writeJSON(w, 429, errResp("登录尝试次数过多，请 5 分钟后再试"))
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, errResp("invalid request body"))
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeJSON(w, 400, errResp("username and password required"))
		return
	}

	user, err := h.Store.GetUserByUsername(req.Username)
	if err != nil {
		writeJSON(w, 500, errResp("internal error"))
		return
	}
	if user == nil {
		recordFailure(ip)
		writeJSON(w, 401, errResp("用户不存在"))
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		recordFailure(ip)
		writeJSON(w, 401, errResp("密码错误"))
		return
	}

	recordSuccess(ip)
	token, err := auth.GenerateJWT(h.JWTSecret, user.ID, user.Username, user.Role)
	if err != nil {
		writeJSON(w, 500, errResp("failed to generate token"))
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, errResp("invalid request body"))
		return
	}
	req.Username = strings.TrimSpace(req.Username)

	if len(req.Username) < 2 || len(req.Username) > 32 {
		writeJSON(w, 400, errResp("username must be 2-32 characters"))
		return
	}
	if len(req.Password) < 6 {
		writeJSON(w, 400, errResp("password must be at least 6 characters"))
		return
	}

	user, err := h.Store.CreateUser(req.Username, req.Password)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeJSON(w, 409, errResp("username already exists"))
			return
		}
		writeJSON(w, 500, errResp("failed to create user"))
		return
	}

	token, err := auth.GenerateJWT(h.JWTSecret, user.ID, user.Username, user.Role)
	if err != nil {
		writeJSON(w, 500, errResp("failed to generate token"))
		return
	}

	writeJSON(w, 201, map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r)

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, errResp("invalid request body"))
		return
	}
	if len(req.NewPassword) < 6 {
		writeJSON(w, 400, errResp("new password must be at least 6 characters"))
		return
	}

	user, err := h.Store.GetUserByID(userID)
	if err != nil || user == nil {
		writeJSON(w, 404, errResp("user not found"))
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		writeJSON(w, 401, errResp("原密码错误"))
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, 500, errResp("failed to hash password"))
		return
	}

	if err := h.Store.UpdatePassword(userID, string(hash)); err != nil {
		writeJSON(w, 500, errResp("failed to update password"))
		return
	}

	writeJSON(w, 200, map[string]string{"message": "密码修改成功"})
}

func (h *AuthHandler) AdminResetPassword(w http.ResponseWriter, r *http.Request) {
	userID, err := parseInt64(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 400, errResp("invalid user id"))
		return
	}

	user, err := h.Store.GetUserByID(userID)
	if err != nil || user == nil {
		writeJSON(w, 404, errResp("user not found"))
		return
	}

	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, errResp("invalid request body"))
		return
	}
	if len(req.NewPassword) < 6 {
		writeJSON(w, 400, errResp("new password must be at least 6 characters"))
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, 500, errResp("failed to hash password"))
		return
	}

	if err := h.Store.UpdatePassword(userID, string(hash)); err != nil {
		writeJSON(w, 500, errResp("failed to update password"))
		return
	}

	writeJSON(w, 200, map[string]string{"message": user.Username + " 密码已重置"})
}
