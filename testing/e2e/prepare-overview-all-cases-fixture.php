<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-overview-all-cases-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/prepare-overview-all-cases-fixture.php';
$_COOKIE['ogamelang'] = 'en';

chdir('/var/www/html/game');
require_once 'config.php';
require_once 'core/core.php';

InitDB();
$GlobalUni = LoadUniverse();
$GlobalUser = array('player_id' => 0, 'lang' => 'en');
$session = '';
ModsInit();

loca_add('common', 'en');
loca_add('events', 'en');
loca_add('fleet', 'en');
loca_add('overview', 'en');
loca_add('technames', 'en');

function overview_all_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

function overview_all_one_row(string $sql): ?array
{
    $res = dbquery($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function overview_all_user_by_name(string $name): ?array
{
    global $db_prefix;
    return overview_all_one_row("SELECT * FROM {$db_prefix}users WHERE name='" . overview_all_sql_escape(mb_strtolower($name, 'UTF-8')) . "' LIMIT 1");
}

function overview_all_prepare_user(string $name, string $password, string $email): array
{
    global $db_prefix, $db_secret;

    $displayName = ucfirst($name);
    $user = overview_all_user_by_name($name);
    if ($user === null) {
        $playerId = CreateUser($displayName, $password, $email, false);
        $user = overview_all_one_row("SELECT * FROM {$db_prefix}users WHERE player_id={$playerId} LIMIT 1");
        if ($user === null) {
            throw new RuntimeException("Failed to create fixture user {$name}.");
        }
    }

    $playerId = (int)$user['player_id'];
    $homePlanetId = (int)$user['hplanetid'];
    if ($homePlanetId <= 0 || overview_all_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE planet_id={$homePlanetId} LIMIT 1") === null) {
        $homePlanetId = CreateHomePlanet($playerId);
        if ($homePlanetId <= 0) {
            throw new RuntimeException("Failed to create fixture home planet for {$name}.");
        }
    }

    $passwordHash = md5($password . $db_secret);
    dbquery(
        "UPDATE {$db_prefix}users SET " .
        "name='" . overview_all_sql_escape(mb_strtolower($name, 'UTF-8')) . "', " .
        "oname='" . overview_all_sql_escape($displayName) . "', " .
        "password='" . overview_all_sql_escape($passwordHash) . "', " .
        "pemail='" . overview_all_sql_escape($email) . "', email='" . overview_all_sql_escape($email) . "', " .
        "validated=1, validatemd='', deact_ip=1, admin=" . USER_TYPE_PLAYER . ", " .
        "vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, " .
        "disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, " .
        "hplanetid={$homePlanetId}, aktplanet={$homePlanetId}, lastclick=" . time() . " " .
        "WHERE player_id={$playerId}"
    );

    return array('player_id' => $playerId, 'planet_id' => $homePlanetId, 'name' => $displayName, 'login' => mb_strtolower($name, 'UTF-8'));
}

function overview_all_empty_resources(): array
{
    global $transportableResources;
    $resources = array();
    foreach ($transportableResources as $rc) {
        $resources[$rc] = 0;
    }
    return $resources;
}

function overview_all_empty_fleet(): array
{
    global $fleetmap;
    $fleet = array();
    foreach ($fleetmap as $gid) {
        $fleet[$gid] = 0;
    }
    return $fleet;
}

function overview_all_find_free_position(array &$reserved): array
{
    for ($galaxy = 1; $galaxy <= 9; $galaxy++) {
        for ($system = 499; $system >= 1; $system--) {
            for ($position = 4; $position <= 15; $position++) {
                $key = "{$galaxy}:{$system}:{$position}";
                if (isset($reserved[$key])) {
                    continue;
                }
                if (LoadPlanet($galaxy, $system, $position, 1) === false && LoadPlanet($galaxy, $system, $position, 2) === false && LoadPlanet($galaxy, $system, $position, 3) === false) {
                    $reserved[$key] = true;
                    return array('g' => $galaxy, 's' => $system, 'p' => $position);
                }
            }
        }
    }
    throw new RuntimeException('No free overview all-cases coordinate slot found.');
}

function overview_all_cleanup(array $userIds): void
{
    global $db_prefix;
    $userList = implode(',', array_map('intval', $userIds));
    $orPlayers = array();
    foreach ($userIds as $userId) {
        $orPlayers[] = "CONCAT(',', players, ',') LIKE '%," . (int)$userId . ",%'";
    }
    dbquery("DELETE FROM {$db_prefix}union WHERE target_player IN ({$userList}) OR " . implode(' OR ', $orPlayers));

    $planetIds = array();
    $res = dbquery("SELECT planet_id FROM {$db_prefix}planets WHERE owner_id IN ({$userList})");
    while ($row = dbarray($res)) {
        $planetIds[] = (int)$row['planet_id'];
    }
    $planetList = empty($planetIds) ? '0' : implode(',', $planetIds);
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
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id IN ({$userList}) AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id IN ({$userList}) OR planet_id IN ({$planetList})");
    dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$userList})");

    $homePlanetIds = array();
    $res = dbquery("SELECT hplanetid FROM {$db_prefix}users WHERE player_id IN ({$userList})");
    while ($row = dbarray($res)) {
        $homePlanetIds[] = (int)$row['hplanetid'];
    }
    $homePlanetList = empty($homePlanetIds) ? '0' : implode(',', $homePlanetIds);
    dbquery("DELETE FROM {$db_prefix}planets WHERE owner_id IN ({$userList}) AND planet_id NOT IN ({$homePlanetList})");
    dbquery("DELETE FROM {$db_prefix}planets WHERE owner_id=" . USER_SPACE . " AND name IN ('Colony Slot', 'Deep Space')");
}

