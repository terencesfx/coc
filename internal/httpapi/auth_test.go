package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoginAndMe(t *testing.T) {
	handler, accounts := setupAPI(t)
	created, err := accounts.Create(context.Background(), "keeper", "守秘人", "long-test-password", "admin", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	loginBody := bytes.NewBufferString(`{"username":"keeper","password":"long-test-password"}`)
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", loginBody)
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("expected login status %d, got %d: %s", http.StatusOK, loginResponse.Code, loginResponse.Body.String())
	}

	var sessionCookie *http.Cookie
	for _, cookie := range loginResponse.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatal("expected session cookie")
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meRequest.AddCookie(sessionCookie)
	meResponse := httptest.NewRecorder()
	handler.ServeHTTP(meResponse, meRequest)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("expected me status %d, got %d", http.StatusOK, meResponse.Code)
	}
	var current map[string]any
	if err := json.Unmarshal(meResponse.Body.Bytes(), &current); err != nil {
		t.Fatalf("decode current account: %v", err)
	}
	if current["id"] != created.ID {
		t.Fatalf("expected account %s, got %v", created.ID, current["id"])
	}
}

func TestAdminCreatesFriendAccount(t *testing.T) {
	handler, accounts := setupAPI(t)
	admin, err := accounts.Create(context.Background(), "admin", "管理员", "admin-test-password", "admin", false)
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	token, err := accounts.CreateSession(context.Background(), admin.ID, sessionTTL)
	if err != nil {
		t.Fatalf("create admin session: %v", err)
	}

	body := bytes.NewBufferString(`{"username":"friend","displayName":"朋友","password":"friend-password"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts", body)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}
	if _, err := accounts.Authenticate(context.Background(), "friend", "friend-password"); err != nil {
		t.Fatalf("authenticate created friend: %v", err)
	}
}

func TestAdminResetsAndDisablesFriendAccount(t *testing.T) {
	handler, accounts := setupAPI(t)
	admin, err := accounts.Create(context.Background(), "admin", "管理员", "admin-test-password", "admin", false)
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	friend, err := accounts.Create(context.Background(), "friend", "朋友", "friend-password", "user", true)
	if err != nil {
		t.Fatalf("create friend: %v", err)
	}
	token, err := accounts.CreateSession(context.Background(), admin.ID, sessionTTL)
	if err != nil {
		t.Fatalf("create admin session: %v", err)
	}

	resetBody := bytes.NewBufferString(`{"password":"new-friend-password"}`)
	resetRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/"+friend.ID+"/reset-password", resetBody)
	resetRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	resetResponse := httptest.NewRecorder()
	handler.ServeHTTP(resetResponse, resetRequest)
	if resetResponse.Code != http.StatusNoContent {
		t.Fatalf("expected reset status %d, got %d: %s", http.StatusNoContent, resetResponse.Code, resetResponse.Body.String())
	}
	updated, err := accounts.Authenticate(context.Background(), "friend", "new-friend-password")
	if err != nil || !updated.MustChangePassword {
		t.Fatalf("expected reset password and forced change, account=%+v error=%v", updated, err)
	}

	statusBody := bytes.NewBufferString(`{"status":"disabled"}`)
	statusRequest := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/accounts/"+friend.ID+"/status", statusBody)
	statusRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	statusResponse := httptest.NewRecorder()
	handler.ServeHTTP(statusResponse, statusRequest)
	if statusResponse.Code != http.StatusNoContent {
		t.Fatalf("expected status update %d, got %d: %s", http.StatusNoContent, statusResponse.Code, statusResponse.Body.String())
	}
	if _, err := accounts.Authenticate(context.Background(), "friend", "new-friend-password"); err == nil {
		t.Fatal("expected disabled account authentication to fail")
	}
}
