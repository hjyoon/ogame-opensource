<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_render_asset_smoke_e2e.php';
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
    $contentType = '';
    foreach ($responseHeaders as $header) {
        if (preg_match('/^HTTP\/\S+\s+(\d+)/', $header, $m)) {
            $status = (int)$m[1];
        } elseif (stripos($header, 'Location:') === 0) {
            $location = trim(substr($header, 9));
        } elseif (stripos($header, 'Content-Type:') === 0) {
            $contentType = trim(substr($header, 13));
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
        'content_type' => $contentType,
        'headers' => $responseHeaders,
        'body' => $body === false ? '' : $body,
    );
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

function e2e_prepare_session(int $userId): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5('render-smoke-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5('render-smoke-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', validated=1, deact_ip=1, " .
        "vacation=0, vacation_until=0, banned=0, banned_until=0, disable=0, disable_until=0, " .
        "lang='en', skin='/evolution/', useskin=1 WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookies' => array('prsess_' . $userId . '_' . $GlobalUni['num'] => $private),
    );
}

function e2e_has_error_marker(string $body): bool
{
    return preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined/i', $body) === 1;
}

function e2e_looks_like_document(string $body): bool
{
    return strlen($body) > 120 && (
        stripos($body, '<html') !== false ||
        stripos($body, '<body') !== false ||
        stripos($body, '<table') !== false ||
        stripos($body, '<form') !== false
    );
}

function e2e_has_loopback_asset(string $body): bool
{
    return preg_match('/(?:src|href|background)=["\']https?:\/\/(?:localhost|127\.0\.0\.1)(?::\d+)?\//i', $body) === 1;
}

function e2e_page_checks(string $label, array $response): array
{
    $body = $response['body'];
    return array(
        e2e_case($response['status'] === 200, $label . ' returns HTTP 200', array('status' => $response['status'], 'location' => $response['location'])),
        e2e_case(!e2e_has_error_marker($body), $label . ' has no PHP error marker'),
        e2e_case(e2e_looks_like_document($body), $label . ' looks like a rendered document', array('size' => strlen($body))),
        e2e_case(stripos($body, 'Master Database Settings') === false, $label . ' does not render the installer form'),
        e2e_case(!e2e_has_loopback_asset($body), $label . ' does not emit localhost/127.0.0.1 absolute asset URLs'),
    );
}

function e2e_origin(string $url): string
{
    $parts = parse_url($url);
    $scheme = $parts['scheme'] ?? 'http';
    $host = $parts['host'] ?? '127.0.0.1';
    $port = isset($parts['port']) ? ':' . $parts['port'] : '';
    return $scheme . '://' . $host . $port;
}

function e2e_normalize_path(string $path): string
{
    $segments = array();
    foreach (explode('/', $path) as $segment) {
        if ($segment === '' || $segment === '.') {
            continue;
        }
        if ($segment === '..') {
            array_pop($segments);
            continue;
        }
        $segments[] = $segment;
    }
    return '/' . implode('/', $segments);
}

function e2e_absolute_url(string $documentUrl, string $assetUrl): string
{
    $assetUrl = trim(html_entity_decode($assetUrl, ENT_QUOTES | ENT_HTML5, 'UTF-8'));
    if ($assetUrl === '' || $assetUrl[0] === '#' || stripos($assetUrl, 'javascript:') === 0 || stripos($assetUrl, 'data:') === 0 || stripos($assetUrl, 'mailto:') === 0) {
        return '';
    }
    if (preg_match('/^https?:\/\//i', $assetUrl)) {
        return $assetUrl;
    }
    $origin = e2e_origin($documentUrl);
    if (str_starts_with($assetUrl, '//')) {
        $scheme = parse_url($documentUrl, PHP_URL_SCHEME) ?: 'http';
        return $scheme . ':' . $assetUrl;
    }
    if ($assetUrl[0] === '/') {
        return $origin . e2e_normalize_path($assetUrl);
    }

    $path = parse_url($documentUrl, PHP_URL_PATH) ?: '/';
    $dir = str_ends_with($path, '/') ? $path : dirname($path);
    if ($dir === '\\' || $dir === '.') {
        $dir = '/';
    }
    return $origin . e2e_normalize_path(rtrim($dir, '/') . '/' . $assetUrl);
}

