<?php

ob_start();
error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = $_SERVER['HTTP_USER_AGENT'] ?? 'ogame-e2e';
$_SERVER['REQUEST_URI'] = $_SERVER['REQUEST_URI'] ?? '/testing/e2e';
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
loca_add('technames', 'en');
loca_add('fleetmsg', 'en');
loca_add('battlereport', 'en');
loca_add('espionage', 'en');

$ATTACKER_ID = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 100162);
$DEFENDER_ID = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 100161);
$ATTACKER_PLANET = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 10163);
$DEFENDER_PLANET = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 10161);

function sql_exec(string $sql): mixed
{
    $res = dbquery($sql);
    if ($res === false) {
        throw new RuntimeException('SQL failed: ' . $sql);
    }
    return $res;
}

function sql_row(string $sql): ?array
{
    $res = sql_exec($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function sql_value(string $sql): mixed
{
    $row = sql_row($sql);
    if ($row === null) return null;
    return array_values($row)[0];
}

function zero_map(array $map): array
{
    $out = array();
    foreach ($map as $gid) $out[$gid] = 0;
    return $out;
}

function with_units(array $base, array $units): array
{
    foreach ($units as $gid => $amount) $base[$gid] = $amount;
    return $base;
}

function cleanup_runtime(): void
{
    global $db_prefix, $ATTACKER_ID, $DEFENDER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET;
    $users = "{$ATTACKER_ID},{$DEFENDER_ID}";
    $planets = "{$ATTACKER_PLANET},{$DEFENDER_PLANET}";

    sql_exec("DELETE FROM {$db_prefix}queue WHERE owner_id IN ({$users}) OR (type='" . QTYP_FLEET . "' AND sub_id IN (SELECT fleet_id FROM {$db_prefix}fleet WHERE owner_id IN ({$users}) OR start_planet IN ({$planets}) OR target_planet IN ({$planets})))");
    sql_exec("DELETE FROM {$db_prefix}fleet WHERE owner_id IN ({$users}) OR start_planet IN ({$planets}) OR target_planet IN ({$planets})");
    sql_exec("DELETE FROM {$db_prefix}buildqueue WHERE owner_id IN ({$users})");
    $target = LoadPlanetById($DEFENDER_PLANET);
    if ($target !== null) {
        sql_exec("DELETE FROM {$db_prefix}planets WHERE type IN (" . PTYP_MOON . "," . PTYP_DF . ") AND g=" . intval($target['g']) . " AND s=" . intval($target['s']) . " AND p=" . intval($target['p']));
    }
}

function set_user_tech(int $userId, array $tech): void
{
    global $db_prefix, $resmap;
    $parts = array();
    foreach ($resmap as $gid) $parts[] = "`{$gid}`=" . intval($tech[$gid] ?? 0);
    sql_exec("UPDATE {$db_prefix}users SET " . implode(',', $parts) . ", validated=1, deact_ip=1, lang='en', skin='/evolution/', useskin=1 WHERE player_id={$userId}");
    InvalidateUserCache();
}

function set_planet_state(int $planetId, string $name, array $ships, array $defence, array $buildings, array $resources): void
{
    global $db_prefix, $fleetmap, $defmap, $rakmap, $buildmap;
    $objects = with_units(with_units(zero_map(array_diff($defmap, $rakmap)) + zero_map($fleetmap), $ships), $defence);
    SetPlanetFleetDefense($planetId, $objects);
    SetPlanetBuildings($planetId, with_units(zero_map($buildmap), $buildings));

    sql_exec(
        "UPDATE {$db_prefix}planets SET name='{$name}', " .
        "`700`=" . floatval($resources[700] ?? 0) . ", " .
        "`701`=" . floatval($resources[701] ?? 0) . ", " .
        "`702`=" . floatval($resources[702] ?? 0) . ", " .
        "`502`=0, `503`=0, lastpeek=" . time() . " WHERE planet_id={$planetId}"
    );
}

function dispatch_test_fleet(int $mission, int $originPlanetId, int $targetPlanetId, array $ships, int $when): array
{
    global $fleetmap, $transportableResources;
    $fleet = with_units(zero_map($fleetmap), $ships);
    $resources = zero_map($transportableResources);
    $origin = LoadPlanetById($originPlanetId);
    $target = LoadPlanetById($targetPlanetId);

    AdjustShips($fleet, $originPlanetId, '-');
    $fleetId = DispatchFleet($fleet, $origin, $target, $mission, 1, $resources, 0, $when);
    return array($fleetId, GetFleetQueue($fleetId));
}

function latest_return_fleet(int $ownerId, int $mission): ?array
{
    global $db_prefix;
    return sql_row("SELECT fleet_id, mission, start_planet, target_planet, `700`, `701`, `702`, `202`, `203`, `204`, `207`, `209` FROM {$db_prefix}fleet WHERE owner_id={$ownerId} AND mission={$mission} ORDER BY fleet_id DESC LIMIT 1");
}

function debris_at_target(): ?array
{
    global $db_prefix;
    global $DEFENDER_PLANET;
    $target = LoadPlanetById($DEFENDER_PLANET);
    return sql_row("SELECT planet_id, `700`, `701`, `702` FROM {$db_prefix}planets WHERE g=" . intval($target['g']) . " AND s=" . intval($target['s']) . " AND p=" . intval($target['p']) . " AND type=" . PTYP_DF . " LIMIT 1");
}

function assert_case(bool $condition, string $message, array $context = array()): array
{
    return array('pass' => $condition, 'message' => $message, 'context' => $context);
}

function run_plunder_debris_case(): array
{
    global $ATTACKER_ID, $DEFENDER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET;

    cleanup_runtime();
    mt_srand(100);
    srand(100);
    set_user_tech($ATTACKER_ID, array(106 => 10, 109 => 10, 110 => 10, 111 => 10));
    set_user_tech($DEFENDER_ID, array(106 => 0, 109 => 0, 110 => 0, 111 => 0));

    $attackFleet = array(GID_F_LC => 20, GID_F_BATTLESHIP => 10);
    set_planet_state($ATTACKER_PLANET, 'ReportAttacker', $attackFleet, array(), array(), array(700 => 1000000, 701 => 1000000, 702 => 1000000));
    set_planet_state(
        $DEFENDER_PLANET,
        'ReportDefender',
        array(GID_F_LF => 5, GID_F_SC => 5),
        array(GID_D_RL => 8, GID_D_LL => 4),
        array(GID_B_METAL_MINE => 10, GID_B_CRYS_MINE => 8, GID_B_DEUT_SYNTH => 6),
        array(700 => 900000, 701 => 600000, 702 => 300000)
    );

    $before = LoadPlanetById($DEFENDER_PLANET);
    [$fleetId, $queue] = dispatch_test_fleet(FTYP_ATTACK, $ATTACKER_PLANET, $DEFENDER_PLANET, $attackFleet, time() + 1);
    $battleResult = StartBattle($fleetId, $DEFENDER_PLANET, intval($queue['end']));
    DeleteFleet($fleetId);
    RemoveQueue(intval($queue['task_id']));

    $after = LoadPlanetById($DEFENDER_PLANET);
    $returnFleet = latest_return_fleet($ATTACKER_ID, FTYP_ATTACK + FTYP_RETURN);
    $debris = debris_at_target();

    $captured = array(
        700 => intval(floor(floatval($before[700]) - floatval($after[700]))),
        701 => intval(floor(floatval($before[701]) - floatval($after[701]))),
        702 => intval(floor(floatval($before[702]) - floatval($after[702]))),
    );

    return array(
        'case' => 'plunder_debris_defence_writeback',
        'battle_result' => $battleResult,
        'checks' => array(
            assert_case($battleResult === BATTLE_RESULT_AWON, 'attacker wins'),
            assert_case(array_sum($captured) > 0, 'defender resources decrease by captured amount', array('captured' => $captured)),
            assert_case($returnFleet !== null && intval($returnFleet[700]) === $captured[700] && intval($returnFleet[701]) === $captured[701] && intval($returnFleet[702]) === $captured[702], 'captured resources are loaded onto the attack return fleet', array('return_fleet' => $returnFleet, 'captured' => $captured)),
            assert_case($debris !== null && (floatval($debris[700]) + floatval($debris[701])) > 0, 'debris field is created with metal/crystal', array('debris' => $debris)),
            assert_case(intval($after[GID_F_LF]) === 0 && intval($after[GID_F_SC]) === 0, 'defender fleet losses are written back to planet', array('after_204' => $after[GID_F_LF], 'after_202' => $after[GID_F_SC])),
            assert_case(intval($after[GID_D_RL]) >= 0 && intval($after[GID_D_LL]) >= 0, 'defensive structures remain valid non-negative counts after combat', array('after_401' => $after[GID_D_RL], 'after_402' => $after[GID_D_LL])),
        ),
    );
}

function run_defence_win_case(): array
{
    global $ATTACKER_ID, $DEFENDER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET;

    cleanup_runtime();
    mt_srand(200);
    srand(200);
    set_user_tech($ATTACKER_ID, array(106 => 0, 109 => 0, 110 => 0, 111 => 0));
    set_user_tech($DEFENDER_ID, array(106 => 0, 109 => 5, 110 => 5, 111 => 5));

    $attackFleet = array(GID_F_LF => 1);
    set_planet_state($ATTACKER_PLANET, 'ReportAttacker', $attackFleet, array(), array(), array(700 => 1000000, 701 => 1000000, 702 => 1000000));
    set_planet_state($DEFENDER_PLANET, 'ReportDefender', array(), array(GID_D_PLASMA => 10, GID_D_RL => 20), array(), array(700 => 100000, 701 => 100000, 702 => 100000));

    [$fleetId, $queue] = dispatch_test_fleet(FTYP_ATTACK, $ATTACKER_PLANET, $DEFENDER_PLANET, $attackFleet, time() + 1);
    $battleResult = StartBattle($fleetId, $DEFENDER_PLANET, intval($queue['end']));
    DeleteFleet($fleetId);
    RemoveQueue(intval($queue['task_id']));
    $after = LoadPlanetById($DEFENDER_PLANET);
    $returnFleet = latest_return_fleet($ATTACKER_ID, FTYP_ATTACK + FTYP_RETURN);
    $debris = debris_at_target();

    return array(
        'case' => 'defence_buildings_destroy_attacker',
        'battle_result' => $battleResult,
        'checks' => array(
            assert_case($battleResult === BATTLE_RESULT_DWON, 'defender wins by defensive structures'),
            assert_case($returnFleet === null, 'destroyed attacker does not get a return fleet'),
            assert_case(intval($after[GID_D_PLASMA]) > 0 && intval($after[GID_D_RL]) > 0, 'defensive structure counts remain on planet', array('plasma' => $after[GID_D_PLASMA], 'rocket_launcher' => $after[GID_D_RL])),
            assert_case($debris !== null && (floatval($debris[700]) + floatval($debris[701])) > 0, 'attacker fleet loss creates debris', array('debris' => $debris)),
        ),
    );
}

function run_recycle_case(): array
{
    global $ATTACKER_ID, $DEFENDER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET;

    cleanup_runtime();
    set_user_tech($ATTACKER_ID, array(106 => 10, 109 => 10, 110 => 10, 111 => 10));
    set_user_tech($DEFENDER_ID, array(106 => 10, 109 => 10, 110 => 10, 111 => 10));

    set_planet_state($ATTACKER_PLANET, 'ReportAttacker', array(GID_F_RECYCLER => 5), array(), array(), array(700 => 1000000, 701 => 1000000, 702 => 1000000));
    set_planet_state($DEFENDER_PLANET, 'ReportDefender', array(), array(), array(), array(700 => 0, 701 => 0, 702 => 0));

    $target = LoadPlanetById($DEFENDER_PLANET);
    $dfId = CreateDebris(intval($target['g']), intval($target['s']), intval($target['p']), $DEFENDER_ID);
    AddDebris($dfId, 120000, 80000);
    $beforeOrigin = LoadPlanetById($ATTACKER_PLANET);
    $beforeDebris = LoadPlanetById($dfId);

    [$fleetId, $queue] = dispatch_test_fleet(FTYP_RECYCLE, $ATTACKER_PLANET, $dfId, array(GID_F_RECYCLER => 5), time() + 1);
    Queue_Fleet_End($queue);

    $afterHarvestDebris = LoadPlanetById($dfId);
    $returnFleet = latest_return_fleet($ATTACKER_ID, FTYP_RECYCLE + FTYP_RETURN);
    $returnQueue = $returnFleet ? GetFleetQueue(intval($returnFleet['fleet_id'])) : null;

    if ($returnQueue) {
        Queue_Fleet_End($returnQueue);
    }

    $afterOrigin = LoadPlanetById($ATTACKER_PLANET);
    $afterReturnFleet = latest_return_fleet($ATTACKER_ID, FTYP_RECYCLE + FTYP_RETURN);

    $harvested = array(
        700 => intval(floor(floatval($beforeDebris[700]) - floatval($afterHarvestDebris[700]))),
        701 => intval(floor(floatval($beforeDebris[701]) - floatval($afterHarvestDebris[701]))),
    );
    $originGain = array(
        700 => intval(floor(floatval($afterOrigin[700]) - floatval($beforeOrigin[700]))),
        701 => intval(floor(floatval($afterOrigin[701]) - floatval($beforeOrigin[701]))),
    );

    return array(
        'case' => 'debris_recycle_and_return',
        'checks' => array(
            assert_case($harvested[700] === 50000 && $harvested[701] === 50000, 'recyclers harvest expected partial debris amount', array('harvested' => $harvested, 'after_debris' => array(700 => $afterHarvestDebris[700], 701 => $afterHarvestDebris[701]))),
            assert_case($returnFleet !== null && intval($returnFleet[700]) === 50000 && intval($returnFleet[701]) === 50000, 'harvested debris is loaded onto recycle return fleet', array('return_fleet' => $returnFleet)),
            assert_case($originGain[700] === 50000 && $originGain[701] === 50000, 'returning recyclers unload harvested debris onto origin planet', array('origin_gain' => $originGain)),
            assert_case(intval($afterOrigin[GID_F_RECYCLER]) === 5, 'recyclers return to origin planet', array('recyclers_after' => $afterOrigin[GID_F_RECYCLER])),
            assert_case($afterReturnFleet === null, 'recycle return fleet is removed after return'),
        ),
    );
}

function restore_test_accounts(): array
{
    global $db_prefix, $ATTACKER_ID, $DEFENDER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET;
    cleanup_runtime();
    set_user_tech($DEFENDER_ID, array(106 => 10, 109 => 10, 110 => 10, 111 => 10, 115 => 10, 117 => 10, 118 => 10));
    set_user_tech($ATTACKER_ID, array(106 => 10, 109 => 10, 110 => 10, 111 => 10, 115 => 10, 117 => 10, 118 => 10));
    set_planet_state($ATTACKER_PLANET, 'Homeplanet', array(GID_F_SC => 20, GID_F_LF => 5, GID_F_PROBE => 5), array(GID_D_RL => 10, GID_D_LL => 10), array(), array(700 => 50000000, 701 => 50000000, 702 => 50000000));
    set_planet_state($DEFENDER_PLANET, 'FlowP100161', array(GID_F_SC => 20, GID_F_LF => 5, GID_F_PROBE => 5), array(GID_D_RL => 10, GID_D_LL => 10), array(), array(700 => 50000000, 701 => 50000000, 702 => 50000000));
    return array(
        'attacker' => sql_row("SELECT planet_id, name, `700`, `701`, `702`, `202`, `204`, `210`, `401`, `402` FROM {$db_prefix}planets WHERE planet_id={$ATTACKER_PLANET}"),
        'defender' => sql_row("SELECT planet_id, name, `700`, `701`, `702`, `202`, `204`, `210`, `401`, `402` FROM {$db_prefix}planets WHERE planet_id={$DEFENDER_PLANET}"),
    );
}

$result = array('cases' => array());

$result['cases'][] = run_plunder_debris_case();
$result['cases'][] = run_defence_win_case();
$result['cases'][] = run_recycle_case();
$result['restored'] = restore_test_accounts();

$noise = trim(ob_get_clean());
$result['captured_output'] = $noise;

$allPass = true;
foreach ($result['cases'] as $case) {
    foreach ($case['checks'] as $check) {
        if (!$check['pass']) $allPass = false;
    }
}
$result['all_pass'] = $allPass;

echo json_encode($result, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
