<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_fleet_recall_edges_e2e.php';
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

function e2e_cleanup_relations(array $userIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', $userIds));
    if ($userList === '') {
        return;
    }
    dbquery("DELETE FROM {$db_prefix}buddy WHERE request_from IN ({$userList}) OR request_to IN ({$userList})");
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
        "noattack=0, noattack_until=0, disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, score1=0 " .
        "WHERE player_id={$userId}"
    );
    InvalidateUserCache();
}

function e2e_prepare_planet(int $planetId, int $ownerId, int $smallCargo = 10, int $largeCargo = 5, int $lightFighter = 5, int $probe = 5): void
{
    global $db_prefix;

    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "`" . GID_B_METAL_MINE . "`=0, `" . GID_B_CRYS_MINE . "`=0, `" . GID_B_DEUT_SYNTH . "`=0, `" . GID_B_SOLAR . "`=10, " .
        "`" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=10, `" . GID_B_RES_LAB . "`=10, " .
        "`" . GID_F_SC . "`={$smallCargo}, `" . GID_F_LC . "`={$largeCargo}, `" . GID_F_LF . "`={$lightFighter}, `" . GID_F_PROBE . "`={$probe}, " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=0, maxfields=200, type=" . PTYP_PLANET . ", owner_id={$ownerId} " .
        "WHERE planet_id={$planetId}"
    );
}

function e2e_reset_user_and_planet(int $userId, int $planetId, int $smallCargo = 10, int $largeCargo = 5, int $lightFighter = 5, int $probe = 5): void
{
    global $db_prefix;

    e2e_cleanup_fleets(array($userId), array($planetId));
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$userId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_UNBAN . "','" . QTYP_ALLOW_ATTACKS . "','" . QTYP_DEBUG . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$userId} OR planet_id={$planetId}");
    e2e_reset_user($userId);
    e2e_prepare_planet($planetId, $userId, $smallCargo, $largeCargo, $lightFighter, $probe);
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$planetId} WHERE player_id={$userId}");
    SelectPlanet($userId, $planetId);
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

    throw new RuntimeException('No empty planet slot found.');
}

function e2e_create_planet_near(int $ownerId, array $near): int
{
    [$g, $s, $p] = e2e_find_empty_position($near);
    $planetId = CreatePlanet($g, $s, $p, $ownerId, 1, 0, 0, time());
    if ($planetId <= 0) {
        throw new RuntimeException('Failed to create a nearby planet.');
    }
    e2e_prepare_planet($planetId, $ownerId, 0, 0, 0, 0);
    return $planetId;
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

function e2e_send_fleet(string $gameBase, string $session, int $currentPlanet, array $payload, array &$cookies): array
{
    return e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $currentPlanet,
        $payload,
        $cookies
    );
}

function e2e_recall_fleet(string $gameBase, string $session, int $currentPlanet, int $fleetId, array &$cookies): array
{
    return e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=flotten1&session=' . rawurlencode($session) . '&cp=' . $currentPlanet,
        array('order_return' => $fleetId),
        $cookies
    );
}

function e2e_latest_fleet(int $ownerId, int $mission): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT * FROM {$db_prefix}fleet WHERE owner_id={$ownerId} AND mission={$mission} ORDER BY fleet_id DESC LIMIT 1");
}

function e2e_fleet_by_id(int $fleetId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT * FROM {$db_prefix}fleet WHERE fleet_id={$fleetId} LIMIT 1");
}

function e2e_queue_snapshot(int $fleetId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT task_id, owner_id, type, sub_id, start, end, prio FROM {$db_prefix}queue WHERE type='" . QTYP_FLEET . "' AND sub_id={$fleetId} LIMIT 1");
}

function e2e_fleet_row_count(int $fleetId): int
{
    global $db_prefix;
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE fleet_id={$fleetId}");
}

function e2e_queue_row_count(int $fleetId): int
{
    global $db_prefix;
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE type='" . QTYP_FLEET . "' AND sub_id={$fleetId}");
}

