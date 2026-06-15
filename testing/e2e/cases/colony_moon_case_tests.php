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
loca_add('graviton', 'en');

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
    sql_exec("DELETE FROM {$db_prefix}planets WHERE g=1 AND s BETWEEN 430 AND 499 AND owner_id IN ({$users}," . USER_SPACE . ")");
}

function set_user_tech(int $userId, array $tech): void
{
    global $db_prefix, $resmap;
    $parts = array();
    foreach ($resmap as $gid) $parts[] = "`{$gid}`=" . intval($tech[$gid] ?? 0);
    sql_exec("UPDATE {$db_prefix}users SET " . implode(',', $parts) . ", validated=1, deact_ip=1, lang='en', skin='/evolution/', useskin=1, adm_until=0 WHERE player_id={$userId}");
    InvalidateUserCache();
}

function set_planet_state(int $planetId, string $name, array $ships, array $defense, array $buildings = array(), array $resources = array()): void
{
    global $db_prefix, $fleetmap, $defmap, $rakmap, $buildmap;
    SetPlanetDefense($planetId, with_units(zero_map($defmap), $defense));
    SetPlanetFleetDefense($planetId, with_units(zero_map(array_diff($defmap, $rakmap)) + zero_map($fleetmap), $ships + $defense));
    SetPlanetBuildings($planetId, with_units(zero_map($buildmap), $buildings));
    sql_exec(
        "UPDATE {$db_prefix}planets SET name='{$name}', " .
        "`700`=" . floatval($resources[700] ?? 50000000) . ", " .
        "`701`=" . floatval($resources[701] ?? 50000000) . ", " .
        "`702`=" . floatval($resources[702] ?? 50000000) . ", " .
        "lastpeek=" . time() . " WHERE planet_id={$planetId}"
    );
}

function find_free_position(int $startSystem = 430): array
{
    global $db_prefix;
    for ($s = $startSystem; $s <= 499; $s++) {
        for ($p = 1; $p <= 15; $p++) {
            $cnt = intval(sql_value("SELECT COUNT(*) FROM {$db_prefix}planets WHERE g=1 AND s={$s} AND p={$p} AND type IN (" . PTYP_PLANET . "," . PTYP_DEST_PLANET . "," . PTYP_ABANDONED . "," . PTYP_COLONY_PHANTOM . "," . PTYP_MOON . "," . PTYP_DEST_MOON . ")"));
            if ($cnt === 0) return array(1, $s, $p);
        }
    }
    throw new RuntimeException('No free test coordinate found');
}

function launch_fleet(int $mission, int $originPlanetId, int $targetPlanetId, array $ships, int $when): array
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

function latest_fleet(int $ownerId, int $mission): ?array
{
    global $db_prefix;
    return sql_row("SELECT * FROM {$db_prefix}fleet WHERE owner_id={$ownerId} AND mission={$mission} ORDER BY fleet_id DESC LIMIT 1");
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
        $subj = $row['subj'] ?? '';
        $plain = trim(preg_replace('/\s+/', ' ', html_entity_decode(strip_tags($subj . ' ' . $text), ENT_QUOTES | ENT_HTML5, 'UTF-8')));
        $rows[] = array(
            'msg_id' => intval($row['msg_id']),
            'owner_id' => intval($row['owner_id']),
            'pm' => intval($row['pm']),
            'text_len' => strlen($text),
            'preview' => mb_substr($plain, 0, 260, 'UTF-8'),
            'flags' => array(
                'colony_success' => str_contains($text, 'finds a new planet'),
                'colony_fail' => str_contains($text, 'finds no planet suitable'),
                'colony_max' => str_contains($text, 'empire becomes too large'),
                'moon_created' => str_contains($text, 'form a moon'),
                'moon_attack' => str_contains($subj, 'Moon attack') || str_contains($subj, 'Moon quakes'),
                'moon_destroyed' => str_contains($text, 'destroy the satellite') || str_contains($text, 'eventually explodes'),
            ),
        );
    }
    return $rows;
}

function assert_case(bool $condition, string $message, array $context = array()): array
{
    return array('pass' => $condition, 'message' => $message, 'context' => $context);
}

