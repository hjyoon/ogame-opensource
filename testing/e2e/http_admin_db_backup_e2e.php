<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_admin_db_backup_e2e.php';
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

function e2e_http_request(string $method, string $url, array $data = array(), array &$cookies = array()): array
{
    $headers = array('User-Agent: ogame-e2e');
    if (!empty($cookies)) {
        $pairs = array();
        foreach ($cookies as $name => $value) {
            $pairs[] = $name . '=' . $value;
        }
        $headers[] = 'Cookie: ' . implode('; ', $pairs);
    }

    $content = null;
    if ($method === 'POST') {
        $content = http_build_query($data);
        $headers[] = 'Content-Type: application/x-www-form-urlencoded';
        $headers[] = 'Content-Length: ' . strlen($content);
    }

    $context = stream_context_create(array(
        'http' => array(
            'method' => $method,
            'header' => implode("\r\n", $headers),
            'content' => $content,
            'ignore_errors' => true,
            'timeout' => 30,
            'follow_location' => 0,
        ),
    ));

    $body = file_get_contents($url, false, $context);
    $responseHeaders = $http_response_header ?? array();
    $status = 0;
    $location = '';
    foreach ($responseHeaders as $header) {
        if (preg_match('/^HTTP\/\S+\s+(\d+)/', $header, $m)) {
            $status = (int)$m[1];
        } elseif (stripos($header, 'Location:') === 0) {
            $location = trim(substr($header, 9));
        } elseif (stripos($header, 'Set-Cookie:') === 0) {
            $cookie = trim(substr($header, 11));
            $cookiePart = explode(';', $cookie, 2)[0];
            $kv = explode('=', $cookiePart, 2);
            if (count($kv) === 2) {
                $cookies[$kv[0]] = $kv[1];
            }
        }
    }

    return array(
        'status' => $status,
        'location' => $location,
        'headers' => $responseHeaders,
        'body' => $body === false ? '' : $body,
    );
}

function e2e_case(bool $pass, string $message, array $context = array()): array
{
    return array('pass' => $pass, 'message' => $message, 'context' => $context);
}

function e2e_response_check(array $response, string $label = 'HTTP request'): array
{
    $body = $response['body'];
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read|file_get_contents|unserialize|scandir)|Notice:\s+Undefined|Unknown column|You have an error in your SQL syntax/i', $body) === 1;
    return array(
        e2e_case(in_array($response['status'], array(200, 301, 302, 303), true), $label . ' returns an accepted status', array('status' => $response['status'], 'location' => $response['location'])),
        e2e_case(!$hasError, $label . ' body has no PHP or SQL error marker'),
    );
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

function e2e_prepare_session(int $userId, int $adminLevel, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', private_session='" . e2e_sql_escape($private) . "', admin={$adminLevel}, " .
        "validated=1, deact_ip=1, vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, " .
        "disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1 WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookies' => array('prsess_' . $userId . '_' . $GlobalUni['num'] => $private),
    );
}

function e2e_snapshot_user(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT player_id, admin, session, private_session, validated, deact_ip, vacation, vacation_until, banned, banned_until, " .
        "noattack, noattack_until, disable, disable_until, lang, skin, useskin FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1"
    );
}

function e2e_restore_user(?array $user): void
{
    global $db_prefix;
    if ($user === null) {
        return;
    }

    $id = (int)$user['player_id'];
    dbquery(
        "UPDATE {$db_prefix}users SET admin=" . (int)$user['admin'] . ", session='" . e2e_sql_escape($user['session']) . "', " .
        "private_session='" . e2e_sql_escape($user['private_session']) . "', validated=" . (int)$user['validated'] . ", deact_ip=" . (int)$user['deact_ip'] . ", " .
        "vacation=" . (int)$user['vacation'] . ", vacation_until=" . (int)$user['vacation_until'] . ", banned=" . (int)$user['banned'] . ", " .
        "banned_until=" . (int)$user['banned_until'] . ", noattack=" . (int)$user['noattack'] . ", noattack_until=" . (int)$user['noattack_until'] . ", " .
        "disable=" . (int)$user['disable'] . ", disable_until=" . (int)$user['disable_until'] . ", lang='" . e2e_sql_escape($user['lang']) . "', " .
        "skin='" . e2e_sql_escape($user['skin']) . "', useskin=" . (int)$user['useskin'] . " WHERE player_id={$id}"
    );
    InvalidateUserCache();
}

function e2e_admin_url(string $gameBase, string $session, string $suffix = ''): string
{
    return $gameBase . '/index.php?page=admin&session=' . rawurlencode($session) . '&mode=DB' . $suffix;
}

function e2e_backup_files(): array
{
    clearstatcache();
    $files = array();
    foreach (glob('temp/backup*.json') ?: array() as $path) {
        $files[basename($path)] = filemtime($path) ?: 0;
    }
    return $files;
}

function e2e_find_new_backup(array $before, int $startedAt): ?string
{
    clearstatcache();
    $newest = null;
    $newestMtime = 0;
    foreach (glob('temp/backup*.json') ?: array() as $path) {
        $base = basename($path);
        $mtime = filemtime($path) ?: 0;
        if (!isset($before[$base]) || $mtime >= $startedAt) {
            if ($mtime >= $newestMtime) {
                $newest = $path;
                $newestMtime = $mtime;
            }
        }
    }
    return $newest;
}

