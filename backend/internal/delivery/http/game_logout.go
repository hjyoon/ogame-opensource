package httpdelivery

import (
	"encoding/json"
	"net/http"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
)

type gameLogoutResponse struct {
	LoggedOut  bool   `json:"loggedOut"`
	RedirectTo string `json:"redirectTo"`
}

func (a app) handleGameLogout(w http.ResponseWriter, r *http.Request) {
	if a.deps.Logout == nil {
		http.Error(w, "logout unavailable", http.StatusServiceUnavailable)
		return
	}

	result, err := a.deps.Logout.Logout(r.Context(), apppublicsite.LogoutCommand{
		PublicSession: r.URL.Query().Get("session"),
	})
	if err != nil {
		http.Error(w, "logout unavailable", http.StatusServiceUnavailable)
		return
	}
	if result.Found {
		clearLoginSessionCookie(w, result.Session.PrivateCookieName())
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(gameLogoutResponse{
		LoggedOut:  result.Found,
		RedirectTo: "/home",
	})
}
