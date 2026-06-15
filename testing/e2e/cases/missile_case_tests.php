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
loca_add('raketen', 'en');

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
    sql_exec("DELETE FROM {$db_prefix}planets WHERE type IN (" . PTYP_MOON . "," . PTYP_DF . ") AND g=1 AND s=24 AND p IN (7,8)");
}

function set_user_tech(int $userId, array $tech): void
{
    global $db_prefix, $resmap;
    $parts = array();
    foreach ($resmap as $gid) $parts[] = "`{$gid}`=" . intval($tech[$gid] ?? 0);
    sql_exec("UPDATE {$db_prefix}users SET " . implode(',', $parts) . ", validated=1, deact_ip=1, lang='en', skin='/evolution/', useskin=1 WHERE player_id={$userId}");
    InvalidateUserCache();
}

function set_planet_defense_state(int $planetId, string $name, array $defense, array $resources = array()): void
{
    global $db_prefix, $fleetmap, $defmap, $buildmap;

    SetPlanetDefense($planetId, with_units(zero_map($defmap), $defense));
    SetPlanetFleetDefense($planetId, zero_map($fleetmap) + with_units(zero_map(array_diff($defmap, array(GID_D_ABM, GID_D_IPM))), $defense));
    SetPlanetBuildings($planetId, with_units(zero_map($buildmap), array(GID_B_MISS_SILO => 8)));
    sql_exec(
        "UPDATE {$db_prefix}planets SET name='{$name}', " .
        "`700`=" . floatval($resources[700] ?? 50000000) . ", " .
        "`701`=" . floatval($resources[701] ?? 50000000) . ", " .
        "`702`=" . floatval($resources[702] ?? 50000000) . ", " .
        "lastpeek=" . time() . " WHERE planet_id={$planetId}"
    );
}

function max_msg_id(): int
{
    global $db_prefix;
    return intval(sql_value("SELECT COALESCE(MAX(msg_id), 0) FROM {$db_prefix}messages"));
}

function message_rows(int $ownerId, int $after): array
{
    global $db_prefix;
    $rows = array();
    $res = sql_exec("SELECT msg_id, owner_id, pm, subj, text FROM {$db_prefix}messages WHERE owner_id={$ownerId} AND msg_id>{$after} ORDER BY msg_id ASC");
    while ($row = dbarray($res)) {
        $text = $row['text'] ?? '';
        $rows[] = array(
            'msg_id' => intval($row['msg_id']),
            'owner_id' => intval($row['owner_id']),
            'pm' => intval($row['pm']),
            'text_len' => strlen($text),
            'preview' => mb_substr(trim(preg_replace('/\s+/', ' ', html_entity_decode(strip_tags($text), ENT_QUOTES | ENT_HTML5, 'UTF-8'))), 0, 260, 'UTF-8'),
            'flags' => array(
                'missile_attack' => str_contains($text, 'missile') || str_contains($row['subj'], 'Missile attack'),
                'intercepted' => str_contains($text, 'destroyed by your interceptor missiles'),
                'defense_table' => str_contains($text, 'Defeated Defense'),
                'rocket_launcher' => str_contains($text, 'Rocket Launcher'),
                'light_laser' => str_contains($text, 'Light Laser'),
                'plasma' => str_contains($text, 'Plasma Turret'),
            ),
        );
    }
    return $rows;
}

function current_fleet_count(): int
{
    global $db_prefix, $ATTACKER_ID, $DEFENDER_ID;
    return intval(sql_value("SELECT COUNT(*) FROM {$db_prefix}fleet WHERE owner_id IN ({$ATTACKER_ID},{$DEFENDER_ID})"));
}

function current_queue_count(): int
{
    global $db_prefix, $ATTACKER_ID, $DEFENDER_ID;
    return intval(sql_value("SELECT COUNT(*) FROM {$db_prefix}queue WHERE owner_id IN ({$ATTACKER_ID},{$DEFENDER_ID}) AND type='" . QTYP_FLEET . "'"));
}

function assert_case(bool $condition, string $message, array $context = array()): array
{
    return array('pass' => $condition, 'message' => $message, 'context' => $context);
}

