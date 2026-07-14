package mail

import (
	"context"
	"errors"
	"fmt"
	"mime"
	netmail "net/mail"
	"net/smtp"
	"net/url"
	"strings"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type AdminCouponMailer struct {
	config SMTPConfig
}

func NewAdminCouponMailer(config SMTPConfig) AdminCouponMailer {
	return AdminCouponMailer{config: config}
}

func (m AdminCouponMailer) SendAdminCoupon(ctx context.Context, coupon domaingame.AdminCouponMail) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	addr := strings.TrimSpace(m.config.Addr)
	if addr == "" {
		return errors.New("admin coupon SMTP address is required")
	}
	message, err := BuildAdminCouponMessage(m.config, coupon)
	if err != nil {
		return err
	}
	from, _ := netmail.ParseAddress(adminCouponFrom(m.config.PublicBaseURL))
	to, _ := netmail.ParseAddress(strings.TrimSpace(coupon.Recipient))
	return smtp.SendMail(addr, nil, from.Address, []string{to.Address}, []byte(message))
}

func BuildAdminCouponMessage(config SMTPConfig, coupon domaingame.AdminCouponMail) (string, error) {
	to, err := netmail.ParseAddress(strings.TrimSpace(coupon.Recipient))
	if err != nil {
		return "", err
	}
	from := adminCouponFrom(config.PublicBaseURL)
	if _, err := netmail.ParseAddress(from); err != nil {
		return "", err
	}
	subject, body := adminCouponLocalizedContent(coupon.Language, strings.TrimSpace(coupon.Character), strings.TrimSpace(coupon.Code))
	headers := []string{
		"From: " + cleanHeader(from),
		"To: " + cleanHeader(to.String()),
		"Subject: " + mime.BEncoding.Encode("UTF-8", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
	}
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + strings.ReplaceAll(body, "\n", "\r\n"), nil
}

func adminCouponFrom(publicBaseURL string) string {
	host := "localhost"
	if parsed, err := url.Parse(strings.TrimSpace(publicBaseURL)); err == nil && parsed.Hostname() != "" {
		host = parsed.Hostname()
	}
	return "coupon@" + host
}

func adminCouponLocalizedContent(language string, character string, code string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "fr":
		return "Un cadeau pour vous", fmt.Sprintf("Cher/Chère %s, vous avez un cadeau : %s", character, code)
	case "ru":
		return "Купон в подарок", fmt.Sprintf("<html>\n<meta http-equiv='content-type' content='text/html; charset=UTF-8' />\n<body style='background-color:#415680; font-size: 15px; font-family: Tahoma,sans-serif; font-weight: bold; color: #E6EBFB;'>\n<p>Уважаемый %s!</p>\n<p>Прими в дар от проекта OGame Open Source этот купон:<p>\n<p><big style='font-size: 22px;'>%s</big></p>\n<p>Использовать его можно в Офицерском казино, прямо в игре. Нанимай командиров, вызывай скупщика и твоя империя будет процветать!</p>\n<p>Удачной игры!</p>\n</body>\n</html>", character, code)
	default:
		return "Present to you", fmt.Sprintf("Dear %s, you have present : %s", character, code)
	}
}
