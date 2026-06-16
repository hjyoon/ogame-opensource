<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_stats_ranking_e2e.php';
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
loca_add('statistics', 'en');
loca_add('debug', 'en');

function e2e_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
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
    );
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

function e2e_response_check(array $response): array
{
    $body = $response['body'];
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined/i', $body) === 1;
    return array(
        e2e_case(in_array($response['status'], array(200, 301, 302, 303), true), 'HTTP request returns an accepted status', array('status' => $response['status'], 'location' => $response['location'])),
        e2e_case(!$hasError, 'HTTP body has no PHP error marker'),
    );
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

function e2e_prepare_session(int $userId, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', validated=1, deact_ip=1, " .
        "lang='en', skin='/evolution/', useskin=1, vacation=0, vacation_until=0, " .
        "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0 " .
        "WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookies' => array('prsess_' . $userId . '_' . $GlobalUni['num'] => $private),
    );
}

function e2e_cleanup_runtime(array $userIds, array $planetIds): void
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

    dbquery(
        "DELETE FROM {$db_prefix}queue WHERE owner_id IN ({$userList}) AND type IN ('" .
        QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" .
        QTYP_SHIPYARD . "','" . QTYP_FLEET . "','" . QTYP_DEBUG . "')"
    );
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id IN ({$userList}) OR planet_id IN ({$planetList})");
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
        "`" . GID_D_ABM . "`=0, `" . GID_D_IPM . "`=0, " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, " .
        "fields=0, maxfields=300, type=" . PTYP_PLANET . ", owner_id={$userId}, lastpeek=" . time() . " " .
        "WHERE planet_id={$planetId}"
    );
}

function e2e_reset_fixture(array $userIds, array $planetIds): void
{
    global $db_prefix;

    e2e_cleanup_runtime($userIds, $planetIds);
    foreach ($userIds as $userId) {
        e2e_set_user_research((int)$userId, array());
        dbquery(
            "UPDATE {$db_prefix}users SET score1=0, score2=0, score3=0, " .
            "oldscore1=0, oldscore2=0, oldscore3=0, place1=0, place2=0, place3=0, " .
            "oldplace1=0, oldplace2=0, oldplace3=0, scoredate=0 WHERE player_id=" . intval($userId)
        );
    }
    foreach ($planetIds as $idx => $planetId) {
        $userId = (int)$userIds[$idx];
        e2e_set_planet_layout((int)$planetId, $userId, array());
    }
}

function e2e_force_complete_queue(int $taskId): void
{
    global $db_prefix;

    $now = time();
    $start = $now - 10;
    $end = $start + 1;
    dbquery("UPDATE {$db_prefix}queue SET start={$start}, end={$end}, freeze=0, frozen=0 WHERE task_id={$taskId}");
    dbquery("UPDATE {$db_prefix}buildqueue SET start={$start}, end={$end} WHERE id = ANY (SELECT sub_id FROM {$db_prefix}queue WHERE task_id={$taskId})");

    for ($i = 0; $i < 4 && e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE task_id={$taskId}") > 0; $i++) {
        UpdateQueue($now);
    }
}

function e2e_user_scores(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT player_id, score1, score2, score3, place1, place2, place3 FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1");
}