function fleet_slots_for(int $userId): array
{
    global $ATTACKER_PLANET;
    $user = LoadUser($userId);
    $planet = LoadPlanetById($ATTACKER_PLANET);
    $max = $maxNoBonus = 0;
    GetMaxFleet($user, $planet, $max, $maxNoBonus);
    $now = dbrows(EnumOwnFleetQueue($userId));
    $nowWithIpm = dbrows(EnumOwnFleetQueue($userId, 1));
    return array('computer' => intval($user[GID_R_COMPUTER]), 'max' => $max, 'max_no_bonus' => $maxNoBonus, 'nowfleet' => $now, 'nowfleet_with_ipm' => $nowWithIpm, 'blocked' => $now >= $max);
}

function add_dummy_fleet(int $ownerId, int $mission): int
{
    global $ATTACKER_PLANET, $DEFENDER_PLANET;
    $fleet = array('owner_id' => $ownerId, 'union_id' => 0, 'fuel' => 0, 'mission' => $mission, 'start_planet' => $ATTACKER_PLANET, 'target_planet' => $DEFENDER_PLANET, 'flight_time' => 3600, 'deploy_time' => 0, GID_F_SC => $mission == FTYP_MISSILE ? 0 : 1, 'ipm_amount' => $mission == FTYP_MISSILE ? 1 : 0, 'ipm_target' => 0);
    $fleetId = AddDBRow($fleet, 'fleet');
    AddQueue($ownerId, QTYP_FLEET, $fleetId, 0, 0, time(), 3600, QUEUE_PRIO_FLEET + $mission);
    return $fleetId;
}

function run_fleet_slot_cases(): array
{
    global $ATTACKER_ID;
    cleanup_runtime();
    set_user_tech($ATTACKER_ID, array(GID_R_COMPUTER => 0));
    $level0Start = fleet_slots_for($ATTACKER_ID);
    add_dummy_fleet($ATTACKER_ID, FTYP_TRANSPORT);
    $level0OneNormal = fleet_slots_for($ATTACKER_ID);
    add_dummy_fleet($ATTACKER_ID, FTYP_MISSILE);
    $level0WithIpm = fleet_slots_for($ATTACKER_ID);

    cleanup_runtime();
    set_user_tech($ATTACKER_ID, array(GID_R_COMPUTER => 2));
    $level2Start = fleet_slots_for($ATTACKER_ID);
    add_dummy_fleet($ATTACKER_ID, FTYP_TRANSPORT);
    add_dummy_fleet($ATTACKER_ID, FTYP_TRANSPORT);
    $level2TwoNormal = fleet_slots_for($ATTACKER_ID);
    add_dummy_fleet($ATTACKER_ID, FTYP_TRANSPORT);
    $level2ThreeNormal = fleet_slots_for($ATTACKER_ID);

    return array(
        'case' => 'computer_technology_fleet_slots',
        'states' => array(
            'level0_start' => $level0Start,
            'level0_one_normal' => $level0OneNormal,
            'level0_with_ipm' => $level0WithIpm,
            'level2_start' => $level2Start,
            'level2_two_normal' => $level2TwoNormal,
            'level2_three_normal' => $level2ThreeNormal,
        ),
        'checks' => array(
            assert_case($level0Start['max'] === 1 && !$level0Start['blocked'], 'Computer 0 gives 1 fleet slot and starts unblocked', $level0Start),
            assert_case($level0OneNormal['nowfleet'] === 1 && $level0OneNormal['blocked'], 'Computer 0 blocks after 1 normal fleet', $level0OneNormal),
            assert_case($level0WithIpm['nowfleet'] === 1 && $level0WithIpm['nowfleet_with_ipm'] === 2, 'IPM queues are excluded from normal fleet slot count', $level0WithIpm),
            assert_case($level2Start['max'] === 3 && !$level2Start['blocked'], 'Computer 2 gives 3 fleet slots', $level2Start),
            assert_case($level2TwoNormal['nowfleet'] === 2 && !$level2TwoNormal['blocked'], 'Computer 2 allows 2 active normal fleets', $level2TwoNormal),
            assert_case($level2ThreeNormal['nowfleet'] === 3 && $level2ThreeNormal['blocked'], 'Computer 2 blocks at 3 active normal fleets', $level2ThreeNormal),
        ),
    );
}

