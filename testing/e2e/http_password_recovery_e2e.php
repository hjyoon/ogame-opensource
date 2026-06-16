<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_password_recovery_e2e.php';
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
loca_add('reg', 'en');

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

function e2e_mailhog_request(string $method, string $path): array
{
    $context = stream_context_create(array(
        'http' => array(
            'method' => $method,
            'ignore_errors' => true,
            'timeout' => 10,
        ),
    ));
    $body = @file_get_contents('http://mailhog:8025' . $path, false, $context);
    $responseHeaders = $http_response_header ?? array();
    $status = 0;
    foreach ($responseHeaders as $header) {
        if (preg_match('/^HTTP\/\S+\s+(\d+)/', $header, $m)) {
            $status = (int)$m[1];
        }
    }

    return array('status' => $status, 'body' => $body === false ? '' : $body);
}

function e2e_mailhog_clear(): bool
{
    $response = e2e_mailhog_request('DELETE', '/api/v1/messages');
    return in_array($response['status'], array(200, 202, 204), true);
}

function e2e_mailhog_messages(): array
{
    $response = e2e_mailhog_request('GET', '/api/v2/messages');
    if ($response['status'] !== 200) {
        return array();
    }
    $json = json_decode($response['body'], true);
    if (!is_array($json)) {
        return array();
    }
    if (isset($json['items']) && is_array($json['items'])) {
        return $json['items'];
    }
    if (isset($json['Items']) && is_array($json['Items'])) {
        return $json['Items'];
    }
    return array();
}

function e2e_message_body(array $message): string
{
    $content = $message['Content'] ?? array();
    if (is_array($content) && isset($content['Body'])) {
        return (string)$content['Body'];
    }
    $raw = $message['Raw'] ?? array();
    if (is_array($raw) && isset($raw['Data'])) {
        return (string)$raw['Data'];
    }
    return '';
}

function e2e_message_recipients(array $message): array
{
    $recipients = array();
    foreach (($message['To'] ?? array()) as $to) {
        if (!is_array($to)) {
            continue;
        }
        $mailbox = $to['Mailbox'] ?? '';
        $domain = $to['Domain'] ?? '';
        if ($mailbox !== '' && $domain !== '') {
            $recipients[] = strtolower($mailbox . '@' . $domain);
        }
    }
    return $recipients;
}

function e2e_find_mail_to(string $email, string $contains = ''): ?array
{
    $email = strtolower($email);
    for ($i = 0; $i < 10; $i++) {
        foreach (e2e_mailhog_messages() as $message) {
            $body = e2e_message_body($message);
            if (in_array($email, e2e_message_recipients($message), true) && ($contains === '' || strpos($body, $contains) !== false)) {
                return $message;
            }
        }
        usleep(200000);
    }
    return null;
}

function e2e_extract_recovery_password(string $body): string
{
    if (preg_match('/your password for .*? is:\s+([a-z0-9]+)\s+You may log in/is', $body, $m)) {
        return trim($m[1]);
    }
    return '';
}

function e2e_case(bool $pass, string $message, array $context = array()): array
{
    return array('pass' => $pass, 'message' => $message, 'context' => $context);
}

function e2e_has_runtime_error(string $body): bool
{
    return preg_match('/Fatal error|Parse error|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined/i', $body) === 1;
}

