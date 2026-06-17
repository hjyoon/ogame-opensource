<?php

ob_start();
error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_recovery_bulk_journey_e2e.php';
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
loca_add('statistics', 'en');
loca_add('admin', 'en');

function e2e_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

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

function e2e_http_request(string $method, string $url, array $data = array(), array &$cookies = array()): array
{
    $headers = array('User-Agent: ogame-e2e');
    if (!empty($cookies)) {
        $pairs = array();
        foreach ($cookies as $name => $value) {
            $pairs[] = $name . '=' . $value;
        }
        $headers[] = 'Cookie: ' . implode('; ', $pairs);
    }

    $content = null;
    if ($method === 'POST') {
        $content = http_build_query($data);
        $headers[] = 'Content-Type: application/x-www-form-urlencoded';
        $headers[] = 'Content-Length: ' . strlen($content);
    }

    $started = microtime(true);
    $context = stream_context_create(array(
        'http' => array(
            'method' => $method,
            'header' => implode("\r\n", $headers),
            'content' => $content,
            'ignore_errors' => true,
            'timeout' => 15,
            'follow_location' => 0,
        ),
    ));

    $body = file_get_contents($url, false, $context);
    $elapsedMs = (int)round((microtime(true) - $started) * 1000);
    $responseHeaders = $http_response_header ?? array();
    $status = 0;
    $location = '';
    foreach ($responseHeaders as $header) {
        if (preg_match('/^HTTP\/\S+\s+(\d+)/', $header, $m)) {
            $status = (int)$m[1];
        } elseif (stripos($header, 'Location:') === 0) {
            $location = trim(substr($header, 9));
        } elseif (stripos($header, 'Set-Cookie:') === 0) {
            $cookie = trim(substr($header, 11));
            $cookiePart = explode(';', $cookie, 2)[0];
            $kv = explode('=', $cookiePart, 2);
            if (count($kv) === 2) {
                $cookies[$kv[0]] = $kv[1];
            }
        }
    }

    return array(
        'status' => $status,
        'location' => $location,
        'headers' => $responseHeaders,
        'body' => $body === false ? '' : $body,
        'elapsed_ms' => $elapsedMs,
    );
}

function e2e_response_check(array $response, string $label = 'HTTP response'): array
{
    $body = $response['body'];
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined/i', $body) === 1;
    return array(
        e2e_case(in_array($response['status'], array(200, 301, 302, 303), true), "{$label} returns an accepted status", array('status' => $response['status'], 'location' => $response['location'], 'elapsed_ms' => $response['elapsed_ms'])),
        e2e_case(!$hasError, "{$label} body has no PHP error marker"),
    );
}

function e2e_prepare_session(int $userId, string $label, int $admin = USER_TYPE_PLAYER): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    e2e_sql_exec(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', admin={$admin}, " .
        "validated=1, validatemd='', deact_ip=1, lang='en', skin='/evolution/', useskin=1, " .
        "vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, " .
        "disable=0, disable_until=0 WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookies' => array('prsess_' . $userId . '_' . $GlobalUni['num'] => $private),
    );
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

function e2e_queue_type_sql(): string
{
    return "'" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "','" . QTYP_RECALC_POINTS . "','" . QTYP_DEBUG . "'";
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
        "lang='en', skin='/evolution/', useskin=1, score1=1000000, score2=0, score3=0 " .
        "WHERE player_id={$userId}"
    );

    SetPlanetFleetDefense($planetId, e2e_zero_map(array_merge(array_diff($defmap, $rakmap), $fleetmap)));
    SetPlanetBuildings($planetId, e2e_zero_map($buildmap));

    $now = time();
    e2e_sql_exec(
        "UPDATE {$db_prefix}planets SET " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "`" . GID_B_SOLAR . "`=10, `" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=10, `" . GID_B_RES_LAB . "`=10, `" . GID_B_NANITES . "`=0, " .
        "`" . GID_F_SC . "`=20, `" . GID_F_LC . "`=20, `" . GID_F_LF . "`=20, `" . GID_F_BATTLESHIP . "`=20, `" . GID_F_RECYCLER . "`=10, `" . GID_F_PROBE . "`=10, " .
        "`" . GID_D_ABM . "`=0, `" . GID_D_IPM . "`=0, prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, " .
        "fields=0, maxfields=300, type=" . PTYP_PLANET . ", owner_id={$userId}, remove=0, lastpeek={$now}, lastakt={$now} " .
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
}

