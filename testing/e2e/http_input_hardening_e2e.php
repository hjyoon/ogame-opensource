<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_input_hardening_e2e.php';
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
loca_add('build', 'en');
loca_add('fleet', 'en');
loca_add('infos', 'en');
loca_add('options', 'en');
loca_add('resources', 'en');
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

function e2e_response_check(array $response, string $label = 'HTTP request'): array
{
    $body = $response['body'];
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read|foreach)|Notice:\s+(Undefined|Trying)/i', $body) === 1;
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

function e2e_prepare_session(int $userId, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', validated=1, deact_ip=1, " .
        "lang='en', skin='/evolution/', useskin=1, vacation=0, vacation_until=0, " .
        "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0 " .
        "WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookies' => array('prsess_' . $userId . '_' . $GlobalUni['num'] => $private),
    );
}

function e2e_cleanup_fleets(array $userIds, array $planetIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', $userIds));
    $planetList = implode(',', array_map('intval', $planetIds));
    if ($userList === '' || $planetList === '') {
        return;
    }

    $fleetIds = array();
    $res = dbquery("SELECT fleet_id FROM {$db_prefix}fleet WHERE owner_id IN ({$userList}) OR start_planet IN ({$planetList}) OR target_planet IN ({$planetList})");
    while ($row = dbarray($res)) {
        $fleetIds[] = (int)$row['fleet_id'];
    }
    if (!empty($fleetIds)) {
        $fleetList = implode(',', $fleetIds);
        dbquery("DELETE FROM {$db_prefix}queue WHERE type='" . QTYP_FLEET . "' AND (owner_id IN ({$userList}) OR sub_id IN ({$fleetList}))");
        dbquery("DELETE FROM {$db_prefix}fleet WHERE fleet_id IN ({$fleetList})");
    }
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id IN ({$userList}) AND type='" . QTYP_FLEET . "'");
}

function e2e_cleanup_runtime(array $userIds, array $planetIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', $userIds));
    $planetList = implode(',', array_map('intval', $planetIds));
    if ($userList === '' || $planetList === '') {
        return;
    }

    e2e_cleanup_fleets($userIds, $planetIds);
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id IN ({$userList}) AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_UNBAN . "','" . QTYP_ALLOW_ATTACKS . "','" . QTYP_DEBUG . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id IN ({$userList}) OR planet_id IN ({$planetList})");
    foreach ($planetIds as $planetId) {
        @unlink('temp/fleetlock_' . (int)$planetId);
    }
}

function e2e_reset_user_and_planet(int $userId, int $planetId, string $name): void
{
    global $db_prefix, $fleetmap, $defmap, $buildmap, $resmap;

    e2e_cleanup_runtime(array($userId), array($planetId));

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=10";
    }
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", admin=0, ally_id=0, allyrank=0, " .
        "validated=1, deact_ip=1, vacation=0, vacation_until=0, banned=0, banned_until=0, " .
        "noattack=0, noattack_until=0, disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, " .
        "score1=10000, score2=0, score3=0, place1=1, place2=1, place3=1, flags=" . USER_FLAG_DEFAULT . " " .
        "WHERE player_id={$userId}"
    );

    $objects = array();
    foreach (array_merge($fleetmap, $defmap, $buildmap) as $gid) {
        $objects[] = "`{$gid}`=0";
    }
    $now = time();
    dbquery(
        "UPDATE {$db_prefix}planets SET " . implode(',', $objects) . ", name='" . e2e_sql_escape($name) . "', " .
        "owner_id={$userId}, type=" . PTYP_PLANET . ", remove=0, " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "`" . GID_B_SOLAR . "`=20, `" . GID_B_SHIPYARD . "`=12, `" . GID_B_ROBOTS . "`=10, `" . GID_B_MISS_SILO . "`=2, " .
        "`" . GID_F_SC . "`=10, `" . GID_F_LF . "`=10, `" . GID_F_PROBE . "`=10, " .
        "`" . GID_D_ABM . "`=5, `" . GID_D_IPM . "`=3, prod1=1, prod2=1, prod3=1, prod4=1, prod12=1, prod212=1, " .
        "fields=0, maxfields=300, lastpeek={$now}, lastakt={$now} WHERE planet_id={$planetId}"
    );
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$planetId}, aktplanet={$planetId} WHERE player_id={$userId}");
    InvalidateUserCache();
}

