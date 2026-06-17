<?php

ob_start();
error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = $_SERVER['HTTP_USER_AGENT'] ?? 'ogame-e2e';
$_SERVER['REQUEST_URI'] = $_SERVER['REQUEST_URI'] ?? '/testing/e2e';
$_SERVER['HTTP_USER_AGENT'] = 'codex-cli';
$_SERVER['REQUEST_URI'] = '/tmp/ogame_moon_edge_tests.php';
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

function run_sql(string $sql): mixed
{
    $res = dbquery($sql);
    if ($res === false) throw new RuntimeException('SQL failed: ' . $sql);
    return $res;
}

function first_value(string $sql): mixed
{
    $res = run_sql($sql);
    $row = dbarray($res);
    if ($row === false) return null;
    return array_values($row)[0];
}

function zero_units(array $map): array
{
    $out = array();
    foreach ($map as $gid) $out[$gid] = 0;
    return $out;
}

function merge_units(array $base, array $units): array
{
    foreach ($units as $gid => $amount) $base[$gid] = $amount;
    return $base;
}

function cleanup_moon_edges(): void
{
    global $db_prefix, $ATTACKER_ID, $DEFENDER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET;
    $users = "{$ATTACKER_ID},{$DEFENDER_ID}";
    $planets = "{$ATTACKER_PLANET},{$DEFENDER_PLANET}";
    run_sql("DELETE FROM {$db_prefix}queue WHERE owner_id IN ({$users}) OR (type='" . QTYP_FLEET . "' AND sub_id IN (SELECT fleet_id FROM {$db_prefix}fleet WHERE owner_id IN ({$users}) OR start_planet IN ({$planets}) OR target_planet IN ({$planets})))");
    run_sql("DELETE FROM {$db_prefix}fleet WHERE owner_id IN ({$users}) OR start_planet IN ({$planets}) OR target_planet IN ({$planets})");
    run_sql("DELETE FROM {$db_prefix}buildqueue WHERE owner_id IN ({$users})");
    $target = LoadPlanetById($DEFENDER_PLANET);
    if ($target !== null) {
        run_sql("DELETE FROM {$db_prefix}planets WHERE type IN (" . PTYP_MOON . "," . PTYP_DEST_MOON . "," . PTYP_DF . ") AND g=" . intval($target['g']) . " AND s=" . intval($target['s']) . " AND p=" . intval($target['p']));
    }
}

function set_user_research(int $userId): void
{
    global $db_prefix, $resmap;
    $parts = array();
    foreach ($resmap as $gid) $parts[] = "`{$gid}`=10";
    run_sql("UPDATE {$db_prefix}users SET " . implode(',', $parts) . ", validated=1, deact_ip=1, lang='en', skin='/evolution/', useskin=1, adm_until=0 WHERE player_id={$userId}");
    InvalidateUserCache();
}

function set_planet_units(int $planetId, string $name, array $ships, array $defense = array()): void
{
    global $fleetmap, $defmap, $rakmap, $buildmap, $db_prefix;
    SetPlanetDefense($planetId, merge_units(zero_units($defmap), $defense));
    SetPlanetFleetDefense($planetId, merge_units(zero_units(array_diff($defmap, $rakmap)) + zero_units($fleetmap), $ships + $defense));
    SetPlanetBuildings($planetId, zero_units($buildmap));
    run_sql("UPDATE {$db_prefix}planets SET name='{$name}', `700`=50000000, `701`=50000000, `702`=50000000, lastpeek=" . time() . " WHERE planet_id={$planetId}");
}

function max_message_id(): int
{
    global $db_prefix;
    return intval(first_value("SELECT COALESCE(MAX(msg_id), 0) FROM {$db_prefix}messages"));
}

