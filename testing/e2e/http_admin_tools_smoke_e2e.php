<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_admin_tools_smoke_e2e.php';
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
            'timeout' => 20,
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

function e2e_admin_url(string $gameBase, string $session, string $mode): string
{
    return $gameBase . '/index.php?page=admin&session=' . rawurlencode($session) . '&mode=' . rawurlencode($mode);
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$cases = array();
$attackerSnapshot = null;

try {
    if ($attackerId <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $attackerSnapshot = e2e_snapshot_user($attackerId);
    if ($attackerSnapshot === null) {
        throw new RuntimeException('Attacker fixture user is missing.');
    }

    $regularAuth = e2e_prepare_session($attackerId, USER_TYPE_PLAYER, 'admin-tools-regular');
    $regularCookies = $regularAuth['cookies'];
    $checks = array();
    foreach (array('Bots', 'BotEdit', 'Mods', 'Checksum', 'DB') as $mode) {
        $response = e2e_http_request('GET', e2e_admin_url($gameBase, $regularAuth['session'], $mode), array(), $regularCookies);
        $checks = array_merge($checks, e2e_response_check($response, 'regular ' . $mode . ' request'), array(
            e2e_case(stripos($response['body'], 'http-equiv') !== false && stripos($response['body'], 'refresh') !== false, 'regular user is redirected away from ' . $mode),
            e2e_case(stripos($response['body'], 'Bot List:') === false && stripos($response['body'], 'Checksum') === false && stripos($response['body'], 'Database Backup') === false, 'regular user does not receive tool content for ' . $mode),
        ));
    }
    $cases[] = e2e_finalize_case(array(
        'case' => 'regular_user_is_denied_for_admin_tool_modes',
        'checks' => $checks,
    ));

    $adminAuth = e2e_prepare_session($attackerId, USER_TYPE_ADMIN, 'admin-tools-admin');
    $adminCookies = $adminAuth['cookies'];
    $botsResponse = e2e_http_request('GET', e2e_admin_url($gameBase, $adminAuth['session'], 'Bots'), array(), $adminCookies);
    $botEditResponse = e2e_http_request('GET', e2e_admin_url($gameBase, $adminAuth['session'], 'BotEdit'), array(), $adminCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_bot_tool_pages_render',
        'checks' => array_merge(
            e2e_response_check($botsResponse, 'admin Bots request'),
            e2e_response_check($botEditResponse, 'admin BotEdit request'),
            array(
                e2e_case(strpos($botsResponse['body'], 'Bot List:') !== false, 'Bots page renders the bot list heading'),
                e2e_case(strpos($botsResponse['body'], 'Add bot:') !== false, 'Bots page renders the add-bot form'),
                e2e_case(strpos($botEditResponse['body'], 'myDiagram') !== false, 'BotEdit page renders the strategy diagram container'),
                e2e_case(strpos($botEditResponse['body'], 'Name of the edited strategy:') !== false, 'BotEdit page renders strategy controls'),
            )
        ),
    ));

    $checksumResponse = e2e_http_request('POST', e2e_admin_url($gameBase, $adminAuth['session'], 'Checksum'), array(), $adminCookies);
    $modsResponse = e2e_http_request('GET', e2e_admin_url($gameBase, $adminAuth['session'], 'Mods'), array(), $adminCookies);
    $dbResponse = e2e_http_request('GET', e2e_admin_url($gameBase, $adminAuth['session'], 'DB'), array(), $adminCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_mods_checksum_and_db_pages_render',
        'checks' => array_merge(
            e2e_response_check($checksumResponse, 'admin Checksum POST/render request'),
            e2e_response_check($modsResponse, 'admin Mods request'),
            e2e_response_check($dbResponse, 'admin DB request'),
            array(
                e2e_case(strpos($checksumResponse['body'], 'File path') !== false && strpos($checksumResponse['body'], 'Registration System') !== false, 'Checksum page renders checksum tables after writing baselines'),
                e2e_case(strpos($modsResponse['body'], 'mods-container') !== false, 'Mods page renders the modifications container'),
                e2e_case(strpos($dbResponse['body'], 'Comparison of tables from install and real database') !== false, 'DB page renders install-vs-database comparison'),
                e2e_case(strpos($dbResponse['body'], 'Database Backup') !== false, 'DB page renders backup management section'),
            )
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'admin_tools_smoke_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    e2e_restore_user($attackerSnapshot);
}

echo json_encode(array(
    'case_group' => 'http_admin_tools_smoke',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
