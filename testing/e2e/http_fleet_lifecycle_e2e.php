<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_fleet_lifecycle_e2e.php';
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

function e2e_reset_user_and_planet(int $userId, int $planetId): void
{
    global $db_prefix, $resmap;

    e2e_cleanup_fleets(array($userId), array($planetId));
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$userId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_UNBAN . "','" . QTYP_ALLOW_ATTACKS . "','" . QTYP_DEBUG . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$userId} OR planet_id={$planetId}");

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=10";
    }
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", " .
        "admin=0, validated=1, deact_ip=1, vacation=0, vacation_until=0, banned=0, banned_until=0, " .
        "noattack=0, noattack_until=0, disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1 " .
        "WHERE player_id={$userId}"
    );

    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "`" . GID_B_METAL_MINE . "`=0, `" . GID_B_CRYS_MINE . "`=0, `" . GID_B_DEUT_SYNTH . "`=0, `" . GID_B_SOLAR . "`=10, " .
        "`" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=10, `" . GID_B_RES_LAB . "`=10, " .
        "`" . GID_F_SC . "`=10, `" . GID_F_LC . "`=5, `" . GID_F_LF . "`=5, `" . GID_F_PROBE . "`=5, " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=0, maxfields=200, type=" . PTYP_PLANET . ", owner_id={$userId} " .
        "WHERE planet_id={$planetId}"
    );
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$planetId} WHERE player_id={$userId}");
    SelectPlanet($userId, $planetId);
    InvalidateUserCache();
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
    if ($queue === null) {
        return false;
    }
    Queue_Fleet_End($queue);
    return true;
}

function e2e_active_fleet_count(int $userId): int
{
    global $db_prefix;
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$userId} AND type='" . QTYP_FLEET . "'");
}

function e2e_planet_snapshot(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, g, s, p, type, " .
        "`" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deuterium, " .
        "`" . GID_F_SC . "` AS small_cargo, `" . GID_F_LC . "` AS large_cargo, `" . GID_F_PROBE . "` AS probe " .
        "FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1"
    );
}

function e2e_create_temp_planet(int $ownerId): int
{
    global $GlobalUni;

    for ($g = 1; $g <= (int)$GlobalUni['galaxies']; $g++) {
        for ($s = 1; $s <= (int)$GlobalUni['systems']; $s++) {
            for ($p = 15; $p >= 1; $p--) {
                if (!HasPlanet($g, $s, $p)) {
                    $planetId = CreatePlanet($g, $s, $p, $ownerId, 1, 0, 0, time());
                    if ($planetId > 0) {
                        return $planetId;
                    }
                }
            }
        }
    }

    throw new RuntimeException('No empty planet slot found for deploy lifecycle test.');
}

