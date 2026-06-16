<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_concurrency_race_e2e.php';
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
loca_add('technames', 'en');

function e2e_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
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

function e2e_response_check(array $response, string $label): array
{
    $body = $response['body'];
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read|foreach|filectime)|Notice:\s+(Undefined|Trying)/i', $body) === 1;
    return array(
        e2e_case(in_array($response['status'], array(200, 301, 302, 303), true), $label . ' returns an accepted status', array('status' => $response['status'], 'error' => $response['error'])),
        e2e_case(!$hasError, $label . ' body has no PHP error marker'),
    );
}

function e2e_multi_http_requests(array $requests): array
{
    $mh = curl_multi_init();
    $handles = array();

    foreach ($requests as $i => $request) {
        $ch = curl_init();
        $headers = array('User-Agent: ogame-e2e');
        if (!empty($request['cookies'])) {
            $pairs = array();
            foreach ($request['cookies'] as $name => $value) {
                $pairs[] = $name . '=' . $value;
            }
            $headers[] = 'Cookie: ' . implode('; ', $pairs);
        }

        curl_setopt($ch, CURLOPT_URL, $request['url']);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        curl_setopt($ch, CURLOPT_HEADER, true);
        curl_setopt($ch, CURLOPT_HTTPHEADER, $headers);
        curl_setopt($ch, CURLOPT_FOLLOWLOCATION, false);
        curl_setopt($ch, CURLOPT_CONNECTTIMEOUT, 5);
        curl_setopt($ch, CURLOPT_TIMEOUT, 20);
        curl_setopt($ch, CURLOPT_FORBID_REUSE, true);
        curl_setopt($ch, CURLOPT_FRESH_CONNECT, true);

        if (($request['method'] ?? 'GET') === 'POST') {
            curl_setopt($ch, CURLOPT_POST, true);
            curl_setopt($ch, CURLOPT_POSTFIELDS, http_build_query($request['data'] ?? array()));
        }

        curl_multi_add_handle($mh, $ch);
        $handles[$i] = $ch;
    }

    $running = null;
    do {
        do {
            $status = curl_multi_exec($mh, $running);
        } while ($status === CURLM_CALL_MULTI_PERFORM);

        if ($running) {
            curl_multi_select($mh, 0.05);
        }
    } while ($running && $status === CURLM_OK);

    $responses = array();
    foreach ($handles as $i => $ch) {
        $raw = curl_multi_getcontent($ch);
        $headerSize = curl_getinfo($ch, CURLINFO_HEADER_SIZE);
        $header = substr($raw, 0, $headerSize);
        $body = substr($raw, $headerSize);
        $location = '';
        foreach (preg_split('/\r?\n/', $header) as $line) {
            if (stripos($line, 'Location:') === 0) {
                $location = trim(substr($line, 9));
            }
        }
        $responses[$i] = array(
            'status' => (int)curl_getinfo($ch, CURLINFO_HTTP_CODE),
            'location' => $location,
            'body' => $body,
            'error' => curl_error($ch),
        );
        curl_multi_remove_handle($mh, $ch);
        curl_close($ch);
    }
    curl_multi_close($mh);
    ksort($responses);
    return array_values($responses);
}

function e2e_parallel_requests(string $method, string $url, array $data, array $cookies, int $count): array
{
    $requests = array();
    for ($i = 0; $i < $count; $i++) {
        $requests[] = array(
            'method' => $method,
            'url' => $url,
            'data' => $data,
            'cookies' => $cookies,
        );
    }
    return e2e_multi_http_requests($requests);
}

function e2e_prepare_session(int $userId, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', admin=0, validated=1, deact_ip=1, " .
        "vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, " .
        "disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, com_until=0, adm_until=0, " .
        "score1=10000, score2=0, score3=0, place1=1, place2=1, place3=1 WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
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
    foreach ($planetIds as $planetId) {
        @unlink('temp/fleetlock_' . (int)$planetId);
    }
}

