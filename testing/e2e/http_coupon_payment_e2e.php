<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['SERVER_NAME'] = '127.0.0.1';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_coupon_payment_e2e.php';
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
loca_add('premium', 'en');

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
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+(Undefined|Trying)/i', $body) === 1;
    return array(
        e2e_case(in_array($response['status'], array(200, 301, 302, 303), true), $label . ' returns an accepted status', array('status' => $response['status'], 'location' => $response['location'])),
        e2e_case(!$hasError, $label . ' body has no PHP error marker'),
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

function e2e_mdb_count(string $sql): int
{
    $row = e2e_mdb_one_row($sql);
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
        "SELECT player_id, admin, validated, deact_ip, vacation, vacation_until, banned, banned_until, " .
        "noattack, noattack_until, disable, disable_until, lang, skin, useskin, dm, dmfree " .
        "FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1"
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
        "validated=" . (int)$user['validated'] . ", " .
        "deact_ip=" . (int)$user['deact_ip'] . ", " .
        "vacation=" . (int)$user['vacation'] . ", " .
        "vacation_until=" . (int)$user['vacation_until'] . ", " .
        "banned=" . (int)$user['banned'] . ", " .
        "banned_until=" . (int)$user['banned_until'] . ", " .
        "noattack=" . (int)$user['noattack'] . ", " .
        "noattack_until=" . (int)$user['noattack_until'] . ", " .
        "disable=" . (int)$user['disable'] . ", " .
        "disable_until=" . (int)$user['disable_until'] . ", " .
        "lang='" . e2e_sql_escape($user['lang']) . "', " .
        "skin='" . e2e_sql_escape($user['skin']) . "', " .
        "useskin=" . (int)$user['useskin'] . ", " .
        "dm=" . (int)$user['dm'] . ", " .
        "dmfree=" . (int)$user['dmfree'] . " WHERE player_id={$id}"
    );
    InvalidateUserCache();
}

function e2e_user_dm(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT player_id, oname, dm, dmfree FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1");
}

function e2e_coupon_by_amount(int $amount, bool $onlyUnused = false): ?array
{
    $whereUnused = $onlyUnused ? ' AND used=0' : '';
    return e2e_mdb_one_row("SELECT id, code, amount, used, user_uni, user_id, user_name FROM coupons WHERE amount={$amount}{$whereUnused} ORDER BY id DESC LIMIT 1");
}

function e2e_coupon_by_id(int $id): ?array
{
    return e2e_mdb_one_row("SELECT id, code, amount, used, user_uni, user_id, user_name FROM coupons WHERE id={$id} LIMIT 1");
}

function e2e_cleanup_coupons(array $ids, array $amounts, array $userIds): void
{
    $intIds = array_values(array_unique(array_filter(array_map('intval', $ids), fn($id) => $id > 0)));
    if (!empty($intIds)) {
        e2e_mdb_query('DELETE FROM coupons WHERE id IN (' . implode(',', $intIds) . ')');
    }

    $intAmounts = array_values(array_unique(array_filter(array_map('intval', $amounts), fn($amount) => $amount > 0)));
    if (!empty($intAmounts)) {
        e2e_mdb_query('DELETE FROM coupons WHERE amount IN (' . implode(',', $intAmounts) . ') AND (used=0 OR user_id IN (' . implode(',', array_map('intval', $userIds)) . '))');
    }
}

