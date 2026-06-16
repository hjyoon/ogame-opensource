<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_global_maintenance_queue_e2e.php';
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
loca_add('ally', 'en');
loca_add('fleet', 'en');
loca_add('technames', 'en');

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

function e2e_user_row(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT player_id, name, oname, email, pemail, admin, ally_id, allyrank, joindate, " .
        "name_changed, name_until, banned, banned_until, noattack, noattack_until, disable, disable_until, " .
        "validated, deact_ip, vacation, vacation_until, score1, score2, score3, place1, place2, place3, " .
        "oldscore1, oldscore2, oldscore3, oldplace1, oldplace2, oldplace3, scoredate, hplanetid " .
        "FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1"
    );
}

function e2e_planet_row(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, type, g, s, p, remove, " .
        "`" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deuterium, " .
        "`" . GID_B_METAL_MINE . "` AS metal_mine, `" . GID_F_RECYCLER . "` AS recycler " .
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

function e2e_clear_queues(array $types, int $ownerId = -1): void
{
    global $db_prefix;

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

function e2e_update_queue_twice(int $now): void
{
    UpdateQueue($now);
    UpdateQueue($now);
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

function e2e_restore_fixture_user(?array $user, int $planetId): void
{
    global $db_prefix, $resmap;

    if ($user === null) {
        return;
    }

    $userId = (int)$user['player_id'];
    e2e_cleanup_fleets(array($userId), array($planetId));
    e2e_clear_queues(array(QTYP_ALLOW_NAME, QTYP_CHANGE_EMAIL, QTYP_UNBAN, QTYP_ALLOW_ATTACKS, QTYP_RECALC_POINTS), $userId);
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$userId} OR planet_id={$planetId}");

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=10";
    }
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", " .
        "email='" . e2e_sql_escape($user['email']) . "', pemail='" . e2e_sql_escape($user['pemail']) . "', " .
        "admin=0, ally_id=0, allyrank=0, joindate=0, name_changed=0, name_until=0, " .
        "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, " .
        "validated=1, deact_ip=1, vacation=0, vacation_until=0, lang='en', skin='/evolution/', useskin=1 " .
        "WHERE player_id={$userId}"
    );

    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "`" . GID_B_METAL_MINE . "`=0, `" . GID_B_CRYS_MINE . "`=0, `" . GID_B_DEUT_SYNTH . "`=0, `" . GID_B_SOLAR . "`=10, " .
        "`" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=10, `" . GID_B_RES_LAB . "`=10, " .
        "`" . GID_F_SC . "`=10, `" . GID_F_LC . "`=5, `" . GID_F_LF . "`=5, `" . GID_F_PROBE . "`=5, `" . GID_F_RECYCLER . "`=5, " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=0, maxfields=200, type=" . PTYP_PLANET . ", remove=0, owner_id={$userId} " .
        "WHERE planet_id={$planetId}"
    );
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$planetId} WHERE player_id={$userId}");
    SelectPlanet($userId, $planetId);
    InvalidateUserCache();
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
    $name = 'e2emq_' . $safeSuffix;
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

    throw new RuntimeException('No empty planet slot found for maintenance queue test.');
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

    throw new RuntimeException('No empty debris slot found for maintenance queue test.');
}

