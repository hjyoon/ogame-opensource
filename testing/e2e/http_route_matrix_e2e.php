<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_route_matrix_e2e.php';
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

function e2e_case(bool $pass, string $message, array $context = array()): array
{
    return array('pass' => $pass, 'message' => $message, 'context' => $context);
}

function e2e_prepare_session(int $userId): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5('route-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5('route-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', validated=1, deact_ip=1, " .
        "lang='en', skin='/evolution/', useskin=1 WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookies' => array('prsess_' . $userId . '_' . $GlobalUni['num'] => $private),
    );
}

function e2e_classify_response(array $response): array
{
    $body = $response['body'];
    $status = $response['status'];
    $size = strlen($body);
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined/i', $body) === 1;
    $hasDocument = stripos($body, '<html') !== false ||
        stripos($body, '<table') !== false ||
        stripos($body, '<form') !== false ||
        stripos($body, '<body') !== false;
    $hasMetaRefresh = stripos($body, 'http-equiv') !== false && stripos($body, 'refresh') !== false;
    $hasLoopbackAsset = preg_match('/(?:src|href|background)=["\']https?:\/\/(?:localhost|127\.0\.0\.1)(?::\d+)?\//i', $body) === 1;

    return array(
        'status' => $status,
        'location' => $response['location'],
        'size' => $size,
        'has_error_marker' => $hasError,
        'looks_like_document' => $hasDocument && $size > 120,
        'has_meta_refresh' => $hasMetaRefresh,
        'has_loopback_asset_url' => $hasLoopbackAsset,
    );
}

function e2e_route_checks(string $label, array $response, bool $allowRedirect = false): array
{
    $class = e2e_classify_response($response);
    $statusOk = $class['status'] === 200 || ($allowRedirect && in_array($class['status'], array(301, 302, 303), true));
    $documentOk = $class['status'] !== 200 || $class['looks_like_document'] || $class['has_meta_refresh'];

    return array(
        'route' => $label,
        'classification' => $class,
        'checks' => array(
            e2e_case($statusOk, 'route returns an accepted HTTP status', array('status' => $class['status'], 'location' => $class['location'])),
            e2e_case(!$class['has_error_marker'], 'route body has no PHP error marker'),
            e2e_case($documentOk, 'route body looks like a rendered document or redirect response', array('size' => $class['size'])),
            e2e_case(!$class['has_loopback_asset_url'], 'route does not emit localhost/127.0.0.1 absolute asset URLs'),
        ),
    );
}

function e2e_finalize_case(array $case): array
{
    $case['pass'] = array_reduce($case['checks'], fn($ok, $check) => $ok && $check['pass'], true);
    return $case;
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderPlanet = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 0);

