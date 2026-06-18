package httpdelivery

import (
	"net/http"
	"strings"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
)

func (a app) handleRegistrationActivation(w http.ResponseWriter, r *http.Request) {
	if a.deps.Activation == nil {
		http.Error(w, "registration activation unavailable", http.StatusServiceUnavailable)
		return
	}

	code := strings.TrimSpace(r.URL.Query().Get("ack"))
	if code == "" {
		http.Redirect(w, r, "/home", http.StatusFound)
		return
	}

	result, err := a.deps.Activation.ActivateAccount(r.Context(), apppublicsite.RegistrationActivationCommand{
		ActivationCode: code,
		RemoteAddr:     remoteIP(r.RemoteAddr),
	})
	if err != nil {
		http.Error(w, "registration activation unavailable", http.StatusServiceUnavailable)
		return
	}
	if !result.Activated {
		http.Redirect(w, r, "/home", http.StatusFound)
		return
	}

	setLoginSessionCookie(w, result.Session)
	http.Redirect(w, r, result.Session.RedirectTarget(), http.StatusFound)
}
