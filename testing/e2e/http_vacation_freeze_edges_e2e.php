<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_vacation_freeze_edges_e2e.php';
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
loca_add('options', 'en');
loca_add('build', 'en');
loca_add('fleet', 'en');
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

function e2e_prepare_session(int $userId, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', private_session='" . e2e_sql_escape($private) . "', " .
        "validated=1, deact_ip=1, lang='en', skin='/evolution/', useskin=1 WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookies' => array('prsess_' . $userId . '_' . $GlobalUni['num'] => $private),
    );
}

function e2e_user_row(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT player_id, oname, pemail, vacation, vacation_until, sortby, sortorder, maxspy, maxfleetmsg FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1");
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
        'settings_sort' => (string)$user['sortby'],
        'settings_order' => (string)$user['sortorder'],
        'noipcheck' => 'on',
        'spio_anz' => (string)$user['maxspy'],
        'settings_fleetactions' => (string)$user['maxfleetmsg'],
    );
    foreach ($overrides as $key => $value) {
        if ($value === null) {
            unset($payload[$key]);
        } else {
            $payload[$key] = $value;
        }
    }
    return $payload;
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

function e2e_reset_user_and_planet(int $userId, int $planetId, string $name): void
{
    global $db_prefix, $buildmap, $fleetmap, $defmap, $resmap;

    e2e_cleanup_fleets(array($userId), array($planetId));
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$userId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$userId} OR planet_id={$planetId}");

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=10";
    }
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", admin=" . USER_TYPE_PLAYER . ", validated=1, deact_ip=1, vacation=0, vacation_until=0, " .
        "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, " .
        "lastclick=" . time() . " WHERE player_id={$userId}"
    );

    $assignments = array();
    foreach (array_merge($buildmap, $fleetmap, $defmap) as $gid) {
        $assignments[] = "`{$gid}`=0";
    }
    dbquery(
        "UPDATE {$db_prefix}planets SET " . implode(',', $assignments) . ", name='" . e2e_sql_escape($name) . "', owner_id={$userId}, type=" . PTYP_PLANET . ", " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "`" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=12, `" . GID_B_METAL_MINE . "`=0, `" . GID_F_SC . "`=10, " .
        "prod1=1, prod2=1, prod3=1, prod4=1, prod12=1, prod212=1, fields=0, maxfields=300, lastpeek=" . time() . ", lastakt=" . time() . ", remove=0 " .
        "WHERE planet_id={$planetId}"
    );
    InvalidateUserCache();
}

function e2e_planet_row(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, `" . GID_B_METAL_MINE . "` AS metal_mine, `" . GID_F_SC . "` AS small_cargo, " .
        "`" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deuterium " .
        "FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1"
    );
}