function e2e_research_score(array $levels): array
{
    $points = 0;
    $rpoints = 0;
    foreach ($levels as $gid => $level) {
        $rpoints += (int)$level;
        for ($lv = 1; $lv <= (int)$level; $lv++) {
            $points += TechPriceInPoints(TechPrice((int)$gid, $lv));
        }
    }
    return array('points' => $points, 'rpoints' => $rpoints);
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderPlanet = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 0);
$attackerName = getenv('OGAME_E2E_ATTACKER_NAME') ?: '';
$defenderName = getenv('OGAME_E2E_DEFENDER_NAME') ?: '';
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$cases = array();

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    e2e_reset_fixture(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    $research = array(GID_R_ESPIONAGE => 2, GID_R_COMPUTER => 1);
    e2e_set_user_research($attackerId, $research);
    e2e_set_planet_layout(
        $attackerPlanet,
        $attackerId,
        array(GID_B_METAL_MINE => 2, GID_B_SOLAR => 1),
        array(GID_F_SC => 2, GID_F_LF => 3),
        array(GID_D_RL => 4)
    );
    $planet = LoadPlanetById($attackerPlanet);
    $planetPrice = PlanetPrice($planet);
    $researchScore = e2e_research_score($research);
    RecalcStats($attackerId);
    $scores = e2e_user_scores($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'recalc_stats_counts_static_assets',
        'checks' => array(
            e2e_case($scores !== null && (int)$scores['score1'] === $planetPrice['points'] + $researchScore['points'], 'RecalcStats total points include buildings, fleet, defense, and research', array('scores' => $scores, 'planet_price' => $planetPrice, 'research_score' => $researchScore)),
            e2e_case($scores !== null && (int)$scores['score2'] === $planetPrice['fpoints'], 'RecalcStats fleet score counts standing ships only', array('scores' => $scores, 'planet_price' => $planetPrice)),
            e2e_case($scores !== null && (int)$scores['score3'] === $researchScore['rpoints'], 'RecalcStats research score counts research levels', array('scores' => $scores, 'research_score' => $researchScore)),
        ),
    ));

    e2e_reset_fixture(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    $buildCost = TechPriceInPoints(TechPrice(GID_B_METAL_MINE, 1));
    $buildText = BuildEnque(LoadUser($attackerId), $attackerPlanet, GID_B_METAL_MINE, 0, time() + 1);
    $buildTask = e2e_one_row("SELECT task_id, obj_id, level FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "' ORDER BY task_id DESC LIMIT 1");
    if ($buildTask !== null) {
        e2e_force_complete_queue((int)$buildTask['task_id']);
    }
    $scores = e2e_user_scores($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'building_completion_adjusts_total_points',
        'checks' => array(
            e2e_case($buildText === '', 'building queue accepts metal mine construction', array('message' => $buildText)),
            e2e_case($buildTask !== null && (int)$buildTask['obj_id'] === GID_B_METAL_MINE, 'building queue task is created', $buildTask ?? array()),
            e2e_case($scores !== null && (int)$scores['score1'] === $buildCost, 'building completion adds construction cost to total score', array('scores' => $scores, 'expected' => $buildCost)),
            e2e_case($scores !== null && (int)$scores['score2'] === 0 && (int)$scores['score3'] === 0, 'building completion does not change fleet or research score', $scores ?? array()),
        ),
    ));

    e2e_reset_fixture(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    e2e_set_planet_layout($attackerPlanet, $attackerId, array(GID_B_RES_LAB => 3));
    $researchCost = TechPriceInPoints(TechPrice(GID_R_ESPIONAGE, 1));
    $researchText = StartResearch($attackerId, $attackerPlanet, GID_R_ESPIONAGE, time() + 2);
    $researchTask = e2e_one_row("SELECT task_id, obj_id, level FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_RESEARCH . "' ORDER BY task_id DESC LIMIT 1");
    if ($researchTask !== null) {
        e2e_force_complete_queue((int)$researchTask['task_id']);
    }
    $scores = e2e_user_scores($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'research_completion_adjusts_total_and_research_points',
        'checks' => array(
            e2e_case($researchText === '', 'research queue accepts espionage technology', array('message' => $researchText)),
            e2e_case($researchTask !== null && (int)$researchTask['obj_id'] === GID_R_ESPIONAGE && (int)$researchTask['level'] === 1, 'research queue task is created', $researchTask ?? array()),
            e2e_case($scores !== null && (int)$scores['score1'] === $researchCost, 'research completion adds research cost to total score', array('scores' => $scores, 'expected' => $researchCost)),
            e2e_case($scores !== null && (int)$scores['score3'] === 1, 'research completion increments research score by one level', $scores ?? array()),
        ),
    ));

    e2e_reset_fixture(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    e2e_set_user_research($attackerId, array(GID_R_COMBUST_DRIVE => 2));
    e2e_set_planet_layout($attackerPlanet, $attackerId, array(GID_B_SHIPYARD => 2));
    $shipCount = 3;
    $shipCost = TechPriceInPoints(TechPrice(GID_F_SC, 1)) * $shipCount;
    $shipyardOk = AddShipyard($attackerId, $attackerPlanet, GID_F_SC, $shipCount, time() + 3);
    $shipyardTask = e2e_one_row("SELECT task_id, obj_id, level FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_SHIPYARD . "' ORDER BY task_id DESC LIMIT 1");
    if ($shipyardTask !== null) {
        e2e_force_complete_queue((int)$shipyardTask['task_id']);
    }
    $scores = e2e_user_scores($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'shipyard_completion_adjusts_fleet_score',
        'checks' => array(
            e2e_case($shipyardOk === true, 'shipyard queue accepts small cargo construction'),
            e2e_case($shipyardTask !== null && (int)$shipyardTask['obj_id'] === GID_F_SC && (int)$shipyardTask['level'] === $shipCount, 'shipyard queue task is created for ships', $shipyardTask ?? array()),
            e2e_case($scores !== null && (int)$scores['score1'] === $shipCost, 'ship completion adds ship cost to total score', array('scores' => $scores, 'expected' => $shipCost)),
            e2e_case($scores !== null && (int)$scores['score2'] === $shipCount, 'ship completion increments fleet score by produced ship count', $scores ?? array()),
        ),
    ));

    e2e_reset_fixture(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    e2e_set_planet_layout($attackerPlanet, $attackerId, array(GID_B_SHIPYARD => 1));
    $defenseCount = 4;
    $defenseCost = TechPriceInPoints(TechPrice(GID_D_RL, 1)) * $defenseCount;
    $defenseOk = AddShipyard($attackerId, $attackerPlanet, GID_D_RL, $defenseCount, time() + 4);
    $defenseTask = e2e_one_row("SELECT task_id, obj_id, level FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_SHIPYARD . "' ORDER BY task_id DESC LIMIT 1");
    if ($defenseTask !== null) {
        e2e_force_complete_queue((int)$defenseTask['task_id']);
    }
    $scores = e2e_user_scores($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'defense_completion_adjusts_total_without_fleet_score',
        'checks' => array(
            e2e_case($defenseOk === true, 'shipyard queue accepts rocket launcher construction'),
            e2e_case($defenseTask !== null && (int)$defenseTask['obj_id'] === GID_D_RL && (int)$defenseTask['level'] === $defenseCount, 'shipyard queue task is created for defense', $defenseTask ?? array()),
            e2e_case($scores !== null && (int)$scores['score1'] === $defenseCost, 'defense completion adds defense cost to total score', array('scores' => $scores, 'expected' => $defenseCost)),
            e2e_case($scores !== null && (int)$scores['score2'] === 0, 'defense completion does not increment fleet score', $scores ?? array()),
        ),
    ));

    e2e_reset_fixture(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    dbquery("UPDATE {$db_prefix}users SET score1=900000000, score2=200, score3=40 WHERE player_id={$attackerId}");
    dbquery("UPDATE {$db_prefix}users SET score1=800000000, score2=100, score3=20 WHERE player_id={$defenderId}");
    RecalcRanks();
    $attackerBefore = e2e_user_scores($attackerId);
    $defenderBefore = e2e_user_scores($defenderId);

    dbquery("UPDATE {$db_prefix}users SET score1=950000000, score2=300, score3=50 WHERE player_id={$defenderId}");
    RecalcRanks();
    $attackerAfter = e2e_user_scores($attackerId);
    $defenderAfter = e2e_user_scores($defenderId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'recalc_ranks_orders_fixture_users',
        'checks' => array(
            e2e_case($attackerBefore !== null && $defenderBefore !== null && (int)$attackerBefore['place1'] < (int)$defenderBefore['place1'], 'higher total score ranks ahead before score swap', array('attacker' => $attackerBefore, 'defender' => $defenderBefore)),
            e2e_case($attackerBefore !== null && $defenderBefore !== null && (int)$attackerBefore['place2'] < (int)$defenderBefore['place2'], 'higher fleet score ranks ahead before score swap', array('attacker' => $attackerBefore, 'defender' => $defenderBefore)),
            e2e_case($attackerBefore !== null && $defenderBefore !== null && (int)$attackerBefore['place3'] < (int)$defenderBefore['place3'], 'higher research score ranks ahead before score swap', array('attacker' => $attackerBefore, 'defender' => $defenderBefore)),
            e2e_case($attackerAfter !== null && $defenderAfter !== null && (int)$defenderAfter['place1'] < (int)$attackerAfter['place1'], 'rank order flips when defender total score becomes higher', array('attacker' => $attackerAfter, 'defender' => $defenderAfter)),
            e2e_case($attackerAfter !== null && $defenderAfter !== null && (int)$defenderAfter['place2'] < (int)$attackerAfter['place2'], 'fleet rank order flips when defender fleet score becomes higher', array('attacker' => $attackerAfter, 'defender' => $defenderAfter)),
            e2e_case($attackerAfter !== null && $defenderAfter !== null && (int)$defenderAfter['place3'] < (int)$attackerAfter['place3'], 'research rank order flips when defender research score becomes higher', array('attacker' => $attackerAfter, 'defender' => $defenderAfter)),
        ),
    ));

    $auth = e2e_prepare_session($attackerId, 'stats-ranking');
    $cookies = $auth['cookies'];
    $statsResponse = e2e_http_request('GET', $gameBase . '/index.php?page=statistics&session=' . rawurlencode($auth['session']) . '&start=1&type=ressources', array(), $cookies);
    $fleetResponse = e2e_http_request('GET', $gameBase . '/index.php?page=statistics&session=' . rawurlencode($auth['session']) . '&start=1&type=fleet', array(), $cookies);
    $researchResponse = e2e_http_request('GET', $gameBase . '/index.php?page=statistics&session=' . rawurlencode($auth['session']) . '&start=1&type=research', array(), $cookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'statistics_page_renders_ranked_fixture_users',
        'checks' => array_merge(
            e2e_response_check($statsResponse),
            e2e_response_check($fleetResponse),
            e2e_response_check($researchResponse),
            array(
                e2e_case(stripos($statsResponse['body'], 'Statistics') !== false && stripos($statsResponse['body'], 'Points') !== false, 'points statistics page renders the expected table headings'),
                e2e_case(strpos($statsResponse['body'], $attackerName) !== false && strpos($statsResponse['body'], $defenderName) !== false, 'points statistics page lists both fixture users'),
                e2e_case(strpos($fleetResponse['body'], $attackerName) !== false && strpos($fleetResponse['body'], $defenderName) !== false, 'fleet statistics page lists both fixture users'),
                e2e_case(strpos($researchResponse['body'], $attackerName) !== false && strpos($researchResponse['body'], $defenderName) !== false, 'research statistics page lists both fixture users'),
                e2e_case(strpos($statsResponse['body'], nicenum(950000)) !== false, 'points statistics page displays defender score in thousands', array('expected' => nicenum(950000))),
            )
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'stats_ranking_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($attackerId > 0 && $attackerPlanet > 0 && $defenderId > 0 && $defenderPlanet > 0) {
        e2e_reset_fixture(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    }
}

echo json_encode(array(
    'case_group' => 'http_stats_ranking',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
