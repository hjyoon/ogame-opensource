<?php

ob_start();
error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_soak_state_invariants_e2e.php';
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
loca_add('battlereport', 'en');

function e2e_sql_exec(string $sql): mixed
{
    $res = dbquery($sql);
    if ($res === false) {
        throw new RuntimeException('SQL failed: ' . $sql);
    }
    return $res;
}

function e2e_one_row(string $sql): ?array
{
    $res = e2e_sql_exec($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function e2e_count(string $sql): int
{
    $row = e2e_one_row($sql);
    return $row === null ? 0 : (int)$row['cnt'];
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

function e2e_zero_map(array $map): array
{
    $out = array();
    foreach ($map as $gid) {
        $out[$gid] = 0;
    }
    return $out;
}

function e2e_with_units(array $base, array $units): array
{
    foreach ($units as $gid => $amount) {
        $base[$gid] = $amount;
    }
    return $base;
}

function e2e_queue_type_sql(): string
{
    return "'" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "','" . QTYP_RECALC_POINTS . "','" . QTYP_UNBAN . "','" . QTYP_ALLOW_ATTACKS . "','" . QTYP_DEBUG . "'";
}

function e2e_cleanup_extra_planets(array $userIds, array $basePlanetIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', $userIds));
    $baseList = implode(',', array_map('intval', $basePlanetIds));
    e2e_sql_exec(
        "DELETE FROM {$db_prefix}planets WHERE owner_id IN ({$userList}) " .
        "AND type IN (" . PTYP_MOON . "," . PTYP_DF . "," . PTYP_DEST_MOON . ") " .
        "AND planet_id NOT IN ({$baseList})"
    );
}

function e2e_cleanup_fleets(array $userIds, array $planetIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', $userIds));
    $planetList = implode(',', array_map('intval', $planetIds));
    $fleetIds = array();
    $res = e2e_sql_exec("SELECT fleet_id FROM {$db_prefix}fleet WHERE owner_id IN ({$userList}) OR start_planet IN ({$planetList}) OR target_planet IN ({$planetList})");
    while ($row = dbarray($res)) {
        $fleetIds[] = (int)$row['fleet_id'];
    }
    if (!empty($fleetIds)) {
        $fleetList = implode(',', $fleetIds);
        e2e_sql_exec("DELETE FROM {$db_prefix}queue WHERE type='" . QTYP_FLEET . "' AND (owner_id IN ({$userList}) OR sub_id IN ({$fleetList}))");
        e2e_sql_exec("DELETE FROM {$db_prefix}fleet WHERE fleet_id IN ({$fleetList})");
    }
    e2e_sql_exec("DELETE FROM {$db_prefix}queue WHERE type='" . QTYP_FLEET . "' AND owner_id IN ({$userList})");
}

function e2e_reset_user_and_planet(int $userId, int $planetId, array $allUserIds, array $allPlanetIds): void
{
    global $db_prefix, $resmap, $fleetmap, $defmap, $rakmap, $buildmap;

    e2e_cleanup_fleets($allUserIds, $allPlanetIds);
    e2e_cleanup_extra_planets($allUserIds, $allPlanetIds);
    e2e_sql_exec("DELETE FROM {$db_prefix}queue WHERE owner_id={$userId} AND type IN (" . e2e_queue_type_sql() . ")");
    e2e_sql_exec("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$userId} OR planet_id={$planetId}");

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=10";
    }
    e2e_sql_exec(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", " .
        "admin=0, validated=1, validatemd='', deact_ip=1, vacation=0, vacation_until=0, " .
        "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, " .
        "lang='en', skin='/evolution/', useskin=1, com_until=0, adm_until=0, eng_until=0, geo_until=0, tec_until=0 " .
        "WHERE player_id={$userId}"
    );

    SetPlanetFleetDefense($planetId, e2e_zero_map(array_merge(array_diff($defmap, $rakmap), $fleetmap)));
    SetPlanetBuildings($planetId, e2e_zero_map($buildmap));

    $now = time();
    e2e_sql_exec(
        "UPDATE {$db_prefix}planets SET " .
        "name='E2E Soak {$userId}', " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "`" . GID_B_SOLAR . "`=10, `" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=10, `" . GID_B_RES_LAB . "`=10, `" . GID_B_NANITES . "`=0, " .
        "`" . GID_F_SC . "`=20, `" . GID_F_LC . "`=10, `" . GID_F_LF . "`=20, `" . GID_F_PROBE . "`=10, `" . GID_F_RECYCLER . "`=10, " .
        "`" . GID_D_ABM . "`=0, `" . GID_D_IPM . "`=0, " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=0, maxfields=220, " .
        "type=" . PTYP_PLANET . ", owner_id={$userId}, remove=0, lastpeek={$now}, lastakt={$now} " .
        "WHERE planet_id={$planetId}"
    );
    e2e_sql_exec("UPDATE {$db_prefix}users SET hplanetid={$planetId} WHERE player_id={$userId}");
    SelectPlanet($userId, $planetId);
    InvalidateUserCache();
}

function e2e_reset_fixtures(int $attackerId, int $attackerPlanet, int $defenderId, int $defenderPlanet): void
{
    $users = array($attackerId, $defenderId);
    $planets = array($attackerPlanet, $defenderPlanet);
    e2e_reset_user_and_planet($attackerId, $attackerPlanet, $users, $planets);
    e2e_reset_user_and_planet($defenderId, $defenderPlanet, $users, $planets);
    e2e_cleanup_fleets($users, $planets);
    e2e_cleanup_extra_planets($users, $planets);
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
    e2e_sql_exec("UPDATE {$db_prefix}queue SET start={$start}, end={$end}, freeze=0, frozen=0 WHERE task_id={$taskId}");

    if ($queue['type'] === QTYP_BUILD || $queue['type'] === QTYP_DEMOLISH) {
        e2e_sql_exec("UPDATE {$db_prefix}buildqueue SET start={$start}, end={$end} WHERE id=" . (int)$queue['sub_id']);
    }
}

function e2e_force_fixture_fleet_queues_due(array $userIds, int $now): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', $userIds));
    $res = e2e_sql_exec("SELECT task_id FROM {$db_prefix}queue WHERE owner_id IN ({$userList}) AND type='" . QTYP_FLEET . "'");
    while ($row = dbarray($res)) {
        e2e_force_queue_due((int)$row['task_id'], $now);
    }
}

function e2e_latest_queue(int $ownerId, string $type): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT * FROM {$db_prefix}queue WHERE owner_id={$ownerId} AND type='" . $type . "' ORDER BY task_id DESC LIMIT 1");
}

