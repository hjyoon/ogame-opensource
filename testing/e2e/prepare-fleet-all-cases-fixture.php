<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-fleet-all-cases-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/prepare-fleet-all-cases-fixture.php';
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
loca_add('fleet', 'en');
loca_add('technames', 'en');
loca_add('union', 'en');

function fleet_all_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

function fleet_all_one_row(string $sql): ?array
{
    $res = dbquery($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function fleet_all_user_by_name(string $name): ?array
{
    global $db_prefix;
    return fleet_all_one_row("SELECT * FROM {$db_prefix}users WHERE name='" . fleet_all_sql_escape(mb_strtolower($name, 'UTF-8')) . "' LIMIT 1");
}

function fleet_all_prepare_user(string $name, string $password, string $email): array
{
    global $db_prefix, $db_secret;

    $displayName = ucfirst($name);
    $user = fleet_all_user_by_name($name);
    if ($user === null) {
        $playerId = CreateUser($displayName, $password, $email, false);
        $user = fleet_all_one_row("SELECT * FROM {$db_prefix}users WHERE player_id={$playerId} LIMIT 1");
        if ($user === null) {
            throw new RuntimeException("Failed to create fixture user {$name}.");
        }
    }

    $playerId = (int)$user['player_id'];
    $homePlanetId = (int)$user['hplanetid'];
    if ($homePlanetId <= 0 || fleet_all_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE planet_id={$homePlanetId} LIMIT 1") === null) {
        $homePlanetId = CreateHomePlanet($playerId);
        if ($homePlanetId <= 0) {
            throw new RuntimeException("Failed to create fixture home planet for {$name}.");
        }
    }

    $passwordHash = md5($password . $db_secret);
    dbquery(
        "UPDATE {$db_prefix}users SET " .
        "name='" . fleet_all_sql_escape(mb_strtolower($name, 'UTF-8')) . "', " .
        "oname='" . fleet_all_sql_escape($displayName) . "', " .
        "password='" . fleet_all_sql_escape($passwordHash) . "', " .
        "pemail='" . fleet_all_sql_escape($email) . "', email='" . fleet_all_sql_escape($email) . "', " .
        "validated=1, validatemd='', deact_ip=1, admin=" . USER_TYPE_PLAYER . ", " .
        "vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, " .
        "disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, " .
        "hplanetid={$homePlanetId}, aktplanet={$homePlanetId}, lastclick=" . time() . " " .
        "WHERE player_id={$playerId}"
    );

    return array('player_id' => $playerId, 'planet_id' => $homePlanetId, 'name' => $displayName, 'login' => mb_strtolower($name, 'UTF-8'));
}

function fleet_all_empty_resources(): array
{
    global $transportableResources;
    $resources = array();
    foreach ($transportableResources as $rc) {
        $resources[$rc] = 0;
    }
    return $resources;
}

function fleet_all_empty_fleet(): array
{
    global $fleetmap;
    $fleet = array();
    foreach ($fleetmap as $gid) {
        $fleet[$gid] = 0;
    }
    return $fleet;
}

function fleet_all_find_free_position(array &$reserved): array
{
    for ($galaxy = 1; $galaxy <= 9; $galaxy++) {
        for ($system = 471; $system <= 499; $system++) {
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
    throw new RuntimeException('No free fleet all-cases coordinate slot found.');
}

function fleet_all_cleanup(array $userIds): void
{
    global $db_prefix;
    $userList = implode(',', array_map('intval', $userIds));
    $orPlayers = array();
    foreach ($userIds as $userId) {
        $orPlayers[] = "CONCAT(',', players, ',') LIKE '%," . (int)$userId . ",%'";
    }
    dbquery("DELETE FROM {$db_prefix}union WHERE target_player IN ({$userList}) OR " . implode(' OR ', $orPlayers));
    dbquery("DELETE FROM {$db_prefix}template WHERE owner_id IN ({$userList})");
    dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$userList})");

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

    $homePlanetIds = array();
    $res = dbquery("SELECT hplanetid FROM {$db_prefix}users WHERE player_id IN ({$userList})");
    while ($row = dbarray($res)) {
        $homePlanetIds[] = (int)$row['hplanetid'];
    }
    $homePlanetList = empty($homePlanetIds) ? '0' : implode(',', $homePlanetIds);
    dbquery("DELETE FROM {$db_prefix}planets WHERE owner_id IN ({$userList}) AND planet_id NOT IN ({$homePlanetList})");
    dbquery("DELETE FROM {$db_prefix}planets WHERE owner_id=" . USER_SPACE . " AND name IN ('Fleet Colony Slot', 'Fleet Deep Space')");
}

function fleet_all_prepare_user_research(int $userId, int $activePlanetId, bool $premium): void
{
    global $db_prefix, $resmap;
    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=12";
    }
    $until = $premium ? time() + 86400 : 0;
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", " .
        "`" . GID_R_EXPEDITION . "`=9, `" . GID_R_COMPUTER . "`=12, `" . GID_R_ESPIONAGE . "`=12, " .
        "com_until={$until}, adm_until={$until}, vacation=0, vacation_until=0, banned=0, banned_until=0, " .
        "hplanetid={$activePlanetId}, aktplanet={$activePlanetId} WHERE player_id={$userId}"
    );
}