$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$cases = array();
$routes = array();
$messageId = 0;
$allyId = 0;

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $auth = e2e_prepare_session($attackerId);
    $cookies = $auth['cookies'];
    $sess = $auth['session'];

    $messageId = SendMessage($attackerId, 'E2E route matrix', 'E2E report document', '<table><tr><th>E2E report body</th></tr></table>', MTYP_SPY_REPORT, time(), $defenderPlanet);
    $allyId = CreateAlly($attackerId, 'E2ERT' . substr((string)time(), -2), 'E2E Route Ally');
    AddApplication($allyId, $defenderId, 'E2E route application');

    $publicRoutes = array(
        'root' => array('GET', $base . '/', array(), false),
        'game_index' => array('GET', $gameBase . '/', array(), true),
        'login_form' => array('GET', $gameBase . '/reg/login.php', array(), false),
        'registration_form' => array('GET', $gameBase . '/reg/new.php', array(), false),
        'public_ainfo' => array('GET', $gameBase . '/index.php?page=ainfo&gid=1', array(), false),
        'public_pranger' => array('GET', $gameBase . '/index.php?page=pranger', array(), false),
    );

    $publicChecks = array();
    foreach ($publicRoutes as $label => $spec) {
        [$method, $url, $data, $allowRedirect] = $spec;
        $response = e2e_http_request($method, $url, $data, $cookies);
        $route = e2e_route_checks($label, $response, $allowRedirect);
        $routes[$label] = e2e_finalize_case($route);
        $publicChecks = array_merge($publicChecks, $route['checks']);
    }
    $home = e2e_http_request('GET', $base . '/', array(), $cookies);
    $publicChecks[] = e2e_case(stripos($home['body'], 'Master Database Settings') === false, 'public root is not the installer form');
    $cases[] = e2e_finalize_case(array('case' => 'public_route_matrix', 'checks' => $publicChecks));

    $get = fn(string $page, string $query = ''): array => array(
        'GET',
        $gameBase . '/index.php?page=' . rawurlencode($page) . '&session=' . rawurlencode($sess) . $query,
        array(),
        true,
    );
    $post = fn(string $page, array $data, string $query = ''): array => array(
        'POST',
        $gameBase . '/index.php?page=' . rawurlencode($page) . '&session=' . rawurlencode($sess) . $query,
        $data,
        true,
    );

    $postLoginRoutes = array(
        'allianzdepot' => $get('allianzdepot'),
        'allianzen' => $get('allianzen'),
        'b_building' => $get('b_building'),
        'bericht' => $get('bericht', '&bericht=' . $messageId),
        'bewerben' => $get('bewerben', '&allyid=' . $allyId),
        'bewerbungen' => $get('bewerbungen'),
        'buddy' => $get('buddy'),
        'buildings' => $get('buildings'),
        'changelog' => $get('changelog'),
        'fleet_templates' => $get('fleet_templates'),
        'flotten1' => $get('flotten1'),
        'flotten2' => $post('flotten2', array('ship202' => 0, 'ship203' => 0)),
        'flotten3' => $post('flotten3', array(
            'thisgalaxy' => 1,
            'thissystem' => 1,
            'thisplanet' => 1,
            'thisplanettype' => 1,
            'speedfactor' => 1,
            'galaxy' => 1,
            'system' => 1,
            'planet' => 1,
            'planettype' => 1,
        )),
        'galaxy' => $get('galaxy', '&galaxy=1&system=1&no_header=1'),
        'imperium' => $get('imperium'),
        'infos' => $get('infos', '&gid=' . GID_B_METAL_MINE),
        'messages' => $get('messages'),
        'micropayment' => $get('micropayment'),
        'notizen' => $get('notizen'),
        'options' => $get('options'),
        'overview' => $get('overview'),
        'payment' => $get('payment'),
        'phalanx' => $get('phalanx', '&spid=' . $defenderPlanet),
        'renameplanet' => $get('renameplanet'),
        'resources' => $get('resources'),
        'sprungtor' => $post('sprungtor', array('qm' => $attackerPlanet, 'zm' => $attackerPlanet)),
        'statistics' => $get('statistics'),
        'suche' => $get('suche'),
        'techtree' => $get('techtree'),
        'techtreedetails' => $get('techtreedetails', '&gid=' . GID_B_METAL_MINE),
        'trader' => $get('trader'),
        'writemessages' => $get('writemessages', '&messageziel=' . $defenderId),
    );

    $postLoginChecks = array();
    foreach ($postLoginRoutes as $label => $spec) {
        [$method, $url, $data, $allowRedirect] = $spec;
        $response = e2e_http_request($method, $url, $data, $cookies);
        $route = e2e_route_checks($label, $response, $allowRedirect);
        $routes[$label] = e2e_finalize_case($route);
        $postLoginChecks = array_merge($postLoginChecks, $route['checks']);
    }
    $cases[] = e2e_finalize_case(array('case' => 'post_login_route_matrix', 'checks' => $postLoginChecks));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'route_matrix_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($messageId > 0 && $attackerId > 0) {
        dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id={$attackerId} AND msg_id={$messageId}");
    }
    if ($allyId > 0) {
        DismissAlly($allyId);
    }
}

echo json_encode(array(
    'case_group' => 'http_route_matrix',
    'base' => $base,
    'routes' => $routes,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
