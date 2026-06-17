<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_security_hardening_e2e.php';
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
loca_add('ally', 'en');

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

function e2e_response_check(array $response, string $label): array
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
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', private_session='" . e2e_sql_escape($private) . "', " .
        "admin={$adminLevel}, validated=1, deact_ip=1, vacation=0, vacation_until=0, banned=0, banned_until=0, " .
        "noattack=0, noattack_until=0, disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1 " .
        "WHERE player_id={$userId}"
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
        "SELECT player_id, session, private_session, admin, validated, deact_ip, vacation, vacation_until, banned, banned_until, " .
        "noattack, noattack_until, disable, disable_until, lang, skin, useskin, ally_id, allyrank, joindate FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1"
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
        "skin='" . e2e_sql_escape($user['skin']) . "', useskin=" . (int)$user['useskin'] . ", ally_id=" . (int)$user['ally_id'] . ", " .
        "allyrank=" . (int)$user['allyrank'] . ", joindate=" . (int)$user['joindate'] . " WHERE player_id={$id}"
    );
    InvalidateUserCache();
}

function e2e_admin_db_url(string $gameBase, string $session, string $suffix = ''): string
{
    return $gameBase . '/index.php?page=admin&session=' . rawurlencode($session) . '&mode=DB' . $suffix;
}

function e2e_delete_file_if_exists(?string $path): void
{
    if ($path !== null && is_file($path)) {
        unlink($path);
    }
}

