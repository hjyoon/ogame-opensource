<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_queue_idempotency_e2e.php';
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
        "`" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=10, `" . GID_B_RES_LAB . "`=10, `" . GID_B_NANITES . "`=0, " .
        "`" . GID_F_SC . "`=10, `" . GID_F_LC . "`=5, `" . GID_F_LF . "`=5, `" . GID_F_PROBE . "`=5, " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=0, maxfields=200, type=" . PTYP_PLANET . ", owner_id={$userId} " .
        "WHERE planet_id={$planetId}"
    );
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$planetId} WHERE player_id={$userId}");
    SelectPlanet($userId, $planetId);
    InvalidateUserCache();
}

function e2e_force_queue_due(int $taskId, int $now): void
{
    global $db_prefix;

    $queue = LoadQueue($taskId);
    if ($queue === null || $queue === false) {
        return;
    }

    $start = $now - 10;
    $end = $now - 9;
    dbquery("UPDATE {$db_prefix}queue SET start={$start}, end={$end}, freeze=0, frozen=0 WHERE task_id={$taskId}");

    if ($queue['type'] === QTYP_BUILD || $queue['type'] === QTYP_DEMOLISH) {
        dbquery("UPDATE {$db_prefix}buildqueue SET start={$start}, end={$end} WHERE id=" . (int)$queue['sub_id']);
    }
}

function e2e_update_queue_twice(int $now): void
{
    UpdateQueue($now);
    UpdateQueue($now);
}

function e2e_planet_snapshot(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, fields, " .
        "`" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deuterium, " .
        "`" . GID_B_METAL_MINE . "` AS metal_mine, `" . GID_F_SC . "` AS small_cargo " .
        "FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1"
    );
}