function e2e_response_check(array $response, string $label = 'HTTP request'): array
{
    $body = $response['body'];
    $runtimeError = e2e_has_runtime_error($body);
    $errorExcerpt = array();
    if ($runtimeError && preg_match('/.{0,120}(Fatal error|Parse error|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined).{0,180}/is', $body, $m)) {
        $errorExcerpt = array('excerpt' => trim(strip_tags($m[0])));
    }

    return array(
        e2e_case(in_array($response['status'], array(200, 301, 302, 303), true), $label . ' returns an accepted status', array('status' => $response['status'], 'location' => $response['location'])),
        e2e_case(!$runtimeError, $label . ' body has no PHP runtime error marker', $errorExcerpt),
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

function e2e_user_by_name(string $name): ?array
{
    global $db_prefix;
    $name = mb_strtolower($name, 'UTF-8');
    return e2e_one_row("SELECT player_id, hplanetid FROM {$db_prefix}users WHERE name='" . e2e_sql_escape($name) . "' LIMIT 1");
}

function e2e_remove_user_by_name(string $name): void
{
    $row = e2e_user_by_name($name);
    if ($row !== null) {
        RemoveUser((int)$row['player_id'], time());
    }
}

function e2e_user_snapshot(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT player_id, name, oname, session, private_session, password, pemail, email, validated, validatemd, " .
        "disable, disable_until, vacation, banned, noattack, deact_ip, lang, skin, useskin " .
        "FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1"
    );
}

function e2e_restore_user(array $snapshot): void
{
    global $db_prefix;
    $userId = (int)$snapshot['player_id'];
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
        "useskin=" . (int)$snapshot['useskin'] . " " .
        "WHERE player_id={$userId}"
    );
    InvalidateUserCache();
}

function e2e_create_recovery_user(string $name, string $password, string $email): array
{
    global $db_prefix;
    e2e_remove_user_by_name($name);
    $id = CreateUser($name, $password, $email, false);
    $session = substr(md5('password-recovery-session-' . $id), 0, 12);
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "validated=1, validatemd='', deact_ip=1, vacation=0, vacation_until=0, banned=0, banned_until=0, " .
        "noattack=0, noattack_until=0, disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1 " .
        "WHERE player_id={$id}"
    );
    InvalidateUserCache();

    $snapshot = e2e_user_snapshot($id);
    if ($snapshot === null) {
        throw new RuntimeException('Failed to create password recovery user.');
    }
    return $snapshot;
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

$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$runToken = substr(md5((string)microtime(true)), 0, 10);
$recoveryName = 'e2epw_' . $runToken;
$recoveryPassword = 'E2E_reset123';
$recoveryEmail = $recoveryName . '@example.local';
$temporaryEmail = $recoveryName . '.temporary@example.local';
$cases = array();
$createdUser = null;

