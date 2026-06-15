<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_flow_e2e.php';
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

function e2e_user_row(string $name): ?array
{
    global $db_prefix;
    $res = dbquery(
        "SELECT u.player_id, u.hplanetid, p.g, p.s, p.p " .
        "FROM {$db_prefix}users u JOIN {$db_prefix}planets p ON p.planet_id=u.hplanetid " .
        "WHERE u.name='" . e2e_sql_escape(mb_strtolower($name, 'UTF-8')) . "' LIMIT 1"
    );
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function e2e_remove_user_by_name(string $name): void
{
    $row = e2e_user_row($name);
    if ($row !== null) {
        RemoveUser((int)$row['player_id'], time());
    }
}

function e2e_case(bool $pass, string $message, array $context = array()): array
{
    return array('pass' => $pass, 'message' => $message, 'context' => $context);
}

function e2e_classify_document(array $response): array
{
    $body = $response['body'];
    $size = strlen($body);
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+Undefined/i', $body) === 1;
    $hasHtml = stripos($body, '<html') !== false || stripos($body, '<table') !== false || stripos($body, '<form') !== false;

    return array(
        'status' => $response['status'],
        'size' => $size,
        'has_error_marker' => $hasError,
        'looks_like_document' => $hasHtml && !$hasError && $size > 250,
        'has_meta_refresh' => stripos($body, 'http-equiv') !== false && stripos($body, 'refresh') !== false,
    );
}

$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$strictPublicHost = (getenv('OGAME_E2E_STRICT_PUBLIC_HOST') ?: '0') === '1';
$runId = (string)time() . random_int(1000, 9999);
$user = 'e2eh' . substr($runId, -10);
$pass = 'E2E_http123';
$email = $user . '@example.local';
putenv('OGAME_E2E_HTTP_USER=' . $user);

$cookies = array();
$cases = array();

try {
    $home = e2e_http_request('GET', $base . '/', array(), $cookies);
    $cases[] = array(
        'case' => 'root_homepage_auto_install_skipped',
        'checks' => array(
            e2e_case($home['status'] === 200, 'root homepage returns 200', array('status' => $home['status'])),
            e2e_case(stripos($home['body'], 'Master Database Settings') === false, 'master database install form is not shown'),
        ),
    );

    $loginForm = e2e_http_request('GET', $gameBase . '/reg/login.php', array(), $cookies);
    $cases[] = array(
        'case' => 'login_form_loads',
        'checks' => array(
            e2e_case($loginForm['status'] === 200, 'login form returns 200', array('status' => $loginForm['status'])),
            e2e_case(stripos($loginForm['body'], 'login2.php') !== false, 'login form posts to login2.php'),
        ),
    );

    $register = e2e_http_request('POST', $gameBase . '/reg/newredirect.php', array(
        'character' => $user,
        'password' => $pass,
        'email' => $email,
        'agb' => 'on',
        'universe' => '127.0.0.1',
    ), $cookies);
    $sessionFromRegister = '';
    if (preg_match('/session=([a-f0-9]{12})/i', $register['location'] . $register['body'], $m)) {
        $sessionFromRegister = $m[1];
    }
    $row = e2e_user_row($user);
    $cases[] = array(
        'case' => 'registration_creates_login_session',
        'checks' => array(
            e2e_case(in_array($register['status'], array(200, 302), true), 'registration returns 200 or 302', array('status' => $register['status'], 'location' => $register['location'])),
            e2e_case($row !== null, 'registration created a user row', $row ?? array()),
            e2e_case($sessionFromRegister !== '', 'registration/login produced a session token', array('session' => $sessionFromRegister)),
        ),
    );

    $login = e2e_http_request('GET', $gameBase . '/reg/login2.php?login=' . rawurlencode($user) . '&pass=' . rawurlencode($pass), array(), $cookies);
    $loginTarget = $login['location'] !== '' ? $login['location'] : $login['body'];
    $loginLeaksLocalhost = preg_match('/https?:\/\/(?:localhost|127\.0\.0\.1)(?::\d+)?\/game\/reg\/login2\.php/i', $loginTarget) === 1;
    $loginUsesStartPageLoopback = preg_match('/https?:\/\/(?:localhost|127\.0\.0\.1)(?::\d+)?/i', $loginTarget) === 1;
    $cases[] = array(
        'case' => 'login_redirect_host_behavior',
        'strict' => $strictPublicHost,
        'checks' => array(
            e2e_case(in_array($login['status'], array(200, 302), true), 'login2 returns 200 or 302', array('status' => $login['status'], 'location' => $login['location'])),
            e2e_case(stripos($loginTarget, '/game/reg/login2.php') === false && !$loginLeaksLocalhost, 'login redirect does not point back to login2.php', array('target' => mb_substr($loginTarget, 0, 200, 'UTF-8'))),
            e2e_case(!$strictPublicHost || !$loginUsesStartPageLoopback, 'strict public-host mode: login redirect must not use localhost/127.0.0.1', array('target' => mb_substr($loginTarget, 0, 200, 'UTF-8'))),
        ),
    );

    if ($sessionFromRegister === '' && preg_match('/session=([a-f0-9]{12})/i', $loginTarget, $m)) {
        $sessionFromRegister = $m[1];
    }
    if ($sessionFromRegister === '' && $row !== null) {
        $u = LoadUser((int)$row['player_id']);
        $sessionFromRegister = $u['session'] ?? '';
    }

    $normalPages = array('overview', 'resources', 'b_building', 'flotten1', 'galaxy', 'messages', 'notizen', 'options', 'allianzen', 'statistics', 'suche', 'techtree', 'trader');
    $routeResults = array();
    foreach ($normalPages as $page) {
        $path = $gameBase . '/index.php?page=' . rawurlencode($page) . '&session=' . rawurlencode($sessionFromRegister);
        if ($page === 'galaxy' && $row !== null) {
            $path .= '&galaxy=' . (int)$row['g'] . '&system=' . (int)$row['s'] . '&no_header=1';
        }
        $response = e2e_http_request('GET', $path, array(), $cookies);
        $routeResults[$page] = e2e_classify_document($response);
    }
    $cases[] = array(
        'case' => 'post_login_core_pages_render',
        'routes' => $routeResults,
        'checks' => array(
            e2e_case(count(array_filter($routeResults, fn($r) => $r['status'] === 200 && !$r['has_error_marker'])) === count($routeResults), 'core post-login pages return 200 without PHP error markers'),
            e2e_case(count(array_filter($routeResults, fn($r) => $r['looks_like_document'] || $r['has_meta_refresh'])) === count($routeResults), 'core post-login pages render document or known meta-refresh response'),
        ),
    );

    $overview = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($sessionFromRegister), array(), $cookies);
    $loopbackAsset = preg_match('/(?:src|href)=["\']https?:\/\/(?:localhost|127\.0\.0\.1)(?::\d+)?\//i', $overview['body']) === 1;
    $cases[] = array(
        'case' => 'internal_assets_are_not_loopback_absolute_urls',
        'checks' => array(
            e2e_case($overview['status'] === 200, 'overview is available for asset inspection', array('status' => $overview['status'])),
            e2e_case(!$loopbackAsset, 'overview HTML does not emit localhost/127.0.0.1 absolute asset URLs'),
        ),
    );
} finally {
    e2e_remove_user_by_name($user);
}

foreach ($cases as &$case) {
    $case['pass'] = array_reduce($case['checks'], fn($ok, $check) => $ok && $check['pass'], true);
}
unset($case);

echo json_encode(array(
    'case_group' => 'http_install_login_routes',
    'base' => $base,
    'strict_public_host' => $strictPublicHost,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