function e2e_force_queue_due(int $taskId, int $now): void
{
    global $db_prefix;

    $queue = LoadQueue($taskId);
    if ($queue === null || $queue === false) {
        return;
    }

    $start = $now - 20;
    $end = $now - 10;
    if ($queue['type'] === QTYP_SHIPYARD) {
        $start = $now - max(20, ((int)$queue['level'] + 1));
        $end = $start + 1;
    }

    e2e_sql_exec("UPDATE {$db_prefix}queue SET start={$start}, end={$end}, freeze=0, frozen=0 WHERE task_id={$taskId}");
    if ($queue['type'] === QTYP_BUILD || $queue['type'] === QTYP_DEMOLISH) {
        e2e_sql_exec("UPDATE {$db_prefix}buildqueue SET start={$start}, end={$end} WHERE id=" . (int)$queue['sub_id']);
    } elseif ($queue['type'] === QTYP_FLEET) {
        e2e_sql_exec("UPDATE {$db_prefix}fleet SET flight_time=5 WHERE fleet_id=" . (int)$queue['sub_id']);
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

function e2e_run_worker_once(int $until): array
{
    $root = getenv('OGAME_E2E_ROOT') ?: '/tmp/ogame-e2e';
    $script = $root . '/update_queue_worker_once.php';
    $pipes = array();
    $proc = proc_open(
        array('php', $script, (string)$until),
        array(
            0 => array('pipe', 'r'),
            1 => array('pipe', 'w'),
            2 => array('pipe', 'w'),
        ),
        $pipes
    );
    if (!is_resource($proc)) {
        return array('started' => false, 'exit_code' => -1, 'stdout' => '', 'stderr' => 'proc_open failed', 'json' => null);
    }
    fclose($pipes[0]);
    $stdout = stream_get_contents($pipes[1]);
    $stderr = stream_get_contents($pipes[2]);
    fclose($pipes[1]);
    fclose($pipes[2]);
    $exit = proc_close($proc);
    $decoded = json_decode(trim($stdout), true);

    return array(
        'started' => true,
        'exit_code' => $exit,
        'stdout' => trim($stdout),
        'stderr' => trim($stderr),
        'json' => is_array($decoded) ? $decoded : null,
    );
}

function e2e_drain_with_fresh_workers(int $until, array $userIds, int $limit = 20): array
{
    $runs = array();
    for ($i = 0; $i < $limit; $i++) {
        $due = e2e_due_queue_count($until, $userIds);
        if ($due === 0) {
            break;
        }
        $worker = e2e_run_worker_once($until);
        $worker['fixture_due_before'] = $due;
        $worker['fixture_due_after'] = e2e_due_queue_count($until, $userIds);
        $runs[] = $worker;
        if (($worker['exit_code'] ?? -1) !== 0) {
            break;
        }
    }

    return array(
        'runs' => $runs,
        'remaining_due' => e2e_due_queue_count($until, $userIds),
    );
}

function e2e_latest_queue(int $ownerId, string $type): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT * FROM {$db_prefix}queue WHERE owner_id={$ownerId} AND type='" . $type . "' ORDER BY task_id DESC LIMIT 1");
}

function e2e_planet_snapshot(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, g, s, p, type, " .
        "`" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deuterium, " .
        "`" . GID_B_METAL_MINE . "` AS metal_mine, `" . GID_F_SC . "` AS small_cargo, `" . GID_F_LC . "` AS large_cargo, " .
        "`" . GID_F_LF . "` AS light_fighter, `" . GID_F_BATTLESHIP . "` AS battleship, `" . GID_F_RECYCLER . "` AS recycler, " .
        "`" . GID_D_RL . "` AS rocket_launcher " .
        "FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1"
    );
}