function e2e_set_uni_freeze(int $freeze): void
{
    global $db_prefix, $GlobalUni;
    dbquery("UPDATE {$db_prefix}uni SET freeze={$freeze}");
    $GlobalUni = LoadUniverse();
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderPlanet = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$cases = array();
$uniSnapshot = LoadUniverse();

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    e2e_set_uni_freeze(0);
    e2e_reset_user_and_planet($attackerId, $attackerPlanet, 'E2E Vacation A');
    e2e_reset_user_and_planet($defenderId, $defenderPlanet, 'E2E Vacation D');
    $auth = e2e_prepare_session($attackerId, 'vacation-freeze-attacker');

    $buildError = BuildEnque(LoadUser($attackerId), $attackerPlanet, GID_B_METAL_MINE, 0, time());
    $buildQueueBefore = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "'");
    $response = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=options&session=' . rawurlencode($auth['session']) . '&mode=change',
        e2e_options_payload(e2e_user_row($attackerId), array('urlaubs_modus' => 'on')),
        $auth['cookies']
    );
    $userAfterBuildVacation = e2e_user_row($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'active_build_queue_rejects_vacation_enable',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($buildError === '' && $buildQueueBefore === 1, 'setup creates one active build queue', array('build_error' => $buildError, 'queue_count' => $buildQueueBefore)),
            e2e_case($userAfterBuildVacation !== null && (int)$userAfterBuildVacation['vacation'] === 0, 'vacation remains disabled while build queue is active', $userAfterBuildVacation ?? array()),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet, 'E2E Vacation A');
    e2e_reset_user_and_planet($defenderId, $defenderPlanet, 'E2E Vacation D');
    $fleet = array();
    foreach ($fleetmap as $gid) {
        $fleet[$gid] = 0;
    }
    $fleet[GID_F_SC] = 1;
    $resources = array();
    foreach ($transportableResources as $rc) {
        $resources[$rc] = 0;
    }
    AdjustShips($fleet, $attackerPlanet, '-');
    $fleetId = DispatchFleet($fleet, LoadPlanetById($attackerPlanet), LoadPlanetById($defenderPlanet), FTYP_TRANSPORT, 600, $resources, 0, time());
    $fleetQueueBefore = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_FLEET . "'");
    $response = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=options&session=' . rawurlencode($auth['session']) . '&mode=change',
        e2e_options_payload(e2e_user_row($attackerId), array('urlaubs_modus' => 'on')),
        $auth['cookies']
    );
    $userAfterFleetVacation = e2e_user_row($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'active_fleet_queue_rejects_vacation_enable',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($fleetId > 0 && $fleetQueueBefore === 1, 'setup creates one active fleet queue', array('fleet_id' => $fleetId, 'queue_count' => $fleetQueueBefore)),
            e2e_case($userAfterFleetVacation !== null && (int)$userAfterFleetVacation['vacation'] === 0, 'vacation remains disabled while fleet is in flight', $userAfterFleetVacation ?? array()),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet, 'E2E Vacation A');
    dbquery("UPDATE {$db_prefix}users SET vacation=1, vacation_until=" . (time() + 3600) . " WHERE player_id={$attackerId}");
    InvalidateUserCache();
    $beforeVacationMutation = e2e_planet_row($attackerPlanet);
    $responseBuild = e2e_http_request('GET', $gameBase . '/index.php?page=b_building&session=' . rawurlencode($auth['session']) . '&modus=add&techid=' . GID_B_METAL_MINE . '&planet=' . $attackerPlanet, array(), $auth['cookies']);
    $responseShipyard = e2e_http_request('POST', $gameBase . '/index.php?page=buildings&mode=Flotte&session=' . rawurlencode($auth['session']) . '&cp=' . $attackerPlanet, array(
        'fmenge' => array(GID_F_SC => '3'),
    ), $auth['cookies']);
    $afterVacationMutation = e2e_planet_row($attackerPlanet);
    $buildRowsAfterVacation = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}buildqueue WHERE owner_id={$attackerId} OR planet_id={$attackerPlanet}");
    $shipyardRowsAfterVacation = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_SHIPYARD . "'");
    $cases[] = e2e_finalize_case(array(
        'case' => 'vacation_mode_blocks_building_and_shipyard_mutations',
        'checks' => array_merge(e2e_response_check($responseBuild), e2e_response_check($responseShipyard), array(
            e2e_case($buildRowsAfterVacation === 0, 'vacation mode does not enqueue building work', array('buildqueue_count' => $buildRowsAfterVacation)),
            e2e_case($shipyardRowsAfterVacation === 0, 'vacation mode does not enqueue shipyard work', array('shipyard_queue_count' => $shipyardRowsAfterVacation)),
            e2e_case($beforeVacationMutation !== null && $afterVacationMutation !== null && (int)$afterVacationMutation['small_cargo'] === (int)$beforeVacationMutation['small_cargo'], 'ship counts stay unchanged after blocked shipyard POST', array('before' => $beforeVacationMutation, 'after' => $afterVacationMutation)),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet, 'E2E Vacation A');
    $buildError = BuildEnque(LoadUser($attackerId), $attackerPlanet, GID_B_METAL_MINE, 0, time() - 120);
    $queueRow = e2e_one_row("SELECT task_id, sub_id FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "' ORDER BY task_id DESC LIMIT 1");
    if ($queueRow !== null) {
        dbquery("UPDATE {$db_prefix}queue SET start=" . (time() - 120) . ", end=" . (time() - 30) . " WHERE task_id=" . (int)$queueRow['task_id']);
        dbquery("UPDATE {$db_prefix}buildqueue SET start=" . (time() - 120) . ", end=" . (time() - 30) . " WHERE id=" . (int)$queueRow['sub_id']);
    }
    e2e_set_uni_freeze(1);
    UpdateQueue(time());
    $whileFrozenPlanet = e2e_planet_row($attackerPlanet);
    $whileFrozenQueue = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "'");
    e2e_set_uni_freeze(0);
    UpdateQueue(time());
    $afterUnfreezePlanet = e2e_planet_row($attackerPlanet);
    $afterUnfreezeQueue = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "'");
    $cases[] = e2e_finalize_case(array(
        'case' => 'universe_freeze_pauses_due_build_queue_until_unfrozen',
        'checks' => array(
            e2e_case($buildError === '' && $queueRow !== null, 'setup creates a due build queue before freezing', array('build_error' => $buildError, 'queue' => $queueRow ?? array())),
            e2e_case($whileFrozenPlanet !== null && (int)$whileFrozenPlanet['metal_mine'] === 0 && $whileFrozenQueue === 1, 'due build queue remains pending while universe is frozen', array('planet' => $whileFrozenPlanet ?? array(), 'queue_count' => $whileFrozenQueue)),
            e2e_case($afterUnfreezePlanet !== null && (int)$afterUnfreezePlanet['metal_mine'] === 1 && $afterUnfreezeQueue === 0, 'due build queue completes after universe freeze is disabled', array('planet' => $afterUnfreezePlanet ?? array(), 'queue_count' => $afterUnfreezeQueue)),
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'vacation_freeze_edges_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e), 'trace' => $e->getTraceAsString()))),
        'pass' => false,
    );
} finally {
    if (isset($uniSnapshot['freeze'])) {
        e2e_set_uni_freeze((int)$uniSnapshot['freeze']);
    }
    $userIds = array_filter(array($attackerId, $defenderId), fn($id) => $id > 0);
    $planetIds = array_filter(array($attackerPlanet, $defenderPlanet), fn($id) => $id > 0);
    if (!empty($userIds) && !empty($planetIds)) {
        e2e_cleanup_fleets($userIds, $planetIds);
        foreach ($userIds as $userId) {
            dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$userId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "')");
        }
        foreach ($planetIds as $planetId) {
            dbquery("DELETE FROM {$db_prefix}buildqueue WHERE planet_id={$planetId}");
        }
        if ($attackerId > 0 && $attackerPlanet > 0) {
            e2e_reset_user_and_planet($attackerId, $attackerPlanet, 'E2E Fixture Home');
            SelectPlanet($attackerId, $attackerPlanet);
        }
        if ($defenderId > 0 && $defenderPlanet > 0) {
            e2e_reset_user_and_planet($defenderId, $defenderPlanet, 'E2E Fixture Defender');
        }
    }
}

echo json_encode(array(
    'case_group' => 'http_vacation_freeze_edges',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
