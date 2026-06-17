package mysqlregistration

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type registrationTx interface {
	QueryContext(ctx context.Context, query string, args ...any) (Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type registrationTxRunner interface {
	WithTx(context.Context, func(registrationTx) error) error
}

type SQLTxRunner struct {
	DB *sql.DB
}

type AccountCreator struct {
	txer        registrationTxRunner
	prefix      string
	secret      string
	now         func() time.Time
	randomBytes func(int) ([]byte, error)
}

type sqlRegistrationTx struct {
	tx *sql.Tx
}

type registrationUniverse struct {
	Number    int
	Systems   int
	Galaxies  int
	Language  string
	ForceLang int
	StartDM   int
}

type planetCoordinates struct {
	Galaxy   int
	System   int
	Position int
}

func NewAccountCreator(db *sql.DB, prefix string, secret string) AccountCreator {
	return NewAccountCreatorWithRunner(SQLTxRunner{DB: db}, prefix, secret, time.Now, cryptoRandomBytes)
}

func NewAccountCreatorWithRunner(
	txer registrationTxRunner,
	prefix string,
	secret string,
	now func() time.Time,
	randomBytes func(int) ([]byte, error),
) AccountCreator {
	if now == nil {
		now = time.Now
	}
	if randomBytes == nil {
		randomBytes = cryptoRandomBytes
	}
	return AccountCreator{txer: txer, prefix: prefix, secret: secret, now: now, randomBytes: randomBytes}
}

func (r SQLTxRunner) WithTx(ctx context.Context, fn func(registrationTx) error) error {
	if r.DB == nil {
		return errors.New("registration database unavailable")
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(sqlRegistrationTx{tx: tx}); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (t sqlRegistrationTx) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

func (t sqlRegistrationTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (c AccountCreator) CreateRegistrationAccount(ctx context.Context, draft domain.RegistrationDraft, remoteAddr string) (domain.RegisteredAccount, error) {
	if c.txer == nil {
		return domain.RegisteredAccount{}, errors.New("registration account creator dependency unavailable")
	}
	usersTable, err := tableName(c.prefix, "users")
	if err != nil {
		return domain.RegisteredAccount{}, err
	}
	planetsTable, err := tableName(c.prefix, "planets")
	if err != nil {
		return domain.RegisteredAccount{}, err
	}
	uniTable, err := tableName(c.prefix, "uni")
	if err != nil {
		return domain.RegisteredAccount{}, err
	}

	random, err := c.randomBytes(17)
	if err != nil {
		return domain.RegisteredAccount{}, err
	}
	activationCode := hex.EncodeToString(random[:16])
	registeredAt := int(c.now().Unix())
	character := strings.TrimSpace(draft.Character)
	email := strings.ToLower(strings.TrimSpace(draft.Email))
	account := domain.RegisteredAccount{ActivationCode: activationCode, Validated: false}

	err = c.txer.WithTx(ctx, func(tx registrationTx) error {
		universe, err := loadRegistrationUniverse(ctx, tx, uniTable)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET usercount = usercount + 1", uniTable)); err != nil {
			return err
		}
		playerID, err := insertRegistrationUser(ctx, tx, usersTable, registrationUserRow{
			RegisteredAt:       registeredAt,
			OriginalName:       character,
			Name:               strings.ToLower(character),
			PasswordHash:       hashLegacyPassword(draft.Password, c.secret),
			Email:              email,
			RemoteAddr:         remoteAddr,
			ActivationCode:     activationCode,
			Language:           universe.Language,
			StartingDarkMatter: universe.StartDM,
		})
		if err != nil {
			return err
		}
		coords, err := nextHomePlanetCoordinates(ctx, tx, planetsTable, universe)
		if err != nil {
			return err
		}
		homePlanetID, err := insertHomePlanet(ctx, tx, planetsTable, homePlanetRow{
			PlayerID:    playerID,
			Coordinates: coords,
			CreatedAt:   registeredAt,
			Temperature: homePlanetTemperature(coords.Position, int(random[16])%10),
		})
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET hplanetid = ?, aktplanet = ? WHERE player_id = ?", usersTable), homePlanetID, homePlanetID, playerID); err != nil {
			return err
		}
		account.PlayerID = playerID
		account.HomePlanetID = homePlanetID
		return nil
	})
	if err != nil {
		return domain.RegisteredAccount{}, err
	}
	return account, nil
}

type registrationUserRow struct {
	RegisteredAt       int
	OriginalName       string
	Name               string
	PasswordHash       string
	Email              string
	RemoteAddr         string
	ActivationCode     string
	Language           string
	StartingDarkMatter int
}

type homePlanetRow struct {
	PlayerID    int
	Coordinates planetCoordinates
	CreatedAt   int
	Temperature int
}

func loadRegistrationUniverse(ctx context.Context, tx registrationTx, uniTable string) (registrationUniverse, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("SELECT num, systems, galaxies, lang, force_lang, start_dm FROM %s LIMIT 1", uniTable))
	if err != nil {
		return registrationUniverse{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return registrationUniverse{}, errors.New("registration universe row not found")
	}
	var universe registrationUniverse
	if err := rows.Scan(&universe.Number, &universe.Systems, &universe.Galaxies, &universe.Language, &universe.ForceLang, &universe.StartDM); err != nil {
		return registrationUniverse{}, err
	}
	if err := rows.Err(); err != nil {
		return registrationUniverse{}, err
	}
	if universe.Systems <= 0 || universe.Galaxies <= 0 {
		return registrationUniverse{}, errors.New("registration universe has invalid galaxy layout")
	}
	if strings.TrimSpace(universe.Language) == "" {
		universe.Language = "en"
	}
	return universe, nil
}

func insertRegistrationUser(ctx context.Context, tx registrationTx, usersTable string, row registrationUserRow) (int, error) {
	columns := []string{
		"regdate", "ally_id", "joindate", "allyrank", "session", "private_session", "name", "oname", "name_changed", "name_until",
		"password", "temp_pass", "pemail", "email", "email_changed", "email_until", "disable", "disable_until", "vacation", "vacation_until",
		"banned", "banned_until", "noattack", "noattack_until", "lastlogin", "lastclick", "ip_addr", "validated", "validatemd", "hplanetid",
		"admin", "sortby", "sortorder", "skin", "useskin", "deact_ip", "maxspy", "maxfleetmsg", "lang", "aktplanet",
		"dm", "dmfree", "sniff", "debug", "trader", "rate_m", "rate_k", "rate_d", "score1", "score2", "score3", "place1", "place2", "place3",
		"oldscore1", "oldscore2", "oldscore3", "oldplace1", "oldplace2", "oldplace3", "scoredate", "flags", "feedid", "lastfeed",
		"com_until", "adm_until", "eng_until", "geo_until", "tec_until",
	}
	args := []any{
		row.RegisteredAt, 0, 0, 0, "", "", row.Name, row.OriginalName, 0, 0,
		row.PasswordHash, "", row.Email, row.Email, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, row.RemoteAddr, 0, row.ActivationCode, 0,
		0, 0, 0, "/evolution/", 1, 0, 1, 3, row.Language, 0,
		0, row.StartingDarkMatter, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 31, "", 0,
		0, 0, 0, 0, 0,
	}
	result, err := tx.ExecContext(ctx, insertStatement(usersTable, columns), args...)
	if err != nil {
		return 0, err
	}
	return lastInsertID(result)
}

func nextHomePlanetCoordinates(ctx context.Context, tx registrationTx, planetsTable string, universe registrationUniverse) (planetCoordinates, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("SELECT g, s, p FROM %s WHERE g >= 1 AND p <= 15 AND type <> 10002 ORDER BY g, s, p", planetsTable))
	if err != nil {
		return planetCoordinates{}, err
	}
	defer rows.Close()
	occupied := make(map[int]bool)
	ppg := 15 * universe.Systems
	for rows.Next() {
		var coords planetCoordinates
		if err := rows.Scan(&coords.Galaxy, &coords.System, &coords.Position); err != nil {
			return planetCoordinates{}, err
		}
		if coords.Galaxy < 1 || coords.System < 1 || coords.Position < 1 {
			continue
		}
		index := ((coords.Galaxy - 1) * ppg) + ((coords.System - 1) * 15) + coords.Position - 1
		occupied[index] = true
	}
	if err := rows.Err(); err != nil {
		return planetCoordinates{}, err
	}
	return firstHomePlanetSlot(universe, occupied)
}

func firstHomePlanetSlot(universe registrationUniverse, occupied map[int]bool) (planetCoordinates, error) {
	ppg := 15 * universe.Systems
	for distance := 0.0; distance < float64(ppg*9); distance += 1.3 {
		index := int(math.Floor(distance))
		galaxy := index/ppg + 1
		if galaxy > universe.Galaxies {
			break
		}
		withinGalaxy := index - ((galaxy - 1) * ppg)
		system := withinGalaxy/15 + 1
		position := withinGalaxy%15 + 1
		if position > 3 && position < 13 && !occupied[index] {
			return planetCoordinates{Galaxy: galaxy, System: system, Position: position}, nil
		}
	}
	return planetCoordinates{}, errors.New("no home planet slots available")
}

func insertHomePlanet(ctx context.Context, tx registrationTx, planetsTable string, row homePlanetRow) (int, error) {
	columns := []string{"name", "type", "g", "s", "p", "owner_id", "diameter", "temp", "fields", "maxfields", "date", "700", "701", "702", "lastpeek", "lastakt", "gate_until", "remove"}
	args := []any{
		homePlanetName(),
		1,
		row.Coordinates.Galaxy,
		row.Coordinates.System,
		row.Coordinates.Position,
		row.PlayerID,
		12800,
		row.Temperature,
		0,
		163,
		row.CreatedAt,
		500,
		500,
		0,
		row.CreatedAt,
		row.CreatedAt,
		0,
		0,
	}
	result, err := tx.ExecContext(ctx, insertStatement(planetsTable, columns), args...)
	if err != nil {
		return 0, err
	}
	return lastInsertID(result)
}

func insertStatement(table string, columns []string) string {
	quoted := make([]string, 0, len(columns))
	placeholders := make([]string, 0, len(columns))
	for _, column := range columns {
		quoted = append(quoted, "`"+column+"`")
		placeholders = append(placeholders, "?")
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(quoted, ", "), strings.Join(placeholders, ", "))
}

func lastInsertID(result sql.Result) (int, error) {
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, errors.New("database returned empty insert id")
	}
	return int(id), nil
}

func homePlanetTemperature(position int, jitter int) int {
	switch {
	case position <= 3:
		return 80 + jitter - 2*position
	case position >= 4 && position <= 6:
		return 30 + jitter - 2*position
	case position >= 7 && position <= 9:
		return 10 + jitter - 2*position
	case position >= 10 && position <= 12:
		return -10 + jitter - 2*position
	default:
		return -60 + jitter - 2*position
	}
}

func homePlanetName() string {
	return "Homeplanet"
}

func cryptoRandomBytes(size int) ([]byte, error) {
	value := make([]byte, size)
	_, err := rand.Read(value)
	return value, err
}