function fleet_all_prepare_planet(int $planetId, int $ownerId, string $name, array $coords, int $type = PTYP_PLANET): void
{
    global $db_prefix, $fleetmap;
    $ships = array();
    foreach ($fleetmap as $gid) {
        $ships[] = "`{$gid}`=20";
    }
    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "name='" . fleet_all_sql_escape($name) . "', type={$type}, owner_id={$ownerId}, " .
        "g=" . (int)$coords['g'] . ", s=" . (int)$coords['s'] . ", p=" . (int)$coords['p'] . ", " .
        "diameter=12800, temp=24, fields=8, maxfields=220, " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "`" . GID_B_METAL_MINE . "`=12, `" . GID_B_CRYS_MINE . "`=11, `" . GID_B_DEUT_SYNTH . "`=10, `" . GID_B_SOLAR . "`=14, " .
        "`" . GID_B_ROBOTS . "`=12, `" . GID_B_SHIPYARD . "`=12, `" . GID_B_RES_LAB . "`=10, " .
        implode(',', $ships) . ", " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, remove=0 WHERE planet_id={$planetId}"
    );
}

function fleet_all_create_owned_planet(int $ownerId, string $name, array $coords): int
{
    global $db_prefix;
    $id = CreatePlanet((int)$coords['g'], (int)$coords['s'], (int)$coords['p'], $ownerId, 1, 0, 0, time());
    if ($id <= 0) {
        $row = fleet_all_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE g=" . (int)$coords['g'] . " AND s=" . (int)$coords['s'] . " AND p=" . (int)$coords['p'] . " AND type=" . PTYP_PLANET . " LIMIT 1");
        $id = $row === null ? 0 : (int)$row['planet_id'];
    }
    if ($id <= 0) {
        throw new RuntimeException("Failed to create fixture planet {$name}.");
    }
    fleet_all_prepare_planet($id, $ownerId, $name, $coords);
    return $id;
}

function fleet_all_create_moon(int $ownerId, string $name, array $coords): int
{
    global $db_prefix;
    $id = CreatePlanet((int)$coords['g'], (int)$coords['s'], (int)$coords['p'], $ownerId, 1, 1, 20, time());
    if ($id <= 0) {
        $row = fleet_all_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE g=" . (int)$coords['g'] . " AND s=" . (int)$coords['s'] . " AND p=" . (int)$coords['p'] . " AND type=" . PTYP_MOON . " LIMIT 1");
        $id = $row === null ? 0 : (int)$row['planet_id'];
    }
    if ($id <= 0) {
        throw new RuntimeException("Failed to create fixture moon {$name}.");
    }
    fleet_all_prepare_planet($id, $ownerId, $name, $coords, PTYP_MOON);
    return $id;
}

