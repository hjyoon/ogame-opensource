<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_admin_permission_matrix_e2e.php';
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
loca_add('admin', 'en');
loca_add('technames', 'en');
loca_add('debug', 'en');

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
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined|Unknown column|SQL syntax/i', $body) === 1;
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

function e2e_count(string $sql): int
{
    $row = e2e_one_row($sql);
    return $row === null ? 0 : (int)$row['cnt'];
}

function e2e_mdb_query(string $sql): mixed
{
    if (!MDBConnect()) {
        throw new RuntimeException('Master database connection is not available.');
    }
    return MDBQuery($sql);
}

function e2e_mdb_one_row(string $sql): ?array
{
    $res = e2e_mdb_query($sql);
    if (!$res) {
        return null;
    }
    $row = MDBArray($res);
    return $row === null ? null : $row;
}

function e2e_prepare_session(int $userId, int $adminLevel, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', private_session='" . e2e_sql_escape($private) . "', admin={$adminLevel}, " .
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

function e2e_snapshot_user(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT player_id, admin, session, private_session, validated, deact_ip, vacation, vacation_until, banned, banned_until, noattack, noattack_until, disable, disable_until, lang, skin, useskin FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1");
}

function e2e_restore_user(?array $user): void
{
    global $db_prefix;
    if ($user === null) {
        return;
    }

    $id = (int)$user['player_id'];
    dbquery(
        "UPDATE {$db_prefix}users SET admin=" . (int)$user['admin'] . ", session='" . e2e_sql_escape($user['session']) . "', private_session='" . e2e_sql_escape($user['private_session']) . "', " .
        "validated=" . (int)$user['validated'] . ", deact_ip=" . (int)$user['deact_ip'] . ", vacation=" . (int)$user['vacation'] . ", vacation_until=" . (int)$user['vacation_until'] . ", " .
        "banned=" . (int)$user['banned'] . ", banned_until=" . (int)$user['banned_until'] . ", noattack=" . (int)$user['noattack'] . ", noattack_until=" . (int)$user['noattack_until'] . ", " .
        "disable=" . (int)$user['disable'] . ", disable_until=" . (int)$user['disable_until'] . ", lang='" . e2e_sql_escape($user['lang']) . "', skin='" . e2e_sql_escape($user['skin']) . "', " .
        "useskin=" . (int)$user['useskin'] . " WHERE player_id={$id}"
    );
    InvalidateUserCache();
}

function e2e_uni_payload(array $uni, bool $freeze): array
{
    return array(
        'news_upd' => '0',
        'news1' => $uni['news1'],
        'news2' => $uni['news2'],
        'maxusers' => (string)$uni['maxusers'],
        'start_dm' => (string)$uni['start_dm'],
        'galaxies' => (string)$uni['galaxies'],
        'systems' => (string)$uni['systems'],
        'max_werf' => (string)$uni['max_werf'],
        'feedage' => (string)$uni['feedage'],
        'speed' => (string)$uni['speed'],
        'fspeed' => (string)$uni['fspeed'],
        'acs' => (string)$uni['acs'],
        'fid' => (string)$uni['fid'],
        'did' => (string)$uni['did'],
        'rapid' => ((int)$uni['rapid']) ? 'on' : '',
        'moons' => ((int)$uni['moons']) ? 'on' : '',
        'defrepair' => (string)$uni['defrepair'],
        'defrepair_delta' => (string)$uni['defrepair_delta'],
        'freeze' => $freeze ? 'on' : '',
        'lang' => $uni['lang'],
        'force_lang' => ((int)$uni['force_lang']) ? 'on' : '',
        'battle_engine' => $uni['battle_engine'],
        'php_battle' => ((int)$uni['php_battle']) ? 'on' : '',
        'battle_max' => (string)$uni['battle_max'],
        'ext_board' => $uni['ext_board'],
        'ext_discord' => $uni['ext_discord'],
        'ext_tutorial' => $uni['ext_tutorial'],
        'ext_rules' => $uni['ext_rules'],
        'ext_impressum' => $uni['ext_impressum'],
    );
}

function e2e_set_uni_freeze(int $freeze): void
{
    global $db_prefix, $GlobalUni;
    dbquery("UPDATE {$db_prefix}uni SET freeze={$freeze}");
    $GlobalUni = LoadUniverse();
}

function e2e_queue_row(int $taskId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT task_id, owner_id, type, freeze, frozen, end FROM {$db_prefix}queue WHERE task_id={$taskId} LIMIT 1");
}

function e2e_coupon_max_id(): int
{
    $row = e2e_mdb_one_row("SELECT COALESCE(MAX(id), 0) AS max_id FROM coupons");
    return $row === null ? 0 : (int)$row['max_id'];
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderPlanet = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$cases = array();
$attackerSnapshot = null;
$defenderSnapshot = null;
$uniSnapshot = LoadUniverse();
$createdQueueId = 0;
$couponMaxBefore = -1;

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $attackerSnapshot = e2e_snapshot_user($attackerId);
    $defenderSnapshot = e2e_snapshot_user($defenderId);
    e2e_set_uni_freeze(0);

    $regularAuth = e2e_prepare_session($attackerId, USER_TYPE_PLAYER, 'admin-matrix-regular');
    $regularChecks = array();
    foreach (array('Queue', 'Uni', 'Coupons', 'Planets') as $mode) {
        $response = e2e_http_request('GET', $gameBase . '/index.php?page=admin&session=' . rawurlencode($regularAuth['session']) . '&mode=' . rawurlencode($mode), array(), $regularAuth['cookies']);
        $regularChecks = array_merge($regularChecks, e2e_response_check($response, 'regular ' . $mode));
        $regularChecks[] = e2e_case(stripos($response['body'], 'http-equiv') !== false && stripos($response['body'], 'refresh') !== false, 'regular user receives redirect shell for admin mode ' . $mode);
        $regularChecks[] = e2e_case(stripos($response['body'], 'mode=' . $mode) === false, 'regular user does not receive requested admin mode content for ' . $mode);
    }
    $cases[] = e2e_finalize_case(array(
        'case' => 'regular_user_denied_across_admin_only_modes',
        'checks' => $regularChecks,
    ));

    $operatorAuth = e2e_prepare_session($attackerId, USER_TYPE_GO, 'admin-matrix-operator');
    $adminAuth = e2e_prepare_session($defenderId, USER_TYPE_ADMIN, 'admin-matrix-admin');

    $createdQueueId = AddQueue($defenderId, QTYP_DEBUG, 0, 0, 0, time(), 3600, QUEUE_PRIO_DEBUG);
    $operatorQueueResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($operatorAuth['session']) . '&mode=Queue', array(
        'order_freeze' => (string)$createdQueueId,
    ), $operatorAuth['cookies']);
    $afterOperatorQueue = e2e_queue_row($createdQueueId);
    $adminQueueResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Queue', array(
        'order_freeze' => (string)$createdQueueId,
    ), $adminAuth['cookies']);
    $afterAdminQueue = e2e_queue_row($createdQueueId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'queue_controls_require_admin_not_operator',
        'checks' => array_merge(e2e_response_check($operatorQueueResponse), e2e_response_check($adminQueueResponse), array(
            e2e_case($createdQueueId > 0, 'setup creates a queue row for permission checks', array('queue_id' => $createdQueueId)),
            e2e_case($afterOperatorQueue !== null && (int)$afterOperatorQueue['freeze'] === 0, 'operator POST cannot freeze queue task', $afterOperatorQueue ?? array()),
            e2e_case($afterAdminQueue !== null && (int)$afterAdminQueue['freeze'] === 1, 'admin POST can freeze queue task', $afterAdminQueue ?? array()),
        )),
    ));

    $operatorUniResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($operatorAuth['session']) . '&mode=Uni', e2e_uni_payload($uniSnapshot, true), $operatorAuth['cookies']);
    $afterOperatorUni = LoadUniverse();
    $cases[] = e2e_finalize_case(array(
        'case' => 'universe_settings_post_requires_admin_not_operator',
        'checks' => array_merge(e2e_response_check($operatorUniResponse), array(
            e2e_case((int)$afterOperatorUni['freeze'] === 0, 'operator POST cannot enable universe freeze', array('freeze_after_operator' => (int)$afterOperatorUni['freeze'])),
        )),
    ));

    $couponMaxBefore = e2e_coupon_max_id();
    $operatorCouponResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($operatorAuth['session']) . '&mode=Coupons&action=add_one', array(
        'dm' => '12345',
    ), $operatorAuth['cookies']);
    $couponMaxAfterOperator = e2e_coupon_max_id();
    $adminCouponResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Coupons&action=add_one', array(
        'dm' => '12345',
    ), $adminAuth['cookies']);
    $couponMaxAfterAdmin = e2e_coupon_max_id();
    $cases[] = e2e_finalize_case(array(
        'case' => 'coupon_creation_requires_admin_not_operator',
        'checks' => array_merge(e2e_response_check($operatorCouponResponse), e2e_response_check($adminCouponResponse), array(
            e2e_case($couponMaxAfterOperator === $couponMaxBefore, 'operator POST cannot create coupon rows', array('before' => $couponMaxBefore, 'after_operator' => $couponMaxAfterOperator)),
            e2e_case($couponMaxAfterAdmin > $couponMaxAfterOperator, 'admin POST can create coupon row', array('after_operator' => $couponMaxAfterOperator, 'after_admin' => $couponMaxAfterAdmin)),
        )),
    ));

    $debrisBefore = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}planets WHERE g=(SELECT g FROM {$db_prefix}planets WHERE planet_id={$defenderPlanet}) AND s=(SELECT s FROM {$db_prefix}planets WHERE planet_id={$defenderPlanet}) AND p=(SELECT p FROM {$db_prefix}planets WHERE planet_id={$defenderPlanet}) AND type=" . PTYP_DF);
    $operatorPlanetResponse = e2e_http_request('GET', $gameBase . '/index.php?page=admin&session=' . rawurlencode($operatorAuth['session']) . '&mode=Planets&action=create_debris&cp=' . $defenderPlanet, array(), $operatorAuth['cookies']);
    $debrisAfterOperator = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}planets WHERE g=(SELECT g FROM {$db_prefix}planets WHERE planet_id={$defenderPlanet}) AND s=(SELECT s FROM {$db_prefix}planets WHERE planet_id={$defenderPlanet}) AND p=(SELECT p FROM {$db_prefix}planets WHERE planet_id={$defenderPlanet}) AND type=" . PTYP_DF);
    $cases[] = e2e_finalize_case(array(
        'case' => 'planet_mutation_actions_require_admin_not_operator',
        'checks' => array_merge(e2e_response_check($operatorPlanetResponse), array(
            e2e_case($debrisAfterOperator === $debrisBefore, 'operator GET action cannot create debris for a planet', array('before' => $debrisBefore, 'after_operator' => $debrisAfterOperator, 'planet' => $defenderPlanet)),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'admin_permission_matrix_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e), 'trace' => $e->getTraceAsString()))),
        'pass' => false,
    );
} finally {
    if ($createdQueueId > 0) {
        RemoveQueue($createdQueueId);
    }
    if ($couponMaxBefore >= 0) {
        e2e_mdb_query("DELETE FROM coupons WHERE id > {$couponMaxBefore}");
    }
    e2e_set_uni_freeze((int)$uniSnapshot['freeze']);
    e2e_restore_user($attackerSnapshot);
    e2e_restore_user($defenderSnapshot);
}

echo json_encode(array(
    'case_group' => 'http_admin_permission_matrix',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