function e2e_user_snapshot(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT player_id, `" . GID_R_ESPIONAGE . "` AS espionage FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1");
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

function e2e_latest_fleet(int $ownerId, int $mission): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT * FROM {$db_prefix}fleet WHERE owner_id={$ownerId} AND mission={$mission} ORDER BY fleet_id DESC LIMIT 1");
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

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    $buildText = BuildEnque(LoadUser($attackerId), $attackerPlanet, GID_B_METAL_MINE, 0, $now);
    $buildTask = e2e_one_row("SELECT task_id FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "' ORDER BY task_id DESC LIMIT 1");
    if ($buildTask !== null) {
        e2e_force_queue_due((int)$buildTask['task_id'], $now);
        e2e_update_queue_twice($now);
    }
    $planetAfterBuild = e2e_planet_snapshot($attackerPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'build_completion_update_queue_is_idempotent',
        'checks' => array(
            e2e_case($buildText === '', 'building enqueue succeeds', array('message' => $buildText)),
            e2e_case($buildTask !== null, 'build queue task is created', $buildTask ?? array()),
            e2e_case($planetAfterBuild !== null && (int)$planetAfterBuild['metal_mine'] === 1, 'metal mine is completed exactly once', $planetAfterBuild ?? array()),
            e2e_case($planetAfterBuild !== null && (int)$planetAfterBuild['fields'] === 1, 'used fields are incremented exactly once', $planetAfterBuild ?? array()),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "'") === 0, 'build queue task is removed after completion'),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}buildqueue WHERE owner_id={$attackerId}") === 0, 'buildqueue row is removed after completion'),
        ),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    dbquery("UPDATE {$db_prefix}users SET `" . GID_R_ESPIONAGE . "`=0 WHERE player_id={$attackerId}");
    InvalidateUserCache();
    $researchText = StartResearch($attackerId, $attackerPlanet, GID_R_ESPIONAGE, $now + 1);
    $researchTask = e2e_one_row("SELECT task_id FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_RESEARCH . "' ORDER BY task_id DESC LIMIT 1");
    if ($researchTask !== null) {
        e2e_force_queue_due((int)$researchTask['task_id'], $now);
        e2e_update_queue_twice($now);
    }
    $userAfterResearch = e2e_user_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'research_completion_update_queue_is_idempotent',
        'checks' => array(
            e2e_case($researchText === '', 'research enqueue succeeds', array('message' => $researchText)),
            e2e_case($researchTask !== null, 'research queue task is created', $researchTask ?? array()),
            e2e_case($userAfterResearch !== null && (int)$userAfterResearch['espionage'] === 1, 'espionage research is completed exactly once', $userAfterResearch ?? array()),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_RESEARCH . "'") === 0, 'research queue task is removed after completion'),
        ),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    dbquery("UPDATE {$db_prefix}planets SET `" . GID_F_SC . "`=0 WHERE planet_id={$attackerPlanet}");
    $shipyardOk = AddShipyard($attackerId, $attackerPlanet, GID_F_SC, 3, $now + 2);
    $shipyardTask = e2e_one_row("SELECT task_id, level FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_SHIPYARD . "' ORDER BY task_id DESC LIMIT 1");
    if ($shipyardTask !== null) {
        e2e_force_queue_due((int)$shipyardTask['task_id'], $now);
        e2e_update_queue_twice($now);
    }
    $planetAfterShipyard = e2e_planet_snapshot($attackerPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'shipyard_completion_update_queue_is_idempotent',
        'checks' => array(
            e2e_case($shipyardOk, 'shipyard enqueue succeeds'),
            e2e_case($shipyardTask !== null && (int)$shipyardTask['level'] === 3, 'shipyard queue task is created for three ships', $shipyardTask ?? array()),
            e2e_case($planetAfterShipyard !== null && (int)$planetAfterShipyard['small_cargo'] === 3, 'small cargos are completed exactly once', $planetAfterShipyard ?? array()),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_SHIPYARD . "'") === 0, 'shipyard queue task is removed after completion'),
        ),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet);
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));

    $originBefore = e2e_planet_snapshot($attackerPlanet);
    $targetBefore = e2e_planet_snapshot($defenderPlanet);
    $origin = LoadPlanetById($attackerPlanet);
    $target = LoadPlanetById($defenderPlanet);
    $fleet = e2e_empty_fleet();
    $fleet[GID_F_SC] = 1;
    $cargo = e2e_empty_resources();
    $cargo[GID_RC_METAL] = 123;
    $cargo[GID_RC_CRYSTAL] = 45;
    AdjustShips($fleet, $attackerPlanet, '-');
    AdjustResources($cargo, $attackerPlanet, '-');
    $fleetId = DispatchFleet($fleet, $origin, $target, FTYP_TRANSPORT, 3600, $cargo, 0, $now + 3);
    $outgoingQueue = $fleetId > 0 ? GetFleetQueue($fleetId) : null;
    if ($outgoingQueue !== null && $outgoingQueue !== false) {
        e2e_force_queue_due((int)$outgoingQueue['task_id'], $now);
        e2e_update_queue_twice($now);
    }
    $targetAfterArrival = e2e_planet_snapshot($defenderPlanet);
    $returnFleet = e2e_latest_fleet($attackerId, FTYP_TRANSPORT + FTYP_RETURN);
    $returnQueue = $returnFleet ? GetFleetQueue((int)$returnFleet['fleet_id']) : null;
    $returnCountAfterArrival = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id={$attackerId} AND mission=" . (FTYP_TRANSPORT + FTYP_RETURN));
    if ($returnQueue !== null && $returnQueue !== false) {
        e2e_force_queue_due((int)$returnQueue['task_id'], $now);
        e2e_update_queue_twice($now);
    }
    $originAfterReturn = e2e_planet_snapshot($attackerPlanet);
    $targetAfterReturn = e2e_planet_snapshot($defenderPlanet);

    $cases[] = e2e_finalize_case(array(
        'case' => 'fleet_transport_update_queue_is_idempotent',
        'checks' => array(
            e2e_case($fleetId > 0, 'transport fleet is dispatched', array('fleet_id' => $fleetId)),
            e2e_case($outgoingQueue !== null && $outgoingQueue !== false, 'outgoing transport queue task is created', $outgoingQueue ?: array()),
            e2e_case($originBefore !== null && $targetBefore !== null, 'origin and target snapshots are available', array('origin' => $originBefore, 'target' => $targetBefore)),
            e2e_case($targetBefore !== null && $targetAfterArrival !== null && (int)$targetAfterArrival['metal'] === (int)$targetBefore['metal'] + 123, 'transport arrival delivers metal exactly once', array('before' => $targetBefore, 'after_arrival' => $targetAfterArrival)),
            e2e_case($targetBefore !== null && $targetAfterArrival !== null && (int)$targetAfterArrival['crystal'] === (int)$targetBefore['crystal'] + 45, 'transport arrival delivers crystal exactly once', array('before' => $targetBefore, 'after_arrival' => $targetAfterArrival)),
            e2e_case($returnFleet !== null && (int)$returnFleet['mission'] === FTYP_TRANSPORT + FTYP_RETURN, 'transport arrival creates one return fleet', $returnFleet ?? array()),
            e2e_case($returnCountAfterArrival === 1, 'second UpdateQueue call does not duplicate the return fleet', array('return_fleets' => $returnCountAfterArrival)),
            e2e_case($returnQueue !== null && $returnQueue !== false, 'return fleet queue task is created', $returnQueue ?: array()),
            e2e_case($originBefore !== null && $originAfterReturn !== null && (int)$originAfterReturn['small_cargo'] === (int)$originBefore['small_cargo'], 'return completion restores the ship exactly once', array('before' => $originBefore, 'after_return' => $originAfterReturn)),
            e2e_case($targetAfterArrival !== null && $targetAfterReturn !== null && (int)$targetAfterReturn['metal'] === (int)$targetAfterArrival['metal'] && (int)$targetAfterReturn['crystal'] === (int)$targetAfterArrival['crystal'], 'return completion does not redeliver transport cargo', array('after_arrival' => $targetAfterArrival, 'after_return' => $targetAfterReturn)),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id={$attackerId}") === 0, 'fleet rows are removed after idempotent return completion'),
            e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_FLEET . "'") === 0, 'fleet queue rows are removed after idempotent return completion'),
        ),
    ));

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet);
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    dbquery("UPDATE {$db_prefix}planets SET `" . GID_F_SC . "`=2 WHERE planet_id={$attackerPlanet}");

    $sameTickTargetBefore = e2e_planet_snapshot($defenderPlanet);
    $sameTickFleetIds = array();
    $sameTickQueues = array();
    $sameTickCargos = array(
        array(GID_RC_METAL => 111, GID_RC_CRYSTAL => 22, GID_RC_DEUTERIUM => 3),
        array(GID_RC_METAL => 222, GID_RC_CRYSTAL => 44, GID_RC_DEUTERIUM => 6),
    );
    foreach ($sameTickCargos as $cargoSpec) {
        $origin = LoadPlanetById($attackerPlanet);
        $target = LoadPlanetById($defenderPlanet);
        $fleet = e2e_empty_fleet();
        $fleet[GID_F_SC] = 1;
        $cargo = e2e_empty_resources();
        foreach ($cargoSpec as $rc => $amount) {
            $cargo[$rc] = $amount;
        }
        AdjustShips($fleet, $attackerPlanet, '-');
        AdjustResources($cargo, $attackerPlanet, '-');
        $sameTickFleetId = DispatchFleet($fleet, $origin, $target, FTYP_TRANSPORT, 3600, $cargo, 0, $now + 4);
        $sameTickFleetIds[] = $sameTickFleetId;
        $sameTickQueue = $sameTickFleetId > 0 ? GetFleetQueue($sameTickFleetId) : null;
        $sameTickQueues[] = $sameTickQueue;
        if ($sameTickQueue !== null && $sameTickQueue !== false) {
            e2e_force_queue_due((int)$sameTickQueue['task_id'], $now);
        }
    }
    e2e_update_queue_twice($now);
    $sameTickTargetAfter = e2e_planet_snapshot($defenderPlanet);
    $sameTickReturnCount = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id={$attackerId} AND mission=" . (FTYP_TRANSPORT + FTYP_RETURN));
    $sameTickOutgoingCount = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id={$attackerId} AND mission=" . FTYP_TRANSPORT);
    $sameTickFleetQueueCount = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_FLEET . "'");
    $sameTickQueuedReturnCount = e2e_count(
        "SELECT COUNT(*) AS cnt FROM {$db_prefix}queue q " .
        "JOIN {$db_prefix}fleet f ON f.fleet_id=q.sub_id " .
        "WHERE q.owner_id={$attackerId} AND q.type='" . QTYP_FLEET . "' AND f.mission=" . (FTYP_TRANSPORT + FTYP_RETURN)
    );
    $sameTickFleetIdsValid = count(array_filter($sameTickFleetIds, fn($id) => $id > 0)) === 2;
    $sameTickQueuesValid = count(array_filter($sameTickQueues, fn($queue) => $queue !== null && $queue !== false)) === 2;
    $sameTickMetal = array_sum(array_map(fn($cargo) => $cargo[GID_RC_METAL], $sameTickCargos));
    $sameTickCrystal = array_sum(array_map(fn($cargo) => $cargo[GID_RC_CRYSTAL], $sameTickCargos));
    $sameTickDeuterium = array_sum(array_map(fn($cargo) => $cargo[GID_RC_DEUTERIUM], $sameTickCargos));

    $cases[] = e2e_finalize_case(array(
        'case' => 'same_tick_transport_arrivals_are_processed_once_each',
        'checks' => array(
            e2e_case($sameTickFleetIdsValid, 'two same-tick transport fleets are dispatched', array('fleet_ids' => $sameTickFleetIds)),
            e2e_case($sameTickQueuesValid, 'two same-tick outgoing queue tasks are created', array('queues' => $sameTickQueues)),
            e2e_case($sameTickTargetBefore !== null && $sameTickTargetAfter !== null && (int)$sameTickTargetAfter['metal'] === (int)$sameTickTargetBefore['metal'] + $sameTickMetal, 'same-tick transports deliver combined metal exactly once', array('before' => $sameTickTargetBefore, 'after' => $sameTickTargetAfter)),
            e2e_case($sameTickTargetBefore !== null && $sameTickTargetAfter !== null && (int)$sameTickTargetAfter['crystal'] === (int)$sameTickTargetBefore['crystal'] + $sameTickCrystal, 'same-tick transports deliver combined crystal exactly once', array('before' => $sameTickTargetBefore, 'after' => $sameTickTargetAfter)),
            e2e_case($sameTickTargetBefore !== null && $sameTickTargetAfter !== null && (int)$sameTickTargetAfter['deuterium'] === (int)$sameTickTargetBefore['deuterium'] + $sameTickDeuterium, 'same-tick transports deliver combined deuterium exactly once', array('before' => $sameTickTargetBefore, 'after' => $sameTickTargetAfter)),
            e2e_case($sameTickOutgoingCount === 0, 'same-tick outgoing transport fleet rows are removed after arrival', array('outgoing_fleets' => $sameTickOutgoingCount)),
            e2e_case($sameTickReturnCount === 2, 'same-tick transport arrivals create one return fleet per outgoing fleet', array('return_fleets' => $sameTickReturnCount)),
            e2e_case($sameTickFleetQueueCount === 2 && $sameTickQueuedReturnCount === 2, 'second UpdateQueue call does not duplicate or remove pending same-tick return queues', array('fleet_queue_rows' => $sameTickFleetQueueCount, 'queued_returns' => $sameTickQueuedReturnCount)),
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'queue_idempotency_exception',
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
    'case_group' => 'http_queue_idempotency',
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