function e2e_reset_user_and_planet(int $userId, int $planetId, string $name): void
{
    global $db_prefix, $fleetmap, $defmap, $buildmap, $resmap;

    e2e_cleanup_fleets(array($userId), array($planetId));
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$userId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_UNBAN . "','" . QTYP_ALLOW_ATTACKS . "','" . QTYP_DEBUG . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$userId} OR planet_id={$planetId}");

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=10";
    }
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", " .
        "admin=0, ally_id=0, allyrank=0, validated=1, deact_ip=1, vacation=0, vacation_until=0, " .
        "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, " .
        "lang='en', skin='/evolution/', useskin=1, com_until=0, adm_until=0, dm=0, dmfree=0, " .
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
        "`" . GID_B_SOLAR . "`=20, `" . GID_B_SHIPYARD . "`=12, `" . GID_B_ROBOTS . "`=10, `" . GID_B_RES_LAB . "`=12, " .
        "`" . GID_F_SC . "`=10, `" . GID_F_LF . "`=10, `" . GID_F_PROBE . "`=10, " .
        "prod1=1, prod2=1, prod3=1, prod4=1, prod12=1, prod212=1, fields=0, maxfields=300, lastpeek={$now}, lastakt={$now} " .
        "WHERE planet_id={$planetId}"
    );
    InvalidateUserCache();
}

function e2e_planet_snapshot(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, type, fields, " .
        "`" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deuterium, " .
        "`" . GID_B_METAL_MINE . "` AS metal_mine, `" . GID_F_SC . "` AS small_cargo " .
        "FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1"
    );
}

function e2e_user_snapshot(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT player_id, `" . GID_R_ESPIONAGE . "` AS espionage FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1");
}

function e2e_queue_sum(string $type, int $ownerId, int $objId = 0, int $subId = 0): array
{
    global $db_prefix;

    $where = "owner_id={$ownerId} AND type='" . e2e_sql_escape($type) . "'";
    if ($objId > 0) {
        $where .= " AND obj_id={$objId}";
    }
    if ($subId > 0) {
        $where .= " AND sub_id={$subId}";
    }
    $row = e2e_one_row("SELECT COUNT(*) AS cnt, COALESCE(SUM(level),0) AS total_level FROM {$db_prefix}queue WHERE {$where}");
    return $row ?? array('cnt' => 0, 'total_level' => 0);
}

function e2e_fleet_payload(array $origin, array $target, int $order, array $ships, array $resources = array()): array
{
    global $fleetmap, $transportableResources, $GlobalUni;

    $data = array(
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
        $data['ship' . $gid] = (string)($ships[$gid] ?? 0);
    }
    foreach ($transportableResources as $i => $rc) {
        $data['resource' . ($i + 1)] = (string)($resources[$rc] ?? 0);
    }
    return $data;
}