function run_colonization_success_case(): array
{
    global $ATTACKER_ID, $ATTACKER_PLANET, $db_prefix;
    cleanup_runtime();
    set_user_tech($ATTACKER_ID, array(GID_R_COMPUTER => 10, GID_R_IMPULSE_DRIVE => 3));
    set_planet_state($ATTACKER_PLANET, 'Homeplanet', array(GID_F_COLON => 1), array(), array(), array());
    [$g, $s, $p] = find_free_position(430);
    $phantomId = CreateColonyPhantom($g, $s, $p, USER_SPACE);
    $afterMsg = max_msg_id();
    [$fleetId, $queue] = launch_fleet(FTYP_COLONIZE, $ATTACKER_PLANET, $phantomId, array(GID_F_COLON => 1), time() + 1);
    Queue_Fleet_End($queue);
    $newPlanet = sql_row("SELECT planet_id, owner_id, type, g, s, p, `700`, `701`, `702` FROM {$db_prefix}planets WHERE g={$g} AND s={$s} AND p={$p} AND type=" . PTYP_PLANET . " LIMIT 1");
    $phantomLeft = sql_row("SELECT planet_id FROM {$db_prefix}planets WHERE planet_id={$phantomId}");
    $origin = LoadPlanetById($ATTACKER_PLANET);
    $msgs = message_rows($ATTACKER_ID, $afterMsg);
    return array(
        'case' => 'colony_ship_success',
        'coordinate' => array($g, $s, $p),
        'new_planet' => $newPlanet,
        'origin' => array('colony_ship' => intval($origin[GID_F_COLON])),
        'messages' => $msgs,
        'checks' => array(
            assert_case($newPlanet !== null && intval($newPlanet['owner_id']) === $ATTACKER_ID && intval($newPlanet['type']) === PTYP_PLANET, 'new owned planet is created at target coordinate', $newPlanet ?? array()),
            assert_case($phantomLeft === null, 'colonization phantom is removed after successful colonization'),
            assert_case(intval($origin[GID_F_COLON]) === 0, 'colony ship is consumed on successful colonization', array('origin_208' => $origin[GID_F_COLON])),
            assert_case(count(array_filter($msgs, fn($m) => $m['flags']['colony_success'])) >= 1, 'success settler report is generated'),
        ),
    );
}

function run_colonization_occupied_case(): array
{
    global $ATTACKER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET;
    cleanup_runtime();
    set_user_tech($ATTACKER_ID, array(GID_R_COMPUTER => 10, GID_R_IMPULSE_DRIVE => 3));
    set_planet_state($ATTACKER_PLANET, 'Homeplanet', array(GID_F_COLON => 1), array(), array(), array());
    $afterMsg = max_msg_id();
    [$fleetId, $queue] = launch_fleet(FTYP_COLONIZE, $ATTACKER_PLANET, $DEFENDER_PLANET, array(GID_F_COLON => 1), time() + 1);
    Queue_Fleet_End($queue);
    $returnFleet = latest_fleet($ATTACKER_ID, FTYP_COLONIZE + FTYP_RETURN);
    if ($returnFleet) {
        $returnQueue = GetFleetQueue(intval($returnFleet['fleet_id']));
        Queue_Fleet_End($returnQueue);
    }
    $origin = LoadPlanetById($ATTACKER_PLANET);
    $msgs = message_rows($ATTACKER_ID, $afterMsg);
    return array(
        'case' => 'colony_ship_occupied_target_fails_and_returns',
        'return_fleet' => $returnFleet ? array('fleet_id' => intval($returnFleet['fleet_id']), 'colony_ship' => intval($returnFleet[GID_F_COLON])) : null,
        'origin' => array('colony_ship' => intval($origin[GID_F_COLON])),
        'messages' => $msgs,
        'checks' => array(
            assert_case($returnFleet !== null && intval($returnFleet[GID_F_COLON]) === 1, 'colony ship enters return fleet when target is occupied', $returnFleet ?? array()),
            assert_case(intval($origin[GID_F_COLON]) === 1, 'colony ship returns to origin after failed colonization', array('origin_208' => $origin[GID_F_COLON])),
            assert_case(count(array_filter($msgs, fn($m) => $m['flags']['colony_fail'])) >= 1, 'failure settler report is generated'),
        ),
    );
}