function e2e_due_queue_count(int $until, array $userIds): int
{
    global $db_prefix;
    $userList = implode(',', array_map('intval', $userIds));
    return e2e_count(
        "SELECT COUNT(*) AS cnt FROM {$db_prefix}queue " .
        "WHERE owner_id IN ({$userList}) AND type IN (" . e2e_queue_type_sql() . ") AND end <= {$until} AND freeze=0"
    );
}

function e2e_fixture_queue_count(array $userIds): int
{
    global $db_prefix;
    $userList = implode(',', array_map('intval', $userIds));
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id IN ({$userList}) AND type IN (" . e2e_queue_type_sql() . ")");
}

function e2e_fixture_fleet_count(array $userIds, array $planetIds): int
{
    global $db_prefix;
    $userList = implode(',', array_map('intval', $userIds));
    $planetList = implode(',', array_map('intval', $planetIds));
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id IN ({$userList}) OR start_planet IN ({$planetList}) OR target_planet IN ({$planetList})");
}

function e2e_drain_update_queue(int $until, array $userIds, int $limit = 50): array
{
    $loops = 0;
    $counts = array();
    while ($loops < $limit) {
        $due = e2e_due_queue_count($until, $userIds);
        $counts[] = $due;
        if ($due === 0) {
            break;
        }
        UpdateQueue($until);
        $loops++;
    }

    return array(
        'loops' => $loops,
        'due_counts' => $counts,
        'remaining_due' => e2e_due_queue_count($until, $userIds),
    );
}

function e2e_empty_fleet(): array
{
    global $fleetmap;
    return e2e_zero_map($fleetmap);
}

function e2e_empty_resources(): array
{
    global $transportableResources;
    return e2e_zero_map($transportableResources);
}

