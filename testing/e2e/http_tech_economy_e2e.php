<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_tech_economy_e2e.php';
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
loca_add('technames', 'en');
loca_add('debug', 'en');

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

function e2e_cleanup_runtime(int $userId, int $planetId): void
{
    global $db_prefix;

    $fleetIds = array();
    $res = dbquery("SELECT fleet_id FROM {$db_prefix}fleet WHERE owner_id={$userId} OR start_planet={$planetId} OR target_planet={$planetId}");
    while ($row = dbarray($res)) {
        $fleetIds[] = (int)$row['fleet_id'];
    }
    if (!empty($fleetIds)) {
        $fleetList = implode(',', $fleetIds);
        dbquery("DELETE FROM {$db_prefix}queue WHERE type='" . QTYP_FLEET . "' AND (owner_id={$userId} OR sub_id IN ({$fleetList}))");
        dbquery("DELETE FROM {$db_prefix}fleet WHERE fleet_id IN ({$fleetList})");
    }

    dbquery(
        "DELETE FROM {$db_prefix}queue WHERE owner_id={$userId} AND type IN ('" .
        QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" .
        QTYP_SHIPYARD . "','" . QTYP_FLEET . "','" . QTYP_DEBUG . "')"
    );
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$userId} OR planet_id={$planetId}");
}

function e2e_set_user_research(int $userId, array $levels): void
{
    global $db_prefix, $resmap;

    $parts = array();
    foreach ($resmap as $gid) {
        $parts[] = "`{$gid}`=" . intval($levels[$gid] ?? 0);
    }
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $parts) . ", " .
        "admin=0, validated=1, deact_ip=1, vacation=0, vacation_until=0, " .
        "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, " .
        "lang='en', skin='/evolution/', useskin=1 WHERE player_id={$userId}"
    );
    InvalidateUserCache();
}

function e2e_set_planet_layout(int $planetId, int $userId, array $buildings, array $ships = array(), array $defence = array()): void
{
    global $fleetmap, $defmap, $rakmap, $buildmap, $db_prefix;

    $objects = e2e_with_units(e2e_with_units(e2e_zero_map(array_diff($defmap, $rakmap)) + e2e_zero_map($fleetmap), $ships), $defence);
    SetPlanetFleetDefense($planetId, $objects);
    SetPlanetBuildings($planetId, e2e_with_units(e2e_zero_map($buildmap), $buildings));

    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "`" . GID_D_ABM . "`=0, `" . GID_D_IPM . "`=0, fields=0, maxfields=300, " .
        "type=" . PTYP_PLANET . ", owner_id={$userId} WHERE planet_id={$planetId}"
    );
}

function e2e_set_planet_economy(
    int $planetId,
    float $metal,
    float $crystal,
    float $deuterium,
    int $lastpeek,
    array $prod = array()
): void {
    global $db_prefix;

    $prod = array_replace(array(
        'prod1' => 1,
        'prod2' => 1,
        'prod3' => 1,
        'prod4' => 1,
        'prod12' => 1,
        'prod212' => 1,
    ), $prod);

    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "`" . GID_RC_METAL . "`={$metal}, `" . GID_RC_CRYSTAL . "`={$crystal}, `" . GID_RC_DEUTERIUM . "`={$deuterium}, " .
        "prod1=" . floatval($prod['prod1']) . ", prod2=" . floatval($prod['prod2']) . ", prod3=" . floatval($prod['prod3']) . ", " .
        "prod4=" . floatval($prod['prod4']) . ", prod12=" . floatval($prod['prod12']) . ", prod212=" . floatval($prod['prod212']) . ", " .
        "temp=20, lastpeek={$lastpeek} WHERE planet_id={$planetId}"
    );
}

function e2e_reset_fixture(int $userId, int $planetId, int $now): void
{
    e2e_cleanup_runtime($userId, $planetId);
    e2e_set_user_research($userId, array());
    e2e_set_planet_layout($planetId, $userId, array());
    e2e_set_planet_economy($planetId, 10000000, 10000000, 10000000, $now);
}

