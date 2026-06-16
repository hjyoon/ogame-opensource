<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_planet_cleanup_edges_e2e.php';
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
loca_add('debug', 'en');

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

function e2e_prepare_user_and_planet(int $userId, int $planetId, string $name): void
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
        "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, lastclick=" . time() . " " .
        "WHERE player_id={$userId}"
    );

    $assignments = array();
    foreach (array_merge($buildmap, $fleetmap, $defmap) as $gid) {
        $assignments[] = "`{$gid}`=0";
    }
    dbquery(
        "UPDATE {$db_prefix}planets SET " . implode(',', $assignments) . ", name='" . e2e_sql_escape($name) . "', owner_id={$userId}, type=" . PTYP_PLANET . ", " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "`" . GID_B_SHIPYARD . "`=12, `" . GID_F_SC . "`=10, `" . GID_F_RECYCLER . "`=5, prod1=1, prod2=1, prod3=1, prod4=1, prod12=1, prod212=1, " .
        "fields=0, maxfields=300, lastpeek=" . time() . ", lastakt=" . time() . ", remove=0 WHERE planet_id={$planetId}"
    );
    InvalidateUserCache();
}

function e2e_find_empty_position(array $near): array
{
    global $GlobalUni, $db_prefix;

    $g = (int)$near['g'];
    for ($system = 1; $system <= (int)$GlobalUni['systems']; $system++) {
        for ($p = 1; $p <= 15; $p++) {
            if ($system === (int)$near['s'] && $p === (int)$near['p']) {
                continue;
            }
            $used = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}planets WHERE g={$g} AND s={$system} AND p={$p} AND type IN (" . PTYP_MOON . "," . PTYP_PLANET . "," . PTYP_DEST_PLANET . "," . PTYP_DEST_MOON . "," . PTYP_ABANDONED . ")");
            if ($used === 0) {
                return array($g, $system, $p);
            }
        }
    }

    throw new RuntimeException('No empty planet position is available for cleanup edge fixtures.');
}

function e2e_create_temp_planet(int $ownerId, array $near, string $name): int
{
    global $db_prefix;

    [$g, $s, $p] = e2e_find_empty_position($near);
    $planetId = CreatePlanet($g, $s, $p, $ownerId, 1, 0, 0, time());
    if ($planetId <= 0) {
        throw new RuntimeException('Failed to create temporary planet.');
    }
    e2e_prepare_user_and_planet($ownerId, $planetId, $name);
    dbquery("UPDATE {$db_prefix}planets SET g={$g}, s={$s}, p={$p} WHERE planet_id={$planetId}");
    return $planetId;
}

function e2e_create_empty_debris(int $g, int $s, int $p, int $ownerId): int
{
    global $db_prefix;

    $debrisId = CreateDebris($g, $s, $p, $ownerId);
    if ($debrisId <= 0) {
        throw new RuntimeException('Failed to create debris field.');
    }
    dbquery("UPDATE {$db_prefix}planets SET `" . GID_RC_METAL . "`=0, `" . GID_RC_CRYSTAL . "`=0, `" . GID_RC_DEUTERIUM . "`=0, remove=0 WHERE planet_id={$debrisId}");
    return $debrisId;
}