function overview_all_add_build_queue(int $ownerId, int $planetId, int $techId, int $level, int $duration): int
{
    $now = time();
    $subId = AddDBRow(array(
        'owner_id' => $ownerId,
        'planet_id' => $planetId,
        'list_id' => 1,
        'tech_id' => $techId,
        'level' => $level,
        'destroy' => 0,
        'start' => $now,
        'end' => $now + $duration,
    ), 'buildqueue');
    AddQueue($ownerId, QTYP_BUILD, $subId, $techId, $level, $now, $duration, QUEUE_PRIO_BUILD);
    return $subId;
}

function overview_all_session(int $userId, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . overview_all_sql_escape($session) . "', " .
        "private_session='" . overview_all_sql_escape($private) . "' WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookie_name' => 'prsess_' . $userId . '_' . $GlobalUni['num'],
        'cookie_value' => $private,
    );
}

function overview_all_create_owned_planet(int $ownerId, string $name, array $coords): int
{
    global $db_prefix;
    $id = CreatePlanet((int)$coords['g'], (int)$coords['s'], (int)$coords['p'], $ownerId, 1, 0, 0, time());
    if ($id <= 0) {
        $row = overview_all_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE g=" . (int)$coords['g'] . " AND s=" . (int)$coords['s'] . " AND p=" . (int)$coords['p'] . " AND type=" . PTYP_PLANET . " LIMIT 1");
        $id = $row === null ? 0 : (int)$row['planet_id'];
    }
    if ($id <= 0) {
        throw new RuntimeException("Failed to create fixture planet {$name}.");
    }
    overview_all_prepare_planet($id, $ownerId, $name, $coords);
    return $id;
}

function overview_all_prepare_planet(int $planetId, int $ownerId, string $name, array $coords, int $type = PTYP_PLANET): void
{
    global $db_prefix, $fleetmap, $resmap;

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=10";
    }
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", " .
        "`" . GID_R_EXPEDITION . "`=9, `" . GID_R_COMPUTER . "`=12, `" . GID_R_ESPIONAGE . "`=12, " .
        "hplanetid={$planetId}, aktplanet={$planetId} WHERE player_id={$ownerId}"
    );

    $ships = array();
    foreach ($fleetmap as $gid) {
        $ships[] = "`{$gid}`=20";
    }
    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "name='" . overview_all_sql_escape($name) . "', type={$type}, owner_id={$ownerId}, " .
        "g=" . (int)$coords['g'] . ", s=" . (int)$coords['s'] . ", p=" . (int)$coords['p'] . ", " .
        "diameter=12800, temp=24, fields=8, maxfields=220, " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "`" . GID_B_METAL_MINE . "`=12, `" . GID_B_CRYS_MINE . "`=11, `" . GID_B_DEUT_SYNTH . "`=10, `" . GID_B_SOLAR . "`=14, " .
        "`" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=10, `" . GID_B_RES_LAB . "`=10, `" . GID_B_MISS_SILO . "`=6, " .
        "`" . GID_D_IPM . "`=20, `" . GID_D_ABM . "`=20, " .
        implode(',', $ships) . ", " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, remove=0 WHERE planet_id={$planetId}"
    );
}

