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

function e2e_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

function e2e_user_row(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT player_id, name, admin, disable, disable_until, validated, deact_ip, vacation, vacation_until, " .
        "score1, score2, score3, place1, place2, place3, oldscore1, oldscore2, oldscore3, " .
        "oldplace1, oldplace2, oldplace3, scoredate, hplanetid " .
        "FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1"
    );
}

function e2e_planet_row(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, type, g, s, p, remove, " .
        "`" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deuterium, " .
        "`" . GID_F_SC . "` AS small_cargo, `" . GID_F_RECYCLER . "` AS recycler " .
        "FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1"
    );
}

function e2e_queue_count(string $type, int $ownerId = -1): int
{
    global $db_prefix;
    $where = "type='" . e2e_sql_escape($type) . "'";
    if ($ownerId >= 0) {
        $where .= " AND owner_id={$ownerId}";
    }
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE {$where}");
}

function e2e_due_queue_count(int $until): int
{
    global $db_prefix;
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE end <= {$until} AND freeze=0");
}

function e2e_due_queue_count_for_users(int $until, array $userIds, array $types): int
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', array_filter($userIds, fn($id) => $id > 0)));
    if ($userList === '' || empty($types)) {
        return 0;
    }
    $quoted = array();
    foreach ($types as $type) {
        $quoted[] = "'" . e2e_sql_escape($type) . "'";
    }
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id IN ({$userList}) AND type IN (" . implode(',', $quoted) . ") AND end <= {$until} AND freeze=0");
}

function e2e_clear_queues(array $types, int $ownerId = -1): void
{
    global $db_prefix;

    if (empty($types)) {
        return;
    }

    $quoted = array();
    foreach ($types as $type) {
        $quoted[] = "'" . e2e_sql_escape($type) . "'";
    }
    $where = "type IN (" . implode(',', $quoted) . ")";
    if ($ownerId >= 0) {
        $where .= " AND owner_id={$ownerId}";
    }
    dbquery("DELETE FROM {$db_prefix}queue WHERE {$where}");
}

function e2e_add_due_queue(int $ownerId, string $type, int $prio = QUEUE_PRIO_LOWEST, int $now = 0): int
{
    if ($now === 0) {
        $now = time();
    }
    return AddQueue($ownerId, $type, 0, 0, 0, $now - 10, 1, $prio);
}

function e2e_update_queue_until_idle(int $until, int $maxRuns = 20): array
{
    $runs = array();
    for ($i = 0; $i < $maxRuns; $i++) {
        $before = e2e_due_queue_count($until);
        if ($before === 0) {
            break;
        }

        UpdateQueue($until);
        $after = e2e_due_queue_count($until);
        $runs[] = array('run' => $i + 1, 'before_due' => $before, 'after_due' => $after);
    }

    return array(
        'runs' => $runs,
        'remaining_due' => e2e_due_queue_count($until),
        'max_runs' => $maxRuns,
    );
}

function e2e_update_queue_until_users_idle(int $until, array $userIds, array $types, int $maxRuns = 20): array
{
    $runs = array();
    for ($i = 0; $i < $maxRuns; $i++) {
        $before = e2e_due_queue_count_for_users($until, $userIds, $types);
        if ($before === 0) {
            break;
        }

        UpdateQueue($until);
        $after = e2e_due_queue_count_for_users($until, $userIds, $types);
        $runs[] = array('run' => $i + 1, 'before_due' => $before, 'after_due' => $after);
    }

    return array(
        'runs' => $runs,
        'remaining_due' => e2e_due_queue_count_for_users($until, $userIds, $types),
        'max_runs' => $maxRuns,
    );
}

