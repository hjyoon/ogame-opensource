<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_planet_context_e2e.php';
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
loca_add('renameplanet', 'en');
loca_add('resources', 'en');
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

function e2e_prepare_planet(int $planetId, int $ownerId, string $name, int $smallCargo = 10): void
{
    global $db_prefix;
    $now = time();

    dbquery(
        "UPDATE {$db_prefix}planets SET name='" . e2e_sql_escape($name) . "', " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "`" . GID_B_METAL_MINE . "`=0, `" . GID_B_CRYS_MINE . "`=0, `" . GID_B_DEUT_SYNTH . "`=0, `" . GID_B_SOLAR . "`=10, " .
        "`" . GID_B_FUSION . "`=0, `" . GID_B_ROBOTS . "`=10, `" . GID_B_NANITES . "`=0, `" . GID_B_SHIPYARD . "`=10, " .
        "`" . GID_B_METAL_STOR . "`=10, `" . GID_B_CRYS_STOR . "`=10, `" . GID_B_DEUT_STOR . "`=10, `" . GID_B_RES_LAB . "`=10, " .
        "`" . GID_B_LUNAR_BASE . "`=0, `" . GID_B_PHALANX . "`=0, `" . GID_B_JUMP_GATE . "`=0, " .
        "`" . GID_F_SC . "`={$smallCargo}, `" . GID_F_LC . "`=5, `" . GID_F_LF . "`=5, `" . GID_F_PROBE . "`=5, `" . GID_F_SAT . "`=0, " .
        "prod1=1, prod2=1, prod3=1, prod4=1, prod12=1, prod212=1, fields=0, maxfields=200, type=" . PTYP_PLANET . ", " .
        "owner_id={$ownerId}, gate_until=0, remove=0, lastpeek={$now} WHERE planet_id={$planetId}"
    );
}

function e2e_prepare_moon(int $moonId, int $ownerId, string $name): void
{
    global $db_prefix, $buildmap, $fleetmap;
    $now = time();

    $buildingAssignments = array();
    foreach ($buildmap as $gid) {
        $buildingAssignments[] = "`{$gid}`=0";
    }
    $shipAssignments = array();
    foreach ($fleetmap as $gid) {
        $shipAssignments[] = "`{$gid}`=0";
    }

    dbquery(
        "UPDATE {$db_prefix}planets SET name='" . e2e_sql_escape($name) . "', " .
        implode(',', $buildingAssignments) . ", " .
        implode(',', $shipAssignments) . ", " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000, " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=0, maxfields=1, type=" . PTYP_MOON . ", " .
        "owner_id={$ownerId}, gate_until=0, remove=0, lastpeek={$now} WHERE planet_id={$moonId}"
    );
}

function e2e_reset_user_and_planet(int $userId, int $planetId, string $name, int $smallCargo = 10): void
{
    global $db_prefix;

    e2e_cleanup_fleets(array($userId), array($planetId));
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$userId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_UNBAN . "','" . QTYP_ALLOW_ATTACKS . "','" . QTYP_DEBUG . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$userId} OR planet_id={$planetId}");
    e2e_reset_user($userId);
    e2e_prepare_planet($planetId, $userId, $name, $smallCargo);
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

function e2e_create_planet_near(int $ownerId, array $near, string $name, int $smallCargo = 0): int
{
    [$g, $s, $p] = e2e_find_empty_position($near);
    $planetId = CreatePlanet($g, $s, $p, $ownerId, 1, 0, 0, time());
    if ($planetId <= 0) {
        throw new RuntimeException('Failed to create a nearby planet.');
    }
    e2e_prepare_planet($planetId, $ownerId, $name, $smallCargo);
    return $planetId;
}

function e2e_create_moon_for_planet(int $planetId, int $ownerId, string $name): int
{
    $planet = LoadPlanetById($planetId);
    if ($planet === null) {
        throw new RuntimeException('Cannot create moon for missing planet.');
    }
    $moonId = CreatePlanet((int)$planet['g'], (int)$planet['s'], (int)$planet['p'], $ownerId, 1, 1, 20, time());
    if ($moonId <= 0) {
        throw new RuntimeException('Failed to create moon.');
    }
    e2e_prepare_moon($moonId, $ownerId, $name);
    return $moonId;
}