function run_missile_case(string $name, int $amount, int $targetType, array $attackerDefense, array $defenderDefense, array $attackerTech, array $defenderTech): array
{
    global $ATTACKER_ID, $DEFENDER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET;

    cleanup_runtime();
    set_user_tech($ATTACKER_ID, $attackerTech);
    set_user_tech($DEFENDER_ID, $defenderTech);
    set_planet_defense_state($ATTACKER_PLANET, 'MissileAttacker', $attackerDefense);
    set_planet_defense_state($DEFENDER_PLANET, 'MissileDefender', $defenderDefense);

    $beforeMessage = max_msg_id();
    $originBefore = LoadPlanetById($ATTACKER_PLANET);
    $targetBefore = LoadPlanetById($DEFENDER_PLANET);
    $fleetId = LaunchRockets($originBefore, $targetBefore, 1, $amount, $targetType);
    $originAfterLaunch = LoadPlanetById($ATTACKER_PLANET);
    $queue = $fleetId ? GetFleetQueue($fleetId) : null;
    if ($queue) {
        Queue_Fleet_End($queue);
    }
    $originAfter = LoadPlanetById($ATTACKER_PLANET);
    $targetAfter = LoadPlanetById($DEFENDER_PLANET);

    return array(
        'case' => $name,
        'fleet_id' => $fleetId,
        'before' => summarize_planet($targetBefore),
        'after' => summarize_planet($targetAfter),
        'origin_after_launch' => summarize_planet($originAfterLaunch),
        'origin_after' => summarize_planet($originAfter),
        'attacker_messages' => message_rows($ATTACKER_ID, $beforeMessage),
        'defender_messages' => message_rows($DEFENDER_ID, $beforeMessage),
        'fleet_count_after' => current_fleet_count(),
        'queue_count_after' => current_queue_count(),
    );
}

function summarize_planet(array $planet): array
{
    return array(
        'planet_id' => intval($planet['planet_id']),
        'name' => $planet['name'],
        'rocket_launcher' => intval($planet[GID_D_RL]),
        'light_laser' => intval($planet[GID_D_LL]),
        'heavy_laser' => intval($planet[GID_D_HL]),
        'gauss' => intval($planet[GID_D_GAUSS]),
        'ion' => intval($planet[GID_D_ION]),
        'plasma' => intval($planet[GID_D_PLASMA]),
        'small_dome' => intval($planet[GID_D_SDOME]),
        'large_dome' => intval($planet[GID_D_LDOME]),
        'abm' => intval($planet[GID_D_ABM]),
        'ipm' => intval($planet[GID_D_IPM]),
    );
}

function with_checks(array $case, array $checks): array
{
    $case['checks'] = $checks;
    return $case;
}

function restore_test_accounts(): array
{
    global $db_prefix, $ATTACKER_ID, $DEFENDER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET;
    cleanup_runtime();
    set_user_tech($DEFENDER_ID, array(106 => 10, 109 => 10, 110 => 10, 111 => 10, 115 => 10, 117 => 10, 118 => 10));
    set_user_tech($ATTACKER_ID, array(106 => 10, 109 => 10, 110 => 10, 111 => 10, 115 => 10, 117 => 10, 118 => 10));
    set_planet_defense_state($ATTACKER_PLANET, 'Homeplanet', array(GID_D_RL => 10, GID_D_LL => 10, GID_D_IPM => 0, GID_D_ABM => 0));
    set_planet_defense_state($DEFENDER_PLANET, 'FlowP100161', array(GID_D_RL => 10, GID_D_LL => 10, GID_D_IPM => 0, GID_D_ABM => 0));
    sql_exec("UPDATE {$db_prefix}planets SET `202`=20, `204`=5, `210`=5 WHERE planet_id IN ({$ATTACKER_PLANET},{$DEFENDER_PLANET})");
    return array(
        'attacker' => summarize_planet(LoadPlanetById($ATTACKER_PLANET)),
        'defender' => summarize_planet(LoadPlanetById($DEFENDER_PLANET)),
    );
}

$result = array('cases' => array());

$case = run_missile_case(
    'abm_fully_intercepts_ipm',
    3,
    GID_D_RL,
    array(GID_D_IPM => 3),
    array(GID_D_ABM => 5, GID_D_RL => 10, GID_D_LL => 5),
    array(109 => 0, 111 => 0),
    array(109 => 0, 111 => 0)
);
$result['cases'][] = with_checks($case, array(
    assert_case($case['fleet_id'] > 0, 'missile fleet is launched'),
    assert_case($case['origin_after_launch']['ipm'] === 0, 'origin IPMs are consumed at launch', array('origin_after_launch' => $case['origin_after_launch'])),
    assert_case($case['after']['abm'] === 2, 'ABMs intercept all incoming IPMs and decrease by 3', array('after' => $case['after'])),
    assert_case($case['after']['rocket_launcher'] === 10 && $case['after']['light_laser'] === 5, 'defense remains unchanged when all IPMs are intercepted', array('after' => $case['after'])),
    assert_case(count($case['attacker_messages']) >= 1 && count($case['defender_messages']) >= 1, 'missile reports are generated for both players'),
    assert_case($case['fleet_count_after'] === 0 && $case['queue_count_after'] === 0, 'missile fleet and queue are removed after arrival'),
));

