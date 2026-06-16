<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_session_security_edges_e2e.php';
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
loca_add('menu', 'en');
loca_add('overview', 'en');
loca_add('admin', 'en');
loca_add('technames', 'en');

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

function e2e_has_runtime_error(string $body): bool
{
    return preg_match('/Fatal error|Parse error|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined/i', $body) === 1;
}

function e2e_response_check(array $response): array
{
    $body = $response['body'];
    $runtimeError = e2e_has_runtime_error($body);
    $errorExcerpt = array();
    if ($runtimeError && preg_match('/.{0,120}(Fatal error|Parse error|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined).{0,180}/is', $body, $m)) {
        $errorExcerpt = array('excerpt' => trim(strip_tags($m[0])));
    }

    return array(
        e2e_case(in_array($response['status'], array(200, 301, 302, 303), true), 'HTTP request returns an accepted status', array('status' => $response['status'], 'location' => $response['location'])),
        e2e_case(!$runtimeError, 'HTTP body has no PHP runtime error marker', $errorExcerpt),
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
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', admin={$adminLevel}, validated=1, deact_ip=1, " .
        "vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, " .
        "disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1 WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookies' => array('prsess_' . $userId . '_' . $GlobalUni['num'] => $private),
    );
}

function e2e_user_snapshot(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT player_id, session, private_session, admin, validated, deact_ip, vacation, vacation_until, " .
        "banned, banned_until, noattack, noattack_until, disable, disable_until, lang, skin, useskin " .
        "FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1"
    );
}

function e2e_restore_user(int $userId, array $snapshot): void
{
    global $db_prefix;

    dbquery(
        "UPDATE {$db_prefix}users SET " .
        "session='" . e2e_sql_escape($snapshot['session']) . "', " .
        "private_session='" . e2e_sql_escape($snapshot['private_session']) . "', " .
        "admin=" . (int)$snapshot['admin'] . ", " .
        "validated=" . (int)$snapshot['validated'] . ", " .
        "deact_ip=" . (int)$snapshot['deact_ip'] . ", " .
        "vacation=" . (int)$snapshot['vacation'] . ", " .
        "vacation_until=" . (int)$snapshot['vacation_until'] . ", " .
        "banned=" . (int)$snapshot['banned'] . ", " .
        "banned_until=" . (int)$snapshot['banned_until'] . ", " .
        "noattack=" . (int)$snapshot['noattack'] . ", " .
        "noattack_until=" . (int)$snapshot['noattack_until'] . ", " .
        "disable=" . (int)$snapshot['disable'] . ", " .
        "disable_until=" . (int)$snapshot['disable_until'] . ", " .
        "lang='" . e2e_sql_escape($snapshot['lang']) . "', " .
        "skin='" . e2e_sql_escape($snapshot['skin']) . "', " .
        "useskin=" . (int)$snapshot['useskin'] . " " .
        "WHERE player_id={$userId}"
    );
    InvalidateUserCache();
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$attackerName = getenv('OGAME_E2E_ATTACKER_NAME') ?: '';
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$cases = array();
$originalAttacker = null;

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $attackerName === '') {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $originalAttacker = e2e_user_snapshot($attackerId);
    if ($originalAttacker === null) {
        throw new RuntimeException('Attacker fixture user is missing.');
    }

    $oldAuth = e2e_prepare_session($attackerId, USER_TYPE_PLAYER, 'session-edge-old');
    $oldCookies = $oldAuth['cookies'];
    $oldValidResponse = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($oldAuth['session']) . '&cp=' . $attackerPlanet, array(), $oldCookies);

    $newAuth = e2e_prepare_session($attackerId, USER_TYPE_PLAYER, 'session-edge-new');
    $newCookies = $newAuth['cookies'];
    $oldSessionCurrentCookieResponse = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($oldAuth['session']) . '&cp=' . $attackerPlanet, array(), $newCookies);
    $oldCookieNewSessionResponse = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($newAuth['session']) . '&cp=' . $attackerPlanet, array(), $oldCookies);
    $newValidResponse = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($newAuth['session']) . '&cp=' . $attackerPlanet, array(), $newCookies);
    $afterRotation = e2e_user_snapshot($attackerId);

    $cases[] = e2e_finalize_case(array(
        'case' => 'public_session_rotation_invalidates_previous_public_and_private_pairings',
        'checks' => array_merge(
            e2e_response_check($oldValidResponse),
            e2e_response_check($oldSessionCurrentCookieResponse),
            e2e_response_check($oldCookieNewSessionResponse),
            e2e_response_check($newValidResponse),
            array(
                e2e_case(strpos($oldValidResponse['body'], $attackerName) !== false, 'old session renders before rotation'),
                e2e_case(strpos($oldSessionCurrentCookieResponse['body'], $attackerName) === false, 'rotated public session rejects the previous public session even with the current private cookie'),
                e2e_case(strpos($oldCookieNewSessionResponse['body'], $attackerName) === false && strpos($oldCookieNewSessionResponse['body'], 'Error-ID:') !== false, 'new public session rejects the old private cookie'),
                e2e_case(strpos($newValidResponse['body'], $attackerName) !== false, 'new public session and private cookie render after rotation'),
                e2e_case($afterRotation !== null && $afterRotation['session'] === $newAuth['session'] && $afterRotation['private_session'] === $newAuth['private'], 'database keeps only the newest session pair', $afterRotation ?? array()),
            )
        ),
    ));

    $adminAuth = e2e_prepare_session($attackerId, USER_TYPE_ADMIN, 'session-edge-admin');
    $adminCookies = $adminAuth['cookies'];
    $adminResponse = e2e_http_request('GET', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']), array(), $adminCookies);
    dbquery("UPDATE {$db_prefix}users SET admin=" . USER_TYPE_PLAYER . " WHERE player_id={$attackerId}");
    InvalidateUserCache();
    $afterDowngrade = e2e_user_snapshot($attackerId);
    $downgradedResponse = e2e_http_request('GET', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']), array(), $adminCookies);

    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_downgrade_blocks_existing_session_without_relogin',
        'checks' => array_merge(
            e2e_response_check($adminResponse),
            e2e_response_check($downgradedResponse),
            array(
                e2e_case(stripos($adminResponse['body'], 'mode=Users') !== false && stripos($adminResponse['body'], 'mode=Uni') !== false, 'admin session can initially render admin navigation'),
                e2e_case($afterDowngrade !== null && (int)$afterDowngrade['admin'] === USER_TYPE_PLAYER, 'fixture user is downgraded in the database without changing the session', $afterDowngrade ?? array()),
                e2e_case(stripos($downgradedResponse['body'], 'mode=Users') === false && stripos($downgradedResponse['body'], 'ADM_USER') === false, 'downgraded existing session no longer renders admin controls'),
            )
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'session_security_edges_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($attackerId > 0 && $originalAttacker !== null) {
        e2e_restore_user($attackerId, $originalAttacker);
    }
}

echo json_encode(array(
    'case_group' => 'http_session_security_edges',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