function run_colonization_max_planets_case(): array
{
    global $ATTACKER_ID, $ATTACKER_PLANET, $db_prefix;
    cleanup_runtime();
    set_user_tech($ATTACKER_ID, array(GID_R_COMPUTER => 10, GID_R_IMPULSE_DRIVE => 3));
    set_planet_state($ATTACKER_PLANET, 'Homeplanet', array(GID_F_COLON => 1), array(), array(), array());
    $created = array();
    while (intval(sql_value("SELECT COUNT(*) FROM {$db_prefix}planets WHERE owner_id={$ATTACKER_ID} AND type=" . PTYP_PLANET)) < MAX_PLANET) {
        [$g, $s, $p] = find_free_position(440);
        $created[] = CreatePlanet($g, $s, $p, $ATTACKER_ID, 1, 0, 0, time());
    }
    [$g, $s, $p] = find_free_position(460);
    $phantomId = CreateColonyPhantom($g, $s, $p, USER_SPACE);
    $afterMsg = max_msg_id();
    [$fleetId, $queue] = launch_fleet(FTYP_COLONIZE, $ATTACKER_PLANET, $phantomId, array(GID_F_COLON => 1), time() + 1);
    Queue_Fleet_End($queue);
    $abandoned = sql_row("SELECT planet_id, owner_id, type, remove FROM {$db_prefix}planets WHERE g={$g} AND s={$s} AND p={$p} AND type=" . PTYP_ABANDONED . " LIMIT 1");
    $returnFleet = latest_fleet($ATTACKER_ID, FTYP_COLONIZE + FTYP_RETURN);
    if ($returnFleet) {
        $returnQueue = GetFleetQueue(intval($returnFleet['fleet_id']));
        Queue_Fleet_End($returnQueue);
    }
    $origin = LoadPlanetById($ATTACKER_PLANET);
    $msgs = message_rows($ATTACKER_ID, $afterMsg);
    return array(
        'case' => 'colony_ship_max_planets_creates_abandoned_and_returns',
        'created_planets_for_limit' => count($created),
        'abandoned' => $abandoned,
        'return_fleet' => $returnFleet ? array('fleet_id' => intval($returnFleet['fleet_id']), 'colony_ship' => intval($returnFleet[GID_F_COLON])) : null,
        'origin' => array('colony_ship' => intval($origin[GID_F_COLON])),
        'checks' => array(
            assert_case($abandoned !== null && intval($abandoned['owner_id']) === USER_SPACE && intval($abandoned['type']) === PTYP_ABANDONED, 'max-planet colonization creates an abandoned colony placeholder', $abandoned ?? array()),
            assert_case($returnFleet !== null && intval($returnFleet[GID_F_COLON]) === 1, 'colony ship returns when empire is at max planet count', $returnFleet ?? array()),
            assert_case(intval($origin[GID_F_COLON]) === 1, 'colony ship is back on origin after max-planet failure', array('origin_208' => $origin[GID_F_COLON])),
            assert_case(count(array_filter($msgs, fn($m) => $m['flags']['colony_max'])) >= 1, 'max-planet settler report is generated'),
        ),
    );
}

