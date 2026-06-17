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

function esc_sql(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
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

function set_user_sessions(): array
{
    global $db_prefix, $ATTACKER_ID, $DEFENDER_ID;

    $sessions = array(
        $ATTACKER_ID => array('session' => substr(md5('attacker-' . microtime(true)), 0, 12), 'private' => md5('attacker-private-' . microtime(true))),
        $DEFENDER_ID => array('session' => substr(md5('defender-' . microtime(true)), 0, 12), 'private' => md5('defender-private-' . microtime(true))),
    );

    foreach ($sessions as $userId => $sess) {
        sql_exec(
            "UPDATE {$db_prefix}users SET " .
            "session='" . esc_sql($sess['session']) . "', " .
            "private_session='" . esc_sql($sess['private']) . "', " .
            "validated=1, deact_ip=1, lang='en', skin='/evolution/', useskin=1 " .
            "WHERE player_id={$userId}"
        );
    }

    return $sessions;
}

function cleanup_runtime(bool $deleteMessages): void
{
    global $db_prefix, $ATTACKER_ID, $DEFENDER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET;

    $users = "{$ATTACKER_ID},{$DEFENDER_ID}";
    $planets = "{$ATTACKER_PLANET},{$DEFENDER_PLANET}";

    sql_exec("DELETE FROM {$db_prefix}queue WHERE owner_id IN ({$users}) OR (type='" . QTYP_FLEET . "' AND sub_id IN (SELECT fleet_id FROM {$db_prefix}fleet WHERE owner_id IN ({$users}) OR start_planet IN ({$planets}) OR target_planet IN ({$planets})))");
    sql_exec("DELETE FROM {$db_prefix}fleet WHERE owner_id IN ({$users}) OR start_planet IN ({$planets}) OR target_planet IN ({$planets})");
    sql_exec("DELETE FROM {$db_prefix}buildqueue WHERE owner_id IN ({$users})");
    sql_exec("DELETE FROM {$db_prefix}planets WHERE type IN (" . PTYP_MOON . "," . PTYP_DF . ") AND g=1 AND s=24 AND p IN (7,8)");

    if ($deleteMessages) {
        sql_exec("DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$users})");
    }
}

function set_user_tech(int $userId, array $tech): void
{
    global $db_prefix, $resmap;
    $parts = array();
    foreach ($resmap as $gid) {
        $parts[] = "`{$gid}`=" . intval($tech[$gid] ?? 0);
    }
    sql_exec("UPDATE {$db_prefix}users SET " . implode(',', $parts) . " WHERE player_id={$userId}");
    InvalidateUserCache();
}

function reset_planets(array $attackerShips, array $defenderShips, array $defenderDefence, array $defenderBuildings, array $defenderResources): void
{
    global $db_prefix, $fleetmap, $defmap, $rakmap, $buildmap;
    global $ATTACKER_PLANET, $DEFENDER_PLANET;

    $emptyFleet = zero_map($fleetmap);
    $emptyDef = zero_map(array_diff($defmap, $rakmap));
    $emptyBuildings = zero_map($buildmap);

    $attObjects = with_units($emptyDef + $emptyFleet, $attackerShips);
    $defObjects = with_units(with_units($emptyDef + $emptyFleet, $defenderShips), $defenderDefence);

    SetPlanetFleetDefense($ATTACKER_PLANET, $attObjects);
    SetPlanetFleetDefense($DEFENDER_PLANET, $defObjects);
    SetPlanetBuildings($ATTACKER_PLANET, $emptyBuildings);
    SetPlanetBuildings($DEFENDER_PLANET, with_units($emptyBuildings, $defenderBuildings));

    $now = time();
    sql_exec("UPDATE {$db_prefix}planets SET name='ReportAttacker', `700`=1000000, `701`=1000000, `702`=1000000, `502`=0, `503`=0, lastpeek={$now} WHERE planet_id={$ATTACKER_PLANET}");
    sql_exec(
        "UPDATE {$db_prefix}planets SET name='ReportDefender', " .
        "`700`=" . floatval($defenderResources[700] ?? 1000000) . ", " .
        "`701`=" . floatval($defenderResources[701] ?? 1000000) . ", " .
        "`702`=" . floatval($defenderResources[702] ?? 1000000) . ", " .
        "`502`=0, `503`=0, lastpeek={$now} WHERE planet_id={$DEFENDER_PLANET}"
    );
}

function dispatch_test_fleet(int $mission, array $ships, int $when): array
{
    global $fleetmap, $transportableResources, $ATTACKER_PLANET, $DEFENDER_PLANET;

    $fleet = with_units(zero_map($fleetmap), $ships);
    $resources = zero_map($transportableResources);
    $origin = LoadPlanetById($ATTACKER_PLANET);
    $target = LoadPlanetById($DEFENDER_PLANET);

    AdjustShips($fleet, $ATTACKER_PLANET, '-');
    $fleetId = DispatchFleet($fleet, $origin, $target, $mission, 1, $resources, 0, $when);
    $queue = GetFleetQueue($fleetId);

    return array($fleetId, $queue);
}

function max_msg_id(): int
{
    global $db_prefix;
    return intval(sql_value("SELECT COALESCE(MAX(msg_id), 0) FROM {$db_prefix}messages"));
}

function max_battle_id(): int
{
    global $db_prefix;
    return intval(sql_value("SELECT COALESCE(MAX(battle_id), 0) FROM {$db_prefix}battledata"));
}

function battle_log_after(int $after): ?array
{
    global $db_prefix;
    return sql_row("SELECT battle_id, title, report FROM {$db_prefix}battledata WHERE battle_id>{$after} ORDER BY battle_id ASC LIMIT 1");
}

function first_attacker_shots(string $report): int
{
    if (preg_match('/The attacking fleet fires ([0-9.,]+) times/i', html_entity_decode(strip_tags($report), ENT_QUOTES | ENT_HTML5, 'UTF-8'), $m) !== 1) {
        return 0;
    }
    return intval(preg_replace('/\D+/', '', $m[1]));
}

function set_universe_battle_options(?int $rapid = null, ?int $defrepair = null, ?int $defrepairDelta = null): array
{
    global $db_prefix, $GlobalUni;
    $before = LoadUniverse();
    $parts = array();
    if ($rapid !== null) $parts[] = "rapid={$rapid}";
    if ($defrepair !== null) $parts[] = "defrepair={$defrepair}";
    if ($defrepairDelta !== null) $parts[] = "defrepair_delta={$defrepairDelta}";
    if (!empty($parts)) {
        sql_exec("UPDATE {$db_prefix}uni SET " . implode(',', $parts));
        $GlobalUni = LoadUniverse();
    }
    return $before;
}

function restore_universe_battle_options(array $snapshot): void
{
    global $db_prefix, $GlobalUni;
    sql_exec(
        "UPDATE {$db_prefix}uni SET " .
        "rapid=" . intval($snapshot['rapid']) . ", " .
        "defrepair=" . intval($snapshot['defrepair']) . ", " .
        "defrepair_delta=" . intval($snapshot['defrepair_delta']) . ", " .
        "battle_max=" . intval($snapshot['battle_max'])
    );
    $GlobalUni = LoadUniverse();
}

function message_rows(int $ownerId, int $after): array
{
    global $db_prefix;
    $rows = array();
    $res = sql_exec("SELECT msg_id, owner_id, pm, subj, text, planet_id FROM {$db_prefix}messages WHERE owner_id={$ownerId} AND msg_id>{$after} ORDER BY msg_id ASC");
    while ($row = dbarray($res)) {
        $rows[] = summarize_message($row);
    }
    return $rows;
}

function summarize_message(array $row): array
{
    $text = $row['text'] ?? '';
    $plain = trim(preg_replace('/\s+/', ' ', html_entity_decode(strip_tags($text), ENT_QUOTES | ENT_HTML5, 'UTF-8')));
    return array(
        'msg_id' => intval($row['msg_id']),
        'owner_id' => intval($row['owner_id']),
        'pm' => intval($row['pm']),
        'planet_id' => intval($row['planet_id']),
        'text_len' => strlen($text),
        'preview' => mb_substr($plain, 0, 220, 'UTF-8'),
        'flags' => array(
            'battle_attacker_won' => str_contains($text, 'The attacker has won the battle!'),
            'battle_defender_won' => str_contains($text, 'The defender has won the battle!'),
            'battle_draw' => str_contains($text, 'The battle ended in a draw'),
            'battle_contact_lost' => str_contains($text, 'Contact with the attacking fleet has been lost'),
            'battle_has_round_text' => str_contains($text, 'The attacking fleet fires') || str_contains($text, 'In total, the defending fleet fires'),
            'spy_resources' => str_contains($text, 'Resources on'),
            'spy_fleet' => str_contains($text, '>Fleet'),
            'spy_defence' => str_contains($text, '>Defence'),
            'spy_buildings' => str_contains($text, '>Buildings'),
            'spy_research' => str_contains($text, '>Research'),
            'spy_counter' => str_contains($text, 'Chance for spy counter'),
        ),
    );
}

function first_report(array $messages, int $pm): ?array
{
    foreach ($messages as $msg) {
        if ($msg['pm'] === $pm) return $msg;
    }
    return null;
}

function assert_case(bool $condition, string $message, array $context = array()): array
{
    return array('pass' => $condition, 'message' => $message, 'context' => $context);
}

function run_battle_case(string $name, array $attShips, array $defShips, array $defence, array $resources, ?int $seed = null): array
{
    global $ATTACKER_ID, $DEFENDER_ID, $DEFENDER_PLANET;

    cleanup_runtime(false);
    set_user_tech($ATTACKER_ID, array(109 => 0, 110 => 0, 111 => 0, 106 => 0));
    set_user_tech($DEFENDER_ID, array(109 => 0, 110 => 0, 111 => 0, 106 => 0));
    reset_planets($attShips, $defShips, $defence, array(), $resources);

    $after = max_msg_id();
    $afterBattle = max_battle_id();
    $when = time() + 1;
    if ($seed !== null) {
        mt_srand($seed);
        srand($seed);
    }
    [$fleetId, $queue] = dispatch_test_fleet(FTYP_ATTACK, $attShips, $when);
    $result = StartBattle($fleetId, $DEFENDER_PLANET, intval($queue['end']));
    DeleteFleet($fleetId);
    RemoveQueue(intval($queue['task_id']));

    $attMessages = message_rows($ATTACKER_ID, $after);
    $defMessages = message_rows($DEFENDER_ID, $after);
    $attackerReport = first_report($attMessages, MTYP_BATTLE_REPORT_TEXT);
    $defenderReport = first_report($defMessages, MTYP_BATTLE_REPORT_TEXT);
    $battleLog = battle_log_after($afterBattle);
    $expected = str_contains($name, 'attacker_wins') ? 'attacker_won' : (str_contains($name, 'defender_wins') ? 'defender_won' : 'draw');
    $actual = array(BATTLE_RESULT_AWON => 'attacker_won', BATTLE_RESULT_DWON => 'defender_won', BATTLE_RESULT_DRAW => 'draw')[$result] ?? 'unknown';

    return array(
        'case' => $name,
        'result' => $actual,
        'attacker_report' => $attackerReport,
        'defender_report' => $defenderReport,
        'battle_log' => $battleLog === null ? null : array('battle_id' => intval($battleLog['battle_id']), 'title' => $battleLog['title'], 'report_len' => strlen($battleLog['report'] ?? '')),
        'attacker_messages' => $attMessages,
        'defender_messages' => $defMessages,
        'checks' => array(
            assert_case($actual === $expected, 'battle result matches expected outcome', array('expected' => $expected, 'actual' => $actual)),
            assert_case($attackerReport !== null, 'attacker battle report is generated'),
            assert_case($defenderReport !== null, 'defender battle report is generated'),
            assert_case($battleLog !== null && strlen($battleLog['report'] ?? '') > 0, 'battledata log report is persisted for simulator/admin views', $battleLog === null ? array() : array('battle_id' => intval($battleLog['battle_id']), 'report_len' => strlen($battleLog['report'] ?? ''))),
        ),
    );
}

function run_rapid_fire_toggle_case(): array
{
    global $ATTACKER_ID, $DEFENDER_ID, $DEFENDER_PLANET;

    $snapshot = set_universe_battle_options(0, null, null);
    try {
        $runs = array();
        foreach (array(0, 1) as $rapid) {
            set_universe_battle_options($rapid, null, null);
            cleanup_runtime(false);
            set_user_tech($ATTACKER_ID, array(109 => 0, 110 => 0, 111 => 0, 106 => 0));
            set_user_tech($DEFENDER_ID, array(109 => 0, 110 => 0, 111 => 0, 106 => 0));
            reset_planets(array(GID_F_CRUISER => 6), array(GID_F_LF => 120), array(), array(), array(700 => 0, 701 => 0, 702 => 0));
            $afterBattle = max_battle_id();
            $when = time() + 1;
            mt_srand(404);
            srand(404);
            [$fleetId, $queue] = dispatch_test_fleet(FTYP_ATTACK, array(GID_F_CRUISER => 6), $when);
            $result = StartBattle($fleetId, $DEFENDER_PLANET, intval($queue['end']));
            DeleteFleet($fleetId);
            RemoveQueue(intval($queue['task_id']));
            $battleLog = battle_log_after($afterBattle);
            $defenderAfter = sql_row("SELECT `" . GID_F_LF . "` AS light_fighter FROM {$GLOBALS['db_prefix']}planets WHERE planet_id={$DEFENDER_PLANET} LIMIT 1");
            $runs[$rapid] = array(
                'result' => $result,
                'shots' => $battleLog === null ? 0 : first_attacker_shots($battleLog['report'] ?? ''),
                'defender_light_fighter_after' => $defenderAfter === null ? null : intval($defenderAfter['light_fighter']),
                'battle_log' => $battleLog === null ? null : array('battle_id' => intval($battleLog['battle_id']), 'report_len' => strlen($battleLog['report'] ?? '')),
            );
        }
    } finally {
        restore_universe_battle_options($snapshot);
    }

    return array(
        'case' => 'rapid_fire_toggle_changes_battle_round_output',
        'runs' => $runs,
        'checks' => array(
            assert_case(($runs[0]['shots'] ?? 0) > 0 && ($runs[1]['shots'] ?? 0) > 0, 'both rapid-fire comparison runs produce parseable attacker shot counts', $runs),
            assert_case(($runs[1]['shots'] ?? 0) > ($runs[0]['shots'] ?? 0), 'rapid-fire enabled produces more attacker shots than rapid-fire disabled for cruiser-vs-light-fighter combat', array('rapid_off_shots' => $runs[0]['shots'] ?? null, 'rapid_on_shots' => $runs[1]['shots'] ?? null)),
            assert_case(($runs[1]['defender_light_fighter_after'] ?? PHP_INT_MAX) <= ($runs[0]['defender_light_fighter_after'] ?? 0), 'rapid-fire enabled does not leave more light fighters than the same seeded battle without rapid-fire', array('rapid_off' => $runs[0]['defender_light_fighter_after'] ?? null, 'rapid_on' => $runs[1]['defender_light_fighter_after'] ?? null)),
        ),
    );
}

function run_defense_repair_writeback_case(): array
{
    global $ATTACKER_ID, $DEFENDER_ID, $DEFENDER_PLANET, $db_prefix;

    $snapshot = set_universe_battle_options(null, 100, 0);
    try {
        cleanup_runtime(false);
        set_user_tech($ATTACKER_ID, array(109 => 10, 110 => 10, 111 => 10, 106 => 0));
        set_user_tech($DEFENDER_ID, array(109 => 0, 110 => 0, 111 => 0, 106 => 0));
        reset_planets(array(GID_F_BATTLESHIP => 80), array(), array(GID_D_RL => 300), array(), array(700 => 0, 701 => 0, 702 => 0));
        $before = sql_row("SELECT `" . GID_D_RL . "` AS rocket_launcher FROM {$db_prefix}planets WHERE planet_id={$DEFENDER_PLANET} LIMIT 1");
        $afterBattle = max_battle_id();
        $when = time() + 1;
        mt_srand(505);
        srand(505);
        [$fleetId, $queue] = dispatch_test_fleet(FTYP_ATTACK, array(GID_F_BATTLESHIP => 80), $when);
        $result = StartBattle($fleetId, $DEFENDER_PLANET, intval($queue['end']));
        DeleteFleet($fleetId);
        RemoveQueue(intval($queue['task_id']));
        $after = sql_row("SELECT `" . GID_D_RL . "` AS rocket_launcher FROM {$db_prefix}planets WHERE planet_id={$DEFENDER_PLANET} LIMIT 1");
        $battleLog = battle_log_after($afterBattle);
        $plainReport = $battleLog === null ? '' : html_entity_decode(strip_tags($battleLog['report'] ?? ''), ENT_QUOTES | ENT_HTML5, 'UTF-8');
    } finally {
        restore_universe_battle_options($snapshot);
    }

    return array(
        'case' => 'defense_repair_report_and_writeback',
        'battle_result' => array(BATTLE_RESULT_AWON => 'attacker_won', BATTLE_RESULT_DWON => 'defender_won', BATTLE_RESULT_DRAW => 'draw')[$result] ?? 'unknown',
        'before' => $before,
        'after' => $after,
        'battle_log' => $battleLog === null ? null : array('battle_id' => intval($battleLog['battle_id']), 'report_len' => strlen($battleLog['report'] ?? '')),
        'checks' => array(
            assert_case($battleLog !== null && str_contains($plainReport, 'could be repaired'), 'battle report includes repaired-defense text', array('report_preview' => mb_substr($plainReport, -260, 260, 'UTF-8'))),
            assert_case($before !== null && $after !== null && intval($after['rocket_launcher']) === intval($before['rocket_launcher']), '100 percent defense repair writes repaired rocket launchers back to the planet', array('before' => $before, 'after' => $after)),
        ),
    );
}

function run_spy_case(string $name, int $attSpyTech, int $defSpyTech, int $probes, array $defShips, array $defence, array $buildings, array $defTech): array
{
    global $ATTACKER_ID, $DEFENDER_ID;

    cleanup_runtime(false);
    set_user_tech($ATTACKER_ID, array(106 => $attSpyTech, 109 => 0, 110 => 0, 111 => 0));
    set_user_tech($DEFENDER_ID, with_units(array(106 => $defSpyTech, 109 => 0, 110 => 0, 111 => 0), $defTech));
    reset_planets(array(GID_F_PROBE => $probes), $defShips, $defence, $buildings, array(700 => 123456, 701 => 65432, 702 => 3210));

    $after = max_msg_id();
    $when = time() + 1;
    mt_srand(42);
    srand(42);
    [$fleetId, $queue] = dispatch_test_fleet(FTYP_SPY, array(GID_F_PROBE => $probes), $when);
    Queue_Fleet_End($queue);

    $attMessages = message_rows($ATTACKER_ID, $after);
    $defMessages = message_rows($DEFENDER_ID, $after);
    $spyReport = first_report($attMessages, MTYP_SPY_REPORT);
    $probeBattleExpected = str_contains($name, 'detected_probe_destroyed');

    return array(
        'case' => $name,
        'attacker_spy_report' => $spyReport,
        'attacker_battle_report' => first_report($attMessages, MTYP_BATTLE_REPORT_TEXT),
        'defender_battle_report' => first_report($defMessages, MTYP_BATTLE_REPORT_TEXT),
        'attacker_messages' => $attMessages,
        'defender_messages' => $defMessages,
        'checks' => array(
            assert_case($spyReport !== null, 'attacker espionage report is generated'),
            assert_case(!$probeBattleExpected || first_report($attMessages, MTYP_BATTLE_REPORT_TEXT) !== null || first_report($defMessages, MTYP_BATTLE_REPORT_TEXT) !== null, 'detected probe case produces a battle report'),
        ),
    );
}

$result = array(
    'users' => array(
        'attacker' => array('player_id' => $ATTACKER_ID, 'planet_id' => $ATTACKER_PLANET),
        'defender' => array('player_id' => $DEFENDER_ID, 'planet_id' => $DEFENDER_PLANET),
    ),
    'sessions' => set_user_sessions(),
    'battle_cases' => array(),
    'spy_cases' => array(),
);

cleanup_runtime(true);

$result['battle_cases'][] = run_battle_case(
    'attacker_wins_and_plunders_empty_planet',
    array(GID_F_BATTLESHIP => 1, GID_F_LC => 2),
    array(),
    array(),
    array(700 => 900000, 701 => 600000, 702 => 300000)
);

$result['battle_cases'][] = run_battle_case(
    'defender_wins_and_attacker_gets_contact_lost',
    array(GID_F_LF => 1),
    array(),
    array(GID_D_PLASMA => 10, GID_D_RL => 20),
    array(700 => 100000, 701 => 100000, 702 => 100000)
);

$result['battle_cases'][] = run_battle_case(
    'draw_after_six_rounds',
    array(GID_F_LC => 1),
    array(GID_F_LC => 1),
    array(),
    array(700 => 0, 701 => 0, 702 => 0)
);

$result['battle_cases'][] = run_rapid_fire_toggle_case();
$result['battle_cases'][] = run_defense_repair_writeback_case();

$result['spy_cases'][] = run_spy_case(
    'resources_only_level_0',
    0,
    0,
    1,
    array(),
    array(),
    array(),
    array()
);

$result['spy_cases'][] = run_spy_case(
    'fleet_and_defence_level_2',
    0,
    0,
    3,
    array(GID_F_LF => 2, GID_F_SC => 1),
    array(GID_D_RL => 3, GID_D_LL => 2),
    array(),
    array()
);

$result['spy_cases'][] = run_spy_case(
    'full_report_level_6_plus',
    10,
    2,
    1,
    array(GID_F_LF => 2, GID_F_SC => 1),
    array(GID_D_RL => 3, GID_D_LL => 2),
    array(GID_B_METAL_MINE => 12, GID_B_CRYS_MINE => 10, GID_B_DEUT_SYNTH => 8, GID_B_SHIPYARD => 5, GID_B_RES_LAB => 4),
    array(109 => 3, 110 => 2, 111 => 4, 115 => 6, 117 => 4)
);

$result['spy_cases'][] = run_spy_case(
    'detected_probe_destroyed',
    0,
    10,
    1,
    array(GID_F_LF => 100),
    array(GID_D_RL => 20),
    array(),
    array(109 => 5, 110 => 5, 111 => 5)
);

cleanup_runtime(false);

$noise = trim(ob_get_clean());
$result['captured_output'] = $noise;

$allPass = true;
foreach (array_merge($result['battle_cases'], $result['spy_cases']) as $case) {
    foreach ($case['checks'] as $check) {
        if (!$check['pass']) $allPass = false;
    }
}
$result['all_pass'] = $allPass;

echo json_encode($result, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
