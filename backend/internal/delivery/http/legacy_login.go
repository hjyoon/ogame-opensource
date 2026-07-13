package httpdelivery

import (
	"html"
	"net/http"
	"net/url"
	"strconv"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
)

func (a app) handleLegacyLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if a.deps.Login == nil {
		http.Error(w, "login unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid login request", http.StatusBadRequest)
		return
	}
	login := r.Form.Get("login")
	result, err := a.deps.Login.AuthenticateLogin(r.Context(), apppublicsite.LoginCommand{
		Login: login, Password: r.Form.Get("pass"), Universe: publicURL(r, "/"), RemoteAddr: remoteIP(r.RemoteAddr),
	})
	if err != nil {
		http.Error(w, "login unavailable", http.StatusServiceUnavailable)
		return
	}
	if result.Valid {
		setLoginSessionCookie(w, result.Session)
		http.Redirect(w, r, result.Session.RedirectTarget(), http.StatusFound)
		return
	}
	errorCode := 2
	if len(result.Issues) > 0 && result.Issues[0].LegacyErrorCode > 0 {
		errorCode = result.Issues[0].LegacyErrorCode
	}
	query := url.Values{}
	query.Set("errorcode", strconv.Itoa(errorCode))
	query.Set("arg1", publicURL(r, "/"))
	query.Set("arg2", login)
	http.Redirect(w, r, "/game/reg/errorpage.php?"+query.Encode(), http.StatusFound)
}

func (a app) handleLegacyLoginForm(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a app) handleLegacyLoginErrorPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	errorCode := query.Get("errorcode")
	universe := query.Get("arg1")
	login := query.Get("arg2")
	bannedUntil := query.Get("arg3")

	message := ""
	switch errorCode {
	case "2":
		message = `This account does not exist or you have entered your password incorrectly. <br>` +
			`Enter <a href='/'>the correct password</a> or use <a href='mail.php'>password recovery</a>.<br>` +
			`You can also create a <a href='new.php'>new account</a>.`
	case "3":
		message = `This account has been locked to ` + html.EscapeString(bannedUntil) +
			`, see more details below <a href=../pranger.php>here</a>.<br> If you have any questions, please contact the person who blocked you <a href='#'>operator</a>.<br><br>WARNING: commander status is not terminated when blocked, termination is done separately!`
	default:
		message = `This account does not exist or you have entered your password incorrectly.`
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`
<html>
 <head>
 <center>
 <meta http-equiv="content-type" content="text/html; charset=UTF-8" />
  <link rel='stylesheet' type='text/css' href='../css/default.css' />
  <link rel='stylesheet' type='text/css' href='../css/formate.css' />
  <link rel='stylesheet' type='text/css' href='css/default.css' />
  <link rel='stylesheet' type='text/css' href='css/formate.css' />
 <link rel="stylesheet" type="text/css" href="/evolution/formate.css" />
 <title>Error</title>
</head>
<body class='style' topmargin='0' leftmargin='0' marginwidth='0' marginheight='0' >
<div id="overDiv" style="position:absolute; visibility:hidden; z-index:1000;"></div>

 <br><br>
 <table width="519">
 <tr>
   <td class="c" align="center" ><font color="red">Error</font></td>
  </tr>
  <tr>
  <th class="errormessage">You tried to enter universe ` + html.EscapeString(universe) + ` under nickname ` + html.EscapeString(login) + `.</th>
  </tr>
  <tr>
   <th class='errormessage'>` + message + `</th>
  </tr>
 </table>
 </center>
</body>
</html>`))
}
