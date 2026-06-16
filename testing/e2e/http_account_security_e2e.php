<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_account_security_e2e.php';
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
loca_add('options', 'en');
loca_add('overview', 'en');
loca_add('reg', 'en');
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

function e2e_count(string $sql): int
{
    $row = e2e_one_row($sql);
    return $row === null ? 0 : (int)$row['cnt'];
}

function e2e_prepare_session(int $userId, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', validated=1, deact_ip=1, " .
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
        "SELECT player_id, name, oname, session, private_session, password, pemail, email, validated, validatemd, " .
        "disable, disable_until, vacation, banned, noattack, deact_ip, lang, skin, useskin, maxspy, maxfleetmsg, sortby, sortorder " .
        "FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1"
    );
}

function e2e_options_payload(array $user, array $overrides = array()): array
{
    $payload = array(
        'db_character' => $user['oname'],
        'db_password' => '',
        'newpass1' => '',
        'newpass2' => '',
        'db_email' => $user['pemail'],
        'dpath' => '/evolution/',
        'design' => 'on',
        'lang' => 'en',
        'settings_sort' => (string)$user['sortby'],
        'settings_order' => (string)$user['sortorder'],
        'noipcheck' => 'on',
        'spio_anz' => (string)$user['maxspy'],
        'settings_fleetactions' => (string)$user['maxfleetmsg'],
    );
    foreach ($overrides as $key => $value) {
        if ($value === null) {
            unset($payload[$key]);
        } else {
            $payload[$key] = $value;
        }
    }
    return $payload;
}

function e2e_login_request(string $base, string $name, string $password, array &$cookies): array
{
    return e2e_http_request(
        'GET',
        $base . '/game/reg/login2.php?login=' . rawurlencode($name) . '&pass=' . rawurlencode($password),
        array(),
        $cookies
    );
}