$case = run_missile_case(
    'partial_intercept_targeted_rocket_launcher_damage',
    3,
    GID_D_RL,
    array(GID_D_IPM => 3),
    array(GID_D_ABM => 2, GID_D_RL => 100, GID_D_LL => 10),
    array(109 => 0, 111 => 0),
    array(109 => 0, 111 => 0)
);
$result['cases'][] = with_checks($case, array(
    assert_case($case['after']['abm'] === 0, 'two ABMs intercept two of three IPMs', array('after' => $case['after'])),
    assert_case($case['after']['rocket_launcher'] === 40, 'one remaining IPM destroys 60 targeted rocket launchers', array('before' => $case['before'], 'after' => $case['after'])),
    assert_case($case['after']['light_laser'] === 10, 'non-target defense remains unchanged when damage is exhausted by primary target', array('after' => $case['after'])),
    assert_case($case['origin_after']['ipm'] === 0, 'origin IPMs remain consumed after impact', array('origin_after' => $case['origin_after'])),
    assert_case($case['fleet_count_after'] === 0 && $case['queue_count_after'] === 0, 'missile fleet and queue are removed after arrival'),
));

$case = run_missile_case(
    'targeted_plasma_damage',
    1,
    GID_D_PLASMA,
    array(GID_D_IPM => 1),
    array(GID_D_ABM => 0, GID_D_PLASMA => 3),
    array(109 => 0, 111 => 0),
    array(109 => 0, 111 => 0)
);
$result['cases'][] = with_checks($case, array(
    assert_case($case['after']['plasma'] === 2, 'one IPM destroys one targeted plasma turret', array('before' => $case['before'], 'after' => $case['after'])),
    assert_case($case['after']['abm'] === 0, 'no ABM interception occurs when target has none', array('after' => $case['after'])),
    assert_case(count($case['attacker_messages']) >= 1 && count($case['defender_messages']) >= 1, 'missile reports are generated for targeted plasma strike'),
));

$case = run_missile_case(
    'no_primary_target_sweeps_defense_order',
    1,
    0,
    array(GID_D_IPM => 1),
    array(GID_D_ABM => 0, GID_D_RL => 20, GID_D_LL => 20),
    array(109 => 0, 111 => 0),
    array(109 => 0, 111 => 0)
);
$result['cases'][] = with_checks($case, array(
    assert_case($case['after']['rocket_launcher'] === 0, 'no-primary IPM destroys rocket launchers in defense order', array('before' => $case['before'], 'after' => $case['after'])),
    assert_case($case['after']['light_laser'] === 0, 'remaining no-primary IPM damage destroys light lasers', array('before' => $case['before'], 'after' => $case['after'])),
    assert_case($case['fleet_count_after'] === 0 && $case['queue_count_after'] === 0, 'missile fleet and queue are removed after no-primary attack'),
));

cleanup_runtime();
set_user_tech($ATTACKER_ID, array(109 => 0, 111 => 0));
set_user_tech($DEFENDER_ID, array(109 => 0, 111 => 0));
set_planet_defense_state($ATTACKER_PLANET, 'MissileAttacker', array(GID_D_IPM => 1));
set_planet_defense_state($DEFENDER_PLANET, 'MissileDefender', array(GID_D_RL => 10));
$before = LoadPlanetById($ATTACKER_PLANET);
$fleetId = LaunchRockets($before, LoadPlanetById($DEFENDER_PLANET), 1, 2, GID_D_RL);
$after = LoadPlanetById($ATTACKER_PLANET);
$result['cases'][] = array(
    'case' => 'cannot_launch_more_ipm_than_available',
    'fleet_id' => $fleetId,
    'before' => summarize_planet($before),
    'after' => summarize_planet($after),
    'checks' => array(
        assert_case($fleetId === 0, 'launch is rejected when requested IPMs exceed available IPMs'),
        assert_case(intval($after[GID_D_IPM]) === 1, 'origin IPM count is unchanged after rejected launch', array('after' => summarize_planet($after))),
        assert_case(current_fleet_count() === 0 && current_queue_count() === 0, 'no missile fleet or queue is created after rejected launch'),
    ),
);

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
