<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_admin_state_e2e.php';
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

function e2e_response_check(array $response): array
{
    $body = $response['body'];
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined/i', $body) === 1;
    return array(
        e2e_case(in_array($response['status'], array(200, 301, 302, 303), true), 'HTTP request returns an accepted status', array('status' => $response['status'], 'location' => $response['location'])),
        e2e_case(!$hasError, 'HTTP body has no PHP error marker'),
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

function e2e_prepare_session(int $userId, int $adminLevel, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', admin={$adminLevel}, " .
        "validated=1, deact_ip=1, lang='en', skin='/evolution/', useskin=1, " .
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

function e2e_snapshot_user(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT player_id, admin, pemail, email, validated, sniff, debug, dm, dmfree, sortby, sortorder, " .
        "skin, useskin, deact_ip, maxspy, maxfleetmsg, vacation, vacation_until, banned, banned_until, " .
        "noattack, noattack_until, disable, disable_until, lang FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1"
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
        "admin=" . (int)$user['admin'] . ", " .
        "pemail='" . e2e_sql_escape($user['pemail']) . "', " .
        "email='" . e2e_sql_escape($user['email']) . "', " .
        "validated=" . (int)$user['validated'] . ", " .
        "sniff=" . (int)$user['sniff'] . ", " .
        "debug=" . (int)$user['debug'] . ", " .
        "dm=" . (int)$user['dm'] . ", " .
        "dmfree=" . (int)$user['dmfree'] . ", " .
        "sortby=" . (int)$user['sortby'] . ", " .
        "sortorder=" . (int)$user['sortorder'] . ", " .
        "skin='" . e2e_sql_escape($user['skin']) . "', " .
        "useskin=" . (int)$user['useskin'] . ", " .
        "deact_ip=" . (int)$user['deact_ip'] . ", " .
        "maxspy=" . (int)$user['maxspy'] . ", " .
        "maxfleetmsg=" . (int)$user['maxfleetmsg'] . ", " .
        "vacation=" . (int)$user['vacation'] . ", " .
        "vacation_until=" . (int)$user['vacation_until'] . ", " .
        "banned=" . (int)$user['banned'] . ", " .
        "banned_until=" . (int)$user['banned_until'] . ", " .
        "noattack=" . (int)$user['noattack'] . ", " .
        "noattack_until=" . (int)$user['noattack_until'] . ", " .
        "disable=" . (int)$user['disable'] . ", " .
        "disable_until=" . (int)$user['disable_until'] . ", " .
        "lang='" . e2e_sql_escape($user['lang']) . "' WHERE player_id={$id}"
    );
    InvalidateUserCache();
}

function e2e_user_update_payload(array $user, array $overrides = array()): array
{
    global $resmap;

    $data = array(
        'pemail' => $user['pemail'],
        'email' => $user['email'],
        'deaktjava' => '',
        'vacation' => '',
        'banned' => '',
        'noattack' => '',
        'admin' => (string)$user['admin'],
        'validated' => ((int)$user['validated']) ? 'on' : '',
        'sniff' => ((int)$user['sniff']) ? 'on' : '',
        'debug' => ((int)$user['debug']) ? 'on' : '',
        'dm' => (string)$user['dm'],
        'dmfree' => (string)$user['dmfree'],
        'settings_sort' => (string)$user['sortby'],
        'settings_order' => (string)$user['sortorder'],
        'dpath' => $user['skin'],
        'design' => ((int)$user['useskin']) ? 'on' : '',
        'deact_ip' => ((int)$user['deact_ip']) ? 'on' : '',
        'spio_anz' => (string)$user['maxspy'],
        'settings_fleetactions' => (string)$user['maxfleetmsg'],
        'pr_' . USER_OFFICER_COMMANDER => '',
        'pr_' . USER_OFFICER_ADMIRAL => '',
        'pr_' . USER_OFFICER_ENGINEER => '',
        'pr_' . USER_OFFICER_GEOLOGE => '',
        'pr_' . USER_OFFICER_TECHNOCRATE => '',
    );
    foreach ($resmap as $gid) {
        $data['r' . $gid] = (string)($user[$gid] ?? 0);
    }

    foreach ($overrides as $key => $value) {
        $data[$key] = $value;
    }
    return $data;
}

function e2e_reset_runtime_state(int $userId, int $planetId): void
{
    global $db_prefix;

    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$userId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "','" . QTYP_UNBAN . "','" . QTYP_ALLOW_ATTACKS . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$userId} OR planet_id={$planetId}");
    dbquery("UPDATE {$db_prefix}planets SET prod1=1, prod2=1, prod3=1, prod4=1, prod12=1, prod212=1, `" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000 WHERE planet_id={$planetId}");
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderPlanet = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 0);
$defenderName = getenv('OGAME_E2E_DEFENDER_NAME') ?: '';
$defenderPassword = getenv('OGAME_E2E_DEFENDER_PASSWORD') ?: '';
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$token = 'E2EADM' . substr(md5((string)microtime(true)), 0, 8);
$cases = array();
$attackerSnapshot = null;
$defenderSnapshot = null;

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0 || $defenderName === '' || $defenderPassword === '') {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $attackerSnapshot = e2e_snapshot_user($attackerId);
    $defenderSnapshot = e2e_snapshot_user($defenderId);
    $defenderFull = LoadUser($defenderId);
    e2e_reset_runtime_state($attackerId, $attackerPlanet);
    e2e_reset_runtime_state($defenderId, $defenderPlanet);

    $regularAuth = e2e_prepare_session($attackerId, USER_TYPE_PLAYER, 'admin-regular');
    $regularCookies = $regularAuth['cookies'];
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=admin&session=' . rawurlencode($regularAuth['session']) . '&mode=Users&player_id=' . $defenderId, array(), $regularCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'regular_user_admin_page_denied',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(stripos($response['body'], 'http-equiv') !== false && stripos($response['body'], 'refresh') !== false, 'regular user receives a redirect shell'),
            e2e_case(stripos($response['body'], 'mode=Users') === false, 'regular user does not receive admin user links'),
            e2e_case(stripos($response['body'], 'ADM_USER') === false, 'regular user does not receive admin localization keys'),
        )),
    ));

    $adminAuth = e2e_prepare_session($attackerId, USER_TYPE_ADMIN, 'admin-full');
    $adminCookies = $adminAuth['cookies'];
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']), array(), $adminCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_home_loads',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(stripos($response['body'], 'mode=Users') !== false, 'admin home links to user management'),
            e2e_case(stripos($response['body'], 'mode=Bans') !== false, 'admin home links to bans'),
            e2e_case(stripos($response['body'], 'mode=Uni') !== false, 'admin home links to universe settings'),
        )),
    ));

    $operatorAuth = e2e_prepare_session($attackerId, USER_TYPE_GO, 'admin-operator');
    $operatorCookies = $operatorAuth['cookies'];
    $beforeOperatorAttempt = e2e_one_row("SELECT player_id, maxspy, dmfree, admin FROM {$db_prefix}users WHERE player_id={$defenderId} LIMIT 1");
    $operatorPayload = e2e_user_update_payload($defenderFull, array(
        'spio_anz' => '17',
        'dmfree' => '777',
        'admin' => (string)USER_TYPE_ADMIN,
    ));
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($operatorAuth['session']) . '&mode=Users&action=update&player_id=' . $defenderId, $operatorPayload, $operatorCookies);
    $afterOperatorAttempt = e2e_one_row("SELECT player_id, maxspy, dmfree, admin FROM {$db_prefix}users WHERE player_id={$defenderId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'operator_cannot_update_admin_user_fields',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($afterOperatorAttempt !== null && $beforeOperatorAttempt !== null && (int)$afterOperatorAttempt['maxspy'] === (int)$beforeOperatorAttempt['maxspy'], 'operator POST does not change target maxspy', $afterOperatorAttempt ?? array()),
            e2e_case($afterOperatorAttempt !== null && $beforeOperatorAttempt !== null && (int)$afterOperatorAttempt['dmfree'] === (int)$beforeOperatorAttempt['dmfree'], 'operator POST does not change target dark matter', $afterOperatorAttempt ?? array()),
            e2e_case($afterOperatorAttempt !== null && $beforeOperatorAttempt !== null && (int)$afterOperatorAttempt['admin'] === (int)$beforeOperatorAttempt['admin'], 'operator POST does not promote target user', $afterOperatorAttempt ?? array()),
        )),
    ));

    $adminAuth = e2e_prepare_session($attackerId, USER_TYPE_ADMIN, 'admin-update');
    $adminCookies = $adminAuth['cookies'];
    $adminPayload = e2e_user_update_payload($defenderFull, array(
        'pemail' => $token . '@primary.example.local',
        'email' => $token . '@login.example.local',
        'spio_anz' => '13',
        'settings_fleetactions' => '14',
        'dmfree' => '321',
        'dm' => '654',
        'settings_sort' => '2',
        'settings_order' => '1',
        'deact_ip' => 'on',
        'validated' => 'on',
        'admin' => (string)USER_TYPE_PLAYER,
    ));
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Users&action=update&player_id=' . $defenderId, $adminPayload, $adminCookies);
    $afterAdminUpdate = e2e_one_row("SELECT player_id, pemail, email, maxspy, maxfleetmsg, dmfree, dm, sortby, sortorder, deact_ip, validated, admin FROM {$db_prefix}users WHERE player_id={$defenderId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_updates_user_settings',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($afterAdminUpdate !== null && $afterAdminUpdate['pemail'] === $token . '@primary.example.local', 'admin updates primary email', $afterAdminUpdate ?? array()),
            e2e_case($afterAdminUpdate !== null && $afterAdminUpdate['email'] === $token . '@login.example.local', 'admin updates login email', $afterAdminUpdate ?? array()),
            e2e_case($afterAdminUpdate !== null && (int)$afterAdminUpdate['maxspy'] === 13 && (int)$afterAdminUpdate['maxfleetmsg'] === 14, 'admin updates user gameplay preferences', $afterAdminUpdate ?? array()),
            e2e_case($afterAdminUpdate !== null && (int)$afterAdminUpdate['dmfree'] === 321 && (int)$afterAdminUpdate['dm'] === 654, 'admin updates dark matter fields', $afterAdminUpdate ?? array()),
            e2e_case($afterAdminUpdate !== null && (int)$afterAdminUpdate['admin'] === USER_TYPE_PLAYER, 'admin keeps target as a regular player', $afterAdminUpdate ?? array()),
        )),
    ));

    $response = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Bans&action=ban', array(
        'id' => array($defenderId => 'on'),
        'banmode' => '1',
        'days' => '0',
        'hours' => '2',
        'reason' => $token . ' ban reason',
    ), $adminCookies);
    $afterBan = e2e_one_row("SELECT player_id, banned, banned_until, vacation, vacation_until FROM {$db_prefix}users WHERE player_id={$defenderId} LIMIT 1");
    $unbanQueueCount = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$defenderId} AND type='" . QTYP_UNBAN . "'");
    $loginCookies = array();
    $login = e2e_http_request('GET', $gameBase . '/reg/login2.php?login=' . rawurlencode($defenderName) . '&pass=' . rawurlencode($defenderPassword), array(), $loginCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_bans_user_and_login_is_blocked',
        'checks' => array_merge(e2e_response_check($response), e2e_response_check($login), array(
            e2e_case($afterBan !== null && (int)$afterBan['banned'] === 1 && (int)$afterBan['banned_until'] > time(), 'admin ban marks target banned with a future expiry', $afterBan ?? array()),
            e2e_case($afterBan !== null && (int)$afterBan['vacation'] === 1 && (int)$afterBan['vacation_until'] > time(), 'ban-with-vacation places target into vacation mode', $afterBan ?? array()),
            e2e_case($unbanQueueCount === 1, 'ban creates one unban queue task', array('unban_queue_count' => $unbanQueueCount)),
            e2e_case(stripos($login['body'] . $login['location'], 'errorcode=3') !== false, 'banned user login redirects to the ban error page', array('location' => $login['location'])),
        )),
    ));

    $response = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Bans&action=ban', array(
        'id' => array($defenderId => 'on'),
        'banmode' => '3',
        'days' => '0',
        'hours' => '0',
        'reason' => $token . ' unban reason',
    ), $adminCookies);
    $afterUnban = e2e_one_row("SELECT player_id, banned, banned_until FROM {$db_prefix}users WHERE player_id={$defenderId} LIMIT 1");
    $afterUnbanQueueCount = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$defenderId} AND type='" . QTYP_UNBAN . "'");
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_unbans_user',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($afterUnban !== null && (int)$afterUnban['banned'] === 0 && (int)$afterUnban['banned_until'] === 0, 'admin unban clears banned flags', $afterUnban ?? array()),
            e2e_case($afterUnbanQueueCount === 0, 'admin unban removes unban queue task', array('unban_queue_count' => $afterUnbanQueueCount)),
        )),
    ));

    e2e_reset_runtime_state($defenderId, $defenderPlanet);
    $vacationAuth = e2e_prepare_session($defenderId, USER_TYPE_PLAYER, 'vacation-state');
    $vacationCookies = $vacationAuth['cookies'];
    dbquery("UPDATE {$db_prefix}users SET vacation=1, vacation_until=" . (time() + 3600) . " WHERE player_id={$defenderId}");
    InvalidateUserCache();

    $productionBefore = e2e_one_row("SELECT planet_id, prod1, prod2, prod3 FROM {$db_prefix}planets WHERE planet_id={$defenderPlanet} LIMIT 1");
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=resources&session=' . rawurlencode($vacationAuth['session']), array(
        'last1' => 80,
        'last2' => 70,
        'last3' => 60,
        'action' => 'Recalculate',
    ), $vacationCookies);
    $productionAfter = e2e_one_row("SELECT planet_id, prod1, prod2, prod3 FROM {$db_prefix}planets WHERE planet_id={$defenderPlanet} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'vacation_blocks_resource_settings',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($productionBefore !== null && $productionAfter !== null && (float)$productionAfter['prod1'] === (float)$productionBefore['prod1'], 'vacation mode does not change metal production setting', $productionAfter ?? array()),
            e2e_case($productionBefore !== null && $productionAfter !== null && (float)$productionAfter['prod2'] === (float)$productionBefore['prod2'], 'vacation mode does not change crystal production setting', $productionAfter ?? array()),
            e2e_case($productionBefore !== null && $productionAfter !== null && (float)$productionAfter['prod3'] === (float)$productionBefore['prod3'], 'vacation mode does not change deuterium production setting', $productionAfter ?? array()),
        )),
    ));

    $buildCountBefore = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}buildqueue WHERE planet_id={$defenderPlanet}");
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=b_building&session=' . rawurlencode($vacationAuth['session']) . '&modus=add&techid=' . GID_B_METAL_MINE . '&planet=' . $defenderPlanet, array(), $vacationCookies);
    $buildCountAfter = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}buildqueue WHERE planet_id={$defenderPlanet}");
    $cases[] = e2e_finalize_case(array(
        'case' => 'vacation_blocks_build_enqueue',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($buildCountAfter === $buildCountBefore, 'vacation mode does not enqueue construction', array('before' => $buildCountBefore, 'after' => $buildCountAfter)),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'admin_state_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($attackerId > 0 && $attackerPlanet > 0) {
        e2e_reset_runtime_state($attackerId, $attackerPlanet);
    }
    if ($defenderId > 0 && $defenderPlanet > 0) {
        e2e_reset_runtime_state($defenderId, $defenderPlanet);
    }
    e2e_restore_user($attackerSnapshot);
    e2e_restore_user($defenderSnapshot);
}

echo json_encode(array(
    'case_group' => 'http_admin_state',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
