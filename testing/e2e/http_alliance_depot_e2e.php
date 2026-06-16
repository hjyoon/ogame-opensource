<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_alliance_depot_e2e.php';
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
loca_add('fleet', 'en');
loca_add('fleetmsg', 'en');
loca_add('infos', 'en');
loca_add('technames', 'en');
loca_add('union', 'en');

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

function e2e_cleanup_relations(array $userIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', $userIds));
    dbquery("DELETE FROM {$db_prefix}buddy WHERE request_from IN ({$userList}) OR request_to IN ({$userList})");
    dbquery("DELETE FROM {$db_prefix}union WHERE target_player IN ({$userList}) OR players REGEXP '(^|,)(" . implode('|', array_map('intval', $userIds)) . ")(,|$)'");
}

function e2e_prepare_user(int $userId): void
{
    global $db_prefix, $resmap;

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=10";
    }
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", " .
        "admin=0, validated=1, deact_ip=1, vacation=0, vacation_until=0, banned=0, banned_until=0, " .
        "noattack=0, noattack_until=0, disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, score1=0 " .
        "WHERE player_id={$userId}"
    );
    InvalidateUserCache();
}

function e2e_prepare_planet(int $planetId, int $ownerId, int $smallCargo = 10, int $lightFighter = 10): void
{
    global $db_prefix;

    $largeCargo = ($smallCargo > 0 || $lightFighter > 0) ? 5 : 0;
    $probe = ($smallCargo > 0 || $lightFighter > 0) ? 5 : 0;

    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "`" . GID_B_METAL_MINE . "`=0, `" . GID_B_CRYS_MINE . "`=0, `" . GID_B_DEUT_SYNTH . "`=0, `" . GID_B_SOLAR . "`=10, " .
        "`" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=10, `" . GID_B_RES_LAB . "`=10, `" . GID_B_ALLY_DEPOT . "`=0, " .
        "`" . GID_F_SC . "`={$smallCargo}, `" . GID_F_LC . "`={$largeCargo}, `" . GID_F_LF . "`={$lightFighter}, `" . GID_F_PROBE . "`={$probe}, " .
        "`" . GID_D_RL . "`=0, `" . GID_D_LL . "`=0, `" . GID_D_HL . "`=0, `" . GID_D_GAUSS . "`=0, `" . GID_D_ION . "`=0, `" . GID_D_PLASMA . "`=0, " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=0, maxfields=200, type=" . PTYP_PLANET . ", owner_id={$ownerId} " .
        "WHERE planet_id={$planetId}"
    );
}

function e2e_reset_user_and_planet(int $userId, int $planetId, int $smallCargo = 10, int $lightFighter = 10): void
{
    global $db_prefix;

    e2e_cleanup_fleets(array($userId), array($planetId));
    e2e_prepare_user($userId);
    e2e_prepare_planet($planetId, $userId, $smallCargo, $lightFighter);
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$planetId} WHERE player_id={$userId}");
    SelectPlanet($userId, $planetId);
}

function e2e_fleet_payload(array $origin, array $target, int $order, array $ships, array $resources = array(), array $extra = array()): array
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
        'planettype' => (string)($target['game_type'] ?? GetPlanetType($target)),
        'speed' => '10',
        'order' => (string)$order,
    );
    foreach ($fleetmap as $gid) {
        $data['ship' . $gid] = (string)($ships[$gid] ?? 0);
    }
    foreach ($transportableResources as $i => $rc) {
        $data['resource' . ($i + 1)] = (string)($resources[$rc] ?? 0);
    }
    foreach ($extra as $key => $value) {
        $data[$key] = (string)$value;
    }
    return $data;
}

function e2e_latest_fleet(int $ownerId, int $mission): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT * FROM {$db_prefix}fleet WHERE owner_id={$ownerId} AND mission={$mission} ORDER BY fleet_id DESC LIMIT 1");
}

function e2e_complete_fleet(?array $fleet): bool
{
    if ($fleet === null) {
        return false;
    }
    $queue = GetFleetQueue((int)$fleet['fleet_id']);
    if ($queue === null || $queue === false) {
        return false;
    }
    Queue_Fleet_End($queue);
    return true;
}

