package main

import (
	"io"
	"log/slog"
	"testing"

	"github.com/hjyoon/ogame-opensource/backend/internal/config"
)

func TestOptionsChangeMailerConfiguration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if mailer := optionsChangeMailer(config.Config{}, logger); mailer != nil {
		t.Fatalf("SMTP-disabled options mailer must be nil: %T", mailer)
	}
	if mailer := optionsChangeMailer(config.Config{
		SMTPEnabled:   true,
		SMTPAddr:      "mailhog:1025",
		SMTPFrom:      "OGame <noreply@example.local>",
		PublicBaseURL: "http://game.example.local",
		UniNumber:     7,
	}, logger); mailer == nil {
		t.Fatal("SMTP-enabled options mailer must be configured")
	}
}

func TestAdminCouponMailerConfiguration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if mailer := adminCouponMailer(config.Config{}, logger); mailer != nil {
		t.Fatalf("SMTP-disabled admin coupon mailer must be nil: %T", mailer)
	}
	if mailer := adminCouponMailer(config.Config{SMTPEnabled: true, SMTPAddr: "mailhog:1025", PublicBaseURL: "http://game.example.local"}, logger); mailer == nil {
		t.Fatal("SMTP-enabled admin coupon mailer must be configured")
	}
}

func TestAdminReactivationMailerConfiguration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if mailer := adminReactivationMailer(config.Config{}, logger); mailer != nil {
		t.Fatalf("SMTP-disabled admin reactivation mailer must be nil: %T", mailer)
	}
	if mailer := adminReactivationMailer(config.Config{SMTPEnabled: true, SMTPAddr: "mailhog:1025", PublicBaseURL: "http://game.example.local"}, logger); mailer == nil {
		t.Fatal("SMTP-enabled admin reactivation mailer must be configured")
	}
}
