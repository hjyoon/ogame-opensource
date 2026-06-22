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

$name = getenv('OGAME_GO_LOGIN_SMOKE_USER') ?: 'legor';
$displayName = $name === mb_strtolower($name, 'UTF-8') ? ucfirst($name) : $name;
$password = getenv('OGAME_GO_LOGIN_SMOKE_PASS') ?: 'admin';
$email = getenv('OGAME_GO_LOGIN_SMOKE_EMAIL') ?: ($name . '@example.local');

$user = smoke_user_by_name($name);
if ($user === null) {
    $playerId = CreateUser($displayName, $password, $email, false);
    $user = smoke_one_row("SELECT * FROM {$db_prefix}users WHERE player_id={$playerId} LIMIT 1");
    if ($user === null) {
        throw new RuntimeException('Failed to create Go smoke login user.');
    }
}

$playerId = (int)$user['player_id'];
$homePlanetId = (int)$user['hplanetid'];
if (!smoke_home_planet_exists($playerId, $homePlanetId)) {
    $homePlanetId = CreateHomePlanet($playerId);
    if ($homePlanetId <= 0) {
        throw new RuntimeException('Failed to create Go smoke home planet.');
    }
}

$passwordHash = md5($password . $db_secret);
dbquery(
    "UPDATE {$db_prefix}users SET " .
    "name='" . smoke_sql_escape(mb_strtolower($name, 'UTF-8')) . "', " .
    "oname='" . smoke_sql_escape($displayName) . "', " .
    "password='" . smoke_sql_escape($passwordHash) . "', " .
    "pemail='" . smoke_sql_escape($email) . "', email='" . smoke_sql_escape($email) . "', " .
    "validated=1, validatemd='', deact_ip=1, admin=" . USER_TYPE_ADMIN . ", " .
    "vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, " .
    "lang='en', skin='/evolution/', useskin=1, hplanetid={$homePlanetId}, aktplanet={$homePlanetId}, " .
    "lastclick=" . time() . " WHERE player_id={$playerId}"
);
InvalidateUserCache();

echo json_encode(array(
    'player_id' => $playerId,
    'name' => $displayName,
    'home_planet_id' => $homePlanetId,
), JSON_UNESCAPED_SLASHES) . PHP_EOL;
