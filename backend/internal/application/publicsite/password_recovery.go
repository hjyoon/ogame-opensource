package publicsite

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type PasswordRecoveryRepository interface {
	RecoverPassword(context.Context, string, string) (domain.PasswordRecoveryAccount, error)
}

type PasswordRecoveryMailer interface {
	SendPasswordRecovery(context.Context, domain.PasswordRecoveryMail) error
}

type PasswordGenerator interface {
	NewPassword() (string, error)
}

type PasswordRecoveryService struct {
	repository     PasswordRecoveryRepository
	mailer         PasswordRecoveryMailer
	passwords      PasswordGenerator
	universeNumber int
	publicBaseURL  string
}

func NewPasswordRecoveryService(repository PasswordRecoveryRepository, mailer PasswordRecoveryMailer, universeNumber int, publicBaseURL string) PasswordRecoveryService {
	return NewPasswordRecoveryServiceWithGenerator(repository, mailer, RandomPasswordGenerator{}, universeNumber, publicBaseURL)
}

func NewPasswordRecoveryServiceWithGenerator(repository PasswordRecoveryRepository, mailer PasswordRecoveryMailer, passwords PasswordGenerator, universeNumber int, publicBaseURL string) PasswordRecoveryService {
	if passwords == nil {
		passwords = RandomPasswordGenerator{}
	}
	return PasswordRecoveryService{
		repository:     repository,
		mailer:         mailer,
		passwords:      passwords,
		universeNumber: universeNumber,
		publicBaseURL:  publicBaseURL,
	}
}

func (s PasswordRecoveryService) RecoverPassword(ctx context.Context, command domain.PasswordRecoveryCommand) (domain.PasswordRecoveryResult, error) {
	email, ok := domain.NormalizeRecoveryEmail(command.Email)
	if !ok {
		return domain.PasswordRecoveryResult{Submitted: true}, nil
	}
	if s.repository == nil || s.mailer == nil || s.passwords == nil {
		return domain.PasswordRecoveryResult{}, errors.New("password recovery dependencies unavailable")
	}

	password, err := s.passwords.NewPassword()
	if err != nil {
		return domain.PasswordRecoveryResult{}, err
	}
	account, err := s.repository.RecoverPassword(ctx, email, password)
	if err != nil {
		return domain.PasswordRecoveryResult{}, err
	}
	if !account.Found {
		return domain.PasswordRecoveryResult{Submitted: true}, nil
	}
	if err := s.mailer.SendPasswordRecovery(ctx, domain.PasswordRecoveryMail{
		Character:      account.Character,
		Email:          account.PermanentEmail,
		Password:       password,
		UniverseNumber: s.universeNumber,
		PublicBaseURL:  s.publicBaseURL,
	}); err != nil {
		return domain.PasswordRecoveryResult{}, err
	}
	return domain.PasswordRecoveryResult{Submitted: true, Sent: true, Account: account}, nil
}

type RandomPasswordGenerator struct{}

const passwordAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

func (RandomPasswordGenerator) NewPassword() (string, error) {
	var builder strings.Builder
	builder.Grow(8)
	max := big.NewInt(int64(len(passwordAlphabet)))
	for builder.Len() < 8 {
		index, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(passwordAlphabet[index.Int64()])
	}
	return builder.String(), nil
}
