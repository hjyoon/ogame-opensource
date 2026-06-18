package mysqlregistration

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func TestAccountActivatorValidatesUserAndCopiesTemporaryEmail(t *testing.T) {
	tx := &fakeRegistrationTx{
		queryResponses: []fakeResponse{{rows: &fakeRows{items: []fakeRow{{values: []any{42, "new@example.local", 0}}}}}},
	}
	activator := NewAccountActivatorWithRunner(&fakeRegistrationTxRunner{tx: tx}, "uni1_")

	account, err := activator.ActivateRegistrationAccount(context.Background(), " ack ")

	if err != nil {
		t.Fatal(err)
	}
	if !account.Found || account.PlayerID != 42 {
		t.Fatalf("unexpected activated account: %+v", account)
	}
	if len(tx.execCalls) != 2 {
		t.Fatalf("expected pemail and validation updates, got %+v", tx.execCalls)
	}
	if !strings.Contains(tx.queryCalls[0].query, "validatemd = ?") || tx.queryCalls[0].args[0] != "ack" {
		t.Fatalf("expected activation lookup by trimmed code, got %+v", tx.queryCalls[0])
	}
	if !strings.Contains(tx.execCalls[0].query, "pemail = ?") || tx.execCalls[0].args[0] != "new@example.local" || tx.execCalls[0].args[1] != 42 {
		t.Fatalf("expected pemail update, got %+v", tx.execCalls[0])
	}
	if !strings.Contains(tx.execCalls[1].query, "validatemd = ''") || tx.execCalls[1].args[0] != 42 {
		t.Fatalf("expected validation update, got %+v", tx.execCalls[1])
	}
}

func TestAccountActivatorSkipsPEmailForAlreadyValidatedUser(t *testing.T) {
	tx := &fakeRegistrationTx{
		queryResponses: []fakeResponse{{rows: &fakeRows{items: []fakeRow{{values: []any{42, "new@example.local", 1}}}}}},
	}
	activator := NewAccountActivatorWithRunner(&fakeRegistrationTxRunner{tx: tx}, "uni1_")

	account, err := activator.ActivateRegistrationAccount(context.Background(), "ack")

	if err != nil {
		t.Fatal(err)
	}
	if !account.Found || len(tx.execCalls) != 1 || strings.Contains(tx.execCalls[0].query, "pemail") {
		t.Fatalf("expected validation-only update, account=%+v exec=%+v", account, tx.execCalls)
	}
}

func TestAccountActivatorReturnsNotFoundForMissingCode(t *testing.T) {
	tx := &fakeRegistrationTx{queryResponses: []fakeResponse{{rows: &fakeRows{}}}}
	activator := NewAccountActivatorWithRunner(&fakeRegistrationTxRunner{tx: tx}, "uni1_")

	account, err := activator.ActivateRegistrationAccount(context.Background(), "missing")

	if err != nil {
		t.Fatal(err)
	}
	if account.Found || len(tx.execCalls) != 0 {
		t.Fatalf("expected no activation for missing code, account=%+v exec=%+v", account, tx.execCalls)
	}
}

func TestNewAccountActivatorUsesSQLRunner(t *testing.T) {
	activator := NewAccountActivator(&sql.DB{}, "uni1_")

	if _, ok := activator.txer.(SQLTxRunner); !ok || activator.prefix != "uni1_" {
		t.Fatalf("unexpected account activator: %+v", activator)
	}
}

func TestAccountActivatorReturnsErrors(t *testing.T) {
	if _, err := (AccountActivator{}).ActivateRegistrationAccount(context.Background(), "ack"); err == nil {
		t.Fatal("expected missing dependency error")
	}
	if _, err := NewAccountActivatorWithRunner(&fakeRegistrationTxRunner{}, "uni1_;DROP").ActivateRegistrationAccount(context.Background(), "ack"); err == nil {
		t.Fatal("expected unsafe prefix error")
	}
	wantTxErr := errors.New("tx failed")
	if _, err := NewAccountActivatorWithRunner(&fakeRegistrationTxRunner{err: wantTxErr}, "uni1_").ActivateRegistrationAccount(context.Background(), "ack"); !errors.Is(err, wantTxErr) {
		t.Fatalf("expected tx error, got %v", err)
	}
	cases := map[string]*fakeRegistrationTx{
		"query": {
			queryResponses: []fakeResponse{{err: errors.New("query failed")}},
		},
		"scan": {
			queryResponses: []fakeResponse{{rows: &fakeRows{items: []fakeRow{{values: []any{42, "new@example.local", 0}, scanErr: errors.New("scan failed")}}}}},
		},
		"rows": {
			queryResponses: []fakeResponse{{rows: &fakeRows{items: []fakeRow{{values: []any{42, "new@example.local", 0}}}, err: errors.New("rows failed")}}},
		},
		"pemail update": {
			queryResponses: []fakeResponse{{rows: &fakeRows{items: []fakeRow{{values: []any{42, "new@example.local", 0}}}}}},
			execErrors:     []error{errors.New("pemail failed")},
		},
		"validate update": {
			queryResponses: []fakeResponse{{rows: &fakeRows{items: []fakeRow{{values: []any{42, "new@example.local", 1}}}}}},
			execErrors:     []error{errors.New("validate failed")},
		},
	}
	for name, tx := range cases {
		t.Run(name, func(t *testing.T) {
			activator := NewAccountActivatorWithRunner(&fakeRegistrationTxRunner{tx: tx}, "uni1_")
			if _, err := activator.ActivateRegistrationAccount(context.Background(), "ack"); err == nil {
				t.Fatal("expected activation error")
			}
		})
	}
}