function run_moon_creation_case(): array
{
    global $ATTACKER_ID, $DEFENDER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET, $db_prefix;
    cleanup_runtime();
    set_user_tech($ATTACKER_ID, array(GID_R_COMPUTER => 10, GID_R_WEAPON => 10, GID_R_SHIELD => 10, GID_R_ARMOUR => 10));
    set_user_tech($DEFENDER_ID, array(GID_R_COMPUTER => 10, GID_R_WEAPON => 0, GID_R_SHIELD => 0, GID_R_ARMOUR => 0));
    $afterMsg = max_msg_id();
    $moonId = 0;
    $attempts = 0;
    for ($attempt = 1; $attempt <= 40; $attempt++) {
        $attempts = $attempt;
        $target = LoadPlanetById($DEFENDER_PLANET);
        sql_exec("DELETE FROM {$db_prefix}planets WHERE g=" . intval($target['g']) . " AND s=" . intval($target['s']) . " AND p=" . intval($target['p']) . " AND type IN (" . PTYP_MOON . "," . PTYP_DF . ")");
        sql_exec("DELETE FROM {$db_prefix}fleet WHERE owner_id IN ({$ATTACKER_ID},{$DEFENDER_ID})");
        sql_exec("DELETE FROM {$db_prefix}queue WHERE owner_id IN ({$ATTACKER_ID},{$DEFENDER_ID}) AND type='" . QTYP_FLEET . "'");
        mt_srand($attempt * 17);
        srand($attempt * 17);
        set_planet_state($ATTACKER_PLANET, 'Homeplanet', array(GID_F_DEATHSTAR => 10), array(), array(), array());
        set_planet_state($DEFENDER_PLANET, 'MoonTarget', array(GID_F_LF => 3000), array(), array(), array(700 => 0, 701 => 0, 702 => 0));
        [$fleetId, $queue] = launch_fleet(FTYP_ATTACK, $ATTACKER_PLANET, $DEFENDER_PLANET, array(GID_F_DEATHSTAR => 10), time() + 1);
        Queue_Fleet_End($queue);
        $moonId = PlanetHasMoon($DEFENDER_PLANET);
        if ($moonId) break;
    }
    $moon = $moonId ? LoadPlanetById($moonId) : null;
    $target = LoadPlanetById($DEFENDER_PLANET);
    $debris = sql_row("SELECT planet_id, `700`, `701` FROM {$db_prefix}planets WHERE g=" . intval($target['g']) . " AND s=" . intval($target['s']) . " AND p=" . intval($target['p']) . " AND type=" . PTYP_DF . " LIMIT 1");
    $attMsgs = message_rows($ATTACKER_ID, $afterMsg);
    $defMsgs = message_rows($DEFENDER_ID, $afterMsg);
    return array(
        'case' => 'battle_moon_creation',
        'attempts' => $attempts,
        'moon' => $moon ? array('planet_id' => intval($moon['planet_id']), 'type' => intval($moon['type']), 'owner_id' => intval($moon['owner_id']), 'diameter' => intval($moon['diameter'])) : null,
        'debris' => $debris,
        'attacker_messages' => array_slice($attMsgs, -4),
        'defender_messages' => array_slice($defMsgs, -4),
        'checks' => array(
            assert_case($moon !== null && intval($moon['type']) === PTYP_MOON && intval($moon['owner_id']) === $DEFENDER_ID, 'moon is created for defender by battle moon chance', $moon ? array('moon_id' => $moon['planet_id'], 'diameter' => $moon['diameter']) : array()),
            assert_case($debris !== null && (floatval($debris[700]) + floatval($debris[701])) >= 2000000, 'battle generated enough debris for max moon chance', $debris ?? array()),
            assert_case(count(array_filter(array_merge($attMsgs, $defMsgs), fn($m) => $m['flags']['moon_created'])) >= 1, 'battle report text mentions moon creation'),
        ),
    );
}

