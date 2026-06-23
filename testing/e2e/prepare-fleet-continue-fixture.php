<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-fleet-continue-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/prepare-fleet-continue-fixture.php';
$_COOKIE['ogamelang'] = 'en';

chdir('/var/www/html/game');
require_once 'config.php';
require_once 'core/core.php';

InitDB();
$GlobalUni = LoadUniverse();
$GlobalUser = array('player_id' => 0);
$session = '';
ModsInit();

loca_add('common', 'en');
loca_add('fleet', 'en');
loca_add('technames', 'en');

function fleet_continue_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

function fleet_continue_one_row(string $sql): ?array
{
    $res = dbquery($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function fleet_continue_user_by_name(string $name): ?array
{
    global $db_prefix;
    $lower = mb_strtolower($name, 'UTF-8');
    return fleet_continue_one_row("SELECT * FROM {$db_prefix}users WHERE name='" . fleet_continue_sql_escape($lower) . "' LIMIT 1");
}

function fleet_continue_home_planet_exists(int $playerId, int $planetId): bool
{
    global $db_prefix;
    if ($planetId <= 0) {
        return false;
    }
    return fleet_continue_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE planet_id={$planetId} AND owner_id={$playerId} LIMIT 1") !== null;
}

function fleet_continue_prepare_user(string $name, string $password, string $email): array
{
    global $db_prefix, $db_secret;

    $displayName = ucfirst($name);
    $user = fleet_continue_user_by_name($name);
    if ($user === null) {
        $playerId = CreateUser($displayName, $password, $email, false);
        $user = fleet_continue_one_row("SELECT * FROM {$db_prefix}users WHERE player_id={$playerId} LIMIT 1");
        if ($user === null) {
            throw new RuntimeException("Failed to create fixture user {$name}.");
        }
    }

    $playerId = (int)$user['player_id'];
    $homePlanetId = (int)$user['hplanetid'];
    if (!fleet_continue_home_planet_exists($playerId, $homePlanetId)) {
        $homePlanetId = CreateHomePlanet($playerId);
        if ($homePlanetId <= 0) {
            throw new RuntimeException("Failed to create fixture home planet for {$name}.");
        }
    }

    $passwordHash = md5($password . $db_secret);
    dbquery(
        "UPDATE {$db_prefix}users SET " .
        "name='" . fleet_continue_sql_escape(mb_strtolower($name, 'UTF-8')) . "', " .
        "oname='" . fleet_continue_sql_escape($displayName) . "', " .
        "password='" . fleet_continue_sql_escape($passwordHash) . "', " .
        "pemail='" . fleet_continue_sql_escape($email) . "', email='" . fleet_continue_sql_escape($email) . "', " .
        "validated=1, validatemd='', deact_ip=1, admin=" . USER_TYPE_PLAYER . ", " .
        "vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, " .
        "disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, " .
        "hplanetid={$homePlanetId}, aktplanet={$homePlanetId}, lastclick=" . time() . " " .
        "WHERE player_id={$playerId}"
    );

    return array('player_id' => $playerId, 'planet_id' => $homePlanetId, 'name' => $displayName);
}

function fleet_continue_cleanup_fleets(array $userIds, array $planetIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', $userIds));
    $planetList = implode(',', array_map('intval', $planetIds));
    $fleetIds = array();
    $res = dbquery("SELECT fleet_id FROM {$db_prefix}fleet WHERE owner_id IN ({$userList}) OR start_planet IN ({$planetList}) OR target_planet IN ({$planetList})");
    while ($row = dbarray($res)) {
        $fleetIds[] = (int)$row['fleet_id'];
    }
    if (!empty($fleetIds)) {
        $fleetList = implode(',', $fleetIds);
        dbquery("DELETE FROM {$db_prefix}queue WHERE type='" . QTYP_FLEET . "' AND (owner_id IN ({$userList}) OR sub_id IN ({$fleetList}))");
        dbquery("DELETE FROM {$db_prefix}fleet WHERE fleet_id IN ({$fleetList})");
    }
    dbquery("DELETE FROM {$db_prefix}queue WHERE type='" . QTYP_FLEET . "' AND owner_id IN ({$userList})");
}

function fleet_continue_reset_user_and_planet(int $userId, int $planetId, string $planetName, int $position): void
{
    global $db_prefix, $resmap;

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=10";
    }
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", " .
        "admin=0, validated=1, deact_ip=1, vacation=0, vacation_until=0, banned=0, banned_until=0, " .
        "noattack=0, noattack_until=0, disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, " .
        "hplanetid={$planetId}, aktplanet={$planetId} WHERE player_id={$userId}"
    );
    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "name='" . fleet_continue_sql_escape($planetName) . "', g=1, s=471, p={$position}, " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "`" . GID_B_METAL_MINE . "`=0, `" . GID_B_CRYS_MINE . "`=0, `" . GID_B_DEUT_SYNTH . "`=0, `" . GID_B_SOLAR . "`=10, " .
        "`" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=10, `" . GID_B_RES_LAB . "`=10, " .
        "`" . GID_F_SC . "`=10, `" . GID_F_LC . "`=5, `" . GID_F_LF . "`=5, `" . GID_F_PROBE . "`=5, " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=0, maxfields=200, " .
        "type=" . PTYP_PLANET . ", owner_id={$userId} WHERE planet_id={$planetId}"
    );
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$userId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$userId} OR planet_id={$planetId}");
    SelectPlanet($userId, $planetId);
}

function fleet_continue_session(int $userId, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . fleet_continue_sql_escape($session) . "', " .
        "private_session='" . fleet_continue_sql_escape($private) . "' WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookie_name' => 'prsess_' . $userId . '_' . $GlobalUni['num'],
        'cookie_value' => $private,
    );
}

$userName = getenv('OGAME_FLEET_CONTINUE_USER') ?: 'fleetcontinue';
$password = getenv('OGAME_FLEET_CONTINUE_PASS') ?: 'admin';
$targetName = getenv('OGAME_FLEET_CONTINUE_TARGET_USER') ?: 'fleetdest';

$player = fleet_continue_prepare_user($userName, $password, $userName . '@example.local');
$target = fleet_continue_prepare_user($targetName, $password, $targetName . '@example.local');
fleet_continue_cleanup_fleets(array($player['player_id'], $target['player_id']), array($player['planet_id'], $target['planet_id']));
fleet_continue_reset_user_and_planet($player['player_id'], $player['planet_id'], 'Fleet Cont Home', 4);
fleet_continue_reset_user_and_planet($target['player_id'], $target['planet_id'], 'Fleet Cont Target', 5);
$auth = fleet_continue_session($player['player_id'], 'fleet-continue');

echo json_encode(array(
    'user' => $userName,
    'password' => $password,
    'player_id' => $player['player_id'],
    'planet_id' => $player['planet_id'],
    'target_player_id' => $target['player_id'],
    'target_planet_id' => $target['planet_id'],
    'session' => $auth['session'],
    'private_cookie_name' => $auth['cookie_name'],
    'private_cookie_value' => $auth['cookie_value'],
), JSON_UNESCAPED_SLASHES) . PHP_EOL;
