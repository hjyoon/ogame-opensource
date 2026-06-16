<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_premium_dm_edges_e2e.php';
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
loca_add('premium', 'en');
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

function e2e_response_check(array $response, string $label = 'HTTP request'): array
{
    $body = $response['body'];
    $runtimeError = e2e_has_runtime_error($body);
    $errorExcerpt = array();
    if ($runtimeError && preg_match('/.{0,120}(Fatal error|Parse error|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined).{0,180}/is', $body, $m)) {
        $errorExcerpt = array('label' => $label, 'excerpt' => trim(strip_tags($m[0])));
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

function e2e_prepare_session(int $userId, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', admin=0, validated=1, deact_ip=1, " .
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
        "banned, banned_until, noattack, noattack_until, disable, disable_until, lang, skin, useskin, " .
        "dm, dmfree, com_until, adm_until, eng_until, geo_until, tec_until " .
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
        "useskin=" . (int)$snapshot['useskin'] . ", " .
        "dm=" . (int)$snapshot['dm'] . ", " .
        "dmfree=" . (int)$snapshot['dmfree'] . ", " .
        "com_until=" . (int)$snapshot['com_until'] . ", " .
        "adm_until=" . (int)$snapshot['adm_until'] . ", " .
        "eng_until=" . (int)$snapshot['eng_until'] . ", " .
        "geo_until=" . (int)$snapshot['geo_until'] . ", " .
        "tec_until=" . (int)$snapshot['tec_until'] . " " .
        "WHERE player_id={$userId}"
    );
    InvalidateUserCache();
}

function e2e_prepare_planet_context(int $userId, int $planetId): void
{
    global $db_prefix;

    dbquery(
        "UPDATE {$db_prefix}users SET hplanetid={$planetId} WHERE player_id={$userId}"
    );
    dbquery(
        "UPDATE {$db_prefix}planets SET owner_id={$userId}, type=" . PTYP_PLANET . ", " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000, " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, remove=0 WHERE planet_id={$planetId}"
    );
    SelectPlanet($userId, $planetId);
}

function e2e_buy_url(string $gameBase, string $session, int $planetId, array $params): string
{
    $query = array_merge(array('page' => 'micropayment', 'session' => $session, 'cp' => (string)$planetId, 'buynow' => '1'), $params);
    return $gameBase . '/index.php?' . http_build_query($query);
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$cases = array();
$originalAttacker = null;

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $originalAttacker = e2e_user_snapshot($attackerId);
    if ($originalAttacker === null) {
        throw new RuntimeException('Attacker fixture user is missing.');
    }

    e2e_prepare_planet_context($attackerId, $attackerPlanet);
    $auth = e2e_prepare_session($attackerId, 'premium-dm-edges');
    $cookies = $auth['cookies'];
    $session = $auth['session'];

    dbquery("UPDATE {$db_prefix}users SET dm=9999, dmfree=0, adm_until=0 WHERE player_id={$attackerId}");
    $before = e2e_user_snapshot($attackerId);
    $response = e2e_http_request('GET', e2e_buy_url($gameBase, $session, $attackerPlanet, array('type' => (string)USER_OFFICER_ADMIRAL, 'days' => '7')), array(), $cookies);
    $after = e2e_user_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'premium_officer_purchase_rejects_insufficient_dark_matter',
        'checks' => array_merge(e2e_response_check($response, 'insufficient officer purchase'), array(
            e2e_case($before !== null && $after !== null && (int)$after['dm'] === (int)$before['dm'] && (int)$after['dmfree'] === (int)$before['dmfree'], 'insufficient purchase does not spend paid or free DM', array('before' => $before, 'after' => $after)),
            e2e_case($after !== null && (int)$after['adm_until'] === 0, 'insufficient purchase does not activate the officer', $after ?? array()),
        )),
    ));

    dbquery("UPDATE {$db_prefix}users SET dm=4000, dmfree=7000, eng_until=0 WHERE player_id={$attackerId}");
    $before = e2e_user_snapshot($attackerId);
    $response = e2e_http_request('GET', e2e_buy_url($gameBase, $session, $attackerPlanet, array('type' => (string)USER_OFFICER_ENGINEER, 'days' => '7')), array(), $cookies);
    $after = e2e_user_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'premium_officer_purchase_spends_paid_dm_before_free_dm',
        'checks' => array_merge(e2e_response_check($response, 'mixed paid/free officer purchase'), array(
            e2e_case($before !== null && $after !== null && (int)$after['dm'] === 0 && (int)$after['dmfree'] === 1000, 'purchase spends paid DM first and then the required free-DM remainder', array('before' => $before, 'after' => $after)),
            e2e_case($after !== null && (int)$after['eng_until'] >= time() + (6 * 24 * 60 * 60), 'mixed paid/free purchase activates the officer timer', $after ?? array()),
        )),
    ));

    $oldGeoUntil = time() + 3 * 24 * 60 * 60;
    dbquery("UPDATE {$db_prefix}users SET dm=20000, dmfree=0, geo_until={$oldGeoUntil} WHERE player_id={$attackerId}");
    $before = e2e_user_snapshot($attackerId);
    $response = e2e_http_request('GET', e2e_buy_url($gameBase, $session, $attackerPlanet, array('type' => (string)USER_OFFICER_GEOLOGE, 'days' => '7')), array(), $cookies);
    $after = e2e_user_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'premium_officer_repurchase_extends_existing_timer',
        'checks' => array_merge(e2e_response_check($response, 'officer repurchase'), array(
            e2e_case($before !== null && $after !== null && (int)$after['dm'] === (int)$before['dm'] - 10000 && (int)$after['dmfree'] === 0, 'repurchase spends one seven-day officer cost', array('before' => $before, 'after' => $after)),
            e2e_case($after !== null && (int)$after['geo_until'] === $oldGeoUntil + 7 * 24 * 60 * 60, 'repurchase extends the existing active officer timer instead of resetting from now', array('old_until' => $oldGeoUntil, 'after' => $after)),
        )),
    ));

    dbquery("UPDATE {$db_prefix}users SET dm=50000, dmfree=500, com_until=0, adm_until=0, eng_until=0, geo_until=0, tec_until=0 WHERE player_id={$attackerId}");
    $before = e2e_user_snapshot($attackerId);
    $invalidTypeResponse = e2e_http_request('GET', e2e_buy_url($gameBase, $session, $attackerPlanet, array('type' => '99', 'days' => '7')), array(), $cookies);
    $missingTypeResponse = e2e_http_request('GET', e2e_buy_url($gameBase, $session, $attackerPlanet, array('days' => '7')), array(), $cookies);
    $missingDaysResponse = e2e_http_request('GET', e2e_buy_url($gameBase, $session, $attackerPlanet, array('type' => (string)USER_OFFICER_COMMANDER)), array(), $cookies);
    $after = e2e_user_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'premium_invalid_purchase_parameters_do_not_spend_or_emit_runtime_errors',
        'checks' => array_merge(
            e2e_response_check($invalidTypeResponse, 'invalid officer type purchase'),
            e2e_response_check($missingTypeResponse, 'missing officer type purchase'),
            e2e_response_check($missingDaysResponse, 'missing officer days purchase'),
            array(
                e2e_case($before !== null && $after !== null && (int)$after['dm'] === (int)$before['dm'] && (int)$after['dmfree'] === (int)$before['dmfree'], 'invalid premium purchase parameters do not spend DM', array('before' => $before, 'after' => $after)),
                e2e_case($after !== null && (int)$after['com_until'] === 0 && (int)$after['adm_until'] === 0 && (int)$after['eng_until'] === 0 && (int)$after['geo_until'] === 0 && (int)$after['tec_until'] === 0, 'invalid premium purchase parameters do not activate any officer', $after ?? array()),
            )
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'premium_dm_edges_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($attackerId > 0 && $originalAttacker !== null) {
        e2e_restore_user($attackerId, $originalAttacker);
    }
}

echo json_encode(array(
    'case_group' => 'http_premium_dm_edges',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
