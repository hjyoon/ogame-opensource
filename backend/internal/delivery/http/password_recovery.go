package httpdelivery

import (
	"encoding/json"
	"html"
	"net/http"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type passwordRecoveryRequest struct {
	Email string `json:"email"`
}

type passwordRecoveryResponse struct {
	Submitted bool   `json:"submitted"`
	Sent      bool   `json:"sent"`
	Message   string `json:"message"`
}

func (a app) handlePasswordRecovery(w http.ResponseWriter, r *http.Request) {
	if a.deps.PasswordRecovery == nil {
		http.Error(w, "password recovery unavailable", http.StatusServiceUnavailable)
		return
	}
	var request passwordRecoveryRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid password recovery request", http.StatusBadRequest)
		return
	}
	result, err := a.deps.PasswordRecovery.RecoverPassword(r.Context(), domain.PasswordRecoveryCommand{Email: request.Email})
	if err != nil {
		http.Error(w, "password recovery unavailable", http.StatusServiceUnavailable)
		return
	}
	message := legacyPasswordRecoveryError
	if result.Sent {
		message = "Your password has been sent to " + result.Account.Character + "."
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(passwordRecoveryResponse{Submitted: result.Submitted, Sent: result.Sent, Message: message})
}

func (a app) handleLegacyPasswordRecoveryForm(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(legacyPasswordRecoveryForm()))
}

func (a app) handleLegacyPasswordRecovery(w http.ResponseWriter, r *http.Request) {
	if a.deps.PasswordRecovery == nil {
		http.Error(w, "password recovery unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid password recovery request", http.StatusBadRequest)
		return
	}
	result, err := a.deps.PasswordRecovery.RecoverPassword(r.Context(), domain.PasswordRecoveryCommand{Email: r.FormValue("email")})
	if err != nil {
		http.Error(w, "password recovery unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(legacyPasswordRecoveryResult(result)))
}

const legacyPasswordRecoveryError = "This email-address doesn't exist as a permanent or variable address"

func legacyPasswordRecoveryForm() string {
	return `<html>
<head>
<title>Overview</title>
<link rel="stylesheet" type="text/css" href="/evolution/formate.css">
  <link rel='stylesheet' type='text/css' href='/game/css/default.css' />
  <link rel='stylesheet' type='text/css' href='/game/css/formate.css' />
<meta http-equiv="content" type="text/html; charset=UTF-8" />
</head>
<body>
<div id="overDiv" style="position:absolute; visibility:hidden; z-index:1000;"></div>
<div class="mybody">
<form action="fa_pass.php" method="post">
<div align="center">
  <h2>Send Password</h2>
  Please enter your email-address<table align="center">
<tr>
        <td>E-Mail:</td>
        <td><input type="text" name="email"></td>
</tr>
<tr>
        <td></td>
        <td><input type="submit" name="send_pass" value="send login data"></td>
</tr>
</table>
</form>
</body>
</html>`
}

func legacyPasswordRecoveryResult(result domain.PasswordRecoveryResult) string {
	message := `<font color="red">` + legacyPasswordRecoveryError + `</font>`
	if result.Sent {
		message = `<font color="lime">Your password has been sent to ` + html.EscapeString(result.Account.Character) + `.</font>`
	}
	return `<!doctype html>
<html>
<head>
<title>Send OGame Password</title>
<link rel="stylesheet" type="text/css" href="/evolution/formate.css">
<meta http-equiv="content-type" content="text/html; charset=UTF-8">
</head>
<body>
<div id="overDiv" style="position:absolute; visibility:hidden; z-index:1000;"></div>
<center><table width="519"><tr><th>` + message + `</th></tr></table></center>
</body>
</html>`
}
