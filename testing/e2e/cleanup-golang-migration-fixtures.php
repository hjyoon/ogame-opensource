<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-go-migration-fixture-cleanup';
$_SERVER['REQUEST_URI'] = '/testing/e2e/cleanup-golang-migration-fixtures.php';
$_COOKIE['ogamelang'] = 'en';

chdir('/var/www/html/game');
require_once 'config.php';
require_once 'core/core.php';

InitDB();
$GlobalUni = LoadUniverse();
$GlobalUser = array('player_id' => 0);
$session = '';
ModsInit();

function cleanup_go_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

function cleanup_go_rows(string $sql): array
{
    $res = dbquery($sql);
    $rows = array();
    while ($row = dbarray($res)) {
        $rows[] = $row;
    }
    return $rows;
}

function cleanup_go_ids(string $sql, string $column): array
{
    $ids = array();
    foreach (cleanup_go_rows($sql) as $row) {
        $ids[] = (int)$row[$column];
    }
    return array_values(array_unique(array_filter($ids, fn($id) => $id > 0)));
}

function cleanup_go_int_list(array $ids): string
{
    $ids = array_values(array_unique(array_map('intval', $ids)));
    return empty($ids) ? '0' : implode(',', $ids);
}

function cleanup_go_quoted_list(array $values): string
{
    return implode(',', array_map(fn($value) => "'" . cleanup_go_sql_escape($value) . "'", $values));
}

function cleanup_go_fixture_names(): array
{
    return array(
        getenv('OGAME_OVERVIEW_FLEET_USER') ?: 'fleetcase',
        getenv('OGAME_OVERVIEW_FLEET_TARGET_USER') ?: 'fleettarget',
        getenv('OGAME_FLEET_CONTINUE_USER') ?: 'fleetcontinue',
        getenv('OGAME_FLEET_CONTINUE_TARGET_USER') ?: 'fleetdest',
        getenv('OGAME_OVERVIEW_ALL_USER') ?: 'overviewall',
        getenv('OGAME_OVERVIEW_ALL_ENEMY_USER') ?: 'overviewenemy',
        getenv('OGAME_OVERVIEW_ALL_SUPPORT_USER') ?: 'overviewsupport',
        getenv('OGAME_FLEET_ALL_USER') ?: 'fleetall',
        getenv('OGAME_FLEET_ALL_ENEMY_USER') ?: 'fleetallenemy',
        getenv('OGAME_FLEET_ALL_SUPPORT_USER') ?: 'fleetallsupport',
        'empirevisual',
        'alliancevisual',
        'allianceapplicant',
        'gotecnanlock',
        'gotecnanopen',
        'gotecreslock',
        'gotecresopen',
        'gotecshiplock',
        'gotecshipopen',
        'gotecdeflock',
        'gotecdefopen',
        'gotececoshort',
        'gotececopower',
        'gotececocap',
        'gotececofull',
        'gotececozero',
        'goqueuebuild',
        'goqueueresearch',
        'goqueueshipyard',
        'goqueuedrain',
        'goqbuildcancel',
        'goqrescancel',
        'goqueuebuildcancel',
        'goconcshipyard',
        'goconcdefense',
        'goconcdome',
        'goconcmissile',
        'goconcfleet',
        'goconcfleettarget',
        'goacshold',
        'goacsholdbuddy',
        'goacsholdstranger',
        'gopctxown',
        'gopctxfor',
    );
}

function cleanup_go_fixture_alliance_tags(): array
{
    return array('AVQA');
}

function cleanup_go_reset_home_planets(array $userIds): void
{
    global $db_prefix;
    foreach ($userIds as $userId) {
        $rows = cleanup_go_rows("SELECT planet_id FROM {$db_prefix}planets WHERE owner_id=" . (int)$userId . " AND type=" . PTYP_PLANET . " ORDER BY planet_id ASC LIMIT 1");
        if (!empty($rows)) {
            $planetId = (int)$rows[0]['planet_id'];
            dbquery("UPDATE {$db_prefix}users SET hplanetid={$planetId}, aktplanet={$planetId} WHERE player_id=" . (int)$userId);
        }
    }
}

global $db_prefix;

if (MDBConnect()) {
    MDBQuery("DELETE FROM unis WHERE num IN (9901,9902)");
}

