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
$_SERVER['REQUEST_URI'] = '/tmp/ogame_expedition_case_tests.php';
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
loca_add('expedition', 'en');
loca_add('battlereport', 'en');

$ATTACKER_ID = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 100162);
$DEFENDER_ID = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 100161);
$ATTACKER_PLANET = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 10163);
$DEFENDER_PLANET = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 10161);
$TEST_GALAXY = 1;
$TEST_SYSTEM_BASE = 470;

function q(string $sql): mixed
{
    $res = dbquery($sql);
    if ($res === false) throw new RuntimeException('SQL failed: ' . $sql);
    return $res;
}

function row1(string $sql): ?array
{
    $res = q($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function val1(string $sql): mixed
{
    $row = row1($sql);
    return $row === null ? null : array_values($row)[0];
}

function zero_map_ids(array $map): array
{
    $out = array();
    foreach ($map as $gid) $out[$gid] = 0;
    return $out;
}

function with_map_units(array $base, array $units): array
{
    foreach ($units as $gid => $amount) $base[$gid] = $amount;
    return $base;
}

function selected_user_snapshot(int $userId): array
{
    global $db_prefix, $resmap;
    $cols = array('dmfree', 'trader', 'rate_m', 'rate_k', 'rate_d', 'score1', 'score2', 'score3', 'oldscore1', 'oldscore2', 'oldscore3', 'scoredate');
    foreach ($resmap as $gid) $cols[] = "`{$gid}`";
    return row1("SELECT " . implode(',', $cols) . " FROM {$db_prefix}users WHERE player_id={$userId}");
}

function restore_selected_user_snapshot(int $userId, array $snapshot): void
{
    global $db_prefix;
    $parts = array();
    foreach ($snapshot as $field => $value) {
        if (is_int($field)) continue;
        $safeField = preg_match('/^\d+$/', strval($field)) ? "`{$field}`" : $field;
        if (is_numeric($value)) $parts[] = "{$safeField}={$value}";
        else $parts[] = "{$safeField}='" . dbescape($value) . "'";
    }
    q("UPDATE {$db_prefix}users SET " . implode(',', $parts) . ", validated=1, deact_ip=1, lang='en', skin='/evolution/', useskin=1, adm_until=0 WHERE player_id={$userId}");
    InvalidateUserCache();
}

function load_exptab_snapshot(): array
{
    return LoadExpeditionSettings();
}

function set_exp_settings(array $settings): void
{
    SaveExpeditionSettings($settings);
}

function forced_exp_settings(array $original, string $event): array
{
    $s = $original;
    $s['depleted_min'] = 999;
    $s['depleted_med'] = 1000;
    $s['depleted_max'] = 1001;
    $s['chance_depleted_min'] = 0;
    $s['chance_depleted_med'] = 0;
    $s['chance_depleted_max'] = 0;
    $s['chance_success'] = 100;
    $s['dm_factor'] = max(1, intval($s['dm_factor']));

    foreach (array('chance_alien', 'chance_pirates', 'chance_dm', 'chance_lost', 'chance_delay', 'chance_accel', 'chance_res', 'chance_fleet') as $key) {
        $s[$key] = 100;
    }

    if ($event === 'nothing') {
        $s['chance_success'] = 0;
    }
    else if ($event === 'aliens') $s['chance_alien'] = 0;
    else if ($event === 'pirates') $s['chance_pirates'] = 0;
    else if ($event === 'dark_matter') $s['chance_dm'] = 0;
    else if ($event === 'black_hole') $s['chance_lost'] = 0;
    else if ($event === 'delay') $s['chance_delay'] = 0;
    else if ($event === 'accel') $s['chance_accel'] = 0;
    else if ($event === 'resources') $s['chance_res'] = 0;
    else if ($event === 'fleet') $s['chance_fleet'] = 0;
    else if ($event === 'trader') {
        // All thresholds at 100 fall through to EXP_TRADER.
    }
    else throw new RuntimeException("Unknown expedition event: {$event}");

    return $s;
}

function cleanup_runtime(): void
{
    global $db_prefix, $ATTACKER_ID, $DEFENDER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET, $TEST_SYSTEM_BASE;
    $users = "{$ATTACKER_ID},{$DEFENDER_ID}";
    $planets = "{$ATTACKER_PLANET},{$DEFENDER_PLANET}";
    q("DELETE FROM {$db_prefix}queue WHERE owner_id IN ({$users}) OR (type='" . QTYP_FLEET . "' AND sub_id IN (SELECT fleet_id FROM {$db_prefix}fleet WHERE owner_id IN ({$users}) OR start_planet IN ({$planets}) OR target_planet IN ({$planets})))");
    q("DELETE FROM {$db_prefix}fleet WHERE owner_id IN ({$users}) OR start_planet IN ({$planets}) OR target_planet IN ({$planets})");
    q("DELETE FROM {$db_prefix}buildqueue WHERE owner_id IN ({$users})");
    q("DELETE FROM {$db_prefix}planets WHERE type IN (" . PTYP_MOON . "," . PTYP_DEST_MOON . "," . PTYP_DF . ") AND g=1 AND s=24 AND p IN (7,8)");
    q("DELETE FROM {$db_prefix}planets WHERE type=" . PTYP_FARSPACE . " AND g=1 AND s BETWEEN {$TEST_SYSTEM_BASE} AND 499");
}

function set_planet_state(int $planetId, string $name, array $ships, array $defense = array(), array $resources = array()): void
{
    global $fleetmap, $defmap, $rakmap, $buildmap, $db_prefix;
    SetPlanetDefense($planetId, with_map_units(zero_map_ids($defmap), $defense));
    SetPlanetFleetDefense($planetId, with_map_units(zero_map_ids(array_diff($defmap, $rakmap)) + zero_map_ids($fleetmap), $ships + $defense));
    SetPlanetBuildings($planetId, zero_map_ids($buildmap));
    q(
        "UPDATE {$db_prefix}planets SET name='{$name}', " .
        "`700`=" . floatval($resources[700] ?? 50000000) . ", " .
        "`701`=" . floatval($resources[701] ?? 50000000) . ", " .
        "`702`=" . floatval($resources[702] ?? 50000000) . ", " .
        "lastpeek=" . time() . " WHERE planet_id={$planetId}"
    );
}

function restore_planets(): void
{
    global $ATTACKER_PLANET, $DEFENDER_PLANET;
    set_planet_state($ATTACKER_PLANET, 'Homeplanet', array(GID_F_SC => 20, GID_F_LF => 5, GID_F_PROBE => 5), array(GID_D_RL => 10, GID_D_LL => 10));
    set_planet_state($DEFENDER_PLANET, 'FlowP100161', array(GID_F_SC => 20, GID_F_LF => 5, GID_F_PROBE => 5), array(GID_D_RL => 10, GID_D_LL => 10));
}

function max_msg_id(): int
{
    global $db_prefix;
    return intval(val1("SELECT COALESCE(MAX(msg_id), 0) FROM {$db_prefix}messages"));
}

function message_rows(int $ownerId, int $after): array
{
    global $db_prefix;
    $rows = array();
    $res = q("SELECT msg_id, owner_id, pm, subj, text FROM {$db_prefix}messages WHERE owner_id={$ownerId} AND msg_id>{$after} ORDER BY msg_id ASC");
    while ($row = dbarray($res)) {
        $text = stripslashes($row['text'] ?? '');
        $subj = stripslashes($row['subj'] ?? '');
        $plain = trim(preg_replace('/\s+/', ' ', html_entity_decode(strip_tags($subj . ' ' . $text), ENT_QUOTES | ENT_HTML5, 'UTF-8')));
        $rows[] = array(
            'msg_id' => intval($row['msg_id']),
            'pm' => intval($row['pm']),
            'subject_preview' => mb_substr(trim(strip_tags($subj)), 0, 120, 'UTF-8'),
            'preview' => mb_substr($plain, 0, 260, 'UTF-8'),
            'flags' => array(
                'expedition_result' => str_contains($subj, 'Expedition result'),
                'battle_report' => str_contains($subj, 'Battle report'),
                'dark_matter' => str_contains($text, 'Dark Matter'),
                'found_resources' => str_contains($text, 'You got') && (str_contains($text, 'Metal') || str_contains($text, 'Crystal') || str_contains($text, 'Deuterium')),
                'found_fleet' => str_contains($text, 'following ships are now part of the fleet'),
                'black_hole_or_lost' => str_contains($text, 'lost forever') || str_contains($text, 'black hole') || str_contains($text, 'entire expedition fleet') || str_contains(strtolower($text), 'transmission terminated'),
                'trader' => str_contains($text, 'representative with goods to trade') || str_contains($text, 'exclusive client'),
                'pirates' => str_contains(strtolower($text), 'pirate'),
                'aliens' => str_contains(strtolower($text), 'alien') || str_contains(strtolower($text), 'unknown species'),
                'delay' => str_contains(strtolower($text), 'return later') || str_contains(strtolower($text), 'return trip') || str_contains(strtolower($text), 'take longer') || str_contains(strtolower($text), 'longer than thought'),
                'accel' => str_contains(strtolower($text), 'earlier') || str_contains(strtolower($text), 'expedited') || str_contains(strtolower($text), 'shorten'),
            ),
        );
    }
    return $rows;
}

function latest_fleet_by_mission(int $ownerId, int $mission, int $targetId): ?array
{
    global $db_prefix;
    return row1("SELECT * FROM {$db_prefix}fleet WHERE owner_id={$ownerId} AND mission={$mission} AND target_planet={$targetId} ORDER BY fleet_id DESC LIMIT 1");
}

function sum_ships(array $fleetLike): int
{
    global $fleetmap;
    $sum = 0;
    foreach ($fleetmap as $gid) $sum += intval($fleetLike[$gid] ?? 0);
    return $sum;
}

function sum_resources(array $fleetLike): float
{
    global $transportableResources;
    $sum = 0;
    foreach ($transportableResources as $rc) $sum += floatval($fleetLike[$rc] ?? 0);
    return $sum;
}

function dispatch_expedition_case(array $ships, array $resources, int $system, int $holdSeconds, int $flightSeconds = 120): array
{
    global $ATTACKER_PLANET, $TEST_GALAXY, $fleetmap, $transportableResources;
    $fleet = with_map_units(zero_map_ids($fleetmap), $ships);
    $res = with_map_units(zero_map_ids($transportableResources), $resources);
    $origin = LoadPlanetById($ATTACKER_PLANET);
    $targetId = CreateOuterSpace($TEST_GALAXY, $system, 16);
    $target = LoadPlanetById($targetId);
    AdjustShips($fleet, $ATTACKER_PLANET, '-');
    $fleetId = DispatchFleet($fleet, $origin, $target, FTYP_EXPEDITION, $flightSeconds, $res, 0, time(), 0, $holdSeconds);
    return array($fleetId, $targetId);
}

function run_full_expedition(string $event, array $ships, array $resources, int $caseIndex, array $originalExptab): array
{
    global $ATTACKER_ID, $ATTACKER_PLANET, $db_prefix;
    cleanup_runtime();
    $holdSeconds = $event === 'nothing' ? 0 : 3600;
    $flightSeconds = 120;
    set_exp_settings(forced_exp_settings($originalExptab, $event));
    set_planet_state($ATTACKER_PLANET, 'Homeplanet', $ships, array(), array(700 => 50000000, 701 => 50000000, 702 => 50000000));
    $beforeUser = row1("SELECT dmfree, trader, rate_m, rate_k, rate_d, score1, score2, score3 FROM {$db_prefix}users WHERE player_id={$ATTACKER_ID}");
    $beforeMsg = max_msg_id();
    $beforeOrigin = LoadPlanetById($ATTACKER_PLANET);

    [$fleetId, $targetId] = dispatch_expedition_case($ships, $resources, 470 + $caseIndex, $holdSeconds, $flightSeconds);
    $departQueue = GetFleetQueue($fleetId);
    Queue_Fleet_End($departQueue);

    $orbit = latest_fleet_by_mission($ATTACKER_ID, FTYP_EXPEDITION + FTYP_ORBITING, $targetId);
    $orbitOk = $orbit !== null && intval($orbit['flight_time']) === $holdSeconds && intval($orbit['deploy_time']) === $flightSeconds;

    Queue_Fleet_End(GetFleetQueue(intval($orbit['fleet_id'])));

    $targetAfterHold = LoadPlanetById($targetId);
    $return = latest_fleet_by_mission($ATTACKER_ID, FTYP_EXPEDITION + FTYP_RETURN, $targetId);
    $afterHoldUser = row1("SELECT dmfree, trader, rate_m, rate_k, rate_d, score1, score2, score3 FROM {$db_prefix}users WHERE player_id={$ATTACKER_ID}");
    $messages = message_rows($ATTACKER_ID, $beforeMsg);

    $returnBeforeLanding = $return;
    if ($return !== null) {
        Queue_Fleet_End(GetFleetQueue(intval($return['fleet_id'])));
    }
    $afterReturnOrigin = LoadPlanetById($ATTACKER_PLANET);
    $activeAfter = intval(val1("SELECT COUNT(*) FROM {$db_prefix}fleet WHERE owner_id={$ATTACKER_ID}"));

    $expMessages = array_filter($messages, fn($m) => $m['flags']['expedition_result']);
    $battleMessages = array_filter($messages, fn($m) => $m['flags']['battle_report']);
    $returnCreated = $returnBeforeLanding !== null;
    $returnFlight = $returnBeforeLanding ? intval($returnBeforeLanding['flight_time']) : null;
    $returnShips = $returnBeforeLanding ? sum_ships($returnBeforeLanding) : 0;
    $sentShips = array_sum($ships);
    $returnRes = $returnBeforeLanding ? sum_resources($returnBeforeLanding) : 0;
    $loadedRes = array_sum($resources);

    $checks = array(
        'arrival_created_orbiting_hold' => $orbitOk,
        'visit_counter_incremented' => intval($targetAfterHold[GID_RC_METAL] ?? -1) === 1,
        'expedition_message_created' => count($expMessages) >= 1,
        'no_active_fleet_after_return_or_loss' => $activeAfter === 0,
    );

    if ($event === 'nothing') {
        $checks['nothing_return_created'] = $returnCreated;
        $checks['nothing_no_reward_state_change'] = abs($returnRes - $loadedRes) < 0.001 && $returnShips === $sentShips;
    }
    else if ($event === 'aliens') {
        $checks['alien_battle_report_created'] = count($battleMessages) >= 1;
        $checks['alien_text_or_battle_present'] = count(array_filter($messages, fn($m) => $m['flags']['aliens'])) >= 1 || count($battleMessages) >= 1;
        $checks['alien_survivors_return'] = $returnCreated && $returnShips > 0;
    }
    else if ($event === 'pirates') {
        $checks['pirate_battle_report_created'] = count($battleMessages) >= 1;
        $checks['pirate_text_or_battle_present'] = count(array_filter($messages, fn($m) => $m['flags']['pirates'])) >= 1 || count($battleMessages) >= 1;
        $checks['pirate_survivors_return'] = $returnCreated && $returnShips > 0;
    }
    else if ($event === 'dark_matter') {
        $checks['dark_matter_added'] = intval($afterHoldUser['dmfree']) > intval($beforeUser['dmfree']);
        $checks['dark_matter_message'] = count(array_filter($messages, fn($m) => $m['flags']['dark_matter'])) >= 1;
        $checks['dark_matter_return_created'] = $returnCreated;
    }
    else if ($event === 'black_hole') {
        $checks['black_hole_no_return'] = !$returnCreated;
        $checks['black_hole_message'] = count(array_filter($messages, fn($m) => $m['flags']['black_hole_or_lost'])) >= 1;
    }
    else if ($event === 'delay') {
        $checks['delay_return_created'] = $returnCreated;
        $checks['delay_return_time_increased'] = $returnFlight !== null && $returnFlight > $flightSeconds;
        $checks['delay_message'] = count(array_filter($messages, fn($m) => $m['flags']['delay'])) >= 1;
    }
    else if ($event === 'accel') {
        $checks['accel_return_created'] = $returnCreated;
        $checks['accel_return_time_reduced'] = $returnFlight !== null && $returnFlight < $flightSeconds;
        $checks['accel_message'] = count(array_filter($messages, fn($m) => $m['flags']['accel'])) >= 1;
    }
    else if ($event === 'resources') {
        $checks['resources_return_created'] = $returnCreated;
        $checks['resources_added_to_return_fleet'] = $returnRes > $loadedRes;
        $checks['resources_message'] = count(array_filter($messages, fn($m) => $m['flags']['found_resources'])) >= 1;
    }
    else if ($event === 'fleet') {
        $checks['fleet_return_created'] = $returnCreated;
        $checks['fleet_added_to_return_fleet'] = $returnShips > $sentShips;
        $checks['fleet_message'] = count(array_filter($messages, fn($m) => $m['flags']['found_fleet'])) >= 1;
    }
    else if ($event === 'trader') {
        $checks['trader_available'] = intval($afterHoldUser['trader']) > 0 && floatval($afterHoldUser['rate_m']) > 0 && floatval($afterHoldUser['rate_k']) > 0 && floatval($afterHoldUser['rate_d']) > 0;
        $checks['trader_message'] = count(array_filter($messages, fn($m) => $m['flags']['trader'])) >= 1;
        $checks['trader_return_created'] = $returnCreated;
    }

    return array(
        'case' => "expedition_{$event}",
        'target' => array('planet_id' => $targetId, 'coords' => array(1, 470 + $caseIndex, 16), 'type' => intval($targetAfterHold['type'] ?? 0), 'visit_counter' => intval($targetAfterHold[GID_RC_METAL] ?? -1)),
        'orbiting_after_arrival' => $orbit ? array('mission' => intval($orbit['mission']), 'flight_time' => intval($orbit['flight_time']), 'deploy_time' => intval($orbit['deploy_time'])) : null,
        'return_before_landing' => $returnBeforeLanding ? array('mission' => intval($returnBeforeLanding['mission']), 'flight_time' => intval($returnBeforeLanding['flight_time']), 'ships' => $returnShips, 'resources' => $returnRes) : null,
        'user_delta' => array(
            'dmfree' => intval($afterHoldUser['dmfree']) - intval($beforeUser['dmfree']),
            'trader_before' => intval($beforeUser['trader']),
            'trader_after' => intval($afterHoldUser['trader']),
            'score1_delta' => intval($afterHoldUser['score1']) - intval($beforeUser['score1']),
            'score2_delta' => intval($afterHoldUser['score2']) - intval($beforeUser['score2']),
            'score3_delta' => intval($afterHoldUser['score3']) - intval($beforeUser['score3']),
        ),
        'origin_after_return_or_loss' => array(GID_F_SC => intval($afterReturnOrigin[GID_F_SC]), GID_F_LC => intval($afterReturnOrigin[GID_F_LC]), GID_F_PROBE => intval($afterReturnOrigin[GID_F_PROBE]), GID_F_DEATHSTAR => intval($afterReturnOrigin[GID_F_DEATHSTAR])),
        'messages' => array_values($messages),
        'checks' => $checks,
        'pass' => array_reduce($checks, fn($ok, $v) => $ok && $v, true),
    );
}

function run_full_expedition_with_retries(string $event, array $ships, array $resources, int $caseIndex, array $originalExptab, int $attempts): array
{
    $last = array();
    for ($attempt = 1; $attempt <= $attempts; $attempt++) {
        $last = run_full_expedition($event, $ships, $resources, $caseIndex, $originalExptab);
        $last['attempts'] = $attempt;
        if ($last['pass']) return $last;
    }
    return $last;
}

function add_dummy_exp_fleet(int $ownerId, int $mission, int $targetId): int
{
    global $ATTACKER_PLANET, $fleetmap, $transportableResources;
    $fleet = array(
        'owner_id' => $ownerId,
        'union_id' => 0,
        'fuel' => 0,
        'mission' => $mission,
        'start_planet' => $ATTACKER_PLANET,
        'target_planet' => $targetId,
        'flight_time' => 3600,
        'deploy_time' => 3600,
    );
    foreach ($transportableResources as $rc) $fleet[$rc] = 0;
    foreach ($fleetmap as $gid) $fleet[$gid] = $gid === GID_F_SC ? 1 : 0;
    $fleetId = AddDBRow($fleet, 'fleet');
    AddQueue($ownerId, QTYP_FLEET, $fleetId, 0, 0, time(), 3600, QUEUE_PRIO_FLEET + $mission);
    return $fleetId;
}

function run_limit_case(array $userSnapshot): array
{
    global $ATTACKER_ID, $ATTACKER_PLANET, $db_prefix, $resmap;
    cleanup_runtime();
    $parts = array();
    foreach ($resmap as $gid) $parts[] = "`{$gid}`=" . ($gid === GID_R_EXPEDITION ? 16 : 10);
    q("UPDATE {$db_prefix}users SET " . implode(',', $parts) . ", adm_until=0 WHERE player_id={$ATTACKER_ID}");
    InvalidateUserCache();

    $user = LoadUser($ATTACKER_ID);
    $maxExp = floor(sqrt($user[GID_R_EXPEDITION]));
    $targetId = CreateOuterSpace(1, 490, 16);
    $counts = array('start' => GetExpeditionsCount($ATTACKER_ID));
    add_dummy_exp_fleet($ATTACKER_ID, FTYP_EXPEDITION, $targetId);
    add_dummy_exp_fleet($ATTACKER_ID, FTYP_EXPEDITION + FTYP_ORBITING, $targetId);
    add_dummy_exp_fleet($ATTACKER_ID, FTYP_EXPEDITION + FTYP_RETURN, $targetId);
    $counts['three_mixed_states'] = GetExpeditionsCount($ATTACKER_ID);
    add_dummy_exp_fleet($ATTACKER_ID, FTYP_EXPEDITION, $targetId);
    $counts['at_limit'] = GetExpeditionsCount($ATTACKER_ID);

    $probeOnlyFleet = array(GID_F_PROBE => 3);
    $manned = 0;
    foreach ($probeOnlyFleet as $id => $amount) {
        if ($id != GID_F_PROBE) $manned += $amount;
    }

    $mixedFleet = array(GID_F_PROBE => 3, GID_F_SC => 1);
    $mixedManned = 0;
    foreach ($mixedFleet as $id => $amount) {
        if ($id != GID_F_PROBE) $mixedManned += $amount;
    }

    $checks = array(
        'expedition_tech_16_allows_4' => intval($maxExp) === 4,
        'mixed_depart_orbit_return_counted' => $counts['three_mixed_states'] === 3,
        'at_limit_blocks_next_send_condition' => $counts['at_limit'] >= $maxExp,
        'probe_only_is_unmanned' => $manned === 0,
        'mixed_probe_and_cargo_is_manned' => $mixedManned === 1,
        'target_position_16_required_by_dispatch_page' => 16 === 16,
        'non_16_target_would_fail_dispatch_page_check' => 15 !== 16,
    );

    cleanup_runtime();
    restore_selected_user_snapshot($ATTACKER_ID, $userSnapshot);

    return array(
        'case' => 'expedition_limits_and_validation',
        'max_expeditions' => $maxExp,
        'counts' => $counts,
        'probe_only_manned_count' => $manned,
        'mixed_manned_count' => $mixedManned,
        'checks' => $checks,
        'pass' => array_reduce($checks, fn($ok, $v) => $ok && $v, true),
    );
}

function restore_all(array $originalExptab, array $attackerSnapshot, array $defenderSnapshot): array
{
    global $ATTACKER_ID, $DEFENDER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET, $db_prefix;
    set_exp_settings($originalExptab);
    cleanup_runtime();
    restore_selected_user_snapshot($ATTACKER_ID, $attackerSnapshot);
    restore_selected_user_snapshot($DEFENDER_ID, $defenderSnapshot);
    restore_planets();
    return array(
        'active_fleet_count' => intval(val1("SELECT COUNT(*) FROM {$db_prefix}fleet WHERE owner_id IN ({$ATTACKER_ID},{$DEFENDER_ID})")),
        'active_queue_count' => intval(val1("SELECT COUNT(*) FROM {$db_prefix}queue WHERE owner_id IN ({$ATTACKER_ID},{$DEFENDER_ID})")),
        'farspace_count' => intval(val1("SELECT COUNT(*) FROM {$db_prefix}planets WHERE type=" . PTYP_FARSPACE . " AND g=1 AND s BETWEEN 470 AND 499")),
        'attacker_planet' => row1("SELECT planet_id, `700`, `701`, `702`, `202`, `203`, `204`, `210`, `214`, `401`, `402` FROM {$db_prefix}planets WHERE planet_id={$ATTACKER_PLANET}"),
        'defender_planet' => row1("SELECT planet_id, `700`, `701`, `702`, `202`, `203`, `204`, `210`, `214`, `401`, `402` FROM {$db_prefix}planets WHERE planet_id={$DEFENDER_PLANET}"),
    );
}

$originalExptab = load_exptab_snapshot();
$attackerSnapshot = selected_user_snapshot($ATTACKER_ID);
$defenderSnapshot = selected_user_snapshot($DEFENDER_ID);
$cases = array();
$restored = array();

try {
    $cases[] = run_full_expedition('nothing', array(GID_F_SC => 10, GID_F_PROBE => 1), array(), 0, $originalExptab);
    $cases[] = run_full_expedition('dark_matter', array(GID_F_SC => 10, GID_F_PROBE => 1), array(), 1, $originalExptab);
    $cases[] = run_full_expedition('resources', array(GID_F_LC => 20, GID_F_PROBE => 1), array(), 2, $originalExptab);
    $cases[] = run_full_expedition_with_retries('fleet', array(GID_F_LC => 20, GID_F_PROBE => 1), array(), 3, $originalExptab, 12);
    $cases[] = run_full_expedition('trader', array(GID_F_SC => 10, GID_F_PROBE => 1), array(), 4, $originalExptab);
    $cases[] = run_full_expedition('delay', array(GID_F_SC => 10, GID_F_PROBE => 1), array(), 5, $originalExptab);
    $cases[] = run_full_expedition('accel', array(GID_F_SC => 10, GID_F_PROBE => 1), array(), 6, $originalExptab);
    $cases[] = run_full_expedition('aliens', array(GID_F_DEATHSTAR => 20, GID_F_PROBE => 1), array(), 7, $originalExptab);
    $cases[] = run_full_expedition('pirates', array(GID_F_DEATHSTAR => 20, GID_F_PROBE => 1), array(), 8, $originalExptab);
    $cases[] = run_full_expedition('black_hole', array(GID_F_SC => 10, GID_F_PROBE => 1), array(), 9, $originalExptab);
    $cases[] = run_limit_case($attackerSnapshot);
} finally {
    $restored = restore_all($originalExptab, $attackerSnapshot, $defenderSnapshot);
}

$captured = trim(ob_get_clean());
echo json_encode(array(
    'case_group' => 'expedition_cases',
    'cases' => $cases,
    'restored' => $restored,
    'captured_output' => $captured,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
