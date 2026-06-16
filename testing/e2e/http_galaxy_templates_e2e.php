<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_galaxy_templates_e2e.php';
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
loca_add('galaxy', 'en');
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
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined|Notice:\s+Trying/i', $body) === 1;
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

function e2e_cleanup_runtime(array $userIds, array $planetIds, array $extraPlanetIds = array()): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', $userIds));
    $planetList = implode(',', array_map('intval', array_merge($planetIds, $extraPlanetIds)));

    if ($userList !== '') {
        dbquery("DELETE FROM {$db_prefix}template WHERE owner_id IN ({$userList})");
        dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$userList})");
    }

    if ($planetList !== '') {
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

        dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id IN ({$userList}) OR planet_id IN ({$planetList})");
    }

    if ($userList !== '') {
        dbquery(
            "DELETE FROM {$db_prefix}queue WHERE owner_id IN ({$userList}) AND type IN ('" .
            QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" .
            QTYP_SHIPYARD . "','" . QTYP_FLEET . "','" . QTYP_DEBUG . "')"
        );
    }

    if (!empty($extraPlanetIds)) {
        $extraList = implode(',', array_map('intval', $extraPlanetIds));
        dbquery("DELETE FROM {$db_prefix}planets WHERE planet_id IN ({$extraList})");
    }
}

function e2e_reset_user(int $userId, bool $commander, int $computerLevel = 10): void
{
    global $db_prefix, $resmap;

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=10";
    }
    $research[] = "`" . GID_R_COMPUTER . "`={$computerLevel}";

    $now = time();
    $commanderUntil = $commander ? $now + 7 * 24 * 60 * 60 : 0;
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", " .
        "admin=0, ally_id=0, allyrank=0, validated=1, deact_ip=1, vacation=0, vacation_until=0, " .
        "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, " .
        "lang='en', skin='/evolution/', useskin=1, score1=10000, score2=0, score3=0, " .
        "place1=1, place2=1, place3=1, flags=" . USER_FLAG_DEFAULT . ", lastclick={$now}, " .
        "com_until={$commanderUntil}, adm_until=0, eng_until=0, geo_until=0, tec_until=0 " .
        "WHERE player_id={$userId}"
    );
    InvalidateUserCache();
}

function e2e_prepare_planet(int $planetId, int $ownerId, int $g, int $s, int $p, string $name): void
{
    global $db_prefix, $fleetmap, $defmap, $buildmap;

    $assignments = array();
    foreach (array_merge($fleetmap, $defmap, $buildmap) as $gid) {
        $assignments[] = "`{$gid}`=0";
    }

    $now = time();
    dbquery(
        "UPDATE {$db_prefix}planets SET " . implode(',', $assignments) . ", " .
        "name='" . e2e_sql_escape($name) . "', type=" . PTYP_PLANET . ", g={$g}, s={$s}, p={$p}, owner_id={$ownerId}, " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "`" . GID_B_SOLAR . "`=20, `" . GID_B_SHIPYARD . "`=12, `" . GID_B_MISS_SILO . "`=6, " .
        "`" . GID_F_SC . "`=10, `" . GID_F_LC . "`=5, `" . GID_F_PROBE . "`=8, `" . GID_F_RECYCLER . "`=4, " .
        "`" . GID_D_IPM . "`=4, prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, " .
        "fields=0, maxfields=300, lastpeek={$now}, lastakt={$now}, remove=0 WHERE planet_id={$planetId}"
    );
}

function e2e_find_open_pair(): array
{
    global $GlobalUni, $db_prefix;

    for ($g = 1; $g <= (int)$GlobalUni['galaxies']; $g++) {
        for ($s = 1; $s <= (int)$GlobalUni['systems']; $s++) {
            $free = array();
            for ($p = 1; $p <= 15; $p++) {
                $used = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}planets WHERE g={$g} AND s={$s} AND p={$p}");
                if ($used === 0) {
                    $free[] = $p;
                }
                if (count($free) >= 2) {
                    return array('g' => $g, 's' => $s, 'attacker_p' => $free[0], 'defender_p' => $free[1]);
                }
            }
        }
    }

    throw new RuntimeException('No open same-system planet pair is available for galaxy E2E fixtures.');
}