function e2e_dispatch_transport(int $fromPlanet, int $toPlanet, array $cargoSpec, int $shipCount, int $flightTime, int $when): array
{
    $fleet = e2e_empty_fleet();
    $fleet[GID_F_SC] = $shipCount;
    $cargo = e2e_empty_resources();
    foreach ($cargoSpec as $rc => $amount) {
        $cargo[$rc] = $amount;
    }

    AdjustShips($fleet, $fromPlanet, '-');
    AdjustResources($cargo, $fromPlanet, '-');
    $fleetId = DispatchFleet($fleet, LoadPlanetById($fromPlanet), LoadPlanetById($toPlanet), FTYP_TRANSPORT, $flightTime, $cargo, 0, $when);
    $queue = $fleetId > 0 ? GetFleetQueue($fleetId) : null;

    return array($fleetId, $queue);
}

function e2e_dispatch_attack(int $fromPlanet, int $toPlanet, array $ships, int $flightTime, int $when): array
{
    $fleet = e2e_with_units(e2e_empty_fleet(), $ships);
    $resources = e2e_empty_resources();
    AdjustShips($fleet, $fromPlanet, '-');
    $fleetId = DispatchFleet($fleet, LoadPlanetById($fromPlanet), LoadPlanetById($toPlanet), FTYP_ATTACK, $flightTime, $resources, 0, $when);
    $queue = $fleetId > 0 ? GetFleetQueue($fleetId) : null;

    return array($fleetId, $queue);
}

function e2e_planet_snapshot(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, fields, " .
        "`" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deuterium, " .
        "`" . GID_B_METAL_MINE . "` AS metal_mine, `" . GID_F_SC . "` AS small_cargo, `" . GID_F_LF . "` AS light_fighter, `" . GID_F_LC . "` AS large_cargo, " .
        "`" . GID_F_CRUISER . "` AS cruiser, `" . GID_F_BATTLESHIP . "` AS battleship, `" . GID_F_RECYCLER . "` AS recycler, " .
        "`" . GID_D_RL . "` AS rocket_launcher, `" . GID_D_LL . "` AS light_laser, `" . GID_D_GAUSS . "` AS gauss, `" . GID_D_PLASMA . "` AS plasma " .
        "FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1"
    );
}

function e2e_user_state_snapshot(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT player_id, banned, banned_until, disable, disable_until, vacation, vacation_until, score1, score2, score3 " .
        "FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1"
    );
}

function e2e_set_user_tech(int $userId, array $tech): void
{
    global $db_prefix, $resmap;
    $parts = array();
    foreach ($resmap as $gid) {
        $parts[] = "`{$gid}`=" . intval($tech[$gid] ?? 0);
    }
    e2e_sql_exec("UPDATE {$db_prefix}users SET " . implode(',', $parts) . " WHERE player_id={$userId}");
    InvalidateUserCache();
}

function e2e_prepare_battle_planets(int $attackerPlanet, int $defenderPlanet, array $attackerShips, array $defenderShips, array $defence, array $resources): void
{
    global $fleetmap, $defmap, $rakmap, $buildmap, $db_prefix;

    $emptyFleet = e2e_zero_map($fleetmap);
    $emptyDef = e2e_zero_map(array_diff($defmap, $rakmap));
    $emptyBuildings = e2e_zero_map($buildmap);

    SetPlanetFleetDefense($attackerPlanet, e2e_with_units($emptyDef + $emptyFleet, $attackerShips));
    SetPlanetFleetDefense($defenderPlanet, e2e_with_units(e2e_with_units($emptyDef + $emptyFleet, $defenderShips), $defence));
    SetPlanetBuildings($attackerPlanet, $emptyBuildings);
    SetPlanetBuildings($defenderPlanet, $emptyBuildings);

    $now = time();
    e2e_sql_exec(
        "UPDATE {$db_prefix}planets SET " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000, " .
        "`" . GID_B_SOLAR . "`=10, `" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=10, `" . GID_B_RES_LAB . "`=10, " .
        "`" . GID_D_ABM . "`=0, `" . GID_D_IPM . "`=0, prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, lastpeek={$now} " .
        "WHERE planet_id={$attackerPlanet}"
    );
    e2e_sql_exec(
        "UPDATE {$db_prefix}planets SET " .
        "`" . GID_RC_METAL . "`=" . floatval($resources[GID_RC_METAL] ?? 0) . ", " .
        "`" . GID_RC_CRYSTAL . "`=" . floatval($resources[GID_RC_CRYSTAL] ?? 0) . ", " .
        "`" . GID_RC_DEUTERIUM . "`=" . floatval($resources[GID_RC_DEUTERIUM] ?? 0) . ", " .
        "`" . GID_B_SOLAR . "`=10, `" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=10, `" . GID_B_RES_LAB . "`=10, " .
        "`" . GID_D_ABM . "`=0, `" . GID_D_IPM . "`=0, prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, lastpeek={$now} " .
        "WHERE planet_id={$defenderPlanet}"
    );
}

