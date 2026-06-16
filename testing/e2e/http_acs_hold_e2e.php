<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_acs_hold_e2e.php';
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
        "`" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=10, `" . GID_B_RES_LAB . "`=10, " .
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

function e2e_user_by_name(string $name): ?array
{
    global $db_prefix;
    $res = dbquery("SELECT player_id, hplanetid FROM {$db_prefix}users WHERE name='" . e2e_sql_escape(mb_strtolower($name, 'UTF-8')) . "' LIMIT 1");
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function e2e_remove_user_by_name(string $name): void
{
    $row = e2e_user_by_name($name);
    if ($row !== null) {
        RemoveUser((int)$row['player_id'], time());
    }
}

function e2e_create_support_user(string $name): array
{
    global $db_prefix;

    e2e_remove_user_by_name($name);

    $password = 'E2E_test123';
    $id = CreateUser($name, $password, $name . '@example.local', false);
    $row = e2e_one_row("SELECT player_id, hplanetid, oname FROM {$db_prefix}users WHERE player_id={$id} LIMIT 1");
    if ($row === null || (int)$row['hplanetid'] <= 0) {
        throw new RuntimeException("Failed to create support user {$name}");
    }
    e2e_reset_user_and_planet($id, (int)$row['hplanetid'], 10, 10);
    return array(
        'player_id' => $id,
        'planet_id' => (int)$row['hplanetid'],
        'name' => $name,
        'password' => $password,
    );
}

function e2e_create_nearby_planet(int $ownerId, array $near): int
{
    global $GlobalUni;

    $g = (int)$near['g'];
    $s = (int)$near['s'];
    for ($p = 1; $p <= 15; $p++) {
        if ($p === (int)$near['p']) {
            continue;
        }
        if (!HasPlanet($g, $s, $p)) {
            $planetId = CreatePlanet($g, $s, $p, $ownerId, 1, 0, 0, time());
            if ($planetId > 0) {
                return $planetId;
            }
        }
    }

    for ($system = 1; $system <= (int)$GlobalUni['systems']; $system++) {
        for ($p = 1; $p <= 15; $p++) {
            if (!HasPlanet($g, $system, $p)) {
                $planetId = CreatePlanet($g, $system, $p, $ownerId, 1, 0, 0, time());
                if ($planetId > 0) {
                    return $planetId;
                }
            }
        }
    }

    throw new RuntimeException('No empty nearby planet slot found.');
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
        "`" . GID_F_SC . "` AS small_cargo, `" . GID_F_LF . "` AS light_fighter " .
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
$supportUser = null;

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $supportName = (getenv('OGAME_E2E_FIXTURE_PREFIX') ?: 'e2e_fixture') . '_support';
    $supportUser = e2e_create_support_user($supportName);
    $supportId = (int)$supportUser['player_id'];
    $supportHome = (int)$supportUser['planet_id'];

    e2e_reset_user_and_planet($attackerId, $attackerPlanet, 10, 10);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet, 0, 0);
    e2e_reset_user_and_planet($supportId, $supportHome, 10, 10);
    e2e_cleanup_relations(array($attackerId, $defenderId, $supportId));
    e2e_cleanup_fleets(array($attackerId, $defenderId, $supportId), array($attackerPlanet, $defenderPlanet, $supportHome));

    $attackerAuth = e2e_prepare_session($attackerId, 'acs-hold-attacker');
    $attackerCookies = $attackerAuth['cookies'];
    $attackerSession = $attackerAuth['session'];
    $supportAuth = e2e_prepare_session($supportId, 'acs-support');
    $supportCookies = $supportAuth['cookies'];
    $supportSession = $supportAuth['session'];

    $origin = LoadPlanetById($attackerPlanet);
    $target = LoadPlanetById($defenderPlanet);
    $unauthorizedResponse = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($attackerSession) . '&cp=' . $attackerPlanet,
        e2e_fleet_payload($origin, $target, FTYP_ACS_HOLD, array(GID_F_LF => 1), array(), array('holdingtime' => 1)),
        $attackerCookies
    );
    $cases[] = e2e_finalize_case(array(
        'case' => 'acs_hold_requires_buddy_or_alliance',
        'checks' => array_merge(e2e_response_check($unauthorizedResponse), array(
            e2e_case(e2e_active_fleet_count($attackerId) === 0, 'unauthorized hold does not create a fleet queue'),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id={$attackerId}") === 0, 'unauthorized hold does not create a fleet row'),
            e2e_case(stripos($unauthorizedResponse['body'], 'error') !== false, 'unauthorized hold renders an error response'),
        )),
    ));

    $buddyId = AddBuddy($attackerId, $defenderId, 'E2E hold relation');
    if ($buddyId > 0) {
        AcceptBuddy($buddyId);
    }
    $beforeHoldOrigin = e2e_planet_snapshot($attackerPlanet);
    $holdResponse = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($attackerSession) . '&cp=' . $attackerPlanet,
        e2e_fleet_payload($origin, $target, FTYP_ACS_HOLD, array(GID_F_LF => 1), array(), array('holdingtime' => 1)),
        $attackerCookies
    );
    $holdFleet = e2e_latest_fleet($attackerId, FTYP_ACS_HOLD);
    $holdQueue = $holdFleet ? GetFleetQueue((int)$holdFleet['fleet_id']) : null;
    $afterHoldLaunch = e2e_planet_snapshot($attackerPlanet);
    $completedHoldOutbound = e2e_complete_fleet($holdFleet);
    $orbitFleet = e2e_latest_fleet($attackerId, FTYP_ACS_HOLD + FTYP_ORBITING);
    $orbitQueue = $orbitFleet ? GetFleetQueue((int)$orbitFleet['fleet_id']) : null;
    $holdingCount = GetHoldingFleetsCount($defenderPlanet);
    $completedHoldOrbit = e2e_complete_fleet($orbitFleet);
    $holdReturnFleet = e2e_latest_fleet($attackerId, FTYP_ACS_HOLD + FTYP_RETURN);
    $holdReturnQueue = $holdReturnFleet ? GetFleetQueue((int)$holdReturnFleet['fleet_id']) : null;
    $completedHoldReturn = e2e_complete_fleet($holdReturnFleet);
    $afterHoldReturn = e2e_planet_snapshot($attackerPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'acs_hold_orbit_and_return_lifecycle',
        'checks' => array_merge(e2e_response_check($holdResponse), array(
            e2e_case($buddyId > 0 && IsBuddy($attackerId, $defenderId), 'accepted buddy relation enables hold mission', array('buddy_id' => $buddyId)),
            e2e_case($holdFleet !== null && (int)$holdFleet['mission'] === FTYP_ACS_HOLD && (int)$holdFleet['deploy_time'] === 3600, 'hold launch creates an outbound ACS hold fleet with one-hour hold time', $holdFleet ?? array()),
            e2e_case($holdQueue !== null, 'outbound hold fleet has a queue task', $holdQueue ?? array()),
            e2e_case($beforeHoldOrigin !== null && $afterHoldLaunch !== null && (int)$afterHoldLaunch['light_fighter'] === (int)$beforeHoldOrigin['light_fighter'] - 1, 'origin fighter count is debited on hold launch', array('before' => $beforeHoldOrigin, 'after_launch' => $afterHoldLaunch)),
            e2e_case($completedHoldOutbound, 'outbound hold queue can be completed'),
            e2e_case($orbitFleet !== null && (int)$orbitFleet['mission'] === FTYP_ACS_HOLD + FTYP_ORBITING && (int)$orbitFleet['flight_time'] === 3600, 'hold arrival creates an orbiting hold fleet', $orbitFleet ?? array()),
            e2e_case($orbitQueue !== null, 'orbiting hold fleet has a queue task', $orbitQueue ?? array()),
            e2e_case($holdingCount >= 1, 'target planet reports at least one holding fleet while orbiting', array('holding_count' => $holdingCount)),
            e2e_case($completedHoldOrbit, 'orbiting hold queue can be completed'),
            e2e_case($holdReturnFleet !== null && (int)$holdReturnFleet['mission'] === FTYP_ACS_HOLD + FTYP_RETURN, 'hold orbit completion creates a return fleet', $holdReturnFleet ?? array()),
            e2e_case($holdReturnQueue !== null, 'hold return fleet has a queue task', $holdReturnQueue ?? array()),
            e2e_case($completedHoldReturn, 'hold return queue can be completed'),
            e2e_case($beforeHoldOrigin !== null && $afterHoldReturn !== null && (int)$afterHoldReturn['light_fighter'] === (int)$beforeHoldOrigin['light_fighter'], 'hold return restores fighter to origin', array('before' => $beforeHoldOrigin, 'after_return' => $afterHoldReturn)),
            e2e_case(e2e_active_fleet_count($attackerId) === 0, 'hold lifecycle leaves no active fleet queue tasks'),
        )),
    ));

    e2e_cleanup_fleets(array($attackerId, $defenderId, $supportId), array($attackerPlanet, $defenderPlanet, $supportHome));
    e2e_cleanup_relations(array($attackerId, $defenderId, $supportId));
    e2e_reset_user_and_planet($attackerId, $attackerPlanet, 10, 10);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet, 0, 0);
    e2e_reset_user_and_planet($supportId, $supportHome, 10, 10);

    $supportPlanet = e2e_create_nearby_planet($supportId, LoadPlanetById($defenderPlanet));
    $createdPlanets[] = $supportPlanet;
    e2e_prepare_planet($supportPlanet, $supportId, 10, 10);

    $headOriginBefore = e2e_planet_snapshot($attackerPlanet);
    $supportOriginBefore = e2e_planet_snapshot($supportPlanet);
    $origin = LoadPlanetById($attackerPlanet);
    $target = LoadPlanetById($defenderPlanet);
    $headResponse = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($attackerSession) . '&cp=' . $attackerPlanet,
        e2e_fleet_payload($origin, $target, FTYP_ATTACK, array(GID_F_LF => 1)),
        $attackerCookies
    );
    $headAttack = e2e_latest_fleet($attackerId, FTYP_ATTACK);
    $unionId = $headAttack === null ? 0 : CreateUnion((int)$headAttack['fleet_id'], 'E2EACS' . substr(md5((string)microtime(true)), 0, 6));
    $headAfterUnion = $unionId > 0 ? e2e_latest_fleet($attackerId, FTYP_ACS_ATTACK_HEAD) : null;
    $GlobalUser = LoadUser($attackerId);
    $inviteResult = $unionId > 0 ? AddUnionMember($unionId, $supportUser['name']) : 'union not created';
    $supportOrigin = LoadPlanetById($supportPlanet);
    $supportResponse = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($supportSession) . '&cp=' . $supportPlanet,
        e2e_fleet_payload($supportOrigin, $target, FTYP_ACS_ATTACK, array(GID_F_LF => 1), array(), array('union2' => $unionId)),
        $supportCookies
    );
    $supportAcsFleet = e2e_latest_fleet($supportId, FTYP_ACS_ATTACK);
    $supportAcsQueue = $supportAcsFleet ? GetFleetQueue((int)$supportAcsFleet['fleet_id']) : null;
    $headQueue = $headAfterUnion ? GetFleetQueue((int)$headAfterUnion['fleet_id']) : null;
    $unionFleetCount = $unionId > 0 ? e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE union_id={$unionId}") : 0;
    $supportAfterLaunch = e2e_planet_snapshot($supportPlanet);
    $completedAcsBattle = e2e_complete_fleet($headAfterUnion);
    $unionAfterBattle = $unionId > 0 ? LoadUnion($unionId) : null;
    $remainingUnionFleets = $unionId > 0 ? e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE union_id={$unionId}") : 0;
    $headReturn = e2e_latest_fleet($attackerId, FTYP_ACS_ATTACK_HEAD + FTYP_RETURN);
    $supportReturn = e2e_latest_fleet($supportId, FTYP_ACS_ATTACK + FTYP_RETURN);
    $completedHeadReturn = e2e_complete_fleet($headReturn);
    $completedSupportReturn = e2e_complete_fleet($supportReturn);
    $headOriginAfterReturn = e2e_planet_snapshot($attackerPlanet);
    $supportOriginAfterReturn = e2e_planet_snapshot($supportPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'acs_attack_union_invite_join_battle_and_return',
        'checks' => array_merge(e2e_response_check($headResponse), e2e_response_check($supportResponse), array(
            e2e_case($headAttack !== null, 'initial attack fleet is created before ACS conversion', $headAttack ?? array()),
            e2e_case($unionId > 0, 'ACS union is created from the initial attack fleet', array('union_id' => $unionId)),
            e2e_case($headAfterUnion !== null && (int)$headAfterUnion['mission'] === FTYP_ACS_ATTACK_HEAD && (int)$headAfterUnion['union_id'] === $unionId, 'initial attack becomes ACS head fleet', $headAfterUnion ?? array()),
            e2e_case($inviteResult === '', 'support user can be invited to ACS union', array('invite_result' => $inviteResult)),
            e2e_case($supportAcsFleet !== null && (int)$supportAcsFleet['mission'] === FTYP_ACS_ATTACK && (int)$supportAcsFleet['union_id'] === $unionId, 'invited support user launches ACS attack into union', $supportAcsFleet ?? array()),
            e2e_case($supportAcsQueue !== null, 'support ACS fleet has a queue task', $supportAcsQueue ?? array()),
            e2e_case($headQueue !== null, 'ACS head fleet has a queue task', $headQueue ?? array()),
            e2e_case($unionFleetCount === 2, 'ACS union contains head and support fleets before battle', array('union_fleet_count' => $unionFleetCount)),
            e2e_case($supportOriginBefore !== null && $supportAfterLaunch !== null && (int)$supportAfterLaunch['light_fighter'] === (int)$supportOriginBefore['light_fighter'] - 1, 'support origin fighter count is debited on ACS launch', array('before' => $supportOriginBefore, 'after_launch' => $supportAfterLaunch)),
            e2e_case($completedAcsBattle, 'ACS head queue can be completed to resolve battle'),
            e2e_case($unionAfterBattle === null, 'ACS union row is removed after battle resolution'),
            e2e_case($remainingUnionFleets === 0, 'ACS battle removes transient union fleet rows', array('remaining_union_fleets' => $remainingUnionFleets)),
            e2e_case($headReturn !== null && (int)$headReturn['mission'] === FTYP_ACS_ATTACK_HEAD + FTYP_RETURN, 'ACS head survivor creates return fleet', $headReturn ?? array()),
            e2e_case($supportReturn !== null && (int)$supportReturn['mission'] === FTYP_ACS_ATTACK + FTYP_RETURN, 'ACS support survivor creates return fleet', $supportReturn ?? array()),
            e2e_case($completedHeadReturn && $completedSupportReturn, 'ACS return queues can be completed'),
            e2e_case($headOriginBefore !== null && $headOriginAfterReturn !== null && (int)$headOriginAfterReturn['light_fighter'] === (int)$headOriginBefore['light_fighter'], 'ACS head return restores attacker fighter', array('before' => $headOriginBefore, 'after_return' => $headOriginAfterReturn)),
            e2e_case($supportOriginBefore !== null && $supportOriginAfterReturn !== null && (int)$supportOriginAfterReturn['light_fighter'] === (int)$supportOriginBefore['light_fighter'], 'ACS support return restores support fighter', array('before' => $supportOriginBefore, 'after_return' => $supportOriginAfterReturn)),
            e2e_case(e2e_active_fleet_count($attackerId) === 0 && e2e_active_fleet_count($supportId) === 0, 'ACS lifecycle leaves no active attacker/support fleet queue tasks'),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'acs_hold_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    $userIds = array_filter(array($attackerId, $defenderId, $supportUser['player_id'] ?? 0), fn($id) => $id > 0);
    $planetIds = array_filter(array($attackerPlanet, $defenderPlanet, $supportUser['planet_id'] ?? 0), fn($id) => $id > 0);
    foreach ($createdPlanets as $planetId) {
        if ($planetId > 0) {
            $planetIds[] = $planetId;
        }
    }
    if (!empty($userIds) && !empty($planetIds)) {
        e2e_cleanup_fleets($userIds, $planetIds);
        e2e_cleanup_relations($userIds);
    }
    foreach ($createdPlanets as $planetId) {
        if ($planetId > 0 && LoadPlanetById($planetId) !== null) {
            DestroyPlanet($planetId);
        }
    }
    if ($attackerId > 0 && $attackerPlanet > 0) {
        e2e_reset_user_and_planet($attackerId, $attackerPlanet, 10, 10);
    }
    if ($defenderId > 0 && $defenderPlanet > 0) {
        e2e_reset_user_and_planet($defenderId, $defenderPlanet, 0, 0);
    }
    if ($supportUser !== null) {
        e2e_remove_user_by_name($supportUser['name']);
    }
}

echo json_encode(array(
    'case_group' => 'http_acs_hold',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
