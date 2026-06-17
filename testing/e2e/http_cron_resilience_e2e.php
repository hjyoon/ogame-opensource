<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_cron_resilience_e2e.php';
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
loca_add('debug', 'en');

function e2e_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

function e2e_http_request(string $method, string $url): array
{
    $headers = array('User-Agent: ogame-e2e');
    $context = stream_context_create(array(
        'http' => array(
            'method' => $method,
            'header' => implode("\r\n", $headers),
            'ignore_errors' => true,
            'timeout' => 15,
            'follow_location' => 0,
        ),
    ));

    $body = file_get_contents($url, false, $context);
    $responseHeaders = $http_response_header ?? array();
    $status = 0;
    foreach ($responseHeaders as $header) {
        if (preg_match('/^HTTP\/\S+\s+(\d+)/', $header, $m)) {
            $status = (int)$m[1];
        }
    }

    return array(
        'status' => $status,
        'headers' => $responseHeaders,
        'body' => $body === false ? '' : $body,
    );
}

function e2e_case(bool $pass, string $message, array $context = array()): array
{
    return array('pass' => $pass, 'message' => $message, 'context' => $context);
}

function e2e_finalize_case(array $case): array
{
    $case['pass'] = array_reduce($case['checks'], fn($ok, $check) => $ok && $check['pass'], true);
    return $case;
}

function e2e_one_row(string $sql): ?array
{
    $res = dbquery($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function e2e_count(string $sql): int
{
    $row = e2e_one_row($sql);
    return $row === null ? 0 : (int)$row['cnt'];
}

function e2e_run_cron(): array
{
    global $from_cron, $GlobalUni, $GlobalUser;

    ob_start();
    include 'cron.php';
    $body = ob_get_clean();
    $from_cron = false;
    $GlobalUni = LoadUniverse();
    $GlobalUser = array('player_id' => 0);

    return array('body' => $body);
}

function e2e_queue_row(int $taskId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT task_id, type, freeze, frozen, end FROM {$db_prefix}queue WHERE task_id={$taskId} LIMIT 1");
}

function e2e_snapshot_uni(): array
{
    global $db_prefix;
    $row = e2e_one_row("SELECT freeze FROM {$db_prefix}uni LIMIT 1");
    if ($row === null) {
        throw new RuntimeException('Universe row is missing.');
    }
    return $row;
}

function e2e_restore_uni(?array $uni): void
{
    global $db_prefix, $GlobalUni;
    if ($uni === null) {
        return;
    }
    dbquery("UPDATE {$db_prefix}uni SET freeze=" . (int)$uni['freeze']);
    $GlobalUni = LoadUniverse();
}

function e2e_set_uni_freeze(int $freeze): void
{
    global $db_prefix, $GlobalUni;
    dbquery("UPDATE {$db_prefix}uni SET freeze={$freeze}");
    $GlobalUni = LoadUniverse();
}

function e2e_cleanup_cron_rows(string $token): void
{
    global $db_prefix;
    $safeToken = e2e_sql_escape($token);
    dbquery("DELETE FROM {$db_prefix}queue WHERE type LIKE '%{$safeToken}%' OR (owner_id=" . USER_SPACE . " AND type='" . QTYP_DEBUG . "' AND obj_id=987654)");
    dbquery("DELETE FROM {$db_prefix}debug WHERE text LIKE '%{$safeToken}%'");
}

$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$token = 'EC' . substr(md5((string)microtime(true)), 0, 8);
$cases = array();
$uniSnapshot = null;

try {
    $uniSnapshot = e2e_snapshot_uni();
    e2e_cleanup_cron_rows($token);

    $cronHttp = e2e_http_request('GET', $gameBase . '/cron.php');
    $cases[] = e2e_finalize_case(array(
        'case' => 'cron_php_is_not_browser_accessible',
        'checks' => array(
            e2e_case($cronHttp['status'] === 403, 'Apache denies direct browser access to cron.php', array('status' => $cronHttp['status'])),
            e2e_case(strpos($cronHttp['body'], '<?php') === false, 'denied cron response does not expose PHP source'),
        ),
    ));

    e2e_set_uni_freeze(0);
    $now = time();
    $debugId = AddQueue(USER_SPACE, QTYP_DEBUG, 0, 987654, 0, $now - 20, 1, QUEUE_PRIO_DEBUG);
    $unknownType = 'UNK_' . $token;
    $unknownId = AddQueue(USER_SPACE, $unknownType, 0, 987654, 0, $now - 20, 1, QUEUE_PRIO_DEBUG);
    $frozenId = AddQueue(USER_SPACE, QTYP_DEBUG, 0, 987654, 0, $now - 20, 1, QUEUE_PRIO_DEBUG);
    FreezeQueue($frozenId, true, $now - 10);

    $cronFirst = e2e_run_cron();
    $cronSecond = e2e_run_cron();
    $debugAfter = e2e_queue_row($debugId);
    $unknownAfter = e2e_queue_row($unknownId);
    $frozenAfter = e2e_queue_row($frozenId);
    $unknownDebugRows = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}debug WHERE text LIKE '%" . e2e_sql_escape($unknownType) . "%'");
    $cases[] = e2e_finalize_case(array(
        'case' => 'cron_processes_due_tasks_once_and_preserves_frozen_tasks',
        'checks' => array(
            e2e_case(trim($cronFirst['body']) === '' && trim($cronSecond['body']) === '', 'cron include emits no output or PHP warnings'),
            e2e_case($debugAfter === null, 'due debug task is removed by cron'),
            e2e_case($unknownAfter === null, 'unknown due task is removed by cron fallback handling'),
            e2e_case($frozenAfter !== null && (int)$frozenAfter['freeze'] === 1, 'frozen due task is preserved by cron', $frozenAfter ?? array()),
            e2e_case($unknownDebugRows === 1, 'unknown task creates exactly one debug audit row after repeated cron runs', array('debug_rows' => $unknownDebugRows)),
        ),
    ));

    e2e_set_uni_freeze(1);
    $freezeHeldId = AddQueue(USER_SPACE, QTYP_DEBUG, 0, 987654, 0, $now - 20, 1, QUEUE_PRIO_DEBUG);
    $cronFrozenUni = e2e_run_cron();
    $rowWhileFrozen = e2e_queue_row($freezeHeldId);
    e2e_set_uni_freeze(0);
    $cronUnfrozenUni = e2e_run_cron();
    $rowAfterUnfreeze = e2e_queue_row($freezeHeldId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'cron_respects_universe_freeze_then_drains_after_unfreeze',
        'checks' => array(
            e2e_case(trim($cronFrozenUni['body']) === '' && trim($cronUnfrozenUni['body']) === '', 'cron freeze/unfreeze runs emit no output or PHP warnings'),
            e2e_case($rowWhileFrozen !== null, 'due task remains queued while universe freeze is enabled', $rowWhileFrozen ?? array()),
            e2e_case($rowAfterUnfreeze === null, 'due task drains after universe freeze is disabled'),
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'cron_resilience_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    e2e_restore_uni($uniSnapshot);
    e2e_cleanup_cron_rows($token);
}

echo json_encode(array(
    'case_group' => 'http_cron_resilience',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