function e2e_planet_depot_snapshot(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, `" . GID_B_ALLY_DEPOT . "` AS depot_level, " .
        "`" . GID_RC_DEUTERIUM . "` AS deuterium FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1"
    );
}

function e2e_set_depot_state(int $planetId, int $level, float $deuterium): void
{
    global $db_prefix;
    dbquery(
        "UPDATE {$db_prefix}planets SET `" . GID_B_ALLY_DEPOT . "`={$level}, `" . GID_RC_DEUTERIUM . "`={$deuterium} " .
        "WHERE planet_id={$planetId}"
    );
}

function e2e_hold_hourly_cost(array $orbitFleet, int $targetPlanetId): float
{
    global $fleetmap;

    $target = LoadPlanetById($targetPlanetId);
    $user = LoadUser((int)$orbitFleet['owner_id']);
    if ($user === null) {
        $user = array(GID_R_COMBUST_DRIVE => 0, GID_R_IMPULSE_DRIVE => 0, GID_R_HYPER_DRIVE => 0);
    }

    $cost = 0.0;
    foreach ($fleetmap as $gid) {
        $amount = (int)($orbitFleet[$gid] ?? 0);
        if ($amount > 0) {
            $cost += $amount * FleetCons($gid, $user, $target) / 10;
        }
    }
    return $cost;
}

function e2e_queue_for_fleet(?array $fleet): ?array
{
    if ($fleet === null) {
        return null;
    }
    $queue = GetFleetQueue((int)$fleet['fleet_id']);
    return ($queue === false || $queue === null) ? null : $queue;
}