function e2e_dispatch_direct(int $ownerId, int $originId, int $targetId, int $mission, array $ships, int $startOffset = 0, int $seconds = 600): int
{
    global $fleetmap, $transportableResources;

    $fleet = array();
    foreach ($fleetmap as $gid) {
        $fleet[$gid] = (int)($ships[$gid] ?? 0);
    }
    $resources = array();
    foreach ($transportableResources as $rc) {
        $resources[$rc] = 0;
    }
    AdjustShips($fleet, $originId, '-');
    return DispatchFleet($fleet, LoadPlanetById($originId), LoadPlanetById($targetId), $mission, $seconds, $resources, 0, time() + $startOffset);
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderPlanet = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$cases = array();
$createdPlanets = array();
$createdDebris = array();
$runStart = time();

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    dbquery("UPDATE {$db_prefix}uni SET freeze=0");
    $GlobalUni = LoadUniverse();
    e2e_prepare_user_and_planet($attackerId, $attackerPlanet, 'E2E Cleanup A');
    e2e_prepare_user_and_planet($defenderId, $defenderPlanet, 'E2E Cleanup D');

    $attackerHome = LoadPlanetById($attackerPlanet);
    $removeTargetId = e2e_create_temp_planet($defenderId, $attackerHome, 'E2E Cleanup Removed');
    $createdPlanets[] = $removeTargetId;
    dbquery("UPDATE {$db_prefix}planets SET remove=" . (time() - 10) . " WHERE planet_id={$removeTargetId}");
    $fleetId = e2e_dispatch_direct($attackerId, $attackerPlanet, $removeTargetId, FTYP_TRANSPORT, array(GID_F_SC => 1), -100, 600);
    Queue_CleanPlanets_End(array('task_id' => 0, 'end' => time()));
    $targetAfterClean = LoadPlanetById($removeTargetId);
    $outboundAfterClean = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE fleet_id={$fleetId}");
    $returnFleet = e2e_one_row("SELECT fleet_id, owner_id, mission, start_planet, target_planet, `" . GID_F_SC . "` AS small_cargo FROM {$db_prefix}fleet WHERE owner_id={$attackerId} AND mission=" . (FTYP_TRANSPORT + FTYP_RETURN) . " ORDER BY fleet_id DESC LIMIT 1");
    $returnQueue = $returnFleet === null ? null : e2e_one_row("SELECT task_id, type, owner_id, sub_id FROM {$db_prefix}queue WHERE type='" . QTYP_FLEET . "' AND sub_id=" . (int)$returnFleet['fleet_id'] . " LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'removed_planet_cleanup_recalls_inbound_fleet_before_delete',
        'checks' => array(
            e2e_case($fleetId > 0, 'setup creates an inbound fleet to the planet scheduled for removal', array('fleet_id' => $fleetId, 'target_planet' => $removeTargetId)),
            e2e_case($targetAfterClean === null, 'cleanup deletes the planet whose remove timestamp is due', array('target_planet' => $removeTargetId)),
            e2e_case($outboundAfterClean === 0, 'cleanup removes the original outbound fleet row', array('outbound_count' => $outboundAfterClean)),
            e2e_case($returnFleet !== null && (int)$returnFleet['start_planet'] === $attackerPlanet && (int)$returnFleet['small_cargo'] === 1, 'cleanup creates a returning fleet for recalled ships', $returnFleet ?? array()),
            e2e_case($returnQueue !== null, 'returning recalled fleet has a queue task', $returnQueue ?? array()),
        ),
    ));

    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet, $removeTargetId));
    e2e_prepare_user_and_planet($attackerId, $attackerPlanet, 'E2E Cleanup A');
    $origin = LoadPlanetById($attackerPlanet);
    [$g1, $s1, $p1] = e2e_find_empty_position($origin);
    $activeDebrisId = e2e_create_empty_debris($g1, $s1, $p1, $attackerId);
    $createdDebris[] = $activeDebrisId;
    [$g2, $s2, $p2] = e2e_find_empty_position(array('g' => $g1, 's' => $s1, 'p' => $p1));
    $inactiveDebrisId = e2e_create_empty_debris($g2, $s2, $p2, $attackerId);
    $createdDebris[] = $inactiveDebrisId;
    $recyclerFleetId = e2e_dispatch_direct($attackerId, $attackerPlanet, $activeDebrisId, FTYP_RECYCLE, array(GID_F_RECYCLER => 1), 0, 1200);
    Queue_CleanDebris_End(array('task_id' => 0));
    $activeDebrisAfter = LoadPlanetById($activeDebrisId);
    $inactiveDebrisAfter = LoadPlanetById($inactiveDebrisId);
    $recyclerFleetAfter = e2e_one_row("SELECT fleet_id, mission, target_planet FROM {$db_prefix}fleet WHERE fleet_id={$recyclerFleetId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'debris_cleanup_preserves_active_recycler_target_and_removes_inactive_empty_debris',
        'checks' => array(
            e2e_case($recyclerFleetId > 0, 'setup creates an active recycler fleet targeting empty debris', array('fleet_id' => $recyclerFleetId, 'active_debris' => $activeDebrisId)),
            e2e_case($activeDebrisAfter !== null && (int)$activeDebrisAfter['type'] === PTYP_DF, 'empty debris targeted by active recycler is preserved', $activeDebrisAfter ?? array()),
            e2e_case($inactiveDebrisAfter === null, 'empty debris without active recycler traffic is removed', array('inactive_debris' => $inactiveDebrisId)),
            e2e_case($recyclerFleetAfter !== null && (int)$recyclerFleetAfter['target_planet'] === $activeDebrisId, 'active recycler fleet remains targeted at preserved debris', $recyclerFleetAfter ?? array()),
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'planet_cleanup_edges_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e), 'trace' => $e->getTraceAsString()))),
        'pass' => false,
    );
} finally {
    $userIds = array_filter(array($attackerId, $defenderId), fn($id) => $id > 0);
    $planetIds = array_filter(array_merge(array($attackerPlanet, $defenderPlanet), $createdPlanets, $createdDebris), fn($id) => $id > 0);
    if (!empty($userIds) && !empty($planetIds)) {
        e2e_cleanup_fleets($userIds, $planetIds);
    }
    foreach (array_reverse($createdDebris) as $debrisId) {
        if ($debrisId > 0 && LoadPlanetById($debrisId) !== null) {
            DestroyPlanet($debrisId);
        }
    }
    foreach (array_reverse($createdPlanets) as $planetId) {
        if ($planetId > 0 && LoadPlanetById($planetId) !== null) {
            DestroyPlanet($planetId);
        }
    }
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id=" . USER_SPACE . " AND type IN ('" . QTYP_CLEAN_DEBRIS . "','" . QTYP_CLEAN_PLANETS . "') AND start >= {$runStart}");
    if ($attackerId > 0 && $attackerPlanet > 0) {
        e2e_prepare_user_and_planet($attackerId, $attackerPlanet, 'E2E Fixture Home');
        SelectPlanet($attackerId, $attackerPlanet);
    }
    if ($defenderId > 0 && $defenderPlanet > 0) {
        e2e_prepare_user_and_planet($defenderId, $defenderPlanet, 'E2E Fixture Defender');
    }
}

echo json_encode(array(
    'case_group' => 'http_planet_cleanup_edges',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