function e2e_create_debris(array $coords, int $ownerId, int $metal, int $crystal): int
{
    global $db_prefix;

    $id = CreateDebris($coords[0], $coords[1], $coords[2], $ownerId);
    dbquery("UPDATE {$db_prefix}planets SET `" . GID_RC_METAL . "`={$metal}, `" . GID_RC_CRYSTAL . "`={$crystal}, `" . GID_RC_DEUTERIUM . "`=0 WHERE planet_id={$id}");
    return $id;
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderPlanet = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 0);
$cases = array();
$createdUsers = array();
$createdPlanets = array();
$createdDebris = array();
$createdAlliances = array();
$reservedPlanetSlots = array();
$reservedDebrisSlots = array();
$originalAttacker = null;
$originalDefender = null;

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $originalAttacker = e2e_user_row($attackerId);
    $originalDefender = e2e_user_row($defenderId);
    $now = time();

    e2e_restore_fixture_user($originalAttacker, $attackerPlanet);
    e2e_restore_fixture_user($originalDefender, $defenderPlanet);

    e2e_clear_queues(array(QTYP_ALLOW_NAME, QTYP_CHANGE_EMAIL, QTYP_UNBAN, QTYP_ALLOW_ATTACKS), $attackerId);
    $newEmail = 'e2e-maint-new-' . $attackerId . '@example.local';
    $oldEmail = 'e2e-maint-old-' . $attackerId . '@example.local';
    dbquery(
        "UPDATE {$db_prefix}users SET name_changed=1, name_until=" . ($now - 1) . ", " .
        "banned=1, banned_until=" . ($now - 1) . ", noattack=1, noattack_until=" . ($now - 1) . ", " .
        "email='" . e2e_sql_escape($newEmail) . "', pemail='" . e2e_sql_escape($oldEmail) . "' WHERE player_id={$attackerId}"
    );
    $allowNameTask = e2e_add_due_queue($attackerId, QTYP_ALLOW_NAME, QUEUE_PRIO_LOWEST, $now);
    $changeEmailTask = e2e_add_due_queue($attackerId, QTYP_CHANGE_EMAIL, QUEUE_PRIO_LOWEST, $now);
    $unbanTask = e2e_add_due_queue($attackerId, QTYP_UNBAN, QUEUE_PRIO_LOWEST, $now);
    $allowAttacksTask = e2e_add_due_queue($attackerId, QTYP_ALLOW_ATTACKS, QUEUE_PRIO_LOWEST, $now);
    e2e_update_queue_twice($now);
    $timerUser = e2e_user_row($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'user_state_timers_complete_once_and_clear_flags',
        'checks' => array(
            e2e_case($allowNameTask > 0 && $changeEmailTask > 0 && $unbanTask > 0 && $allowAttacksTask > 0, 'all user state timer queue tasks are created', array('allow_name' => $allowNameTask, 'change_email' => $changeEmailTask, 'unban' => $unbanTask, 'allow_attacks' => $allowAttacksTask)),
            e2e_case($timerUser !== null && (int)$timerUser['name_changed'] === 0, 'AllowName clears the name-change cooldown flag', $timerUser ?? array()),
            e2e_case($timerUser !== null && (int)$timerUser['banned'] === 0 && (int)$timerUser['banned_until'] === 0, 'Unban clears ban state', $timerUser ?? array()),
            e2e_case($timerUser !== null && (int)$timerUser['noattack'] === 0 && (int)$timerUser['noattack_until'] === 0, 'AllowAttacks clears attack-ban state', $timerUser ?? array()),
            e2e_case($timerUser !== null && $timerUser['pemail'] === $newEmail, 'ChangeEmail copies temporary email to permanent email', $timerUser ?? array()),
            e2e_case(e2e_queue_count(QTYP_ALLOW_NAME, $attackerId) === 0 && e2e_queue_count(QTYP_CHANGE_EMAIL, $attackerId) === 0 && e2e_queue_count(QTYP_UNBAN, $attackerId) === 0 && e2e_queue_count(QTYP_ALLOW_ATTACKS, $attackerId) === 0, 'completed user state timer queues are removed'),
        ),
    ));

    e2e_restore_fixture_user($originalAttacker, $attackerPlanet);

    e2e_clear_queues(array(QTYP_RECALC_POINTS), $attackerId);
    global $resmap;
    $researchZero = array();
    foreach ($resmap as $gid) {
        $researchZero[] = "`{$gid}`=0";
    }
    dbquery("UPDATE {$db_prefix}users SET " . implode(',', $researchZero) . ", score1=0, score2=0, score3=0 WHERE player_id={$attackerId}");
    dbquery("UPDATE {$db_prefix}planets SET `" . GID_B_METAL_MINE . "`=3, `" . GID_F_SC . "`=0, `" . GID_F_RECYCLER . "`=0 WHERE planet_id={$attackerPlanet}");
    $recalcTask = e2e_add_due_queue($attackerId, QTYP_RECALC_POINTS, QUEUE_PRIO_RECALC_POINTS, $now);
    e2e_update_queue_twice($now);
    $recalcUser = e2e_user_row($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'recalc_points_queue_recomputes_user_scores',
        'checks' => array(
            e2e_case($recalcTask > 0, 'recalc-points queue task is created', array('task_id' => $recalcTask)),
            e2e_case($recalcUser !== null && (int)$recalcUser['score1'] > 0, 'RecalcPoints recomputes non-zero asset score from planet state', $recalcUser ?? array()),
            e2e_case($recalcUser !== null && (int)$recalcUser['score3'] === 0, 'RecalcPoints keeps research score at zero when research levels are zeroed', $recalcUser ?? array()),
            e2e_case(e2e_queue_count(QTYP_RECALC_POINTS, $attackerId) === 0, 'recalc-points queue task is removed after completion'),
        ),
    ));

    e2e_restore_fixture_user($originalAttacker, $attackerPlanet);

    e2e_clear_queues(array(QTYP_RECALC_ALLY_POINTS));
    $allyId = CreateAlly($attackerId, 'E2EMQ', 'E2E Maintenance Queue');
    $createdAlliances[] = $allyId;
    dbquery("UPDATE {$db_prefix}users SET score1=321, score2=4, score3=5 WHERE player_id={$attackerId}");
    dbquery("UPDATE {$db_prefix}ally SET score1=0, score2=0, score3=0, place1=0, place2=0, place3=0 WHERE ally_id={$allyId}");
    $allyTask = e2e_add_due_queue(USER_SPACE, QTYP_RECALC_ALLY_POINTS, QUEUE_PRIO_RECALC_ALLY_POINTS, $now);
    e2e_update_queue_twice($now);
    $allyAfterRecalc = e2e_one_row("SELECT ally_id, score1, score2, score3, place1, place2, place3 FROM {$db_prefix}ally WHERE ally_id={$allyId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'recalc_ally_points_queue_recomputes_alliance_scores',
        'checks' => array(
            e2e_case($allyId > 0 && $allyTask > 0, 'alliance and recalc-alliance queue task are created', array('ally_id' => $allyId, 'task_id' => $allyTask)),
            e2e_case($allyAfterRecalc !== null && (int)$allyAfterRecalc['score1'] === 321 && (int)$allyAfterRecalc['score2'] === 4 && (int)$allyAfterRecalc['score3'] === 5, 'RecalcAllyPoints aggregates member scores into alliance scores', $allyAfterRecalc ?? array()),
            e2e_case($allyAfterRecalc !== null && (int)$allyAfterRecalc['place1'] > 0 && (int)$allyAfterRecalc['place2'] > 0 && (int)$allyAfterRecalc['place3'] > 0, 'RecalcAllyPoints refreshes alliance rank places', $allyAfterRecalc ?? array()),
            e2e_case(e2e_queue_count(QTYP_RECALC_ALLY_POINTS) === 0, 'recalc-alliance queue task is removed after completion'),
        ),
    ));
    DismissAlly($allyId);
    $createdAlliances = array_values(array_filter($createdAlliances, fn($id) => $id !== $allyId));

    e2e_restore_fixture_user($originalAttacker, $attackerPlanet);

    e2e_clear_queues(array(QTYP_UPDATE_STATS));
    $statsAllyId = CreateAlly($attackerId, 'E2ESTAT', 'E2E Stats Queue');
    $createdAlliances[] = $statsAllyId;
    dbquery("UPDATE {$db_prefix}users SET score1=1111, score2=22, score3=33, place1=4, place2=5, place3=6, oldscore1=0, oldscore2=0, oldscore3=0, oldplace1=0, oldplace2=0, oldplace3=0, scoredate=0 WHERE player_id={$attackerId}");
    dbquery("UPDATE {$db_prefix}ally SET score1=4444, score2=55, score3=66, place1=7, place2=8, place3=9, oldscore1=0, oldscore2=0, oldscore3=0, oldplace1=0, oldplace2=0, oldplace3=0, scoredate=0 WHERE ally_id={$statsAllyId}");
    $updateStatsTask = e2e_add_due_queue(USER_SPACE, QTYP_UPDATE_STATS, QUEUE_PRIO_UPDATE_STATS, $now);
    $updateStatsQueue = LoadQueue($updateStatsTask);
    e2e_update_queue_twice($now);
    $userStatsAfter = e2e_user_row($attackerId);
    $allyStatsAfter = e2e_one_row("SELECT ally_id, oldscore1, oldscore2, oldscore3, oldplace1, oldplace2, oldplace3, scoredate FROM {$db_prefix}ally WHERE ally_id={$statsAllyId} LIMIT 1");
    $futureStatsQueue = e2e_one_row("SELECT task_id, end FROM {$db_prefix}queue WHERE type='" . QTYP_UPDATE_STATS . "' ORDER BY end DESC LIMIT 1");
    $expectedScoreDate = $updateStatsQueue === null || $updateStatsQueue === false ? 0 : (int)$updateStatsQueue['end'];
    $cases[] = e2e_finalize_case(array(
        'case' => 'update_stats_queue_snapshots_old_scores_and_reschedules',
        'checks' => array(
            e2e_case($updateStatsTask > 0, 'update-stats queue task is created', array('task_id' => $updateStatsTask)),
            e2e_case($userStatsAfter !== null && (int)$userStatsAfter['oldscore1'] === 1111 && (int)$userStatsAfter['oldscore2'] === 22 && (int)$userStatsAfter['oldscore3'] === 33 && (int)$userStatsAfter['oldplace1'] === 4 && (int)$userStatsAfter['scoredate'] === $expectedScoreDate, 'UpdateStats snapshots user score/place fields', array('user' => $userStatsAfter, 'expected_scoredate' => $expectedScoreDate)),
            e2e_case($allyStatsAfter !== null && (int)$allyStatsAfter['oldscore1'] === 4444 && (int)$allyStatsAfter['oldscore2'] === 55 && (int)$allyStatsAfter['oldscore3'] === 66 && (int)$allyStatsAfter['oldplace1'] === 7 && (int)$allyStatsAfter['scoredate'] === $expectedScoreDate, 'UpdateStats snapshots alliance score/place fields', array('ally' => $allyStatsAfter, 'expected_scoredate' => $expectedScoreDate)),
            e2e_case($futureStatsQueue !== null && (int)$futureStatsQueue['task_id'] !== $updateStatsTask && (int)$futureStatsQueue['end'] > $now, 'UpdateStats schedules the next future stats snapshot', $futureStatsQueue ?? array()),
        ),
    ));
    DismissAlly($statsAllyId);
    $createdAlliances = array_values(array_filter($createdAlliances, fn($id) => $id !== $statsAllyId));

    e2e_restore_fixture_user($originalAttacker, $attackerPlanet);

    e2e_clear_queues(array(QTYP_CLEAN_DEBRIS));
    $emptyDebrisId = e2e_create_debris(e2e_find_free_debris_slot($reservedDebrisSlots), USER_SPACE, 0, 0);
    $createdDebris[] = $emptyDebrisId;
    $richDebrisId = e2e_create_debris(e2e_find_free_debris_slot($reservedDebrisSlots), USER_SPACE, 99, 88);
    $createdDebris[] = $richDebrisId;
    $targetedDebrisId = e2e_create_debris(e2e_find_free_debris_slot($reservedDebrisSlots), USER_SPACE, 0, 0);
    $createdDebris[] = $targetedDebrisId;
    $origin = LoadPlanetById($attackerPlanet);
    $targetedDebris = LoadPlanetById($targetedDebrisId);
    $recyclerFleet = e2e_empty_fleet();
    $recyclerFleet[GID_F_RECYCLER] = 1;
    $recyclerFleetId = DispatchFleet($recyclerFleet, $origin, $targetedDebris, FTYP_RECYCLE, 3600, e2e_empty_resources(), 0, $now);
    $cleanDebrisTask = e2e_add_due_queue(USER_SPACE, QTYP_CLEAN_DEBRIS, QUEUE_PRIO_CLEAN_DEBRIS, $now);
    e2e_update_queue_twice($now);
    $emptyDebrisAfter = e2e_planet_row($emptyDebrisId);
    $richDebrisAfter = e2e_planet_row($richDebrisId);
    $targetedDebrisAfter = e2e_planet_row($targetedDebrisId);
    $futureCleanDebris = e2e_one_row("SELECT task_id, end FROM {$db_prefix}queue WHERE type='" . QTYP_CLEAN_DEBRIS . "' ORDER BY end DESC LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'clean_debris_queue_removes_only_empty_untargeted_debris',
        'checks' => array(
            e2e_case($emptyDebrisId > 0 && $richDebrisId > 0 && $targetedDebrisId > 0 && $recyclerFleetId > 0 && $cleanDebrisTask > 0, 'debris cleanup fixtures and queue task are created', array('empty' => $emptyDebrisId, 'rich' => $richDebrisId, 'targeted' => $targetedDebrisId, 'fleet' => $recyclerFleetId, 'task' => $cleanDebrisTask)),
            e2e_case($emptyDebrisAfter === null, 'empty untargeted debris field is removed'),
            e2e_case($richDebrisAfter !== null && (int)$richDebrisAfter['metal'] === 99 && (int)$richDebrisAfter['crystal'] === 88, 'non-empty debris field is preserved', $richDebrisAfter ?? array()),
            e2e_case($targetedDebrisAfter !== null, 'empty debris field targeted by a recycler fleet is preserved', $targetedDebrisAfter ?? array()),
            e2e_case($futureCleanDebris !== null && (int)$futureCleanDebris['task_id'] !== $cleanDebrisTask && (int)$futureCleanDebris['end'] > $now, 'CleanDebris schedules the next future cleanup task', $futureCleanDebris ?? array()),
        ),
    ));
    e2e_cleanup_fleets(array($attackerId), array($attackerPlanet, $targetedDebrisId));

    e2e_clear_queues(array(QTYP_CLEAN_PLANETS));
    $expiredCoords = e2e_find_free_planet_slot($reservedPlanetSlots);
    $futureCoords = e2e_find_free_planet_slot($reservedPlanetSlots);
    $expiredPlanetId = CreatePlanet($expiredCoords[0], $expiredCoords[1], $expiredCoords[2], $attackerId, 1, 0, 0, $now);
    $createdPlanets[] = $expiredPlanetId;
    $futurePlanetId = CreatePlanet($futureCoords[0], $futureCoords[1], $futureCoords[2], $attackerId, 1, 0, 0, $now);
    $createdPlanets[] = $futurePlanetId;
    dbquery("UPDATE {$db_prefix}planets SET type=" . PTYP_DEST_PLANET . ", remove=" . ($now - 20) . " WHERE planet_id={$expiredPlanetId}");
    dbquery("UPDATE {$db_prefix}planets SET type=" . PTYP_DEST_PLANET . ", remove=" . ($now + 86400) . " WHERE planet_id={$futurePlanetId}");
    $cleanPlanetsTask = e2e_add_due_queue(USER_SPACE, QTYP_CLEAN_PLANETS, QUEUE_PRIO_CLEAN_PLANETS, $now);
    e2e_update_queue_twice($now);
    $expiredPlanetAfter = e2e_planet_row($expiredPlanetId);
    $futurePlanetAfter = e2e_planet_row($futurePlanetId);
    $futureCleanPlanets = e2e_one_row("SELECT task_id, end FROM {$db_prefix}queue WHERE type='" . QTYP_CLEAN_PLANETS . "' ORDER BY end DESC LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'clean_planets_queue_removes_only_due_removed_planets',
        'checks' => array(
            e2e_case($expiredPlanetId > 0 && $futurePlanetId > 0 && $cleanPlanetsTask > 0, 'removed-planet cleanup fixtures and queue task are created', array('expired' => $expiredPlanetId, 'future' => $futurePlanetId, 'task' => $cleanPlanetsTask)),
            e2e_case($expiredPlanetAfter === null, 'expired removed planet is destroyed', $expiredPlanetAfter ?? array()),
            e2e_case($futurePlanetAfter !== null && (int)$futurePlanetAfter['remove'] > $now, 'future-scheduled removed planet is preserved', $futurePlanetAfter ?? array()),
            e2e_case($futureCleanPlanets !== null && (int)$futureCleanPlanets['task_id'] !== $cleanPlanetsTask && (int)$futureCleanPlanets['end'] > $now, 'CleanPlanets schedules the next future cleanup task', $futureCleanPlanets ?? array()),
        ),
    ));
    $createdPlanets = array_values(array_filter($createdPlanets, fn($id) => $id !== $expiredPlanetId));

    e2e_clear_queues(array(QTYP_CLEAN_PLAYERS));
    $disabledUser = e2e_create_temp_user('disabled');
    $adminUser = e2e_create_temp_user('admin');
    $createdUsers[] = (int)$disabledUser['player_id'];
    $createdUsers[] = (int)$adminUser['player_id'];
    dbquery("UPDATE {$db_prefix}users SET disable=1, disable_until=" . ($now - 20) . ", admin=0, lastclick={$now}, dm=0 WHERE player_id=" . (int)$disabledUser['player_id']);
    dbquery("UPDATE {$db_prefix}users SET disable=1, disable_until=" . ($now - 20) . ", admin=1, lastclick={$now}, dm=0 WHERE player_id=" . (int)$adminUser['player_id']);
    $cleanPlayersTask = e2e_add_due_queue(USER_SPACE, QTYP_CLEAN_PLAYERS, QUEUE_PRIO_CLEAN_PLAYERS, $now);
    e2e_update_queue_twice($now);
    $disabledUserAfter = e2e_user_row((int)$disabledUser['player_id']);
    $disabledPlanetAfter = e2e_planet_row((int)$disabledUser['planet_id']);
    $adminUserAfter = e2e_user_row((int)$adminUser['player_id']);
    $futureCleanPlayers = e2e_one_row("SELECT task_id, end FROM {$db_prefix}queue WHERE type='" . QTYP_CLEAN_PLAYERS . "' ORDER BY end DESC LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'clean_players_queue_removes_due_disabled_non_admin_users',
        'checks' => array(
            e2e_case((int)$disabledUser['player_id'] > 0 && (int)$adminUser['player_id'] > 0 && $cleanPlayersTask > 0, 'clean-player fixtures and queue task are created', array('disabled_user' => $disabledUser, 'admin_user' => $adminUser, 'task' => $cleanPlayersTask)),
            e2e_case($disabledUserAfter === null, 'disabled non-admin user due for deletion is removed'),
            e2e_case($disabledPlanetAfter === null, 'removed disabled user home planet is deleted with the user'),
            e2e_case($adminUserAfter !== null && (int)$adminUserAfter['admin'] === 1, 'disabled admin user is preserved by CleanPlayers', $adminUserAfter ?? array()),
            e2e_case($futureCleanPlayers !== null && (int)$futureCleanPlayers['task_id'] !== $cleanPlayersTask && (int)$futureCleanPlayers['end'] > $now, 'CleanPlayers schedules the next future cleanup task', $futureCleanPlayers ?? array()),
        ),
    ));
    $createdUsers = array_values(array_filter($createdUsers, fn($id) => $id !== (int)$disabledUser['player_id']));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'global_maintenance_queue_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    foreach ($createdAlliances as $allyId) {
        if ($allyId > 0 && e2e_one_row("SELECT ally_id FROM {$db_prefix}ally WHERE ally_id={$allyId} LIMIT 1") !== null) {
            DismissAlly($allyId);
        }
    }
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
    if ($attackerId > 0 && $attackerPlanet > 0) {
        e2e_restore_fixture_user($originalAttacker, $attackerPlanet);
    }
    if ($defenderId > 0 && $defenderPlanet > 0) {
        e2e_restore_fixture_user($originalDefender, $defenderPlanet);
    }
}

echo json_encode(array(
    'case_group' => 'http_global_maintenance_queue',
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