function e2e_delete_file_if_exists(?string $path): void
{
    if ($path !== null && is_file($path)) {
        unlink($path);
    }
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$token = 'E2EDB' . substr(md5((string)microtime(true)), 0, 8);
$cases = array();
$attackerSnapshot = null;
$createdBackup = null;
$operatorGuardBackup = 'temp/backup_e2e_operator_guard_' . substr(md5((string)microtime(true)), 0, 8) . '.json';

try {
    if ($attackerId <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $attackerSnapshot = e2e_snapshot_user($attackerId);
    if ($attackerSnapshot === null) {
        throw new RuntimeException('Attacker fixture user is missing.');
    }

    $operatorAuth = e2e_prepare_session($attackerId, USER_TYPE_GO, 'admin-db-operator');
    $operatorCookies = $operatorAuth['cookies'];
    file_put_contents($operatorGuardBackup, '{"operator":"guard"}');
    $beforeOperatorCreate = e2e_backup_files();
    $operatorCreateResponse = e2e_http_request('POST', e2e_admin_url($gameBase, $operatorAuth['session'], '&action=create'), array(), $operatorCookies);
    $afterOperatorCreate = e2e_backup_files();
    $operatorDeleteResponse = e2e_http_request('GET', e2e_admin_url($gameBase, $operatorAuth['session'], '&action=delete&fname=' . rawurlencode(basename($operatorGuardBackup))), array(), $operatorCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'operator_cannot_create_or_delete_database_backups',
        'checks' => array_merge(
            e2e_response_check($operatorCreateResponse, 'operator DB create POST'),
            e2e_response_check($operatorDeleteResponse, 'operator DB delete GET'),
            array(
                e2e_case(count($afterOperatorCreate) === count($beforeOperatorCreate), 'operator create action does not add a backup file', array('before' => count($beforeOperatorCreate), 'after' => count($afterOperatorCreate))),
                e2e_case(is_file($operatorGuardBackup), 'operator delete action leaves the guard backup file intact'),
            )
        ),
    ));
    e2e_delete_file_if_exists($operatorGuardBackup);

    $adminAuth = e2e_prepare_session($attackerId, USER_TYPE_ADMIN, 'admin-db-admin');
    $adminCookies = $adminAuth['cookies'];
    $beforeBackups = e2e_backup_files();
    $startedAt = time();
    $createResponse = e2e_http_request('POST', e2e_admin_url($gameBase, $adminAuth['session'], '&action=create'), array(), $adminCookies);
    $createdBackup = e2e_find_new_backup($beforeBackups, $startedAt);
    $backupBase = $createdBackup === null ? '' : basename($createdBackup);
    $backupSize = $createdBackup === null || !is_file($createdBackup) ? 0 : filesize($createdBackup);
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_can_create_database_backup_file',
        'checks' => array_merge(e2e_response_check($createResponse, 'admin DB create POST'), array(
            e2e_case($createdBackup !== null && is_file($createdBackup), 'admin create action writes a backup file', array('backup' => $backupBase)),
            e2e_case($backupSize > 0, 'backup file is not empty', array('bytes' => $backupSize)),
            e2e_case(strpos($createResponse['body'], 'The backup is saved to file') !== false, 'create response reports saved backup'),
        )),
    ));

    if ($createdBackup === null) {
        throw new RuntimeException('Admin backup file was not created.');
    }

    UserLog($attackerId, 'E2E_DB_RESTORE', $token . ' restore marker', time());
    $markerBeforeRestore = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}userlogs WHERE type='E2E_DB_RESTORE' AND text LIKE '%" . e2e_sql_escape($token) . "%'");
    $restoreResponse = e2e_http_request('GET', e2e_admin_url($gameBase, $adminAuth['session'], '&action=restore&fname=' . rawurlencode($backupBase)), array(), $adminCookies);
    $markerAfterRestore = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}userlogs WHERE type='E2E_DB_RESTORE' AND text LIKE '%" . e2e_sql_escape($token) . "%'");
    $deleteResponse = e2e_http_request('GET', e2e_admin_url($gameBase, $adminAuth['session'], '&action=delete&fname=' . rawurlencode($backupBase)), array(), $adminCookies);
    clearstatcache(true, $createdBackup);
    $backupExistsAfterDelete = is_file($createdBackup);
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_can_restore_from_then_delete_database_backup',
        'checks' => array_merge(
            e2e_response_check($restoreResponse, 'admin DB restore GET'),
            e2e_response_check($deleteResponse, 'admin DB delete GET'),
            array(
                e2e_case($markerBeforeRestore === 1, 'restore marker exists after backup creation', array('before_restore' => $markerBeforeRestore)),
                e2e_case($markerAfterRestore === 0, 'restore reverts rows created after backup creation', array('after_restore' => $markerAfterRestore)),
                e2e_case(strpos($restoreResponse['body'], 'Backup restored from file') !== false, 'restore response reports restored backup'),
                e2e_case(!$backupExistsAfterDelete, 'admin delete action removes the backup file'),
                e2e_case(strpos($deleteResponse['body'], 'Backup deleted') !== false, 'delete response reports deleted backup'),
            )
        ),
    ));
    $createdBackup = null;
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'admin_db_backup_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    e2e_delete_file_if_exists($operatorGuardBackup);
    e2e_delete_file_if_exists($createdBackup);
    if ($attackerId > 0) {
        dbquery("DELETE FROM {$db_prefix}userlogs WHERE type='E2E_DB_RESTORE' AND text LIKE '%" . e2e_sql_escape($token) . "%'");
    }
    e2e_restore_user($attackerSnapshot);
}

echo json_encode(array(
    'case_group' => 'http_admin_db_backup',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
