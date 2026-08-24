package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mushan/coc/internal/account"
)

const (
	sessionCookieName = "coc_session"
	sessionTTL        = 30 * 24 * time.Hour
)

type authContextKey string

const currentAccountKey authContextKey = "current-account"

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	current, err := a.accounts.Authenticate(r.Context(), request.Username, request.Password)
	if errors.Is(err, account.ErrInvalidCredentials) || errors.Is(err, account.ErrAccountDisabled) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
		return
	}
	if err != nil {
		a.logger.Error("login failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "登录失败")
		return
	}
	token, err := a.accounts.CreateSession(r.Context(), current.ID, sessionTTL)
	if err != nil {
		a.logger.Error("create session failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "登录失败")
		return
	}
	a.setSessionCookie(w, token, int(sessionTTL.Seconds()))
	writeJSON(w, http.StatusOK, current)
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		_ = a.accounts.DeleteSession(r.Context(), cookie.Value)
	}
	a.setSessionCookie(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentAccount(r.Context()))
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (a *API) changePassword(w http.ResponseWriter, r *http.Request) {
	var request changePasswordRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	current := currentAccount(r.Context())
	err := a.accounts.ChangePassword(r.Context(), current.ID, request.CurrentPassword, request.NewPassword)
	if errors.Is(err, account.ErrInvalidCredentials) {
		writeError(w, http.StatusBadRequest, "invalid_current_password", "当前密码错误")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "is empty") {
			writeError(w, http.StatusBadRequest, "empty_password", "新密码不能为空")
			return
		}
		a.logger.Error("change password failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "修改密码失败")
		return
	}
	a.setSessionCookie(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "authentication_required", "请先登录")
			return
		}
		current, err := a.accounts.AccountBySession(r.Context(), cookie.Value)
		if err != nil {
			a.setSessionCookie(w, "", -1)
			writeError(w, http.StatusUnauthorized, "authentication_required", "登录状态已失效")
			return
		}
		ctx := context.WithValue(r.Context(), currentAccountKey, current)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *API) requireAdmin(next http.Handler) http.Handler {
	return a.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := currentAccount(r.Context())
		if current.Role != "admin" {
			writeError(w, http.StatusForbidden, "admin_required", "需要管理员权限")
			return
		}
		if current.MustChangePassword {
			writeError(w, http.StatusForbidden, "password_change_required", "请先修改初始密码")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func currentAccount(ctx context.Context) account.Account {
	current, _ := ctx.Value(currentAccountKey).(account.Account)
	return current
}

func (a *API) setSessionCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: a.cookieSecure, SameSite: http.SameSiteLaxMode,
	})
}