function e2e_restore_user(int $userId, array $snapshot): void
{
    global $db_prefix;

    dbquery(
        "UPDATE {$db_prefix}users SET " .
        "name='" . e2e_sql_escape($snapshot['name']) . "', " .
        "oname='" . e2e_sql_escape($snapshot['oname']) . "', " .
        "session='" . e2e_sql_escape($snapshot['session']) . "', " .
        "private_session='" . e2e_sql_escape($snapshot['private_session']) . "', " .
        "password='" . e2e_sql_escape($snapshot['password']) . "', " .
        "pemail='" . e2e_sql_escape($snapshot['pemail']) . "', " .
        "email='" . e2e_sql_escape($snapshot['email']) . "', " .
        "validated=" . (int)$snapshot['validated'] . ", " .
        "validatemd='" . e2e_sql_escape($snapshot['validatemd']) . "', " .
        "disable=" . (int)$snapshot['disable'] . ", " .
        "disable_until=" . (int)$snapshot['disable_until'] . ", " .
        "vacation=" . (int)$snapshot['vacation'] . ", " .
        "banned=" . (int)$snapshot['banned'] . ", " .
        "noattack=" . (int)$snapshot['noattack'] . ", " .
        "deact_ip=" . (int)$snapshot['deact_ip'] . ", " .
        "lang='" . e2e_sql_escape($snapshot['lang']) . "', " .
        "skin='" . e2e_sql_escape($snapshot['skin']) . "', " .
        "useskin=" . (int)$snapshot['useskin'] . ", " .
        "maxspy=" . (int)$snapshot['maxspy'] . ", " .
        "maxfleetmsg=" . (int)$snapshot['maxfleetmsg'] . ", " .
        "sortby=" . (int)$snapshot['sortby'] . ", " .
        "sortorder=" . (int)$snapshot['sortorder'] . " " .
        "WHERE player_id={$userId}"
    );
    InvalidateUserCache();
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$attackerName = getenv('OGAME_E2E_ATTACKER_NAME') ?: '';
$attackerPassword = getenv('OGAME_E2E_ATTACKER_PASSWORD') ?: '';
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderName = getenv('OGAME_E2E_DEFENDER_NAME') ?: '';
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$cases = array();
$originalAttacker = null;

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $attackerName === '' || $attackerPassword === '' || $defenderName === '') {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $originalAttacker = e2e_user_snapshot($attackerId);
    if ($originalAttacker === null) {
        throw new RuntimeException('Attacker fixture user is missing.');
    }
    $defenderOriginal = e2e_user_snapshot($defenderId);
    if ($defenderOriginal === null) {
        throw new RuntimeException('Defender fixture user is missing.');
    }

    $attackerAuth = e2e_prepare_session($attackerId, 'account-security-attacker');
    $defenderAuth = e2e_prepare_session($defenderId, 'account-security-defender');
    $attackerSession = $attackerAuth['session'];
    $attackerCookies = $attackerAuth['cookies'];

    $validResponse = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($attackerSession) . '&cp=' . $attackerPlanet, array(), $attackerCookies);
    $missingCookies = array();
    $missingCookieResponse = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($attackerSession) . '&cp=' . $attackerPlanet, array(), $missingCookies);
    $wrongCookies = array('prsess_' . $attackerId . '_' . $GlobalUni['num'] => $defenderAuth['private']);
    $wrongCookieResponse = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($attackerSession) . '&cp=' . $attackerPlanet, array(), $wrongCookies);
    $foreignCookies = $defenderAuth['cookies'];
    $foreignCookieResponse = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($attackerSession) . '&cp=' . $attackerPlanet, array(), $foreignCookies);
    $sessionAfterInvalidChecks = e2e_user_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'private_session_cookie_is_required_and_bound_to_public_session',
        'checks' => array_merge(
            e2e_response_check($validResponse),
            e2e_response_check($missingCookieResponse),
            e2e_response_check($wrongCookieResponse),
            e2e_response_check($foreignCookieResponse),
            array(
                e2e_case(strpos($validResponse['body'], $attackerName) !== false, 'valid public session and private cookie render the game page'),
                e2e_case(strpos($missingCookieResponse['body'], 'Error-ID:') !== false && strpos($missingCookieResponse['body'], $attackerName) === false, 'missing private cookie renders the invalid-session page'),
                e2e_case(strpos($wrongCookieResponse['body'], 'Error-ID:') !== false && strpos($wrongCookieResponse['body'], $attackerName) === false, 'wrong private cookie value renders the invalid-session page'),
                e2e_case(strpos($foreignCookieResponse['body'], 'Error-ID:') !== false && strpos($foreignCookieResponse['body'], $attackerName) === false, 'different user private cookie does not satisfy the public session'),
                e2e_case($sessionAfterInvalidChecks !== null && $sessionAfterInvalidChecks['session'] === $attackerSession, 'invalid private-cookie attempts do not destroy the valid public session', $sessionAfterInvalidChecks ?? array()),
            )
        ),
    ));

    $logoutResponse = e2e_http_request('GET', $gameBase . '/index.php?page=logout&session=' . rawurlencode($attackerSession), array(), $attackerCookies);
    $afterLogout = e2e_user_snapshot($attackerId);
    $oldSessionResponse = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($attackerSession) . '&cp=' . $attackerPlanet, array(), $attackerCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'logout_invalidates_public_session',
        'checks' => array_merge(e2e_response_check($logoutResponse), e2e_response_check($oldSessionResponse), array(
            e2e_case($afterLogout !== null && $afterLogout['session'] === '', 'logout clears the public session in the database', $afterLogout ?? array()),
            e2e_case(strpos($oldSessionResponse['body'], $attackerName) === false && strpos($oldSessionResponse['body'], 'overview') === false, 'old public session no longer renders a private game page'),
        )),
    ));

    $attackerAuth = e2e_prepare_session($attackerId, 'account-security-password');
    $attackerSession = $attackerAuth['session'];
    $attackerCookies = $attackerAuth['cookies'];
    $currentUser = e2e_user_snapshot($attackerId);
    $newPassword = 'E2Esec123';
    $wrongOldResponse = e2e_http_request('POST', $gameBase . '/index.php?page=options&session=' . rawurlencode($attackerSession) . '&mode=change', e2e_options_payload($currentUser, array(
        'db_password' => 'wrong-old-password',
        'newpass1' => $newPassword,
        'newpass2' => $newPassword,
    )), $attackerCookies);
    $afterWrongOld = e2e_user_snapshot($attackerId);
    $mismatchResponse = e2e_http_request('POST', $gameBase . '/index.php?page=options&session=' . rawurlencode($attackerSession) . '&mode=change', e2e_options_payload($currentUser, array(
        'db_password' => $attackerPassword,
        'newpass1' => $newPassword,
        'newpass2' => $newPassword . 'x',
    )), $attackerCookies);
    $afterMismatch = e2e_user_snapshot($attackerId);
    $validChangeResponse = e2e_http_request('POST', $gameBase . '/index.php?page=options&session=' . rawurlencode($attackerSession) . '&mode=change', e2e_options_payload($currentUser, array(
        'db_password' => $attackerPassword,
        'newpass1' => $newPassword,
        'newpass2' => $newPassword,
    )), $attackerCookies);
    $afterValidPasswordChange = e2e_user_snapshot($attackerId);
    $oldLoginCookies = array();
    $oldLoginResponse = e2e_login_request($base, $attackerName, $attackerPassword, $oldLoginCookies);
    $newLoginCookies = array();
    $newLoginResponse = e2e_login_request($base, $attackerName, $newPassword, $newLoginCookies);
    $afterNewLogin = e2e_user_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'password_change_requires_old_password_and_new_confirmation_then_reauthenticates',
        'checks' => array_merge(
            e2e_response_check($wrongOldResponse),
            e2e_response_check($mismatchResponse),
            e2e_response_check($validChangeResponse),
            e2e_response_check($oldLoginResponse),
            e2e_response_check($newLoginResponse),
            array(
                e2e_case($afterWrongOld !== null && $afterWrongOld['password'] === $currentUser['password'], 'wrong old password does not change the password hash'),
                e2e_case($afterMismatch !== null && $afterMismatch['password'] === $currentUser['password'], 'mismatched new password confirmation does not change the password hash'),
                e2e_case($afterValidPasswordChange !== null && $afterValidPasswordChange['password'] !== $currentUser['password'] && $afterValidPasswordChange['session'] === '', 'valid password change updates the hash and logs out the old session', $afterValidPasswordChange ?? array()),
                e2e_case(strpos($oldLoginResponse['location'], 'errorpage.php') !== false, 'old password login is rejected after password change', array('location' => $oldLoginResponse['location'])),
                e2e_case(strpos($newLoginResponse['location'], 'page=overview') !== false && $afterNewLogin !== null && $afterNewLogin['session'] !== '', 'new password login succeeds and issues a fresh session', array('location' => $newLoginResponse['location'], 'session' => $afterNewLogin['session'] ?? '')),
            )
        ),
    ));

    e2e_restore_user($attackerId, $originalAttacker);
    $attackerAuth = e2e_prepare_session($attackerId, 'account-security-email');
    $attackerSession = $attackerAuth['session'];
    $attackerCookies = $attackerAuth['cookies'];
    $currentUser = e2e_user_snapshot($attackerId);
    $defenderUser = e2e_user_snapshot($defenderId);
    $duplicateEmailResponse = e2e_http_request('POST', $gameBase . '/index.php?page=options&session=' . rawurlencode($attackerSession) . '&mode=change', e2e_options_payload($currentUser, array(
        'db_password' => $attackerPassword,
        'db_email' => $defenderUser['pemail'],
    )), $attackerCookies);
    $afterDuplicateEmail = e2e_user_snapshot($attackerId);
    $newEmail = 'account-security-' . substr(md5((string)microtime(true)), 0, 10) . '@example.local';
    $validEmailResponse = e2e_http_request('POST', $gameBase . '/index.php?page=options&session=' . rawurlencode($attackerSession) . '&mode=change', e2e_options_payload($currentUser, array(
        'db_password' => $attackerPassword,
        'db_email' => $newEmail,
    )), $attackerCookies);
    $afterEmailChange = e2e_user_snapshot($attackerId);
    $emailQueue = e2e_one_row("SELECT task_id, owner_id, type, end FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_CHANGE_EMAIL . "' ORDER BY task_id DESC LIMIT 1");
    $validateCookies = array();
    $validateResponse = e2e_http_request('GET', $gameBase . '/validate.php?ack=' . rawurlencode($afterEmailChange['validatemd'] ?? ''), array(), $validateCookies);
    $afterEmailValidate = e2e_user_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'email_change_rejects_duplicates_then_validation_promotes_permanent_email',
        'checks' => array_merge(
            e2e_response_check($duplicateEmailResponse),
            e2e_response_check($validEmailResponse),
            e2e_response_check($validateResponse),
            array(
                e2e_case($afterDuplicateEmail !== null && $afterDuplicateEmail['pemail'] === $currentUser['pemail'] && $afterDuplicateEmail['email'] === $currentUser['email'], 'duplicate email does not change account email fields', $afterDuplicateEmail ?? array()),
                e2e_case($afterEmailChange !== null && (int)$afterEmailChange['validated'] === 0 && $afterEmailChange['email'] === $newEmail && $afterEmailChange['pemail'] === $currentUser['pemail'] && $afterEmailChange['validatemd'] !== '', 'valid email change stores a pending email and validation code', $afterEmailChange ?? array()),
                e2e_case($emailQueue !== null && (int)$emailQueue['owner_id'] === $attackerId, 'valid email change creates a pending permanent-email queue task', $emailQueue ?? array()),
                e2e_case(strpos($validateResponse['location'], 'page=overview') !== false, 'validation link logs the user into the overview page', array('location' => $validateResponse['location'])),
                e2e_case($afterEmailValidate !== null && (int)$afterEmailValidate['validated'] === 1 && $afterEmailValidate['pemail'] === $newEmail && $afterEmailValidate['email'] === $newEmail && $afterEmailValidate['validatemd'] === '', 'validation promotes pending email to permanent email', $afterEmailValidate ?? array()),
            )
        ),
    ));

    e2e_restore_user($attackerId, $originalAttacker);
    $attackerAuth = e2e_prepare_session($attackerId, 'account-security-delete');
    $attackerSession = $attackerAuth['session'];
    $attackerCookies = $attackerAuth['cookies'];
    $currentUser = e2e_user_snapshot($attackerId);
    $scheduleDeleteResponse = e2e_http_request('POST', $gameBase . '/index.php?page=options&session=' . rawurlencode($attackerSession) . '&mode=change', e2e_options_payload($currentUser, array(
        'db_deaktjava' => 'on',
    )), $attackerCookies);
    $afterScheduleDelete = e2e_user_snapshot($attackerId);
    $cancelDeleteResponse = e2e_http_request('POST', $gameBase . '/index.php?page=options&session=' . rawurlencode($attackerSession) . '&mode=change', e2e_options_payload($currentUser), $attackerCookies);
    $afterCancelDelete = e2e_user_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'account_delete_schedule_and_cancel',
        'checks' => array_merge(e2e_response_check($scheduleDeleteResponse), e2e_response_check($cancelDeleteResponse), array(
            e2e_case($afterScheduleDelete !== null && (int)$afterScheduleDelete['disable'] === 1 && (int)$afterScheduleDelete['disable_until'] > time(), 'account deletion can be scheduled from options', $afterScheduleDelete ?? array()),
            e2e_case($afterCancelDelete !== null && (int)$afterCancelDelete['disable'] === 0 && (int)$afterCancelDelete['disable_until'] === 0, 'account deletion schedule can be canceled from options', $afterCancelDelete ?? array()),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'account_security_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($attackerId > 0 && $originalAttacker !== null) {
        e2e_restore_user($attackerId, $originalAttacker);
        dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_CHANGE_EMAIL . "'");
    }
}

echo json_encode(array(
    'case_group' => 'http_account_security',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
