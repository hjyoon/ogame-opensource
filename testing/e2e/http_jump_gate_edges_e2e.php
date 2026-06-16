<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_jump_gate_edges_e2e.php';
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
loca_add('infos', 'en');
loca_add('jumpgate', 'en');
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
        "`" . GID_F_SC . "`=10, `" . GID_F_LC . "`=5, `" . GID_F_LF . "`=5, `" . GID_F_PROBE . "`=5, `" . GID_F_SAT . "`=0, " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=0, maxfields=200, type=" . PTYP_PLANET . ", owner_id={$ownerId}, gate_until=0, lastpeek={$now} " .
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

function e2e_set_gate_until(int $moonId, int $until): void
{
    global $db_prefix;
    dbquery("UPDATE {$db_prefix}planets SET gate_until={$until} WHERE planet_id={$moonId}");
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

function e2e_planet_snapshot(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, type, gate_until, " .
        "`" . GID_B_JUMP_GATE . "` AS jump_gate, " .
        "`" . GID_F_SC . "` AS small_cargo, `" . GID_F_LF . "` AS light_fighter, `" . GID_F_SAT . "` AS solar_sat " .
        "FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1"
    );
}

function e2e_jump_payload(int $sourceId, int $targetId, array $ships): array
{
    $payload = array('qm' => $sourceId, 'zm' => $targetId);
    foreach ($ships as $gid => $amount) {
        $payload['c' . $gid] = $amount;
    }
    return $payload;
}

function e2e_jump_request(string $gameBase, string $session, int $currentPlanet, array $payload, array &$cookies): array
{
    return e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=sprungtor&session=' . rawurlencode($session) . '&cp=' . $currentPlanet,
        $payload,
        $cookies
    );
}

function e2e_same_moon_state(?array $before, ?array $after): bool
{
    if ($before === null || $after === null) {
        return false;
    }
    foreach (array('small_cargo', 'light_fighter', 'solar_sat', 'gate_until') as $key) {
        if ((int)$before[$key] !== (int)$after[$key]) {
            return false;
        }
    }
    return true;
}

