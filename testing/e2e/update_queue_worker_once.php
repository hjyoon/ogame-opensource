<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/update_queue_worker_once.php';
$_COOKIE['ogamelang'] = 'en';

chdir('/var/www/html/game');
require_once 'config.php';
require_once 'core/core.php';

InitDB();
$GlobalUni = LoadUniverse();
$GlobalUser = array('player_id' => 0);
$session = '';
ModsInit();

$until = intval($argv[1] ?? time());
$before = 0;
$after = 0;

global $db_prefix;
$row = dbarray(dbquery("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE end <= {$until} AND freeze=0"));
if ($row !== false) {
    $before = intval($row['cnt']);
}

UpdateQueue($until);

$row = dbarray(dbquery("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE end <= {$until} AND freeze=0"));
if ($row !== false) {
    $after = intval($row['cnt']);
}

echo json_encode(array(
    'worker' => getmypid(),
    'until' => $until,
    'due_before' => $before,
    'due_after' => $after,
), JSON_UNESCAPED_SLASHES) . PHP_EOL;