function e2e_find_free_planet_slot(array &$reserved = array()): array
{
    global $GlobalUni;

    for ($g = 1; $g <= (int)$GlobalUni['galaxies']; $g++) {
        for ($s = 1; $s <= (int)$GlobalUni['systems']; $s++) {
            for ($p = 15; $p >= 1; $p--) {
                $key = "{$g}:{$s}:{$p}";
                if (!isset($reserved[$key]) && !HasPlanet($g, $s, $p)) {
                    $reserved[$key] = true;
                    return array($g, $s, $p);
                }
            }
        }
    }

    throw new RuntimeException('No empty planet slot found for long scheduler test.');
}

function e2e_find_free_debris_slot(array &$reserved = array()): array
{
    global $GlobalUni;

    for ($g = 1; $g <= (int)$GlobalUni['galaxies']; $g++) {
        for ($s = 1; $s <= (int)$GlobalUni['systems']; $s++) {
            for ($p = 1; $p <= 15; $p++) {
                $key = "{$g}:{$s}:{$p}";
                if (!isset($reserved[$key]) && HasDebris($g, $s, $p) === 0) {
                    $reserved[$key] = true;
                    return array($g, $s, $p);
                }
            }
        }
    }

    throw new RuntimeException('No empty debris slot found for long scheduler test.');
}

function e2e_create_debris(array $coords, int $ownerId, int $metal, int $crystal): int
{
    global $db_prefix;

    $id = CreateDebris($coords[0], $coords[1], $coords[2], $ownerId);
    dbquery("UPDATE {$db_prefix}planets SET `" . GID_RC_METAL . "`={$metal}, `" . GID_RC_CRYSTAL . "`={$crystal}, `" . GID_RC_DEUTERIUM . "`=0 WHERE planet_id={$id}");
    return $id;
}

function e2e_remove_user_if_exists(int $userId): void
{
    global $db_prefix;

    if ($userId <= 0 || e2e_user_row($userId) === null) {
        return;
    }
    dbquery("UPDATE {$db_prefix}users SET admin=0 WHERE player_id={$userId}");
    InvalidateUserCache();
    RemoveUser($userId, time());
}