function message_summary(int $ownerId, int $after): array
{
    global $db_prefix;
    $rows = array();
    $res = run_sql("SELECT msg_id, owner_id, pm, subj, text FROM {$db_prefix}messages WHERE owner_id={$ownerId} AND msg_id>{$after} ORDER BY msg_id ASC");
    while ($row = dbarray($res)) {
        $text = $row['text'] ?? '';
        $subj = $row['subj'] ?? '';
        $plain = trim(preg_replace('/\s+/', ' ', html_entity_decode(strip_tags($subj . ' ' . $text), ENT_QUOTES | ENT_HTML5, 'UTF-8')));
        $normalized = stripslashes($plain . ' ' . $text);
        $rows[] = array(
            'msg_id' => intval($row['msg_id']),
            'pm' => intval($row['pm']),
            'preview' => mb_substr($plain, 0, 220, 'UTF-8'),
            'moon_notice' => str_contains($subj, 'Moon attack') || str_contains($subj, 'Moon quakes'),
            'destroy_success' => str_contains($text, 'destroy the satellite') || str_contains($text, 'eventually explodes'),
            'destroy_failed' => str_contains($normalized, "wasn't weakened enough") || str_contains($normalized, 'failed attack') || str_contains($normalized, 'explodes into millions of pieces'),
        );
    }
    return $rows;
}

function launch_to_moon(int $mission, int $targetMoonId, array $ships): void
{
    global $ATTACKER_PLANET, $fleetmap, $transportableResources;
    $fleet = merge_units(zero_units($fleetmap), $ships);
    $resources = zero_units($transportableResources);
    $origin = LoadPlanetById($ATTACKER_PLANET);
    $target = LoadPlanetById($targetMoonId);
    AdjustShips($fleet, $ATTACKER_PLANET, '-');
    $fleetId = DispatchFleet($fleet, $origin, $target, $mission, 1, $resources, 0, time() + 1);
    $queue = GetFleetQueue($fleetId);
    Queue_Fleet_End($queue);
}

function dispatch_between(int $mission, int $originPlanetId, int $targetPlanetId, array $ships, int $when, array $resources = array()): array
{
    global $fleetmap, $transportableResources;
    $fleet = merge_units(zero_units($fleetmap), $ships);
    $cargo = zero_units($transportableResources);
    foreach ($resources as $gid => $amount) {
        $cargo[$gid] = $amount;
    }
    $origin = LoadPlanetById($originPlanetId);
    $target = LoadPlanetById($targetPlanetId);
    AdjustShips($fleet, $originPlanetId, '-');
    AdjustResources($cargo, $originPlanetId, '-');
    $fleetId = DispatchFleet($fleet, $origin, $target, $mission, 3600, $cargo, 0, $when);
    return array($fleetId, GetFleetQueue($fleetId));
}