function overview_all_create_moon(int $ownerId, string $name, array $coords): int
{
    global $db_prefix;
    $id = CreatePlanet((int)$coords['g'], (int)$coords['s'], (int)$coords['p'], $ownerId, 1, 1, 20, time());
    if ($id <= 0) {
        $row = overview_all_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE g=" . (int)$coords['g'] . " AND s=" . (int)$coords['s'] . " AND p=" . (int)$coords['p'] . " AND type=" . PTYP_MOON . " LIMIT 1");
        $id = $row === null ? 0 : (int)$row['planet_id'];
    }
    if ($id <= 0) {
        throw new RuntimeException("Failed to create fixture moon {$name}.");
    }
    overview_all_prepare_planet($id, $ownerId, $name, $coords, PTYP_MOON);
    return $id;
}

function overview_all_create_special_target(string $name, array $coords, int $type): int
{
    global $db_prefix;
    $row = overview_all_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE g=" . (int)$coords['g'] . " AND s=" . (int)$coords['s'] . " AND p=" . (int)$coords['p'] . " AND type={$type} LIMIT 1");
    if ($row !== null) {
        $id = (int)$row['planet_id'];
        dbquery("UPDATE {$db_prefix}planets SET name='" . overview_all_sql_escape($name) . "', owner_id=" . USER_SPACE . ", remove=0 WHERE planet_id={$id}");
        return $id;
    }
    return AddDBRow(array(
        'name' => $name,
        'type' => $type,
        'g' => (int)$coords['g'],
        's' => (int)$coords['s'],
        'p' => (int)$coords['p'],
        'owner_id' => USER_SPACE,
        'diameter' => 0,
        'temp' => 0,
        'fields' => 0,
        'maxfields' => 0,
        'date' => time(),
        GID_RC_METAL => 0,
        GID_RC_CRYSTAL => 0,
        GID_RC_DEUTERIUM => 0,
        'lastpeek' => time(),
        'lastakt' => time(),
        'gate_until' => 0,
        'remove' => 0,
    ), 'planets');
}

$userName = getenv('OGAME_OVERVIEW_ALL_USER') ?: 'overviewall';
$password = getenv('OGAME_OVERVIEW_ALL_PASS') ?: 'admin';
$enemyName = getenv('OGAME_OVERVIEW_ALL_ENEMY_USER') ?: 'overviewenemy';
$supportName = getenv('OGAME_OVERVIEW_ALL_SUPPORT_USER') ?: 'overviewsupport';

$main = overview_all_prepare_user($userName, $password, $userName . '@example.local');
$enemy = overview_all_prepare_user($enemyName, $password, $enemyName . '@example.local');
$support = overview_all_prepare_user($supportName, $password, $supportName . '@example.local');
overview_all_cleanup(array($main['player_id'], $enemy['player_id'], $support['player_id']));
dbquery("UPDATE {$db_prefix}uni SET freeze=0, news_until=0");
$GlobalUni = LoadUniverse();

$reserved = array();
$homeCoords = overview_all_find_free_position($reserved);
$enemyCoords = overview_all_find_free_position($reserved);
$supportCoords = overview_all_find_free_position($reserved);
$colonyCoords = overview_all_find_free_position($reserved);
$phantomCoords = overview_all_find_free_position($reserved);
$farspaceCoords = array('g' => $homeCoords['g'], 's' => $homeCoords['s'], 'p' => 16);