function e2e_planet_snapshot(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, name, type, g, s, p, fields, maxfields, remove, prod1, prod2, prod3, prod4, prod12, prod212, " .
        "`" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deuterium, " .
        "`" . GID_B_METAL_MINE . "` AS metal_mine, `" . GID_B_LUNAR_BASE . "` AS lunar_base, " .
        "`" . GID_F_SC . "` AS small_cargo " .
        "FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1"
    );
}

function e2e_selected_planet(int $userId): int
{
    global $db_prefix;
    $row = e2e_one_row("SELECT aktplanet FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1");
    return $row === null ? 0 : (int)$row['aktplanet'];
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

function e2e_clear_action_state(array $userIds, array $planetIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', $userIds));
    $planetList = implode(',', array_map('intval', $planetIds));
    if ($userList !== '') {
        dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id IN ({$userList}) AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "')");
        dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id IN ({$userList})");
    }
    if ($planetList !== '') {
        dbquery("DELETE FROM {$db_prefix}buildqueue WHERE planet_id IN ({$planetList})");
    }
    e2e_cleanup_fleets($userIds, $planetIds);
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$attackerPassword = getenv('OGAME_E2E_ATTACKER_PASSWORD') ?: '';
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderPlanet = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$cases = array();
$createdPlanets = array();
$createdMoons = array();

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0 || $attackerPassword === '') {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $homeName = 'E2E Context Home';
    $colonyName = 'E2E Context Colony';
    $moonName = 'E2E Context Moon';
    $attackerHome = LoadPlanetById($attackerPlanet);
    if ($attackerHome === null) {
        throw new RuntimeException('Attacker home planet is missing.');
    }
    $attackerColony = e2e_create_planet_near($attackerId, $attackerHome, $colonyName, 0);
    $createdPlanets[] = $attackerColony;
    $attackerMoon = e2e_create_moon_for_planet($attackerPlanet, $attackerId, $moonName);
    $createdMoons[] = $attackerMoon;

    e2e_reset_user_and_planet($attackerId, $attackerPlanet, $homeName, 10);
    e2e_prepare_planet($attackerColony, $attackerId, $colonyName, 0);
    e2e_prepare_moon($attackerMoon, $attackerId, $moonName);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet, 'E2E Context Defender', 10);
    e2e_clear_action_state(array($attackerId, $defenderId), array($attackerPlanet, $attackerColony, $attackerMoon, $defenderPlanet));

    $auth = e2e_prepare_session($attackerId, 'planet-context-attacker');
    $cookies = $auth['cookies'];
    $session = $auth['session'];

    SelectPlanet($attackerId, $attackerPlanet);
    $responseColony = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($session) . '&cp=' . $attackerColony, array(), $cookies);
    $selectedAfterColony = e2e_selected_planet($attackerId);
    $responseForeign = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($session) . '&cp=' . $defenderPlanet, array(), $cookies);
    $selectedAfterForeign = e2e_selected_planet($attackerId);
    $responseMissing = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($session) . '&cp=987654321', array(), $cookies);
    $selectedAfterMissing = e2e_selected_planet($attackerId);
    $responseMoon = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($session) . '&cp=' . $attackerMoon, array(), $cookies);
    $selectedAfterMoon = e2e_selected_planet($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'cp_switch_foreign_rejection_missing_fallback_and_moon_selection',
        'checks' => array_merge(
            e2e_response_check($responseColony),
            e2e_response_check($responseForeign),
            e2e_response_check($responseMissing),
            e2e_response_check($responseMoon),
            array(
                e2e_case($selectedAfterColony === $attackerColony, 'owned colony cp becomes selected', array('selected' => $selectedAfterColony, 'colony' => $attackerColony)),
                e2e_case(strpos($responseColony['body'], $colonyName) !== false, 'owned colony overview renders the colony name'),
                e2e_case($selectedAfterForeign === $attackerColony, 'foreign cp does not replace the selected owned planet', array('selected' => $selectedAfterForeign, 'expected' => $attackerColony)),
                e2e_case($selectedAfterMissing === $attackerPlanet, 'missing cp falls back to the home planet', array('selected' => $selectedAfterMissing, 'home' => $attackerPlanet)),
                e2e_case($selectedAfterMoon === $attackerMoon, 'owned moon cp can be selected independently', array('selected' => $selectedAfterMoon, 'moon' => $attackerMoon)),
                e2e_case(strpos($responseMoon['body'], $moonName) !== false, 'owned moon overview renders the moon name'),
            )
        ),
    ));

    e2e_clear_action_state(array($attackerId), array($attackerPlanet, $attackerColony, $attackerMoon));
    e2e_prepare_planet($attackerPlanet, $attackerId, $homeName, 10);
    e2e_prepare_planet($attackerColony, $attackerId, $colonyName, 0);
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=resources&session=' . rawurlencode($session) . '&cp=' . $attackerColony, array(
        'last1' => 80,
        'last2' => 70,
        'last3' => 60,
        'last4' => 100,
        'last12' => 90,
        'last212' => 50,
        'action' => 'Recalculate',
    ), $cookies);
    $homeAfterResources = e2e_planet_snapshot($attackerPlanet);
    $colonyAfterResources = e2e_planet_snapshot($attackerColony);
    $cases[] = e2e_finalize_case(array(
        'case' => 'resource_settings_apply_only_to_selected_cp',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($colonyAfterResources !== null && abs((float)$colonyAfterResources['prod1'] - 0.8) < 0.001 && abs((float)$colonyAfterResources['prod2'] - 0.7) < 0.001, 'selected colony production factors are updated', $colonyAfterResources ?? array()),
            e2e_case($homeAfterResources !== null && abs((float)$homeAfterResources['prod1'] - 1.0) < 0.001 && abs((float)$homeAfterResources['prod2'] - 1.0) < 0.001, 'home production factors stay unchanged', $homeAfterResources ?? array()),
        )),
    ));

    e2e_clear_action_state(array($attackerId), array($attackerPlanet, $attackerColony, $attackerMoon));
    e2e_prepare_planet($attackerPlanet, $attackerId, $homeName, 10);
    e2e_prepare_planet($attackerColony, $attackerId, $colonyName, 0);
    $homeBeforeBuild = e2e_planet_snapshot($attackerPlanet);
    $colonyBeforeBuild = e2e_planet_snapshot($attackerColony);
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=b_building&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&modus=add&techid=' . GID_B_METAL_MINE . '&planet=' . $attackerColony, array(), $cookies);
    $homeAfterBuild = e2e_planet_snapshot($attackerPlanet);
    $colonyAfterBuild = e2e_planet_snapshot($attackerColony);
    $buildRow = e2e_one_row("SELECT id, owner_id, planet_id, tech_id, level FROM {$db_prefix}buildqueue WHERE owner_id={$attackerId} ORDER BY id DESC LIMIT 1");
    $buildTask = e2e_one_row("SELECT task_id, owner_id, type, sub_id, obj_id FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "' ORDER BY task_id DESC LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'owned_non_current_planet_build_queue_is_scoped_to_payload_planet',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($buildRow !== null && (int)$buildRow['planet_id'] === $attackerColony && (int)$buildRow['tech_id'] === GID_B_METAL_MINE, 'buildqueue row is created for the owned payload planet', $buildRow ?? array()),
            e2e_case($buildTask !== null && (int)$buildTask['sub_id'] === (int)($buildRow['id'] ?? 0), 'global build task points at the payload planet buildqueue row', $buildTask ?? array()),
            e2e_case($homeBeforeBuild !== null && $homeAfterBuild !== null && (int)$homeAfterBuild['metal'] === (int)$homeBeforeBuild['metal'], 'current cp planet resources are not debited by non-current owned planet build', array('before' => $homeBeforeBuild, 'after' => $homeAfterBuild)),
            e2e_case($colonyBeforeBuild !== null && $colonyAfterBuild !== null && (int)$colonyAfterBuild['metal'] < (int)$colonyBeforeBuild['metal'], 'payload planet resources are debited by its build queue', array('before' => $colonyBeforeBuild, 'after' => $colonyAfterBuild)),
        )),
    ));

    e2e_clear_action_state(array($attackerId), array($attackerPlanet, $attackerColony, $attackerMoon));
    e2e_prepare_moon($attackerMoon, $attackerId, $moonName);
    $responseBadMoonBuild = e2e_http_request('GET', $gameBase . '/index.php?page=b_building&session=' . rawurlencode($session) . '&cp=' . $attackerMoon . '&modus=add&techid=' . GID_B_METAL_MINE . '&planet=' . $attackerMoon, array(), $cookies);
    $badMoonBuildCount = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}buildqueue WHERE owner_id={$attackerId} AND planet_id={$attackerMoon}");
    $responseLunarBuild = e2e_http_request('GET', $gameBase . '/index.php?page=b_building&session=' . rawurlencode($session) . '&cp=' . $attackerMoon . '&modus=add&techid=' . GID_B_LUNAR_BASE . '&planet=' . $attackerMoon, array(), $cookies);
    $moonBuildRow = e2e_one_row("SELECT id, owner_id, planet_id, tech_id, level FROM {$db_prefix}buildqueue WHERE owner_id={$attackerId} AND planet_id={$attackerMoon} ORDER BY id DESC LIMIT 1");
    $planetBuildRows = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}buildqueue WHERE owner_id={$attackerId} AND planet_id={$attackerPlanet}");
    $cases[] = e2e_finalize_case(array(
        'case' => 'moon_and_planet_building_types_stay_isolated',
        'checks' => array_merge(e2e_response_check($responseBadMoonBuild), e2e_response_check($responseLunarBuild), array(
            e2e_case($badMoonBuildCount === 0, 'planet-only building is rejected on moon context', array('bad_moon_build_count' => $badMoonBuildCount)),
            e2e_case($moonBuildRow !== null && (int)$moonBuildRow['tech_id'] === GID_B_LUNAR_BASE, 'lunar base can be queued on the moon context', $moonBuildRow ?? array()),
            e2e_case($planetBuildRows === 0, 'moon build does not create a buildqueue row on the underlying planet', array('planet_build_rows' => $planetBuildRows)),
        )),
    ));

    e2e_clear_action_state(array($attackerId, $defenderId), array($attackerPlanet, $attackerColony, $attackerMoon, $defenderPlanet));
    e2e_prepare_planet($attackerPlanet, $attackerId, $homeName, 10);
    e2e_prepare_planet($attackerColony, $attackerId, $colonyName, 0);
    e2e_prepare_planet($defenderPlanet, $defenderId, 'E2E Context Defender', 10);
    @unlink('temp/fleetlock_' . $attackerPlanet);
    @unlink('temp/fleetlock_' . $attackerColony);
    $homeBeforeFleet = e2e_planet_snapshot($attackerPlanet);
    $colonyBeforeFleet = e2e_planet_snapshot($attackerColony);
    $defenderBeforeFleet = e2e_planet_snapshot($defenderPlanet);
    $payload = e2e_fleet_payload(LoadPlanetById($attackerColony), LoadPlanetById($defenderPlanet), FTYP_TRANSPORT, array(GID_F_SC => 1), array(GID_RC_METAL => 123));
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, $payload, $cookies);
    $homeAfterFleet = e2e_planet_snapshot($attackerPlanet);
    $colonyAfterFleet = e2e_planet_snapshot($attackerColony);
    $defenderAfterFleet = e2e_planet_snapshot($defenderPlanet);
    $fleetRowsAfterSpoof = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id={$attackerId}");
    $fleetQueueAfterSpoof = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_FLEET . "'");
    $cases[] = e2e_finalize_case(array(
        'case' => 'fleet_dispatch_rejects_origin_payload_that_differs_from_current_cp',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($fleetRowsAfterSpoof === 0 && $fleetQueueAfterSpoof === 0, 'mismatched origin payload does not create fleet rows or queue tasks', array('fleet_rows' => $fleetRowsAfterSpoof, 'queue_rows' => $fleetQueueAfterSpoof)),
            e2e_case($homeBeforeFleet !== null && $homeAfterFleet !== null && (int)$homeAfterFleet['small_cargo'] === (int)$homeBeforeFleet['small_cargo'] && (int)$homeAfterFleet['metal'] === (int)$homeBeforeFleet['metal'], 'current cp planet is not debited by rejected spoofed-origin fleet', array('before' => $homeBeforeFleet, 'after' => $homeAfterFleet)),
            e2e_case($colonyBeforeFleet !== null && $colonyAfterFleet !== null && (int)$colonyAfterFleet['small_cargo'] === (int)$colonyBeforeFleet['small_cargo'] && (int)$colonyAfterFleet['metal'] === (int)$colonyBeforeFleet['metal'], 'spoofed origin planet is not debited by rejected fleet', array('before' => $colonyBeforeFleet, 'after' => $colonyAfterFleet)),
            e2e_case($defenderBeforeFleet !== null && $defenderAfterFleet !== null && (int)$defenderAfterFleet['metal'] === (int)$defenderBeforeFleet['metal'], 'target planet receives no resources from rejected spoofed-origin fleet', array('before' => $defenderBeforeFleet, 'after' => $defenderAfterFleet)),
        )),
    ));

    e2e_clear_action_state(array($attackerId), array($attackerPlanet, $attackerColony, $attackerMoon));
    e2e_prepare_planet($attackerPlanet, $attackerId, $homeName, 10);
    e2e_prepare_planet($attackerColony, $attackerId, $colonyName, 0);
    SelectPlanet($attackerId, $attackerColony);
    $responseWrongPassword = e2e_http_request('POST', $gameBase . '/index.php?page=renameplanet&session=' . rawurlencode($session) . '&cp=' . $attackerColony . '&pl=' . $attackerColony, array(
        'page' => 'renameplanet',
        'deleteid' => $attackerColony,
        'pw' => 'wrong-password',
        'aktion' => loca('REN_DELETE_PLANET'),
    ), $cookies);
    $colonyAfterWrongPassword = e2e_planet_snapshot($attackerColony);
    $responseHomeDelete = e2e_http_request('POST', $gameBase . '/index.php?page=renameplanet&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&pl=' . $attackerPlanet, array(
        'page' => 'renameplanet',
        'deleteid' => $attackerPlanet,
        'pw' => $attackerPassword,
        'aktion' => loca('REN_DELETE_PLANET'),
    ), $cookies);
    $homeAfterDeleteAttempt = e2e_planet_snapshot($attackerPlanet);
    $responseColonyDelete = e2e_http_request('POST', $gameBase . '/index.php?page=renameplanet&session=' . rawurlencode($session) . '&cp=' . $attackerColony . '&pl=' . $attackerColony, array(
        'page' => 'renameplanet',
        'deleteid' => $attackerColony,
        'pw' => $attackerPassword,
        'aktion' => loca('REN_DELETE_PLANET'),
    ), $cookies);
    $colonyAfterDelete = e2e_planet_snapshot($attackerColony);
    $selectedAfterDelete = e2e_selected_planet($attackerId);
    $responseDeletedCp = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($session) . '&cp=' . $attackerColony, array(), $cookies);
    $selectedAfterDeletedCp = e2e_selected_planet($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'colony_abandon_requires_password_preserves_home_and_falls_back_after_delete',
        'checks' => array_merge(e2e_response_check($responseWrongPassword), e2e_response_check($responseHomeDelete), e2e_response_check($responseColonyDelete), e2e_response_check($responseDeletedCp), array(
            e2e_case($colonyAfterWrongPassword !== null && (int)$colonyAfterWrongPassword['owner_id'] === $attackerId && (int)$colonyAfterWrongPassword['type'] === PTYP_PLANET, 'wrong password does not abandon colony', $colonyAfterWrongPassword ?? array()),
            e2e_case($homeAfterDeleteAttempt !== null && (int)$homeAfterDeleteAttempt['owner_id'] === $attackerId && (int)$homeAfterDeleteAttempt['type'] === PTYP_PLANET, 'home planet cannot be abandoned even with the correct password', $homeAfterDeleteAttempt ?? array()),
            e2e_case($colonyAfterDelete !== null && (int)$colonyAfterDelete['owner_id'] === USER_SPACE && (int)$colonyAfterDelete['type'] === PTYP_DEST_PLANET && (int)$colonyAfterDelete['remove'] > time(), 'colony abandon marks the colony as destroyed and scheduled for removal', $colonyAfterDelete ?? array()),
            e2e_case($selectedAfterDelete === $attackerPlanet, 'successful colony abandon selects the home planet', array('selected' => $selectedAfterDelete, 'home' => $attackerPlanet)),
            e2e_case($selectedAfterDeletedCp === $attackerPlanet, 'opening a destroyed colony cp keeps the user on the home planet', array('selected' => $selectedAfterDeletedCp, 'home' => $attackerPlanet)),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'planet_context_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    $userIds = array_filter(array($attackerId, $defenderId), fn($id) => $id > 0);
    $planetIds = array_filter(array_merge(array($attackerPlanet, $defenderPlanet), $createdPlanets, $createdMoons), fn($id) => $id > 0);
    if (!empty($userIds) && !empty($planetIds)) {
        e2e_clear_action_state($userIds, $planetIds);
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
        e2e_reset_user_and_planet($attackerId, $attackerPlanet, 'E2E Fixture Home', 10);
        SelectPlanet($attackerId, $attackerPlanet);
    }
    if ($defenderId > 0 && $defenderPlanet > 0) {
        e2e_reset_user_and_planet($defenderId, $defenderPlanet, 'E2E Fixture Defender', 10);
    }
}

echo json_encode(array(
    'case_group' => 'http_planet_context',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
