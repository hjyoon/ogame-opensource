<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_trader_moon_e2e.php';
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
loca_add('fleetmsg', 'en');
loca_add('premium', 'en');
loca_add('trader', 'en');
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

function e2e_prepare_session(int $userId, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', validated=1, deact_ip=1, " .
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

function e2e_cleanup_fleets(array $userIds, array $planetIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', $userIds));
    $planetList = implode(',', array_map('intval', $planetIds));
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
    dbquery("DELETE FROM {$db_prefix}queue WHERE type='" . QTYP_FLEET . "' AND owner_id IN ({$userList})");
}

function e2e_reset_user(int $userId): void
{
    global $db_prefix, $resmap;

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=10";
    }
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", " .
        "admin=0, validated=1, deact_ip=1, vacation=0, vacation_until=0, banned=0, banned_until=0, " .
        "noattack=0, noattack_until=0, disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, " .
        "dm=0, dmfree=0, trader=0, rate_m=0, rate_k=0, rate_d=0, com_until=0, adm_until=0, eng_until=0, geo_until=0, tec_until=0 " .
        "WHERE player_id={$userId}"
    );
    InvalidateUserCache();
}

function e2e_prepare_planet(int $planetId, int $ownerId): void
{
    global $db_prefix;
    $now = time();

    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000, " .
        "`" . GID_B_METAL_MINE . "`=0, `" . GID_B_CRYS_MINE . "`=0, `" . GID_B_DEUT_SYNTH . "`=0, `" . GID_B_SOLAR . "`=10, " .
        "`" . GID_B_FUSION . "`=0, `" . GID_B_ROBOTS . "`=10, `" . GID_B_NANITES . "`=0, `" . GID_B_SHIPYARD . "`=10, " .
        "`" . GID_B_METAL_STOR . "`=10, `" . GID_B_CRYS_STOR . "`=10, `" . GID_B_DEUT_STOR . "`=10, `" . GID_B_RES_LAB . "`=10, " .
        "`" . GID_B_LUNAR_BASE . "`=0, `" . GID_B_PHALANX . "`=0, `" . GID_B_JUMP_GATE . "`=0, " .
        "`" . GID_F_SC . "`=10, `" . GID_F_LC . "`=5, `" . GID_F_LF . "`=5, `" . GID_F_PROBE . "`=5, " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=0, maxfields=200, type=" . PTYP_PLANET . ", owner_id={$ownerId}, lastpeek={$now} " .
        "WHERE planet_id={$planetId}"
    );
}

function e2e_reset_user_and_planet(int $userId, int $planetId): void
{
    global $db_prefix;

    e2e_cleanup_fleets(array($userId), array($planetId));
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$userId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_UNBAN . "','" . QTYP_ALLOW_ATTACKS . "','" . QTYP_DEBUG . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$userId} OR planet_id={$planetId}");
    e2e_reset_user($userId);
    e2e_prepare_planet($planetId, $userId);
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$planetId} WHERE player_id={$userId}");
    SelectPlanet($userId, $planetId);
}

function e2e_prepare_moon(int $moonId, int $ownerId, array $buildings = array(), array $ships = array(), array $resources = array()): void
{
    global $db_prefix, $buildmap, $fleetmap;
    $now = time();

    $buildingAssignments = array();
    foreach ($buildmap as $gid) {
        $buildingAssignments[] = "`{$gid}`=" . (int)($buildings[$gid] ?? 0);
    }
    $shipAssignments = array();
    foreach ($fleetmap as $gid) {
        $shipAssignments[] = "`{$gid}`=" . (int)($ships[$gid] ?? 0);
    }
    $lunarBase = (int)($buildings[GID_B_LUNAR_BASE] ?? 0);
    $fields = array_sum(array_map('intval', $buildings));
    $maxfields = 1 + ($lunarBase * 3);

    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        implode(',', $buildingAssignments) . ", " .
        implode(',', $shipAssignments) . ", " .
        "`" . GID_RC_METAL . "`=" . (int)($resources[GID_RC_METAL] ?? 1000000) . ", " .
        "`" . GID_RC_CRYSTAL . "`=" . (int)($resources[GID_RC_CRYSTAL] ?? 1000000) . ", " .
        "`" . GID_RC_DEUTERIUM . "`=" . (int)($resources[GID_RC_DEUTERIUM] ?? 1000000) . ", " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields={$fields}, maxfields={$maxfields}, type=" . PTYP_MOON . ", owner_id={$ownerId}, gate_until=0, lastpeek={$now} " .
        "WHERE planet_id={$moonId}"
    );
}

