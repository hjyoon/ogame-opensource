<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-empire-visual-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/prepare-empire-visual-fixture.php';
$_COOKIE['ogamelang'] = 'en';

chdir('/var/www/html/game');
require_once 'config.php';
require_once 'core/core.php';

InitDB();
$GlobalUni = LoadUniverse();
$GlobalUser = array('player_id' => 0, 'lang' => 'en');
$session = '';
ModsInit();

function empire_visual_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

function empire_visual_one_row(string $sql): ?array
{
    $res = dbquery($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function empire_visual_user_by_name(string $name): ?array
{
    global $db_prefix;
    return empire_visual_one_row("SELECT * FROM {$db_prefix}users WHERE name='" . empire_visual_sql_escape(mb_strtolower($name, 'UTF-8')) . "' LIMIT 1");
}

function empire_visual_prepare_user(string $name, string $password, string $email): array
{
    global $db_prefix, $db_secret;

    $displayName = ucfirst($name);
    $user = empire_visual_user_by_name($name);
    if ($user === null) {
        $playerId = CreateUser($displayName, $password, $email, false);
        $user = empire_visual_one_row("SELECT * FROM {$db_prefix}users WHERE player_id={$playerId} LIMIT 1");
        if ($user === null) {
            throw new RuntimeException("Failed to create fixture user {$name}.");
        }
    }

    $playerId = (int)$user['player_id'];
    $homePlanetId = (int)$user['hplanetid'];
    if ($homePlanetId <= 0 || empire_visual_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE planet_id={$homePlanetId} LIMIT 1") === null) {
        $homePlanetId = CreateHomePlanet($playerId);
    }
    if ($homePlanetId <= 0) {
        throw new RuntimeException("Failed to create fixture home planet.");
    }

    $passwordHash = md5($password . $db_secret);
    $until = time() + 30 * 24 * 60 * 60;
    dbquery(
        "UPDATE {$db_prefix}users SET " .
        "name='" . empire_visual_sql_escape(mb_strtolower($name, 'UTF-8')) . "', " .
        "oname='" . empire_visual_sql_escape($displayName) . "', " .
        "password='" . empire_visual_sql_escape($passwordHash) . "', " .
        "pemail='" . empire_visual_sql_escape($email) . "', email='" . empire_visual_sql_escape($email) . "', " .
        "validated=1, validatemd='', deact_ip=1, admin=" . USER_TYPE_PLAYER . ", " .
        "vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, " .
        "disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, " .
        "com_until={$until}, geo_until={$until}, eng_until={$until}, adm_until=0, tec_until=0, " .
        "`" . GID_R_ENERGY . "`=9, `" . GID_R_COMPUTER . "`=8, `" . GID_R_ESPIONAGE . "`=7, `" . GID_R_EXPEDITION . "`=4, " .
        "hplanetid={$homePlanetId}, aktplanet={$homePlanetId}, lastclick=" . time() . " " .
        "WHERE player_id={$playerId}"
    );

    return array('player_id' => $playerId, 'planet_id' => $homePlanetId, 'name' => $displayName, 'login' => mb_strtolower($name, 'UTF-8'));
}

function empire_visual_find_free_position(array &$reserved): array
{
    for ($galaxy = 1; $galaxy <= 9; $galaxy++) {
        for ($system = 430; $system <= 470; $system++) {
            for ($position = 4; $position <= 15; $position++) {
                $key = "{$galaxy}:{$system}:{$position}";
                if (isset($reserved[$key])) {
                    continue;
                }
                if (LoadPlanet($galaxy, $system, $position, 1) === false && LoadPlanet($galaxy, $system, $position, 3) === false) {
                    $reserved[$key] = true;
                    return array('g' => $galaxy, 's' => $system, 'p' => $position);
                }
            }
        }
    }
    throw new RuntimeException('No free empire visual coordinate slot found.');
}

function empire_visual_prepare_planet(int $planetId, int $ownerId, string $name, array $coords, int $type = PTYP_PLANET): void
{
    global $db_prefix, $fleetmap, $defmap;

    $ships = array();
    foreach ($fleetmap as $gid) {
        $ships[] = "`{$gid}`=0";
    }
    $defense = array();
    foreach ($defmap as $gid) {
        $defense[] = "`{$gid}`=0";
    }

    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "name='" . empire_visual_sql_escape($name) . "', type={$type}, owner_id={$ownerId}, " .
        "g=" . (int)$coords['g'] . ", s=" . (int)$coords['s'] . ", p=" . (int)$coords['p'] . ", " .
        "diameter=12800, temp=24, fields=120, maxfields=220, " .
        "`" . GID_RC_METAL . "`=123456789, `" . GID_RC_CRYSTAL . "`=65432123, `" . GID_RC_DEUTERIUM . "`=23456789, " .
        "`" . GID_B_METAL_MINE . "`=15, `" . GID_B_CRYS_MINE . "`=14, `" . GID_B_DEUT_SYNTH . "`=13, `" . GID_B_SOLAR . "`=16, " .
        "`" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=9, `" . GID_B_RES_LAB . "`=8, `" . GID_B_MISS_SILO . "`=4, " .
        implode(',', $ships) . ", " .
        implode(',', $defense) . ", " .
        "`" . GID_F_SC . "`=12, `" . GID_F_LC . "`=6, `" . GID_F_PROBE . "`=9, " .
        "`" . GID_D_RL . "`=25, `" . GID_D_LL . "`=12, `" . GID_D_ABM . "`=6, " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, remove=0 WHERE planet_id={$planetId}"
    );
}

function empire_visual_create_planet(int $ownerId, string $name, array $coords): int
{
    global $db_prefix;
    $id = CreatePlanet((int)$coords['g'], (int)$coords['s'], (int)$coords['p'], $ownerId, 1, 0, 0, time());
    if ($id <= 0) {
        $row = empire_visual_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE g=" . (int)$coords['g'] . " AND s=" . (int)$coords['s'] . " AND p=" . (int)$coords['p'] . " AND type=" . PTYP_PLANET . " LIMIT 1");
        $id = $row === null ? 0 : (int)$row['planet_id'];
    }
    if ($id <= 0) {
        throw new RuntimeException("Failed to create fixture planet {$name}.");
    }
    empire_visual_prepare_planet($id, $ownerId, $name, $coords, PTYP_PLANET);
    return $id;
}

function empire_visual_create_moon(int $ownerId, string $name, array $coords): int
{
    global $db_prefix;
    $id = CreatePlanet((int)$coords['g'], (int)$coords['s'], (int)$coords['p'], $ownerId, 1, 1, 20, time());
    if ($id <= 0) {
        $row = empire_visual_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE g=" . (int)$coords['g'] . " AND s=" . (int)$coords['s'] . " AND p=" . (int)$coords['p'] . " AND type=" . PTYP_MOON . " LIMIT 1");
        $id = $row === null ? 0 : (int)$row['planet_id'];
    }
    if ($id <= 0) {
        throw new RuntimeException("Failed to create fixture moon {$name}.");
    }
    empire_visual_prepare_planet($id, $ownerId, $name, $coords, PTYP_MOON);
    return $id;
}

$fixture = empire_visual_prepare_user('empirevisual', 'empirevisual', 'empirevisual@example.local');
$playerId = (int)$fixture['player_id'];
$homeId = (int)$fixture['planet_id'];
$home = LoadPlanetById($homeId);
if ($home === null) {
    throw new RuntimeException('Fixture home planet not found.');
}

dbquery("DELETE FROM {$db_prefix}fleet WHERE owner_id={$playerId}");
dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$playerId}");
dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$playerId}");
dbquery("DELETE FROM {$db_prefix}planets WHERE owner_id={$playerId} AND planet_id<>{$homeId}");

$reserved = array($home['g'] . ':' . $home['s'] . ':' . $home['p'] => true);
$homeCoords = array('g' => (int)$home['g'], 's' => (int)$home['s'], 'p' => (int)$home['p']);
$colonyCoords = empire_visual_find_free_position($reserved);

empire_visual_prepare_planet($homeId, $playerId, 'Empire Prime', $homeCoords, PTYP_PLANET);
$colonyId = empire_visual_create_planet($playerId, 'Empire Colony', $colonyCoords);
$moonId = empire_visual_create_moon($playerId, 'Empire Moon', $homeCoords);

header('Content-Type: application/json');
echo json_encode(array(
    'login' => $fixture['login'],
    'password' => 'empirevisual',
    'user_id' => $playerId,
    'home_planet_id' => $homeId,
    'colony_planet_id' => $colonyId,
    'moon_id' => $moonId,
), JSON_PRETTY_PRINT) . "\n";