function e2e_all_responses_ok(array $responses, string $label): array
{
    $checks = array(e2e_case(count($responses) > 1, $label . ' produced multiple parallel responses', array('count' => count($responses))));
    foreach ($responses as $i => $response) {
        $checks = array_merge($checks, e2e_response_check($response, $label . ' response ' . ($i + 1)));
    }
    return $checks;
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
    if (!function_exists('curl_multi_init')) {
        throw new RuntimeException('curl_multi_init is required for concurrency E2E tests.');
    }

    $auth = e2e_prepare_session($attackerId, 'concurrency-attacker');
    $session = $auth['session'];
    $cookies = $auth['cookies'];

    e2e_reset_user_and_planet($attackerId, $attackerPlanet, 'Concurrency Origin');
    e2e_reset_user_and_planet($defenderId, $defenderPlanet, 'Concurrency Target');
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));

    $cost = TechPrice(GID_B_METAL_MINE, 1);
    dbquery(
        "UPDATE {$db_prefix}planets SET `" . GID_RC_METAL . "`=" . (int)$cost[GID_RC_METAL] . ", `" .
        GID_RC_CRYSTAL . "`=" . (int)$cost[GID_RC_CRYSTAL] . ", `" . GID_RC_DEUTERIUM . "`=" . (int)($cost[GID_RC_DEUTERIUM] ?? 0) .
        " WHERE planet_id={$attackerPlanet}"
    );
    $buildUrl = $gameBase . '/index.php?page=b_building&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&modus=add&techid=' . GID_B_METAL_MINE . '&planet=' . $attackerPlanet;
    $buildResponses = e2e_parallel_requests('GET', $buildUrl, array(), $cookies, 4);
    $buildQueue = e2e_queue_sum(QTYP_BUILD, $attackerId, GID_B_METAL_MINE);
    $buildRows = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}buildqueue WHERE owner_id={$attackerId} AND planet_id={$attackerPlanet} AND tech_id=" . GID_B_METAL_MINE);
    $planetAfterBuildRace = e2e_planet_snapshot($attackerPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'parallel_build_enqueue_does_not_duplicate_queue_or_overspend',
        'checks' => array_merge(e2e_all_responses_ok($buildResponses, 'parallel build enqueue'), array(
            e2e_case((int)$buildQueue['cnt'] === 1 && (int)$buildQueue['total_level'] === 1, 'parallel build requests leave exactly one active build queue task', $buildQueue),
            e2e_case($buildRows === 1, 'parallel build requests leave exactly one buildqueue row', array('buildqueue_rows' => $buildRows)),
            e2e_case($planetAfterBuildRace !== null && (int)$planetAfterBuildRace['metal'] >= 0 && (int)$planetAfterBuildRace['crystal'] >= 0 && (int)$planetAfterBuildRace['deuterium'] >= 0, 'parallel build requests do not overspend resources below zero', $planetAfterBuildRace ?? array()),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet, 'Concurrency Origin');
    dbquery("UPDATE {$db_prefix}users SET `" . GID_R_ESPIONAGE . "`=0 WHERE player_id={$attackerId}");
    InvalidateUserCache();
    $researchCost = TechPrice(GID_R_ESPIONAGE, 1);
    dbquery(
        "UPDATE {$db_prefix}planets SET `" . GID_RC_METAL . "`=" . (int)($researchCost[GID_RC_METAL] ?? 0) . ", `" .
        GID_RC_CRYSTAL . "`=" . (int)($researchCost[GID_RC_CRYSTAL] ?? 0) . ", `" . GID_RC_DEUTERIUM . "`=" . (int)($researchCost[GID_RC_DEUTERIUM] ?? 0) .
        " WHERE planet_id={$attackerPlanet}"
    );
    $researchUrl = $gameBase . '/index.php?page=buildings&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&mode=Forschung&bau=' . GID_R_ESPIONAGE;
    $researchResponses = e2e_parallel_requests('GET', $researchUrl, array(), $cookies, 4);
    $researchQueue = e2e_queue_sum(QTYP_RESEARCH, $attackerId, GID_R_ESPIONAGE, $attackerPlanet);
    $planetAfterResearchRace = e2e_planet_snapshot($attackerPlanet);
    $userAfterResearchRace = e2e_user_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'parallel_research_enqueue_does_not_duplicate_queue_or_overspend',
        'checks' => array_merge(e2e_all_responses_ok($researchResponses, 'parallel research enqueue'), array(
            e2e_case((int)$researchQueue['cnt'] === 1 && (int)$researchQueue['total_level'] === 1, 'parallel research requests leave exactly one active research queue task', $researchQueue),
            e2e_case($userAfterResearchRace !== null && (int)$userAfterResearchRace['espionage'] === 0, 'parallel research enqueue does not prematurely complete research', $userAfterResearchRace ?? array()),
            e2e_case($planetAfterResearchRace !== null && (int)$planetAfterResearchRace['metal'] >= 0 && (int)$planetAfterResearchRace['crystal'] >= 0 && (int)$planetAfterResearchRace['deuterium'] >= 0, 'parallel research requests do not overspend resources below zero', $planetAfterResearchRace ?? array()),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet, 'Concurrency Origin');
    $shipCost = TechPrice(GID_F_SC, 1);
    $shipCount = 3;
    dbquery(
        "UPDATE {$db_prefix}planets SET `" . GID_F_SC . "`=0, `" . GID_RC_METAL . "`=" . ((int)$shipCost[GID_RC_METAL] * $shipCount) . ", `" .
        GID_RC_CRYSTAL . "`=" . ((int)$shipCost[GID_RC_CRYSTAL] * $shipCount) . ", `" . GID_RC_DEUTERIUM . "`=" . ((int)($shipCost[GID_RC_DEUTERIUM] ?? 0) * $shipCount) .
        " WHERE planet_id={$attackerPlanet}"
    );
    $shipyardUrl = $gameBase . '/index.php?page=buildings&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&mode=Flotte';
    $shipyardResponses = e2e_parallel_requests('POST', $shipyardUrl, array('fmenge' => array(GID_F_SC => $shipCount)), $cookies, 4);
    $shipyardQueue = e2e_queue_sum(QTYP_SHIPYARD, $attackerId, GID_F_SC, $attackerPlanet);
    $planetAfterShipyardRace = e2e_planet_snapshot($attackerPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'parallel_shipyard_orders_do_not_exceed_available_resources',
        'checks' => array_merge(e2e_all_responses_ok($shipyardResponses, 'parallel shipyard order'), array(
            e2e_case((int)$shipyardQueue['total_level'] <= $shipCount, 'parallel shipyard requests do not queue more ships than resources allow', $shipyardQueue),
            e2e_case((int)$shipyardQueue['cnt'] <= 1, 'parallel shipyard requests leave at most one small-cargo queue row for the resource-limited order', $shipyardQueue),
            e2e_case($planetAfterShipyardRace !== null && (int)$planetAfterShipyardRace['metal'] >= 0 && (int)$planetAfterShipyardRace['crystal'] >= 0 && (int)$planetAfterShipyardRace['deuterium'] >= 0, 'parallel shipyard requests do not overspend resources below zero', $planetAfterShipyardRace ?? array()),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet, 'Concurrency Origin');
    e2e_reset_user_and_planet($defenderId, $defenderPlanet, 'Concurrency Target');
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    dbquery("UPDATE {$db_prefix}planets SET `" . GID_F_SC . "`=1, `" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000 WHERE planet_id={$attackerPlanet}");
    $origin = LoadPlanetById($attackerPlanet);
    $target = LoadPlanetById($defenderPlanet);
    $payload = e2e_fleet_payload($origin, $target, FTYP_TRANSPORT, array(GID_F_SC => 1), array());
    $fleetUrl = $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet;
    $fleetResponses = e2e_parallel_requests('POST', $fleetUrl, $payload, $cookies, 4);
    $fleetRows = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id={$attackerId} AND mission=" . FTYP_TRANSPORT);
    $fleetQueueRows = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_FLEET . "'");
    $planetAfterFleetRace = e2e_planet_snapshot($attackerPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'parallel_fleet_dispatch_does_not_duplicate_ship_or_queue',
        'checks' => array_merge(e2e_all_responses_ok($fleetResponses, 'parallel fleet dispatch'), array(
            e2e_case($fleetRows === 1, 'parallel fleet dispatch leaves exactly one outgoing fleet row when only one ship exists', array('fleet_rows' => $fleetRows)),
            e2e_case($fleetQueueRows === 1, 'parallel fleet dispatch leaves exactly one active fleet queue task', array('fleet_queue_rows' => $fleetQueueRows)),
            e2e_case($planetAfterFleetRace !== null && (int)$planetAfterFleetRace['small_cargo'] === 0, 'parallel fleet dispatch debits the single available ship exactly once', $planetAfterFleetRace ?? array()),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'concurrency_race_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($attackerId > 0 && $attackerPlanet > 0) {
        e2e_reset_user_and_planet($attackerId, $attackerPlanet, 'E2E Attacker');
    }
    if ($defenderId > 0 && $defenderPlanet > 0) {
        e2e_reset_user_and_planet($defenderId, $defenderPlanet, 'E2E Defender');
    }
    if ($attackerId > 0 && $defenderId > 0 && $attackerPlanet > 0 && $defenderPlanet > 0) {
        e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    }
}

echo json_encode(array(
    'case_group' => 'http_concurrency_race',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