function e2e_max_msg_id(): int
{
    global $db_prefix;
    $row = e2e_one_row("SELECT COALESCE(MAX(msg_id), 0) AS id FROM {$db_prefix}messages");
    return $row === null ? 0 : (int)$row['id'];
}

function e2e_max_battle_id(): int
{
    global $db_prefix;
    $row = e2e_one_row("SELECT COALESCE(MAX(battle_id), 0) AS id FROM {$db_prefix}battledata");
    return $row === null ? 0 : (int)$row['id'];
}

function e2e_message_count_after(int $ownerId, int $after, int $pm): int
{
    global $db_prefix;
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}messages WHERE owner_id={$ownerId} AND msg_id>{$after} AND pm={$pm}");
}

function e2e_battle_log_after(int $afterBattle): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT battle_id, title, report FROM {$db_prefix}battledata WHERE battle_id>{$afterBattle} ORDER BY battle_id ASC LIMIT 1");
}

function e2e_negative_fixture_values(array $userIds, array $planetIds): array
{
    global $db_prefix, $transportableResources, $fleetmap, $defmap;

    $userList = implode(',', array_map('intval', $userIds));
    $planetList = implode(',', array_map('intval', $planetIds));
    $cols = array_values(array_unique(array_merge($transportableResources, $fleetmap, $defmap)));
    $select = array('planet_id', 'type');
    foreach ($cols as $gid) {
        $select[] = "`{$gid}`";
    }

    $negatives = array();
    $res = e2e_sql_exec(
        "SELECT " . implode(',', $select) . " FROM {$db_prefix}planets " .
        "WHERE planet_id IN ({$planetList}) OR (owner_id IN ({$userList}) AND type IN (" . PTYP_MOON . "," . PTYP_DF . "," . PTYP_DEST_MOON . "))"
    );
    while ($row = dbarray($res)) {
        foreach ($cols as $gid) {
            if (isset($row[$gid]) && (float)$row[$gid] < 0) {
                $negatives[] = array('planet_id' => (int)$row['planet_id'], 'type' => (int)$row['type'], 'gid' => (int)$gid, 'value' => (float)$row[$gid]);
            }
        }
    }

    return $negatives;
}