function e2e_template_payload(string $name, int $templateId, array $ships): array
{
    global $fleetmap;

    $payload = array(
        'mode' => 'save',
        'template_id' => (string)$templateId,
        'template_name' => $name,
        'ship' => array(),
    );
    foreach (array_diff($fleetmap, array(GID_F_SAT)) as $gid) {
        $payload['ship'][$gid] = (string)($ships[$gid] ?? 0);
    }
    return $payload;
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
$extraPlanets = array();

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    e2e_cleanup_runtime(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    $coords = e2e_find_open_pair();
    e2e_reset_user($attackerId, false, 1);
    e2e_reset_user($defenderId, false, 10);
    e2e_prepare_planet($attackerPlanet, $attackerId, $coords['g'], $coords['s'], $coords['attacker_p'], 'E2E Alpha');
    e2e_prepare_planet($defenderPlanet, $defenderId, $coords['g'], $coords['s'], $coords['defender_p'], 'E2E Target');

    $nonCommanderAuth = e2e_prepare_session($attackerId, 'galaxy-template-non-commander');
    $nonCommanderCookies = $nonCommanderAuth['cookies'];
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=fleet_templates&session=' . rawurlencode($nonCommanderAuth['session']) . '&cp=' . $attackerPlanet, array(), $nonCommanderCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'fleet_templates_require_commander',
        'checks' => array_merge(e2e_response_check($response, 'fleet template non-commander request'), array(
            e2e_case(stripos($response['location'], 'page=overview') !== false || stripos($response['body'], 'page=overview') !== false, 'non-commander fleet template page redirects to overview', array('location' => $response['location'])),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}template WHERE owner_id={$attackerId}") === 0, 'non-commander visit does not create templates'),
        )),
    ));

    e2e_reset_user($attackerId, true, 1);
    $commanderAuth = e2e_prepare_session($attackerId, 'galaxy-template-commander');
    $commanderCookies = $commanderAuth['cookies'];
    $session = $commanderAuth['session'];

    $responseCreateA = e2e_http_request('POST', $gameBase . '/index.php?page=fleet_templates&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, e2e_template_payload('E2E Scout', 0, array(
        GID_F_SC => 2,
        GID_F_PROBE => 4,
        GID_F_RECYCLER => 1,
    )), $commanderCookies);
    $firstTemplate = e2e_one_row("SELECT id, name, `" . GID_F_SC . "` AS small_cargo, `" . GID_F_PROBE . "` AS probes, `" . GID_F_RECYCLER . "` AS recyclers FROM {$db_prefix}template WHERE owner_id={$attackerId} AND name='E2E Scout' ORDER BY id DESC LIMIT 1");

    $responseCreateB = e2e_http_request('POST', $gameBase . '/index.php?page=fleet_templates&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, e2e_template_payload('E2E Cargo', 0, array(
        GID_F_LC => 1,
        GID_F_SC => 3,
    )), $commanderCookies);
    $responseOverflow = e2e_http_request('POST', $gameBase . '/index.php?page=fleet_templates&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, e2e_template_payload('E2E Overflow', 0, array(
        GID_F_LF => 9,
    )), $commanderCookies);
    $templateCountAfterOverflow = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}template WHERE owner_id={$attackerId}");

    $responseUpdate = e2e_http_request('POST', $gameBase . '/index.php?page=fleet_templates&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, e2e_template_payload('E2E Updated', (int)($firstTemplate['id'] ?? 0), array(
        GID_F_SC => 5,
        GID_F_PROBE => 1,
    )), $commanderCookies);
    $updatedTemplate = e2e_one_row("SELECT id, name, `" . GID_F_SC . "` AS small_cargo, `" . GID_F_PROBE . "` AS probes, `" . GID_F_RECYCLER . "` AS recyclers FROM {$db_prefix}template WHERE id=" . (int)($firstTemplate['id'] ?? 0) . " LIMIT 1");

    $defenderAuth = e2e_prepare_session($defenderId, 'galaxy-template-defender');
    $defenderCookies = $defenderAuth['cookies'];
    $responseForeignDelete = e2e_http_request('GET', $gameBase . '/index.php?page=fleet_templates&session=' . rawurlencode($defenderAuth['session']) . '&cp=' . $defenderPlanet . '&mode=delete&id=' . (int)($firstTemplate['id'] ?? 0), array(), $defenderCookies);
    $existsAfterForeignDelete = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}template WHERE id=" . (int)($firstTemplate['id'] ?? 0));

    $responseFlotten1 = e2e_http_request('GET', $gameBase . '/index.php?page=flotten1&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&galaxy=' . $coords['g'] . '&system=' . $coords['s'] . '&planet=' . $coords['defender_p'] . '&planettype=1&target_mission=3', array(), $commanderCookies);
    $responseDelete = e2e_http_request('GET', $gameBase . '/index.php?page=fleet_templates&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&mode=delete&id=' . (int)($firstTemplate['id'] ?? 0), array(), $commanderCookies);
    $existsAfterOwnerDelete = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}template WHERE id=" . (int)($firstTemplate['id'] ?? 0));

    $cases[] = e2e_finalize_case(array(
        'case' => 'fleet_template_crud_limit_and_flotten1_integration',
        'checks' => array_merge(
            e2e_response_check($responseCreateA, 'first fleet template create'),
            e2e_response_check($responseCreateB, 'second fleet template create'),
            e2e_response_check($responseOverflow, 'overflow fleet template create'),
            e2e_response_check($responseUpdate, 'fleet template update'),
            e2e_response_check($responseForeignDelete, 'foreign fleet template delete'),
            e2e_response_check($responseFlotten1, 'flotten1 template render'),
            e2e_response_check($responseDelete, 'owner fleet template delete'),
            array(
                e2e_case($firstTemplate !== null && (int)$firstTemplate['small_cargo'] === 2 && (int)$firstTemplate['probes'] === 4 && (int)$firstTemplate['recyclers'] === 1, 'created template stores selected ship counts', $firstTemplate ?? array()),
                e2e_case($templateCountAfterOverflow === 2, 'template count is capped by computer technology plus one', array('count' => $templateCountAfterOverflow)),
                e2e_case($updatedTemplate !== null && $updatedTemplate['name'] === 'E2E Updated' && (int)$updatedTemplate['small_cargo'] === 5 && (int)$updatedTemplate['probes'] === 1 && (int)$updatedTemplate['recyclers'] === 0, 'owner can update an existing template and clear omitted ships', $updatedTemplate ?? array()),
                e2e_case($existsAfterForeignDelete === 1, 'another player cannot delete the owner template', array('remaining' => $existsAfterForeignDelete)),
                e2e_case(stripos($responseFlotten1['body'], 'fleet_templates') !== false && stripos($responseFlotten1['body'], 'E2E Updated') !== false && stripos($responseFlotten1['body'], 'javascript:setShips') !== false, 'flotten1 renders commander template picker and saved template'),
                e2e_case($existsAfterOwnerDelete === 0, 'owner delete removes the selected template', array('remaining' => $existsAfterOwnerDelete)),
            )
        ),
    ));

    e2e_cleanup_runtime(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    e2e_reset_user($attackerId, true, 10);
    e2e_reset_user($defenderId, false, 10);
    e2e_prepare_planet($attackerPlanet, $attackerId, $coords['g'], $coords['s'], $coords['attacker_p'], 'E2E Alpha');
    e2e_prepare_planet($defenderPlanet, $defenderId, $coords['g'], $coords['s'], $coords['defender_p'], 'E2E Target');
    $moonId = CreatePlanet($coords['g'], $coords['s'], $coords['defender_p'], $defenderId, 1, 1, 20);
    if ($moonId > 0) {
        $extraPlanets[] = $moonId;
    }
    $debrisId = CreateDebris($coords['g'], $coords['s'], $coords['defender_p'], $defenderId);
    if ($debrisId > 0) {
        $extraPlanets[] = $debrisId;
        AddDebris($debrisId, 5000, 3000);
    }
    $spyReportId = SendMessage($attackerId, 'E2E spy', 'E2E spy report', '<table><tr><th>E2E spy report body</th></tr></table>', MTYP_SPY_REPORT, time(), $defenderPlanet);
    $auth = e2e_prepare_session($attackerId, 'galaxy-actions');
    $cookies = $auth['cookies'];
    $session = $auth['session'];

    $galaxyUrl = $gameBase . '/index.php?page=galaxy&no_header=1&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&galaxy=' . $coords['g'] . '&system=' . $coords['s'];
    $responseGalaxy = e2e_http_request('GET', $galaxyUrl, array(), $cookies);
    $body = $responseGalaxy['body'];
    $ipmFormUrl = $gameBase . '/index.php?page=galaxy&no_header=1&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&mode=1&p1=' . $coords['g'] . '&p2=' . $coords['s'] . '&p3=' . $coords['defender_p'] . '&pdd=' . $defenderPlanet . '&zp=' . $defenderId;
    $responseIpmForm = e2e_http_request('GET', $ipmFormUrl, array(), $cookies);

    $cases[] = e2e_finalize_case(array(
        'case' => 'galaxy_renders_action_links_reports_debris_and_ipm_form',
        'checks' => array_merge(e2e_response_check($responseGalaxy, 'galaxy action render'), e2e_response_check($responseIpmForm, 'galaxy IPM form render'), array(
            e2e_case(stripos($body, 'Solar system ' . $coords['g'] . ':' . $coords['s']) !== false, 'galaxy renders the selected system heading'),
            e2e_case(stripos($body, 'E2E Target') !== false, 'galaxy renders defender planet name'),
            e2e_case(preg_match('/doit\(6,\s*' . $coords['g'] . '\s*,\s*' . $coords['s'] . '\s*,\s*' . $coords['defender_p'] . '\s*,\s*1\s*,/i', $body) === 1, 'galaxy renders planet espionage action'),
            e2e_case(stripos($body, 'target_mission=1') !== false && stripos($body, 'target_mission=3') !== false && stripos($body, 'target_mission=5') !== false, 'galaxy renders attack, transport, and defend links'),
            e2e_case(stripos($body, 'page=writemessages') !== false && stripos($body, 'page=buddy') !== false, 'galaxy renders message and buddy actions'),
            e2e_case(stripos($body, 'bericht=' . $spyReportId) !== false, 'commander galaxy renders shared spy report shortcut', array('spy_report_id' => $spyReportId)),
            e2e_case(stripos($body, 'img/r.gif') !== false && stripos($body, 'mode=1') !== false && stripos($body, 'pdd=' . $defenderPlanet) !== false, 'galaxy renders rocket attack shortcut for in-range target'),
            e2e_case(preg_match('/doit\(8,\s*' . $coords['g'] . '\s*,\s*' . $coords['s'] . '\s*,\s*' . $coords['defender_p'] . '\s*,\s*2\s*,/i', $body) === 1, 'galaxy renders debris recycle action'),
            e2e_case($moonId > 0 && stripos($body, 'planettype=3') !== false && stripos($body, 'target_mission=9') !== false, 'galaxy renders moon destroy action when a moon exists', array('moon_id' => $moonId)),
            e2e_case(stripos($body, "id=\"probes\"") !== false && stripos($body, "id=\"recyclers\"") !== false && stripos($body, "id=\"missiles\"") !== false && stripos($body, "id='slots'") !== false, 'commander galaxy renders quick-action counters'),
            e2e_case(stripos($responseIpmForm['body'], 'Launch a rocket to') !== false && stripos($responseIpmForm['body'], 'name="anz"') !== false && stripos($responseIpmForm['body'], 'name="pziel"') !== false, 'IPM shortcut opens the missile target form'),
        )),
    ));

    $remoteSystem = $coords['s'] < (int)$GlobalUni['systems'] ? $coords['s'] + 1 : $coords['s'] - 1;
    dbquery("UPDATE {$db_prefix}planets SET `" . GID_RC_DEUTERIUM . "`=25, lastpeek=" . time() . " WHERE planet_id={$attackerPlanet}");
    $beforeRemote = e2e_one_row("SELECT `" . GID_RC_DEUTERIUM . "` AS deut FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");
    $responseRemote = e2e_http_request('GET', $gameBase . '/index.php?page=galaxy&no_header=1&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&galaxy=' . $coords['g'] . '&system=' . $remoteSystem, array(), $cookies);
    $afterRemote = e2e_one_row("SELECT `" . GID_RC_DEUTERIUM . "` AS deut FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");
    dbquery("UPDATE {$db_prefix}planets SET `" . GID_RC_DEUTERIUM . "`=0, lastpeek=" . time() . " WHERE planet_id={$attackerPlanet}");
    $responseNoDeut = e2e_http_request('GET', $gameBase . '/index.php?page=galaxy&no_header=1&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&galaxy=' . $coords['g'] . '&system=' . $remoteSystem, array(), $cookies);

    $cases[] = e2e_finalize_case(array(
        'case' => 'galaxy_remote_system_deuterium_cost_and_empty_error',
        'checks' => array_merge(e2e_response_check($responseRemote, 'remote galaxy render'), e2e_response_check($responseNoDeut, 'remote galaxy no-deuterium render'), array(
            e2e_case($beforeRemote !== null && $afterRemote !== null && (int)$afterRemote['deut'] === (int)$beforeRemote['deut'] - GALAXY_DEUTERIUM_CONS, 'viewing another system charges galaxy deuterium cost', array('before' => $beforeRemote, 'after' => $afterRemote, 'cost' => GALAXY_DEUTERIUM_CONS)),
            e2e_case(stripos($responseNoDeut['body'], 'Not enough deuterium') !== false, 'viewing another system without deuterium renders the deuterium error'),
        )),
    ));

    e2e_cleanup_runtime(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet), $extraPlanets);
    $extraPlanets = array();
    e2e_reset_user($attackerId, true, 10);
    e2e_reset_user($defenderId, false, 10);
    e2e_prepare_planet($attackerPlanet, $attackerId, $coords['g'], $coords['s'], $coords['attacker_p'], 'E2E Alpha');
    e2e_prepare_planet($defenderPlanet, $defenderId, $coords['g'], $coords['s'], $coords['defender_p'], 'E2E Target');
    $debrisId = CreateDebris($coords['g'], $coords['s'], $coords['defender_p'], $defenderId);
    if ($debrisId > 0) {
        $extraPlanets[] = $debrisId;
        AddDebris($debrisId, 9000, 9000);
    }
    @unlink('temp/fleetlock_' . $attackerPlanet);

    $auth = e2e_prepare_session($attackerId, 'galaxy-ajax');
    $cookies = $auth['cookies'];
    $session = $auth['session'];
    $ajaxUrl = $gameBase . '/index.php?ajax=1&page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet;
    $beforeProbeFleetCount = e2e_active_fleet_count($attackerId);
    $beforeProbePlanet = e2e_one_row("SELECT `" . GID_F_PROBE . "` AS probes FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");
    $responseAjaxSpy = e2e_http_request('POST', $ajaxUrl, array(
        'session' => $session,
        'order' => FTYP_SPY,
        'galaxy' => $coords['g'],
        'system' => $coords['s'],
        'planet' => $coords['defender_p'],
        'planettype' => GAME_PTYP_PLANET,
        'shipcount' => 2,
        'speed' => 10,
        'reply' => 'short',
    ), $cookies);
    $spyFleet = e2e_one_row("SELECT fleet_id, mission, target_planet, `" . GID_F_PROBE . "` AS probes FROM {$db_prefix}fleet WHERE owner_id={$attackerId} AND mission=" . FTYP_SPY . " ORDER BY fleet_id DESC LIMIT 1");
    $afterProbePlanet = e2e_one_row("SELECT `" . GID_F_PROBE . "` AS probes FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");

    e2e_cleanup_runtime(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet), $extraPlanets);
    $extraPlanets = array();
    e2e_reset_user($attackerId, true, 10);
    e2e_reset_user($defenderId, false, 10);
    e2e_prepare_planet($attackerPlanet, $attackerId, $coords['g'], $coords['s'], $coords['attacker_p'], 'E2E Alpha');
    e2e_prepare_planet($defenderPlanet, $defenderId, $coords['g'], $coords['s'], $coords['defender_p'], 'E2E Target');
    $debrisId = CreateDebris($coords['g'], $coords['s'], $coords['defender_p'], $defenderId);
    if ($debrisId > 0) {
        $extraPlanets[] = $debrisId;
        AddDebris($debrisId, 9000, 9000);
    }
    @unlink('temp/fleetlock_' . $attackerPlanet);
    $auth = e2e_prepare_session($attackerId, 'galaxy-ajax-recycle');
    $cookies = $auth['cookies'];
    $session = $auth['session'];
    $ajaxUrl = $gameBase . '/index.php?ajax=1&page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet;
    $beforeRecyclerPlanet = e2e_one_row("SELECT `" . GID_F_RECYCLER . "` AS recyclers FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");
    $responseAjaxRecycle = e2e_http_request('POST', $ajaxUrl, array(
        'session' => $session,
        'order' => FTYP_RECYCLE,
        'galaxy' => $coords['g'],
        'system' => $coords['s'],
        'planet' => $coords['defender_p'],
        'planettype' => GAME_PTYP_DF,
        'shipcount' => 1,
        'speed' => 10,
        'reply' => 'short',
    ), $cookies);
    $recycleFleet = e2e_one_row("SELECT fleet_id, mission, target_planet, `" . GID_F_RECYCLER . "` AS recyclers FROM {$db_prefix}fleet WHERE owner_id={$attackerId} AND mission=" . FTYP_RECYCLE . " ORDER BY fleet_id DESC LIMIT 1");
    $afterRecyclerPlanet = e2e_one_row("SELECT `" . GID_F_RECYCLER . "` AS recyclers FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");

    $cases[] = e2e_finalize_case(array(
        'case' => 'galaxy_ajax_spy_and_recycle_dispatch',
        'checks' => array_merge(e2e_response_check($responseAjaxSpy, 'galaxy AJAX spy'), e2e_response_check($responseAjaxRecycle, 'galaxy AJAX recycle'), array(
            e2e_case(str_starts_with(trim($responseAjaxSpy['body']), '600 '), 'AJAX spy returns success code', array('body' => trim($responseAjaxSpy['body']))),
            e2e_case($spyFleet !== null && (int)$spyFleet['mission'] === FTYP_SPY && (int)$spyFleet['target_planet'] === $defenderPlanet && (int)$spyFleet['probes'] === 2, 'AJAX spy creates a probe fleet to the target planet', $spyFleet ?? array()),
            e2e_case(e2e_active_fleet_count($attackerId) >= $beforeProbeFleetCount + 1, 'AJAX spy creates an active fleet queue task', array('before' => $beforeProbeFleetCount, 'after' => e2e_active_fleet_count($attackerId))),
            e2e_case($beforeProbePlanet !== null && $afterProbePlanet !== null && (int)$afterProbePlanet['probes'] === (int)$beforeProbePlanet['probes'] - 2, 'AJAX spy debits probes from origin planet', array('before' => $beforeProbePlanet, 'after' => $afterProbePlanet)),
            e2e_case(str_starts_with(trim($responseAjaxRecycle['body']), '600 '), 'AJAX recycle returns success code', array('body' => trim($responseAjaxRecycle['body']))),
            e2e_case($recycleFleet !== null && (int)$recycleFleet['mission'] === FTYP_RECYCLE && (int)$recycleFleet['target_planet'] === $debrisId && (int)$recycleFleet['recyclers'] === 1, 'AJAX recycle creates a recycler fleet to the debris field', $recycleFleet ?? array()),
            e2e_case($beforeRecyclerPlanet !== null && $afterRecyclerPlanet !== null && (int)$afterRecyclerPlanet['recyclers'] === (int)$beforeRecyclerPlanet['recyclers'] - 1, 'AJAX recycle debits recyclers from origin planet', array('before' => $beforeRecyclerPlanet, 'after' => $afterRecyclerPlanet)),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'galaxy_templates_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e), 'trace' => $e->getTraceAsString()))),
        'pass' => false,
    );
} finally {
    if ($attackerId > 0 && $defenderId > 0 && $attackerPlanet > 0 && $defenderPlanet > 0) {
        e2e_cleanup_runtime(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet), $extraPlanets);
    }
}

echo json_encode(array(
    'case_group' => 'http_galaxy_templates',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