function e2e_create_temp_user(string $suffix): array
{
    global $db_prefix, $resmap;

    $safeSuffix = substr(preg_replace('/[^a-z0-9_]/', '', mb_strtolower($suffix, 'UTF-8')), 0, 10);
    $name = 'e2els_' . $safeSuffix;
    $existing = e2e_one_row("SELECT player_id FROM {$db_prefix}users WHERE name='" . e2e_sql_escape(mb_strtolower($name, 'UTF-8')) . "' LIMIT 1");
    if ($existing !== null) {
        e2e_remove_user_if_exists((int)$existing['player_id']);
    }

    $id = CreateUser($name, 'E2E_test123', $name . '@example.local', false);
    $user = e2e_user_row($id);
    if ($user === null) {
        throw new RuntimeException("Failed to create temporary user {$name}");
    }

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=0";
    }
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", " .
        "validated=1, validatemd='', deact_ip=1, admin=0, vacation=0, vacation_until=0, " .
        "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, " .
        "lang='en', skin='/evolution/', useskin=1, dm=0, dmfree=0 WHERE player_id={$id}"
    );
    InvalidateUserCache();

    return array(
        'player_id' => $id,
        'planet_id' => (int)$user['hplanetid'],
        'name' => $name,
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
$createdUsers = array();
$createdPlanets = array();
$createdDebris = array();
$reservedPlanetSlots = array();
$reservedDebrisSlots = array();
$uniSnapshot = $GlobalUni;

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

    e2e_reset_user_and_planet($attackerId, $attackerPlanet);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet);
    e2e_cleanup_fleets(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    dbquery("UPDATE {$db_prefix}uni SET freeze=0, speed=128, fspeed=1024");
    $GlobalUni = LoadUniverse();

    $maintenanceTypes = array(QTYP_CLEAN_DEBRIS, QTYP_CLEAN_PLANETS, QTYP_CLEAN_PLAYERS, QTYP_UPDATE_STATS, QTYP_RECALC_ALLY_POINTS);
    e2e_clear_queues($maintenanceTypes);

    $emptyDebrisId = e2e_create_debris(e2e_find_free_debris_slot($reservedDebrisSlots), USER_SPACE, 0, 0);
    $createdDebris[] = $emptyDebrisId;
    $richDebrisId = e2e_create_debris(e2e_find_free_debris_slot($reservedDebrisSlots), USER_SPACE, 75, 125);
    $createdDebris[] = $richDebrisId;

    $expiredCoords = e2e_find_free_planet_slot($reservedPlanetSlots);
    $futureCoords = e2e_find_free_planet_slot($reservedPlanetSlots);
    $expiredPlanetId = CreatePlanet($expiredCoords[0], $expiredCoords[1], $expiredCoords[2], $attackerId, 1, 0, 0, $now);
    $createdPlanets[] = $expiredPlanetId;
    $futurePlanetId = CreatePlanet($futureCoords[0], $futureCoords[1], $futureCoords[2], $attackerId, 1, 0, 0, $now);
    $createdPlanets[] = $futurePlanetId;
    dbquery("UPDATE {$db_prefix}planets SET type=" . PTYP_DEST_PLANET . ", remove=" . ($now - 20) . " WHERE planet_id={$expiredPlanetId}");
    dbquery("UPDATE {$db_prefix}planets SET type=" . PTYP_DEST_PLANET . ", remove=" . ($now + 86400) . " WHERE planet_id={$futurePlanetId}");

    $disabledUser = e2e_create_temp_user('disabled');
    $adminUser = e2e_create_temp_user('admin');
    $createdUsers[] = (int)$disabledUser['player_id'];
    $createdUsers[] = (int)$adminUser['player_id'];
    dbquery("UPDATE {$db_prefix}users SET disable=1, disable_until=" . ($now - 20) . ", admin=0, lastclick={$now}, dm=0 WHERE player_id=" . (int)$disabledUser['player_id']);
    dbquery("UPDATE {$db_prefix}users SET disable=1, disable_until=" . ($now - 20) . ", admin=1, lastclick={$now}, dm=0 WHERE player_id=" . (int)$adminUser['player_id']);

    dbquery("UPDATE {$db_prefix}users SET score1=777, score2=88, score3=9, place1=4, place2=5, place3=6, oldscore1=0, oldscore2=0, oldscore3=0, oldplace1=0, oldplace2=0, oldplace3=0, scoredate=0 WHERE player_id={$attackerId}");
    $cleanDebrisTask = e2e_add_due_queue(USER_SPACE, QTYP_CLEAN_DEBRIS, QUEUE_PRIO_CLEAN_DEBRIS, $now);
    $cleanPlanetsTask = e2e_add_due_queue(USER_SPACE, QTYP_CLEAN_PLANETS, QUEUE_PRIO_CLEAN_PLANETS, $now);
    $cleanPlayersTask = e2e_add_due_queue(USER_SPACE, QTYP_CLEAN_PLAYERS, QUEUE_PRIO_CLEAN_PLAYERS, $now);
    $updateStatsTask = e2e_add_due_queue(USER_SPACE, QTYP_UPDATE_STATS, QUEUE_PRIO_UPDATE_STATS, $now);
    $recalcAllyTask = e2e_add_due_queue(USER_SPACE, QTYP_RECALC_ALLY_POINTS, QUEUE_PRIO_RECALC_ALLY_POINTS, $now);
    $updateStatsQueue = LoadQueue($updateStatsTask);
    $maintenanceDrain = e2e_update_queue_until_idle($now, 10);

    $emptyDebrisAfter = e2e_planet_row($emptyDebrisId);
    $richDebrisAfter = e2e_planet_row($richDebrisId);
    $expiredPlanetAfter = e2e_planet_row($expiredPlanetId);
    $futurePlanetAfter = e2e_planet_row($futurePlanetId);
    $disabledUserAfter = e2e_user_row((int)$disabledUser['player_id']);
    $disabledPlanetAfter = e2e_planet_row((int)$disabledUser['planet_id']);
    $adminUserAfter = e2e_user_row((int)$adminUser['player_id']);
    $statsUserAfter = e2e_user_row($attackerId);
    $expectedScoreDate = $updateStatsQueue === null || $updateStatsQueue === false ? 0 : (int)$updateStatsQueue['end'];
    $futureCleanDebris = e2e_one_row("SELECT task_id, end FROM {$db_prefix}queue WHERE type='" . QTYP_CLEAN_DEBRIS . "' ORDER BY end DESC LIMIT 1");
    $futureCleanPlanets = e2e_one_row("SELECT task_id, end FROM {$db_prefix}queue WHERE type='" . QTYP_CLEAN_PLANETS . "' ORDER BY end DESC LIMIT 1");
    $futureCleanPlayers = e2e_one_row("SELECT task_id, end FROM {$db_prefix}queue WHERE type='" . QTYP_CLEAN_PLAYERS . "' ORDER BY end DESC LIMIT 1");
    $futureUpdateStats = e2e_one_row("SELECT task_id, end FROM {$db_prefix}queue WHERE type='" . QTYP_UPDATE_STATS . "' ORDER BY end DESC LIMIT 1");
    $recalcAllyRemaining = e2e_queue_count(QTYP_RECALC_ALLY_POINTS);
    $createdUsers = array_values(array_filter($createdUsers, fn($id) => $id !== (int)$disabledUser['player_id']));
    $createdPlanets = array_values(array_filter($createdPlanets, fn($id) => $id !== $expiredPlanetId));
    e2e_clear_queues($maintenanceTypes);

    $fleetOriginBefore = e2e_planet_snapshot($attackerPlanet);
    $fleetTargetBefore = e2e_planet_snapshot($defenderPlanet);
    $fleet = e2e_empty_fleet();
    $fleet[GID_F_SC] = 1;
    $cargo = e2e_empty_resources();
    $cargo[GID_RC_METAL] = 222;
    $cargo[GID_RC_CRYSTAL] = 111;
    $cargo[GID_RC_DEUTERIUM] = 33;
    AdjustShips($fleet, $attackerPlanet, '-');
    AdjustResources($cargo, $attackerPlanet, '-');
    $fleetId = DispatchFleet($fleet, LoadPlanetById($attackerPlanet), LoadPlanetById($defenderPlanet), FTYP_TRANSPORT, 300, $cargo, 0, $now);
    $fleetQueue = $fleetId > 0 ? GetFleetQueue($fleetId) : null;
    $fleetDrain = e2e_update_queue_until_users_idle($now + 700, array($attackerId, $defenderId), array(QTYP_FLEET), 10);
    $fleetRecalcTask = e2e_add_due_queue($attackerId, QTYP_RECALC_POINTS, QUEUE_PRIO_RECALC_POINTS, $now);
    $postFleetRecalcDrain = e2e_update_queue_until_users_idle($now, array($attackerId), array(QTYP_RECALC_POINTS), 5);
    $fleetOriginAfter = e2e_planet_snapshot($attackerPlanet);
    $fleetTargetAfter = e2e_planet_snapshot($defenderPlanet);
    $fleetUserAfter = e2e_user_snapshot($attackerId);
    $remainingFixtureFleets = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id IN ({$attackerId},{$defenderId}) OR start_planet IN ({$attackerPlanet},{$defenderPlanet}) OR target_planet IN ({$attackerPlanet},{$defenderPlanet})");
    $remainingFixtureQueues = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id IN ({$attackerId},{$defenderId}) AND type IN ('" . QTYP_FLEET . "','" . QTYP_RECALC_POINTS . "')");

    $cases[] = e2e_finalize_case(array(
        'case' => 'actual_update_queue_soak_drains_operational_and_maintenance_tasks',
        'checks' => array(
            e2e_case((int)$GlobalUni['speed'] === 128 && (int)$GlobalUni['fspeed'] === 1024 && (int)$GlobalUni['freeze'] === 0, 'soak run uses accelerated non-frozen universe settings', array('speed' => $GlobalUni['speed'], 'fspeed' => $GlobalUni['fspeed'], 'freeze' => $GlobalUni['freeze'])),
            e2e_case($cleanDebrisTask > 0 && $cleanPlanetsTask > 0 && $cleanPlayersTask > 0 && $updateStatsTask > 0 && $recalcAllyTask > 0, 'maintenance queue tasks are created for the soak run', array('clean_debris' => $cleanDebrisTask, 'clean_planets' => $cleanPlanetsTask, 'clean_players' => $cleanPlayersTask, 'update_stats' => $updateStatsTask, 'recalc_ally' => $recalcAllyTask)),
            e2e_case($maintenanceDrain['remaining_due'] === 0 && count($maintenanceDrain['runs']) > 0, 'UpdateQueue drains all due maintenance tasks in the soak window', $maintenanceDrain),
            e2e_case($emptyDebrisAfter === null, 'maintenance soak removes empty debris fields'),
            e2e_case($richDebrisAfter !== null && (int)$richDebrisAfter['metal'] === 75 && (int)$richDebrisAfter['crystal'] === 125, 'maintenance soak preserves non-empty debris fields', $richDebrisAfter ?? array()),
            e2e_case($expiredPlanetAfter === null, 'maintenance soak destroys removed planets whose deletion time is due'),
            e2e_case($futurePlanetAfter !== null && (int)$futurePlanetAfter['remove'] > $now, 'maintenance soak preserves removed planets scheduled for future deletion', $futurePlanetAfter ?? array()),
            e2e_case($disabledUserAfter === null && $disabledPlanetAfter === null, 'maintenance soak removes disabled non-admin users and their home planets'),
            e2e_case($adminUserAfter !== null && (int)$adminUserAfter['admin'] === 1, 'maintenance soak preserves disabled admin users', $adminUserAfter ?? array()),
            e2e_case($statsUserAfter !== null && (int)$statsUserAfter['oldscore1'] === 777 && (int)$statsUserAfter['oldscore2'] === 88 && (int)$statsUserAfter['oldscore3'] === 9 && (int)$statsUserAfter['scoredate'] === $expectedScoreDate, 'maintenance soak snapshots old score fields', array('user' => $statsUserAfter ?? array(), 'expected_scoredate' => $expectedScoreDate)),
            e2e_case($futureCleanDebris !== null && (int)$futureCleanDebris['task_id'] !== $cleanDebrisTask && (int)$futureCleanDebris['end'] > $now, 'CleanDebris reschedules a future cleanup task', $futureCleanDebris ?? array()),
            e2e_case($futureCleanPlanets !== null && (int)$futureCleanPlanets['task_id'] !== $cleanPlanetsTask && (int)$futureCleanPlanets['end'] > $now, 'CleanPlanets reschedules a future cleanup task', $futureCleanPlanets ?? array()),
            e2e_case($futureCleanPlayers !== null && (int)$futureCleanPlayers['task_id'] !== $cleanPlayersTask && (int)$futureCleanPlayers['end'] > $now, 'CleanPlayers reschedules a future cleanup task', $futureCleanPlayers ?? array()),
            e2e_case($futureUpdateStats !== null && (int)$futureUpdateStats['task_id'] !== $updateStatsTask && (int)$futureUpdateStats['end'] > $now, 'UpdateStats reschedules a future score snapshot task', $futureUpdateStats ?? array()),
            e2e_case($recalcAllyRemaining === 0, 'RecalcAllyPoints due queue is removed after the soak run', array('remaining' => $recalcAllyRemaining)),
            e2e_case($fleetId > 0 && $fleetQueue !== null && $fleetQueue !== false && $fleetRecalcTask > 0, 'fleet and recalc tasks are created for the operational soak run', array('fleet_id' => $fleetId, 'fleet_queue' => $fleetQueue ?: array(), 'recalc_task' => $fleetRecalcTask)),
            e2e_case($fleetDrain['remaining_due'] === 0 && count($fleetDrain['runs']) >= 2, 'UpdateQueue drains outbound and return fleet tasks across repeated passes', $fleetDrain),
            e2e_case($postFleetRecalcDrain['remaining_due'] === 0 && count($postFleetRecalcDrain['runs']) === 1, 'UpdateQueue drains post-fleet score recalculation without active fleets', $postFleetRecalcDrain),
            e2e_case($fleetTargetBefore !== null && $fleetTargetAfter !== null && (int)$fleetTargetAfter['metal'] >= (int)$fleetTargetBefore['metal'] + 222 && (int)$fleetTargetAfter['crystal'] >= (int)$fleetTargetBefore['crystal'] + 111 && (int)$fleetTargetAfter['deuterium'] >= (int)$fleetTargetBefore['deuterium'] + 33, 'operational soak delivers transport cargo to the target planet', array('before' => $fleetTargetBefore, 'after' => $fleetTargetAfter)),
            e2e_case($fleetOriginBefore !== null && $fleetOriginAfter !== null && (int)$fleetOriginAfter['small_cargo'] === (int)$fleetOriginBefore['small_cargo'], 'operational soak returns the transport ship to origin', array('before' => $fleetOriginBefore, 'after' => $fleetOriginAfter)),
            e2e_case($fleetUserAfter !== null && (int)$fleetUserAfter['score1'] >= 0 && (int)$fleetUserAfter['score2'] >= 0 && (int)$fleetUserAfter['score3'] >= 0, 'operational soak leaves recalculated user scores non-negative', $fleetUserAfter ?? array()),
            e2e_case($remainingFixtureFleets === 0 && $remainingFixtureQueues === 0, 'operational soak leaves no fixture fleet or recalc queues behind', array('fleets' => $remainingFixtureFleets, 'queues' => $remainingFixtureQueues)),
            e2e_case($fleetOriginAfter !== null && $fleetTargetAfter !== null && min((int)$fleetOriginAfter['metal'], (int)$fleetOriginAfter['crystal'], (int)$fleetOriginAfter['deuterium'], (int)$fleetTargetAfter['metal'], (int)$fleetTargetAfter['crystal'], (int)$fleetTargetAfter['deuterium']) >= 0, 'operational soak leaves fixture resources non-negative', array('origin' => $fleetOriginAfter, 'target' => $fleetTargetAfter)),
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'long_scheduler_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    e2e_clear_queues(array(QTYP_CLEAN_DEBRIS, QTYP_CLEAN_PLANETS, QTYP_CLEAN_PLAYERS, QTYP_UPDATE_STATS, QTYP_RECALC_ALLY_POINTS));
    foreach ($createdDebris as $planetId) {
        if ($planetId > 0 && e2e_planet_row($planetId) !== null) {
            DestroyPlanet($planetId);
        }
    }
    foreach ($createdPlanets as $planetId) {
        if ($planetId > 0 && e2e_planet_row($planetId) !== null) {
            DestroyPlanet($planetId);
        }
    }
    foreach ($createdUsers as $userId) {
        e2e_remove_user_if_exists((int)$userId);
    }
    if (isset($uniSnapshot['speed'], $uniSnapshot['fspeed'], $uniSnapshot['freeze'])) {
        dbquery("UPDATE {$db_prefix}uni SET speed=" . (float)$uniSnapshot['speed'] . ", fspeed=" . (float)$uniSnapshot['fspeed'] . ", freeze=" . (int)$uniSnapshot['freeze']);
        $GlobalUni = LoadUniverse();
    }
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
