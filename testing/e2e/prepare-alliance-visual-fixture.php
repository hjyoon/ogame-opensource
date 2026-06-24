<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-alliance-visual-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/prepare-alliance-visual-fixture.php';
$_COOKIE['ogamelang'] = 'en';

chdir('/var/www/html/game');
require_once 'config.php';
require_once 'core/core.php';

InitDB();
$GlobalUni = LoadUniverse();
$GlobalUser = array('player_id' => 0, 'lang' => 'en');
$session = '';
ModsInit();

function alliance_visual_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

function alliance_visual_one_row(string $sql): ?array
{
    $res = dbquery($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function alliance_visual_user_by_name(string $name): ?array
{
    global $db_prefix;
    return alliance_visual_one_row("SELECT * FROM {$db_prefix}users WHERE name='" . alliance_visual_sql_escape(mb_strtolower($name, 'UTF-8')) . "' LIMIT 1");
}

function alliance_visual_prepare_user(string $login, string $displayName, string $password, string $email): array
{
    global $db_prefix, $db_secret;

    $user = alliance_visual_user_by_name($login);
    if ($user === null) {
        $playerId = CreateUser($displayName, $password, $email, false);
        $user = alliance_visual_one_row("SELECT * FROM {$db_prefix}users WHERE player_id={$playerId} LIMIT 1");
        if ($user === null) {
            throw new RuntimeException("Failed to create fixture user {$login}.");
        }
    }

    $playerId = (int)$user['player_id'];
    $homePlanetId = (int)$user['hplanetid'];
    if ($homePlanetId <= 0 || alliance_visual_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE planet_id={$homePlanetId} AND owner_id={$playerId} LIMIT 1") === null) {
        $homePlanetId = CreateHomePlanet($playerId);
    }
    if ($homePlanetId <= 0) {
        throw new RuntimeException("Failed to create fixture home planet.");
    }

    $passwordHash = md5($password . $db_secret);
    $now = time();
    dbquery(
        "UPDATE {$db_prefix}users SET " .
        "name='" . alliance_visual_sql_escape(mb_strtolower($login, 'UTF-8')) . "', " .
        "oname='" . alliance_visual_sql_escape($displayName) . "', " .
        "password='" . alliance_visual_sql_escape($passwordHash) . "', " .
        "pemail='" . alliance_visual_sql_escape($email) . "', email='" . alliance_visual_sql_escape($email) . "', " .
        "validated=1, validatemd='', deact_ip=1, admin=" . USER_TYPE_PLAYER . ", " .
        "vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, " .
        "disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, " .
        "hplanetid={$homePlanetId}, aktplanet={$homePlanetId}, lastclick={$now} " .
        "WHERE player_id={$playerId}"
    );
    dbquery(
        "UPDATE {$db_prefix}planets SET name='Alliance Prime', " .
        "`" . GID_RC_METAL . "`=500000, `" . GID_RC_CRYSTAL . "`=500000, `" . GID_RC_DEUTERIUM . "`=500000, " .
        "maxfields=200, fields=0 WHERE planet_id={$homePlanetId}"
    );

    return array('player_id' => $playerId, 'planet_id' => $homePlanetId, 'login' => mb_strtolower($login, 'UTF-8'));
}

function alliance_visual_reset_alliance(string $tag): void
{
    global $db_prefix;

    $res = dbquery("SELECT ally_id FROM {$db_prefix}ally WHERE tag='" . alliance_visual_sql_escape($tag) . "'");
    while ($row = dbarray($res)) {
        $allyId = (int)$row['ally_id'];
        dbquery("UPDATE {$db_prefix}users SET ally_id=0, allyrank=0, joindate=0 WHERE ally_id={$allyId}");
        dbquery("DELETE FROM {$db_prefix}allyapps WHERE ally_id={$allyId}");
        dbquery("DELETE FROM {$db_prefix}allyranks WHERE ally_id={$allyId}");
        dbquery("DELETE FROM {$db_prefix}ally WHERE ally_id={$allyId}");
    }
}

$password = 'alliancevisual';
$fixture = alliance_visual_prepare_user('alliancevisual', 'Alliance Visual', $password, 'alliancevisual@example.local');
$playerId = (int)$fixture['player_id'];

alliance_visual_reset_alliance('AVQA');
dbquery("UPDATE {$db_prefix}users SET ally_id=0, allyrank=0, joindate=0 WHERE player_id={$playerId}");
$allyId = CreateAlly($playerId, 'AVQA', 'Alliance Visual QA');
dbquery(
    "UPDATE {$db_prefix}ally SET " .
    "homepage='https://example.com/alliance', imglogo='', open=1, insertapp=1, " .
    "exttext='Welcome to the alliance page', inttext='Internal alliance notice', apptext='Please describe your application.' " .
    "WHERE ally_id={$allyId}"
);
dbquery("UPDATE {$db_prefix}allyranks SET name='Founder' WHERE ally_id={$allyId} AND rank_id=0");
dbquery("UPDATE {$db_prefix}allyranks SET name='Newcomer' WHERE ally_id={$allyId} AND rank_id=1");
InvalidateUserCache();

header('Content-Type: application/json');
echo json_encode(array(
    'login' => $fixture['login'],
    'password' => $password,
    'user_id' => $playerId,
    'home_planet_id' => (int)$fixture['planet_id'],
    'alliance_id' => $allyId,
), JSON_PRETTY_PRINT) . "\n";
