<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-overview-all-cases-reset';
$_SERVER['REQUEST_URI'] = '/testing/e2e/reset-overview-all-cases-fixture.php';
$_COOKIE['ogamelang'] = 'en';

chdir('/var/www/html/game');
require_once 'config.php';
require_once 'core/core.php';

InitDB();

function overview_all_reset_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

global $db_prefix;
$names = array(
    getenv('OGAME_OVERVIEW_ALL_USER') ?: 'overviewall',
    getenv('OGAME_OVERVIEW_ALL_ENEMY_USER') ?: 'overviewenemy',
    getenv('OGAME_OVERVIEW_ALL_SUPPORT_USER') ?: 'overviewsupport',
);
$safeNames = array_map(fn($name) => "'" . overview_all_reset_sql_escape(mb_strtolower($name, 'UTF-8')) . "'", $names);
dbquery("UPDATE {$db_prefix}uni SET freeze=0, news_until=0 WHERE 1=1");
dbquery("UPDATE {$db_prefix}users SET admin=" . USER_TYPE_PLAYER . ", vacation=0, vacation_until=0 WHERE name IN (" . implode(',', $safeNames) . ")");
InvalidateUserCache();

echo json_encode(array('reset' => true), JSON_UNESCAPED_SLASHES) . PHP_EOL;