function fleet_all_create_special_target(string $name, array $coords, int $type): int
{
    global $db_prefix;
    $row = fleet_all_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE g=" . (int)$coords['g'] . " AND s=" . (int)$coords['s'] . " AND p=" . (int)$coords['p'] . " AND type={$type} LIMIT 1");
    if ($row !== null) {
        $id = (int)$row['planet_id'];
        dbquery("UPDATE {$db_prefix}planets SET name='" . fleet_all_sql_escape($name) . "', owner_id=" . USER_SPACE . ", remove=0 WHERE planet_id={$id}");
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

function fleet_all_insert_template(int $ownerId, string $name, array $ships): int
{
    global $fleetmap;
    $row = array('owner_id' => $ownerId, 'name' => $name, 'date' => time());
    foreach ($fleetmap as $gid) {
        $row[$gid] = (int)($ships[$gid] ?? 0);
    }
    return AddDBRow($row, 'template');
}

function fleet_all_session(int $userId, string $label): array
{
    global $db_prefix, $GlobalUni;
    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . fleet_all_sql_escape($session) . "', " .
        "private_session='" . fleet_all_sql_escape($private) . "' WHERE player_id={$userId}"
    );
    InvalidateUserCache();
    return array(
        'session' => $session,
        'private' => $private,
        'cookie_name' => 'prsess_' . $userId . '_' . $GlobalUni['num'],
        'cookie_value' => $private,
    );
}

$userName = getenv('OGAME_FLEET_ALL_USER') ?: 'fleetall';
$password = getenv('OGAME_FLEET_ALL_PASS') ?: 'admin';
$enemyName = getenv('OGAME_FLEET_ALL_ENEMY_USER') ?: 'fleetallenemy';
$supportName = getenv('OGAME_FLEET_ALL_SUPPORT_USER') ?: 'fleetallsupport';

$main = fleet_all_prepare_user($userName, $password, $userName . '@example.local');
$enemy = fleet_all_prepare_user($enemyName, $password, $enemyName . '@example.local');
$support = fleet_all_prepare_user($supportName, $password, $supportName . '@example.local');
fleet_all_cleanup(array($main['player_id'], $enemy['player_id'], $support['player_id']));
dbquery("UPDATE {$db_prefix}uni SET freeze=0, acs=3, news_until=0");
$GlobalUni = LoadUniverse();

$reserved = array();
$homeCoords = fleet_all_find_free_position($reserved);
$enemyCoords = fleet_all_find_free_position($reserved);
$supportCoords = fleet_all_find_free_position($reserved);
$colonyCoords = fleet_all_find_free_position($reserved);
$emptyCoords = fleet_all_find_free_position($reserved);
$farspaceCoords = array('g' => $homeCoords['g'], 's' => $homeCoords['s'], 'p' => 16);

fleet_all_prepare_planet($main['planet_id'], $main['player_id'], 'Fleet Home', $homeCoords);
fleet_all_prepare_planet($enemy['planet_id'], $enemy['player_id'], 'Fleet Target', $enemyCoords);
fleet_all_prepare_planet($support['planet_id'], $support['player_id'], 'Fleet Support', $supportCoords);
$colonyId = fleet_all_create_owned_planet($main['player_id'], 'Fleet Colony', $colonyCoords);
$enemyMoonId = fleet_all_create_moon($enemy['player_id'], 'Fleet Moon', $enemyCoords);
$phantomId = fleet_all_create_special_target('Fleet Colony Slot', $emptyCoords, PTYP_COLONY_PHANTOM);
$farspaceId = fleet_all_create_special_target('Fleet Deep Space', $farspaceCoords, PTYP_FARSPACE);
$debrisId = CreateDebris((int)$enemyCoords['g'], (int)$enemyCoords['s'], (int)$enemyCoords['p'], USER_SPACE);
AddDebris($debrisId, 120000, 80000);

fleet_all_prepare_user_research($main['player_id'], $main['planet_id'], true);
fleet_all_prepare_user_research($enemy['player_id'], $enemy['planet_id'], false);
fleet_all_prepare_user_research($support['player_id'], $support['planet_id'], false);

$templateA = fleet_all_empty_fleet();
$templateA[GID_F_SC] = 2;
$templateA[GID_F_LF] = 3;
$templateB = fleet_all_empty_fleet();
$templateB[GID_F_RECYCLER] = 1;
$templateB[GID_F_PROBE] = 4;
fleet_all_insert_template($main['player_id'], 'Raid Pair', $templateA);
fleet_all_insert_template($main['player_id'], 'Probe Sweep', $templateB);

$resources = fleet_all_empty_resources();
$transportResources = $resources;
$transportResources[GID_RC_METAL] = 12345;
$transportResources[GID_RC_CRYSTAL] = 678;
$transportResources[GID_RC_DEUTERIUM] = 9;

$smallCargo = fleet_all_empty_fleet();
$smallCargo[GID_F_SC] = 1;
$fighter = fleet_all_empty_fleet();
$fighter[GID_F_LF] = 2;
$probe = fleet_all_empty_fleet();
$probe[GID_F_PROBE] = 3;
$recycler = fleet_all_empty_fleet();
$recycler[GID_F_RECYCLER] = 1;
$deathstar = fleet_all_empty_fleet();
$deathstar[GID_F_DEATHSTAR] = 1;

$origin = LoadPlanetById($main['planet_id']);
$target = LoadPlanetById($enemy['planet_id']);
$supportOrigin = LoadPlanetById($support['planet_id']);
$colonyTarget = LoadPlanetById($colonyId);
$enemyMoon = LoadPlanetById($enemyMoonId);
$farspaceTarget = LoadPlanetById($farspaceId);
$debrisTarget = LoadPlanetById($debrisId);

$now = time();
$fleetIds = array();
$fleetIds['own_transport'] = DispatchFleet($smallCargo, $origin, $target, FTYP_TRANSPORT, 7500, $transportResources, 0, $now);
$fleetIds['own_deploy'] = DispatchFleet($smallCargo, $origin, $colonyTarget, FTYP_DEPLOY, 7800, $resources, 0, $now + 1);
$fleetIds['own_spy'] = DispatchFleet($probe, $origin, $target, FTYP_SPY, 8100, $resources, 0, $now + 2);
$fleetIds['own_recycle'] = DispatchFleet($recycler, $origin, $debrisTarget, FTYP_RECYCLE, 8400, $resources, 0, $now + 3);
$fleetIds['own_destroy'] = DispatchFleet($deathstar, $origin, $enemyMoon, FTYP_DESTROY, 8700, $resources, 0, $now + 4);
$fleetIds['own_expedition'] = DispatchFleet($smallCargo, $origin, $farspaceTarget, FTYP_EXPEDITION, 9000, $resources, 0, $now + 5, 0, 3600);

$headFleetId = DispatchFleet($fighter, $origin, $target, FTYP_ATTACK, 9300, $resources, 0, $now + 6);
$GlobalUser = LoadUser($main['player_id']);
$unionId = $headFleetId > 0 ? CreateUnion($headFleetId, 'FLEETQA') : 0;
if ($unionId > 0) {
    AddUnionMember($unionId, $support['login']);
}
$fleetIds['acs_head'] = $headFleetId;

foreach ($fleetIds as $name => $id) {
    if ($id <= 0) {
        throw new RuntimeException("Failed to dispatch fleet all-cases fleet {$name}.");
    }
}

$auth = fleet_all_session($main['player_id'], 'fleet-all-cases');

echo json_encode(array(
    'user' => $userName,
    'password' => $password,
    'player_id' => $main['player_id'],
    'planet_id' => $main['planet_id'],
    'colony_id' => $colonyId,
    'enemy_player_id' => $enemy['player_id'],
    'enemy_planet_id' => $enemy['planet_id'],
    'enemy_moon_id' => $enemyMoonId,
    'support_player_id' => $support['player_id'],
    'support_planet_id' => $support['planet_id'],
    'debris_id' => $debrisId,
    'phantom_id' => $phantomId,
    'farspace_id' => $farspaceId,
    'union_id' => $unionId,
    'fleet_ids' => $fleetIds,
    'coordinates' => array(
        'home' => $homeCoords,
        'enemy' => $enemyCoords,
        'colony' => $colonyCoords,
        'moon' => $enemyCoords,
        'debris' => $enemyCoords,
        'farspace' => $farspaceCoords,
        'empty' => $emptyCoords,
    ),
    'session' => $auth['session'],
    'private_cookie_name' => $auth['cookie_name'],
    'private_cookie_value' => $auth['cookie_value'],
), JSON_UNESCAPED_SLASHES) . PHP_EOL;