try {
    $createdUser = e2e_create_recovery_user($recoveryName, $recoveryPassword, $recoveryEmail);

    $formCookies = array();
    $formResponse = e2e_http_request('GET', $gameBase . '/reg/mail.php', array(), $formCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'password_recovery_form_renders',
        'checks' => array_merge(e2e_response_check($formResponse, 'Password recovery form'), array(
            e2e_case(strpos($formResponse['body'], 'Send Password') !== false, 'form shows the password recovery title'),
            e2e_case(strpos($formResponse['body'], 'name="email"') !== false && strpos($formResponse['body'], 'fa_pass.php') !== false, 'form posts an email address to fa_pass.php'),
        )),
    ));

    e2e_mailhog_clear();
    $missingEmailCookies = array();
    $missingEmailResponse = e2e_http_request('POST', $gameBase . '/reg/fa_pass.php', array(), $missingEmailCookies);
    $afterMissingEmail = e2e_user_snapshot((int)$createdUser['player_id']);
    $cases[] = e2e_finalize_case(array(
        'case' => 'missing_email_post_is_rejected_without_account_or_mail_changes',
        'checks' => array_merge(e2e_response_check($missingEmailResponse, 'Missing-email recovery request'), array(
            e2e_case(strpos($missingEmailResponse['body'], "doesn't exist") !== false, 'missing email request renders the generic recovery error'),
            e2e_case($afterMissingEmail !== null && $afterMissingEmail['password'] === $createdUser['password'] && $afterMissingEmail['session'] === $createdUser['session'], 'missing email request does not change password or session', $afterMissingEmail ?? array()),
            e2e_case(count(e2e_mailhog_messages()) === 0, 'missing email request does not send mail'),
        )),
    ));

    e2e_mailhog_clear();
    $unknownEmailCookies = array();
    $unknownEmailResponse = e2e_http_request('POST', $gameBase . '/reg/fa_pass.php', array('email' => 'missing-' . $runToken . '@example.local'), $unknownEmailCookies);
    $afterUnknownEmail = e2e_user_snapshot((int)$createdUser['player_id']);
    $cases[] = e2e_finalize_case(array(
        'case' => 'unknown_email_is_rejected_without_account_or_mail_changes',
        'checks' => array_merge(e2e_response_check($unknownEmailResponse, 'Unknown-email recovery request'), array(
            e2e_case(strpos($unknownEmailResponse['body'], "doesn't exist") !== false, 'unknown email request renders the generic recovery error'),
            e2e_case($afterUnknownEmail !== null && $afterUnknownEmail['password'] === $createdUser['password'] && $afterUnknownEmail['session'] === $createdUser['session'], 'unknown email request does not change password or session', $afterUnknownEmail ?? array()),
            e2e_case(count(e2e_mailhog_messages()) === 0, 'unknown email request does not send mail'),
        )),
    ));

    e2e_mailhog_clear();
    $permanentEmailCookies = array();
    $permanentEmailResponse = e2e_http_request('POST', $gameBase . '/reg/fa_pass.php', array('email' => $recoveryEmail), $permanentEmailCookies);
    $afterPermanentReset = e2e_user_snapshot((int)$createdUser['player_id']);
    $permanentMail = e2e_find_mail_to($recoveryEmail, 'your password for');
    $permanentMailBody = $permanentMail === null ? '' : e2e_message_body($permanentMail);
    $permanentNewPassword = e2e_extract_recovery_password($permanentMailBody);
    $oldPasswordCookies = array();
    $oldPasswordLoginResponse = e2e_login_request($base, $recoveryName, $recoveryPassword, $oldPasswordCookies);
    $newPasswordCookies = array();
    $newPasswordLoginResponse = $permanentNewPassword === '' ? array('status' => 0, 'location' => '', 'headers' => array(), 'body' => '') : e2e_login_request($base, $recoveryName, $permanentNewPassword, $newPasswordCookies);
    $afterPermanentLogin = e2e_user_snapshot((int)$createdUser['player_id']);
    $cases[] = e2e_finalize_case(array(
        'case' => 'permanent_email_recovery_sends_new_password_and_invalidates_old_login',
        'checks' => array_merge(
            e2e_response_check($permanentEmailResponse, 'Permanent-email recovery request'),
            e2e_response_check($oldPasswordLoginResponse, 'Old password login after recovery'),
            $permanentNewPassword === '' ? array(e2e_case(false, 'new password was extracted before login')) : e2e_response_check($newPasswordLoginResponse, 'Recovered password login'),
            array(
                e2e_case(strpos($permanentEmailResponse['body'], 'Your password has been sent to ' . $recoveryName) !== false, 'permanent email request renders the success message'),
                e2e_case($afterPermanentReset !== null && $afterPermanentReset['password'] !== $createdUser['password'] && $afterPermanentReset['session'] === '', 'permanent email recovery changes password and clears active session', $afterPermanentReset ?? array()),
                e2e_case($permanentMail !== null, 'permanent email recovery sends mail to the permanent address', array('recipients' => $permanentMail === null ? array() : e2e_message_recipients($permanentMail))),
                e2e_case(strpos($permanentMailBody, $recoveryName) !== false && strpos($permanentMailBody, (string)$GlobalUni['num']) !== false, 'recovery email body includes the player and universe'),
                e2e_case($permanentNewPassword !== '' && $afterPermanentReset !== null && $afterPermanentReset['password'] === md5($permanentNewPassword . $GLOBALS['db_secret']), 'email password matches the stored password hash'),
                e2e_case(strpos($oldPasswordLoginResponse['location'], 'errorpage.php') !== false, 'old password is rejected after recovery', array('location' => $oldPasswordLoginResponse['location'])),
                e2e_case(strpos($newPasswordLoginResponse['location'], 'page=overview') !== false && $afterPermanentLogin !== null && $afterPermanentLogin['session'] !== '', 'recovered password logs into the overview page', array('location' => $newPasswordLoginResponse['location'], 'session' => $afterPermanentLogin['session'] ?? '')),
            )
        ),
    ));

    e2e_restore_user($createdUser);
    dbquery(
        "UPDATE {$db_prefix}users SET email='" . e2e_sql_escape($temporaryEmail) . "', validated=0, " .
        "validatemd='" . e2e_sql_escape(md5('temporary-' . $runToken)) . "' WHERE player_id=" . (int)$createdUser['player_id']
    );
    InvalidateUserCache();
    $temporaryBaseline = e2e_user_snapshot((int)$createdUser['player_id']);
    e2e_mailhog_clear();
    $temporaryEmailCookies = array();
    $temporaryEmailResponse = e2e_http_request('POST', $gameBase . '/reg/fa_pass.php', array('email' => $temporaryEmail), $temporaryEmailCookies);
    $afterTemporaryReset = e2e_user_snapshot((int)$createdUser['player_id']);
    $temporaryMail = e2e_find_mail_to($recoveryEmail, 'your password for');
    $temporaryMailBody = $temporaryMail === null ? '' : e2e_message_body($temporaryMail);
    $temporaryNewPassword = e2e_extract_recovery_password($temporaryMailBody);
    $temporaryPasswordCookies = array();
    $temporaryPasswordLoginResponse = $temporaryNewPassword === '' ? array('status' => 0, 'location' => '', 'headers' => array(), 'body' => '') : e2e_login_request($base, $recoveryName, $temporaryNewPassword, $temporaryPasswordCookies);
    $afterTemporaryLogin = e2e_user_snapshot((int)$createdUser['player_id']);
    $cases[] = e2e_finalize_case(array(
        'case' => 'temporary_email_recovery_sends_mail_to_permanent_address',
        'checks' => array_merge(
            e2e_response_check($temporaryEmailResponse, 'Temporary-email recovery request'),
            $temporaryNewPassword === '' ? array(e2e_case(false, 'temporary-email password was extracted before login')) : e2e_response_check($temporaryPasswordLoginResponse, 'Temporary-email recovered password login'),
            array(
                e2e_case($afterTemporaryReset !== null && $temporaryBaseline !== null && $afterTemporaryReset['password'] !== $temporaryBaseline['password'] && $afterTemporaryReset['session'] === '', 'temporary email recovery changes password and clears active session', $afterTemporaryReset ?? array()),
                e2e_case($temporaryMail !== null && in_array(strtolower($recoveryEmail), e2e_message_recipients($temporaryMail), true), 'temporary email lookup sends mail only to the permanent address', array('recipients' => $temporaryMail === null ? array() : e2e_message_recipients($temporaryMail))),
                e2e_case($temporaryNewPassword !== '' && $afterTemporaryReset !== null && $afterTemporaryReset['password'] === md5($temporaryNewPassword . $GLOBALS['db_secret']), 'temporary-email recovery password matches the stored password hash'),
                e2e_case(strpos($temporaryPasswordLoginResponse['location'], 'page=overview') !== false && $afterTemporaryLogin !== null && $afterTemporaryLogin['session'] !== '', 'temporary-email recovered password logs into the overview page', array('location' => $temporaryPasswordLoginResponse['location'], 'session' => $afterTemporaryLogin['session'] ?? '')),
            )
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'password_recovery_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    e2e_remove_user_by_name($recoveryName);
}

echo json_encode(array(
    'case_group' => 'http_password_recovery',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