function e2e_user_research_snapshot(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT player_id, `" . GID_R_ESPIONAGE . "` AS espionage FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1");
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

function e2e_fleet_payload(array $origin, array $target, int $order, array $ships, array $resources = array(), array $extra = array()): array
{
    global $fleetmap, $transportableResources, $GlobalUni;

    $data = array(
        'thisgalaxy' => (string)$origin['g'],
        'thissystem' => (string)$origin['s'],
        'thisplanet' => (string)$origin['p'],
        'thisplanettype' => (string)GetPlanetType($origin),
        'speedfactor' => (string)$GlobalUni['fspeed'],
        'galaxy' => (string)$target['g'],
        'system' => (string)$target['s'],
        'planet' => (string)$target['p'],
        'planettype' => (string)GetPlanetType($target),
        'speed' => '10',
        'order' => (string)$order,
    );
    foreach ($fleetmap as $gid) {
        $data['ship' . $gid] = (string)($ships[$gid] ?? 0);
    }
    foreach ($transportableResources as $i => $rc) {
        $data['resource' . ($i + 1)] = (string)($resources[$rc] ?? 0);
    }
    foreach ($extra as $key => $value) {
        $data[$key] = (string)$value;
    }
    return $data;
}

function e2e_latest_battle_report(int $ownerId, int $afterMsgId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT msg_id, owner_id, subj, text FROM {$db_prefix}messages " .
        "WHERE owner_id={$ownerId} AND msg_id>{$afterMsgId} AND pm=" . MTYP_BATTLE_REPORT_TEXT . " " .
        "ORDER BY msg_id DESC LIMIT 1"
    );
}

function e2e_max_msg_id(): int
{
    global $db_prefix;
    $row = e2e_one_row("SELECT COALESCE(MAX(msg_id), 0) AS id FROM {$db_prefix}messages");
    return $row === null ? 0 : (int)$row['id'];
}

function e2e_find_debris_at_planet(int $planetId): ?array
{
    global $db_prefix;
    $planet = LoadPlanetById($planetId);
    if ($planet === null || $planet === false) {
        return null;
    }
    return e2e_one_row(
        "SELECT planet_id, owner_id, g, s, p, type, `" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal " .
        "FROM {$db_prefix}planets WHERE g=" . (int)$planet['g'] . " AND s=" . (int)$planet['s'] . " AND p=" . (int)$planet['p'] . " AND type=" . PTYP_DF . " LIMIT 1"
    );
}

function e2e_set_planet_for_battle(int $attackerPlanet, int $defenderPlanet): void
{
    global $db_prefix, $fleetmap, $defmap, $rakmap;

    SetPlanetFleetDefense($attackerPlanet, e2e_with_units(e2e_zero_map(array_merge(array_diff($defmap, $rakmap), $fleetmap)), array(
        GID_F_LC => 2,
        GID_F_BATTLESHIP => 10,
        GID_F_RECYCLER => 5,
    )));
    SetPlanetFleetDefense($defenderPlanet, e2e_with_units(e2e_zero_map(array_merge(array_diff($defmap, $rakmap), $fleetmap)), array(
        GID_F_LF => 40,
        GID_D_RL => 30,
    )));
    e2e_sql_exec(
        "UPDATE {$db_prefix}planets SET `" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000 " .
        "WHERE planet_id={$attackerPlanet}"
    );
    e2e_sql_exec(
        "UPDATE {$db_prefix}planets SET `" . GID_RC_METAL . "`=500000, `" . GID_RC_CRYSTAL . "`=300000, `" . GID_RC_DEUTERIUM . "`=100000 " .
        "WHERE planet_id={$defenderPlanet}"
    );
}

function e2e_remove_bulk_users(): void
{
    global $db_prefix;
    $ids = array();
    $res = e2e_sql_exec("SELECT player_id FROM {$db_prefix}users WHERE name LIKE 'e2ebulk_%'");
    while ($row = dbarray($res)) {
        $ids[] = (int)$row['player_id'];
    }
    foreach ($ids as $id) {
        RemoveUser($id, time());
    }
}