function e2e_body_has_raw_payload_html(string $body, string $payloadToken): bool
{
    return preg_match('/<script\b[^>]*>.*?' . preg_quote($payloadToken, '/') . '|<img\b[^>]*(onerror|javascript:)|<\/textarea>\s*<script/i', $body) === 1;
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$attackerName = getenv('OGAME_E2E_ATTACKER_NAME') ?: '';
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$token = 'E2EHARD' . substr(md5((string)microtime(true)), 0, 8);
$cases = array();
$attackerSnapshot = null;
$defenderSnapshot = null;
$createdAlly = 0;
$badBackup = null;
$nonBackup = null;

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $attackerName === '') {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $attackerSnapshot = e2e_user_snapshot($attackerId);
    $defenderSnapshot = e2e_user_snapshot($defenderId);
    if ($attackerSnapshot === null || $defenderSnapshot === null) {
        throw new RuntimeException('Fixture users are missing.');
    }

    $adminAuth = e2e_prepare_session($attackerId, USER_TYPE_ADMIN, 'hardening-admin');
    $adminCookies = $adminAuth['cookies'];
    $badBackup = 'temp/backup_' . $token . '_bad.json';
    $nonBackup = 'temp/e2e_' . $token . '_not_backup.json';
    file_put_contents($badBackup, '{"broken":true}');
    file_put_contents($nonBackup, '{"guard":true}');
    UserLog($attackerId, 'E2E_BAD_RESTORE', $token . ' marker survives invalid backup restore', time());
    $markerBeforeRestore = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}userlogs WHERE type='E2E_BAD_RESTORE' AND text LIKE '%" . e2e_sql_escape($token) . "%'");
    $traversalDelete = e2e_http_request('GET', e2e_admin_db_url($gameBase, $adminAuth['session'], '&action=delete&fname=' . rawurlencode('../config.php')), array(), $adminCookies);
    $nonBackupDelete = e2e_http_request('GET', e2e_admin_db_url($gameBase, $adminAuth['session'], '&action=delete&fname=' . rawurlencode(basename($nonBackup))), array(), $adminCookies);
    $badRestore = e2e_http_request('GET', e2e_admin_db_url($gameBase, $adminAuth['session'], '&action=restore&fname=' . rawurlencode(basename($badBackup))), array(), $adminCookies);
    $markerAfterRestore = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}userlogs WHERE type='E2E_BAD_RESTORE' AND text LIKE '%" . e2e_sql_escape($token) . "%'");
    $badBackupDelete = e2e_http_request('GET', e2e_admin_db_url($gameBase, $adminAuth['session'], '&action=delete&fname=' . rawurlencode(basename($badBackup))), array(), $adminCookies);
    clearstatcache(true, $badBackup);
    clearstatcache(true, $nonBackup);
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_db_backup_rejects_unsafe_filenames_and_invalid_restore_payloads',
        'checks' => array_merge(
            e2e_response_check($traversalDelete, 'admin DB traversal delete'),
            e2e_response_check($nonBackupDelete, 'admin DB non-backup delete'),
            e2e_response_check($badRestore, 'admin DB malformed restore'),
            e2e_response_check($badBackupDelete, 'admin DB valid backup delete'),
            array(
                e2e_case(is_file($nonBackup), 'non-backup temp file is not deleted through DB backup action'),
                e2e_case($markerBeforeRestore === 1 && $markerAfterRestore === 1, 'malformed backup restore does not deserialize or roll back live data', array('before' => $markerBeforeRestore, 'after' => $markerAfterRestore)),
                e2e_case(!is_file($badBackup), 'validly named malformed backup can still be deleted after restore rejection'),
                e2e_case(strpos($traversalDelete['body'], 'Backup deleted') === false, 'path traversal delete is not reported as a successful backup deletion'),
            )
        ),
    ));
    e2e_delete_file_if_exists($nonBackup);
    $nonBackup = null;
    $badBackup = null;

    $attackerAuth = e2e_prepare_session($attackerId, USER_TYPE_PLAYER, 'hardening-attacker');
    $attackerCookies = $attackerAuth['cookies'];
    $defenderAuth = e2e_prepare_session($defenderId, USER_TYPE_PLAYER, 'hardening-defender');
    $defenderCookies = $defenderAuth['cookies'];
    $payloadSubject = $token . ' "><script>alert("pm-subject")</script>';
    $payloadBody = $token . ' body <img src=x onerror=alert("pm-body")> </textarea><script>' . $token . '</script>';
    $sendMessage = e2e_http_request('POST', $gameBase . '/index.php?page=writemessages&session=' . rawurlencode($attackerAuth['session']) . '&gesendet=1&messageziel=' . $defenderId, array(
        'betreff' => $payloadSubject,
        'text' => $payloadBody,
    ), $attackerCookies);
    $messagePage = e2e_http_request('GET', $gameBase . '/index.php?page=messages&dsp=1&session=' . rawurlencode($defenderAuth['session']), array(), $defenderCookies);
    $createdMessageRows = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}messages WHERE owner_id={$defenderId} AND (subj LIKE '%" . e2e_sql_escape($token) . "%' OR text LIKE '%" . e2e_sql_escape($token) . "%')");
    $cases[] = e2e_finalize_case(array(
        'case' => 'private_message_subject_and_body_escape_user_html_payloads',
        'checks' => array_merge(
            e2e_response_check($sendMessage, 'send private XSS payload message'),
            e2e_response_check($messagePage, 'render private XSS payload message'),
            array(
                e2e_case($createdMessageRows >= 1, 'payload private message is created for render verification', array('messages' => $createdMessageRows)),
                e2e_case(strpos($messagePage['body'], $token) !== false, 'message page renders the payload marker'),
                e2e_case(!e2e_body_has_raw_payload_html($messagePage['body'], $token), 'message page does not render raw executable user HTML'),
            )
        ),
    ));

    e2e_prepare_session($attackerId, USER_TYPE_PLAYER, 'hardening-ally-founder');
    $createdAlly = CreateAlly($attackerId, 'H' . substr($token, -5), 'Hardening ' . substr($token, -6));
    $allyAuth = e2e_prepare_session($attackerId, USER_TYPE_PLAYER, 'hardening-ally-session');
    $allyCookies = $allyAuth['cookies'];
    $allyPayload = $token . '</textarea><script>alert("ally-text")</script><img src=x onerror=alert("ally-img")>';
    $badUrl = 'javascript:alert("' . $token . '")';
    $settingsTextPost = e2e_http_request('POST', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($allyAuth['session']) . '&a=11&d=1&t=1', array(
        'text' => $allyPayload,
    ), $allyCookies);
    $settingsUrlPost = e2e_http_request('POST', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($allyAuth['session']) . '&a=11&d=2', array(
        'hp' => $badUrl,
        'logo' => $badUrl,
        'bew' => '0',
        'fname' => '',
    ), $allyCookies);
    $settingsPage = e2e_http_request('GET', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($allyAuth['session']) . '&a=5&t=1', array(), $allyCookies);
    $allyMainPage = e2e_http_request('GET', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($allyAuth['session']), array(), $allyCookies);
    $allyInfoPage = e2e_http_request('GET', $gameBase . '/ainfo.php?allyid=' . $createdAlly);
    $allyRow = e2e_one_row("SELECT homepage, imglogo, exttext FROM {$db_prefix}ally WHERE ally_id={$createdAlly} LIMIT 1");
    $combinedAllyRender = $settingsPage['body'] . $allyMainPage['body'] . $allyInfoPage['body'];
    $cases[] = e2e_finalize_case(array(
        'case' => 'alliance_settings_escape_textareas_and_reject_script_urls',
        'checks' => array_merge(
            e2e_response_check($settingsTextPost, 'alliance XSS text POST'),
            e2e_response_check($settingsUrlPost, 'alliance bad URL POST'),
            e2e_response_check($settingsPage, 'alliance settings render'),
            e2e_response_check($allyMainPage, 'alliance main render'),
            e2e_response_check($allyInfoPage, 'alliance public info render'),
            array(
                e2e_case($allyRow !== null && $allyRow['homepage'] === '' && $allyRow['imglogo'] === '', 'script-scheme homepage and logo URLs are rejected at save time', $allyRow ?? array()),
                e2e_case($allyRow !== null && strpos($allyRow['exttext'], $token) !== false, 'alliance payload text is stored for render verification', $allyRow ?? array()),
                e2e_case(strpos($combinedAllyRender, '</textarea><script') === false, 'alliance settings textarea payload cannot break into executable HTML'),
                e2e_case(stripos($combinedAllyRender, 'href="redir.php?url=javascript:') === false && stripos($combinedAllyRender, 'pic.php?url=javascript:') === false, 'alliance pages do not render script-scheme URL attributes'),
                e2e_case(!e2e_body_has_raw_payload_html($allyMainPage['body'] . $allyInfoPage['body'], $token), 'alliance public/internal pages do not render raw executable user HTML'),
            )
        ),
    ));

    $uniAuth = e2e_prepare_session($attackerId, USER_TYPE_PLAYER, 'hardening-uni-cookie');
    $correctCookies = $uniAuth['cookies'];
    $wrongUni = ((int)$GlobalUni['num']) + 1;
    $wrongCookies = array('prsess_' . $attackerId . '_' . $wrongUni => $uniAuth['private']);
    $correctUniResponse = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($uniAuth['session']) . '&cp=' . $attackerPlanet, array(), $correctCookies);
    $wrongUniResponse = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($uniAuth['session']) . '&cp=' . $attackerPlanet, array(), $wrongCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'private_session_cookie_is_scoped_by_universe_number',
        'checks' => array_merge(
            e2e_response_check($correctUniResponse, 'correct universe private cookie request'),
            array(
                e2e_case(in_array($wrongUniResponse['status'], array(200, 301, 302, 303), true), 'wrong universe private cookie request returns an accepted denial status', array('status' => $wrongUniResponse['status'], 'location' => $wrongUniResponse['location'])),
                e2e_case(strpos($wrongUniResponse['body'], 'Error-ID:') !== false, 'wrong universe private cookie request renders the invalid-session error page'),
                e2e_case(strpos($correctUniResponse['body'], $attackerName) !== false, 'correct universe cookie authenticates the page'),
                e2e_case(strpos($wrongUniResponse['body'], $attackerName) === false, 'private cookie with a different universe suffix does not authenticate'),
            )
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'security_hardening_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    e2e_delete_file_if_exists($badBackup);
    e2e_delete_file_if_exists($nonBackup);
    dbquery("DELETE FROM {$db_prefix}userlogs WHERE type='E2E_BAD_RESTORE' AND text LIKE '%" . e2e_sql_escape($token) . "%'");
    dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$attackerId},{$defenderId}) AND (subj LIKE '%" . e2e_sql_escape($token) . "%' OR text LIKE '%" . e2e_sql_escape($token) . "%' OR msgfrom LIKE '%" . e2e_sql_escape($token) . "%')");
    dbquery("DELETE FROM {$db_prefix}reports WHERE subj LIKE '%" . e2e_sql_escape($token) . "%' OR text LIKE '%" . e2e_sql_escape($token) . "%' OR msgfrom LIKE '%" . e2e_sql_escape($token) . "%'");
    if ($createdAlly > 0) {
        DismissAlly($createdAlly);
    }
    e2e_restore_user($attackerSnapshot);
    e2e_restore_user($defenderSnapshot);
}

echo json_encode(array(
    'case_group' => 'http_security_hardening',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