function e2e_force_complete_queue(int $taskId, int $unitSeconds = 1): void
{
    global $db_prefix;
    $now = time();
    $start = $now - 10;
    $end = $start + max(1, $unitSeconds);
    dbquery("UPDATE {$db_prefix}queue SET start={$start}, end={$end}, freeze=0, frozen=0 WHERE task_id={$taskId}");
    dbquery("UPDATE {$db_prefix}buildqueue SET start={$start}, end={$end} WHERE id = ANY (SELECT sub_id FROM {$db_prefix}queue WHERE task_id={$taskId})");
    UpdateQueue($now);
}

function e2e_find_empty_position(array $near): array
{
    global $GlobalUni;

    $g = (int)$near['g'];
    $s = (int)$near['s'];
    for ($p = 1; $p <= 15; $p++) {
        if ($p === (int)$near['p']) {
            continue;
        }
        if (!HasPlanet($g, $s, $p)) {
            return array($g, $s, $p);
        }
    }

    for ($system = 1; $system <= (int)$GlobalUni['systems']; $system++) {
        for ($p = 1; $p <= 15; $p++) {
            if (!HasPlanet($g, $system, $p)) {
                return array($g, $system, $p);
            }
        }
    }

    throw new RuntimeException('No empty position found.');
}

function e2e_create_planet_near(int $ownerId, array $near): int
{
    [$g, $s, $p] = e2e_find_empty_position($near);
    $planetId = CreatePlanet($g, $s, $p, $ownerId, 1, 0, 0, time());
    if ($planetId <= 0) {
        throw new RuntimeException('Failed to create a nearby planet.');
    }
    e2e_prepare_planet($planetId, $ownerId);
    return $planetId;
}

function e2e_create_moon_for_planet(int $planetId, int $ownerId): int
{
    $planet = LoadPlanetById($planetId);
    if ($planet === null) {
        throw new RuntimeException('Cannot create moon for missing planet.');
    }
    $moonId = CreatePlanet((int)$planet['g'], (int)$planet['s'], (int)$planet['p'], $ownerId, 1, 1, 20, time());
    if ($moonId <= 0) {
        throw new RuntimeException('Failed to create moon.');
    }
    return $moonId;
}

function e2e_user_snapshot(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT player_id, dm, dmfree, trader, rate_m, rate_k, rate_d, com_until FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1");
}

function e2e_planet_snapshot(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, type, g, s, p, fields, maxfields, gate_until, lastpeek, " .
        "`" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deuterium, " .
        "`" . GID_B_LUNAR_BASE . "` AS lunar_base, `" . GID_B_PHALANX . "` AS phalanx, `" . GID_B_JUMP_GATE . "` AS jump_gate, " .
        "`" . GID_F_SC . "` AS small_cargo, `" . GID_F_LF . "` AS light_fighter " .
        "FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1"
    );
}

