<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-go-smoke-fixture';
$_SERVER['REQUEST_URI'] = '/testing/e2e/prepare-golang-smoke-fixture.php';
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

function smoke_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

function smoke_one_row(string $sql): ?array
{
    $res = dbquery($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function smoke_user_by_name(string $name): ?array
{
    global $db_prefix;
    $lower = mb_strtolower($name, 'UTF-8');
    return smoke_one_row("SELECT * FROM {$db_prefix}users WHERE name='" . smoke_sql_escape($lower) . "' LIMIT 1");
}

function smoke_home_planet_exists(int $playerId, int $planetId): bool
{
    global $db_prefix;
    if ($planetId <= 0) {
        return false;
    }
    $row = smoke_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE planet_id={$planetId} AND owner_id={$playerId} LIMIT 1");
    return $row !== null;
}

function smoke_prepare_user(string $name, string $password, string $email, int $adminLevel): array
{
    global $db_prefix, $db_secret;

    $displayName = $name === mb_strtolower($name, 'UTF-8') ? ucfirst($name) : $name;
    $user = smoke_user_by_name($name);
    if ($user === null) {
        $playerId = CreateUser($displayName, $password, $email, false);
        $user = smoke_one_row("SELECT * FROM {$db_prefix}users WHERE player_id={$playerId} LIMIT 1");
        if ($user === null) {
            throw new RuntimeException('Failed to create Go smoke user ' . $name . '.');
        }
    }

    $playerId = (int)$user['player_id'];
    $homePlanetId = (int)$user['hplanetid'];
    if (!smoke_home_planet_exists($playerId, $homePlanetId)) {
        $homePlanetId = CreateHomePlanet($playerId);
        if ($homePlanetId <= 0) {
            throw new RuntimeException('Failed to create Go smoke home planet for ' . $name . '.');
        }
    }

    $passwordHash = md5($password . $db_secret);
    dbquery(
        "UPDATE {$db_prefix}users SET " .
        "name='" . smoke_sql_escape(mb_strtolower($name, 'UTF-8')) . "', " .
        "oname='" . smoke_sql_escape($displayName) . "', " .
        "password='" . smoke_sql_escape($passwordHash) . "', " .
        "pemail='" . smoke_sql_escape($email) . "', email='" . smoke_sql_escape($email) . "', " .
        "validated=1, validatemd='', deact_ip=1, admin={$adminLevel}, " .
        "vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, " .
        "lang='en', skin='/evolution/', useskin=1, hplanetid={$homePlanetId}, aktplanet={$homePlanetId}, " .
        "lastclick=" . time() . " WHERE player_id={$playerId}"
    );
    InvalidateUserCache();

    return array('player_id' => $playerId, 'name' => $displayName, 'home_planet_id' => $homePlanetId);
}

function smoke_cleanup_fleets(array $userIds, array $planetIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', array_filter($userIds, fn($id) => $id > 0)));
    $planetList = implode(',', array_map('intval', array_filter($planetIds, fn($id) => $id > 0)));
    if ($userList === '' || $planetList === '') {
        return;
    }

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

function smoke_cleanup_alliances(array $userIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', array_filter($userIds, fn($id) => $id > 0)));
    if ($userList === '') {
        return;
    }

    $allyIds = array();
    $res = dbquery("SELECT DISTINCT ally_id FROM {$db_prefix}users WHERE player_id IN ({$userList}) AND ally_id > 0");
    while ($row = dbarray($res)) {
        $allyIds[] = (int)$row['ally_id'];
    }
    $res = dbquery("SELECT ally_id FROM {$db_prefix}ally WHERE tag LIKE 'GOSM%' OR name LIKE 'Go smoke alliance%'");
    while ($row = dbarray($res)) {
        $allyIds[] = (int)$row['ally_id'];
    }
    foreach (array_unique($allyIds) as $allyId) {
        DismissAlly($allyId);
    }

    dbquery("DELETE FROM {$db_prefix}allyapps WHERE player_id IN ({$userList})");
    dbquery("UPDATE {$db_prefix}users SET ally_id=0, allyrank=0, joindate=0 WHERE player_id IN ({$userList})");
}

function smoke_find_empty_position(array $near): array
{
    $g = (int)$near['g'];
    $system = (int)$near['s'];
    for ($p = 1; $p <= 15; $p++) {
        if ($p === (int)$near['p']) {
            continue;
        }
        if (!HasPlanet($g, $system, $p)) {
            return array($g, $system, $p);
        }
    }
    throw new RuntimeException('No empty same-system phalanx fixture position found.');
}

function smoke_prepare_planet(int $planetId, int $ownerId, string $name, array $coords): void
{
    global $db_prefix, $buildmap, $fleetmap;

    [$g, $s, $p] = $coords;
    $buildings = array();
    foreach ($buildmap as $gid) {
        $buildings[] = "`{$gid}`=0";
    }
    $ships = array();
    foreach ($fleetmap as $gid) {
        $ships[] = "`{$gid}`=" . ($gid === GID_F_SC ? 3 : 0);
    }
    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "name='" . smoke_sql_escape($name) . "', g={$g}, s={$s}, p={$p}, type=" . PTYP_PLANET . ", owner_id={$ownerId}, " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000, " .
        implode(',', $buildings) . ", " . implode(',', $ships) . ", " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=0, maxfields=200, lastpeek=" . time() . " " .
        "WHERE planet_id={$planetId}"
    );
}

function smoke_prepare_moon(int $homePlanetId, int $ownerId): int
{
    global $db_prefix, $buildmap, $fleetmap;

    $home = LoadPlanetById($homePlanetId);
    if ($home === null) {
        throw new RuntimeException('Cannot prepare phalanx moon without a home planet.');
    }
    $moonId = PlanetHasMoon($homePlanetId);
    if ($moonId <= 0) {
        $moonId = CreatePlanet((int)$home['g'], (int)$home['s'], (int)$home['p'], $ownerId, 1, 1, 20, time());
    }
    if ($moonId <= 0) {
        throw new RuntimeException('Failed to prepare phalanx fixture moon.');
    }

    $buildings = array();
    foreach ($buildmap as $gid) {
        $level = 0;
        if ($gid === GID_B_LUNAR_BASE) {
            $level = 1;
        } elseif ($gid === GID_B_PHALANX) {
            $level = 3;
        }
        $buildings[] = "`{$gid}`={$level}";
    }
    $ships = array();
    foreach ($fleetmap as $gid) {
        $ships[] = "`{$gid}`=0";
    }
    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "name='Go Smoke Moon', g=" . (int)$home['g'] . ", s=" . (int)$home['s'] . ", p=" . (int)$home['p'] . ", type=" . PTYP_MOON . ", owner_id={$ownerId}, " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=20000, " .
        implode(',', $buildings) . ", " . implode(',', $ships) . ", " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=2, maxfields=4, lastpeek=" . time() . " " .
        "WHERE planet_id={$moonId}"
    );
    return $moonId;
}

function smoke_dispatch_phalanx_fleet(int $ownerId, int $originPlanetId, int $targetPlanetId): int
{
    global $fleetmap, $transportableResources;

    $fleet = array();
    foreach ($fleetmap as $gid) {
        $fleet[$gid] = $gid === GID_F_SC ? 1 : 0;
    }
    $resources = array();
    foreach ($transportableResources as $gid) {
        $resources[$gid] = 0;
    }
    $origin = LoadPlanetById($originPlanetId);
    $target = LoadPlanetById($targetPlanetId);
    if ($origin === null || $target === null) {
        throw new RuntimeException('Cannot dispatch phalanx fixture fleet.');
    }
    AdjustShips($fleet, $originPlanetId, '-');
    return DispatchFleet($fleet, $origin, $target, FTYP_TRANSPORT, 3600, $resources, 0, time());
}

$name = getenv('OGAME_GO_LOGIN_SMOKE_USER') ?: 'legor';
$password = getenv('OGAME_GO_LOGIN_SMOKE_PASS') ?: 'admin';
$email = getenv('OGAME_GO_LOGIN_SMOKE_EMAIL') ?: ($name . '@example.local');

$login = smoke_prepare_user($name, $password, $email, USER_TYPE_ADMIN);
$target = smoke_prepare_user('gophalaxtarget', $password, 'gophalaxtarget@example.local', 0);
smoke_cleanup_alliances(array((int)$login['player_id'], (int)$target['player_id']));
$home = LoadPlanetById((int)$login['home_planet_id']);
if ($home === null) {
    throw new RuntimeException('Go smoke home planet is missing.');
}
$targetCoords = smoke_find_empty_position($home);
smoke_cleanup_fleets(array((int)$login['player_id'], (int)$target['player_id']), array((int)$login['home_planet_id'], (int)$target['home_planet_id']));
smoke_prepare_planet((int)$login['home_planet_id'], (int)$login['player_id'], 'Go Smoke Home', array((int)$home['g'], (int)$home['s'], (int)$home['p']));
smoke_prepare_planet((int)$target['home_planet_id'], (int)$target['player_id'], 'Go Smoke Target', $targetCoords);
$moonId = smoke_prepare_moon((int)$login['home_planet_id'], (int)$login['player_id']);
$fleetId = smoke_dispatch_phalanx_fleet((int)$target['player_id'], (int)$target['home_planet_id'], (int)$login['home_planet_id']);
SelectPlanet((int)$login['player_id'], (int)$login['home_planet_id']);

echo json_encode(array(
    'player_id' => (int)$login['player_id'],
    'name' => $login['name'],
    'home_planet_id' => (int)$login['home_planet_id'],
    'phalanx' => array(
        'source_moon_id' => $moonId,
        'target_planet_id' => (int)$target['home_planet_id'],
        'target_player_id' => (int)$target['player_id'],
        'fleet_id' => $fleetId,
        'initial_deuterium' => 20000,
        'cost' => 5000,
    ),
), JSON_UNESCAPED_SLASHES) . PHP_EOL;