function e2e_planet_resource_snapshot(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, prod1, prod2, prod3, prod4, prod12, prod212, `" . GID_RC_METAL . "` AS metal, " .
        "`" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deuterium, `" . GID_F_SC . "` AS small_cargo, " .
        "`" . GID_F_PROBE . "` AS probe, `" . GID_D_ABM . "` AS abm, `" . GID_D_IPM . "` AS ipm " .
        "FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1"
    );
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
        'settings_sort' => '0',
        'settings_order' => '0',
        'noipcheck' => 'on',
        'spio_anz' => '1',
        'settings_fleetactions' => '1',
    );
    foreach ($overrides as $key => $value) {
        $payload[$key] = (string)$value;
    }
    return $payload;
}

function e2e_fleet_payload(array $origin, array $target, int $order, array $ships, array $resources = array(), array $extra = array()): array
{
    global $fleetmap, $transportableResources, $GlobalUni;

    $payload = array(
        'thisgalaxy' => (string)$origin['g'],
        'thissystem' => (string)$origin['s'],
        'thisplanet' => (string)$origin['p'],
        'thisplanettype' => (string)GetPlanetType($origin),
        'speedfactor' => (string)$GlobalUni['fspeed'],
        'galaxy' => (string)$target['g'],
        'system' => (string)$target['s'],
        'planet' => (string)$target['p'],
        'planettype' => (string)GetPlanetType($target),
        'speed' => '10',
        'order' => (string)$order,
    );
    foreach ($fleetmap as $gid) {
        $payload['ship' . $gid] = (string)($ships[$gid] ?? 0);
    }
    foreach ($transportableResources as $i => $rc) {
        $payload['resource' . ($i + 1)] = (string)($resources[$rc] ?? 0);
    }
    foreach ($extra as $key => $value) {
        $payload[$key] = (string)$value;
    }
    return $payload;
}

function e2e_fleet_count(int $userId): int
{
    global $db_prefix;
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id={$userId}");
}

