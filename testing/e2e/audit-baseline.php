<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/audit-baseline.php';
$_COOKIE['ogamelang'] = 'en';

chdir('/var/www/html/game');
require_once 'config.php';
require_once 'core/core.php';

InitDB();
$GlobalUni = LoadUniverse();
$GlobalUser = array('player_id' => 0);
$session = '';
ModsInit();

function e2e_max_id(string $table, string $column): int
{
    global $db_prefix;

    $res = dbquery("SELECT COALESCE(MAX({$column}), 0) AS max_id FROM {$db_prefix}{$table}");
    $row = dbarray($res);
    return $row === false ? 0 : (int)$row['max_id'];
}

function e2e_export(string $name, int $value): void
{
    echo 'export ' . $name . '=' . $value . PHP_EOL;
}

function e2e_cleanup_runtime_files(): void
{
    foreach (array('temp/fleetlock_*', 'battledata/battle_*.txt', 'battleresult/battle_*.txt') as $pattern) {
        foreach (glob($pattern) ?: array() as $path) {
            if (is_file($path)) {
                unlink($path);
            }
        }
    }
}

e2e_cleanup_runtime_files();

e2e_export('OGAME_E2E_AUDIT_BASE_USER_ID', e2e_max_id('users', 'player_id'));
e2e_export('OGAME_E2E_AUDIT_BASE_MESSAGE_ID', e2e_max_id('messages', 'msg_id'));
e2e_export('OGAME_E2E_AUDIT_BASE_NOTE_ID', e2e_max_id('notes', 'note_id'));
e2e_export('OGAME_E2E_AUDIT_BASE_REPORT_ID', e2e_max_id('reports', 'id'));
e2e_export('OGAME_E2E_AUDIT_BASE_TEMPLATE_ID', e2e_max_id('template', 'id'));
e2e_export('OGAME_E2E_AUDIT_BASE_BOTVAR_ID', e2e_max_id('botvars', 'id'));
e2e_export('OGAME_E2E_AUDIT_BASE_ALLYAPP_ID', e2e_max_id('allyapps', 'app_id'));
e2e_export('OGAME_E2E_AUDIT_BASE_UNION_ID', e2e_max_id('union', 'union_id'));
e2e_export('OGAME_E2E_AUDIT_BASE_BATTLE_ID', e2e_max_id('battledata', 'battle_id'));
