<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_queue_fleet_e2e.php';
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
loca_add('options', 'en');
loca_add('admin', 'en');

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

function e2e_prepare_session(int $userId, string $label, int $admin = USER_TYPE_PLAYER): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', admin={$admin}, " .
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

function e2e_options_payload(array $user): array
{
    return array(
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
        'urlaubs_modus' => 'on',
    );
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

function e2e_active_fleet_count(int $userId): int
{
    global $db_prefix;
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$userId} AND type='" . QTYP_FLEET . "'");
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

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet);
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));

    $auth = e2e_prepare_session($attackerId, 'queue-fleet-attacker');
    $cookies = $auth['cookies'];
    $session = $auth['session'];

    $before = e2e_one_row("SELECT `" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=b_building&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&modus=add&techid=' . GID_B_METAL_MINE . '&planet=' . $attackerPlanet, array(), $cookies);
    $buildRow = e2e_one_row("SELECT id, list_id, tech_id, level FROM {$db_prefix}buildqueue WHERE owner_id={$attackerId} AND planet_id={$attackerPlanet} ORDER BY id DESC LIMIT 1");
    $buildTask = e2e_one_row("SELECT task_id, type, sub_id FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "' ORDER BY task_id DESC LIMIT 1");
    $responseCancel = e2e_http_request('GET', $gameBase . '/index.php?page=b_building&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&modus=remove&listid=1&planet=' . $attackerPlanet, array(), $cookies);
    $afterCancel = e2e_one_row("SELECT `" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'building_enqueue_and_cancel_refunds',
        'checks' => array_merge(e2e_response_check($response), e2e_response_check($responseCancel), array(
            e2e_case($buildRow !== null && (int)$buildRow['tech_id'] === GID_B_METAL_MINE, 'building queue row is created for metal mine', $buildRow ?? array()),
            e2e_case($buildTask !== null && (int)$buildTask['sub_id'] === (int)$buildRow['id'], 'global build queue task points at buildqueue row', $buildTask ?? array()),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}buildqueue WHERE owner_id={$attackerId}") === 0, 'buildqueue row is removed after cancel'),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "'") === 0, 'global build queue task is removed after cancel'),
            e2e_case($before !== null && $afterCancel !== null && (int)$afterCancel['metal'] >= (int)$before['metal'] && (int)$afterCancel['crystal'] >= (int)$before['crystal'], 'cancel returns first-slot construction resources', array('before' => $before, 'after' => $afterCancel)),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=b_building&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&modus=add&techid=' . GID_B_CRYS_MINE . '&planet=' . $attackerPlanet, array(), $cookies);
    $buildTask = e2e_one_row("SELECT task_id FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "' ORDER BY task_id DESC LIMIT 1");
    if ($buildTask !== null) {
        e2e_force_complete_queue((int)$buildTask['task_id']);
    }
    $planetAfterBuild = e2e_one_row("SELECT planet_id, `" . GID_B_CRYS_MINE . "` AS crystal_mine, fields FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'building_queue_completion',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($buildTask !== null, 'build completion case creates a queue task', $buildTask ?? array()),
            e2e_case($planetAfterBuild !== null && (int)$planetAfterBuild['crystal_mine'] === 1, 'completed build increments building level', $planetAfterBuild ?? array()),
            e2e_case($planetAfterBuild !== null && (int)$planetAfterBuild['fields'] === 1, 'completed build increments used fields', $planetAfterBuild ?? array()),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "'") === 0, 'completed build queue task is removed'),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}buildqueue WHERE owner_id={$attackerId}") === 0, 'completed buildqueue row is removed'),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    dbquery("UPDATE {$db_prefix}users SET `" . GID_R_ESPIONAGE . "`=0 WHERE player_id={$attackerId}");
    $beforeResearchCancel = e2e_one_row("SELECT `" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deut FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=buildings&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&mode=Forschung&bau=' . GID_R_ESPIONAGE, array(), $cookies);
    $researchTask = e2e_one_row("SELECT task_id, obj_id, level FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_RESEARCH . "' ORDER BY task_id DESC LIMIT 1");
    $responseCancel = e2e_http_request('GET', $gameBase . '/index.php?page=buildings&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&mode=Forschung&unbau=' . GID_R_ESPIONAGE, array(), $cookies);
    $afterResearchCancel = e2e_one_row("SELECT `" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deut FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'research_enqueue_and_cancel_refunds',
        'checks' => array_merge(e2e_response_check($response), e2e_response_check($responseCancel), array(
            e2e_case($researchTask !== null && (int)$researchTask['obj_id'] === GID_R_ESPIONAGE && (int)$researchTask['level'] === 1, 'research queue task is created for espionage level 1', $researchTask ?? array()),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_RESEARCH . "'") === 0, 'research queue task is removed after cancel'),
            e2e_case($beforeResearchCancel !== null && $afterResearchCancel !== null && (int)$afterResearchCancel['crystal'] >= (int)$beforeResearchCancel['crystal'], 'research cancel returns crystal cost', array('before' => $beforeResearchCancel, 'after' => $afterResearchCancel)),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    dbquery("UPDATE {$db_prefix}users SET `" . GID_R_ESPIONAGE . "`=0 WHERE player_id={$attackerId}");
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=buildings&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&mode=Forschung&bau=' . GID_R_ESPIONAGE, array(), $cookies);
    $researchTask = e2e_one_row("SELECT task_id FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_RESEARCH . "' ORDER BY task_id DESC LIMIT 1");
    if ($researchTask !== null) {
        e2e_force_complete_queue((int)$researchTask['task_id']);
    }
    $userAfterResearch = e2e_one_row("SELECT player_id, `" . GID_R_ESPIONAGE . "` AS espionage FROM {$db_prefix}users WHERE player_id={$attackerId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'research_queue_completion',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($researchTask !== null, 'research completion case creates a queue task', $researchTask ?? array()),
            e2e_case($userAfterResearch !== null && (int)$userAfterResearch['espionage'] === 1, 'completed research increments user research level', $userAfterResearch ?? array()),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_RESEARCH . "'") === 0, 'completed research queue task is removed'),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    dbquery("UPDATE {$db_prefix}planets SET `" . GID_F_SC . "`=0 WHERE planet_id={$attackerPlanet}");
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=buildings&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&mode=Flotte', array(
        'fmenge' => array(GID_F_SC => 3),
    ), $cookies);
    $shipyardTask = e2e_one_row("SELECT task_id, obj_id, level FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_SHIPYARD . "' ORDER BY task_id DESC LIMIT 1");
    if ($shipyardTask !== null) {
        e2e_force_complete_queue((int)$shipyardTask['task_id']);
    }
    $planetAfterShipyard = e2e_one_row("SELECT planet_id, `" . GID_F_SC . "` AS small_cargo FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'shipyard_queue_completion',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($shipyardTask !== null && (int)$shipyardTask['obj_id'] === GID_F_SC && (int)$shipyardTask['level'] === 3, 'shipyard queue task is created for three small cargos', $shipyardTask ?? array()),
            e2e_case($planetAfterShipyard !== null && (int)$planetAfterShipyard['small_cargo'] === 3, 'completed shipyard queue adds ships to planet', $planetAfterShipyard ?? array()),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_SHIPYARD . "'") === 0, 'completed shipyard queue task is removed'),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=b_building&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&modus=add&techid=' . GID_B_METAL_MINE . '&planet=' . $attackerPlanet, array(), $cookies);
    $user = LoadUser($attackerId);
    $responseVacation = e2e_http_request('POST', $gameBase . '/index.php?page=options&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&mode=change', e2e_options_payload($user), $cookies);
    $userAfterVacationAttempt = e2e_one_row("SELECT player_id, vacation, vacation_until FROM {$db_prefix}users WHERE player_id={$attackerId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'active_queue_blocks_vacation_mode',
        'checks' => array_merge(e2e_response_check($response), e2e_response_check($responseVacation), array(
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "'") === 1, 'active build queue exists before vacation attempt'),
            e2e_case($userAfterVacationAttempt !== null && (int)$userAfterVacationAttempt['vacation'] === 0 && (int)$userAfterVacationAttempt['vacation_until'] === 0, 'vacation mode is not enabled while account has active queue', $userAfterVacationAttempt ?? array()),
        )),
    ));

    $adminAuth = e2e_prepare_session($attackerId, 'queue-admin', USER_TYPE_ADMIN);
    $adminCookies = $adminAuth['cookies'];
    $taskId = AddQueue($attackerId, QTYP_DEBUG, 0, 0, 0, time(), 3600, QUEUE_PRIO_DEBUG);
    $responseFreeze = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Queue', array('order_freeze' => $taskId), $adminCookies);
    $frozenTask = e2e_one_row("SELECT task_id, freeze, frozen FROM {$db_prefix}queue WHERE task_id={$taskId} LIMIT 1");
    $responseUnfreeze = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Queue', array('order_unfreeze' => $taskId), $adminCookies);
    $unfrozenTask = e2e_one_row("SELECT task_id, freeze, frozen FROM {$db_prefix}queue WHERE task_id={$taskId} LIMIT 1");
    $responseRemove = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Queue', array('order_remove' => $taskId), $adminCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_queue_freeze_unfreeze_remove',
        'checks' => array_merge(e2e_response_check($responseFreeze), e2e_response_check($responseUnfreeze), e2e_response_check($responseRemove), array(
            e2e_case($frozenTask !== null && (int)$frozenTask['freeze'] === 1 && (int)$frozenTask['frozen'] > 0, 'admin queue freeze marks task frozen', $frozenTask ?? array()),
            e2e_case($unfrozenTask !== null && (int)$unfrozenTask['freeze'] === 0 && (int)$unfrozenTask['frozen'] === 0, 'admin queue unfreeze clears frozen state', $unfrozenTask ?? array()),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE task_id={$taskId}") === 0, 'admin queue remove deletes task'),
        )),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet);
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    $auth = e2e_prepare_session($attackerId, 'fleet-attacker');
    $cookies = $auth['cookies'];
    $session = $auth['session'];
    $origin = LoadPlanetById($attackerPlanet);
    $target = LoadPlanetById($defenderPlanet);

    $beforeFleetCount = e2e_active_fleet_count($attackerId);
    $originBeforeFleet = e2e_one_row("SELECT planet_id, `" . GID_F_SC . "` AS small_cargo FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");
    $payload = e2e_fleet_payload($origin, $target, FTYP_TRANSPORT, array(GID_F_SC => 1), array(GID_RC_METAL => 123, GID_RC_CRYSTAL => 45));
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, $payload, $cookies);
    $fleet = e2e_one_row("SELECT fleet_id, owner_id, mission, start_planet, target_planet, `" . GID_F_SC . "` AS small_cargo, `" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal FROM {$db_prefix}fleet WHERE owner_id={$attackerId} ORDER BY fleet_id DESC LIMIT 1");
    $originAfterFleet = e2e_one_row("SELECT planet_id, `" . GID_F_SC . "` AS small_cargo FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'fleet_transport_success_creates_queue',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($fleet !== null && (int)$fleet['mission'] === FTYP_TRANSPORT, 'transport fleet row is created', $fleet ?? array()),
            e2e_case($fleet !== null && (int)$fleet['start_planet'] === $attackerPlanet && (int)$fleet['target_planet'] === $defenderPlanet, 'transport fleet has expected route', $fleet ?? array()),
            e2e_case($fleet !== null && (int)$fleet['small_cargo'] === 1 && (int)$fleet['metal'] === 123 && (int)$fleet['crystal'] === 45, 'transport fleet stores ships and cargo', $fleet ?? array()),
            e2e_case(e2e_active_fleet_count($attackerId) === $beforeFleetCount + 1, 'transport creates one active fleet queue task'),
            e2e_case($originBeforeFleet !== null && $originAfterFleet !== null && (int)$originAfterFleet['small_cargo'] === (int)$originBeforeFleet['small_cargo'] - 1, 'origin ship count is debited on launch', array('before' => $originBeforeFleet, 'after' => $originAfterFleet)),
        )),
    ));
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));

    $failureSpecs = array(
        'fleet_no_ships_rejected' => array(
            'target' => $target,
            'payload' => e2e_fleet_payload($origin, $target, FTYP_TRANSPORT, array()),
            'before' => function (): void {},
            'after' => function (): void {},
        ),
        'fleet_same_planet_rejected' => array(
            'target' => $origin,
            'payload' => e2e_fleet_payload($origin, $origin, FTYP_TRANSPORT, array(GID_F_SC => 1)),
            'before' => function (): void {},
            'after' => function (): void {},
        ),
        'fleet_deploy_to_foreign_rejected' => array(
            'target' => $target,
            'payload' => e2e_fleet_payload($origin, $target, FTYP_DEPLOY, array(GID_F_SC => 1)),
            'before' => function (): void {},
            'after' => function (): void {},
        ),
        'fleet_target_vacation_rejected' => array(
            'target' => $target,
            'payload' => e2e_fleet_payload($origin, $target, FTYP_TRANSPORT, array(GID_F_SC => 1)),
            'before' => function () use ($db_prefix, $defenderId): void {
                dbquery("UPDATE {$db_prefix}users SET vacation=1, vacation_until=" . (time() + 3600) . " WHERE player_id={$defenderId}");
                InvalidateUserCache();
            },
            'after' => function () use ($db_prefix, $defenderId): void {
                dbquery("UPDATE {$db_prefix}users SET vacation=0, vacation_until=0 WHERE player_id={$defenderId}");
                InvalidateUserCache();
            },
        ),
        'fleet_insufficient_fuel_rejected' => array(
            'target' => $target,
            'payload' => e2e_fleet_payload($origin, $target, FTYP_TRANSPORT, array(GID_F_SC => 1)),
            'before' => function () use ($db_prefix, $attackerPlanet): void {
                dbquery("UPDATE {$db_prefix}planets SET `" . GID_RC_DEUTERIUM . "`=0 WHERE planet_id={$attackerPlanet}");
            },
            'after' => function () use ($db_prefix, $attackerPlanet): void {
                dbquery("UPDATE {$db_prefix}planets SET `" . GID_RC_DEUTERIUM . "`=10000000 WHERE planet_id={$attackerPlanet}");
            },
        ),
        'fleet_invalid_coordinates_rejected' => array(
            'target' => array('g' => $GlobalUni['galaxies'] + 1, 's' => $GlobalUni['systems'] + 1, 'p' => 99, 'game_type' => GAME_PTYP_PLANET),
            'payload' => e2e_fleet_payload($origin, array('g' => $GlobalUni['galaxies'] + 1, 's' => $GlobalUni['systems'] + 1, 'p' => 99, 'game_type' => GAME_PTYP_PLANET), FTYP_TRANSPORT, array(GID_F_SC => 1)),
            'before' => function (): void {},
            'after' => function (): void {},
        ),
    );

    foreach ($failureSpecs as $caseName => $spec) {
        e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
        e2e_reset_user_and_planet($attackerId, $attackerPlanet);
        $origin = LoadPlanetById($attackerPlanet);
        $spec['before']();
        $before = e2e_active_fleet_count($attackerId);
        $response = e2e_http_request('POST', $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, $spec['payload'], $cookies);
        $after = e2e_active_fleet_count($attackerId);
        $fleetRows = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id={$attackerId}");
        $spec['after']();
        $cases[] = e2e_finalize_case(array(
            'case' => $caseName,
            'checks' => array_merge(e2e_response_check($response), array(
                e2e_case($after === $before, 'rejected fleet action does not create a fleet queue task', array('before' => $before, 'after' => $after)),
                e2e_case($fleetRows === 0, 'rejected fleet action does not create a fleet row', array('fleet_rows' => $fleetRows)),
                e2e_case(stripos($response['body'], 'error') !== false || stripos($response['body'], 'Cheater') !== false, 'rejected fleet action renders an error response'),
            )),
        ));
    }
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'queue_fleet_exception',
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
}

echo json_encode(array(
    'case_group' => 'http_queue_fleet',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