function e2e_dispatch_phalanx_visible_fleet(int $ownerId, int $originPlanetId, int $targetPlanetId): int
{
    global $fleetmap, $transportableResources;

    $fleet = array();
    foreach ($fleetmap as $gid) {
        $fleet[$gid] = $gid === GID_F_SC ? 1 : 0;
    }
    $resources = array();
    foreach ($transportableResources as $rc) {
        $resources[$rc] = 0;
    }

    $origin = LoadPlanetById($originPlanetId);
    $target = LoadPlanetById($targetPlanetId);
    if ($origin === null || $target === null) {
        throw new RuntimeException('Cannot dispatch phalanx fixture fleet.');
    }
    AdjustShips($fleet, $originPlanetId, '-');
    return DispatchFleet($fleet, $origin, $target, FTYP_TRANSPORT, 3600, $resources, 0, time());
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderPlanet = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$cases = array();
$createdPlanets = array();
$createdMoons = array();

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet);
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));

    $auth = e2e_prepare_session($attackerId, 'trader-moon-attacker');
    $cookies = $auth['cookies'];
    $session = $auth['session'];

    dbquery("UPDATE {$db_prefix}users SET dm=0, dmfree=0, trader=0, rate_m=0, rate_k=0, rate_d=0 WHERE player_id={$attackerId}");
    $before = e2e_user_snapshot($attackerId);
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=trader&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, array(
        'offer_id' => '1',
        'call_trader' => 'Call',
    ), $cookies);
    $after = e2e_user_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'trader_call_requires_dark_matter',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($before !== null && $after !== null && (int)$after['trader'] === 0, 'insufficient DM does not assign a trader', $after ?? array()),
            e2e_case($before !== null && $after !== null && (int)$after['dm'] === (int)$before['dm'] && (int)$after['dmfree'] === (int)$before['dmfree'], 'insufficient DM does not spend dark matter', array('before' => $before, 'after' => $after)),
        )),
    ));

    dbquery("UPDATE {$db_prefix}users SET dm=1000, dmfree=2000, trader=0, rate_m=0, rate_k=0, rate_d=0 WHERE player_id={$attackerId}");
    $before = e2e_user_snapshot($attackerId);
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=trader&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, array(
        'offer_id' => '1',
        'call_trader' => 'Call',
    ), $cookies);
    $after = e2e_user_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'trader_call_spends_dark_matter_and_assigns_rates',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($after !== null && (int)$after['trader'] === 1, 'calling a metal trader assigns trader type 1', $after ?? array()),
            e2e_case($before !== null && $after !== null && ((int)$before['dm'] + (int)$before['dmfree']) - ((int)$after['dm'] + (int)$after['dmfree']) === TRADER_DM, 'calling a trader spends the configured dark matter cost', array('before' => $before, 'after' => $after, 'cost' => TRADER_DM)),
            e2e_case($after !== null && (float)$after['rate_m'] > 0 && (float)$after['rate_k'] > 0 && (float)$after['rate_d'] > 0, 'calling a trader stores non-zero exchange rates', $after ?? array()),
        )),
    ));

    dbquery("UPDATE {$db_prefix}users SET trader=1, rate_m=3, rate_k=2, rate_d=1 WHERE player_id={$attackerId}");
    dbquery("UPDATE {$db_prefix}planets SET `" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=100000, `" . GID_RC_DEUTERIUM . "`=100000, lastpeek=" . time() . " WHERE planet_id={$attackerPlanet}");
    $beforePlanet = e2e_planet_snapshot($attackerPlanet);
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=trader&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, array(
        'trade' => 'Trade',
        '1_value' => '0',
        '2_value' => '2000',
        '3_value' => '1000',
    ), $cookies);
    $afterPlanet = e2e_planet_snapshot($attackerPlanet);
    $afterUser = e2e_user_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'trader_exchange_applies_rates_and_consumes_offer',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($beforePlanet !== null && $afterPlanet !== null && (int)$afterPlanet['metal'] === (int)$beforePlanet['metal'] - 6000, 'trader exchange subtracts calculated metal cost', array('before' => $beforePlanet, 'after' => $afterPlanet)),
            e2e_case($beforePlanet !== null && $afterPlanet !== null && (int)$afterPlanet['crystal'] === (int)$beforePlanet['crystal'] + 2000 && (int)$afterPlanet['deuterium'] === (int)$beforePlanet['deuterium'] + 1000, 'trader exchange adds requested crystal and deuterium', array('before' => $beforePlanet, 'after' => $afterPlanet)),
            e2e_case($afterUser !== null && (int)$afterUser['trader'] === 0, 'successful exchange consumes the active trader offer', $afterUser ?? array()),
        )),
    ));

    dbquery("UPDATE {$db_prefix}users SET trader=1, rate_m=3, rate_k=2, rate_d=1 WHERE player_id={$attackerId}");
    dbquery("UPDATE {$db_prefix}planets SET `" . GID_RC_METAL . "`=1000, `" . GID_RC_CRYSTAL . "`=100000, `" . GID_RC_DEUTERIUM . "`=100000, lastpeek=" . time() . " WHERE planet_id={$attackerPlanet}");
    $beforePlanet = e2e_planet_snapshot($attackerPlanet);
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=trader&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, array(
        'trade' => 'Trade',
        '1_value' => '0',
        '2_value' => '2000',
        '3_value' => '1000',
    ), $cookies);
    $afterPlanet = e2e_planet_snapshot($attackerPlanet);
    $afterUser = e2e_user_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'trader_exchange_rejects_insufficient_resource',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($beforePlanet !== null && $afterPlanet !== null && (int)$afterPlanet['metal'] === (int)$beforePlanet['metal'] && (int)$afterPlanet['crystal'] === (int)$beforePlanet['crystal'] && (int)$afterPlanet['deuterium'] === (int)$beforePlanet['deuterium'], 'failed exchange leaves planet resources unchanged', array('before' => $beforePlanet, 'after' => $afterPlanet)),
            e2e_case($afterUser !== null && (int)$afterUser['trader'] === 1, 'failed exchange keeps the active trader offer', $afterUser ?? array()),
        )),
    ));

    dbquery("UPDATE {$db_prefix}users SET dm=12000, dmfree=0, com_until=0 WHERE player_id={$attackerId}");
    $before = e2e_user_snapshot($attackerId);
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=micropayment&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&buynow=1&type=' . USER_OFFICER_COMMANDER . '&days=7', array(), $cookies);
    $after = e2e_user_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'premium_officer_purchase_spends_dark_matter_and_extends_timer',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($before !== null && $after !== null && (int)$after['dm'] === (int)$before['dm'] - 10000, 'officer purchase spends seven-day officer cost from paid DM', array('before' => $before, 'after' => $after)),
            e2e_case($after !== null && (int)$after['com_until'] >= time() + (6 * 24 * 60 * 60), 'commander timer is extended by purchase', $after ?? array()),
        )),
    ));

    $home = LoadPlanetById($attackerPlanet);
    $attackerMoon = e2e_create_moon_for_planet($attackerPlanet, $attackerId);
    $createdMoons[] = $attackerMoon;
    e2e_prepare_moon($attackerMoon, $attackerId, array(), array(), array(GID_RC_METAL => 1000000, GID_RC_CRYSTAL => 1000000, GID_RC_DEUTERIUM => 1000000));
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=b_building&session=' . rawurlencode($session) . '&cp=' . $attackerMoon . '&modus=add&techid=' . GID_B_LUNAR_BASE . '&planet=' . $attackerMoon, array(), $cookies);
    $buildTask = e2e_one_row("SELECT task_id FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "' AND obj_id=" . GID_B_LUNAR_BASE . " ORDER BY task_id DESC LIMIT 1");
    if ($buildTask !== null) {
        e2e_force_complete_queue((int)$buildTask['task_id']);
    }
    $moonAfterBuild = e2e_planet_snapshot($attackerMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'moon_lunar_base_build_queue_completion',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($buildTask !== null, 'lunar base build creates a build queue task on a moon', $buildTask ?? array()),
            e2e_case($moonAfterBuild !== null && (int)$moonAfterBuild['lunar_base'] === 1, 'completed moon build increments lunar base level', $moonAfterBuild ?? array()),
            e2e_case($moonAfterBuild !== null && (int)$moonAfterBuild['fields'] === 1 && (int)$moonAfterBuild['maxfields'] === 4, 'lunar base completion updates moon fields and max fields', $moonAfterBuild ?? array()),
        )),
    ));

    $response = e2e_http_request('GET', $gameBase . '/index.php?page=b_building&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&modus=add&techid=' . GID_B_LUNAR_BASE . '&planet=' . $attackerPlanet, array(), $cookies);
    $planetLunarQueueCount = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "' AND obj_id=" . GID_B_LUNAR_BASE);
    $cases[] = e2e_finalize_case(array(
        'case' => 'planet_rejects_lunar_building',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($planetLunarQueueCount === 0, 'planet cannot enqueue a lunar base build task', array('queue_count' => $planetLunarQueueCount)),
        )),
    ));

    $secondPlanet = e2e_create_planet_near($attackerId, $home);
    $createdPlanets[] = $secondPlanet;
    $secondMoon = e2e_create_moon_for_planet($secondPlanet, $attackerId);
    $createdMoons[] = $secondMoon;
    e2e_prepare_moon($attackerMoon, $attackerId, array(GID_B_LUNAR_BASE => 1, GID_B_PHALANX => 3, GID_B_JUMP_GATE => 1), array(GID_F_SC => 5), array(GID_RC_DEUTERIUM => 100000));
    e2e_prepare_moon($secondMoon, $attackerId, array(GID_B_LUNAR_BASE => 1, GID_B_JUMP_GATE => 1), array(GID_F_SC => 0), array(GID_RC_DEUTERIUM => 100000));
    $sourceBefore = e2e_planet_snapshot($attackerMoon);
    $targetBefore = e2e_planet_snapshot($secondMoon);
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=sprungtor&session=' . rawurlencode($session) . '&cp=' . $attackerMoon, array(
        'qm' => $attackerMoon,
        'zm' => $secondMoon,
        'c' . GID_F_SC => 2,
    ), $cookies);
    $sourceAfter = e2e_planet_snapshot($attackerMoon);
    $targetAfter = e2e_planet_snapshot($secondMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'moon_jump_gate_moves_ships_and_sets_cooldown',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($sourceBefore !== null && $sourceAfter !== null && (int)$sourceAfter['small_cargo'] === (int)$sourceBefore['small_cargo'] - 2, 'jump gate subtracts ships from source moon', array('before' => $sourceBefore, 'after' => $sourceAfter)),
            e2e_case($targetBefore !== null && $targetAfter !== null && (int)$targetAfter['small_cargo'] === (int)$targetBefore['small_cargo'] + 2, 'jump gate adds ships to target moon', array('before' => $targetBefore, 'after' => $targetAfter)),
            e2e_case($sourceAfter !== null && $targetAfter !== null && (int)$sourceAfter['gate_until'] > time() && (int)$targetAfter['gate_until'] > time(), 'jump gate sets cooldown on both moons', array('source' => $sourceAfter, 'target' => $targetAfter)),
        )),
    ));

    $defenderScanPlanet = e2e_create_planet_near($defenderId, $home);
    $createdPlanets[] = $defenderScanPlanet;
    e2e_prepare_planet($defenderScanPlanet, $defenderId);
    $fixtureFleetId = e2e_dispatch_phalanx_visible_fleet($defenderId, $defenderScanPlanet, $attackerPlanet);
    $moonBeforePhalanx = e2e_planet_snapshot($attackerMoon);
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=phalanx&session=' . rawurlencode($session) . '&cp=' . $attackerMoon . '&spid=' . $defenderScanPlanet, array(), $cookies);
    $moonAfterPhalanx = e2e_planet_snapshot($attackerMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'moon_phalanx_scan_costs_deuterium_and_renders_fleet_event',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($fixtureFleetId > 0, 'fixture fleet exists for phalanx visibility', array('fleet_id' => $fixtureFleetId)),
            e2e_case($moonBeforePhalanx !== null && $moonAfterPhalanx !== null && (int)$moonAfterPhalanx['deuterium'] === (int)$moonBeforePhalanx['deuterium'] - 5000, 'phalanx scan subtracts deuterium cost from moon', array('before' => $moonBeforePhalanx, 'after' => $moonAfterPhalanx)),
            e2e_case(stripos($response['body'], 'phalanx_fleet') !== false, 'phalanx report renders the active fleet event'),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'trader_moon_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    $planetIds = array_filter(array_merge(array($attackerPlanet, $defenderPlanet), $createdPlanets, $createdMoons), fn($id) => $id > 0);
    $userIds = array_filter(array($attackerId, $defenderId), fn($id) => $id > 0);
    if (!empty($userIds) && !empty($planetIds)) {
        e2e_cleanup_fleets($userIds, $planetIds);
    }
    foreach (array_reverse($createdMoons) as $moonId) {
        if ($moonId > 0 && LoadPlanetById($moonId) !== null) {
            DestroyPlanet($moonId);
        }
    }
    foreach (array_reverse($createdPlanets) as $planetId) {
        if ($planetId > 0 && LoadPlanetById($planetId) !== null) {
            DestroyPlanet($planetId);
        }
    }
    if ($attackerId > 0 && $attackerPlanet > 0) {
        e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    }
    if ($defenderId > 0 && $defenderPlanet > 0) {
        e2e_reset_user_and_planet($defenderId, $defenderPlanet);
    }
}

echo json_encode(array(
    'case_group' => 'http_trader_moon',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