function e2e_prepare_ready_pair(int $sourceMoon, int $targetMoon, int $attackerId): void
{
    e2e_prepare_moon(
        $sourceMoon,
        $attackerId,
        array(GID_B_LUNAR_BASE => 1, GID_B_JUMP_GATE => 1),
        array(GID_F_SC => 5, GID_F_LF => 3, GID_F_SAT => 4)
    );
    e2e_prepare_moon(
        $targetMoon,
        $attackerId,
        array(GID_B_LUNAR_BASE => 1, GID_B_JUMP_GATE => 1),
        array(GID_F_SC => 1, GID_F_LF => 1, GID_F_SAT => 0)
    );
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

    $auth = e2e_prepare_session($attackerId, 'jump-gate-edges-attacker');
    $cookies = $auth['cookies'];
    $session = $auth['session'];

    $home = LoadPlanetById($attackerPlanet);
    if ($home === null) {
        throw new RuntimeException('Attacker home planet is missing.');
    }

    $sourceMoon = e2e_create_moon_for_planet($attackerPlanet, $attackerId);
    $createdMoons[] = $sourceMoon;
    $targetPlanet = e2e_create_planet_near($attackerId, $home);
    $createdPlanets[] = $targetPlanet;
    $targetMoon = e2e_create_moon_for_planet($targetPlanet, $attackerId);
    $createdMoons[] = $targetMoon;
    $noGatePlanet = e2e_create_planet_near($attackerId, $home);
    $createdPlanets[] = $noGatePlanet;
    $noGateMoon = e2e_create_moon_for_planet($noGatePlanet, $attackerId);
    $createdMoons[] = $noGateMoon;
    $coolingPlanet = e2e_create_planet_near($attackerId, $home);
    $createdPlanets[] = $coolingPlanet;
    $coolingMoon = e2e_create_moon_for_planet($coolingPlanet, $attackerId);
    $createdMoons[] = $coolingMoon;
    $foreignMoon = e2e_create_moon_for_planet($defenderPlanet, $defenderId);
    $createdMoons[] = $foreignMoon;

    e2e_prepare_ready_pair($sourceMoon, $targetMoon, $attackerId);
    e2e_prepare_moon($noGateMoon, $attackerId, array(GID_B_LUNAR_BASE => 1), array(GID_F_SC => 5));
    e2e_prepare_moon($coolingMoon, $attackerId, array(GID_B_LUNAR_BASE => 1, GID_B_JUMP_GATE => 1), array(GID_F_SC => 5));
    e2e_set_gate_until($coolingMoon, time() + 3600);
    e2e_prepare_moon($foreignMoon, $defenderId, array(GID_B_LUNAR_BASE => 1, GID_B_JUMP_GATE => 1), array(GID_F_SC => 0));

    $response = e2e_http_request('GET', $gameBase . '/index.php?page=infos&session=' . rawurlencode($session) . '&cp=' . $sourceMoon . '&gid=' . GID_B_JUMP_GATE, array(), $cookies);
    $body = $response['body'];
    $cases[] = e2e_finalize_case(array(
        'case' => 'jump_gate_info_page_filters_ready_owned_target_moons',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($body, 'page=sprungtor') !== false, 'ready source moon renders the jump form'),
            e2e_case(strpos($body, 'name="qm" value="' . $sourceMoon . '"') !== false, 'form uses current moon as source'),
            e2e_case(strpos($body, '<option value="' . $targetMoon . '">') !== false, 'ready owned target moon appears as selectable target'),
            e2e_case(strpos($body, '<option value="' . $sourceMoon . '">') === false, 'current moon is not selectable as its own target'),
            e2e_case(strpos($body, '<option value="' . $noGateMoon . '">') === false, 'owned moon without a jump gate is filtered out'),
            e2e_case(strpos($body, '<option value="' . $coolingMoon . '">') === false, 'owned moon with active cooldown is filtered out'),
            e2e_case(strpos($body, '<option value="' . $foreignMoon . '">') === false, 'foreign moon is not selectable'),
        )),
    ));

    e2e_prepare_ready_pair($sourceMoon, $targetMoon, $attackerId);
    $sourceBefore = e2e_planet_snapshot($sourceMoon);
    $targetBefore = e2e_planet_snapshot($targetMoon);
    $response = e2e_jump_request($gameBase, $session, $attackerPlanet, e2e_jump_payload($attackerPlanet, $targetMoon, array(GID_F_SC => 1)), $cookies);
    $sourceAfter = e2e_planet_snapshot($sourceMoon);
    $targetAfter = e2e_planet_snapshot($targetMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'jump_gate_rejects_planet_source',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($response['body'], 'no source moon selected') !== false, 'planet source is rejected with the source moon error'),
            e2e_case(e2e_same_moon_state($sourceBefore, $sourceAfter) && e2e_same_moon_state($targetBefore, $targetAfter), 'planet-source rejection leaves moon ships and cooldown unchanged', array('source_before' => $sourceBefore, 'source_after' => $sourceAfter, 'target_before' => $targetBefore, 'target_after' => $targetAfter)),
        )),
    ));

    e2e_prepare_ready_pair($sourceMoon, $targetMoon, $attackerId);
    $sourceBefore = e2e_planet_snapshot($sourceMoon);
    $targetBefore = e2e_planet_snapshot($targetMoon);
    $response = e2e_jump_request($gameBase, $session, $sourceMoon, e2e_jump_payload($sourceMoon, $attackerPlanet, array(GID_F_SC => 1)), $cookies);
    $sourceAfter = e2e_planet_snapshot($sourceMoon);
    $targetAfter = e2e_planet_snapshot($targetMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'jump_gate_rejects_planet_target',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($response['body'], 'no target moon selected') !== false, 'planet target is rejected with the target moon error'),
            e2e_case(e2e_same_moon_state($sourceBefore, $sourceAfter) && e2e_same_moon_state($targetBefore, $targetAfter), 'planet-target rejection leaves moon ships and cooldown unchanged', array('source_before' => $sourceBefore, 'source_after' => $sourceAfter, 'target_before' => $targetBefore, 'target_after' => $targetAfter)),
        )),
    ));

    e2e_prepare_ready_pair($sourceMoon, $targetMoon, $attackerId);
    e2e_prepare_moon($noGateMoon, $attackerId, array(GID_B_LUNAR_BASE => 1), array(GID_F_SC => 5));
    $noGateBefore = e2e_planet_snapshot($noGateMoon);
    $targetBefore = e2e_planet_snapshot($targetMoon);
    $response = e2e_jump_request($gameBase, $session, $noGateMoon, e2e_jump_payload($noGateMoon, $targetMoon, array(GID_F_SC => 1)), $cookies);
    $noGateAfter = e2e_planet_snapshot($noGateMoon);
    $targetAfter = e2e_planet_snapshot($targetMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'jump_gate_rejects_missing_source_gate',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($response['body'], 'no jump gate found at source moon') !== false, 'missing source gate is rejected'),
            e2e_case(e2e_same_moon_state($noGateBefore, $noGateAfter) && e2e_same_moon_state($targetBefore, $targetAfter), 'missing-source-gate rejection leaves ships and cooldown unchanged', array('source_before' => $noGateBefore, 'source_after' => $noGateAfter, 'target_before' => $targetBefore, 'target_after' => $targetAfter)),
        )),
    ));

    e2e_prepare_ready_pair($sourceMoon, $targetMoon, $attackerId);
    e2e_prepare_moon($noGateMoon, $attackerId, array(GID_B_LUNAR_BASE => 1), array(GID_F_SC => 0));
    $sourceBefore = e2e_planet_snapshot($sourceMoon);
    $noGateBefore = e2e_planet_snapshot($noGateMoon);
    $response = e2e_jump_request($gameBase, $session, $sourceMoon, e2e_jump_payload($sourceMoon, $noGateMoon, array(GID_F_SC => 1)), $cookies);
    $sourceAfter = e2e_planet_snapshot($sourceMoon);
    $noGateAfter = e2e_planet_snapshot($noGateMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'jump_gate_rejects_missing_target_gate',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($response['body'], 'no jump gate found at the target moon') !== false, 'missing target gate is rejected'),
            e2e_case(e2e_same_moon_state($sourceBefore, $sourceAfter) && e2e_same_moon_state($noGateBefore, $noGateAfter), 'missing-target-gate rejection leaves ships and cooldown unchanged', array('source_before' => $sourceBefore, 'source_after' => $sourceAfter, 'target_before' => $noGateBefore, 'target_after' => $noGateAfter)),
        )),
    ));

    e2e_prepare_ready_pair($sourceMoon, $targetMoon, $attackerId);
    e2e_prepare_moon($foreignMoon, $defenderId, array(GID_B_LUNAR_BASE => 1, GID_B_JUMP_GATE => 1), array(GID_F_SC => 0));
    $sourceBefore = e2e_planet_snapshot($sourceMoon);
    $foreignBefore = e2e_planet_snapshot($foreignMoon);
    $response = e2e_jump_request($gameBase, $session, $sourceMoon, e2e_jump_payload($sourceMoon, $foreignMoon, array(GID_F_SC => 1)), $cookies);
    $sourceAfter = e2e_planet_snapshot($sourceMoon);
    $foreignAfter = e2e_planet_snapshot($foreignMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'jump_gate_rejects_foreign_target_moon',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($response['body'], "either the source moon or target moon doesn't belong to you") !== false, 'foreign moon is rejected with the ownership error'),
            e2e_case(e2e_same_moon_state($sourceBefore, $sourceAfter) && e2e_same_moon_state($foreignBefore, $foreignAfter), 'foreign-target rejection leaves ships and cooldown unchanged', array('source_before' => $sourceBefore, 'source_after' => $sourceAfter, 'target_before' => $foreignBefore, 'target_after' => $foreignAfter)),
        )),
    ));

    e2e_prepare_ready_pair($sourceMoon, $targetMoon, $attackerId);
    $sourceBefore = e2e_planet_snapshot($sourceMoon);
    $targetBefore = e2e_planet_snapshot($targetMoon);
    $response = e2e_jump_request($gameBase, $session, $sourceMoon, e2e_jump_payload($sourceMoon, $targetMoon, array()), $cookies);
    $sourceAfter = e2e_planet_snapshot($sourceMoon);
    $targetAfter = e2e_planet_snapshot($targetMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'jump_gate_rejects_no_selected_ships',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($response['body'], 'no ships selected') !== false, 'empty ship selection is rejected'),
            e2e_case(e2e_same_moon_state($sourceBefore, $sourceAfter) && e2e_same_moon_state($targetBefore, $targetAfter), 'empty-selection rejection leaves ships and cooldown unchanged', array('source_before' => $sourceBefore, 'source_after' => $sourceAfter, 'target_before' => $targetBefore, 'target_after' => $targetAfter)),
        )),
    ));

    e2e_prepare_ready_pair($sourceMoon, $targetMoon, $attackerId);
    $sourceBefore = e2e_planet_snapshot($sourceMoon);
    $targetBefore = e2e_planet_snapshot($targetMoon);
    $response = e2e_jump_request($gameBase, $session, $sourceMoon, e2e_jump_payload($sourceMoon, $targetMoon, array(GID_F_SAT => 3)), $cookies);
    $sourceAfter = e2e_planet_snapshot($sourceMoon);
    $targetAfter = e2e_planet_snapshot($targetMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'jump_gate_rejects_satellite_only_payload',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($response['body'], 'no ships selected') !== false, 'solar-satellite-only payload does not count as selected ships'),
            e2e_case(e2e_same_moon_state($sourceBefore, $sourceAfter) && e2e_same_moon_state($targetBefore, $targetAfter), 'solar satellites are not moved or used to warm up gates', array('source_before' => $sourceBefore, 'source_after' => $sourceAfter, 'target_before' => $targetBefore, 'target_after' => $targetAfter)),
        )),
    ));

    e2e_prepare_ready_pair($sourceMoon, $targetMoon, $attackerId);
    $sourceBefore = e2e_planet_snapshot($sourceMoon);
    $targetBefore = e2e_planet_snapshot($targetMoon);
    $response = e2e_jump_request($gameBase, $session, $sourceMoon, e2e_jump_payload($sourceMoon, $targetMoon, array(GID_F_SC => 999)), $cookies);
    $sourceAfter = e2e_planet_snapshot($sourceMoon);
    $targetAfter = e2e_planet_snapshot($targetMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'jump_gate_rejects_not_enough_ships',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($response['body'], 'not enough ships available') !== false, 'oversized ship amount is rejected'),
            e2e_case(e2e_same_moon_state($sourceBefore, $sourceAfter) && e2e_same_moon_state($targetBefore, $targetAfter), 'not-enough-ships rejection leaves ships and cooldown unchanged', array('source_before' => $sourceBefore, 'source_after' => $sourceAfter, 'target_before' => $targetBefore, 'target_after' => $targetAfter)),
        )),
    ));

    e2e_prepare_ready_pair($sourceMoon, $targetMoon, $attackerId);
    $sourceBefore = e2e_planet_snapshot($sourceMoon);
    $response = e2e_jump_request($gameBase, $session, $sourceMoon, e2e_jump_payload($sourceMoon, $sourceMoon, array(GID_F_SC => 1)), $cookies);
    $sourceAfter = e2e_planet_snapshot($sourceMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'jump_gate_rejects_same_source_and_target_moon',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($response['body'], 'no target moon selected') !== false, 'same source and target moon is rejected'),
            e2e_case(e2e_same_moon_state($sourceBefore, $sourceAfter), 'same-moon rejection leaves ships and cooldown unchanged', array('source_before' => $sourceBefore, 'source_after' => $sourceAfter)),
        )),
    ));

    e2e_prepare_ready_pair($sourceMoon, $targetMoon, $attackerId);
    e2e_set_gate_until($sourceMoon, time() + 3600);
    e2e_set_gate_until($targetMoon, time() + 3600);
    $sourceBefore = e2e_planet_snapshot($sourceMoon);
    $targetBefore = e2e_planet_snapshot($targetMoon);
    $response = e2e_jump_request($gameBase, $session, $sourceMoon, e2e_jump_payload($sourceMoon, $targetMoon, array(GID_F_SC => 1)), $cookies);
    $sourceAfter = e2e_planet_snapshot($sourceMoon);
    $targetAfter = e2e_planet_snapshot($targetMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'jump_gate_rejects_direct_post_during_cooldown',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($response['body'], 'The Jump Gate is in recharge mode') !== false, 'direct POST during cooldown is rejected'),
            e2e_case(e2e_same_moon_state($sourceBefore, $sourceAfter) && e2e_same_moon_state($targetBefore, $targetAfter), 'cooldown rejection leaves ships and cooldown unchanged', array('source_before' => $sourceBefore, 'source_after' => $sourceAfter, 'target_before' => $targetBefore, 'target_after' => $targetAfter)),
        )),
    ));

    e2e_prepare_ready_pair($sourceMoon, $targetMoon, $attackerId);
    $sourceBefore = e2e_planet_snapshot($sourceMoon);
    $targetBefore = e2e_planet_snapshot($targetMoon);
    $response = e2e_jump_request($gameBase, $session, $sourceMoon, e2e_jump_payload($sourceMoon, $targetMoon, array(GID_F_SC => 2, GID_F_LF => 1, GID_F_SAT => 4)), $cookies);
    $sourceAfter = e2e_planet_snapshot($sourceMoon);
    $targetAfter = e2e_planet_snapshot($targetMoon);
    $cases[] = e2e_finalize_case(array(
        'case' => 'jump_gate_moves_multiple_ship_types_and_ignores_satellites',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(in_array($response['status'], array(301, 302, 303), true) && strpos($response['location'], 'page=infos') !== false && strpos($response['location'], 'cp=' . $targetMoon) !== false && strpos($response['location'], 'gid=' . GID_B_JUMP_GATE) !== false, 'successful jump redirects to the target moon jump-gate info page', array('status' => $response['status'], 'location' => $response['location'])),
            e2e_case($sourceBefore !== null && $sourceAfter !== null && (int)$sourceAfter['small_cargo'] === (int)$sourceBefore['small_cargo'] - 2 && (int)$sourceAfter['light_fighter'] === (int)$sourceBefore['light_fighter'] - 1, 'jump subtracts selected mobile ships from source moon', array('before' => $sourceBefore, 'after' => $sourceAfter)),
            e2e_case($targetBefore !== null && $targetAfter !== null && (int)$targetAfter['small_cargo'] === (int)$targetBefore['small_cargo'] + 2 && (int)$targetAfter['light_fighter'] === (int)$targetBefore['light_fighter'] + 1, 'jump adds selected mobile ships to target moon', array('before' => $targetBefore, 'after' => $targetAfter)),
            e2e_case($sourceAfter !== null && $targetAfter !== null && (int)$sourceAfter['solar_sat'] === (int)$sourceBefore['solar_sat'] && (int)$targetAfter['solar_sat'] === (int)$targetBefore['solar_sat'], 'solar satellites remain on their original moons even when posted'),
            e2e_case($sourceAfter !== null && $targetAfter !== null && (int)$sourceAfter['gate_until'] > time() && (int)$targetAfter['gate_until'] > time(), 'successful jump sets cooldown on both gates', array('source' => $sourceAfter, 'target' => $targetAfter)),
        )),
    ));

    $response = e2e_http_request('GET', $gameBase . '/index.php?page=infos&session=' . rawurlencode($session) . '&cp=' . $sourceMoon . '&gid=' . GID_B_JUMP_GATE, array(), $cookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'jump_gate_info_page_shows_cooldown_after_success',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($response['body'], 'The Jump Gate is in recharge mode') !== false, 'cooling source moon renders recharge message'),
            e2e_case(strpos($response['body'], 'page=sprungtor') === false, 'cooling source moon does not render another jump form'),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'jump_gate_edges_exception',
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
    'case_group' => 'http_jump_gate_edges',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