function e2e_launch_orbiting_hold(string $gameBase, int $attackerPlanet, int $defenderPlanet, string $attackerSession, array &$attackerCookies): array
{
    $origin = LoadPlanetById($attackerPlanet);
    $target = LoadPlanetById($defenderPlanet);
    $response = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($attackerSession) . '&cp=' . $attackerPlanet,
        e2e_fleet_payload($origin, $target, FTYP_ACS_HOLD, array(GID_F_LF => 1), array(), array('holdingtime' => 1)),
        $attackerCookies
    );
    $holdFleet = e2e_latest_fleet((int)$origin['owner_id'], FTYP_ACS_HOLD);
    $completedOutbound = e2e_complete_fleet($holdFleet);
    $orbitFleet = e2e_latest_fleet((int)$origin['owner_id'], FTYP_ACS_HOLD + FTYP_ORBITING);
    $orbitQueue = e2e_queue_for_fleet($orbitFleet);

    return array(
        'response' => $response,
        'hold_fleet' => $holdFleet,
        'completed_outbound' => $completedOutbound,
        'orbit_fleet' => $orbitFleet,
        'orbit_queue' => $orbitQueue,
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

    e2e_reset_user_and_planet($attackerId, $attackerPlanet, 10, 10);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet, 0, 0);
    e2e_cleanup_relations(array($attackerId, $defenderId));
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));

    $buddyId = AddBuddy($attackerId, $defenderId, 'E2E alliance depot hold relation');
    if ($buddyId > 0) {
        AcceptBuddy($buddyId);
    }

    $attackerAuth = e2e_prepare_session($attackerId, 'alliance-depot-attacker');
    $attackerCookies = $attackerAuth['cookies'];
    $attackerSession = $attackerAuth['session'];
    $defenderAuth = e2e_prepare_session($defenderId, 'alliance-depot-defender');
    $defenderCookies = $defenderAuth['cookies'];
    $defenderSession = $defenderAuth['session'];

    $hold = e2e_launch_orbiting_hold($gameBase, $attackerPlanet, $defenderPlanet, $attackerSession, $attackerCookies);
    $orbitFleet = $hold['orbit_fleet'];
    $orbitQueue = $hold['orbit_queue'];
    $hourlyCost = $orbitFleet === null ? 0.0 : e2e_hold_hourly_cost($orbitFleet, $defenderPlanet);

    $cases[] = e2e_finalize_case(array(
        'case' => 'alliance_depot_fixture_hold_orbits',
        'checks' => array_merge(e2e_response_check($hold['response'], 'ACS hold launch'), array(
            e2e_case($buddyId > 0 && IsBuddy($attackerId, $defenderId), 'accepted buddy relation enables ACS hold', array('buddy_id' => $buddyId)),
            e2e_case($hold['hold_fleet'] !== null && (int)$hold['hold_fleet']['mission'] === FTYP_ACS_HOLD, 'outbound ACS hold fleet is created', $hold['hold_fleet'] ?? array()),
            e2e_case($hold['completed_outbound'], 'outbound ACS hold queue is completed into orbit'),
            e2e_case($orbitFleet !== null && (int)$orbitFleet['mission'] === FTYP_ACS_HOLD + FTYP_ORBITING, 'ACS hold fleet is orbiting the defender planet', $orbitFleet ?? array()),
            e2e_case($orbitQueue !== null, 'orbiting ACS hold fleet has a queue task', $orbitQueue ?? array()),
            e2e_case($hourlyCost > 0, 'orbiting ACS hold fleet has a positive hourly deuterium cost', array('hourly_cost' => $hourlyCost)),
        )),
    ));

    e2e_set_depot_state($defenderPlanet, 2, 50000);
    $infoResponse = e2e_http_request(
        'GET',
        $gameBase . '/index.php?page=infos&session=' . rawurlencode($defenderSession) . '&cp=' . $defenderPlanet . '&gid=' . GID_B_ALLY_DEPOT,
        array(),
        $defenderCookies
    );
    $infoBody = $infoResponse['body'];
    $attackerUser = LoadUser($attackerId);
    $attackerDisplayName = $attackerUser['oname'] ?? '';
    $cases[] = e2e_finalize_case(array(
        'case' => 'alliance_depot_info_page_renders_holding_fleet',
        'checks' => array_merge(e2e_response_check($infoResponse, 'Alliance depot info page'), array(
            e2e_case(stripos($infoBody, 'Capacity:') !== false, 'info page renders depot capacity'),
            e2e_case($attackerDisplayName !== '' && stripos($infoBody, 'Fleet ' . $attackerDisplayName) !== false, 'info page lists an orbiting hold fleet owner', array('owner_name' => $attackerDisplayName)),
            e2e_case(stripos($infoBody, 'Light Fighter') !== false, 'info page lists held ship counts'),
            e2e_case(preg_match('/name=[\'"]c1[\'"]/i', $infoBody) === 1, 'info page renders the first supply-hours input'),
            e2e_case(stripos($infoBody, 'Cost') !== false && stripos($infoBody, '/ hr') !== false, 'info page renders hourly fuel cost'),
            e2e_case(stripos($infoBody, 'Launch a rocket with supplies') !== false, 'info page renders the supply submit button'),
        )),
    ));

    e2e_set_depot_state($defenderPlanet, 0, 50000);
    $queueBeforeNoDepot = e2e_queue_for_fleet($orbitFleet);
    $planetBeforeNoDepot = e2e_planet_depot_snapshot($defenderPlanet);
    $noDepotResponse = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=allianzdepot&session=' . rawurlencode($defenderSession) . '&cp=' . $defenderPlanet,
        array('c1' => 2),
        $defenderCookies
    );
    $queueAfterNoDepot = e2e_queue_for_fleet($orbitFleet);
    $planetAfterNoDepot = e2e_planet_depot_snapshot($defenderPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'alliance_depot_without_building_does_not_supply',
        'checks' => array_merge(e2e_response_check($noDepotResponse, 'Alliance depot no-building POST'), array(
            e2e_case($queueBeforeNoDepot !== null && $queueAfterNoDepot !== null && (int)$queueAfterNoDepot['end'] === (int)$queueBeforeNoDepot['end'], 'missing alliance depot leaves orbit queue end unchanged', array('before' => $queueBeforeNoDepot, 'after' => $queueAfterNoDepot)),
            e2e_case($planetBeforeNoDepot !== null && $planetAfterNoDepot !== null && (float)$planetAfterNoDepot['deuterium'] === (float)$planetBeforeNoDepot['deuterium'], 'missing alliance depot does not spend deuterium', array('before' => $planetBeforeNoDepot, 'after' => $planetAfterNoDepot)),
        )),
    ));

    e2e_set_depot_state($defenderPlanet, 1, max(0, floor($hourlyCost) - 1));
    $queueBeforeInsufficient = e2e_queue_for_fleet($orbitFleet);
    $planetBeforeInsufficient = e2e_planet_depot_snapshot($defenderPlanet);
    $insufficientResponse = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=allianzdepot&session=' . rawurlencode($defenderSession) . '&cp=' . $defenderPlanet,
        array('c1' => 1),
        $defenderCookies
    );
    $queueAfterInsufficient = e2e_queue_for_fleet($orbitFleet);
    $planetAfterInsufficient = e2e_planet_depot_snapshot($defenderPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'alliance_depot_insufficient_deuterium_does_not_supply',
        'checks' => array_merge(e2e_response_check($insufficientResponse, 'Alliance depot insufficient-fuel POST'), array(
            e2e_case($queueBeforeInsufficient !== null && $queueAfterInsufficient !== null && (int)$queueAfterInsufficient['end'] === (int)$queueBeforeInsufficient['end'], 'insufficient loaded deuterium leaves orbit queue end unchanged', array('before' => $queueBeforeInsufficient, 'after' => $queueAfterInsufficient)),
            e2e_case($planetBeforeInsufficient !== null && $planetAfterInsufficient !== null && (float)$planetAfterInsufficient['deuterium'] === (float)$planetBeforeInsufficient['deuterium'], 'insufficient loaded deuterium is not spent', array('before' => $planetBeforeInsufficient, 'after' => $planetAfterInsufficient, 'hourly_cost' => $hourlyCost)),
        )),
    ));

    $supplyHours = 2;
    e2e_set_depot_state($defenderPlanet, 2, 50000);
    $queueBeforeSupply = e2e_queue_for_fleet($orbitFleet);
    $planetBeforeSupply = e2e_planet_depot_snapshot($defenderPlanet);
    $expectedSpent = $hourlyCost * $supplyHours;
    $supplyResponse = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=allianzdepot&session=' . rawurlencode($defenderSession) . '&cp=' . $defenderPlanet,
        array('c1' => $supplyHours),
        $defenderCookies
    );
    $queueAfterSupply = e2e_queue_for_fleet($orbitFleet);
    $planetAfterSupply = e2e_planet_depot_snapshot($defenderPlanet);
    $expectedDeuterium = $planetBeforeSupply === null ? 0.0 : (float)$planetBeforeSupply['deuterium'] - $expectedSpent;
    $actualDeuterium = $planetAfterSupply === null ? 0.0 : (float)$planetAfterSupply['deuterium'];
    $cases[] = e2e_finalize_case(array(
        'case' => 'alliance_depot_supplies_holding_fleet',
        'checks' => array_merge(e2e_response_check($supplyResponse, 'Alliance depot supply POST'), array(
            e2e_case($queueBeforeSupply !== null && $queueAfterSupply !== null && (int)$queueAfterSupply['end'] === (int)$queueBeforeSupply['end'] + $supplyHours * 3600, 'successful supply extends orbit queue by requested hours', array('before' => $queueBeforeSupply, 'after' => $queueAfterSupply, 'hours' => $supplyHours)),
            e2e_case($planetBeforeSupply !== null && $planetAfterSupply !== null && abs($actualDeuterium - $expectedDeuterium) < 0.0001, 'successful supply spends exactly the calculated deuterium cost', array('before' => $planetBeforeSupply, 'after' => $planetAfterSupply, 'hourly_cost' => $hourlyCost, 'expected_spent' => $expectedSpent)),
            e2e_case(strpos($supplyResponse['location'], 'page=infos') !== false && strpos($supplyResponse['location'], 'gid=' . GID_B_ALLY_DEPOT) !== false, 'successful supply redirects back to alliance depot info', array('location' => $supplyResponse['location'])),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'alliance_depot_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    $userIds = array_filter(array($attackerId, $defenderId), fn($id) => $id > 0);
    $planetIds = array_filter(array($attackerPlanet, $defenderPlanet), fn($id) => $id > 0);
    if (!empty($userIds) && !empty($planetIds)) {
        e2e_cleanup_fleets($userIds, $planetIds);
        e2e_cleanup_relations($userIds);
    }
    if ($attackerId > 0 && $attackerPlanet > 0) {
        e2e_reset_user_and_planet($attackerId, $attackerPlanet, 10, 10);
    }
    if ($defenderId > 0 && $defenderPlanet > 0) {
        e2e_reset_user_and_planet($defenderId, $defenderPlanet, 0, 0);
    }
}

echo json_encode(array(
    'case_group' => 'http_alliance_depot',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