function e2e_queue_snapshot(int $taskId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT task_id, owner_id, type, sub_id, obj_id, level, start, end, prio FROM {$db_prefix}queue WHERE task_id={$taskId} LIMIT 1");
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$token = 'E2ECP' . substr(md5((string)microtime(true)), 0, 8);
$seed = hexdec(substr(md5($token), 0, 4)) % 100000;
$redeemAmount = 800000 + $seed;
$deleteAmount = $redeemAmount + 1;
$deniedAmount = $redeemAmount + 2;
$queueAmount = $redeemAmount + 3;
$invalidCode = 'AAAA-BBBB-CCCC-DDDD-EEEE';
$cases = array();
$createdCouponIds = array();
$createdQueueIds = array();
$attackerSnapshot = null;
$defenderSnapshot = null;

try {
    if ($attackerId <= 0 || $defenderId <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }
    if (!MDBConnect()) {
        throw new RuntimeException('Master database is disabled or unavailable.');
    }

    $attackerSnapshot = e2e_snapshot_user($attackerId);
    $defenderSnapshot = e2e_snapshot_user($defenderId);
    e2e_cleanup_coupons(array(), array($redeemAmount, $deleteAmount, $deniedAmount, $queueAmount), array($attackerId, $defenderId));
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id=" . USER_SPACE . " AND type='" . QTYP_COUPON . "' AND sub_id IN ({$queueAmount},{$deniedAmount},{$redeemAmount},{$deleteAmount})");

    dbquery("UPDATE {$db_prefix}users SET dm=0, dmfree=0 WHERE player_id={$defenderId}");
    InvalidateUserCache();

    $regularAuth = e2e_prepare_session($defenderId, USER_TYPE_PLAYER, 'coupon-regular');
    $regularCookies = $regularAuth['cookies'];
    $deniedBefore = e2e_mdb_count("SELECT COUNT(*) AS cnt FROM coupons WHERE amount={$deniedAmount}");
    $deniedResponse = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=admin&session=' . rawurlencode($regularAuth['session']) . '&mode=Coupons&action=add_one',
        array('dm' => $deniedAmount),
        $regularCookies
    );
    $deniedAfter = e2e_mdb_count("SELECT COUNT(*) AS cnt FROM coupons WHERE amount={$deniedAmount}");
    $cases[] = e2e_finalize_case(array(
        'case' => 'regular_user_cannot_create_coupon',
        'checks' => array_merge(e2e_response_check($deniedResponse, 'regular coupon admin POST'), array(
            e2e_case($deniedAfter === $deniedBefore, 'regular user direct POST does not add a master coupon', array('before' => $deniedBefore, 'after' => $deniedAfter)),
            e2e_case(stripos($deniedResponse['body'], 'http-equiv') !== false && stripos($deniedResponse['body'], 'refresh') !== false, 'regular user receives admin redirect shell'),
        )),
    ));

    $adminAuth = e2e_prepare_session($attackerId, USER_TYPE_ADMIN, 'coupon-admin');
    $adminCookies = $adminAuth['cookies'];
    $createResponse = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Coupons&action=add_one',
        array('dm' => $redeemAmount),
        $adminCookies
    );
    $coupon = e2e_coupon_by_amount($redeemAmount, true);
    if ($coupon !== null) {
        $createdCouponIds[] = (int)$coupon['id'];
    }
    $listResponse = e2e_http_request(
        'GET',
        $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Coupons',
        array(),
        $adminCookies
    );
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_creates_coupon_and_list_renders_it',
        'checks' => array_merge(e2e_response_check($createResponse, 'admin coupon create POST'), e2e_response_check($listResponse, 'admin coupon list GET'), array(
            e2e_case($coupon !== null && (int)$coupon['amount'] === $redeemAmount && (int)$coupon['used'] === 0, 'admin creates an unused master coupon with requested paid DM amount', $coupon ?? array()),
            e2e_case($coupon !== null && preg_match('/^[0-9A-Z]{4}(?:-[0-9A-Z]{4}){4}$/', $coupon['code']) === 1, 'created coupon has the expected code shape', $coupon ?? array()),
            e2e_case($coupon !== null && strpos($createResponse['body'], $coupon['code']) !== false, 'admin create response shows the new coupon code'),
            e2e_case($coupon !== null && strpos($listResponse['body'], $coupon['code']) !== false && strpos($listResponse['body'], nicenum($redeemAmount)) !== false, 'admin coupon list renders the new coupon and amount'),
        )),
    ));

    $defenderAuth = e2e_prepare_session($defenderId, USER_TYPE_PLAYER, 'coupon-user');
    $defenderCookies = $defenderAuth['cookies'];
    $dmBeforeInvalid = e2e_user_dm($defenderId);
    $invalidResponse = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=payment&session=' . rawurlencode($defenderAuth['session']),
        array('action' => 'check', 'couponcode' => $invalidCode),
        $defenderCookies
    );
    $dmAfterInvalid = e2e_user_dm($defenderId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'payment_rejects_unknown_coupon_code',
        'checks' => array_merge(e2e_response_check($invalidResponse, 'invalid payment check POST'), array(
            e2e_case(stripos($invalidResponse['body'], 'Incorrect code or coupon already redeemed') !== false, 'unknown coupon check renders invalid-code error'),
            e2e_case($dmBeforeInvalid !== null && $dmAfterInvalid !== null && (int)$dmAfterInvalid['dm'] === (int)$dmBeforeInvalid['dm'] && (int)$dmAfterInvalid['dmfree'] === (int)$dmBeforeInvalid['dmfree'], 'unknown coupon does not change paid or free DM', array('before' => $dmBeforeInvalid, 'after' => $dmAfterInvalid)),
        )),
    ));

    $couponCode = $coupon['code'] ?? '';
    $dmBeforeCheck = e2e_user_dm($defenderId);
    $checkResponse = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=payment&session=' . rawurlencode($defenderAuth['session']),
        array('action' => 'check', 'couponcode' => $couponCode),
        $defenderCookies
    );
    $couponAfterCheck = $coupon === null ? null : e2e_coupon_by_id((int)$coupon['id']);
    $dmAfterCheck = e2e_user_dm($defenderId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'payment_check_shows_valid_coupon_without_redeeming',
        'checks' => array_merge(e2e_response_check($checkResponse, 'valid payment check POST'), array(
            e2e_case($couponCode !== '' && strpos($checkResponse['body'], $couponCode) !== false, 'valid coupon check renders activation form with the coupon code'),
            e2e_case(strpos($checkResponse['body'], nicenum($redeemAmount)) !== false, 'valid coupon check renders the paid DM amount'),
            e2e_case($couponAfterCheck !== null && (int)$couponAfterCheck['used'] === 0, 'checking a coupon leaves it unused', $couponAfterCheck ?? array()),
            e2e_case($dmBeforeCheck !== null && $dmAfterCheck !== null && (int)$dmAfterCheck['dm'] === (int)$dmBeforeCheck['dm'] && (int)$dmAfterCheck['dmfree'] === (int)$dmBeforeCheck['dmfree'], 'checking a coupon does not change user DM', array('before' => $dmBeforeCheck, 'after' => $dmAfterCheck)),
        )),
    ));

    $dmBeforeActivate = e2e_user_dm($defenderId);
    $activateResponse = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=payment&session=' . rawurlencode($defenderAuth['session']),
        array('action' => 'activate', 'couponcode' => $couponCode),
        $defenderCookies
    );
    $couponAfterActivate = $coupon === null ? null : e2e_coupon_by_id((int)$coupon['id']);
    $dmAfterActivate = e2e_user_dm($defenderId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'payment_activate_redeems_coupon_as_paid_dm',
        'checks' => array_merge(e2e_response_check($activateResponse, 'valid payment activate POST'), array(
            e2e_case(strpos($activateResponse['location'], 'page=micropayment') !== false, 'coupon activation redirects back to micropayment', array('location' => $activateResponse['location'])),
            e2e_case($couponAfterActivate !== null && (int)$couponAfterActivate['used'] === 1 && (int)$couponAfterActivate['user_id'] === $defenderId && (int)$couponAfterActivate['user_uni'] === (int)$GlobalUni['num'], 'activated coupon records the redeeming user and universe', $couponAfterActivate ?? array()),
            e2e_case($dmBeforeActivate !== null && $dmAfterActivate !== null && (int)$dmAfterActivate['dm'] === (int)$dmBeforeActivate['dm'] + $redeemAmount, 'activated coupon increases paid DM by coupon amount', array('before' => $dmBeforeActivate, 'after' => $dmAfterActivate, 'amount' => $redeemAmount)),
            e2e_case($dmBeforeActivate !== null && $dmAfterActivate !== null && (int)$dmAfterActivate['dmfree'] === (int)$dmBeforeActivate['dmfree'], 'activated coupon does not change free DM', array('before' => $dmBeforeActivate, 'after' => $dmAfterActivate)),
        )),
    ));

    $dmBeforeDuplicate = e2e_user_dm($defenderId);
    $duplicateActivateResponse = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=payment&session=' . rawurlencode($defenderAuth['session']),
        array('action' => 'activate', 'couponcode' => $couponCode),
        $defenderCookies
    );
    $dmAfterDuplicate = e2e_user_dm($defenderId);
    $usedCheckResponse = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=payment&session=' . rawurlencode($defenderAuth['session']),
        array('action' => 'check', 'couponcode' => $couponCode),
        $defenderCookies
    );
    $cases[] = e2e_finalize_case(array(
        'case' => 'used_coupon_cannot_be_redeemed_or_checked_again',
        'checks' => array_merge(e2e_response_check($duplicateActivateResponse, 'duplicate activate POST'), e2e_response_check($usedCheckResponse, 'used coupon check POST'), array(
            e2e_case($dmBeforeDuplicate !== null && $dmAfterDuplicate !== null && (int)$dmAfterDuplicate['dm'] === (int)$dmBeforeDuplicate['dm'] && (int)$dmAfterDuplicate['dmfree'] === (int)$dmBeforeDuplicate['dmfree'], 'duplicate activation does not grant additional DM', array('before' => $dmBeforeDuplicate, 'after' => $dmAfterDuplicate)),
            e2e_case(stripos($usedCheckResponse['body'], 'Incorrect code or coupon already redeemed') !== false, 'checking an already-used coupon renders invalid-code error'),
        )),
    ));

    $deleteResponseCreate = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Coupons&action=add_one',
        array('dm' => $deleteAmount),
        $adminCookies
    );
    $deleteCoupon = e2e_coupon_by_amount($deleteAmount, true);
    if ($deleteCoupon !== null) {
        $createdCouponIds[] = (int)$deleteCoupon['id'];
    }
    $deleteResponse = e2e_http_request(
        'GET',
        $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Coupons&action=remove_one&item_id=' . (int)($deleteCoupon['id'] ?? 0),
        array(),
        $adminCookies
    );
    $deleteCouponAfter = $deleteCoupon === null ? null : e2e_coupon_by_id((int)$deleteCoupon['id']);
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_deletes_unused_coupon',
        'checks' => array_merge(e2e_response_check($deleteResponseCreate, 'delete fixture coupon create POST'), e2e_response_check($deleteResponse, 'admin coupon delete GET'), array(
            e2e_case($deleteCoupon !== null && (int)$deleteCoupon['amount'] === $deleteAmount, 'admin creates a second unused coupon for deletion', $deleteCoupon ?? array()),
            e2e_case($deleteCouponAfter === null, 'admin deletes an unused coupon from the master database', $deleteCouponAfter ?? array()),
        )),
    ));

    $ddmm = date('d.m', time() + 86400);
    $hhmm = date('H:i', time() + 86400);
    $queueBefore = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id=" . USER_SPACE . " AND type='" . QTYP_COUPON . "' AND sub_id={$queueAmount}");
    $queueCreateResponse = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Coupons&action=add_date',
        array(
            'ddmm' => $ddmm,
            'hhmm' => $hhmm,
            'darkmatter' => $queueAmount,
            'inactive_days' => '11',
            'ingame_days' => '22',
            'periodic' => '33',
        ),
        $adminCookies
    );
    $queue = e2e_one_row("SELECT task_id, owner_id, type, sub_id, obj_id, level, prio FROM {$db_prefix}queue WHERE owner_id=" . USER_SPACE . " AND type='" . QTYP_COUPON . "' AND sub_id={$queueAmount} ORDER BY task_id DESC LIMIT 1");
    if ($queue !== null) {
        $createdQueueIds[] = (int)$queue['task_id'];
    }
    $queueDeleteResponse = e2e_http_request(
        'GET',
        $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Coupons&action=remove_date&item_id=' . (int)($queue['task_id'] ?? 0),
        array(),
        $adminCookies
    );
    $queueAfterDelete = $queue === null ? null : e2e_queue_snapshot((int)$queue['task_id']);
    $packedCriteria = (11 << 16) | 22;
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_creates_and_removes_periodic_coupon_queue',
        'checks' => array_merge(e2e_response_check($queueCreateResponse, 'coupon queue create POST'), e2e_response_check($queueDeleteResponse, 'coupon queue delete GET'), array(
            e2e_case($queueBefore === 0, 'test starts without an existing coupon queue for the selected amount', array('before' => $queueBefore)),
            e2e_case($queue !== null && (int)$queue['sub_id'] === $queueAmount && (int)$queue['obj_id'] === $packedCriteria && (int)$queue['level'] === 33 && (int)$queue['prio'] === QUEUE_PRIO_COUPON, 'admin periodic coupon form creates a coupon queue with packed criteria', array('queue' => $queue, 'expected_obj_id' => $packedCriteria)),
            e2e_case($queueAfterDelete === null, 'admin removes the periodic coupon queue task', $queueAfterDelete ?? array()),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'coupon_payment_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if (!empty($createdQueueIds)) {
        dbquery("DELETE FROM {$db_prefix}queue WHERE task_id IN (" . implode(',', array_map('intval', $createdQueueIds)) . ")");
    }
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id=" . USER_SPACE . " AND type='" . QTYP_COUPON . "' AND sub_id IN ({$queueAmount},{$deniedAmount},{$redeemAmount},{$deleteAmount})");
    try {
        e2e_cleanup_coupons($createdCouponIds, array($redeemAmount, $deleteAmount, $deniedAmount, $queueAmount), array($attackerId, $defenderId));
    } catch (Throwable $ignored) {
    }
    e2e_restore_user($attackerSnapshot);
    e2e_restore_user($defenderSnapshot);
}

echo json_encode(array(
    'case_group' => 'http_coupon_payment',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
