<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_long_scheduler_e2e.php';
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
loca_add('fleetmsg', 'en');
loca_add('technames', 'en');

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
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$userId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_RECALC_POINTS . "','" . QTYP_UNBAN . "','" . QTYP_ALLOW_ATTACKS . "','" . QTYP_DEBUG . "')");
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
        "`" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=10, `" . GID_B_RES_LAB . "`=10, `" . GID_B_NANITES . "`=0, " .
        "`" . GID_F_SC . "`=10, `" . GID_F_LC . "`=5, `" . GID_F_LF . "`=5, `" . GID_F_PROBE . "`=5, " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=0, maxfields=200, type=" . PTYP_PLANET . ", owner_id={$userId}, remove=0 " .
        "WHERE planet_id={$planetId}"
    );
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$planetId} WHERE player_id={$userId}");
    SelectPlanet($userId, $planetId);
    InvalidateUserCache();
}

function e2e_set_queue_window(int $taskId, int $start, int $end): void
{
    global $db_prefix;

    $queue = LoadQueue($taskId);
    if ($queue === null || $queue === false) {
        return;
    }

    if ($queue['type'] === QTYP_SHIPYARD) {
        $unitSeconds = 1;
        $start = $end - max(1, (int)$queue['level']) * $unitSeconds;
        $end = $start + $unitSeconds;
    }

    dbquery("UPDATE {$db_prefix}queue SET start={$start}, end={$end}, freeze=0, frozen=0 WHERE task_id={$taskId}");
    if ($queue['type'] === QTYP_BUILD || $queue['type'] === QTYP_DEMOLISH) {
        dbquery("UPDATE {$db_prefix}buildqueue SET start={$start}, end={$end} WHERE id=" . (int)$queue['sub_id']);
    }
}

function e2e_update_until_for_users(int $until, array $userIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', $userIds));
    $typeList = "'" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "','" . QTYP_RECALC_POINTS . "'";
    for ($i = 0; $i < 20; $i++) {
        $result = dbquery("SELECT * FROM {$db_prefix}queue WHERE owner_id IN ({$userList}) AND type IN ({$typeList}) AND end <= {$until} AND freeze=0 ORDER BY end ASC, prio DESC LIMIT " . QUEUE_BATCH);
        if (dbrows($result) === 0) {
            break;
        }

        while ($queue = dbarray($result)) {
            switch ($queue['type']) {
                case QTYP_BUILD:
                case QTYP_DEMOLISH:
                    Queue_Build_End($queue);
                    break;
                case QTYP_RESEARCH:
                    Queue_Research_End($queue);
                    break;
                case QTYP_SHIPYARD:
                    Queue_Shipyard_End($queue, $until);
                    break;
                case QTYP_FLEET:
                    Queue_Fleet_End($queue);
                    break;
                case QTYP_RECALC_POINTS:
                    Queue_RecalcPoints_End($queue);
                    break;
                default:
                    RemoveQueue((int)$queue['task_id']);
                    break;
            }
        }
    }
}

function e2e_empty_fleet(): array
{
    global $fleetmap;

    $fleet = array();
    foreach ($fleetmap as $gid) {
        $fleet[$gid] = 0;
    }
    return $fleet;
}

function e2e_empty_resources(): array
{
    global $transportableResources;

    $resources = array();
    foreach ($transportableResources as $rc) {
        $resources[$rc] = 0;
    }
    return $resources;
}

function e2e_planet_snapshot(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, fields, " .
        "`" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deuterium, " .
        "`" . GID_B_METAL_MINE . "` AS metal_mine, `" . GID_F_SC . "` AS small_cargo, `" . GID_F_LF . "` AS light_fighter " .
        "FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1"
    );
}

function e2e_user_snapshot(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT player_id, score1, score2, score3, `" . GID_R_ESPIONAGE . "` AS espionage FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1");
}

function e2e_latest_queue(int $ownerId, string $type): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT * FROM {$db_prefix}queue WHERE owner_id={$ownerId} AND type='" . $type . "' ORDER BY task_id DESC LIMIT 1");
}

