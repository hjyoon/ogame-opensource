<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_admin_audit_logs_e2e.php';
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
            'timeout' => 15,
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
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined|Unknown column|You have an error in your SQL syntax/i', $body) === 1;
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

function e2e_seed_audit_rows(int $ownerId, string $token, string $loginIp): void
{
    $now = time();
    UserLog($ownerId, 'E2E_AUDIT', $token . ' user log marker', $now);
    AddDBRow(array(
        'owner_id' => $ownerId,
        'ip' => '127.0.0.1',
        'agent' => 'ogame-e2e',
        'url' => '/e2e/audit/' . $token,
        'text' => $token . ' debug marker',
        'date' => $now,
    ), 'debug');
    AddDBRow(array(
        'owner_id' => $ownerId,
        'ip' => '127.0.0.1',
        'agent' => 'ogame-e2e',
        'url' => '/e2e/audit/' . $token,
        'text' => $token . ' error marker',
        'date' => $now,
    ), 'errors');
    AddDBRow(array(
        'owner_id' => $ownerId,
        'url' => '/e2e/audit/' . $token,
        'method' => 'GET',
        'getdata' => serialize(array('token' => $token)),
        'postdata' => serialize(array()),
        'date' => $now,
    ), 'browse');
    LogIPAddress($loginIp, $ownerId, 0);
}

