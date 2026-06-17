<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_performance_baseline_e2e.php';
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

function e2e_sql_exec(string $sql): mixed
{
    $res = dbquery($sql);
    if ($res === false) {
        throw new RuntimeException('SQL failed: ' . $sql);
    }
    return $res;
}

function e2e_one_row(string $sql): ?array
{
    $res = e2e_sql_exec($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
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

    $started = microtime(true);
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

    $body = @file_get_contents($url, false, $context);
    $elapsedMs = (int)round((microtime(true) - $started) * 1000);
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
        'elapsed_ms' => $elapsedMs,
        'bytes' => $body === false ? 0 : strlen($body),
    );
}

function e2e_prepare_session(int $userId, string $label, int $admin = USER_TYPE_PLAYER): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    e2e_sql_exec(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', admin={$admin}, " .
        "validated=1, validatemd='', deact_ip=1, lang='en', skin='/evolution/', useskin=1, " .
        "vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, " .
        "disable=0, disable_until=0 WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookies' => array('prsess_' . $userId . '_' . $GlobalUni['num'] => $private),
    );
}

function e2e_restore_player_role(int $userId): void
{
    global $db_prefix;

    if ($userId > 0) {
        e2e_sql_exec("UPDATE {$db_prefix}users SET admin=" . USER_TYPE_PLAYER . " WHERE player_id={$userId}");
        InvalidateUserCache();
    }
}

function e2e_response_checks(array $response, string $label, int $thresholdMs): array
{
    $body = $response['body'];
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined/i', $body) === 1;
    $looksLikeDocument = stripos($body, '<html') !== false ||
        stripos($body, '<body') !== false ||
        stripos($body, '<table') !== false ||
        stripos($body, '<form') !== false;
    $isInstallPrompt = stripos($body, 'Master Database Settings') !== false;

    return array(
        e2e_case($response['status'] === 200, "{$label} returns HTTP 200", array('status' => $response['status'], 'location' => $response['location'])),
        e2e_case(!$hasError, "{$label} body has no PHP error marker"),
        e2e_case(!$isInstallPrompt, "{$label} does not render the installer"),
        e2e_case($looksLikeDocument && $response['bytes'] > 120, "{$label} renders a non-empty HTML document", array('bytes' => $response['bytes'])),
        e2e_case($response['elapsed_ms'] <= $thresholdMs, "{$label} stays within the render baseline", array('elapsed_ms' => $response['elapsed_ms'], 'threshold_ms' => $thresholdMs)),
    );
}

function e2e_metric(array $response, int $thresholdMs): array
{
    return array(
        'status' => $response['status'],
        'elapsed_ms' => $response['elapsed_ms'],
        'bytes' => $response['bytes'],
        'threshold_ms' => $thresholdMs,
    );
}

$cases = array();
$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);

try {
    if ($attackerId <= 0) {
        throw new RuntimeException('OGAME_E2E_ATTACKER_ID is required.');
    }

    $attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
    if ($attackerPlanet <= 0) {
        throw new RuntimeException('OGAME_E2E_ATTACKER_PLANET is required.');
    }

    $planet = LoadPlanetById($attackerPlanet);
    if ($planet === null || empty($planet)) {
        throw new RuntimeException('Attacker fixture planet is missing.');
    }

    $base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
    $gameBase = preg_match('#/game$#', $base) ? $base : $base . '/game';
    $pageThreshold = max(1, intval(getenv('OGAME_E2E_PERF_PAGE_MS') ?: 5000));
    $adminThreshold = max(1, intval(getenv('OGAME_E2E_PERF_ADMIN_MS') ?: 8000));
    $totalThreshold = max(1, intval(getenv('OGAME_E2E_PERF_TOTAL_MS') ?: 15000));

    $playerAuth = e2e_prepare_session($attackerId, 'perf-player', USER_TYPE_PLAYER);
    $playerCookies = $playerAuth['cookies'];
    $session = rawurlencode($playerAuth['session']);
    $cp = $attackerPlanet;
    $galaxy = (int)$planet['g'];
    $system = (int)$planet['s'];

    $requests = array(
        'overview' => array(
            'threshold' => $pageThreshold,
            'response' => e2e_http_request('GET', "{$gameBase}/index.php?page=overview&session={$session}&cp={$cp}", array(), $playerCookies),
        ),
        'resources' => array(
            'threshold' => $pageThreshold,
            'response' => e2e_http_request('GET', "{$gameBase}/index.php?page=resources&session={$session}&cp={$cp}", array(), $playerCookies),
        ),
        'galaxy' => array(
            'threshold' => $pageThreshold,
            'response' => e2e_http_request('GET', "{$gameBase}/index.php?page=galaxy&no_header=1&session={$session}&cp={$cp}&galaxy={$galaxy}&system={$system}", array(), $playerCookies),
        ),
        'statistics' => array(
            'threshold' => $pageThreshold,
            'response' => e2e_http_request('GET', "{$gameBase}/index.php?page=statistics&session={$session}&start=1&type=ressources", array(), $playerCookies),
        ),
        'messages' => array(
            'threshold' => $pageThreshold,
            'response' => e2e_http_request('GET', "{$gameBase}/index.php?page=messages&session={$session}&dsp=1", array(), $playerCookies),
        ),
    );

    $adminAuth = e2e_prepare_session($attackerId, 'perf-admin', USER_TYPE_ADMIN);
    $adminCookies = $adminAuth['cookies'];
    $adminSession = rawurlencode($adminAuth['session']);
    $requests['admin_queue'] = array(
        'threshold' => $adminThreshold,
        'response' => e2e_http_request('GET', "{$gameBase}/index.php?page=admin&session={$adminSession}&mode=Queue", array(), $adminCookies),
    );

    $checks = array();
    $metrics = array();
    $totalMs = 0;
    foreach ($requests as $label => $request) {
        $response = $request['response'];
        $threshold = $request['threshold'];
        $totalMs += (int)$response['elapsed_ms'];
        $metrics[$label] = e2e_metric($response, $threshold);
        $checks = array_merge($checks, e2e_response_checks($response, $label, $threshold));
    }
    $checks[] = e2e_case($totalMs <= $totalThreshold, 'tracked pages stay within the aggregate render baseline', array('total_ms' => $totalMs, 'threshold_ms' => $totalThreshold));

    $cases[] = e2e_finalize_case(array(
        'case' => 'core_pages_stay_within_performance_baseline',
        'metrics' => $metrics,
        'checks' => $checks,
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'performance_baseline_exception',
        'pass' => false,
        'checks' => array(e2e_case(false, $e->getMessage(), array(
            'file' => $e->getFile(),
            'line' => $e->getLine(),
        ))),
    );
} finally {
    e2e_restore_player_role($attackerId);
}

$pass = array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true);
echo json_encode(array(
    'case_group' => 'http_performance_baseline',
    'all_pass' => $pass,
    'cases' => $cases,
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
