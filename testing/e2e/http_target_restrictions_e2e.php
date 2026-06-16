<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_target_restrictions_e2e.php';
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
loca_add('fleet', 'en');
loca_add('galaxy', 'en');
loca_add('technames', 'en');

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

function e2e_response_check(array $response, string $label = 'HTTP request'): array
{
    $body = $response['body'];
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+(Undefined|Trying)/i', $body) === 1;
    return array(
        e2e_case(in_array($response['status'], array(200, 301, 302, 303), true), $label . ' returns an accepted status', array('status' => $response['status'], 'location' => $response['location'])),
        e2e_case(!$hasError, $label . ' body has no PHP error marker'),
    );
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

function e2e_prepare_session(int $userId, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', validated=1, deact_ip=1, " .
        "lang='en', skin='/evolution/', useskin=1 WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookies' => array('prsess_' . $userId . '_' . $GlobalUni['num'] => $private),
    );
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
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id IN ({$userList}) AND type='" . QTYP_FLEET . "'");
}

function e2e_cleanup_runtime(array $userIds, array $planetIds): void
{
    global $db_prefix;

    e2e_cleanup_fleets($userIds, $planetIds);
    $userList = implode(',', array_map('intval', $userIds));
    $planetList = implode(',', array_map('intval', $planetIds));
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id IN ({$userList}) AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_UNBAN . "','" . QTYP_ALLOW_ATTACKS . "','" . QTYP_DEBUG . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id IN ({$userList}) OR planet_id IN ({$planetList})");
}

function e2e_find_open_pair(): array
{
    global $GlobalUni, $db_prefix;

    for ($g = 1; $g <= (int)$GlobalUni['galaxies']; $g++) {
        for ($s = 1; $s <= (int)$GlobalUni['systems']; $s++) {
            $free = array();
            for ($p = 1; $p <= 15; $p++) {
                $used = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}planets WHERE g={$g} AND s={$s} AND p={$p}");
                if ($used === 0) {
                    $free[] = $p;
                }
                if (count($free) >= 2) {
                    return array('g' => $g, 's' => $s, 'attacker_p' => $free[0], 'defender_p' => $free[1]);
                }
            }
        }
    }

    throw new RuntimeException('No open same-system planet pair is available for target restriction E2E fixtures.');
}

function e2e_set_user_state(int $userId, int $score, array $options = array()): void
{
    global $db_prefix, $resmap;

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=10";
    }
    $now = time();
    $admin = (int)($options['admin'] ?? USER_TYPE_PLAYER);
    $vacation = (int)($options['vacation'] ?? 0);
    $vacationUntil = $vacation ? $now + 3600 : 0;
    $banned = (int)($options['banned'] ?? 0);
    $bannedUntil = $banned ? $now + 3600 : 0;
    $noattack = (int)($options['noattack'] ?? 0);
    $noattackUntil = $noattack ? $now + 3600 : 0;

    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", " .
        "admin={$admin}, ally_id=0, allyrank=0, validated=1, deact_ip=1, vacation={$vacation}, vacation_until={$vacationUntil}, " .
        "banned={$banned}, banned_until={$bannedUntil}, noattack={$noattack}, noattack_until={$noattackUntil}, " .
        "disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, score1={$score}, score2=0, score3=0, " .
        "place1=1, place2=1, place3=1, flags=" . USER_FLAG_DEFAULT . ", lastclick={$now}, " .
        "com_until=0, adm_until=0, eng_until=0, geo_until=0, tec_until=0 WHERE player_id={$userId}"
    );
    InvalidateUserCache();
}

function e2e_prepare_planet(int $planetId, int $ownerId, int $g, int $s, int $p, string $name): void
{
    global $db_prefix, $fleetmap, $defmap, $buildmap;

    $assignments = array();
    foreach (array_merge($fleetmap, $defmap, $buildmap) as $gid) {
        $assignments[] = "`{$gid}`=0";
    }

    $now = time();
    dbquery(
        "UPDATE {$db_prefix}planets SET " . implode(',', $assignments) . ", " .
        "name='" . e2e_sql_escape($name) . "', type=" . PTYP_PLANET . ", g={$g}, s={$s}, p={$p}, owner_id={$ownerId}, " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "`" . GID_B_SOLAR . "`=20, `" . GID_B_SHIPYARD . "`=12, `" . GID_B_MISS_SILO . "`=6, " .
        "`" . GID_F_SC . "`=10, `" . GID_F_LF . "`=10, `" . GID_F_PROBE . "`=10, `" . GID_F_RECYCLER . "`=4, " .
        "`" . GID_D_RL . "`=10, `" . GID_D_IPM . "`=4, prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, " .
        "fields=0, maxfields=300, lastpeek={$now}, lastakt={$now}, remove=0 WHERE planet_id={$planetId}"
    );
}

