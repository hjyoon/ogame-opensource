<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_registration_validation_e2e.php';
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
    $runtimeError = e2e_has_runtime_error($response['body']);
    $errorExcerpt = array();
    if ($runtimeError && preg_match('/.{0,120}(Fatal error|Parse error|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined).{0,180}/is', $response['body'], $m)) {
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

function e2e_user_snapshot_by_name(string $name): ?array
{
    global $db_prefix;
    $name = mb_strtolower($name, 'UTF-8');
    return e2e_one_row(
        "SELECT player_id, name, oname, session, private_session, password, pemail, email, validated, validatemd, hplanetid " .
        "FROM {$db_prefix}users WHERE name='" . e2e_sql_escape($name) . "' LIMIT 1"
    );
}

function e2e_wait_for_user_activation(string $name): ?array
{
    $last = null;
    for ($i = 0; $i < 10; $i++) {
        $last = e2e_user_snapshot_by_name($name);
        if ($last !== null && (int)$last['validated'] === 1 && $last['validatemd'] === '') {
            return $last;
        }
        usleep(100000);
    }
    return $last;
}

function e2e_user_snapshot_by_id(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT player_id, name, oname, session, private_session, password, pemail, email, validated, validatemd, hplanetid " .
        "FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1"
    );
}

function e2e_user_snapshot_by_session(string $session): ?array
{
    global $db_prefix;
    if ($session === '') {
        return null;
    }
    return e2e_one_row(
        "SELECT player_id, name, oname, session, private_session, password, pemail, email, validated, validatemd, hplanetid " .
        "FROM {$db_prefix}users WHERE session='" . e2e_sql_escape($session) . "' LIMIT 1"
    );
}

function e2e_wait_for_user_activation_by_id(int $userId): ?array
{
    $last = null;
    for ($i = 0; $i < 10; $i++) {
        $last = e2e_user_snapshot_by_id($userId);
        if ($last !== null && (int)$last['validated'] === 1 && $last['validatemd'] === '') {
            return $last;
        }
        usleep(100000);
    }
    return $last;
}

function e2e_user_count_by_name(string $name): int
{
    global $db_prefix;
    $name = mb_strtolower($name, 'UTF-8');
    $row = e2e_one_row("SELECT COUNT(*) AS cnt FROM {$db_prefix}users WHERE name='" . e2e_sql_escape($name) . "'");
    return $row === null ? 0 : (int)$row['cnt'];
}

function e2e_remove_user_by_name(string $name): void
{
    $row = e2e_user_snapshot_by_name($name);
    if ($row !== null) {
        RemoveUser((int)$row['player_id'], time());
    }
}

