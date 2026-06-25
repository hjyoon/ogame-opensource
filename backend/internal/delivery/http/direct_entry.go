package httpdelivery

import (
	"context"
	"fmt"
	"html"
	"net/http"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
)

type DirectEntryUseCase interface {
	ResolveExternalRedirect(context.Context, string) apppublicsite.ExternalRedirectResult
	ProxyExternalImage(context.Context, string) (apppublicsite.ExternalImageProxyResult, error)
}

func (a app) handleLegacyRedirect(w http.ResponseWriter, r *http.Request) {
	result := apppublicsite.ExternalRedirectResult{}
	if a.deps.DirectEntry != nil {
		result = a.deps.DirectEntry.ResolveExternalRedirect(r.Context(), r.URL.Query().Get("url"))
	}
	if !result.Allowed {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("<html><head><title>Invalid URL</title></head><body>Invalid URL</body></html>"))
		return
	}
	safeURL := html.EscapeString(result.URL)
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	_, _ = fmt.Fprintf(w, "<HTML>\n<HEAD>\n<META HTTP-EQUIV=\"refresh\" content=\"0;URL=%s\">\n<TITLE>Page has moved</TITLE>\n</HEAD>\n<BODY>\nPage has moved\n</BODY>\n</HTML>\n", safeURL)
}

func (a app) handleLegacyImageProxy(w http.ResponseWriter, r *http.Request) {
	if a.deps.DirectEntry != nil {
		result, err := a.deps.DirectEntry.ProxyExternalImage(r.Context(), r.URL.Query().Get("url"))
		if err != nil && a.deps.Logger != nil {
			a.deps.Logger.Warn("legacy image proxy unavailable", "error", err.Error())
		}
		if err == nil && result.Available {
			w.Header().Set("Content-Type", result.ContentType)
			_, _ = w.Write(result.Body)
			return
		}
	}
	writeLegacyImageUnavailable(w)
}

func writeLegacyImageUnavailable(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	_, _ = w.Write([]byte("<font color=red><b>Графика недоступна</b></font>"))
}