function e2e_prepare_pair(int $attackerId, int $attackerPlanet, int $defenderId, int $defenderPlanet, array $coords, int $attackerScore, int $defenderScore, array $attackerOptions = array(), array $defenderOptions = array()): array
{
    e2e_cleanup_runtime(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    @unlink('temp/fleetlock_' . $attackerPlanet);
    e2e_set_user_state($attackerId, $attackerScore, $attackerOptions);
    e2e_set_user_state($defenderId, $defenderScore, $defenderOptions);
    e2e_prepare_planet($attackerPlanet, $attackerId, $coords['g'], $coords['s'], $coords['attacker_p'], 'E2E Restrict A');
    e2e_prepare_planet($defenderPlanet, $defenderId, $coords['g'], $coords['s'], $coords['defender_p'], 'E2E Restrict D');

    return array(
        'origin' => LoadPlanetById($attackerPlanet),
        'target' => LoadPlanetById($defenderPlanet),
    );
}

function e2e_fleet_payload(array $origin, array $target, int $order, array $ships, array $extra = array()): array
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
        $data['resource' . ($i + 1)] = '0';
    }
    foreach ($extra as $key => $value) {
        $data[$key] = (string)$value;
    }
    return $data;
}

function e2e_active_fleet_count(int $userId): int
{
    global $db_prefix;
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$userId} AND type='" . QTYP_FLEET . "'");
}

function e2e_fleet_row_count(int $userId): int
{
    global $db_prefix;
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id={$userId}");
}

function e2e_direct_fleet_request(string $gameBase, string $session, int $attackerPlanet, array $cookies, array $origin, array $target, int $order, array $ships): array
{
    return e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet,
        e2e_fleet_payload($origin, $target, $order, $ships),
        $cookies
    );
}

function e2e_ajax_spy_request(string $gameBase, string $session, int $attackerPlanet, array $cookies, array $target, int $shipCount = 1): array
{
    return e2e_http_request(
        'POST',
        $gameBase . '/index.php?ajax=1&page=flottenversand&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet,
        array(
            'session' => $session,
            'order' => FTYP_SPY,
            'galaxy' => $target['g'],
            'system' => $target['s'],
            'planet' => $target['p'],
            'planettype' => GetPlanetType($target),
            'shipcount' => $shipCount,
            'speed' => 10,
            'reply' => 'short',
        ),
        $cookies
    );
}