function e2e_clear_registration_rate_limit(): void
{
    global $db_prefix;
    dbquery("DELETE FROM {$db_prefix}iplogs WHERE reg=1");
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

function e2e_extract_session(string $text): string
{
    if (preg_match('/session=([a-f0-9]{12})/i', $text, $m)) {
        return $m[1];
    }
    return '';
}

function e2e_extract_activation_link(string $body): string
{
    if (preg_match('~https?://\S+/game/validate\.php\?ack=[a-f0-9]+~i', $body, $m)) {
        return trim($m[0]);
    }
    return '';
}

function e2e_extract_registered_password(string $body): string
{
    if (preg_match('/Password:\s*([a-z0-9]+)/i', $body, $m)) {
        return trim($m[1]);
    }
    return '';
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$publicBase = rtrim(getenv('OGAME_E2E_HTTP_PUBLIC_BASE') ?: 'http://server', '/');
$gameBase = $base . '/game';
$publicGameBase = $publicBase . '/game';
$token = substr(md5((string)microtime(true)), 0, 8);
$successName = 'e2erg_' . $token;
$successEmail = $successName . '@example.local';
$invalidEmailName = 'e2ebad_' . substr($token, 0, 6);
$shortName = 'ab';
$specialName = 'bad<name';
$missingRedirectName = 'e2emiss_' . substr($token, 0, 6);
$cases = array();

try {
    if ($attackerId <= 0) {
        throw new RuntimeException('Fixture attacker user is missing.');
    }
    $attackerUser = e2e_user_snapshot_by_id($attackerId);
    if ($attackerUser === null) {
        throw new RuntimeException('Fixture attacker row is missing.');
    }

    $formCookies = array();
    $formResponse = e2e_http_request('GET', $gameBase . '/reg/new.php', array(), $formCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'registration_form_renders',
        'checks' => array_merge(e2e_response_check($formResponse, 'Registration form'), array(
            e2e_case(strpos($formResponse['body'], 'name="character"') !== false, 'registration form contains the character input'),
            e2e_case(strpos($formResponse['body'], 'name="email"') !== false, 'registration form contains the email input'),
            e2e_case(strpos($formResponse['body'], 'name="agb"') !== false, 'registration form contains the AGB checkbox'),
        )),
    ));

    $missingAgbCookies = array();
    $missingAgbResponse = e2e_http_request('POST', $gameBase . '/reg/new.php', array(
        'character' => 'e2eagb' . substr($token, 0, 5),
        'email' => 'e2eagb' . substr($token, 0, 5) . '@example.local',
    ), $missingAgbCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'missing_agb_is_rejected',
        'checks' => array_merge(e2e_response_check($missingAgbResponse, 'Missing-AGB registration'), array(
            e2e_case(strpos($missingAgbResponse['body'], 'must accept the Basic Policies') !== false, 'missing AGB renders the expected validation error'),
        )),
    ));

    $invalidEmailCookies = array();
    $invalidEmailResponse = e2e_http_request('POST', $gameBase . '/reg/new.php', array(
        'character' => $invalidEmailName,
        'email' => 'not-an-email',
        'agb' => 'on',
    ), $invalidEmailCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'invalid_email_is_rejected_without_creating_a_user',
        'checks' => array_merge(e2e_response_check($invalidEmailResponse, 'Invalid-email registration'), array(
            e2e_case(strpos($invalidEmailResponse['body'], 'is invalid!') !== false, 'invalid email renders the email validation error'),
            e2e_case(e2e_user_snapshot_by_name($invalidEmailName) === null, 'invalid email does not create a user row'),
        )),
    ));

    $duplicateNameCookies = array();
    $duplicateNameResponse = e2e_http_request('POST', $gameBase . '/reg/new.php', array(
        'character' => $attackerUser['oname'],
        'email' => 'dup-name-' . $token . '@example.local',
        'agb' => 'on',
    ), $duplicateNameCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'duplicate_username_is_rejected',
        'checks' => array_merge(e2e_response_check($duplicateNameResponse, 'Duplicate-name registration'), array(
            e2e_case(strpos($duplicateNameResponse['body'], 'already exists') !== false, 'duplicate username renders the duplicate-name error'),
            e2e_case(e2e_user_count_by_name($attackerUser['name']) === 1, 'duplicate username does not create an extra user row'),
        )),
    ));

    $duplicateEmailCookies = array();
    $duplicateEmailResponse = e2e_http_request('POST', $gameBase . '/reg/new.php', array(
        'character' => 'dupeml' . substr($token, 0, 6),
        'email' => $attackerUser['pemail'],
        'agb' => 'on',
    ), $duplicateEmailCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'duplicate_email_is_rejected',
        'checks' => array_merge(e2e_response_check($duplicateEmailResponse, 'Duplicate-email registration'), array(
            e2e_case(strpos($duplicateEmailResponse['body'], 'already exists!') !== false, 'duplicate email renders the duplicate-email error'),
        )),
    ));

    $shortNameCookies = array();
    $shortNameResponse = e2e_http_request('POST', $gameBase . '/reg/new.php', array(
        'character' => $shortName,
        'email' => 'short-' . $token . '@example.local',
        'agb' => 'on',
    ), $shortNameCookies);
    $specialNameCookies = array();
    $specialNameResponse = e2e_http_request('POST', $gameBase . '/reg/new.php', array(
        'character' => $specialName,
        'email' => 'special-' . $token . '@example.local',
        'agb' => 'on',
    ), $specialNameCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'short_and_special_character_names_are_rejected',
        'checks' => array_merge(
            e2e_response_check($shortNameResponse, 'Short-name registration'),
            e2e_response_check($specialNameResponse, 'Special-name registration'),
            array(
                e2e_case(strpos($shortNameResponse['body'], 'contains invalid characters or too few/many characters') !== false, 'short name renders the character validation error'),
                e2e_case(strpos($specialNameResponse['body'], 'contains invalid characters or too few/many characters') !== false, 'special-character name renders the character validation error'),
                e2e_case(e2e_user_snapshot_by_name($shortName) === null && e2e_user_snapshot_by_name($specialName) === null, 'invalid names do not create user rows'),
            )
        ),
    ));

    $missingRedirectCookies = array();
    $missingRedirectResponse = e2e_http_request('POST', $gameBase . '/reg/newredirect.php', array(
        'agb' => 'on',
        'universe' => '127.0.0.1',
    ), $missingRedirectCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'newredirect_missing_required_fields_is_rejected_without_runtime_errors',
        'checks' => array_merge(e2e_response_check($missingRedirectResponse, 'Missing-field newredirect registration'), array(
            e2e_case(strpos($missingRedirectResponse['body'], 'errorCode=107') !== false, 'missing password is rejected through the expected redirect code'),
            e2e_case(e2e_user_snapshot_by_name($missingRedirectName) === null, 'missing-field newredirect request does not create a user row'),
        )),
    ));

    e2e_clear_registration_rate_limit();
    e2e_mailhog_clear();
    $successCookies = array();
    $successResponse = e2e_http_request('POST', $publicGameBase . '/reg/new.php', array(
        'character' => $successName,
        'email' => $successEmail,
        'agb' => 'on',
    ), $successCookies);
    $registeredUser = e2e_user_snapshot_by_name($successName);
    $welcomeMail = e2e_find_mail_to($successEmail, 'activate your account');
    $welcomeMailBody = $welcomeMail === null ? '' : e2e_message_body($welcomeMail);
    $welcomePassword = e2e_extract_registered_password($welcomeMailBody);
    $activationLink = e2e_extract_activation_link($welcomeMailBody);
    $cases[] = e2e_finalize_case(array(
        'case' => 'successful_registration_sends_welcome_mail_and_activation_link',
        'checks' => array_merge(e2e_response_check($successResponse, 'Successful registration'), array(
            e2e_case(strpos($successResponse['body'], 'Registration was a success!') !== false, 'successful registration renders the success page'),
            e2e_case($registeredUser !== null && (int)$registeredUser['validated'] === 0 && $registeredUser['validatemd'] !== '', 'successful registration creates an unvalidated user with an activation code', $registeredUser ?? array()),
            e2e_case($welcomeMail !== null, 'successful registration sends a welcome mail through MailHog', array('recipients' => $welcomeMail === null ? array() : e2e_message_recipients($welcomeMail))),
            e2e_case($welcomePassword !== '', 'welcome mail contains the generated password'),
            e2e_case($activationLink !== '', 'welcome mail contains an activation link', array('link' => $activationLink)),
        )),
    ));

    $preActivationLoginCookies = array();
    $preActivationLogin = $welcomePassword === '' ? array('status' => 0, 'location' => '', 'headers' => array(), 'body' => '') : e2e_login_request($publicBase, $successName, $welcomePassword, $preActivationLoginCookies);
    $preActivationSession = e2e_extract_session($preActivationLogin['location'] . $preActivationLogin['body']);
    if ($preActivationSession === '' && $registeredUser !== null) {
        $reloadedUser = e2e_user_snapshot_by_name($successName);
        $preActivationSession = $reloadedUser['session'] ?? '';
    }
    $preActivationOverview = $preActivationSession === '' ? array('status' => 0, 'location' => '', 'headers' => array(), 'body' => '') : e2e_http_request(
        'GET',
        $publicGameBase . '/index.php?page=overview&session=' . rawurlencode($preActivationSession) . '&lgn=1',
        array(),
        $preActivationLoginCookies
    );
    $activationCookies = array();
    $activationResponse = $activationLink === '' ? array('status' => 0, 'location' => '', 'headers' => array(), 'body' => '') : e2e_http_request('GET', $activationLink, array(), $activationCookies);
    $activationSession = e2e_extract_session($activationResponse['location'] . $activationResponse['body']);
    $afterActivation = $registeredUser === null ? e2e_wait_for_user_activation($successName) : e2e_wait_for_user_activation_by_id((int)$registeredUser['player_id']);
    $activationSessionUser = e2e_user_snapshot_by_session($activationSession);
    if ($activationSession === '' && $afterActivation !== null) {
        $activationSession = $afterActivation['session'] ?? '';
    }
    $postActivationOverview = $activationSession === '' ? array('status' => 0, 'location' => '', 'headers' => array(), 'body' => '') : e2e_http_request(
        'GET',
        $publicGameBase . '/index.php?page=overview&session=' . rawurlencode($activationSession) . '&lgn=1',
        array(),
        $activationCookies
    );
    $cases[] = e2e_finalize_case(array(
        'case' => 'activation_link_validates_account_and_clears_pre_activation_warning',
        'checks' => array_merge(
            $welcomePassword === '' ? array(e2e_case(false, 'welcome mail password was extracted before pre-activation login')) : e2e_response_check($preActivationLogin, 'Pre-activation login'),
            $preActivationSession === '' ? array(e2e_case(false, 'pre-activation session token was produced before overview render')) : e2e_response_check($preActivationOverview, 'Pre-activation overview'),
            $activationLink === '' ? array(e2e_case(false, 'activation link was extracted before activation request')) : e2e_response_check($activationResponse, 'Activation link request'),
            $activationSession === '' ? array(e2e_case(false, 'activation produced a session token before overview render')) : e2e_response_check($postActivationOverview, 'Post-activation overview'),
            array(
                e2e_case(strpos($preActivationLogin['location'], 'page=overview') !== false, 'pre-activation login still reaches the overview page', array('location' => $preActivationLogin['location'])),
                e2e_case(stripos($preActivationOverview['body'], 'has not been activated yet') !== false, 'pre-activation overview displays the activation warning'),
                e2e_case(strpos($activationResponse['location'], 'page=overview') !== false, 'activation link redirects into the overview page', array('location' => $activationResponse['location'])),
                e2e_case($afterActivation !== null && (int)$afterActivation['validated'] === 1 && $afterActivation['validatemd'] === '' && $afterActivation['pemail'] === $successEmail, 'activation marks the account validated and clears the activation code', array('by_id' => $afterActivation, 'by_session' => $activationSessionUser)),
                e2e_case(stripos($postActivationOverview['body'], 'has not been activated yet') === false, 'post-activation overview no longer displays the activation warning'),
            )
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'registration_validation_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    e2e_remove_user_by_name($successName);
}

echo json_encode(array(
    'case_group' => 'http_registration_validation',
    'base' => $base,
    'public_base' => $publicBase,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