function e2e_prepare_temp_planet(int $planetId, int $ownerId): void
{
    global $db_prefix;

    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "`" . GID_RC_METAL . "`=1000, `" . GID_RC_CRYSTAL . "`=1000, `" . GID_RC_DEUTERIUM . "`=1000, " .
        "`" . GID_F_SC . "`=0, `" . GID_F_LC . "`=0, `" . GID_F_LF . "`=0, `" . GID_F_PROBE . "`=0, " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=0, maxfields=200, type=" . PTYP_PLANET . ", owner_id={$ownerId} " .
        "WHERE planet_id={$planetId}"
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

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $auth = e2e_prepare_session($attackerId, 'fleet-lifecycle-attacker');
    $cookies = $auth['cookies'];
    $session = $auth['session'];

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet);
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));

    $messageCountBefore = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}messages WHERE owner_id IN ({$attackerId},{$defenderId})");
    $origin = LoadPlanetById($attackerPlanet);
    $target = LoadPlanetById($defenderPlanet);
    $originBefore = e2e_planet_snapshot($attackerPlanet);
    $targetBefore = e2e_planet_snapshot($defenderPlanet);
    $payload = e2e_fleet_payload($origin, $target, FTYP_TRANSPORT, array(GID_F_SC => 1), array(GID_RC_METAL => 123, GID_RC_CRYSTAL => 45));
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, $payload, $cookies);
    $outgoing = e2e_latest_fleet($attackerId, FTYP_TRANSPORT);
    $outgoingQueue = $outgoing ? GetFleetQueue((int)$outgoing['fleet_id']) : null;
    $originAfterLaunch = e2e_planet_snapshot($attackerPlanet);
    $completedOutgoing = e2e_complete_fleet($outgoing);
    $targetAfterArrival = e2e_planet_snapshot($defenderPlanet);
    $returnFleet = e2e_latest_fleet($attackerId, FTYP_TRANSPORT + FTYP_RETURN);
    $returnQueue = $returnFleet ? GetFleetQueue((int)$returnFleet['fleet_id']) : null;
    $completedReturn = e2e_complete_fleet($returnFleet);
    $originAfterReturn = e2e_planet_snapshot($attackerPlanet);
    $messageCountAfter = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}messages WHERE owner_id IN ({$attackerId},{$defenderId})");
    $cases[] = e2e_finalize_case(array(
        'case' => 'transport_arrival_creates_return_and_final_return_restores_ships',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($outgoing !== null && (int)$outgoing['mission'] === FTYP_TRANSPORT, 'outgoing transport fleet is created', $outgoing ?? array()),
            e2e_case($outgoingQueue !== null, 'outgoing transport has a fleet queue task', $outgoingQueue ?? array()),
            e2e_case($originBefore !== null && $originAfterLaunch !== null && (int)$originAfterLaunch['small_cargo'] === (int)$originBefore['small_cargo'] - 1, 'origin ship count is debited on transport launch', array('before' => $originBefore, 'after_launch' => $originAfterLaunch)),
            e2e_case($completedOutgoing, 'outgoing transport queue can be completed by the fleet queue handler'),
            e2e_case($targetBefore !== null && $targetAfterArrival !== null && (int)$targetAfterArrival['metal'] >= (int)$targetBefore['metal'] + 123 && (int)$targetAfterArrival['crystal'] >= (int)$targetBefore['crystal'] + 45, 'transported resources are delivered to target planet', array('before' => $targetBefore, 'after_arrival' => $targetAfterArrival)),
            e2e_case($returnFleet !== null && (int)$returnFleet['mission'] === FTYP_TRANSPORT + FTYP_RETURN, 'transport arrival creates a return fleet', $returnFleet ?? array()),
            e2e_case($returnQueue !== null, 'transport return fleet has a queue task', $returnQueue ?? array()),
            e2e_case($completedReturn, 'transport return queue can be completed by the fleet queue handler'),
            e2e_case($originBefore !== null && $originAfterReturn !== null && (int)$originAfterReturn['small_cargo'] === (int)$originBefore['small_cargo'], 'transport return restores ship to origin', array('before' => $originBefore, 'after_return' => $originAfterReturn)),
            e2e_case(e2e_active_fleet_count($attackerId) === 0, 'transport lifecycle leaves no active fleet queue tasks'),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id={$attackerId}") === 0, 'transport lifecycle removes all transient fleet rows'),
            e2e_case($messageCountAfter >= $messageCountBefore + 3, 'transport arrival and return write fleet messages for participants', array('before' => $messageCountBefore, 'after' => $messageCountAfter)),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    $deployTargetPlanet = e2e_create_temp_planet($attackerId);
    $createdPlanets[] = $deployTargetPlanet;
    e2e_prepare_temp_planet($deployTargetPlanet, $attackerId);
    $origin = LoadPlanetById($attackerPlanet);
    $deployTarget = LoadPlanetById($deployTargetPlanet);
    $originBefore = e2e_planet_snapshot($attackerPlanet);
    $targetBefore = e2e_planet_snapshot($deployTargetPlanet);
    $payload = e2e_fleet_payload($origin, $deployTarget, FTYP_DEPLOY, array(GID_F_SC => 2), array(GID_RC_METAL => 77, GID_RC_CRYSTAL => 22, GID_RC_DEUTERIUM => 11));
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, $payload, $cookies);
    $deployFleet = e2e_latest_fleet($attackerId, FTYP_DEPLOY);
    $deployQueue = $deployFleet ? GetFleetQueue((int)$deployFleet['fleet_id']) : null;
    $originAfterLaunch = e2e_planet_snapshot($attackerPlanet);
    $completedDeploy = e2e_complete_fleet($deployFleet);
    $targetAfterDeploy = e2e_planet_snapshot($deployTargetPlanet);
    $returnDeploy = e2e_latest_fleet($attackerId, FTYP_DEPLOY + FTYP_RETURN);
    $cases[] = e2e_finalize_case(array(
        'case' => 'deploy_arrival_keeps_ships_and_resources_on_owned_target',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($deployFleet !== null && (int)$deployFleet['mission'] === FTYP_DEPLOY, 'deploy fleet is created for owned target planet', $deployFleet ?? array()),
            e2e_case($deployQueue !== null, 'deploy fleet has a queue task', $deployQueue ?? array()),
            e2e_case($originBefore !== null && $originAfterLaunch !== null && (int)$originAfterLaunch['small_cargo'] === (int)$originBefore['small_cargo'] - 2, 'origin ship count is debited on deploy launch', array('before' => $originBefore, 'after_launch' => $originAfterLaunch)),
            e2e_case($completedDeploy, 'deploy queue can be completed by the fleet queue handler'),
            e2e_case($targetBefore !== null && $targetAfterDeploy !== null && (int)$targetAfterDeploy['small_cargo'] === (int)$targetBefore['small_cargo'] + 2, 'deployed ships remain on target planet', array('before' => $targetBefore, 'after_deploy' => $targetAfterDeploy)),
            e2e_case($targetBefore !== null && $targetAfterDeploy !== null && (int)$targetAfterDeploy['metal'] >= (int)$targetBefore['metal'] + 77 && (int)$targetAfterDeploy['crystal'] >= (int)$targetBefore['crystal'] + 22 && (int)$targetAfterDeploy['deuterium'] >= (int)$targetBefore['deuterium'] + 11, 'deployed resources are unloaded on target planet', array('before' => $targetBefore, 'after_deploy' => $targetAfterDeploy)),
            e2e_case($returnDeploy === null, 'completed deploy does not create a return fleet'),
            e2e_case(e2e_active_fleet_count($attackerId) === 0, 'deploy lifecycle leaves no active fleet queue tasks'),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet);
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    $origin = LoadPlanetById($attackerPlanet);
    $target = LoadPlanetById($defenderPlanet);
    $originBefore = e2e_planet_snapshot($attackerPlanet);
    $targetBefore = e2e_planet_snapshot($defenderPlanet);
    $payload = e2e_fleet_payload($origin, $target, FTYP_TRANSPORT, array(GID_F_SC => 1), array(GID_RC_METAL => 88, GID_RC_CRYSTAL => 33));
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, $payload, $cookies);
    $recallOutgoing = e2e_latest_fleet($attackerId, FTYP_TRANSPORT);
    $recallQueue = $recallOutgoing ? GetFleetQueue((int)$recallOutgoing['fleet_id']) : null;
    $recallWhen = $recallQueue === null ? time() : ((int)$recallQueue['start'] + 1);
    if ($recallOutgoing !== null) {
        RecallFleet((int)$recallOutgoing['fleet_id'], $recallWhen);
    }
    $returnAfterRecall = e2e_latest_fleet($attackerId, FTYP_TRANSPORT + FTYP_RETURN);
    $originalAfterRecall = $recallOutgoing === null ? null : LoadFleet((int)$recallOutgoing['fleet_id']);
    $completedRecallReturn = e2e_complete_fleet($returnAfterRecall);
    $originAfterRecallReturn = e2e_planet_snapshot($attackerPlanet);
    $targetAfterRecall = e2e_planet_snapshot($defenderPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'transport_recall_returns_ships_and_cargo_without_delivery',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($recallOutgoing !== null && $recallQueue !== null, 'recall case creates an outgoing transport fleet and queue', array('fleet' => $recallOutgoing, 'queue' => $recallQueue)),
            e2e_case($originalAfterRecall === null || $originalAfterRecall === false, 'recall removes the original outgoing fleet row'),
            e2e_case($returnAfterRecall !== null && (int)$returnAfterRecall['mission'] === FTYP_TRANSPORT + FTYP_RETURN, 'recall creates a transport return fleet', $returnAfterRecall ?? array()),
            e2e_case($completedRecallReturn, 'recalled transport return queue can be completed by the fleet queue handler'),
            e2e_case($originBefore !== null && $originAfterRecallReturn !== null && (int)$originAfterRecallReturn['small_cargo'] === (int)$originBefore['small_cargo'], 'recalled transport restores ship to origin', array('before' => $originBefore, 'after_return' => $originAfterRecallReturn)),
            e2e_case($originBefore !== null && $originAfterRecallReturn !== null && (int)$originAfterRecallReturn['metal'] >= (int)$originBefore['metal'] && (int)$originAfterRecallReturn['crystal'] >= (int)$originBefore['crystal'], 'recalled transport returns loaded cargo to origin', array('before' => $originBefore, 'after_return' => $originAfterRecallReturn)),
            e2e_case($targetBefore !== null && $targetAfterRecall !== null && (int)$targetAfterRecall['metal'] === (int)$targetBefore['metal'] && (int)$targetAfterRecall['crystal'] === (int)$targetBefore['crystal'], 'recalled transport does not deliver cargo to target', array('before' => $targetBefore, 'after_recall' => $targetAfterRecall)),
            e2e_case(e2e_active_fleet_count($attackerId) === 0, 'recall lifecycle leaves no active fleet queue tasks'),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id={$attackerId}") === 0, 'recall lifecycle removes all transient fleet rows'),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'fleet_lifecycle_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($attackerId > 0 && $attackerPlanet > 0) {
        e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    }
    if ($defenderId > 0 && $defenderPlanet > 0) {
        e2e_reset_user_and_planet($defenderId, $defenderPlanet);
    }
    if ($attackerId > 0 && $defenderId > 0 && $attackerPlanet > 0 && $defenderPlanet > 0) {
        e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    }
    foreach ($createdPlanets as $planetId) {
        if ($planetId > 0 && LoadPlanetById($planetId) !== null) {
            DestroyPlanet($planetId);
        }
    }
}

echo json_encode(array(
    'case_group' => 'http_fleet_lifecycle',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