function e2e_extract_assets(string $documentUrl, string $body): array
{
    $assets = array();
    if (preg_match_all('/(?:src|href|background)\s*=\s*["\']([^"\']+)["\']/i', $body, $matches)) {
        foreach ($matches[1] as $rawUrl) {
            if (!preg_match('/\.(?:css|js|png|jpe?g|gif|ico|webp)(?:[?#].*)?$/i', $rawUrl)) {
                continue;
            }
            $url = e2e_absolute_url($documentUrl, $rawUrl);
            if ($url !== '') {
                $assets[$url] = true;
            }
        }
    }
    if (preg_match_all('/url\(([^)]+)\)/i', $body, $matches)) {
        foreach ($matches[1] as $rawUrl) {
            $rawUrl = trim($rawUrl, " \t\n\r\0\x0B'\"");
            if (!preg_match('/\.(?:png|jpe?g|gif|ico|webp)(?:[?#].*)?$/i', $rawUrl)) {
                continue;
            }
            $url = e2e_absolute_url($documentUrl, $rawUrl);
            if ($url !== '') {
                $assets[$url] = true;
            }
        }
    }
    return array_keys($assets);
}

function e2e_same_origin(string $left, string $right): bool
{
    return e2e_origin($left) === e2e_origin($right);
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$cases = array();
$pageResults = array();
$assetUrls = array();

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $auth = e2e_prepare_session($attackerId);
    $cookies = $auth['cookies'];
    $session = $auth['session'];

    $publicPages = array(
        'public_root' => $base . '/',
        'login_form' => $gameBase . '/reg/login.php',
    );
    $authenticatedPages = array(
        'overview' => $gameBase . '/index.php?page=overview&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet,
        'resources' => $gameBase . '/index.php?page=resources&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet,
        'buildings' => $gameBase . '/index.php?page=buildings&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet,
        'fleet' => $gameBase . '/index.php?page=flotten1&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet,
        'galaxy' => $gameBase . '/index.php?page=galaxy&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&galaxy=1&system=1&no_header=1',
        'messages' => $gameBase . '/index.php?page=messages&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet,
    );

    foreach ($publicPages + $authenticatedPages as $label => $url) {
        $response = e2e_http_request('GET', $url, array(), $cookies);
        $checks = e2e_page_checks($label, $response);
        $pageResults[$label] = array(
            'status' => $response['status'],
            'size' => strlen($response['body']),
            'asset_count' => 0,
        );

        $assets = e2e_extract_assets($url, $response['body']);
        $pageResults[$label]['asset_count'] = count($assets);
        foreach ($assets as $assetUrl) {
            if (e2e_same_origin($base, $assetUrl)) {
                $assetUrls[$assetUrl] = true;
            }
        }

        $cases[] = e2e_finalize_case(array(
            'case' => 'render_smoke_' . $label,
            'checks' => $checks,
        ));
    }

    $assetUrls = array_keys($assetUrls);
    sort($assetUrls);
    $assetUrls = array_slice($assetUrls, 0, 80);
    $assetChecks = array(
        e2e_case(count($assetUrls) > 0, 'rendered pages expose at least one same-origin CSS/JS/image asset', array('asset_count' => count($assetUrls))),
    );
    foreach ($assetUrls as $assetUrl) {
        $assetResponse = e2e_http_request('GET', $assetUrl, array(), $cookies);
        $body = $assetResponse['body'];
        $looksLikeHtml = stripos(ltrim($body), '<!doctype html') === 0 || stripos(ltrim($body), '<html') === 0;
        $assetChecks[] = e2e_case(
            $assetResponse['status'] === 200 && strlen($body) > 0 && !$looksLikeHtml,
            'referenced asset loads as a non-empty non-HTML resource',
            array(
                'url' => $assetUrl,
                'status' => $assetResponse['status'],
                'content_type' => $assetResponse['content_type'],
                'size' => strlen($body),
            )
        );
    }
    $cases[] = e2e_finalize_case(array(
        'case' => 'referenced_assets_load',
        'checks' => $assetChecks,
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'render_asset_smoke_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
}

echo json_encode(array(
    'case_group' => 'http_render_asset_smoke',
    'base' => $base,
    'pages' => $pageResults,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
