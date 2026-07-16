package sqlitedb

import (
	"context"
	"crypto/md5"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

const (
	userLegor = 1
	userSpace = 99999
)

var (
	prefixPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	memoryID      atomic.Uint64

	//go:embed schema/*.sql
	schemaFS embed.FS
)

type BootstrapOptions struct {
	Prefix        string
	Secret        string
	Universe      int
	PublicBaseURL string
	AdminEmail    string
	AdminPassword string
	Now           time.Time
}

func Open(path string) (*sql.DB, error) {
	dsn, err := dataSourceName(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func dataSourceName(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("sqlite database path is empty")
	}
	if path == ":memory:" {
		name := "ogame-" + strconv.FormatUint(memoryID.Add(1), 10)
		return "file:" + name + "?mode=memory&cache=shared&_pragma=busy_timeout%285000%29&_pragma=foreign_keys%28ON%29", nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	return u.String() + "?_pragma=busy_timeout%285000%29&_pragma=journal_mode%28WAL%29&_pragma=foreign_keys%28ON%29", nil
}

func BootstrapMaster(ctx context.Context, db *sql.DB, options BootstrapOptions) error {
	if db == nil {
		return errors.New("sqlite master database is unavailable")
	}
	if err := executeSchema(ctx, db, "schema/master.sql", ""); err != nil {
		return err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(options.PublicBaseURL), "/")
	_, err := db.ExecContext(ctx, "INSERT OR IGNORE INTO unis (id, num, dbhost, dbuser, dbpass, dbname, uniurl) VALUES (1, ?, '', '', '', ?, ?)", universeNumber(options.Universe), "sqlite", baseURL)
	return err
}

func BootstrapUniverse(ctx context.Context, db *sql.DB, options BootstrapOptions) error {
	if db == nil {
		return errors.New("sqlite universe database is unavailable")
	}
	if !prefixPattern.MatchString(options.Prefix) {
		return fmt.Errorf("invalid sqlite table prefix %q", options.Prefix)
	}
	if err := executeSchema(ctx, db, "schema/universe.sql", options.Prefix); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := seedUniverse(ctx, tx, options); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func executeSchema(ctx context.Context, db *sql.DB, name string, prefix string) error {
	data, err := schemaFS.ReadFile(name)
	if err != nil {
		return err
	}
	for _, statement := range strings.Split(strings.ReplaceAll(string(data), "{{prefix}}", prefix), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply sqlite schema %s: %w", name, err)
		}
	}
	return nil
}

func seedUniverse(ctx context.Context, tx *sql.Tx, options BootstrapOptions) error {
	prefix := options.Prefix
	now := options.Now
	if now.IsZero() {
		now = time.Now()
	}
	unix := now.Unix()
	uni := quote(prefix + "uni")
	users := quote(prefix + "users")
	planets := quote(prefix + "planets")
	if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO "+uni+" (num,speed,fspeed,galaxies,systems,maxusers,acs,fid,did,rapid,moons,defrepair,defrepair_delta,usercount,freeze,news1,news2,news_until,startdate,battle_engine,lang,hacks,ext_board,ext_discord,ext_tutorial,ext_rules,ext_impressum,php_battle,battle_max,force_lang,start_dm,max_werf,feedage,modlist) VALUES (?,1,1,9,499,12500,4,30,0,0,0,70,10,1,0,'','',0,?,'../cgi-bin/battle','en',0,'','','','','',0,1000000,0,0,999,60,'')", universeNumber(options.Universe), unix); err != nil {
		return err
	}
	if err := seedUser(ctx, tx, users, userSpace, "space", "space", "", legacyPassword("space", options.Secret), 2, 1, 0, unix); err != nil {
		return err
	}
	if err := seedUser(ctx, tx, users, userLegor, "legor", "Legor", options.AdminEmail, legacyPassword(defaultString(options.AdminPassword, "admin"), options.Secret), 2, 1, 1, unix); err != nil {
		return err
	}
	planetInsert := "INSERT OR IGNORE INTO " + planets + " (planet_id,name,type,g,s,p,owner_id,diameter,temp,fields,maxfields,date,lastpeek,lastakt,gate_until,remove) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,0,0)"
	if _, err := tx.ExecContext(ctx, planetInsert, 1, "Arakis", 1, 1, 1, 2, userLegor, 12800, 40, 0, 163, unix, unix, unix); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, planetInsert, 2, "Mond", 0, 1, 1, 2, userLegor, 8944, 10, 0, 1, unix, unix, unix); err != nil {
		return err
	}
	if err := seedRules(ctx, tx, prefix); err != nil {
		return err
	}
	return seedSequences(ctx, tx, prefix)
}

func seedUser(ctx context.Context, tx *sql.Tx, table string, id int, name string, originalName string, email string, password string, admin int, homePlanet int, activePlanet int, now int64) error {
	columns := "player_id,regdate,ally_id,joindate,allyrank,session,private_session,name,oname,name_changed,name_until,password,temp_pass,pemail,email,email_changed,email_until,disable,disable_until,vacation,vacation_until,banned,banned_until,noattack,noattack_until,lastlogin,lastclick,ip_addr,validated,validatemd,hplanetid,admin,sortby,sortorder,skin,useskin,deact_ip,maxspy,maxfleetmsg,lang,aktplanet,dm,dmfree,sniff,debug,trader,rate_m,rate_k,rate_d,score1,score2,score3,place1,place2,place3,oldscore1,oldscore2,oldscore3,oldplace1,oldplace2,oldplace3,scoredate,flags,feedid,lastfeed,com_until,adm_until,eng_until,geo_until,tec_until"
	values := "?,?,0,0,0,'','',?,?,0,0,?,'',?,?,0,0,0,0,0,0,0,0,0,0,0,0,'0.0.0.0',1,'',?,?,0,0,'/evolution/',1,1,1,3,'en',?,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,31,'',0,0,0,0,0,0"
	_, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO "+table+" ("+columns+") VALUES ("+values+")", id, now, name, originalName, password, email, email, homePlanet, admin, activePlanet)
	return err
}

func seedRules(ctx context.Context, tx *sql.Tx, prefix string) error {
	expedition := []any{70, 25, 50, 75, 25, 50, 75, 95, 85, 70, 69, 63, 60, 25, 1, 3, 10000, 100000, 1000000, 5000000, 25000000, 50000000, 75000000, 100000000, 9000, 9000, 9000, 9000, 12000, 12000, 12000, 12000, 12000}
	if err := insertRuleRow(ctx, tx, quote(prefix+"exptab"), expedition); err != nil {
		return err
	}
	colony := []any{50, 120, 72, 50, 150, 120, 50, 120, 120, 50, 120, 96, 50, 150, 96}
	if err := insertRuleRow(ctx, tx, quote(prefix+"coltab"), colony); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO "+quote(prefix+"botstrat")+" (id,name,source) VALUES (1,'backup','')")
	return err
}

func insertRuleRow(ctx context.Context, tx *sql.Tx, table string, values []any) error {
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil || count > 0 {
		return err
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(values)), ",")
	_, err := tx.ExecContext(ctx, "INSERT INTO "+table+" VALUES ("+placeholders+")", values...)
	return err
}

func seedSequences(ctx context.Context, tx *sql.Tx, prefix string) error {
	for table, sequence := range map[string]int64{"planets": 9999, "messages": 9999, "allyapps": 9999, "debug": 9999, "errors": 9999, "fleet": 9999, "union": 9999} {
		name := prefix + table
		if _, err := tx.ExecContext(ctx, "INSERT INTO sqlite_sequence(name,seq) SELECT ?,? WHERE NOT EXISTS (SELECT 1 FROM sqlite_sequence WHERE name=?)", name, sequence, name); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE sqlite_sequence SET seq = CASE WHEN seq < ? THEN ? ELSE seq END WHERE name = ?", sequence, sequence, name); err != nil {
			return err
		}
	}
	return nil
}

func quote(identifier string) string {
	return `"` + identifier + `"`
}

func universeNumber(number int) int {
	if number < 1 {
		return 1
	}
	return number
}

func legacyPassword(password string, secret string) string {
	sum := md5.Sum([]byte(password + secret))
	return hex.EncodeToString(sum[:])
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