function e2e_latest_fleet(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT fleet_id, owner_id, mission, `" . GID_F_SC . "` AS small_cargo, `" . GID_F_PROBE . "` AS probe, " .
        "`" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deuterium " .
        "FROM {$db_prefix}fleet WHERE owner_id={$userId} ORDER BY fleet_id DESC LIMIT 1"
    );
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderPlanet = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$cases = array();

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    e2e_reset_user_and_planet($attackerId, $attackerPlanet, 'E2E Harden A');
    e2e_reset_user_and_planet($defenderId, $defenderPlanet, 'E2E Harden D');
    $auth = e2e_prepare_session($attackerId, 'input-hardening');
    $cookies = $auth['cookies'];
    $session = $auth['session'];

    $resourceResponse = e2e_http_request('POST', $gameBase . '/index.php?page=resources&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, array(
        'last1' => '-250',
        'last2' => 'not-a-number',
        'last3' => '35',
        'last4' => '100',
        'last12' => '70',
        'last212' => '10',
        'action' => 'Recalculate',
    ), $cookies);
    $resourcePlanet = e2e_planet_resource_snapshot($attackerPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'resource_percent_inputs_are_clamped_or_normalized',
        'checks' => array_merge(e2e_response_check($resourceResponse, 'Resource settings POST'), array(
            e2e_case($resourcePlanet !== null && abs((float)$resourcePlanet['prod1'] - 0.0) < 0.001, 'negative production percent is clamped to zero', $resourcePlanet ?? array()),
            e2e_case($resourcePlanet !== null && abs((float)$resourcePlanet['prod2'] - 0.0) < 0.001, 'non-numeric production percent is treated as zero', $resourcePlanet ?? array()),
            e2e_case($resourcePlanet !== null && abs((float)$resourcePlanet['prod3'] - 0.4) < 0.001, 'odd production percent is rounded to the nearest 10 percent', $resourcePlanet ?? array()),
        )),
    ));

    $user = LoadUser($attackerId);
    $optionsResponse = e2e_http_request('POST', $gameBase . '/index.php?page=options&session=' . rawurlencode($session) . '&mode=change&cp=' . $attackerPlanet, e2e_options_payload($user, array(
        'settings_sort' => '9999',
        'settings_order' => '-9999',
        'spio_anz' => '-42',
        'settings_fleetactions' => '99999',
    )), $cookies);
    $optionsUser = e2e_one_row("SELECT player_id, sortby, sortorder, maxspy, maxfleetmsg FROM {$db_prefix}users WHERE player_id={$attackerId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'option_numeric_inputs_are_bounded',
        'checks' => array_merge(e2e_response_check($optionsResponse, 'Options POST'), array(
            e2e_case($optionsUser !== null && (int)$optionsUser['sortby'] === 2, 'sort field is capped at the highest supported option', $optionsUser ?? array()),
            e2e_case($optionsUser !== null && (int)$optionsUser['sortorder'] === 0, 'negative sort order is clamped to zero', $optionsUser ?? array()),
            e2e_case($optionsUser !== null && (int)$optionsUser['maxspy'] === 1, 'negative spy count is clamped to one', $optionsUser ?? array()),
            e2e_case($optionsUser !== null && (int)$optionsUser['maxfleetmsg'] === 99, 'oversized fleet message count is capped', $optionsUser ?? array()),
        )),
    ));

    e2e_cleanup_runtime(array($attackerId), array($attackerPlanet));
    $missingShipyardResponse = e2e_http_request('POST', $gameBase . '/index.php?page=buildings&mode=Flotte&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, array(), $cookies);
    $missingShipyardQueue = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_SHIPYARD . "'");
    $cases[] = e2e_finalize_case(array(
        'case' => 'shipyard_post_without_order_array_is_noop',
        'checks' => array_merge(e2e_response_check($missingShipyardResponse, 'Missing shipyard order POST'), array(
            e2e_case($missingShipyardQueue === 0, 'missing fmenge array does not create shipyard queue tasks', array('queue_count' => $missingShipyardQueue)),
        )),
    ));

    $negativeShipyardResponse = e2e_http_request('POST', $gameBase . '/index.php?page=buildings&mode=Flotte&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, array(
        'fmenge' => array(GID_F_SC => '-999', GID_F_LF => 'not-a-number'),
    ), $cookies);
    $negativeShipyardQueue = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_SHIPYARD . "'");
    $cases[] = e2e_finalize_case(array(
        'case' => 'shipyard_negative_and_non_numeric_orders_are_noop',
        'checks' => array_merge(e2e_response_check($negativeShipyardResponse, 'Invalid shipyard order POST'), array(
            e2e_case($negativeShipyardQueue === 0, 'negative and non-numeric shipyard quantities do not create queue tasks', array('queue_count' => $negativeShipyardQueue)),
        )),
    ));

    $oversizedShipyardResponse = e2e_http_request('POST', $gameBase . '/index.php?page=buildings&mode=Flotte&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, array(
        'fmenge' => array(GID_F_SC => '999999999999999999'),
    ), $cookies);
    $oversizedShipyardQueue = e2e_one_row("SELECT task_id, obj_id, level FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_SHIPYARD . "' ORDER BY task_id DESC LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'shipyard_oversized_order_is_capped',
        'checks' => array_merge(e2e_response_check($oversizedShipyardResponse, 'Oversized shipyard order POST'), array(
            e2e_case($oversizedShipyardQueue !== null && (int)$oversizedShipyardQueue['obj_id'] === GID_F_SC, 'oversized shipyard order still creates the intended queue item', $oversizedShipyardQueue ?? array()),
            e2e_case($oversizedShipyardQueue !== null && (int)$oversizedShipyardQueue['level'] > 0 && (int)$oversizedShipyardQueue['level'] <= (int)$GlobalUni['max_werf'], 'oversized shipyard quantity is capped by max_werf', $oversizedShipyardQueue ?? array()),
        )),
    ));
    e2e_cleanup_runtime(array($attackerId), array($attackerPlanet));

    $beforeMissiles = e2e_planet_resource_snapshot($attackerPlanet);
    $missileResponse = e2e_http_request('POST', $gameBase . '/index.php?page=infos&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&gid=' . GID_B_MISS_SILO, array(
        'action' => 'Destroy',
        'ab' . GID_D_ABM => '-99',
        'ab' . GID_D_IPM => 'not-a-number',
    ), $cookies);
    $afterMissiles = e2e_planet_resource_snapshot($attackerPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'missile_silo_demolition_rejects_negative_amounts',
        'checks' => array_merge(e2e_response_check($missileResponse, 'Missile silo demolition POST'), array(
            e2e_case($beforeMissiles !== null && $afterMissiles !== null && (int)$afterMissiles['abm'] === (int)$beforeMissiles['abm'], 'negative ABM demolition amount does not change ABM count', array('before' => $beforeMissiles, 'after' => $afterMissiles)),
            e2e_case($beforeMissiles !== null && $afterMissiles !== null && (int)$afterMissiles['ipm'] === (int)$beforeMissiles['ipm'], 'non-numeric IPM demolition amount does not change IPM count', array('before' => $beforeMissiles, 'after' => $afterMissiles)),
        )),
    ));

    e2e_cleanup_runtime(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    $origin = LoadPlanetById($attackerPlanet);
    $target = LoadPlanetById($defenderPlanet);
    $fleetResponse = e2e_http_request('POST', $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, e2e_fleet_payload(
        $origin,
        $target,
        FTYP_TRANSPORT,
        array(GID_F_SC => 1),
        array(GID_RC_METAL => -500, GID_RC_CRYSTAL => -7, GID_RC_DEUTERIUM => -1)
    ), $cookies);
    $negativeResourceFleet = e2e_latest_fleet($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'fleet_negative_resource_payloads_are_not_abs_loaded',
        'checks' => array_merge(e2e_response_check($fleetResponse, 'Negative-resource fleet POST'), array(
            e2e_case($negativeResourceFleet !== null && (int)$negativeResourceFleet['mission'] === FTYP_TRANSPORT, 'transport fleet is still sent with valid ships', $negativeResourceFleet ?? array()),
            e2e_case($negativeResourceFleet !== null && (float)$negativeResourceFleet['metal'] == 0.0 && (float)$negativeResourceFleet['crystal'] == 0.0 && (float)$negativeResourceFleet['deuterium'] == 0.0, 'negative resource payloads are clamped to zero instead of converted to positive cargo', $negativeResourceFleet ?? array()),
        )),
    ));

    e2e_cleanup_runtime(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    $origin = LoadPlanetById($attackerPlanet);
    $target = LoadPlanetById($defenderPlanet);
    $beforeNegativeShips = e2e_fleet_count($attackerId);
    $negativeShipsResponse = e2e_http_request('POST', $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, e2e_fleet_payload(
        $origin,
        $target,
        FTYP_TRANSPORT,
        array(GID_F_SC => -3),
        array(GID_RC_METAL => 100)
    ), $cookies);
    $afterNegativeShips = e2e_fleet_count($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'fleet_negative_ship_counts_are_rejected',
        'checks' => array_merge(e2e_response_check($negativeShipsResponse, 'Negative-ship fleet POST'), array(
            e2e_case($afterNegativeShips === $beforeNegativeShips, 'negative ship count does not create a fleet row', array('before' => $beforeNegativeShips, 'after' => $afterNegativeShips)),
        )),
    ));

    e2e_cleanup_runtime(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    $target = LoadPlanetById($defenderPlanet);
    $beforeAjax = e2e_fleet_count($attackerId);
    $ajaxCookies = $auth['cookies'];
    $ajaxResponse = e2e_http_request('POST', $gameBase . '/index.php?ajax=1&page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, array(
        'session' => $session,
        'order' => FTYP_SPY,
        'galaxy' => $target['g'],
        'system' => $target['s'],
        'planet' => $target['p'],
        'planettype' => GetPlanetType($target),
        'shipcount' => '-5',
        'speed' => '10',
        'reply' => 'short',
    ), $ajaxCookies);
    $afterAjax = e2e_fleet_count($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'ajax_negative_shipcount_is_rejected',
        'checks' => array_merge(e2e_response_check($ajaxResponse, 'Negative AJAX shipcount POST'), array(
            e2e_case(strpos(trim($ajaxResponse['body']), '611 ') === 0, 'negative AJAX shipcount returns the no-ships error code', array('body' => trim($ajaxResponse['body']))),
            e2e_case($afterAjax === $beforeAjax, 'negative AJAX shipcount does not create a fleet row', array('before' => $beforeAjax, 'after' => $afterAjax)),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'input_hardening_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($attackerId > 0 && $defenderId > 0 && $attackerPlanet > 0 && $defenderPlanet > 0) {
        e2e_reset_user_and_planet($attackerId, $attackerPlanet, 'E2E Fixture Attacker');
        e2e_reset_user_and_planet($defenderId, $defenderPlanet, 'E2E Fixture Defender');
    }
}

echo json_encode(array(
    'case_group' => 'http_input_hardening',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
