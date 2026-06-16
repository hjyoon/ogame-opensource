<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/setup-fixtures.php';
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

function e2e_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

function e2e_user_by_name(string $name): ?array
{
    global $db_prefix;
    $res = dbquery("SELECT player_id, hplanetid FROM {$db_prefix}users WHERE name='" . e2e_sql_escape(mb_strtolower($name, 'UTF-8')) . "' LIMIT 1");
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function e2e_remove_user_by_name(string $name): void
{
    $row = e2e_user_by_name($name);
    if ($row !== null) {
        RemoveUser((int)$row['player_id'], time());
    }
}

function e2e_create_user(string $name): array
{
    global $db_prefix;
    $pass = 'E2E_test123';
    $email = $name . '@example.local';
    $id = CreateUser($name, $pass, $email, false);

    $res = dbquery("SELECT hplanetid FROM {$db_prefix}users WHERE player_id={$id} LIMIT 1");
    $user = dbarray($res);
    if ($user === false || (int)$user['hplanetid'] <= 0) {
        throw new RuntimeException("Failed to create user {$name}");
    }

    global $resmap;
    $parts = array();
    foreach ($resmap as $gid) {
        $parts[] = "`{$gid}`=10";
    }
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $parts) .
        ", validated=1, validatemd='', deact_ip=1, lang='en', skin='/evolution/', useskin=1, adm_until=0, " .
        "dmfree=0, trader=0, rate_m=0, rate_k=0, rate_d=0 WHERE player_id={$id}"
    );
    InvalidateUserCache();

    return array(
        'player_id' => $id,
        'planet_id' => (int)$user['hplanetid'],
        'name' => $name,
        'password' => $pass,
        'email' => $email,
    );
}

$prefix = getenv('OGAME_E2E_FIXTURE_PREFIX') ?: 'e2e_fixture';
$attackerName = $prefix . '_attacker';
$defenderName = $prefix . '_defender';

e2e_remove_user_by_name($attackerName);
e2e_remove_user_by_name($defenderName);

$attacker = e2e_create_user($attackerName);
$defender = e2e_create_user($defenderName);

echo 'export OGAME_E2E_ATTACKER_ID=' . (int)$attacker['player_id'] . PHP_EOL;
echo 'export OGAME_E2E_ATTACKER_PLANET=' . (int)$attacker['planet_id'] . PHP_EOL;
echo 'export OGAME_E2E_DEFENDER_ID=' . (int)$defender['player_id'] . PHP_EOL;
echo 'export OGAME_E2E_DEFENDER_PLANET=' . (int)$defender['planet_id'] . PHP_EOL;
echo 'export OGAME_E2E_ATTACKER_NAME=' . escapeshellarg($attacker['name']) . PHP_EOL;
echo 'export OGAME_E2E_ATTACKER_PASSWORD=' . escapeshellarg($attacker['password']) . PHP_EOL;
echo 'export OGAME_E2E_DEFENDER_NAME=' . escapeshellarg($defender['name']) . PHP_EOL;
echo 'export OGAME_E2E_DEFENDER_PASSWORD=' . escapeshellarg($defender['password']) . PHP_EOL;