function e2e_find_free_coordinate_block(int $needed): array
{
    global $GlobalUni;

    for ($g = 1; $g <= (int)$GlobalUni['galaxies']; $g++) {
        for ($s = 1; $s <= (int)$GlobalUni['systems']; $s++) {
            $coords = array();
            for ($p = 1; $p <= 15; $p++) {
                if (!HasPlanet($g, $s, $p)) {
                    $coords[] = array($g, $s, $p);
                    if (count($coords) >= $needed) {
                        return $coords;
                    }
                }
            }
        }
    }

    throw new RuntimeException('No free coordinate block for bulk smoke users.');
}

function e2e_create_bulk_users(int $count): array
{
    global $db_prefix;

    e2e_remove_bulk_users();
    $coords = e2e_find_free_coordinate_block(min(12, $count));
    $created = array();
    for ($i = 0; $i < $count; $i++) {
        $name = 'e2ebulk_' . str_pad((string)$i, 2, '0', STR_PAD_LEFT);
        $existing = e2e_one_row("SELECT player_id FROM {$db_prefix}users WHERE name='" . e2e_sql_escape($name) . "' LIMIT 1");
        if ($existing !== null) {
            RemoveUser((int)$existing['player_id'], time());
        }
        $id = CreateUser($name, 'E2E_test123', $name . '@example.local', false);
        $user = e2e_one_row("SELECT player_id, hplanetid FROM {$db_prefix}users WHERE player_id={$id} LIMIT 1");
        if ($user === null) {
            throw new RuntimeException("Failed to create bulk user {$name}");
        }
        e2e_sql_exec(
            "UPDATE {$db_prefix}users SET validated=1, validatemd='', deact_ip=1, lang='en', skin='/evolution/', useskin=1, " .
            "score1=" . (1000 + $i * 7) . ", score2=" . (10 + $i) . ", score3=" . (5 + $i) . " WHERE player_id={$id}"
        );
        if ($i < count($coords)) {
            [$g, $s, $p] = $coords[$i];
            e2e_sql_exec("UPDATE {$db_prefix}planets SET g={$g}, s={$s}, p={$p}, type=" . PTYP_PLANET . " WHERE planet_id=" . (int)$user['hplanetid']);
        }
        AddQueue($id, QTYP_DEBUG, 0, 0, 0, time(), 3600 + $i, QUEUE_PRIO_DEBUG);
        $created[] = array('player_id' => $id, 'planet_id' => (int)$user['hplanetid'], 'name' => $name);
    }
    RecalcRanks();

    return $created;
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderPlanet = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$cases = array();

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $users = array($attackerId, $defenderId);
    $planets = array($attackerPlanet, $defenderPlanet);
    $now = time();

    e2e_reset_fixtures($attackerId, $attackerPlanet, $defenderId, $defenderPlanet);
    e2e_sql_exec("UPDATE {$db_prefix}users SET `" . GID_R_ESPIONAGE . "`=0 WHERE player_id={$attackerId}");
    $originBefore = e2e_planet_snapshot($attackerPlanet);
    $targetBefore = e2e_planet_snapshot($defenderPlanet);

    $buildText = BuildEnque(LoadUser($attackerId), $attackerPlanet, GID_B_METAL_MINE, 0, $now);
    $buildTask = e2e_latest_queue($attackerId, QTYP_BUILD);
    if ($buildTask !== null) {
        e2e_force_queue_due((int)$buildTask['task_id'], $now);
    }

    $researchText = StartResearch($attackerId, $attackerPlanet, GID_R_ESPIONAGE, $now + 1);
    $researchTask = e2e_latest_queue($attackerId, QTYP_RESEARCH);
    if ($researchTask !== null) {
        e2e_force_queue_due((int)$researchTask['task_id'], $now);
    }

    $shipyardOk = AddShipyard($attackerId, $attackerPlanet, GID_F_LF, 3, $now + 2);
    $shipyardTask = e2e_latest_queue($attackerId, QTYP_SHIPYARD);
    if ($shipyardTask !== null) {
        e2e_force_queue_due((int)$shipyardTask['task_id'], $now);
    }

    [$transportFleetId, $transportQueue] = e2e_dispatch_transport(
        $attackerPlanet,
        $defenderPlanet,
        array(GID_RC_METAL => 321, GID_RC_CRYSTAL => 123, GID_RC_DEUTERIUM => 45),
        1,
        5,
        $now - 20
    );
    if ($transportQueue !== null && $transportQueue !== false) {
        e2e_force_queue_due((int)$transportQueue['task_id'], $now);
    }

    $preparedDue = e2e_due_queue_count($now, $users);
    $freshDrain = e2e_drain_with_fresh_workers($now, $users);
    $originAfterRecovery = e2e_planet_snapshot($attackerPlanet);
    $targetAfterRecovery = e2e_planet_snapshot($defenderPlanet);
    $userAfterRecovery = e2e_user_research_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'fresh_worker_recovers_persisted_build_research_shipyard_and_fleet_queues',
        'worker_drain' => $freshDrain,
        'checks' => array(
            e2e_case($buildText === '' && $buildTask !== null, 'build queue is persisted before fresh-worker drain', array('message' => $buildText, 'queue' => $buildTask ?? array())),
            e2e_case($researchText === '' && $researchTask !== null, 'research queue is persisted before fresh-worker drain', array('message' => $researchText, 'queue' => $researchTask ?? array())),
            e2e_case($shipyardOk && $shipyardTask !== null, 'shipyard queue is persisted before fresh-worker drain', array('queue' => $shipyardTask ?? array())),
            e2e_case($transportFleetId > 0 && $transportQueue !== null && $transportQueue !== false, 'fleet queue is persisted before fresh-worker drain', array('fleet_id' => $transportFleetId, 'queue' => $transportQueue ?: array())),
            e2e_case($preparedDue >= 4, 'fixture has multiple due queues before fresh-worker recovery', array('prepared_due' => $preparedDue)),
            e2e_case(($freshDrain['remaining_due'] ?? 1) === 0 && count($freshDrain['runs']) >= 1, 'fresh PHP workers drain all due fixture queues', $freshDrain),
            e2e_case($originAfterRecovery !== null && (int)$originAfterRecovery['metal_mine'] === 1, 'fresh worker completes persisted building queue', $originAfterRecovery ?? array()),
            e2e_case($userAfterRecovery !== null && (int)$userAfterRecovery['espionage'] === 1, 'fresh worker completes persisted research queue', $userAfterRecovery ?? array()),
            e2e_case($originBefore !== null && $originAfterRecovery !== null && (int)$originAfterRecovery['light_fighter'] === (int)$originBefore['light_fighter'] + 3, 'fresh worker completes persisted shipyard queue', array('before' => $originBefore, 'after' => $originAfterRecovery)),
            e2e_case($targetBefore !== null && $targetAfterRecovery !== null && (int)$targetAfterRecovery['metal'] === (int)$targetBefore['metal'] + 321, 'fresh worker delivers persisted transport cargo', array('before' => $targetBefore, 'after' => $targetAfterRecovery)),
            e2e_case($originBefore !== null && $originAfterRecovery !== null && (int)$originAfterRecovery['small_cargo'] === (int)$originBefore['small_cargo'], 'fresh worker returns persisted transport ship', array('before' => $originBefore, 'after' => $originAfterRecovery)),
            e2e_case(e2e_fixture_queue_count($users) === 0, 'fresh-worker recovery leaves no fixture queue rows'),
            e2e_case(e2e_fixture_fleet_count($users, $planets) === 0, 'fresh-worker recovery leaves no fixture fleet rows'),
        ),
    ));

    e2e_reset_fixtures($attackerId, $attackerPlanet, $defenderId, $defenderPlanet);
    e2e_set_planet_for_battle($attackerPlanet, $defenderPlanet);
    $auth = e2e_prepare_session($attackerId, 'http-core-journey');
    $cookies = $auth['cookies'];
    $session = $auth['session'];

    $overviewBefore = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet, array(), $cookies);
    $buildResponse = e2e_http_request('GET', $gameBase . '/index.php?page=b_building&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&modus=add&techid=' . GID_B_METAL_MINE . '&planet=' . $attackerPlanet, array(), $cookies);
    $httpBuildTask = e2e_latest_queue($attackerId, QTYP_BUILD);
    if ($httpBuildTask !== null) {
        e2e_force_queue_due((int)$httpBuildTask['task_id'], $now);
        e2e_drain_with_fresh_workers($now, $users);
    }
    $afterHttpBuild = e2e_planet_snapshot($attackerPlanet);

    $battleReportAfter = e2e_max_msg_id();
    $originForAttack = LoadPlanetById($attackerPlanet);
    $targetForAttack = LoadPlanetById($defenderPlanet);
    $attackResponse = e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet,
        e2e_fleet_payload($originForAttack, $targetForAttack, FTYP_ATTACK, array(GID_F_BATTLESHIP => 10, GID_F_LC => 2)),
        $cookies
    );
    e2e_force_fixture_fleet_queues_due($users, $now);
    $battleDrain = e2e_drain_with_fresh_workers($now, $users);
    $battleReport = e2e_latest_battle_report($attackerId, $battleReportAfter);
    $battleReportResponse = $battleReport === null
        ? array('status' => 0, 'location' => '', 'headers' => array(), 'body' => '', 'elapsed_ms' => 0)
        : e2e_http_request('GET', $gameBase . '/index.php?page=bericht&session=' . rawurlencode($session) . '&bericht=' . (int)$battleReport['msg_id'], array(), $cookies);

    $debrisBeforeRecycle = e2e_find_debris_at_planet($defenderPlanet);
    $recycleResponse = array('status' => 0, 'location' => '', 'headers' => array(), 'body' => '', 'elapsed_ms' => 0);
    $recycleDrain = array('runs' => array(), 'remaining_due' => -1);
    $debrisAfterRecycle = null;
    $recyclerBefore = e2e_planet_snapshot($attackerPlanet);
    if ($debrisBeforeRecycle !== null && ((int)$debrisBeforeRecycle['metal'] + (int)$debrisBeforeRecycle['crystal']) > 0) {
        $originForRecycle = LoadPlanetById($attackerPlanet);
        $targetDebris = LoadPlanetById((int)$debrisBeforeRecycle['planet_id']);
        $recycleResponse = e2e_http_request(
            'POST',
            $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet,
            e2e_fleet_payload($originForRecycle, $targetDebris, FTYP_RECYCLE, array(GID_F_RECYCLER => 5)),
            $cookies
        );
        e2e_force_fixture_fleet_queues_due($users, $now);
        $recycleDrain = e2e_drain_with_fresh_workers($now, $users);
        $debrisAfterRecycle = e2e_find_debris_at_planet($defenderPlanet);
    }
    $recyclerAfter = e2e_planet_snapshot($attackerPlanet);
    $messagesResponse = e2e_http_request('GET', $gameBase . '/index.php?page=messages&session=' . rawurlencode($session), array(), $cookies);

    $debrisBeforeTotal = $debrisBeforeRecycle === null ? 0 : (int)$debrisBeforeRecycle['metal'] + (int)$debrisBeforeRecycle['crystal'];
    $debrisAfterTotal = $debrisAfterRecycle === null ? 0 : (int)$debrisAfterRecycle['metal'] + (int)$debrisAfterRecycle['crystal'];
    $cases[] = e2e_finalize_case(array(
        'case' => 'http_core_journey_build_attack_report_and_recycle',
        'battle_drain' => $battleDrain,
        'recycle_drain' => $recycleDrain,
        'checks' => array_merge(
            e2e_response_check($overviewBefore, 'overview before HTTP journey'),
            e2e_response_check($buildResponse, 'HTTP building enqueue'),
            e2e_response_check($attackResponse, 'HTTP attack dispatch'),
            e2e_response_check($battleReportResponse, 'HTTP battle report document'),
            e2e_response_check($recycleResponse, 'HTTP recycle dispatch'),
            e2e_response_check($messagesResponse, 'messages after HTTP journey'),
            array(
                e2e_case($httpBuildTask !== null && $afterHttpBuild !== null && (int)$afterHttpBuild['metal_mine'] === 1, 'HTTP building action creates and completes a build queue', array('queue' => $httpBuildTask ?? array(), 'planet' => $afterHttpBuild ?? array())),
                e2e_case(($battleDrain['remaining_due'] ?? 1) === 0, 'fresh worker drains HTTP attack and return queues', $battleDrain),
                e2e_case($battleReport !== null && strlen($battleReport['text'] ?? '') > 100, 'HTTP attack generates a battle report message', $battleReport === null ? array() : array('msg_id' => (int)$battleReport['msg_id'], 'text_len' => strlen($battleReport['text'] ?? ''))),
                e2e_case($battleReportResponse['status'] === 200 && stripos($battleReportResponse['body'], 'Battle') !== false, 'battle report opens as a rendered document', array('status' => $battleReportResponse['status'])),
                e2e_case($debrisBeforeRecycle !== null && $debrisBeforeTotal > 0, 'HTTP attack creates recyclable debris', $debrisBeforeRecycle ?? array()),
                e2e_case(($recycleDrain['remaining_due'] ?? 1) === 0, 'fresh worker drains HTTP recycle and return queues', $recycleDrain),
                e2e_case($debrisAfterTotal < $debrisBeforeTotal, 'HTTP recycle mission harvests debris', array('before' => $debrisBeforeRecycle, 'after' => $debrisAfterRecycle)),
                e2e_case($recyclerBefore !== null && $recyclerAfter !== null && (int)$recyclerAfter['recycler'] === (int)$recyclerBefore['recycler'], 'recycle return restores recycler ships', array('before' => $recyclerBefore, 'after' => $recyclerAfter)),
                e2e_case(e2e_fixture_queue_count($users) === 0, 'HTTP journey leaves no fixture queue rows'),
                e2e_case(e2e_fixture_fleet_count($users, $planets) === 0, 'HTTP journey leaves no fixture fleet rows'),
            )
        ),
    ));

    e2e_reset_fixtures($attackerId, $attackerPlanet, $defenderId, $defenderPlanet);
    $bulkUsers = e2e_create_bulk_users(28);
    $bulkAuth = e2e_prepare_session($attackerId, 'bulk-player');
    $bulkCookies = $bulkAuth['cookies'];
    $firstBulkPlanet = $bulkUsers[0] ?? null;
    $bulkUserIds = array_map(fn($row) => (int)$row['player_id'], $bulkUsers);
    $bulkUserList = implode(',', $bulkUserIds);
    $firstBulkPlanetRow = $firstBulkPlanet === null ? null : LoadPlanetById((int)$firstBulkPlanet['planet_id']);
    $firstBulkRank = $firstBulkPlanet === null ? null : e2e_one_row("SELECT place1 FROM {$db_prefix}users WHERE player_id=" . (int)$firstBulkPlanet['player_id'] . " LIMIT 1");
    $statsStart = $firstBulkRank === null ? 1 : max(1, ((int)floor(((int)$firstBulkRank['place1'] - 1) / 100) * 100) + 1);
    $galaxyQuery = $firstBulkPlanetRow === null
        ? '&galaxy=1&system=1'
        : '&galaxy=' . (int)$firstBulkPlanetRow['g'] . '&system=' . (int)$firstBulkPlanetRow['s'];
    $seededSystemPlanets = $firstBulkPlanetRow === null || empty($bulkUserIds)
        ? 0
        : e2e_count(
            "SELECT COUNT(*) AS cnt FROM {$db_prefix}planets " .
            "WHERE owner_id IN ({$bulkUserList}) AND type=" . PTYP_PLANET . " " .
            "AND g=" . (int)$firstBulkPlanetRow['g'] . " AND s=" . (int)$firstBulkPlanetRow['s']
        );

    $bulkOverview = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($bulkAuth['session']) . '&cp=' . $attackerPlanet, array(), $bulkCookies);
    $bulkGalaxy = e2e_http_request('GET', $gameBase . '/index.php?page=galaxy&no_header=1&session=' . rawurlencode($bulkAuth['session']) . '&cp=' . $attackerPlanet . $galaxyQuery, array(), $bulkCookies);
    $bulkStats = e2e_http_request('GET', $gameBase . '/index.php?page=statistics&session=' . rawurlencode($bulkAuth['session']) . '&start=' . $statsStart . '&type=ressources', array(), $bulkCookies);
    $adminAuth = e2e_prepare_session($attackerId, 'bulk-admin', USER_TYPE_ADMIN);
    $adminCookies = $adminAuth['cookies'];
    $bulkAdminQueue = e2e_http_request('GET', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Queue', array(), $adminCookies);
    $bulkQueueRows = empty($bulkUserIds) ? 0 : e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id IN ({$bulkUserList})");
    $bulkNamesInStats = 0;
    foreach ($bulkUsers as $row) {
        if (strpos($bulkStats['body'], $row['name']) !== false) {
            $bulkNamesInStats++;
        }
    }
    $bulkNamesInGalaxy = 0;
    foreach ($bulkUsers as $row) {
        if (strpos($bulkGalaxy['body'], $row['name']) !== false) {
            $bulkNamesInGalaxy++;
        }
    }

    $cases[] = e2e_finalize_case(array(
        'case' => 'bulk_data_pages_render_under_bounded_load',
        'bulk_user_count' => count($bulkUsers),
        'checks' => array_merge(
            e2e_response_check($bulkOverview, 'bulk overview page'),
            e2e_response_check($bulkGalaxy, 'bulk galaxy page'),
            e2e_response_check($bulkStats, 'bulk statistics page'),
            e2e_response_check($bulkAdminQueue, 'bulk admin queue page'),
            array(
                e2e_case(count($bulkUsers) === 28, 'bulk smoke creates the expected number of temporary users', array('created' => count($bulkUsers))),
                e2e_case($bulkQueueRows === 28, 'bulk smoke creates one queued task per temporary user', array('queue_rows' => $bulkQueueRows)),
                e2e_case($bulkOverview['elapsed_ms'] < 10000 && $bulkGalaxy['elapsed_ms'] < 10000 && $bulkStats['elapsed_ms'] < 10000 && $bulkAdminQueue['elapsed_ms'] < 10000, 'bulk pages render before the HTTP timeout budget', array('overview_ms' => $bulkOverview['elapsed_ms'], 'galaxy_ms' => $bulkGalaxy['elapsed_ms'], 'statistics_ms' => $bulkStats['elapsed_ms'], 'admin_queue_ms' => $bulkAdminQueue['elapsed_ms'])),
                e2e_case($bulkNamesInStats >= 5, 'statistics page includes multiple temporary users under load', array('names_found' => $bulkNamesInStats, 'start' => $statsStart, 'first_bulk_rank' => $firstBulkRank ?? array())),
                e2e_case($seededSystemPlanets >= 5 && strlen($bulkGalaxy['body']) > 1000, 'galaxy page renders a seeded multi-player system under load', array('seeded_system_planets' => $seededSystemPlanets, 'names_found' => $bulkNamesInGalaxy, 'query' => $galaxyQuery, 'body_len' => strlen($bulkGalaxy['body']))),
                e2e_case(strlen($bulkStats['body']) > 1000 && strlen($bulkAdminQueue['body']) > 1000, 'statistics and admin queue pages render substantial documents under load', array('statistics_body_len' => strlen($bulkStats['body']), 'admin_queue_body_len' => strlen($bulkAdminQueue['body']))),
                e2e_case(stripos($bulkAdminQueue['body'], 'Queue') !== false || stripos($bulkAdminQueue['body'], 'Task') !== false || stripos($bulkAdminQueue['body'], QTYP_DEBUG) !== false, 'admin queue page renders queue content under load'),
            )
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'recovery_bulk_journey_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    e2e_remove_bulk_users();
    if ($attackerId > 0 && $attackerPlanet > 0 && $defenderId > 0 && $defenderPlanet > 0) {
        e2e_reset_fixtures($attackerId, $attackerPlanet, $defenderId, $defenderPlanet);
    }
}

$noise = trim(ob_get_clean());

echo json_encode(array(
    'case_group' => 'http_recovery_bulk_journey',
    'base' => $base,
    'cases' => $cases,
    'captured_output' => $noise,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
