<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/teardown-fixtures.php';
$_COOKIE['ogamelang'] = 'en';

chdir('/var/www/html/game');
require_once 'config.php';
require_once 'core/core.php';

InitDB();
$GlobalUni = LoadUniverse();
$GlobalUser = array('player_id' => 0);
$session = '';
ModsInit();

function e2e_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

function e2e_remove_user(string $name): void
{
    global $db_prefix;
    $name = mb_strtolower($name, 'UTF-8');
    $res = dbquery("SELECT player_id FROM {$db_prefix}users WHERE name='" . e2e_sql_escape($name) . "' LIMIT 1");
    $row = dbarray($res);
    if ($row !== false) {
        RemoveUser((int)$row['player_id'], time());
    }
}

$prefix = getenv('OGAME_E2E_FIXTURE_PREFIX') ?: 'e2e_fixture';
e2e_remove_user($prefix . '_attacker');
e2e_remove_user($prefix . '_defender');

$httpUser = getenv('OGAME_E2E_HTTP_USER');
if ($httpUser) {
    e2e_remove_user($httpUser);
}

echo json_encode(array('teardown' => 'ok'), JSON_UNESCAPED_SLASHES) . PHP_EOL;