function e2e_queue_count(int $userId, string $type): int
{
    global $db_prefix;
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$userId} AND type='{$type}'");
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$cases = array();
$now = time();

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    e2e_reset_fixture($attackerId, $attackerPlanet, $now);
    e2e_set_user_research($attackerId, array(GID_R_COMPUTER => 10));
    e2e_set_planet_layout($attackerPlanet, $attackerId, array(GID_B_ROBOTS => 9));
    $lockedUser = LoadUser($attackerId);
    $lockedPlanet = GetUpdatePlanet($attackerPlanet, $now);
    $lockedText = CanBuild($lockedUser, $lockedPlanet, GID_B_NANITES, 1, false);
    $lockedEnqueueText = BuildEnque($lockedUser, $attackerPlanet, GID_B_NANITES, 0, $now + 1);

    e2e_cleanup_runtime($attackerId, $attackerPlanet);
    e2e_set_planet_layout($attackerPlanet, $attackerId, array(GID_B_ROBOTS => 10));
    e2e_set_planet_economy($attackerPlanet, 10000000, 10000000, 10000000, $now + 2);
    $unlockedUser = LoadUser($attackerId);
    $unlockedPlanet = GetUpdatePlanet($attackerPlanet, $now + 2);
    $unlockedText = CanBuild($unlockedUser, $unlockedPlanet, GID_B_NANITES, 1, false);
    $unlockedEnqueueText = BuildEnque($unlockedUser, $attackerPlanet, GID_B_NANITES, 0, $now + 3);
    $naniteTask = e2e_one_row("SELECT task_id, obj_id, level FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "' ORDER BY task_id DESC LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'building_requirement_gate_nanites',
        'checks' => array(
            e2e_case($lockedText !== '', 'nanite factory is locked below robotics requirement', array('message' => $lockedText)),
            e2e_case($lockedEnqueueText !== '', 'locked nanite factory cannot be queued', array('message' => $lockedEnqueueText)),
            e2e_case($unlockedText === '', 'nanite factory unlocks when robotics and computer requirements are met', array('message' => $unlockedText)),
            e2e_case($unlockedEnqueueText === '', 'unlocked nanite factory queues successfully', array('message' => $unlockedEnqueueText)),
            e2e_case($naniteTask !== null && (int)$naniteTask['obj_id'] === GID_B_NANITES && (int)$naniteTask['level'] === 1, 'nanite build queue task is created', $naniteTask ?? array()),
        ),
    ));

    e2e_reset_fixture($attackerId, $attackerPlanet, $now + 10);
    e2e_set_planet_layout($attackerPlanet, $attackerId, array(GID_B_RES_LAB => 6));
    e2e_set_user_research($attackerId, array(GID_R_ENERGY => 2));
    $lockedResearchText = StartResearch($attackerId, $attackerPlanet, GID_R_SHIELD, $now + 11);
    $lockedResearchCount = e2e_queue_count($attackerId, QTYP_RESEARCH);

    e2e_cleanup_runtime($attackerId, $attackerPlanet);
    e2e_set_planet_layout($attackerPlanet, $attackerId, array(GID_B_RES_LAB => 6));
    e2e_set_planet_economy($attackerPlanet, 10000000, 10000000, 10000000, $now + 12);
    e2e_set_user_research($attackerId, array(GID_R_ENERGY => 3));
    $unlockedResearchText = StartResearch($attackerId, $attackerPlanet, GID_R_SHIELD, $now + 13);
    $shieldTask = e2e_one_row("SELECT task_id, obj_id, level FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_RESEARCH . "' ORDER BY task_id DESC LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'research_requirement_gate_shield',
        'checks' => array(
            e2e_case($lockedResearchText !== '', 'shielding technology is locked below energy requirement', array('message' => $lockedResearchText)),
            e2e_case($lockedResearchCount === 0, 'locked research does not create a queue task', array('queue_count' => $lockedResearchCount)),
            e2e_case($unlockedResearchText === '', 'shielding technology unlocks when lab and energy requirements are met', array('message' => $unlockedResearchText)),
            e2e_case($shieldTask !== null && (int)$shieldTask['obj_id'] === GID_R_SHIELD && (int)$shieldTask['level'] === 1, 'shield research queue task is created', $shieldTask ?? array()),
        ),
    ));

    e2e_reset_fixture($attackerId, $attackerPlanet, $now + 20);
    e2e_set_planet_layout($attackerPlanet, $attackerId, array(GID_B_SHIPYARD => 5));
    e2e_set_user_research($attackerId, array(GID_R_IMPULSE_DRIVE => 4, GID_R_ION_TECH => 1));
    $lockedCruiser = AddShipyard($attackerId, $attackerPlanet, GID_F_CRUISER, 1, $now + 21);
    $lockedShipyardCount = e2e_queue_count($attackerId, QTYP_SHIPYARD);

    e2e_cleanup_runtime($attackerId, $attackerPlanet);
    e2e_set_planet_layout($attackerPlanet, $attackerId, array(GID_B_SHIPYARD => 5));
    e2e_set_planet_economy($attackerPlanet, 10000000, 10000000, 10000000, $now + 22);
    e2e_set_user_research($attackerId, array(GID_R_IMPULSE_DRIVE => 4, GID_R_ION_TECH => 2));
    $unlockedCruiser = AddShipyard($attackerId, $attackerPlanet, GID_F_CRUISER, 1, $now + 23);
    $cruiserTask = e2e_one_row("SELECT task_id, obj_id, level FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_SHIPYARD . "' ORDER BY task_id DESC LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'shipyard_requirement_gate_cruiser',
        'checks' => array(
            e2e_case($lockedCruiser === false, 'cruiser build is locked below ion technology requirement'),
            e2e_case($lockedShipyardCount === 0, 'locked cruiser build does not create a shipyard queue task', array('queue_count' => $lockedShipyardCount)),
            e2e_case($unlockedCruiser === true, 'cruiser build unlocks when shipyard and research requirements are met'),
            e2e_case($cruiserTask !== null && (int)$cruiserTask['obj_id'] === GID_F_CRUISER && (int)$cruiserTask['level'] === 1, 'cruiser shipyard queue task is created', $cruiserTask ?? array()),
        ),
    ));

    e2e_reset_fixture($attackerId, $attackerPlanet, $now + 30);
    e2e_set_planet_layout($attackerPlanet, $attackerId, array(GID_B_SHIPYARD => 8));
    e2e_set_user_research($attackerId, array(GID_R_PLASMA_TECH => 6));
    $lockedPlasma = AddShipyard($attackerId, $attackerPlanet, GID_D_PLASMA, 1, $now + 31);
    $lockedPlasmaCount = e2e_queue_count($attackerId, QTYP_SHIPYARD);

    e2e_cleanup_runtime($attackerId, $attackerPlanet);
    e2e_set_planet_layout($attackerPlanet, $attackerId, array(GID_B_SHIPYARD => 8));
    e2e_set_planet_economy($attackerPlanet, 10000000, 10000000, 10000000, $now + 32);
    e2e_set_user_research($attackerId, array(GID_R_PLASMA_TECH => 7));
    $unlockedPlasma = AddShipyard($attackerId, $attackerPlanet, GID_D_PLASMA, 1, $now + 33);
    $plasmaTask = e2e_one_row("SELECT task_id, obj_id, level FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_SHIPYARD . "' ORDER BY task_id DESC LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'defense_requirement_gate_plasma_turret',
        'checks' => array(
            e2e_case($lockedPlasma === false, 'plasma turret is locked below plasma technology requirement'),
            e2e_case($lockedPlasmaCount === 0, 'locked plasma turret does not create a shipyard queue task', array('queue_count' => $lockedPlasmaCount)),
            e2e_case($unlockedPlasma === true, 'plasma turret unlocks when shipyard and research requirements are met'),
            e2e_case($plasmaTask !== null && (int)$plasmaTask['obj_id'] === GID_D_PLASMA && (int)$plasmaTask['level'] === 1, 'plasma turret shipyard queue task is created', $plasmaTask ?? array()),
        ),
    ));

    e2e_reset_fixture($attackerId, $attackerPlanet, $now + 40);
    e2e_set_planet_layout($attackerPlanet, $attackerId, array(GID_B_METAL_MINE => 10));
    e2e_set_planet_economy($attackerPlanet, 0, 0, 0, $now + 40);
    $shortagePlanet = GetUpdatePlanet($attackerPlanet, $now + 40 + 3600);
    $shortageMetal = (float)$shortagePlanet[GID_RC_METAL];
    $naturalMetal = 20 * (float)$GlobalUni['speed'];

    e2e_reset_fixture($attackerId, $attackerPlanet, $now + 50);
    e2e_set_planet_layout($attackerPlanet, $attackerId, array(GID_B_METAL_MINE => 10, GID_B_SOLAR => 20));
    e2e_set_planet_economy($attackerPlanet, 0, 0, 0, $now + 50);
    $poweredPlanet = GetUpdatePlanet($attackerPlanet, $now + 50 + 3600);
    $poweredMetal = (float)$poweredPlanet[GID_RC_METAL];
    $cases[] = e2e_finalize_case(array(
        'case' => 'energy_shortage_reduces_mine_output',
        'checks' => array(
            e2e_case((float)$shortagePlanet['factor'] < 0.01, 'planet production factor drops to zero during full energy shortage', array('factor' => $shortagePlanet['factor'], 'energy_balance' => $shortagePlanet['balance'][GID_RC_ENERGY])),
            e2e_case($shortageMetal > 0 && $shortageMetal <= $naturalMetal + 1, 'energy-starved planet only gains natural metal production', array('metal' => $shortageMetal, 'natural_hourly' => $naturalMetal)),
            e2e_case((float)$poweredPlanet['factor'] >= 0.99, 'powered planet keeps full production factor', array('factor' => $poweredPlanet['factor'], 'energy_balance' => $poweredPlanet['balance'][GID_RC_ENERGY])),
            e2e_case($poweredMetal > $shortageMetal * 2, 'powered mines produce materially more metal than energy-starved mines', array('powered_metal' => $poweredMetal, 'shortage_metal' => $shortageMetal)),
        ),
    ));

    e2e_reset_fixture($attackerId, $attackerPlanet, $now + 60);
    e2e_set_planet_layout($attackerPlanet, $attackerId, array(GID_B_METAL_MINE => 20, GID_B_SOLAR => 30, GID_B_METAL_STOR => 0));
    e2e_set_planet_economy($attackerPlanet, 99990, 0, 0, $now + 60);
    $cappedPlanet = GetUpdatePlanet($attackerPlanet, $now + 60 + 3600);
    $cases[] = e2e_finalize_case(array(
        'case' => 'storage_capacity_caps_resource_growth',
        'checks' => array(
            e2e_case(isset($cappedPlanet['max' . GID_RC_METAL]) && (int)$cappedPlanet['max' . GID_RC_METAL] === store_capacity(0), 'metal storage capacity is calculated for storage level zero', array('max_metal' => $cappedPlanet['max' . GID_RC_METAL] ?? null)),
            e2e_case((float)$cappedPlanet[GID_RC_METAL] <= (float)$cappedPlanet['max' . GID_RC_METAL], 'metal never exceeds storage capacity after production tick', array('metal' => $cappedPlanet[GID_RC_METAL], 'max_metal' => $cappedPlanet['max' . GID_RC_METAL])),
            e2e_case(abs((float)$cappedPlanet[GID_RC_METAL] - (float)$cappedPlanet['max' . GID_RC_METAL]) < 0.001, 'metal production fills exactly to the storage cap', array('metal' => $cappedPlanet[GID_RC_METAL], 'max_metal' => $cappedPlanet['max' . GID_RC_METAL])),
        ),
    ));

    e2e_reset_fixture($attackerId, $attackerPlanet, $now + 70);
    e2e_set_planet_layout($attackerPlanet, $attackerId, array(GID_B_METAL_MINE => 10, GID_B_SOLAR => 20));
    e2e_set_planet_economy($attackerPlanet, 0, 0, 0, $now + 70, array('prod1' => 1));
    $fullProductionPlanet = GetUpdatePlanet($attackerPlanet, $now + 70 + 3600);
    $fullProductionMetal = (float)$fullProductionPlanet[GID_RC_METAL];

    e2e_reset_fixture($attackerId, $attackerPlanet, $now + 80);
    e2e_set_planet_layout($attackerPlanet, $attackerId, array(GID_B_METAL_MINE => 10, GID_B_SOLAR => 20));
    e2e_set_planet_economy($attackerPlanet, 0, 0, 0, $now + 80, array('prod1' => 0));
    $zeroProductionPlanet = GetUpdatePlanet($attackerPlanet, $now + 80 + 3600);
    $zeroProductionMetal = (float)$zeroProductionPlanet[GID_RC_METAL];
    $cases[] = e2e_finalize_case(array(
        'case' => 'production_ratio_controls_resource_tick',
        'checks' => array(
            e2e_case($zeroProductionMetal > 0 && $zeroProductionMetal <= $naturalMetal + 1, 'zero-percent metal mine keeps only natural metal production', array('metal' => $zeroProductionMetal, 'natural_hourly' => $naturalMetal)),
            e2e_case($fullProductionMetal > $zeroProductionMetal * 2, 'one-hundred-percent metal mine produces more than zero-percent setting', array('full_metal' => $fullProductionMetal, 'zero_metal' => $zeroProductionMetal)),
            e2e_case((float)$fullProductionPlanet['factor'] >= 0.99 && (float)$zeroProductionPlanet['factor'] >= 0.99, 'production ratio comparison is not caused by energy shortage', array('full_factor' => $fullProductionPlanet['factor'], 'zero_factor' => $zeroProductionPlanet['factor'])),
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'tech_economy_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($attackerId > 0 && $attackerPlanet > 0) {
        e2e_reset_fixture($attackerId, $attackerPlanet, time());
    }
}

echo json_encode(array(
    'case_group' => 'http_tech_economy',
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
