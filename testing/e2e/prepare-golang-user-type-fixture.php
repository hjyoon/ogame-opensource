<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-go-user-type-fixture';
$_SERVER['REQUEST_URI'] = '/testing/e2e/prepare-golang-user-type-fixture.php';
$_COOKIE['ogamelang'] = 'en';

chdir('/var/www/html/game');
require_once 'config.php';
require_once 'core/core.php';

InitDB();
$GlobalUni = LoadUniverse();
$GlobalUser = array('player_id' => 0);
$session = '';
ModsInit();

function user_type_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

function user_type_one_row(string $sql): ?array
{
    $res = dbquery($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function user_type_user_by_name(string $name): ?array
{
    global $db_prefix;
    return user_type_one_row("SELECT * FROM {$db_prefix}users WHERE name='" . user_type_sql_escape(mb_strtolower($name, 'UTF-8')) . "' LIMIT 1");
}

function user_type_home_planet_exists(int $playerId, int $planetId): bool
{
    global $db_prefix;
    if ($planetId <= 0) {
        return false;
    }
    $row = user_type_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE planet_id={$planetId} AND owner_id={$playerId} LIMIT 1");
    return $row !== null;
}

function user_type_prepare_account(string $login, string $displayName, string $password, int $admin, array $state): array
{
    global $db_prefix, $db_secret;

    $email = $login . '@example.local';
    $user = user_type_user_by_name($login);
    if ($user === null) {
        $playerId = CreateUser($displayName, $password, $email, false);
        $user = user_type_one_row("SELECT * FROM {$db_prefix}users WHERE player_id={$playerId} LIMIT 1");
        if ($user === null) {
            throw new RuntimeException('Failed to create fixture user ' . $login);
        }
    }

    $playerId = (int)$user['player_id'];
    $homePlanetId = (int)$user['hplanetid'];
    if (!user_type_home_planet_exists($playerId, $homePlanetId)) {
        $homePlanetId = CreateHomePlanet($playerId);
        if ($homePlanetId <= 0) {
            throw new RuntimeException('Failed to create home planet for ' . $login);
        }
    }

    $now = time();
    $validated = (int)($state['validated'] ?? 1);
    $validateCode = $validated ? '' : md5($login . '-activation');
    $vacation = (int)($state['vacation'] ?? 0);
    $vacationUntil = $vacation ? $now + 86400 : 0;
    $banned = (int)($state['banned'] ?? 0);
    $bannedUntil = $banned ? $now + 86400 : 0;
    $disable = (int)($state['disable'] ?? 0);
    $disableUntil = $disable ? $now + 7 * 86400 : 0;
    $passwordHash = md5($password . $db_secret);

    dbquery(
        "UPDATE {$db_prefix}users SET " .
        "name='" . user_type_sql_escape(mb_strtolower($login, 'UTF-8')) . "', " .
        "oname='" . user_type_sql_escape($displayName) . "', " .
        "password='" . user_type_sql_escape($passwordHash) . "', " .
        "pemail='" . user_type_sql_escape($email) . "', email='" . user_type_sql_escape($email) . "', " .
        "validated={$validated}, validatemd='" . user_type_sql_escape($validateCode) . "', deact_ip=1, admin={$admin}, " .
        "vacation={$vacation}, vacation_until={$vacationUntil}, banned={$banned}, banned_until={$bannedUntil}, " .
        "noattack=0, noattack_until=0, disable={$disable}, disable_until={$disableUntil}, " .
        "lang='en', skin='/evolution/', useskin=1, hplanetid={$homePlanetId}, aktplanet={$homePlanetId}, " .
        "lastclick={$now} WHERE player_id={$playerId}"
    );
    dbquery(
        "UPDATE {$db_prefix}planets SET name='" . user_type_sql_escape(substr($login, 0, 20)) . "', " .
        "`" . GID_RC_METAL . "`=500000, `" . GID_RC_CRYSTAL . "`=500000, `" . GID_RC_DEUTERIUM . "`=500000, " .
        "maxfields=200, fields=0 WHERE planet_id={$homePlanetId}"
    );
    InvalidateUserCache();

    return array(
        'login' => $login,
        'display' => $displayName,
        'player_id' => $playerId,
        'home_planet_id' => $homePlanetId,
        'admin' => $admin,
        'validated' => $validated,
        'vacation' => $vacation,
        'banned' => $banned,
        'disable' => $disable,
    );
}

$password = getenv('OGAME_GO_USER_TYPE_PASS') ?: 'qa-type-pass';
$users = array(
    'player' => user_type_prepare_account('qa_type_player', 'QA Type Player', $password, USER_TYPE_PLAYER, array()),
    'operator' => user_type_prepare_account('qa_type_operator', 'QA Type Operator', $password, USER_TYPE_GO, array()),
    'admin' => user_type_prepare_account('qa_type_admin', 'QA Type Admin', $password, USER_TYPE_ADMIN, array()),
    'unvalidated' => user_type_prepare_account('qa_type_unvalidated', 'QA Type Unvalidated', $password, USER_TYPE_PLAYER, array('validated' => 0)),
    'vacation' => user_type_prepare_account('qa_type_vacation', 'QA Type Vacation', $password, USER_TYPE_PLAYER, array('vacation' => 1)),
    'banned' => user_type_prepare_account('qa_type_banned', 'QA Type Banned', $password, USER_TYPE_PLAYER, array('banned' => 1)),
    'deletion_queued' => user_type_prepare_account('qa_type_delete', 'QA Type Delete', $password, USER_TYPE_PLAYER, array('disable' => 1)),
);

echo json_encode(array(
    'password' => $password,
    'users' => $users,
), JSON_UNESCAPED_SLASHES) . PHP_EOL;
