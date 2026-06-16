<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_localization_edges_e2e.php';
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
loca_add('common', 'de');
loca_add('menu', 'en');
loca_add('menu', 'de');
loca_add('options', 'en');
loca_add('options', 'de');
loca_add('overview', 'en');
loca_add('overview', 'de');

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

function e2e_prepare_session(int $userId, string $label, string $lang = 'en'): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', admin=" . USER_TYPE_PLAYER . ", validated=1, deact_ip=1, " .
        "vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, " .
        "disable=0, disable_until=0, lang='" . e2e_sql_escape($lang) . "', skin='/evolution/', useskin=1 WHERE player_id={$userId}"
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
        "SELECT player_id, name, oname, session, private_session, password, pemail, email, validated, validatemd, admin, deact_ip, " .
        "vacation, vacation_until, banned, banned_until, noattack, noattack_until, disable, disable_until, lang, skin, useskin, " .
        "maxspy, maxfleetmsg, sortby, sortorder FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1"
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
        "UPDATE {$db_prefix}users SET " .
        "name='" . e2e_sql_escape($user['name']) . "', oname='" . e2e_sql_escape($user['oname']) . "', " .
        "session='" . e2e_sql_escape($user['session']) . "', private_session='" . e2e_sql_escape($user['private_session']) . "', " .
        "password='" . e2e_sql_escape($user['password']) . "', pemail='" . e2e_sql_escape($user['pemail']) . "', email='" . e2e_sql_escape($user['email']) . "', " .
        "validated=" . (int)$user['validated'] . ", validatemd='" . e2e_sql_escape($user['validatemd']) . "', admin=" . (int)$user['admin'] . ", " .
        "deact_ip=" . (int)$user['deact_ip'] . ", vacation=" . (int)$user['vacation'] . ", vacation_until=" . (int)$user['vacation_until'] . ", " .
        "banned=" . (int)$user['banned'] . ", banned_until=" . (int)$user['banned_until'] . ", noattack=" . (int)$user['noattack'] . ", " .
        "noattack_until=" . (int)$user['noattack_until'] . ", disable=" . (int)$user['disable'] . ", disable_until=" . (int)$user['disable_until'] . ", " .
        "lang='" . e2e_sql_escape($user['lang']) . "', skin='" . e2e_sql_escape($user['skin']) . "', useskin=" . (int)$user['useskin'] . ", " .
        "maxspy=" . (int)$user['maxspy'] . ", maxfleetmsg=" . (int)$user['maxfleetmsg'] . ", sortby=" . (int)$user['sortby'] . ", " .
        "sortorder=" . (int)$user['sortorder'] . " WHERE player_id={$id}"
    );
    InvalidateUserCache();
}

function e2e_snapshot_uni(): array
{
    global $db_prefix;
    $row = e2e_one_row("SELECT lang, force_lang FROM {$db_prefix}uni LIMIT 1");
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

    dbquery("UPDATE {$db_prefix}uni SET lang='" . e2e_sql_escape($uni['lang']) . "', force_lang=" . (int)$uni['force_lang']);
    $GlobalUni = LoadUniverse();
}

function e2e_set_uni_language(string $lang, int $forceLang): void
{
    global $db_prefix, $GlobalUni;
    dbquery("UPDATE {$db_prefix}uni SET lang='" . e2e_sql_escape($lang) . "', force_lang={$forceLang}");
    $GlobalUni = LoadUniverse();
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

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$cases = array();
$userSnapshot = null;
$uniSnapshot = null;

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $userSnapshot = e2e_snapshot_user($attackerId);
    $uniSnapshot = e2e_snapshot_uni();
    if ($userSnapshot === null) {
        throw new RuntimeException('Attacker fixture user is missing.');
    }

    $missingKey = 'E2E_MISSING_LOCA_KEY_' . substr(md5((string)microtime(true)), 0, 8);
    $loca_lang = 'en';
    $cases[] = e2e_finalize_case(array(
        'case' => 'missing_localization_keys_fall_back_to_key_name',
        'checks' => array(
            e2e_case(loca($missingKey) === $missingKey, 'loca() returns the key name when the active language has no translation'),
            e2e_case(loca_lang($missingKey, 'de') === $missingKey, 'loca_lang() returns the key name when the requested language has no translation'),
        ),
    ));

    e2e_set_uni_language('en', 0);
    $auth = e2e_prepare_session($attackerId, 'localization-user-lang', 'en');
    $cookies = $auth['cookies'];
    $currentUser = e2e_snapshot_user($attackerId);
    $response = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=options&session=' . rawurlencode($auth['session']) . '&mode=change',
        e2e_options_payload($currentUser, array('lang' => 'de')),
        $cookies
    );
    $afterLangChange = e2e_snapshot_user($attackerId);
    $optionsResponse = e2e_http_request('GET', $gameBase . '/index.php?page=options&session=' . rawurlencode($auth['session']), array(), $cookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'user_language_option_persists_when_force_language_is_disabled',
        'checks' => array_merge(
            e2e_response_check($response, 'options language POST'),
            e2e_response_check($optionsResponse, 'options page after language POST'),
            array(
                e2e_case($afterLangChange !== null && $afterLangChange['lang'] === 'de', 'user language is persisted as German when force_lang is disabled', $afterLangChange ?? array()),
                e2e_case(strpos($optionsResponse['body'], 'Generelle Einstellungen') !== false, 'subsequent options page renders the German options localization'),
            )
        ),
    ));

    e2e_set_uni_language('en', 1);
    $auth = e2e_prepare_session($attackerId, 'localization-force-lang', 'de');
    $cookies = $auth['cookies'];
    $currentUser = e2e_snapshot_user($attackerId);
    $response = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=options&session=' . rawurlencode($auth['session']) . '&mode=change',
        e2e_options_payload($currentUser, array('lang' => 'de')),
        $cookies
    );
    $afterForcedLang = e2e_snapshot_user($attackerId);
    $overviewResponse = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($auth['session']) . '&cp=' . $attackerPlanet, array(), $cookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'forced_universe_language_overrides_user_language_choice',
        'checks' => array_merge(
            e2e_response_check($response, 'forced-language options POST'),
            e2e_response_check($overviewResponse, 'forced-language overview page'),
            array(
                e2e_case($afterForcedLang !== null && $afterForcedLang['lang'] === 'en', 'options save stores the universe language while force_lang is enabled', $afterForcedLang ?? array()),
                e2e_case(strpos($overviewResponse['body'], 'Server time') !== false, 'overview renders English text from the forced universe language'),
                e2e_case(strpos($overviewResponse['body'], 'Serverzeit') === false, 'overview does not render the user-selected German text while forced to English'),
            )
        ),
    ));

    e2e_set_uni_language('en', 0);
    $auth = e2e_prepare_session($attackerId, 'localization-invalid-user-lang', 'zz');
    $cookies = $auth['cookies'];
    $overviewResponse = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($auth['session']) . '&cp=' . $attackerPlanet, array(), $cookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'invalid_user_language_falls_back_to_universe_language',
        'checks' => array_merge(e2e_response_check($overviewResponse, 'invalid-language overview page'), array(
            e2e_case(strpos($overviewResponse['body'], 'Server time') !== false, 'overview falls back to the English universe language when user lang is invalid'),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'localization_edges_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    e2e_restore_uni($uniSnapshot);
    e2e_restore_user($userSnapshot);
}

echo json_encode(array(
    'case_group' => 'http_localization_edges',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
