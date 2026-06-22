package mysqlgame

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type adminChecksumFileGroup struct {
	title        string
	baselineFile string
	files        []string
}

var adminChecksumGroups = []adminChecksumFileGroup{
	{
		title:        "Engine",
		baselineFile: "engine.md5",
		files: []string{
			"ainfo.php",
			"core/acs.php",
			"core/ally.php",
			"core/allyapps.php",
			"core/allyranks.php",
			"core/battle.php",
			"core/battle_engine.php",
			"core/battle_report.php",
			"core/bbcode.php",
			"core/bot.php",
			"core/botapi.php",
			"core/buddy.php",
			"core/core.php",
			"core/coupon.php",
			"core/db.php",
			"core/defs.php",
			"core/debug.php",
			"core/expedition.php",
			"core/expedition_battle.php",
			"core/fleet.php",
			"core/graviton.php",
			"index.php",
			"install.php",
			"core/install_tabs.php",
			"core/loca.php",
			"maintenance.php",
			"core/mods.php",
			"core/msg.php",
			"core/notes.php",
			"core/page.php",
			"pic.php",
			"core/planet.php",
			"pranger.php",
			"core/prod.php",
			"core/queue.php",
			"core/raketen.php",
			"redir.php",
			"core/techs.php",
			"core/uni.php",
			"core/user.php",
			"core/utils.php",
			"validate.php",
			"feed/show.php",
			"feed/viewitem.php",
		},
	},
	{
		title:        "Admin Area",
		baselineFile: "page_admin.md5",
		files: []string{
			"pages_admin/admin.php",
			"pages_admin/admin_bans.php",
			"pages_admin/admin_battle.php",
			"pages_admin/admin_botedit.php",
			"pages_admin/admin_bots.php",
			"pages_admin/admin_broadcast.php",
			"pages_admin/admin_browse.php",
			"pages_admin/admin_checksum.php",
			"pages_admin/admin_colony_settings.php",
			"pages_admin/admin_coupons.php",
			"pages_admin/admin_db.php",
			"pages_admin/admin_debug.php",
			"pages_admin/admin_errors.php",
			"pages_admin/admin_expedition.php",
			"pages_admin/admin_fleetlogs.php",
			"pages_admin/admin_home.php",
			"pages_admin/admin_loca.php",
			"pages_admin/admin_logins.php",
			"pages_admin/admin_mods.php",
			"pages_admin/admin_panel.php",
			"pages_admin/admin_planets.php",
			"pages_admin/admin_queue.php",
			"pages_admin/admin_raksim.php",
			"pages_admin/admin_reports.php",
			"pages_admin/admin_router.json",
			"pages_admin/admin_sim.php",
			"pages_admin/admin_uni.php",
			"pages_admin/admin_userlogs.php",
			"pages_admin/admin_users.php",
		},
	},
	{
		title:        "Game Pages",
		baselineFile: "page.md5",
		files: []string{
			"pages/ainfo.php",
			"pages/allianzdepot.php",
			"pages/allianzen.php",
			"pages/allianzen_circular.php",
			"pages/allianzen_main.php",
			"pages/allianzen_members.php",
			"pages/allianzen_misc.php",
			"pages/allianzen_ranks.php",
			"pages/allianzen_settings.php",
			"pages/b_building.php",
			"pages/bericht.php",
			"pages/bewerben.php",
			"pages/bewerbungen.php",
			"pages/buddy.php",
			"pages/buildings.php",
			"pages/changelog.php",
			"pages/event_list.php",
			"pages/fleet_templates.php",
			"pages/flotten1.php",
			"pages/flotten2.php",
			"pages/flotten3.php",
			"pages/flottenversand.php",
			"pages/flottenversand_ajax.php",
			"pages/galaxy.php",
			"pages/galaxy_js.php",
			"pages/imperium.php",
			"pages/infos.php",
			"pages/leftmenu.json",
			"pages/logout.php",
			"pages/messages.php",
			"pages/micropayment.php",
			"pages/notizen.php",
			"pages/options.php",
			"pages/overview.php",
			"pages/overview_events.php",
			"pages/payment.php",
			"pages/phalanx.php",
			"pages/phalanx_events.php",
			"pages/pranger.php",
			"pages/renameplanet.php",
			"pages/resources.php",
			"pages/res_panel.json",
			"pages/sprungtor.php",
			"pages/statistics.php",
			"pages/suche.php",
			"pages/techtree.php",
			"pages/techtreedetails.php",
			"pages/trader.php",
			"pages/writemessages.php",
		},
	},
	{
		title:        "Registration System",
		baselineFile: "reg.md5",
		files: []string{
			"reg/check_registration.php",
			"reg/errorpage.php",
			"reg/fa_pass.php",
			"reg/login.php",
			"reg/login2.php",
			"reg/mail.php",
			"reg/new.php",
			"reg/newredirect.php",
		},
	},
}

var phpSerializedChecksumPairPattern = regexp.MustCompile(`s:\d+:"([^"]+)";s:\d+:"([0-9a-fA-F]{32})";`)

func (r AdminRepository) loadAdminChecksumGroups(_ context.Context) ([]domaingame.AdminChecksumGroup, error) {
	result := make([]domaingame.AdminChecksumGroup, 0, len(adminChecksumGroups))
	for _, group := range adminChecksumGroups {
		baseline, err := loadPHPSerializedChecksumMap(filepath.Join(r.legacyGameDir, "temp", group.baselineFile))
		if err != nil {
			return nil, err
		}
		rows := make([]domaingame.AdminChecksumRow, 0, len(group.files))
		for _, name := range group.files {
			checksum, err := md5FileHex(filepath.Join(r.legacyGameDir, name))
			if err != nil {
				return nil, err
			}
			rows = append(rows, domaingame.AdminChecksumRow{
				Path:     name,
				Checksum: checksum,
				Status:   legacyAdminChecksumStatus(group.title, baseline, name, checksum),
			})
		}
		result = append(result, domaingame.AdminChecksumGroup{Title: group.title, Rows: rows})
	}
	return result, nil
}

func legacyAdminChecksumStatus(groupTitle string, baseline map[string]string, path string, checksum string) string {
	expected, ok := baseline[path]
	if !ok && groupTitle == "Engine" {
		return "UNVERSIONED"
	}
	if expected == checksum {
		return "OK"
	}
	return "BAD"
}

func loadPHPSerializedChecksumMap(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	matches := phpSerializedChecksumPairPattern.FindAllSubmatch(data, -1)
	result := make(map[string]string, len(matches))
	for _, match := range matches {
		result[string(match[1])] = string(match[2])
	}
	return result, nil
}

func md5FileHex(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:]), nil
}