overview_all_prepare_planet($main['planet_id'], $main['player_id'], 'Overview Home', $homeCoords);
overview_all_prepare_planet($enemy['planet_id'], $enemy['player_id'], 'Overview Target', $enemyCoords);
overview_all_prepare_planet($support['planet_id'], $support['player_id'], 'Overview Support', $supportCoords);
$colonyId = overview_all_create_owned_planet($main['player_id'], 'Overview Colony', $colonyCoords);
$moonId = overview_all_create_moon($main['player_id'], 'Overview Moon', $homeCoords);
$enemyMoonId = overview_all_create_moon($enemy['player_id'], 'Target Moon', $enemyCoords);
$phantomId = overview_all_create_special_target('Colony Slot', $phantomCoords, PTYP_COLONY_PHANTOM);
$farspaceId = overview_all_create_special_target('Deep Space', $farspaceCoords, PTYP_FARSPACE);
$debrisId = CreateDebris((int)$enemyCoords['g'], (int)$enemyCoords['s'], (int)$enemyCoords['p'], USER_SPACE);
AddDebris($debrisId, 120000, 80000);

$now = time();
overview_all_add_build_queue($main['player_id'], $main['planet_id'], GID_B_METAL_MINE, 13, 3600);
overview_all_add_build_queue($main['player_id'], $colonyId, GID_B_CRYS_MINE, 12, 5400);
SendMessage($main['player_id'], 'Overview QA', 'overview unread one', 'overview unread one', MTYP_MISC, $now);
SendMessage($main['player_id'], 'Overview QA', 'overview unread two', 'overview unread two', MTYP_MISC, $now + 1);

$resources = overview_all_empty_resources();
$transportResources = $resources;
$transportResources[GID_RC_METAL] = 12345;
$transportResources[GID_RC_CRYSTAL] = 678;
$transportResources[GID_RC_DEUTERIUM] = 9;

$smallCargo = overview_all_empty_fleet();
$smallCargo[GID_F_SC] = 1;
$fighter = overview_all_empty_fleet();
$fighter[GID_F_LF] = 2;
$probe = overview_all_empty_fleet();
$probe[GID_F_PROBE] = 3;
$colonyShip = overview_all_empty_fleet();
$colonyShip[GID_F_COLON] = 1;
$recycler = overview_all_empty_fleet();
$recycler[GID_F_RECYCLER] = 1;
$deathstar = overview_all_empty_fleet();
$deathstar[GID_F_DEATHSTAR] = 1;

$origin = LoadPlanetById($main['planet_id']);
$target = LoadPlanetById($enemy['planet_id']);
$supportOrigin = LoadPlanetById($support['planet_id']);
$enemyOrigin = LoadPlanetById($enemy['planet_id']);
$homeTarget = LoadPlanetById($main['planet_id']);
$colonyTarget = LoadPlanetById($colonyId);
$enemyMoon = LoadPlanetById($enemyMoonId);
$phantomTarget = LoadPlanetById($phantomId);
$farspaceTarget = LoadPlanetById($farspaceId);
$debrisTarget = LoadPlanetById($debrisId);