$names = array_map(fn($name) => mb_strtolower($name, 'UTF-8'), cleanup_go_fixture_names());
$nameList = cleanup_go_quoted_list($names);
$userIds = cleanup_go_ids("SELECT player_id FROM {$db_prefix}users WHERE name IN ({$nameList})", 'player_id');
$userList = cleanup_go_int_list($userIds);
$allianceTagList = cleanup_go_quoted_list(cleanup_go_fixture_alliance_tags());
$allianceIds = cleanup_go_ids("SELECT ally_id FROM {$db_prefix}ally WHERE tag IN ({$allianceTagList}) OR owner_id IN ({$userList})", 'ally_id');
$allianceList = cleanup_go_int_list($allianceIds);
$planetIds = cleanup_go_ids("SELECT planet_id FROM {$db_prefix}planets WHERE owner_id IN ({$userList})", 'planet_id');
$planetList = cleanup_go_int_list($planetIds);

$fleetIds = cleanup_go_ids(
    "SELECT fleet_id FROM {$db_prefix}fleet WHERE owner_id IN ({$userList}) OR start_planet IN ({$planetList}) OR target_planet IN ({$planetList})",
    'fleet_id'
);
$fleetList = cleanup_go_int_list($fleetIds);
$fleetTargetIds = cleanup_go_ids("SELECT target_planet FROM {$db_prefix}fleet WHERE fleet_id IN ({$fleetList})", 'target_planet');
$targetList = cleanup_go_int_list($fleetTargetIds);

dbquery("UPDATE {$db_prefix}uni SET freeze=0, news_until=0 WHERE 1=1");

if (!empty($userIds)) {
    $orPlayers = array();
    foreach ($userIds as $userId) {
        $orPlayers[] = "CONCAT(',', players, ',') LIKE '%," . (int)$userId . ",%'";
    }
    dbquery("DELETE FROM {$db_prefix}union WHERE target_player IN ({$userList}) OR " . implode(' OR ', $orPlayers));
    dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$userList})");
    dbquery("DELETE FROM {$db_prefix}buddy WHERE request_from IN ({$userList}) OR request_to IN ({$userList})");
    dbquery("DELETE FROM {$db_prefix}userlogs WHERE owner_id IN ({$userList})");
    dbquery("DELETE FROM {$db_prefix}template WHERE owner_id IN ({$userList})");
    dbquery("DELETE FROM {$db_prefix}queue WHERE type='" . QTYP_CLEAN_PLAYERS . "' AND sub_id IN ({$userList})");
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id IN ({$userList}) AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id IN ({$userList}) OR planet_id IN ({$planetList})");
    dbquery("UPDATE {$db_prefix}users SET admin=" . USER_TYPE_PLAYER . ", vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0 WHERE player_id IN ({$userList})");
    cleanup_go_reset_home_planets($userIds);
}

if (!empty($allianceIds)) {
    dbquery("UPDATE {$db_prefix}users SET ally_id=0, allyrank=0, joindate=0 WHERE ally_id IN ({$allianceList}) OR player_id IN ({$userList})");
    dbquery("DELETE FROM {$db_prefix}allyapps WHERE ally_id IN ({$allianceList}) OR player_id IN ({$userList})");
    dbquery("DELETE FROM {$db_prefix}allyranks WHERE ally_id IN ({$allianceList})");
    dbquery("DELETE FROM {$db_prefix}ally WHERE ally_id IN ({$allianceList})");
}

if (!empty($fleetIds)) {
    dbquery("DELETE FROM {$db_prefix}queue WHERE type='" . QTYP_FLEET . "' AND (owner_id IN ({$userList}) OR sub_id IN ({$fleetList}))");
    dbquery("DELETE FROM {$db_prefix}fleet WHERE fleet_id IN ({$fleetList})");
}

$specialNames = cleanup_go_quoted_list(array('Colony Slot', 'Deep Space', 'Fleet Colony Slot', 'Fleet Deep Space'));
dbquery(
    "DELETE p FROM {$db_prefix}planets p " .
    "LEFT JOIN {$db_prefix}fleet f ON f.start_planet=p.planet_id OR f.target_planet=p.planet_id " .
    "WHERE p.owner_id=" . USER_SPACE . " AND f.fleet_id IS NULL AND (p.planet_id IN ({$targetList}) OR p.name IN ({$specialNames}))"
);

InvalidateUserCache();

echo json_encode(array(
    'cleaned' => true,
    'users' => count($userIds),
    'alliances' => count($allianceIds),
    'fleets' => count($fleetIds),
    'targets' => count($fleetTargetIds),
), JSON_UNESCAPED_SLASHES) . PHP_EOL;
