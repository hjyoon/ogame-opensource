<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-authenticated-game-visual-fixture';
$_SERVER['REQUEST_URI'] = '/testing/e2e/prepare-authenticated-game-visual-fixture.php';
$_COOKIE['ogamelang'] = 'en';

chdir('/var/www/html/game');
require_once 'config.php';
require_once 'core/core.php';

InitDB();
$GlobalUni = LoadUniverse();
$GlobalUser = array('player_id' => 0);
$session = '';
ModsInit();

function auth_visual_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

function auth_visual_one_row(string $sql): ?array
{
    $res = dbquery($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function auth_visual_user_by_name(string $name): ?array
{
    global $db_prefix;
    $lower = mb_strtolower($name, 'UTF-8');
    return auth_visual_one_row("SELECT * FROM {$db_prefix}users WHERE name='" . auth_visual_sql_escape($lower) . "' LIMIT 1");
}

function auth_visual_prepare_user(string $name, string $password, int $adminLevel): array
{
    global $db_prefix, $db_secret;

    $lower = mb_strtolower($name, 'UTF-8');
    $displayName = ucfirst($lower);
    $user = auth_visual_user_by_name($lower);
    if ($user === null) {
        $playerId = CreateUser($displayName, $password, $lower . '@visual.local', false);
        $user = auth_visual_one_row("SELECT * FROM {$db_prefix}users WHERE player_id={$playerId} LIMIT 1");
        if ($user === null) {
            throw new RuntimeException('failed to create visual fixture user');
        }
    }

    $playerId = (int)$user['player_id'];
    $homePlanetId = (int)$user['hplanetid'];
    $home = $homePlanetId > 0 ? auth_visual_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE planet_id={$homePlanetId} AND owner_id={$playerId} LIMIT 1") : null;
    if ($home === null) {
        $homePlanetId = CreateHomePlanet($playerId);
        if ($homePlanetId <= 0) {
            throw new RuntimeException('failed to create visual fixture home planet');
        }
    }

    $passwordHash = md5($password . $db_secret);
    $now = time();
    dbquery(
        "UPDATE {$db_prefix}users SET " .
        "name='" . auth_visual_sql_escape($lower) . "', oname='" . auth_visual_sql_escape($displayName) . "', " .
        "password='" . auth_visual_sql_escape($passwordHash) . "', pemail='" . auth_visual_sql_escape($lower . '@visual.local') . "', " .
        "email='" . auth_visual_sql_escape($lower . '@visual.local') . "', validated=1, validatemd='', deact_ip=1, " .
        "admin={$adminLevel}, vacation=0, vacation_until=0, banned=0, banned_until=0, disable=0, disable_until=0, " .
        "noattack=0, noattack_until=0, lang='en', skin='/evolution/', useskin=1, " .
        "hplanetid={$homePlanetId}, aktplanet={$homePlanetId}, lastclick={$now} WHERE player_id={$playerId}"
    );

    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$playerId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$playerId} OR planet_id={$homePlanetId}");
    dbquery("DELETE FROM {$db_prefix}fleet WHERE owner_id={$playerId} OR start_planet={$homePlanetId} OR target_planet={$homePlanetId}");
    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000, " .
        "prod1=1, prod2=1, prod3=1, prod4=1, prod12=1, prod212=1, fields=0, maxfields=200, lastpeek={$now} " .
        "WHERE planet_id={$homePlanetId} AND owner_id={$playerId}"
    );

    InvalidateUserCache();
    SelectPlanet($playerId, $homePlanetId);

    return array('player_id' => $playerId, 'name' => $displayName, 'home_planet_id' => $homePlanetId);
}

function auth_visual_prepare_session(int $playerId): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5('auth-game-visual-session-' . $playerId . '-' . microtime(true)), 0, 12);
    $private = md5('auth-game-visual-private-' . $playerId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . auth_visual_sql_escape($session) . "', " .
        "private_session='" . auth_visual_sql_escape($private) . "', lastclick=" . time() . " WHERE player_id={$playerId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private_session' => $private,
        'cookies' => array('prsess_' . $playerId . '_' . $GlobalUni['num'] => $private),
    );
}

try {
    $name = getenv('OGAME_GAME_VISUAL_USER') ?: 'legor';
    $password = getenv('OGAME_GAME_VISUAL_PASS') ?: 'admin';
    $adminLevel = intval(getenv('OGAME_GAME_VISUAL_ADMIN') ?: USER_TYPE_ADMIN);
    $user = auth_visual_prepare_user($name, $password, $adminLevel);
    $auth = auth_visual_prepare_session((int)$user['player_id']);
    echo json_encode(array(
        'login_user' => $user['name'],
        'player_id' => (int)$user['player_id'],
        'home_planet_id' => (int)$user['home_planet_id'],
        'session' => $auth['session'],
        'private_session' => $auth['private_session'],
        'cookies' => $auth['cookies'],
    ), JSON_PRETTY_PRINT) . "\n";
} catch (Throwable $e) {
    fwrite(STDERR, $e->getMessage() . "\n");
    exit(1);
}