$fleetIds = array();
$fleetIds['own_attack'] = DispatchFleet($fighter, $origin, $target, FTYP_ATTACK, 7200, $resources, 0, $now);
$fleetIds['own_transport'] = DispatchFleet($smallCargo, $origin, $target, FTYP_TRANSPORT, 7500, $transportResources, 0, $now + 1);
$fleetIds['own_deploy'] = DispatchFleet($smallCargo, $origin, $colonyTarget, FTYP_DEPLOY, 7800, $resources, 0, $now + 2);
$fleetIds['own_hold'] = DispatchFleet($fighter, $origin, $target, FTYP_ACS_HOLD, 8100, $resources, 0, $now + 3, 0, 3600);
$fleetIds['own_spy'] = DispatchFleet($probe, $origin, $target, FTYP_SPY, 8400, $resources, 0, $now + 4);
$fleetIds['own_colonize'] = DispatchFleet($colonyShip, $origin, $phantomTarget, FTYP_COLONIZE, 8700, $resources, 0, $now + 5);
$fleetIds['own_recycle'] = DispatchFleet($recycler, $origin, $debrisTarget, FTYP_RECYCLE, 9000, $resources, 0, $now + 6);
$fleetIds['own_destroy'] = DispatchFleet($deathstar, $origin, $enemyMoon, FTYP_DESTROY, 9300, $resources, 0, $now + 7);
$fleetIds['own_expedition'] = DispatchFleet($smallCargo, $origin, $farspaceTarget, FTYP_EXPEDITION, 9600, $resources, 0, $now + 8, 0, 3600);
$fleetIds['enemy_attack'] = DispatchFleet($fighter, $enemyOrigin, $homeTarget, FTYP_ATTACK, 9900, $resources, 0, $now + 9);
$fleetIds['enemy_transport'] = DispatchFleet($smallCargo, $enemyOrigin, $homeTarget, FTYP_TRANSPORT, 10200, $resources, 0, $now + 10);
$fleetIds['enemy_spy'] = DispatchFleet($probe, $enemyOrigin, $homeTarget, FTYP_SPY, 10500, $resources, 0, $now + 11);
$fleetIds['enemy_destroy'] = DispatchFleet($deathstar, $enemyOrigin, LoadPlanetById($moonId), FTYP_DESTROY, 10800, $resources, 0, $now + 12);
$fleetIds['support_hold'] = DispatchFleet($fighter, $supportOrigin, $homeTarget, FTYP_ACS_HOLD, 11100, $resources, 0, $now + 13, 0, 3600);
$fleetIds['missile'] = DispatchFleet(overview_all_empty_fleet(), $origin, $target, FTYP_MISSILE, 11400, $resources, 0, $now + 14);

if ($fleetIds['missile'] > 0) {
    global $db_prefix;
    dbquery("UPDATE {$db_prefix}fleet SET ipm_amount=3, ipm_target=" . GID_D_RL . " WHERE fleet_id=" . (int)$fleetIds['missile']);
}

$headFleetId = DispatchFleet($fighter, $origin, $target, FTYP_ATTACK, 11700, $resources, 0, $now + 15);
$unionId = $headFleetId > 0 ? CreateUnion($headFleetId, 'OVALL' . substr(md5((string)microtime(true)), 0, 6)) : 0;
$GlobalUser = LoadUser($main['player_id']);
if ($unionId > 0) {
    AddUnionMember($unionId, $support['login']);
    $fleetIds['acs_support'] = DispatchFleet($fighter, $supportOrigin, $target, FTYP_ACS_ATTACK, 11700, $resources, 0, $now + 16, $unionId);
}
$fleetIds['acs_head'] = $headFleetId;

foreach ($fleetIds as $name => $id) {
    if ($id <= 0) {
        throw new RuntimeException("Failed to dispatch overview all-cases fleet {$name}.");
    }
}

global $db_prefix;
dbquery(
    "UPDATE {$db_prefix}uni SET " .
    "freeze=1, news1='Overview QA notice', news2='All overview cases are active', news_until=" . ($now + 3600) . ", " .
    "ext_board='https://example.local/board', ext_discord='https://example.local/discord'"
);
dbquery(
    "UPDATE {$db_prefix}users SET admin=" . USER_TYPE_ADMIN . ", vacation=1, vacation_until=" . ($now + 86400) . ", " .
    "aktplanet=" . (int)$main['planet_id'] . " WHERE player_id=" . (int)$main['player_id']
);
InvalidateUserCache();
$GlobalUni = LoadUniverse();

$auth = overview_all_session($main['player_id'], 'overview-all-cases');

echo json_encode(array(
    'user' => $userName,
    'password' => $password,
    'player_id' => $main['player_id'],
    'planet_id' => $main['planet_id'],
    'moon_id' => $moonId,
    'colony_id' => $colonyId,
    'enemy_player_id' => $enemy['player_id'],
    'enemy_planet_id' => $enemy['planet_id'],
    'support_player_id' => $support['player_id'],
    'support_planet_id' => $support['planet_id'],
    'debris_id' => $debrisId,
    'phantom_id' => $phantomId,
    'farspace_id' => $farspaceId,
    'union_id' => $unionId,
    'fleet_ids' => $fleetIds,
    'session' => $auth['session'],
    'private_cookie_name' => $auth['cookie_name'],
    'private_cookie_value' => $auth['cookie_value'],
), JSON_UNESCAPED_SLASHES) . PHP_EOL;
