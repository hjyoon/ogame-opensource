<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_multi_universe_isolation_e2e.php';
$_COOKIE['ogamelang'] = 'en';

chdir('/var/www/html/game');
require_once 'config.php';
require_once 'core/core.php';

InitDB();
$GlobalUni = LoadUniverse();
$GlobalUser = array('player_id' => 0);
$session = '';
ModsInit();

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

    return array('status' => $status, 'location' => $location, 'headers' => $responseHeaders, 'body' => $body === false ? '' : $body);
}

function e2e_case(bool $pass, string $message, array $context = array()): array
{
    return array('pass' => $pass, 'message' => $message, 'context' => $context);
}

function e2e_finalize_case(array $case): array
{
    $case['pass'] = array_reduce($case['checks'], fn($ok, $check) => $ok && $check['pass'], true);
    return $case;
}

function e2e_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

function e2e_mdb_escape(string $value): string
{
    global $MDB_link;
    return mysqli_real_escape_string($MDB_link, $value);
}

function e2e_one_row(string $sql): ?array
{
    $res = dbquery($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function e2e_mdb_rows(string $sql): array
{
    $rows = array();
    $res = MDBQuery($sql);
    if ($res === null) {
        return $rows;
    }
    while ($row = MDBArray($res)) {
        $rows[] = $row;
    }
    return $rows;
}

function e2e_prepare_session(int $userId, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', private_session='" . e2e_sql_escape($private) . "', " .
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

function option_value_present(string $body, string $value): bool
{
    return preg_match('/<option\b[^>]*\bvalue=["\']' . preg_quote($value, '/') . '["\']/i', $body) === 1;
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$attackerName = getenv('OGAME_E2E_ATTACKER_NAME') ?: '';
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$cases = array();
$tempNums = array(9901, 9902);

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }
    if (!MDBConnect()) {
        throw new RuntimeException('Master database connection is unavailable.');
    }

    MDBQuery("DELETE FROM unis WHERE num IN (" . implode(',', $tempNums) . ")");
    $sameHostUniverse = 'localhost:8888/u9901';
    $remoteUniverse = 'https://uni9902.example.test';
    MDBQuery(
        "INSERT INTO unis (num, dbhost, dbuser, dbpass, dbname, uniurl) VALUES " .
        "(9901, 'mysql', 'root', '', 'uni9901', '" . e2e_mdb_escape($sameHostUniverse) . "')," .
        "(9902, 'mysql', 'root', '', 'uni9902', '" . e2e_mdb_escape($remoteUniverse) . "')"
    );

    $masterRows = e2e_mdb_rows("SELECT num, uniurl FROM unis WHERE num IN (" . implode(',', $tempNums) . ") ORDER BY num ASC");
    $home = e2e_http_request('GET', $base . '/home.php');
    $register = e2e_http_request('GET', $base . '/register.php');
    $commonJs = file_get_contents('/var/www/html/common.js');

    $cases[] = e2e_finalize_case(array(
        'case' => 'master_universe_rows_render_in_lobby_selects',
        'master_rows' => $masterRows,
        'checks' => array(
            e2e_case(count($masterRows) === 2, 'temporary universe rows are present only in the master DB', array('rows' => $masterRows)),
            e2e_case($home['status'] === 200 && option_value_present($home['body'], $sameHostUniverse) && option_value_present($home['body'], $remoteUniverse), 'home login universe select renders both temporary universe URLs', array('status' => $home['status'])),
            e2e_case($register['status'] === 200 && option_value_present($register['body'], $sameHostUniverse) && option_value_present($register['body'], $remoteUniverse), 'register universe select renders both temporary universe URLs', array('status' => $register['status'])),
            e2e_case($commonJs !== false && str_contains($commonJs, 'return selected.path + actionPath;') && str_contains($commonJs, 'isCurrentUniverse'), 'lobby JavaScript keeps same-host universe actions relative to the selected path'),
        ),
    ));

    $auth = e2e_prepare_session($attackerId, 'multi-universe-cookie');
    $correctCookies = $auth['cookies'];
    $fakeUniverseCookies = array('prsess_' . $attackerId . '_9901' => $auth['private']);
    $correctResponse = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($auth['session']) . '&cp=' . $attackerPlanet, array(), $correctCookies);
    $fakeUniverseResponse = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($auth['session']) . '&cp=' . $attackerPlanet, array(), $fakeUniverseCookies);

    $cases[] = e2e_finalize_case(array(
        'case' => 'session_cookie_suffix_blocks_cross_universe_reuse',
        'checks' => array(
            e2e_case($correctResponse['status'] === 200 && strpos($correctResponse['body'], $attackerName) !== false, 'current-universe private cookie authenticates the overview page', array('status' => $correctResponse['status'])),
            e2e_case(in_array($fakeUniverseResponse['status'], array(200, 301, 302, 303), true), 'fake-universe cookie request returns an accepted denial status', array('status' => $fakeUniverseResponse['status'], 'location' => $fakeUniverseResponse['location'])),
            e2e_case(strpos($fakeUniverseResponse['body'], 'Error-ID:') !== false, 'fake-universe cookie renders invalid-session error instead of authenticating'),
            e2e_case(strpos($fakeUniverseResponse['body'], $attackerName) === false, 'fake-universe cookie does not reveal the authenticated player page'),
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'multi_universe_isolation_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if (isset($MDB_link) && $MDB_link) {
        MDBQuery("DELETE FROM unis WHERE num IN (" . implode(',', $tempNums) . ")");
    }
}

echo json_encode(array(
    'case_group' => 'http_multi_universe_isolation',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