function e2e_latest_return_fleet(int $ownerId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT * FROM {$db_prefix}fleet WHERE owner_id={$ownerId} AND mission=" . (FTYP_TRANSPORT + FTYP_RETURN) . " ORDER BY fleet_id DESC LIMIT 1");
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderPlanet = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 0);
$cases = array();

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $now = time();
    $phase1 = $now + 86400;
    $phase2 = $now + 2 * 86400;
    $phase3 = $now + 3 * 86400;
    $phase4 = $now + 4 * 86400;

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet);
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    dbquery("UPDATE {$db_prefix}users SET `" . GID_R_ESPIONAGE . "`=0 WHERE player_id={$attackerId}");
    InvalidateUserCache();

    $originBefore = e2e_planet_snapshot($attackerPlanet);
    $targetBefore = e2e_planet_snapshot($defenderPlanet);

    $buildText = BuildEnque(LoadUser($attackerId), $attackerPlanet, GID_B_METAL_MINE, 0, $now);
    $buildTask = e2e_latest_queue($attackerId, QTYP_BUILD);
    if ($buildTask !== null) {
        e2e_set_queue_window((int)$buildTask['task_id'], $now, $phase1);
    }

    $researchText = StartResearch($attackerId, $attackerPlanet, GID_R_ESPIONAGE, $now);
    $researchTask = e2e_latest_queue($attackerId, QTYP_RESEARCH);
    if ($researchTask !== null) {
        e2e_set_queue_window((int)$researchTask['task_id'], $now, $phase2);
    }

    $shipyardOk = AddShipyard($attackerId, $attackerPlanet, GID_F_LF, 4, $now);
    $shipyardTask = e2e_latest_queue($attackerId, QTYP_SHIPYARD);
    if ($shipyardTask !== null) {
        e2e_set_queue_window((int)$shipyardTask['task_id'], $now, $phase2);
    }

    $fleet = e2e_empty_fleet();
    $fleet[GID_F_SC] = 1;
    $cargo = e2e_empty_resources();
    $cargo[GID_RC_METAL] = 321;
    $cargo[GID_RC_CRYSTAL] = 123;
    $cargo[GID_RC_DEUTERIUM] = 45;
    AdjustShips($fleet, $attackerPlanet, '-');
    AdjustResources($cargo, $attackerPlanet, '-');
    $fleetId = DispatchFleet($fleet, LoadPlanetById($attackerPlanet), LoadPlanetById($defenderPlanet), FTYP_TRANSPORT, 7200, $cargo, 0, $now);
    $outgoingQueue = $fleetId > 0 ? GetFleetQueue($fleetId) : null;
    if ($outgoingQueue !== null && $outgoingQueue !== false) {
        e2e_set_queue_window((int)$outgoingQueue['task_id'], $now, $phase3);
    }

    $recalcTaskId = AddQueue($attackerId, QTYP_RECALC_POINTS, 0, 0, 0, $now, $phase4 - $now, QUEUE_PRIO_RECALC_POINTS);

    e2e_update_until_for_users($phase1, array($attackerId, $defenderId));
    $afterPhase1 = e2e_planet_snapshot($attackerPlanet);

    e2e_update_until_for_users($phase2, array($attackerId, $defenderId));
    $afterPhase2Planet = e2e_planet_snapshot($attackerPlanet);
    $afterPhase2User = e2e_user_snapshot($attackerId);

    e2e_update_until_for_users($phase3, array($attackerId, $defenderId));
    $targetAfterArrival = e2e_planet_snapshot($defenderPlanet);
    $returnFleet = e2e_latest_return_fleet($attackerId);
    $returnQueue = $returnFleet !== null ? GetFleetQueue((int)$returnFleet['fleet_id']) : null;
    $returnFleetCount = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id={$attackerId} AND mission=" . (FTYP_TRANSPORT + FTYP_RETURN));

    e2e_update_until_for_users($phase4, array($attackerId, $defenderId));
    $originAfterReturn = e2e_planet_snapshot($attackerPlanet);
    $targetAfterReturn = e2e_planet_snapshot($defenderPlanet);
    $userAfterRecalc = e2e_user_snapshot($attackerId);
    $remainingQueues = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id IN ({$attackerId},{$defenderId}) AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "','" . QTYP_RECALC_POINTS . "')");
    $remainingFleets = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id IN ({$attackerId},{$defenderId}) OR start_planet IN ({$attackerPlanet},{$defenderPlanet}) OR target_planet IN ({$attackerPlanet},{$defenderPlanet})");

    $cases[] = e2e_finalize_case(array(
        'case' => 'long_scheduler_drains_multi_day_user_queues',
        'checks' => array(
            e2e_case($buildText === '' && $buildTask !== null, 'building queue is created for the long scheduler run', array('message' => $buildText, 'queue' => $buildTask ?? array())),
            e2e_case($researchText === '' && $researchTask !== null, 'research queue is created for the long scheduler run', array('message' => $researchText, 'queue' => $researchTask ?? array())),
            e2e_case($shipyardOk && $shipyardTask !== null, 'shipyard queue is created for the long scheduler run', $shipyardTask ?? array()),
            e2e_case($fleetId > 0 && $outgoingQueue !== null && $outgoingQueue !== false, 'transport fleet queue is created for the long scheduler run', array('fleet_id' => $fleetId, 'queue' => $outgoingQueue ?: array())),
            e2e_case($recalcTaskId > 0, 'recalc-points queue is created for the long scheduler run', array('task_id' => $recalcTaskId)),
            e2e_case($afterPhase1 !== null && (int)$afterPhase1['metal_mine'] === 1, 'day-one scheduler tick completes the building task once', $afterPhase1 ?? array()),
            e2e_case($afterPhase2User !== null && (int)$afterPhase2User['espionage'] === 1, 'day-two scheduler tick completes the research task once', $afterPhase2User ?? array()),
            e2e_case($originBefore !== null && $afterPhase2Planet !== null && (int)$afterPhase2Planet['light_fighter'] === (int)$originBefore['light_fighter'] + 4, 'day-two scheduler tick completes queued ships once', array('before' => $originBefore, 'after' => $afterPhase2Planet)),
            e2e_case($targetBefore !== null && $targetAfterArrival !== null && (int)$targetAfterArrival['metal'] === (int)$targetBefore['metal'] + 321 && (int)$targetAfterArrival['crystal'] === (int)$targetBefore['crystal'] + 123 && (int)$targetAfterArrival['deuterium'] === (int)$targetBefore['deuterium'] + 45, 'day-three scheduler tick delivers transport cargo once', array('before' => $targetBefore, 'after' => $targetAfterArrival)),
            e2e_case($returnFleet !== null && $returnQueue !== null && $returnQueue !== false && $returnFleetCount === 1, 'transport arrival creates exactly one return fleet queue', array('fleet' => $returnFleet ?? array(), 'queue' => $returnQueue ?: array(), 'count' => $returnFleetCount)),
            e2e_case($originBefore !== null && $originAfterReturn !== null && (int)$originAfterReturn['small_cargo'] === (int)$originBefore['small_cargo'], 'day-four scheduler tick returns the transport ship once', array('before' => $originBefore, 'after' => $originAfterReturn)),
            e2e_case($targetAfterArrival !== null && $targetAfterReturn !== null && (int)$targetAfterReturn['metal'] === (int)$targetAfterArrival['metal'] && (int)$targetAfterReturn['crystal'] === (int)$targetAfterArrival['crystal'] && (int)$targetAfterReturn['deuterium'] === (int)$targetAfterArrival['deuterium'], 'return completion does not redeliver transport cargo', array('after_arrival' => $targetAfterArrival, 'after_return' => $targetAfterReturn)),
            e2e_case($userAfterRecalc !== null && (int)$userAfterRecalc['score1'] >= 0 && (int)$userAfterRecalc['score2'] >= 0 && (int)$userAfterRecalc['score3'] >= 0, 'recalc-points completion leaves non-negative score fields', $userAfterRecalc ?? array()),
            e2e_case($remainingQueues === 0, 'all user-owned scheduler queues are drained by the long run', array('remaining_queues' => $remainingQueues)),
            e2e_case($remainingFleets === 0, 'all fixture fleets are drained by the long run', array('remaining_fleets' => $remainingFleets)),
            e2e_case($originAfterReturn !== null && $targetAfterReturn !== null && min((int)$originAfterReturn['metal'], (int)$originAfterReturn['crystal'], (int)$originAfterReturn['deuterium'], (int)$targetAfterReturn['metal'], (int)$targetAfterReturn['crystal'], (int)$targetAfterReturn['deuterium']) >= 0, 'scheduler run leaves fixture resources non-negative', array('origin' => $originAfterReturn, 'target' => $targetAfterReturn)),
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'long_scheduler_exception',
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
    'case_group' => 'http_long_scheduler',
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