function e2e_ipm_request(string $gameBase, string $session, int $attackerPlanet, array $cookies, array $target, int $amount = 1): array
{
    return e2e_http_request(
        'POST',
        $gameBase . '/index.php?page=galaxy&no_header=1&session=' . rawurlencode($session) . '&cp=' . $attackerPlanet . '&p1=' . $target['g'] . '&p2=' . $target['s'] . '&p3=' . $target['p'] . '&pdd=' . $target['planet_id'] . '&zp=' . $target['owner_id'],
        array(
            'aktion' => 'Attack',
            'anz' => $amount,
            'pziel' => 0,
        ),
        $cookies
    );
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

    e2e_cleanup_runtime(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    $coords = e2e_find_open_pair();

    $pair = e2e_prepare_pair($attackerId, $attackerPlanet, $defenderId, $defenderPlanet, $coords, 10000, 10000);
    $auth = e2e_prepare_session($attackerId, 'target-allowed');
    $cookies = $auth['cookies'];
    $response = e2e_direct_fleet_request($gameBase, $auth['session'], $attackerPlanet, $cookies, $pair['origin'], $pair['target'], FTYP_ATTACK, array(GID_F_LF => 1));
    $fleet = e2e_one_row("SELECT fleet_id, mission, start_planet, target_planet, `" . GID_F_LF . "` AS light_fighters FROM {$db_prefix}fleet WHERE owner_id={$attackerId} ORDER BY fleet_id DESC LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'direct_attack_allowed_when_scores_are_comparable',
        'checks' => array_merge(e2e_response_check($response, 'allowed direct attack'), array(
            e2e_case(stripos($response['body'], 'Fleet dispatched') !== false, 'allowed attack renders the dispatch success page', array('body_excerpt' => substr(trim(preg_replace('/\s+/', ' ', strip_tags($response['body']))), 0, 1000))),
            e2e_case($fleet !== null && (int)$fleet['mission'] === FTYP_ATTACK && (int)$fleet['target_planet'] === $defenderPlanet && (int)$fleet['light_fighters'] === 1, 'allowed attack creates an attack fleet row', $fleet ?? array()),
            e2e_case(e2e_active_fleet_count($attackerId) === 1, 'allowed attack creates one active fleet queue task'),
        )),
    ));

    $restrictionCases = array(
        'direct_attack_rejects_newbie_target' => array(
            'attacker_score' => 100000,
            'defender_score' => 1000,
            'order' => FTYP_ATTACK,
            'ships' => array(GID_F_LF => 1),
            'expected_text' => 'protected for newbies',
            'message' => 'newbie protected target blocks direct attack',
        ),
        'direct_attack_rejects_strong_target' => array(
            'attacker_score' => 1000,
            'defender_score' => 100000,
            'order' => FTYP_ATTACK,
            'ships' => array(GID_F_LF => 1),
            'expected_text' => 'protected for newbies',
            'message' => 'strong target blocks direct attack from a weak player',
        ),
        'direct_spy_rejects_newbie_target' => array(
            'attacker_score' => 100000,
            'defender_score' => 1000,
            'order' => FTYP_SPY,
            'ships' => array(GID_F_PROBE => 1),
            'expected_text' => 'newbie protection',
            'message' => 'newbie protected target blocks direct espionage',
        ),
        'direct_attack_rejects_vacation_target' => array(
            'attacker_score' => 10000,
            'defender_score' => 10000,
            'defender_options' => array('vacation' => 1),
            'order' => FTYP_ATTACK,
            'ships' => array(GID_F_LF => 1),
            'expected_text' => 'vacation mode',
            'message' => 'vacation target blocks direct attack',
        ),
        'direct_spy_rejects_operator_target' => array(
            'attacker_score' => 10000,
            'defender_score' => 10000,
            'defender_options' => array('admin' => USER_TYPE_GO),
            'order' => FTYP_SPY,
            'ships' => array(GID_F_PROBE => 1),
            'expected_text' => 'game operators or administrators',
            'message' => 'operator target blocks direct espionage',
        ),
        'direct_attack_rejects_self_attack_ban' => array(
            'attacker_score' => 10000,
            'defender_score' => 10000,
            'attacker_options' => array('noattack' => 1),
            'order' => FTYP_ATTACK,
            'ships' => array(GID_F_LF => 1),
            'expected_text' => 'Ban attacks to',
            'message' => 'temporary attack ban blocks direct attack',
        ),
    );

    foreach ($restrictionCases as $caseName => $spec) {
        $pair = e2e_prepare_pair(
            $attackerId,
            $attackerPlanet,
            $defenderId,
            $defenderPlanet,
            $coords,
            $spec['attacker_score'],
            $spec['defender_score'],
            $spec['attacker_options'] ?? array(),
            $spec['defender_options'] ?? array()
        );
        $auth = e2e_prepare_session($attackerId, $caseName);
        $cookies = $auth['cookies'];
        $before = e2e_fleet_row_count($attackerId);
        $response = e2e_direct_fleet_request($gameBase, $auth['session'], $attackerPlanet, $cookies, $pair['origin'], $pair['target'], $spec['order'], $spec['ships']);
        $after = e2e_fleet_row_count($attackerId);
        $cases[] = e2e_finalize_case(array(
            'case' => $caseName,
            'checks' => array_merge(e2e_response_check($response, $caseName), array(
                e2e_case(stripos($response['body'], 'The fleet could not be dispatched') !== false, $spec['message'] . ' and renders dispatch failure'),
                e2e_case(stripos($response['body'], $spec['expected_text']) !== false, $spec['message'] . ' with expected text', array('expected_text' => $spec['expected_text'])),
                e2e_case($after === $before, $spec['message'] . ' without creating a fleet row', array('before' => $before, 'after' => $after)),
                e2e_case(e2e_active_fleet_count($attackerId) === 0, $spec['message'] . ' without creating a queue task'),
            )),
        ));
    }

    $ajaxCases = array(
        'ajax_spy_rejects_newbie_target' => array(
            'attacker_score' => 100000,
            'defender_score' => 1000,
            'expected_code' => '603',
        ),
        'ajax_spy_rejects_strong_target' => array(
            'attacker_score' => 1000,
            'defender_score' => 100000,
            'expected_code' => '604',
        ),
        'ajax_spy_rejects_vacation_target' => array(
            'attacker_score' => 10000,
            'defender_score' => 10000,
            'defender_options' => array('vacation' => 1),
            'expected_code' => '605',
        ),
        'ajax_spy_rejects_operator_target' => array(
            'attacker_score' => 10000,
            'defender_score' => 10000,
            'defender_options' => array('admin' => USER_TYPE_GO),
            'expected_code' => '601',
        ),
    );

    foreach ($ajaxCases as $caseName => $spec) {
        $pair = e2e_prepare_pair(
            $attackerId,
            $attackerPlanet,
            $defenderId,
            $defenderPlanet,
            $coords,
            $spec['attacker_score'],
            $spec['defender_score'],
            array(),
            $spec['defender_options'] ?? array()
        );
        $auth = e2e_prepare_session($attackerId, $caseName);
        $cookies = $auth['cookies'];
        $response = e2e_ajax_spy_request($gameBase, $auth['session'], $attackerPlanet, $cookies, $pair['target']);
        $body = trim($response['body']);
        $cases[] = e2e_finalize_case(array(
            'case' => $caseName,
            'checks' => array_merge(e2e_response_check($response, $caseName), array(
                e2e_case(str_starts_with($body, $spec['expected_code'] . ' '), $caseName . ' returns expected AJAX error code', array('body' => $body, 'expected_code' => $spec['expected_code'])),
                e2e_case(e2e_fleet_row_count($attackerId) === 0, $caseName . ' does not create a fleet row'),
                e2e_case(e2e_active_fleet_count($attackerId) === 0, $caseName . ' does not create a queue task'),
            )),
        ));
    }

    $ipmCases = array(
        'ipm_rejects_newbie_target' => array(
            'attacker_score' => 100000,
            'defender_score' => 1000,
            'expected_text' => 'noob protection',
        ),
        'ipm_rejects_vacation_target' => array(
            'attacker_score' => 10000,
            'defender_score' => 10000,
            'defender_options' => array('vacation' => 1),
            'expected_text' => 'vacation mode',
        ),
        'ipm_rejects_operator_target' => array(
            'attacker_score' => 10000,
            'defender_score' => 10000,
            'defender_options' => array('admin' => USER_TYPE_GO),
            'expected_text' => 'game operators or administrators',
        ),
    );

    foreach ($ipmCases as $caseName => $spec) {
        $pair = e2e_prepare_pair(
            $attackerId,
            $attackerPlanet,
            $defenderId,
            $defenderPlanet,
            $coords,
            $spec['attacker_score'],
            $spec['defender_score'],
            array(),
            $spec['defender_options'] ?? array()
        );
        $auth = e2e_prepare_session($attackerId, $caseName);
        $cookies = $auth['cookies'];
        $beforeIpm = e2e_one_row("SELECT `" . GID_D_IPM . "` AS ipm FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");
        $response = e2e_ipm_request($gameBase, $auth['session'], $attackerPlanet, $cookies, $pair['target']);
        $afterIpm = e2e_one_row("SELECT `" . GID_D_IPM . "` AS ipm FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");
        $cases[] = e2e_finalize_case(array(
            'case' => $caseName,
            'checks' => array_merge(e2e_response_check($response, $caseName), array(
                e2e_case(stripos($response['body'], $spec['expected_text']) !== false, $caseName . ' renders expected galaxy rocket restriction text', array('expected_text' => $spec['expected_text'])),
                e2e_case($beforeIpm !== null && $afterIpm !== null && (int)$afterIpm['ipm'] === (int)$beforeIpm['ipm'], $caseName . ' does not consume an interplanetary missile', array('before' => $beforeIpm, 'after' => $afterIpm)),
                e2e_case(e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id={$attackerId} AND ipm_amount > 0") === 0, $caseName . ' does not create an IPM fleet row'),
            )),
        ));
    }
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'target_restrictions_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e), 'trace' => $e->getTraceAsString()))),
        'pass' => false,
    );
} finally {
    if ($attackerId > 0 && $defenderId > 0 && $attackerPlanet > 0 && $defenderPlanet > 0) {
        e2e_cleanup_runtime(array($attackerId, $defenderId), array($attackerPlanet, $defenderPlanet));
    }
}

echo json_encode(array(
    'case_group' => 'http_target_restrictions',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