function e2e_cleanup_audit_rows(int $ownerId, string $token, string $loginIp): void
{
    global $db_prefix;
    $safeToken = e2e_sql_escape($token);
    $safeIp = e2e_sql_escape($loginIp);
    dbquery("DELETE FROM {$db_prefix}userlogs WHERE owner_id={$ownerId} AND (type='E2E_AUDIT' OR text LIKE '%{$safeToken}%')");
    dbquery("DELETE FROM {$db_prefix}debug WHERE text LIKE '%{$safeToken}%' OR url LIKE '%{$safeToken}%'");
    dbquery("DELETE FROM {$db_prefix}errors WHERE text LIKE '%{$safeToken}%' OR url LIKE '%{$safeToken}%'");
    dbquery("DELETE FROM {$db_prefix}browse WHERE url LIKE '%{$safeToken}%' OR getdata LIKE '%{$safeToken}%' OR postdata LIKE '%{$safeToken}%'");
    dbquery("DELETE FROM {$db_prefix}iplogs WHERE user_id={$ownerId} AND ip='{$safeIp}'");
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderName = getenv('OGAME_E2E_DEFENDER_NAME') ?: '';
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$token = 'E2EAUDIT' . substr(md5((string)microtime(true)), 0, 8);
$loginIp = '203.0.113.' . (10 + (time() % 80));
$cases = array();
$attackerSnapshot = null;
$defenderSnapshot = null;

try {
    if ($attackerId <= 0 || $defenderId <= 0 || $defenderName === '') {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $attackerSnapshot = e2e_snapshot_user($attackerId);
    $defenderSnapshot = e2e_snapshot_user($defenderId);
    if ($attackerSnapshot === null || $defenderSnapshot === null) {
        throw new RuntimeException('Fixture users are missing.');
    }

    e2e_cleanup_audit_rows($defenderId, $token, $loginIp);
    e2e_seed_audit_rows($defenderId, $token, $loginIp);

    $regularAuth = e2e_prepare_session($attackerId, USER_TYPE_PLAYER, 'audit-regular');
    $regularCookies = $regularAuth['cookies'];
    $checks = array();
    foreach (array('UserLogs', 'Debug', 'Errors', 'Browse', 'Logins', 'Fleetlogs') as $mode) {
        $response = e2e_http_request('GET', $gameBase . '/index.php?page=admin&session=' . rawurlencode($regularAuth['session']) . '&mode=' . rawurlencode($mode), array(), $regularCookies);
        $checks = array_merge($checks, e2e_response_check($response, 'regular ' . $mode . ' request'), array(
            e2e_case(stripos($response['body'], 'http-equiv') !== false && stripos($response['body'], 'refresh') !== false, 'regular user is redirected away from ' . $mode),
            e2e_case(strpos($response['body'], $token) === false, 'regular user does not receive seeded audit marker for ' . $mode),
        ));
    }
    $cases[] = e2e_finalize_case(array(
        'case' => 'regular_user_is_denied_for_admin_audit_modes',
        'checks' => $checks,
    ));

    $adminAuth = e2e_prepare_session($attackerId, USER_TYPE_ADMIN, 'audit-admin');
    $adminCookies = $adminAuth['cookies'];
    $userLogsResponse = e2e_http_request('GET', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=UserLogs', array(), $adminCookies);
    $debugResponse = e2e_http_request('GET', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Debug', array(), $adminCookies);
    $errorsResponse = e2e_http_request('GET', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Errors', array(), $adminCookies);
    $browseResponse = e2e_http_request('GET', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Browse', array(), $adminCookies);
    $loginsResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Logins', array(
        'name' => '',
        'id' => (string)$defenderId,
        'ip' => '',
    ), $adminCookies);
    $fleetlogsResponse = e2e_http_request('GET', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Fleetlogs', array(), $adminCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_audit_pages_render_seeded_markers',
        'checks' => array_merge(
            e2e_response_check($userLogsResponse, 'admin UserLogs request'),
            e2e_response_check($debugResponse, 'admin Debug request'),
            e2e_response_check($errorsResponse, 'admin Errors request'),
            e2e_response_check($browseResponse, 'admin Browse request'),
            e2e_response_check($loginsResponse, 'admin Logins request'),
            e2e_response_check($fleetlogsResponse, 'admin Fleetlogs request'),
            array(
                e2e_case(strpos($userLogsResponse['body'], $token . ' user log marker') !== false, 'UserLogs page shows the seeded user action marker'),
                e2e_case(strpos($debugResponse['body'], $token . ' debug marker') !== false, 'Debug page shows the seeded debug marker'),
                e2e_case(strpos($errorsResponse['body'], $token . ' error marker') !== false, 'Errors page shows the seeded error marker'),
                e2e_case(strpos($browseResponse['body'], '/e2e/audit/' . $token) !== false, 'Browse page shows the seeded browse-history URL'),
                e2e_case(strpos($loginsResponse['body'], $loginIp) !== false, 'Logins search shows the seeded login IP'),
                e2e_case(strpos($fleetlogsResponse['body'], 'Sent') !== false && strpos($fleetlogsResponse['body'], 'Start') !== false, 'Fleetlogs page renders the fleet-log table headers'),
            )
        ),
    ));

    $operatorAuth = e2e_prepare_session($attackerId, USER_TYPE_GO, 'audit-operator');
    $operatorCookies = $operatorAuth['cookies'];
    $operatorUserLogsResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($operatorAuth['session']) . '&mode=UserLogs', array(
        'name' => $defenderName,
        'type' => 'E2E_AUDIT',
        'days' => '2',
        'hours' => '0',
        'since' => date('j.n.Y', time() - 24 * 60 * 60),
    ), $operatorCookies);
    $debugBefore = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}debug WHERE text LIKE '%" . e2e_sql_escape($token) . "%'");
    $errorsBefore = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}errors WHERE text LIKE '%" . e2e_sql_escape($token) . "%'");
    $operatorDebugDeleteResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($operatorAuth['session']) . '&mode=Debug', array(
        'deletemessages' => 'deleteall',
    ), $operatorCookies);
    $operatorErrorsDeleteResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($operatorAuth['session']) . '&mode=Errors', array(
        'deletemessages' => 'deleteall',
    ), $operatorCookies);
    $debugAfter = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}debug WHERE text LIKE '%" . e2e_sql_escape($token) . "%'");
    $errorsAfter = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}errors WHERE text LIKE '%" . e2e_sql_escape($token) . "%'");
    $cases[] = e2e_finalize_case(array(
        'case' => 'operator_can_search_userlogs_but_cannot_clear_debug_or_errors',
        'checks' => array_merge(
            e2e_response_check($operatorUserLogsResponse, 'operator UserLogs search'),
            e2e_response_check($operatorDebugDeleteResponse, 'operator Debug delete-all POST'),
            e2e_response_check($operatorErrorsDeleteResponse, 'operator Errors delete-all POST'),
            array(
                e2e_case(strpos($operatorUserLogsResponse['body'], $token . ' user log marker') !== false, 'operator UserLogs search can find the seeded audit marker'),
                e2e_case($debugBefore === 1 && $debugAfter === 1, 'operator delete-all POST does not remove debug rows', array('before' => $debugBefore, 'after' => $debugAfter)),
                e2e_case($errorsBefore === 1 && $errorsAfter === 1, 'operator delete-all POST does not remove error rows', array('before' => $errorsBefore, 'after' => $errorsAfter)),
            )
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'admin_audit_logs_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($defenderId > 0) {
        e2e_cleanup_audit_rows($defenderId, $token, $loginIp);
    }
    e2e_restore_user($attackerSnapshot);
    e2e_restore_user($defenderSnapshot);
}

echo json_encode(array(
    'case_group' => 'http_admin_audit_logs',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