function e2e_run_battle_invariant_case(
    string $name,
    int $seed,
    int $attackerId,
    int $attackerPlanet,
    int $defenderId,
    int $defenderPlanet,
    array $attackerShips,
    array $defenderShips,
    array $defence,
    array $resources,
    array $attackerTech,
    array $defenderTech
): array {
    $users = array($attackerId, $defenderId);
    $planets = array($attackerPlanet, $defenderPlanet);
    e2e_reset_fixtures($attackerId, $attackerPlanet, $defenderId, $defenderPlanet);
    e2e_set_user_tech($attackerId, $attackerTech);
    e2e_set_user_tech($defenderId, $defenderTech);
    e2e_prepare_battle_planets($attackerPlanet, $defenderPlanet, $attackerShips, $defenderShips, $defence, $resources);

    $now = time();
    $afterMsg = e2e_max_msg_id();
    $afterBattle = e2e_max_battle_id();
    mt_srand($seed);
    srand($seed);

    [$fleetId, $queue] = e2e_dispatch_attack($attackerPlanet, $defenderPlanet, $attackerShips, 5, $now - 20);
    $result = ($fleetId > 0 && $queue !== null && $queue !== false)
        ? StartBattle($fleetId, $defenderPlanet, (int)$queue['end'])
        : -1;

    if ($fleetId > 0) {
        DeleteFleet($fleetId);
    }
    if ($queue !== null && $queue !== false) {
        RemoveQueue((int)$queue['task_id']);
    }

    e2e_force_fixture_fleet_queues_due($users, $now);
    $drain = e2e_drain_update_queue($now, $users);

    $battleLog = e2e_battle_log_after($afterBattle);
    $attackerReports = e2e_message_count_after($attackerId, $afterMsg, MTYP_BATTLE_REPORT_TEXT);
    $defenderReports = e2e_message_count_after($defenderId, $afterMsg, MTYP_BATTLE_REPORT_TEXT);
    $negatives = e2e_negative_fixture_values($users, $planets);
    $remainingFleets = e2e_fixture_fleet_count($users, $planets);
    $remainingQueues = e2e_fixture_queue_count($users);
    $validResult = in_array($result, array(BATTLE_RESULT_AWON, BATTLE_RESULT_DWON, BATTLE_RESULT_DRAW), true);

    return e2e_finalize_case(array(
        'case' => $name,
        'battle_result' => array(BATTLE_RESULT_AWON => 'attacker_won', BATTLE_RESULT_DWON => 'defender_won', BATTLE_RESULT_DRAW => 'draw')[$result] ?? 'unknown',
        'drain' => $drain,
        'checks' => array(
            e2e_case($fleetId > 0 && $queue !== null && $queue !== false, 'attack fleet and queue are created', array('fleet_id' => $fleetId, 'queue' => $queue ?: array())),
            e2e_case($validResult, 'battle result is one of the supported outcomes', array('raw_result' => $result)),
            e2e_case($attackerReports >= 1, 'attacker battle report message is generated', array('count' => $attackerReports)),
            e2e_case($defenderReports >= 1, 'defender battle report message is generated', array('count' => $defenderReports)),
            e2e_case($battleLog !== null && strlen($battleLog['report'] ?? '') > 100, 'battledata report is persisted with printable content', $battleLog === null ? array() : array('battle_id' => (int)$battleLog['battle_id'], 'report_len' => strlen($battleLog['report'] ?? ''))),
            e2e_case(empty($negatives), 'battle writeback leaves resources and unit counts non-negative', array('negatives' => $negatives)),
            e2e_case($remainingFleets === 0, 'battle return processing leaves no fixture fleet rows', array('remaining_fleets' => $remainingFleets)),
            e2e_case($remainingQueues === 0, 'battle return processing leaves no fixture queue rows', array('remaining_queues' => $remainingQueues)),
        ),
    ));
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

    $users = array($attackerId, $defenderId);
    $planets = array($attackerPlanet, $defenderPlanet);
    $now = time();

    e2e_reset_fixtures($attackerId, $attackerPlanet, $defenderId, $defenderPlanet);
    $originBefore = e2e_planet_snapshot($attackerPlanet);
    $targetBefore = e2e_planet_snapshot($defenderPlanet);

    $recalcTasks = array();
    for ($i = 0; $i < QUEUE_BATCH + 8; $i++) {
        $owner = $i % 2 === 0 ? $attackerId : $defenderId;
        $recalcTasks[] = AddQueue($owner, QTYP_RECALC_POINTS, 0, 0, 0, $now - 20 - $i, 1, QUEUE_PRIO_RECALC_POINTS);
    }

    $buildText = BuildEnque(LoadUser($attackerId), $attackerPlanet, GID_B_METAL_MINE, 0, $now);
    $buildTask = e2e_latest_queue($attackerId, QTYP_BUILD);
    if ($buildTask !== null) {
        e2e_force_queue_due((int)$buildTask['task_id'], $now);
    }

    $shipyardOk = AddShipyard($attackerId, $attackerPlanet, GID_F_LF, 4, $now);
    $shipyardTask = e2e_latest_queue($attackerId, QTYP_SHIPYARD);
    if ($shipyardTask !== null) {
        e2e_force_queue_due((int)$shipyardTask['task_id'], $now);
    }

    $transportCargos = array(
        array(GID_RC_METAL => 101, GID_RC_CRYSTAL => 11, GID_RC_DEUTERIUM => 1),
        array(GID_RC_METAL => 202, GID_RC_CRYSTAL => 22, GID_RC_DEUTERIUM => 2),
        array(GID_RC_METAL => 303, GID_RC_CRYSTAL => 33, GID_RC_DEUTERIUM => 3),
        array(GID_RC_METAL => 404, GID_RC_CRYSTAL => 44, GID_RC_DEUTERIUM => 4),
    );
    $transportFleetIds = array();
    $transportQueues = array();
    foreach ($transportCargos as $cargoSpec) {
        [$fleetId, $queue] = e2e_dispatch_transport($attackerPlanet, $defenderPlanet, $cargoSpec, 1, 5, $now - 20);
        $transportFleetIds[] = $fleetId;
        $transportQueues[] = $queue;
        if ($queue !== null && $queue !== false) {
            e2e_force_queue_due((int)$queue['task_id'], $now);
        }
    }

    $preparedDue = e2e_due_queue_count($now, $users);
    $drain = e2e_drain_update_queue($now, $users);
    $originAfter = e2e_planet_snapshot($attackerPlanet);
    $targetAfter = e2e_planet_snapshot($defenderPlanet);
    $remainingQueues = e2e_fixture_queue_count($users);
    $remainingFleets = e2e_fixture_fleet_count($users, $planets);
    $totalCargo = array(
        GID_RC_METAL => array_sum(array_map(fn($cargo) => $cargo[GID_RC_METAL], $transportCargos)),
        GID_RC_CRYSTAL => array_sum(array_map(fn($cargo) => $cargo[GID_RC_CRYSTAL], $transportCargos)),
        GID_RC_DEUTERIUM => array_sum(array_map(fn($cargo) => $cargo[GID_RC_DEUTERIUM], $transportCargos)),
    );

    $cases[] = e2e_finalize_case(array(
        'case' => 'batch_soak_drains_more_than_queue_batch_with_returns',
        'drain' => $drain,
        'checks' => array(
            e2e_case(count(array_filter($recalcTasks, fn($id) => $id > 0)) === QUEUE_BATCH + 8, 'recalc tasks are created beyond the queue batch size', array('task_count' => count($recalcTasks), 'queue_batch' => QUEUE_BATCH)),
            e2e_case($buildText === '' && $buildTask !== null, 'building task is prepared for the batch soak', array('message' => $buildText, 'queue' => $buildTask ?? array())),
            e2e_case($shipyardOk && $shipyardTask !== null, 'shipyard task is prepared for the batch soak', array('queue' => $shipyardTask ?? array())),
            e2e_case(count(array_filter($transportFleetIds, fn($id) => $id > 0)) === count($transportCargos), 'transport fleets are prepared for the batch soak', array('fleet_ids' => $transportFleetIds)),
            e2e_case($preparedDue > QUEUE_BATCH, 'prepared due queues exceed one UpdateQueue batch', array('prepared_due' => $preparedDue, 'queue_batch' => QUEUE_BATCH)),
            e2e_case(($drain['loops'] ?? 0) > 1 && ($drain['remaining_due'] ?? 1) === 0, 'UpdateQueue drains all due fixture queues across multiple batches', $drain),
            e2e_case($originAfter !== null && (int)$originAfter['metal_mine'] === 1, 'building completion is applied once during the batch soak', $originAfter ?? array()),
            e2e_case($originBefore !== null && $originAfter !== null && (int)$originAfter['light_fighter'] === (int)$originBefore['light_fighter'] + 4, 'shipyard completion is applied once during the batch soak', array('before' => $originBefore, 'after' => $originAfter)),
            e2e_case($targetBefore !== null && $targetAfter !== null && (int)$targetAfter['metal'] === (int)$targetBefore['metal'] + $totalCargo[GID_RC_METAL], 'all transport metal is delivered exactly once', array('before' => $targetBefore, 'after' => $targetAfter, 'expected_delta' => $totalCargo[GID_RC_METAL])),
            e2e_case($targetBefore !== null && $targetAfter !== null && (int)$targetAfter['crystal'] === (int)$targetBefore['crystal'] + $totalCargo[GID_RC_CRYSTAL], 'all transport crystal is delivered exactly once', array('before' => $targetBefore, 'after' => $targetAfter, 'expected_delta' => $totalCargo[GID_RC_CRYSTAL])),
            e2e_case($targetBefore !== null && $targetAfter !== null && (int)$targetAfter['deuterium'] === (int)$targetBefore['deuterium'] + $totalCargo[GID_RC_DEUTERIUM], 'all transport deuterium is delivered exactly once', array('before' => $targetBefore, 'after' => $targetAfter, 'expected_delta' => $totalCargo[GID_RC_DEUTERIUM])),
            e2e_case($originBefore !== null && $originAfter !== null && (int)$originAfter['small_cargo'] === (int)$originBefore['small_cargo'], 'transport return queues restore all cargo ships once', array('before' => $originBefore, 'after' => $originAfter)),
            e2e_case($remainingQueues === 0, 'batch soak leaves no fixture queue rows', array('remaining_queues' => $remainingQueues)),
            e2e_case($remainingFleets === 0, 'batch soak leaves no fixture fleet rows', array('remaining_fleets' => $remainingFleets)),
        ),
    ));

    e2e_reset_fixtures($attackerId, $attackerPlanet, $defenderId, $defenderPlanet);
    $stateOriginBefore = e2e_planet_snapshot($attackerPlanet);
    $stateTargetBefore = e2e_planet_snapshot($defenderPlanet);
    $stateBuildText = BuildEnque(LoadUser($attackerId), $attackerPlanet, GID_B_METAL_MINE, 0, $now + 10);
    $stateBuildTask = e2e_latest_queue($attackerId, QTYP_BUILD);
    if ($stateBuildTask !== null) {
        e2e_force_queue_due((int)$stateBuildTask['task_id'], $now);
    }

    [$stateFleetId, $stateFleetQueue] = e2e_dispatch_transport(
        $attackerPlanet,
        $defenderPlanet,
        array(GID_RC_METAL => 777, GID_RC_CRYSTAL => 66, GID_RC_DEUTERIUM => 5),
        1,
        5,
        $now - 20
    );
    if ($stateFleetQueue !== null && $stateFleetQueue !== false) {
        e2e_force_queue_due((int)$stateFleetQueue['task_id'], $now);
    }

    e2e_sql_exec(
        "UPDATE {$db_prefix}users SET banned=1, banned_until=" . ($now + 3600) . ", " .
        "disable=1, disable_until=" . ($now + 86400) . ", vacation=1, vacation_until=" . ($now + 7200) . " " .
        "WHERE player_id={$attackerId}"
    );
    InvalidateUserCache();

    $stateBeforeDrain = e2e_user_state_snapshot($attackerId);
    $statePreparedDue = e2e_due_queue_count($now, $users);
    $stateDrain = e2e_drain_update_queue($now, $users);
    $stateAfterDrain = e2e_user_state_snapshot($attackerId);
    $stateOriginAfter = e2e_planet_snapshot($attackerPlanet);
    $stateTargetAfter = e2e_planet_snapshot($defenderPlanet);
    $stateRemainingQueues = e2e_fixture_queue_count($users);
    $stateRemainingFleets = e2e_fixture_fleet_count($users, $planets);

    $cases[] = e2e_finalize_case(array(
        'case' => 'active_queues_complete_after_account_state_changes',
        'drain' => $stateDrain,
        'checks' => array(
            e2e_case($stateBuildText === '' && $stateBuildTask !== null, 'building task is active before account-state mutation', array('message' => $stateBuildText, 'queue' => $stateBuildTask ?? array())),
            e2e_case($stateFleetId > 0 && $stateFleetQueue !== null && $stateFleetQueue !== false, 'fleet task is active before account-state mutation', array('fleet_id' => $stateFleetId, 'queue' => $stateFleetQueue ?: array())),
            e2e_case($stateBeforeDrain !== null && (int)$stateBeforeDrain['banned'] === 1 && (int)$stateBeforeDrain['disable'] === 1 && (int)$stateBeforeDrain['vacation'] === 1, 'account state flags are set before queue drain', $stateBeforeDrain ?? array()),
            e2e_case($statePreparedDue >= 2 && ($stateDrain['remaining_due'] ?? 1) === 0, 'active build and fleet queues drain after account state changes', array('prepared_due' => $statePreparedDue, 'drain' => $stateDrain)),
            e2e_case($stateOriginAfter !== null && (int)$stateOriginAfter['metal_mine'] === 1, 'active building completes while account flags remain set', $stateOriginAfter ?? array()),
            e2e_case($stateTargetBefore !== null && $stateTargetAfter !== null && (int)$stateTargetAfter['metal'] === (int)$stateTargetBefore['metal'] + 777, 'active fleet delivers cargo while account flags remain set', array('before' => $stateTargetBefore, 'after' => $stateTargetAfter)),
            e2e_case($stateOriginBefore !== null && $stateOriginAfter !== null && (int)$stateOriginAfter['small_cargo'] === (int)$stateOriginBefore['small_cargo'], 'active fleet return restores the ship while account flags remain set', array('before' => $stateOriginBefore, 'after' => $stateOriginAfter)),
            e2e_case($stateAfterDrain !== null && (int)$stateAfterDrain['banned'] === 1 && (int)$stateAfterDrain['disable'] === 1 && (int)$stateAfterDrain['vacation'] === 1, 'queue completion does not clear account state flags', $stateAfterDrain ?? array()),
            e2e_case($stateRemainingQueues === 0, 'account-state queue drain leaves no fixture queue rows', array('remaining_queues' => $stateRemainingQueues)),
            e2e_case($stateRemainingFleets === 0, 'account-state queue drain leaves no fixture fleet rows', array('remaining_fleets' => $stateRemainingFleets)),
        ),
    ));

    $commonAttackerTech = array(GID_R_WEAPON => 5, GID_R_SHIELD => 5, GID_R_ARMOUR => 5, GID_R_COMBUST_DRIVE => 5, GID_R_IMPULSE_DRIVE => 5, GID_R_HYPER_DRIVE => 5);
    $commonDefenderTech = array(GID_R_WEAPON => 4, GID_R_SHIELD => 4, GID_R_ARMOUR => 4, GID_R_ESPIONAGE => 4);
    $battleCases = array(
        array(
            'name' => 'battle_invariant_plunder_empty_planet',
            'seed' => 611,
            'attacker_ships' => array(GID_F_BATTLESHIP => 1, GID_F_LC => 2),
            'defender_ships' => array(),
            'defence' => array(),
            'resources' => array(GID_RC_METAL => 900000, GID_RC_CRYSTAL => 600000, GID_RC_DEUTERIUM => 300000),
        ),
        array(
            'name' => 'battle_invariant_defense_holds',
            'seed' => 612,
            'attacker_ships' => array(GID_F_LF => 2),
            'defender_ships' => array(),
            'defence' => array(GID_D_PLASMA => 8, GID_D_RL => 40),
            'resources' => array(GID_RC_METAL => 100000, GID_RC_CRYSTAL => 100000, GID_RC_DEUTERIUM => 100000),
        ),
        array(
            'name' => 'battle_invariant_rapid_fire_mixed_units',
            'seed' => 613,
            'attacker_ships' => array(GID_F_CRUISER => 8, GID_F_BATTLECRUISER => 2),
            'defender_ships' => array(GID_F_LF => 80, GID_F_PROBE => 30),
            'defence' => array(GID_D_RL => 80, GID_D_LL => 25),
            'resources' => array(GID_RC_METAL => 250000, GID_RC_CRYSTAL => 150000, GID_RC_DEUTERIUM => 50000),
        ),
        array(
            'name' => 'battle_invariant_six_round_draw_pressure',
            'seed' => 614,
            'attacker_ships' => array(GID_F_LC => 1),
            'defender_ships' => array(GID_F_LC => 1),
            'defence' => array(),
            'resources' => array(GID_RC_METAL => 0, GID_RC_CRYSTAL => 0, GID_RC_DEUTERIUM => 0),
        ),
    );

    foreach ($battleCases as $battleCase) {
        $cases[] = e2e_run_battle_invariant_case(
            $battleCase['name'],
            $battleCase['seed'],
            $attackerId,
            $attackerPlanet,
            $defenderId,
            $defenderPlanet,
            $battleCase['attacker_ships'],
            $battleCase['defender_ships'],
            $battleCase['defence'],
            $battleCase['resources'],
            $commonAttackerTech,
            $commonDefenderTech
        );
    }
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'soak_state_invariants_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($attackerId > 0 && $attackerPlanet > 0 && $defenderId > 0 && $defenderPlanet > 0) {
        e2e_reset_fixtures($attackerId, $attackerPlanet, $defenderId, $defenderPlanet);
    }
}

$noise = trim(ob_get_clean());

echo json_encode(array(
    'case_group' => 'http_soak_state_invariants',
    'cases' => $cases,
    'captured_output' => $noise,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