function e2e_age_fleet_queue(int $fleetId, int $elapsedSeconds): void
{
    global $db_prefix;
    $start = time() - $elapsedSeconds;
    dbquery("UPDATE {$db_prefix}queue SET start={$start} WHERE type='" . QTYP_FLEET . "' AND sub_id={$fleetId}");
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

function e2e_active_fleet_count(int $userId): int
{
    global $db_prefix;
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$userId} AND type='" . QTYP_FLEET . "'");
}

function e2e_fleet_count(int $userId): int
{
    global $db_prefix;
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id={$userId}");
}

function e2e_planet_snapshot(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, g, s, p, type, " .
        "`" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deuterium, " .
        "`" . GID_F_SC . "` AS small_cargo, `" . GID_F_LC . "` AS large_cargo, `" . GID_F_LF . "` AS light_fighter, `" . GID_F_PROBE . "` AS probe " .
        "FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1"
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

    $attackerAuth = e2e_prepare_session($attackerId, 'fleet-recall-attacker');
    $attackerCookies = $attackerAuth['cookies'];
    $attackerSession = $attackerAuth['session'];
    $defenderAuth = e2e_prepare_session($defenderId, 'fleet-recall-defender');
    $defenderCookies = $defenderAuth['cookies'];
    $defenderSession = $defenderAuth['session'];

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet);
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    $response = e2e_recall_fleet($gameBase, $attackerSession, $attackerPlanet, 987654321, $attackerCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'recall_ignores_missing_fleet_id_without_php_warning',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(e2e_active_fleet_count($attackerId) === 0, 'missing fleet recall does not create queue tasks'),
            e2e_case(e2e_fleet_count($attackerId) === 0, 'missing fleet recall does not create fleet rows'),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet);
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    $defenderOrigin = LoadPlanetById($defenderPlanet);
    $attackerTarget = LoadPlanetById($attackerPlanet);
    $sendResponse = e2e_send_fleet(
        $gameBase,
        $defenderSession,
        $defenderPlanet,
        e2e_fleet_payload($defenderOrigin, $attackerTarget, FTYP_TRANSPORT, array(GID_F_SC => 1), array(GID_RC_METAL => 17)),
        $defenderCookies
    );
    $foreignFleet = e2e_latest_fleet($defenderId, FTYP_TRANSPORT);
    $foreignBefore = $foreignFleet === null ? null : e2e_fleet_by_id((int)$foreignFleet['fleet_id']);
    $response = e2e_recall_fleet($gameBase, $attackerSession, $attackerPlanet, (int)($foreignFleet['fleet_id'] ?? 0), $attackerCookies);
    $foreignAfter = $foreignFleet === null ? null : e2e_fleet_by_id((int)$foreignFleet['fleet_id']);
    $foreignReturn = e2e_latest_fleet($defenderId, FTYP_TRANSPORT + FTYP_RETURN);
    $cases[] = e2e_finalize_case(array(
        'case' => 'recall_rejects_foreign_fleet_direct_post',
        'checks' => array_merge(e2e_response_check($sendResponse), e2e_response_check($response), array(
            e2e_case($foreignBefore !== null && $foreignAfter !== null, 'foreign fleet remains after another user recall POST', array('before' => $foreignBefore, 'after' => $foreignAfter)),
            e2e_case($foreignAfter !== null && (int)$foreignAfter['mission'] === FTYP_TRANSPORT, 'foreign fleet keeps its outbound mission', $foreignAfter ?? array()),
            e2e_case($foreignReturn === null, 'foreign recall attempt does not create a return fleet', $foreignReturn ?? array()),
            e2e_case(e2e_active_fleet_count($attackerId) === 0, 'foreign recall attempt does not create attacker queue tasks'),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet);
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    $origin = LoadPlanetById($attackerPlanet);
    $target = LoadPlanetById($defenderPlanet);
    $originBefore = e2e_planet_snapshot($attackerPlanet);
    $targetBefore = e2e_planet_snapshot($defenderPlanet);
    $sendResponse = e2e_send_fleet(
        $gameBase,
        $attackerSession,
        $attackerPlanet,
        e2e_fleet_payload($origin, $target, FTYP_TRANSPORT, array(GID_F_SC => 1), array(GID_RC_METAL => 200, GID_RC_CRYSTAL => 50)),
        $attackerCookies
    );
    $outgoing = e2e_latest_fleet($attackerId, FTYP_TRANSPORT);
    if ($outgoing !== null) {
        e2e_age_fleet_queue((int)$outgoing['fleet_id'], 10);
    }
    $response = e2e_recall_fleet($gameBase, $attackerSession, $attackerPlanet, (int)($outgoing['fleet_id'] ?? 0), $attackerCookies);
    $outgoingAfterRecall = $outgoing === null ? null : e2e_fleet_by_id((int)$outgoing['fleet_id']);
    $returnFleet = e2e_latest_fleet($attackerId, FTYP_TRANSPORT + FTYP_RETURN);
    $targetAfterRecall = e2e_planet_snapshot($defenderPlanet);
    $completedReturn = e2e_complete_fleet($returnFleet);
    $originAfterReturn = e2e_planet_snapshot($attackerPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'transport_recall_via_flotten1_returns_ship_and_cargo',
        'checks' => array_merge(e2e_response_check($sendResponse), e2e_response_check($response), array(
            e2e_case($outgoing !== null, 'outbound transport exists before recall', $outgoing ?? array()),
            e2e_case($outgoingAfterRecall === null, 'recall removes original outbound transport fleet'),
            e2e_case($returnFleet !== null && (int)$returnFleet['mission'] === FTYP_TRANSPORT + FTYP_RETURN, 'recall creates a transport return fleet', $returnFleet ?? array()),
            e2e_case($targetBefore !== null && $targetAfterRecall !== null && (int)$targetAfterRecall['metal'] === (int)$targetBefore['metal'] && (int)$targetAfterRecall['crystal'] === (int)$targetBefore['crystal'], 'recalled transport does not deliver resources to target', array('before' => $targetBefore, 'after_recall' => $targetAfterRecall)),
            e2e_case($completedReturn, 'recalled transport return queue can be completed'),
            e2e_case($originBefore !== null && $originAfterReturn !== null && (int)$originAfterReturn['small_cargo'] === (int)$originBefore['small_cargo'], 'transport recall restores small cargo to origin', array('before' => $originBefore, 'after_return' => $originAfterReturn)),
            e2e_case($originBefore !== null && $originAfterReturn !== null && (int)$originAfterReturn['metal'] === (int)$originBefore['metal'] && (int)$originAfterReturn['crystal'] === (int)$originBefore['crystal'], 'transport recall returns loaded metal and crystal to origin', array('before' => $originBefore, 'after_return' => $originAfterReturn)),
            e2e_case(e2e_active_fleet_count($attackerId) === 0 && e2e_fleet_count($attackerId) === 0, 'transport recall lifecycle leaves no active fleet state'),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet);
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    $origin = LoadPlanetById($attackerPlanet);
    $target = LoadPlanetById($defenderPlanet);
    $sendResponse = e2e_send_fleet(
        $gameBase,
        $attackerSession,
        $attackerPlanet,
        e2e_fleet_payload($origin, $target, FTYP_TRANSPORT, array(GID_F_SC => 1), array(GID_RC_METAL => 100)),
        $attackerCookies
    );
    $outgoing = e2e_latest_fleet($attackerId, FTYP_TRANSPORT);
    if ($outgoing !== null) {
        e2e_age_fleet_queue((int)$outgoing['fleet_id'], 10);
        $response = e2e_recall_fleet($gameBase, $attackerSession, $attackerPlanet, (int)$outgoing['fleet_id'], $attackerCookies);
    } else {
        $response = array('status' => 0, 'location' => '', 'headers' => array(), 'body' => '');
    }
    $returnBefore = e2e_latest_fleet($attackerId, FTYP_TRANSPORT + FTYP_RETURN);
    $returnQueueBefore = $returnBefore === null ? null : e2e_queue_snapshot((int)$returnBefore['fleet_id']);
    $secondResponse = e2e_recall_fleet($gameBase, $attackerSession, $attackerPlanet, (int)($returnBefore['fleet_id'] ?? 0), $attackerCookies);
    $returnAfter = $returnBefore === null ? null : e2e_fleet_by_id((int)$returnBefore['fleet_id']);
    $returnQueueAfter = $returnBefore === null ? null : e2e_queue_snapshot((int)$returnBefore['fleet_id']);
    $returnFleetRowsAfterSecondRecall = $returnBefore === null ? 0 : e2e_fleet_row_count((int)$returnBefore['fleet_id']);
    $returnQueueRowsAfterSecondRecall = $returnBefore === null ? 0 : e2e_queue_row_count((int)$returnBefore['fleet_id']);
    $completedReturn = e2e_complete_fleet($returnAfter);
    $cases[] = e2e_finalize_case(array(
        'case' => 'recall_ignores_already_returning_fleet',
        'checks' => array_merge(e2e_response_check($sendResponse), e2e_response_check($response), e2e_response_check($secondResponse), array(
            e2e_case($returnBefore !== null && $returnAfter !== null, 'return fleet still exists after second recall POST', array('before' => $returnBefore, 'after' => $returnAfter)),
            e2e_case($returnAfter !== null && (int)$returnAfter['mission'] === FTYP_TRANSPORT + FTYP_RETURN, 'second recall keeps return mission unchanged', $returnAfter ?? array()),
            e2e_case($returnQueueBefore !== null && $returnQueueAfter !== null && (int)$returnQueueAfter['end'] === (int)$returnQueueBefore['end'], 'second recall does not replace or reschedule return queue', array('before' => $returnQueueBefore, 'after' => $returnQueueAfter)),
            e2e_case($returnBefore !== null && $returnFleetRowsAfterSecondRecall === 1 && $returnQueueRowsAfterSecondRecall === 1, 'second recall keeps exactly one queue-backed return fleet for the same id', array('fleet_rows' => $returnFleetRowsAfterSecondRecall, 'queue_rows' => $returnQueueRowsAfterSecondRecall)),
            e2e_case(strpos($secondResponse['body'], 'value="Recall"') === false, 'fleet page does not render a Recall button for returning fleet'),
            e2e_case($completedReturn, 'unchanged return fleet can still complete normally'),
        )),
    ));

    $completedFleetId = (int)($returnBefore['fleet_id'] ?? 0);
    $postCompleteResponse = e2e_recall_fleet($gameBase, $attackerSession, $attackerPlanet, $completedFleetId, $attackerCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'recall_ignores_completed_fleet_id_without_php_warning',
        'checks' => array_merge(e2e_response_check($postCompleteResponse), array(
            e2e_case($completedFleetId > 0, 'completed fleet id was captured before deletion', array('fleet_id' => $completedFleetId)),
            e2e_case(e2e_fleet_by_id($completedFleetId) === null, 'completed fleet id remains deleted after recall POST'),
            e2e_case(e2e_fleet_count($attackerId) === 0 && e2e_active_fleet_count($attackerId) === 0, 'completed fleet recall leaves no fleet state'),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    $home = LoadPlanetById($attackerPlanet);
    $deployTargetPlanet = e2e_create_planet_near($attackerId, $home);
    $createdPlanets[] = $deployTargetPlanet;
    e2e_prepare_planet($deployTargetPlanet, $attackerId, 0, 0, 0, 0);
    $origin = LoadPlanetById($attackerPlanet);
    $deployTarget = LoadPlanetById($deployTargetPlanet);
    $originBefore = e2e_planet_snapshot($attackerPlanet);
    $targetBefore = e2e_planet_snapshot($deployTargetPlanet);
    $sendResponse = e2e_send_fleet(
        $gameBase,
        $attackerSession,
        $attackerPlanet,
        e2e_fleet_payload($origin, $deployTarget, FTYP_DEPLOY, array(GID_F_SC => 2), array(GID_RC_METAL => 77, GID_RC_CRYSTAL => 22)),
        $attackerCookies
    );
    $deployFleet = e2e_latest_fleet($attackerId, FTYP_DEPLOY);
    if ($deployFleet !== null) {
        e2e_age_fleet_queue((int)$deployFleet['fleet_id'], 10);
    }
    $response = e2e_recall_fleet($gameBase, $attackerSession, $attackerPlanet, (int)($deployFleet['fleet_id'] ?? 0), $attackerCookies);
    $deployAfterRecall = $deployFleet === null ? null : e2e_fleet_by_id((int)$deployFleet['fleet_id']);
    $deployReturn = e2e_latest_fleet($attackerId, FTYP_DEPLOY + FTYP_RETURN);
    $targetAfterRecall = e2e_planet_snapshot($deployTargetPlanet);
    $completedDeployReturn = e2e_complete_fleet($deployReturn);
    $originAfterDeployReturn = e2e_planet_snapshot($attackerPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'deploy_recall_returns_ships_and_cargo_to_origin',
        'checks' => array_merge(e2e_response_check($sendResponse), e2e_response_check($response), array(
            e2e_case($deployFleet !== null, 'outbound deploy exists before recall', $deployFleet ?? array()),
            e2e_case($deployAfterRecall === null, 'deploy recall removes original outbound fleet'),
            e2e_case($deployReturn !== null && (int)$deployReturn['mission'] === FTYP_DEPLOY + FTYP_RETURN, 'deploy recall creates a deploy return fleet', $deployReturn ?? array()),
            e2e_case($targetBefore !== null && $targetAfterRecall !== null && (int)$targetAfterRecall['small_cargo'] === (int)$targetBefore['small_cargo'] && (int)$targetAfterRecall['metal'] === (int)$targetBefore['metal'] && (int)$targetAfterRecall['crystal'] === (int)$targetBefore['crystal'], 'recalled deploy does not unload ships or cargo on target', array('before' => $targetBefore, 'after_recall' => $targetAfterRecall)),
            e2e_case($completedDeployReturn, 'deploy return queue can be completed'),
            e2e_case($originBefore !== null && $originAfterDeployReturn !== null && (int)$originAfterDeployReturn['small_cargo'] === (int)$originBefore['small_cargo'], 'deploy recall restores small cargo to origin', array('before' => $originBefore, 'after_return' => $originAfterDeployReturn)),
            e2e_case($originBefore !== null && $originAfterDeployReturn !== null && (int)$originAfterDeployReturn['metal'] === (int)$originBefore['metal'] && (int)$originAfterDeployReturn['crystal'] === (int)$originBefore['crystal'], 'deploy recall returns loaded resources to origin', array('before' => $originBefore, 'after_return' => $originAfterDeployReturn)),
            e2e_case(e2e_active_fleet_count($attackerId) === 0 && e2e_fleet_count($attackerId) === 0, 'deploy recall lifecycle leaves no active fleet state'),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet, 10, 5, 10, 5);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet, 0, 0, 0, 0);
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    e2e_cleanup_relations(array($attackerId, $defenderId));
    $buddyId = AddBuddy($attackerId, $defenderId, 'E2E recall hold relation');
    if ($buddyId > 0) {
        AcceptBuddy($buddyId);
    }
    $origin = LoadPlanetById($attackerPlanet);
    $target = LoadPlanetById($defenderPlanet);
    $originBefore = e2e_planet_snapshot($attackerPlanet);
    $sendResponse = e2e_send_fleet(
        $gameBase,
        $attackerSession,
        $attackerPlanet,
        e2e_fleet_payload($origin, $target, FTYP_ACS_HOLD, array(GID_F_LF => 1), array(), array('holdingtime' => 1)),
        $attackerCookies
    );
    $holdFleet = e2e_latest_fleet($attackerId, FTYP_ACS_HOLD);
    $completedOutboundHold = e2e_complete_fleet($holdFleet);
    $orbitingFleet = e2e_latest_fleet($attackerId, FTYP_ACS_HOLD + FTYP_ORBITING);
    if ($orbitingFleet !== null) {
        e2e_age_fleet_queue((int)$orbitingFleet['fleet_id'], 10);
    }
    $holdingCountBeforeRecall = GetHoldingFleetsCount($defenderPlanet);
    $response = e2e_recall_fleet($gameBase, $attackerSession, $attackerPlanet, (int)($orbitingFleet['fleet_id'] ?? 0), $attackerCookies);
    $orbitAfterRecall = $orbitingFleet === null ? null : e2e_fleet_by_id((int)$orbitingFleet['fleet_id']);
    $holdReturn = e2e_latest_fleet($attackerId, FTYP_ACS_HOLD + FTYP_RETURN);
    $holdingCountAfterRecall = GetHoldingFleetsCount($defenderPlanet);
    $completedHoldReturn = e2e_complete_fleet($holdReturn);
    $originAfterHoldReturn = e2e_planet_snapshot($attackerPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'orbiting_hold_recall_returns_fleet_and_clears_hold_state',
        'checks' => array_merge(e2e_response_check($sendResponse), e2e_response_check($response), array(
            e2e_case($buddyId > 0 && IsBuddy($attackerId, $defenderId), 'buddy relation enables ACS hold fixture', array('buddy_id' => $buddyId)),
            e2e_case($holdFleet !== null && $completedOutboundHold, 'outbound hold reaches orbit before recall', $holdFleet ?? array()),
            e2e_case($orbitingFleet !== null && (int)$orbitingFleet['mission'] === FTYP_ACS_HOLD + FTYP_ORBITING, 'orbiting hold fleet exists before recall', $orbitingFleet ?? array()),
            e2e_case($holdingCountBeforeRecall >= 1, 'target reports holding fleet before orbit recall', array('holding_count' => $holdingCountBeforeRecall)),
            e2e_case($orbitAfterRecall === null, 'orbiting hold recall removes orbiting fleet row'),
            e2e_case($holdReturn !== null && (int)$holdReturn['mission'] === FTYP_ACS_HOLD + FTYP_RETURN, 'orbiting hold recall creates ACS hold return fleet', $holdReturn ?? array()),
            e2e_case($holdingCountAfterRecall === 0, 'target no longer reports holding fleets after orbit recall', array('holding_count' => $holdingCountAfterRecall)),
            e2e_case($completedHoldReturn, 'orbiting hold return queue can be completed'),
            e2e_case($originBefore !== null && $originAfterHoldReturn !== null && (int)$originAfterHoldReturn['light_fighter'] === (int)$originBefore['light_fighter'], 'orbiting hold recall restores fighter to origin', array('before' => $originBefore, 'after_return' => $originAfterHoldReturn)),
            e2e_case(e2e_active_fleet_count($attackerId) === 0 && e2e_fleet_count($attackerId) === 0, 'orbiting hold recall lifecycle leaves no active fleet state'),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'fleet_recall_edges_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    $userIds = array_filter(array($attackerId, $defenderId), fn($id) => $id > 0);
    $planetIds = array_filter(array_merge(array($attackerPlanet, $defenderPlanet), $createdPlanets), fn($id) => $id > 0);
    if (!empty($userIds) && !empty($planetIds)) {
        e2e_cleanup_fleets($userIds, $planetIds);
        e2e_cleanup_relations($userIds);
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
    'case_group' => 'http_fleet_recall_edges',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