function run_moon_destruction_case(): array
{
    global $ATTACKER_ID, $DEFENDER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET, $db_prefix;
    cleanup_runtime();
    set_user_tech($ATTACKER_ID, array(GID_R_COMPUTER => 10, GID_R_WEAPON => 10, GID_R_SHIELD => 10, GID_R_ARMOUR => 10, GID_R_GRAVITON => 1));
    set_user_tech($DEFENDER_ID, array(GID_R_COMPUTER => 10, GID_R_WEAPON => 0, GID_R_SHIELD => 0, GID_R_ARMOUR => 0));
    $moonDestroyed = false;
    $moonId = 0;
    $afterMsg = max_msg_id();
    for ($attempt = 1; $attempt <= 5; $attempt++) {
        $target = LoadPlanetById($DEFENDER_PLANET);
        sql_exec("DELETE FROM {$db_prefix}planets WHERE g=" . intval($target['g']) . " AND s=" . intval($target['s']) . " AND p=" . intval($target['p']) . " AND type IN (" . PTYP_MOON . "," . PTYP_DF . ")");
        $moonId = CreatePlanet(intval($target['g']), intval($target['s']), intval($target['p']), $DEFENDER_ID, 0, 1, 20, time());
        sql_exec("UPDATE {$db_prefix}planets SET diameter=1000 WHERE planet_id={$moonId}");
        mt_srand(1000 + $attempt);
        srand(1000 + $attempt);
        set_planet_state($ATTACKER_PLANET, 'Homeplanet', array(GID_F_DEATHSTAR => 10), array(), array(), array());
        $moonBefore = LoadPlanetById($moonId);
        [$fleetId, $queue] = launch_fleet(FTYP_DESTROY, $ATTACKER_PLANET, $moonId, array(GID_F_DEATHSTAR => 10), time() + 1);
        Queue_Fleet_End($queue);
        $moonAfter = LoadPlanetById($moonId);
        if ($moonAfter === null) {
            $moonDestroyed = true;
            break;
        }
    }
    $attMsgs = message_rows($ATTACKER_ID, $afterMsg);
    $defMsgs = message_rows($DEFENDER_ID, $afterMsg);
    return array(
        'case' => 'deathstar_moon_destruction',
        'moon_id' => $moonId,
        'planet_has_moon_after' => PlanetHasMoon($DEFENDER_PLANET),
        'attacker_messages' => array_slice($attMsgs, -4),
        'defender_messages' => array_slice($defMsgs, -4),
        'checks' => array(
            assert_case($moonDestroyed && PlanetHasMoon($DEFENDER_PLANET) === 0, 'Deathstar destroy mission removes the moon row', array('moon_id' => $moonId)),
            assert_case(count(array_filter($attMsgs, fn($m) => $m['flags']['moon_attack'])) >= 1 && count(array_filter($defMsgs, fn($m) => $m['flags']['moon_attack'])) >= 1, 'moon attack messages are generated for attacker and defender'),
            assert_case(count(array_filter(array_merge($attMsgs, $defMsgs), fn($m) => $m['flags']['moon_destroyed'])) >= 1, 'moon destruction message text confirms satellite destruction'),
        ),
    );
}

function restore_test_accounts(): array
{
    global $db_prefix, $ATTACKER_ID, $DEFENDER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET;
    cleanup_runtime();
    set_user_tech($DEFENDER_ID, array(GID_R_ESPIONAGE => 10, GID_R_COMPUTER => 10, GID_R_WEAPON => 10, GID_R_SHIELD => 10, GID_R_ARMOUR => 10, GID_R_COMBUST_DRIVE => 10, GID_R_IMPULSE_DRIVE => 10, GID_R_HYPER_DRIVE => 10));
    set_user_tech($ATTACKER_ID, array(GID_R_ESPIONAGE => 10, GID_R_COMPUTER => 10, GID_R_WEAPON => 10, GID_R_SHIELD => 10, GID_R_ARMOUR => 10, GID_R_COMBUST_DRIVE => 10, GID_R_IMPULSE_DRIVE => 10, GID_R_HYPER_DRIVE => 10));
    set_planet_state($ATTACKER_PLANET, 'Homeplanet', array(GID_F_SC => 20, GID_F_LF => 5, GID_F_PROBE => 5), array(GID_D_RL => 10, GID_D_LL => 10), array(), array(700 => 50000000, 701 => 50000000, 702 => 50000000));
    set_planet_state($DEFENDER_PLANET, 'FlowP100161', array(GID_F_SC => 20, GID_F_LF => 5, GID_F_PROBE => 5), array(GID_D_RL => 10, GID_D_LL => 10), array(), array(700 => 50000000, 701 => 50000000, 702 => 50000000));
    return array(
        'attacker' => sql_row("SELECT planet_id, name, `700`, `701`, `702`, `202`, `204`, `210`, `401`, `402` FROM {$db_prefix}planets WHERE planet_id={$ATTACKER_PLANET}"),
        'defender' => sql_row("SELECT planet_id, name, `700`, `701`, `702`, `202`, `204`, `210`, `401`, `402` FROM {$db_prefix}planets WHERE planet_id={$DEFENDER_PLANET}"),
    );
}

$result = array('cases' => array());
$result['cases'][] = run_fleet_slot_cases();
$result['cases'][] = run_colonization_success_case();
$result['cases'][] = run_colonization_occupied_case();
$result['cases'][] = run_colonization_max_planets_case();
$result['cases'][] = run_moon_creation_case();
$result['cases'][] = run_moon_destruction_case();
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