function fleet_row(int $fleetId): ?array
{
    global $db_prefix;
    $res = run_sql("SELECT fleet_id, owner_id, mission, start_planet, target_planet, `" . GID_F_SC . "` AS small_cargo, `" . GID_RC_METAL . "` AS metal FROM {$db_prefix}fleet WHERE fleet_id={$fleetId} LIMIT 1");
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function latest_owner_fleet(int $ownerId, int $mission): ?array
{
    global $db_prefix;
    $res = run_sql("SELECT fleet_id, owner_id, mission, start_planet, target_planet, `" . GID_F_SC . "` AS small_cargo, `" . GID_RC_METAL . "` AS metal FROM {$db_prefix}fleet WHERE owner_id={$ownerId} AND mission={$mission} ORDER BY fleet_id DESC LIMIT 1");
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function planet_snapshot(int $planetId): ?array
{
    global $db_prefix;
    $res = run_sql("SELECT planet_id, owner_id, type, g, s, p, `" . GID_F_SC . "` AS small_cargo, `" . GID_RC_METAL . "` AS metal FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1");
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function create_test_moon(int $diameter): int
{
    global $DEFENDER_ID, $DEFENDER_PLANET, $db_prefix;
    $target = LoadPlanetById($DEFENDER_PLANET);
    $moonId = CreatePlanet(intval($target['g']), intval($target['s']), intval($target['p']), $DEFENDER_ID, 1, 1, 20);
    run_sql("UPDATE {$db_prefix}planets SET diameter={$diameter}, name='EdgeMoon' WHERE planet_id={$moonId}");
    return $moonId;
}

function restore_accounts(): array
{
    global $ATTACKER_ID, $DEFENDER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET, $db_prefix;
    cleanup_moon_edges();
    set_user_research($ATTACKER_ID);
    set_user_research($DEFENDER_ID);
    set_planet_units($ATTACKER_PLANET, 'Homeplanet', array(GID_F_SC => 20, GID_F_LF => 5, GID_F_PROBE => 5), array(GID_D_RL => 10, GID_D_LL => 10));
    set_planet_units($DEFENDER_PLANET, 'FlowP100161', array(GID_F_SC => 20, GID_F_LF => 5, GID_F_PROBE => 5), array(GID_D_RL => 10, GID_D_LL => 10));
    $queueCount = intval(first_value("SELECT COUNT(*) FROM {$db_prefix}queue WHERE owner_id IN ({$ATTACKER_ID},{$DEFENDER_ID})"));
    $fleetCount = intval(first_value("SELECT COUNT(*) FROM {$db_prefix}fleet WHERE owner_id IN ({$ATTACKER_ID},{$DEFENDER_ID})"));
    return array('queue_count' => $queueCount, 'fleet_count' => $fleetCount);
}

$cases = array();

try {
    cleanup_moon_edges();
    set_user_research($ATTACKER_ID);
    set_user_research($DEFENDER_ID);

    set_planet_units($ATTACKER_PLANET, 'Homeplanet', array(GID_F_DEATHSTAR => 1));
    set_planet_units($DEFENDER_PLANET, 'FlowP100161', array(), array());
    $moonId = create_test_moon(10000);
    $after = max_message_id();
    launch_to_moon(FTYP_DESTROY, $moonId, array(GID_F_DEATHSTAR => 1));
    $moonStillExists = PlanetHasMoon($DEFENDER_PLANET) === $moonId;
    $attackerMessages = message_summary($ATTACKER_ID, $after);
    $defenderMessages = message_summary($DEFENDER_ID, $after);
    $cases[] = array(
        'case' => 'moon_destruction_zero_percent_fails',
        'moon_id' => $moonId,
        'moon_still_exists' => $moonStillExists,
        'attacker_messages' => $attackerMessages,
        'defender_messages' => $defenderMessages,
        'pass' =>
            $moonStillExists &&
            count(array_filter($attackerMessages, fn($m) => $m['moon_notice'] && !$m['destroy_success'] && $m['destroy_failed'])) > 0 &&
            count(array_filter($defenderMessages, fn($m) => $m['moon_notice'] && !$m['destroy_success'] && $m['destroy_failed'])) > 0,
    );

    cleanup_moon_edges();
    set_planet_units($ATTACKER_PLANET, 'Homeplanet', array(GID_F_SC => 1));
    set_planet_units($DEFENDER_PLANET, 'FlowP100161', array(), array());
    $moonId = create_test_moon(1000);
    $after = max_message_id();
    launch_to_moon(FTYP_DESTROY, $moonId, array(GID_F_SC => 1));
    $moonStillExists = PlanetHasMoon($DEFENDER_PLANET) === $moonId;
    $attackerMessages = message_summary($ATTACKER_ID, $after);
    $defenderMessages = message_summary($DEFENDER_ID, $after);
    $cases[] = array(
        'case' => 'moon_destruction_without_deathstar_has_no_graviton_effect',
        'moon_id' => $moonId,
        'moon_still_exists' => $moonStillExists,
        'attacker_messages' => $attackerMessages,
        'defender_messages' => $defenderMessages,
        'pass' =>
            $moonStillExists &&
            count(array_filter($attackerMessages, fn($m) => $m['moon_notice'])) === 0 &&
            count(array_filter($defenderMessages, fn($m) => $m['moon_notice'])) === 0,
    );

    cleanup_moon_edges();
    set_user_research($ATTACKER_ID);
    set_user_research($DEFENDER_ID);
    set_planet_units($ATTACKER_PLANET, 'MoonFollowupAttacker', array(GID_F_SC => 2));
    set_planet_units($DEFENDER_PLANET, 'MoonFollowupDefender', array(GID_F_SC => 2), array());
    $moonId = create_test_moon(1000);
    set_planet_units($moonId, 'EdgeMoon', array(GID_F_SC => 1), array());
    $attackerBefore = planet_snapshot($ATTACKER_PLANET);
    $when = time() + 1;
    [$foreignInboundId, $foreignInboundQueue] = dispatch_between(FTYP_TRANSPORT, $ATTACKER_PLANET, $moonId, array(GID_F_SC => 1), $when, array(GID_RC_METAL => 100));
    [$ownerOutboundId, $ownerOutboundQueue] = dispatch_between(FTYP_TRANSPORT, $moonId, $ATTACKER_PLANET, array(GID_F_SC => 1), $when, array());
    [$ownerInboundId, $ownerInboundQueue] = dispatch_between(FTYP_TRANSPORT, $DEFENDER_PLANET, $moonId, array(GID_F_SC => 1), $when, array());

    DestroyMoon($moonId, $when + 1, 0);
    $foreignReturn = latest_owner_fleet($ATTACKER_ID, FTYP_TRANSPORT + FTYP_RETURN);
    $ownerOutboundAfter = fleet_row($ownerOutboundId);
    $ownerInboundAfter = fleet_row($ownerInboundId);
    $moonReferencesAfterDestroy = intval(first_value("SELECT COUNT(*) FROM {$db_prefix}fleet WHERE start_planet={$moonId} OR target_planet={$moonId}"));
    $moonRowAfterDestroy = planet_snapshot($moonId);

    $foreignReturnCompleted = false;
    if ($foreignReturn !== null) {
        $returnQueue = GetFleetQueue(intval($foreignReturn['fleet_id']));
        if ($returnQueue) {
            Queue_Fleet_End($returnQueue);
            $foreignReturnCompleted = true;
        }
    }
    $attackerAfterReturn = planet_snapshot($ATTACKER_PLANET);
    $foreignReturnLeft = $foreignReturn === null ? 0 : intval(first_value("SELECT COUNT(*) FROM {$db_prefix}fleet WHERE fleet_id=" . intval($foreignReturn['fleet_id'])));

    $cases[] = array(
        'case' => 'destroyed_moon_retargets_related_fleets_to_planet',
        'moon_id' => $moonId,
        'prepared_fleets' => array(
            'foreign_inbound' => array('fleet_id' => $foreignInboundId, 'queue' => $foreignInboundQueue),
            'owner_outbound' => array('fleet_id' => $ownerOutboundId, 'queue' => $ownerOutboundQueue),
            'owner_inbound' => array('fleet_id' => $ownerInboundId, 'queue' => $ownerInboundQueue),
        ),
        'foreign_return' => $foreignReturn,
        'owner_outbound_after' => $ownerOutboundAfter,
        'owner_inbound_after' => $ownerInboundAfter,
        'pass' =>
            $foreignInboundId > 0 &&
            $ownerOutboundId > 0 &&
            $ownerInboundId > 0 &&
            PlanetHasMoon($DEFENDER_PLANET) === 0 &&
            $moonRowAfterDestroy === null &&
            $foreignReturn !== null &&
            intval($foreignReturn['target_planet']) === $DEFENDER_PLANET &&
            $ownerOutboundAfter !== null &&
            intval($ownerOutboundAfter['start_planet']) === $DEFENDER_PLANET &&
            $ownerInboundAfter !== null &&
            intval($ownerInboundAfter['target_planet']) === $DEFENDER_PLANET &&
            $moonReferencesAfterDestroy === 0 &&
            $foreignReturnCompleted &&
            $foreignReturnLeft === 0 &&
            $attackerBefore !== null &&
            $attackerAfterReturn !== null &&
            intval($attackerAfterReturn['small_cargo']) === intval($attackerBefore['small_cargo']) &&
            intval($attackerAfterReturn['metal']) === intval($attackerBefore['metal']),
    );
} finally {
    $restored = restore_accounts();
}

$captured = trim(ob_get_clean());
echo json_encode(array(
    'case_group' => 'moon_destruction_edge_cases',
    'cases' => $cases,
    'restored' => $restored,
    'captured_output' => $captured,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
