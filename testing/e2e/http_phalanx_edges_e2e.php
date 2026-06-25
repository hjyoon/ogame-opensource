<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_phalanx_edges_e2e.php';
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
loca_add('events', 'en');
loca_add('fleet', 'en');
loca_add('phalanx', 'en');
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

function e2e_prepare_moon(int $moonId, int $ownerId, int $phalanxLevel, int $deuterium): void
{
    global $db_prefix, $buildmap, $fleetmap;
    $now = time();

    $buildingAssignments = array();
    foreach ($buildmap as $gid) {
        $level = $gid === GID_B_LUNAR_BASE ? 1 : 0;
        if ($gid === GID_B_PHALANX) {
            $level = $phalanxLevel;
        }
        $buildingAssignments[] = "`{$gid}`={$level}";
    }
    $shipAssignments = array();
    foreach ($fleetmap as $gid) {
        $shipAssignments[] = "`{$gid}`=0";
    }

    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        implode(',', $buildingAssignments) . ", " .
        implode(',', $shipAssignments) . ", " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`={$deuterium}, " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=2, maxfields=4, type=" . PTYP_MOON . ", owner_id={$ownerId}, lastpeek={$now} " .
        "WHERE planet_id={$moonId}"
    );
}

function e2e_find_empty_position(array $near, bool $sameSystem): array
{
    global $GlobalUni;

    $g = (int)$near['g'];
    $originSystem = (int)$near['s'];
    $systems = $sameSystem ? array($originSystem) : range(1, (int)$GlobalUni['systems']);
    foreach ($systems as $system) {
        if (!$sameSystem && $system === $originSystem) {
            continue;
        }
        for ($p = 1; $p <= 15; $p++) {
            if ($sameSystem && $p === (int)$near['p']) {
                continue;
            }
            if (!HasPlanet($g, $system, $p)) {
                return array($g, $system, $p);
            }
        }
    }

    throw new RuntimeException($sameSystem ? 'No empty same-system position found.' : 'No empty out-of-range position found.');
}

function e2e_create_planet_at(int $ownerId, array $coords): int
{
    [$g, $s, $p] = $coords;
    $planetId = CreatePlanet($g, $s, $p, $ownerId, 1, 0, 0, time());
    if ($planetId <= 0) {
        throw new RuntimeException('Failed to create phalanx target planet.');
    }
    e2e_prepare_planet($planetId, $ownerId);
    return $planetId;
}

function e2e_create_moon_for_planet(int $planetId, int $ownerId): int
{
    $existingMoon = PlanetHasMoon($planetId);
    if ($existingMoon > 0) {
        DestroyPlanet($existingMoon);
    }
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

function e2e_planet_snapshot(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, type, g, s, p, lastpeek, " .
        "`" . GID_RC_DEUTERIUM . "` AS deuterium, `" . GID_B_PHALANX . "` AS phalanx, `" . GID_F_SC . "` AS small_cargo " .
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

function e2e_phalanx_request(string $gameBase, string $session, int $sourcePlanetId, int $targetPlanetId, array &$cookies): array
{
    return e2e_http_request(
        'GET',
        $gameBase . '/index.php?page=phalanx&session=' . rawurlencode($session) . '&cp=' . $sourcePlanetId . '&spid=' . $targetPlanetId,
        array(),
        $cookies
    );
}

function e2e_same_deuterium(?array $before, ?array $after): bool
{
    return $before !== null && $after !== null && (int)$before['deuterium'] === (int)$after['deuterium'];
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

    $auth = e2e_prepare_session($attackerId, 'phalanx-edges-attacker');
    $cookies = $auth['cookies'];
    $session = $auth['session'];

    $home = LoadPlanetById($attackerPlanet);
    if ($home === null) {
        throw new RuntimeException('Attacker home planet is missing.');
    }

    $nearTargetPlanet = e2e_create_planet_at($defenderId, e2e_find_empty_position($home, true));
    $createdPlanets[] = $nearTargetPlanet;
    $farTargetPlanet = e2e_create_planet_at($defenderId, e2e_find_empty_position($home, false));
    $createdPlanets[] = $farTargetPlanet;
    $attackerMoon = e2e_create_moon_for_planet($attackerPlanet, $attackerId);
    $createdMoons[] = $attackerMoon;

    $before = e2e_planet_snapshot($attackerPlanet);
    $response = e2e_phalanx_request($gameBase, $session, $attackerPlanet, $nearTargetPlanet, $cookies);
    $after = e2e_planet_snapshot($attackerPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'phalanx_rejects_source_without_sensor_array',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($response['body'], 'No cheating!') !== false, 'planet source without phalanx renders missing-sensor error'),
            e2e_case(e2e_same_deuterium($before, $after), 'missing sensor rejection leaves source deuterium unchanged', array('before' => $before, 'after' => $after)),
        )),
    ));

    e2e_prepare_moon($attackerMoon, $attackerId, 3, 4999);
    $before = e2e_planet_snapshot($attackerMoon);
    $response = e2e_phalanx_request($gameBase, $session, $attackerMoon, $nearTargetPlanet, $cookies);
    $after = e2e_planet_snapshot($attackerMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'phalanx_rejects_insufficient_deuterium',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($response['body'], 'Not enough deuterium!') !== false, 'insufficient deuterium renders configured error'),
            e2e_case(e2e_same_deuterium($before, $after), 'insufficient deuterium rejection does not alter moon resources', array('before' => $before, 'after' => $after)),
        )),
    ));

    e2e_prepare_moon($attackerMoon, $attackerId, 3, 20000);
    $before = e2e_planet_snapshot($attackerMoon);
    $response = e2e_phalanx_request($gameBase, $session, $attackerMoon, $attackerPlanet, $cookies);
    $after = e2e_planet_snapshot($attackerMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'phalanx_rejects_own_planet_target',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($response['body'], 'attempting to manipulate a phalanx') !== false, 'own target renders phalanx manipulation warning'),
            e2e_case(e2e_same_deuterium($before, $after), 'own target rejection does not spend deuterium', array('before' => $before, 'after' => $after)),
        )),
    ));

    e2e_prepare_moon($attackerMoon, $attackerId, 1, 20000);
    $before = e2e_planet_snapshot($attackerMoon);
    $response = e2e_phalanx_request($gameBase, $session, $attackerMoon, $farTargetPlanet, $cookies);
    $after = e2e_planet_snapshot($attackerMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'phalanx_rejects_out_of_range_target',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($response['body'], 'attempting to manipulate a phalanx') !== false, 'out-of-range target renders phalanx manipulation warning'),
            e2e_case(e2e_same_deuterium($before, $after), 'out-of-range rejection does not spend deuterium', array('before' => $before, 'after' => $after)),
        )),
    ));

    e2e_prepare_moon($attackerMoon, $attackerId, 3, 20000);
    e2e_prepare_planet($nearTargetPlanet, $defenderId);
    $fixtureFleetId = e2e_dispatch_phalanx_visible_fleet($defenderId, $nearTargetPlanet, $attackerPlanet);
    $before = e2e_planet_snapshot($attackerMoon);
    $response = e2e_phalanx_request($gameBase, $session, $attackerMoon, $nearTargetPlanet, $cookies);
    $after = e2e_planet_snapshot($attackerMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'phalanx_success_spends_deuterium_and_renders_event',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($fixtureFleetId > 0, 'fixture fleet exists for phalanx report', array('fleet_id' => $fixtureFleetId)),
            e2e_case($before !== null && $after !== null && (int)$after['deuterium'] === (int)$before['deuterium'] - 5000, 'successful scan spends exactly 5000 deuterium', array('before' => $before, 'after' => $after)),
            e2e_case(strpos($response['body'], 'Sensor report from the moon on the coordinates') !== false, 'successful scan renders report heading'),
            e2e_case(stripos($response['body'], 'phalanx_fleet') !== false, 'successful scan renders active fleet event span'),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'phalanx_edges_exception',
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
    'case_group' => 'http_phalanx_edges',
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . "\n";
