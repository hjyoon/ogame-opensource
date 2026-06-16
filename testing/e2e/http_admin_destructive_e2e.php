<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_admin_destructive_e2e.php';
$_COOKIE['ogamelang'] = 'en';

chdir('/var/www/html/game');
require_once 'config.php';
require_once 'core/core.php';

InitDB();
$GlobalUni = LoadUniverse();
$GlobalUser = array('player_id' => 0);
$session = '';
ModsInit();

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
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined|Unknown column|You have an error in your SQL syntax/i', $body) === 1;
    return array(
        e2e_case(in_array($response['status'], array(200, 301, 302, 303), true), $label . ' returns an accepted status', array('status' => $response['status'], 'location' => $response['location'])),
        e2e_case(!$hasError, $label . ' body has no PHP or SQL error marker'),
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

function e2e_prepare_session(int $userId, int $adminLevel, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', admin={$adminLevel}, validated=1, deact_ip=1, " .
        "lang='en', skin='/evolution/', useskin=1, vacation=0, vacation_until=0, banned=0, banned_until=0, " .
        "noattack=0, noattack_until=0, disable=0, disable_until=0 WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookies' => array('prsess_' . $userId . '_' . $GlobalUni['num'] => $private),
    );
}

function e2e_snapshot_user(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT player_id, admin, session, private_session, validated, deact_ip, vacation, vacation_until, banned, banned_until, " .
        "noattack, noattack_until, disable, disable_until, lang, skin, useskin, pemail, email, sniff, debug, dm, dmfree, " .
        "sortby, sortorder, maxspy, maxfleetmsg, score1, score2, score3, place1, place2, place3, lastclick, hplanetid, aktplanet " .
        "FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1"
    );
}

function e2e_restore_user(?array $user): void
{
    global $db_prefix;
    if ($user === null) {
        return;
    }

    $id = (int)$user['player_id'];
    dbquery(
        "UPDATE {$db_prefix}users SET " .
        "admin=" . (int)$user['admin'] . ", session='" . e2e_sql_escape($user['session']) . "', private_session='" . e2e_sql_escape($user['private_session']) . "', " .
        "validated=" . (int)$user['validated'] . ", deact_ip=" . (int)$user['deact_ip'] . ", vacation=" . (int)$user['vacation'] . ", vacation_until=" . (int)$user['vacation_until'] . ", " .
        "banned=" . (int)$user['banned'] . ", banned_until=" . (int)$user['banned_until'] . ", noattack=" . (int)$user['noattack'] . ", noattack_until=" . (int)$user['noattack_until'] . ", " .
        "disable=" . (int)$user['disable'] . ", disable_until=" . (int)$user['disable_until'] . ", lang='" . e2e_sql_escape($user['lang']) . "', skin='" . e2e_sql_escape($user['skin']) . "', " .
        "useskin=" . (int)$user['useskin'] . ", pemail='" . e2e_sql_escape($user['pemail']) . "', email='" . e2e_sql_escape($user['email']) . "', sniff=" . (int)$user['sniff'] . ", " .
        "debug=" . (int)$user['debug'] . ", dm=" . (int)$user['dm'] . ", dmfree=" . (int)$user['dmfree'] . ", sortby=" . (int)$user['sortby'] . ", sortorder=" . (int)$user['sortorder'] . ", " .
        "maxspy=" . (int)$user['maxspy'] . ", maxfleetmsg=" . (int)$user['maxfleetmsg'] . ", score1=" . (int)$user['score1'] . ", score2=" . (int)$user['score2'] . ", score3=" . (int)$user['score3'] . ", " .
        "place1=" . (int)$user['place1'] . ", place2=" . (int)$user['place2'] . ", place3=" . (int)$user['place3'] . ", lastclick=" . (int)$user['lastclick'] . ", " .
        "hplanetid=" . (int)$user['hplanetid'] . ", aktplanet=" . (int)$user['aktplanet'] . " WHERE player_id={$id}"
    );
    InvalidateUserCache();
}

function e2e_restore_uni(array $uni): void
{
    global $db_prefix, $GlobalUni;

    dbquery(
        "UPDATE {$db_prefix}uni SET lang='" . e2e_sql_escape($uni['lang']) . "', battle_engine='" . e2e_sql_escape($uni['battle_engine']) . "', " .
        "freeze=" . (int)$uni['freeze'] . ", speed=" . (float)$uni['speed'] . ", fspeed=" . (float)$uni['fspeed'] . ", acs=" . (int)$uni['acs'] . ", " .
        "fid=" . (int)$uni['fid'] . ", did=" . (int)$uni['did'] . ", defrepair=" . (int)$uni['defrepair'] . ", defrepair_delta=" . (int)$uni['defrepair_delta'] . ", " .
        "galaxies=" . (int)$uni['galaxies'] . ", systems=" . (int)$uni['systems'] . ", maxusers=" . (int)$uni['maxusers'] . ", rapid=" . (int)$uni['rapid'] . ", moons=" . (int)$uni['moons'] . ", " .
        "php_battle=" . (int)$uni['php_battle'] . ", battle_max=" . (int)$uni['battle_max'] . ", force_lang=" . (int)$uni['force_lang'] . ", start_dm=" . (int)$uni['start_dm'] . ", " .
        "max_werf=" . (int)$uni['max_werf'] . ", feedage=" . (int)$uni['feedage'] . ", news1='" . e2e_sql_escape($uni['news1']) . "', news2='" . e2e_sql_escape($uni['news2']) . "', " .
        "news_until=" . (int)$uni['news_until'] . ", ext_board='" . e2e_sql_escape($uni['ext_board']) . "', ext_discord='" . e2e_sql_escape($uni['ext_discord']) . "', " .
        "ext_tutorial='" . e2e_sql_escape($uni['ext_tutorial']) . "', ext_rules='" . e2e_sql_escape($uni['ext_rules']) . "', ext_impressum='" . e2e_sql_escape($uni['ext_impressum']) . "'"
    );
    $GlobalUni = LoadUniverse();
}

function e2e_user_update_payload(array $user, array $overrides = array()): array
{
    global $resmap;

    $data = array(
        'pemail' => $user['pemail'],
        'email' => $user['email'],
        'deaktjava' => '',
        'vacation' => '',
        'banned' => '',
        'noattack' => '',
        'admin' => (string)$user['admin'],
        'validated' => ((int)$user['validated']) ? 'on' : '',
        'sniff' => ((int)$user['sniff']) ? 'on' : '',
        'debug' => ((int)$user['debug']) ? 'on' : '',
        'dm' => (string)$user['dm'],
        'dmfree' => (string)$user['dmfree'],
        'settings_sort' => (string)$user['sortby'],
        'settings_order' => (string)$user['sortorder'],
        'dpath' => $user['skin'],
        'design' => ((int)$user['useskin']) ? 'on' : '',
        'deact_ip' => ((int)$user['deact_ip']) ? 'on' : '',
        'spio_anz' => (string)$user['maxspy'],
        'settings_fleetactions' => (string)$user['maxfleetmsg'],
        'pr_' . USER_OFFICER_COMMANDER => '',
        'pr_' . USER_OFFICER_ADMIRAL => '',
        'pr_' . USER_OFFICER_ENGINEER => '',
        'pr_' . USER_OFFICER_GEOLOGE => '',
        'pr_' . USER_OFFICER_TECHNOCRATE => '',
    );
    foreach ($resmap as $gid) {
        $data['r' . $gid] = (string)($user[$gid] ?? 0);
    }
    foreach ($overrides as $key => $value) {
        $data[$key] = $value;
    }
    return $data;
}

function e2e_uni_payload(array $uni, bool $freeze): array
{
    return array(
        'news_upd' => '0',
        'news1' => $uni['news1'],
        'news2' => $uni['news2'],
        'maxusers' => (string)$uni['maxusers'],
        'start_dm' => (string)$uni['start_dm'],
        'galaxies' => (string)$uni['galaxies'],
        'systems' => (string)$uni['systems'],
        'max_werf' => (string)$uni['max_werf'],
        'feedage' => (string)$uni['feedage'],
        'speed' => (string)$uni['speed'],
        'fspeed' => (string)$uni['fspeed'],
        'acs' => (string)$uni['acs'],
        'fid' => (string)$uni['fid'],
        'did' => (string)$uni['did'],
        'rapid' => ((int)$uni['rapid']) ? 'on' : '',
        'moons' => ((int)$uni['moons']) ? 'on' : '',
        'defrepair' => (string)$uni['defrepair'],
        'defrepair_delta' => (string)$uni['defrepair_delta'],
        'freeze' => $freeze ? 'on' : '',
        'lang' => $uni['lang'],
        'force_lang' => ((int)$uni['force_lang']) ? 'on' : '',
        'battle_engine' => $uni['battle_engine'],
        'php_battle' => ((int)$uni['php_battle']) ? 'on' : '',
        'battle_max' => (string)$uni['battle_max'],
        'ext_board' => $uni['ext_board'],
        'ext_discord' => $uni['ext_discord'],
        'ext_tutorial' => $uni['ext_tutorial'],
        'ext_rules' => $uni['ext_rules'],
        'ext_impressum' => $uni['ext_impressum'],
    );
}

function e2e_queue_row(int $taskId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT task_id, owner_id, type, start, end, freeze, frozen FROM {$db_prefix}queue WHERE task_id={$taskId} LIMIT 1");
}

function e2e_find_empty_position(array $near): array
{
    global $GlobalUni;

    $g = (int)$near['g'];
    for ($system = 1; $system <= (int)$GlobalUni['systems']; $system++) {
        for ($p = 1; $p <= 15; $p++) {
            if ($system === (int)$near['s'] && $p === (int)$near['p']) {
                continue;
            }
            if (!HasPlanet($g, $system, $p)) {
                return array($g, $system, $p);
            }
        }
    }
    throw new RuntimeException('No empty coordinate found for admin destructive test.');
}

function e2e_reset_runtime_state(int $userId, int $planetId): void
{
    global $db_prefix;

    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$userId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "','" . QTYP_UNBAN . "','" . QTYP_ALLOW_ATTACKS . "','" . QTYP_ALLOW_NAME . "','" . QTYP_RECALC_POINTS . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$userId} OR planet_id={$planetId}");
    dbquery("UPDATE {$db_prefix}planets SET prod1=1, prod2=1, prod3=1, prod4=1, prod12=1, prod212=1, `" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000 WHERE planet_id={$planetId}");
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderPlanet = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$cases = array();
$attackerSnapshot = null;
$defenderSnapshot = null;
$uniSnapshot = LoadUniverse();
$createdQueueIds = array();
$createdPlanetIds = array();

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $attackerSnapshot = e2e_snapshot_user($attackerId);
    $defenderSnapshot = e2e_snapshot_user($defenderId);
    $defenderFull = LoadUser($defenderId);
    if ($defenderFull === null) {
        throw new RuntimeException('Failed to load defender fixture.');
    }

    e2e_reset_runtime_state($attackerId, $attackerPlanet);
    e2e_reset_runtime_state($defenderId, $defenderPlanet);

    $operatorAuth = e2e_prepare_session($attackerId, USER_TYPE_GO, 'admin-destructive-operator');
    $operatorCookies = $operatorAuth['cookies'];
    $queueId = AddQueue($defenderId, QTYP_ALLOW_NAME, 0, 0, 0, time(), 3600, QUEUE_PRIO_LOWEST);
    $createdQueueIds[] = $queueId;
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($operatorAuth['session']) . '&mode=Queue', array('order_remove' => (string)$queueId), $operatorCookies);
    $operatorQueueAfter = e2e_queue_row($queueId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'operator_cannot_delete_queue_task',
        'checks' => array_merge(e2e_response_check($response, 'operator queue delete POST'), array(
            e2e_case($operatorQueueAfter !== null, 'operator-level admin cannot remove queue tasks', $operatorQueueAfter ?? array()),
        )),
    ));

    $adminAuth = e2e_prepare_session($attackerId, USER_TYPE_ADMIN, 'admin-destructive-full');
    $adminCookies = $adminAuth['cookies'];
    $freezeResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Queue', array('order_freeze' => (string)$queueId), $adminCookies);
    $frozenQueue = e2e_queue_row($queueId);
    $unfreezeResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Queue', array('order_unfreeze' => (string)$queueId), $adminCookies);
    $unfrozenQueue = e2e_queue_row($queueId);
    $removeResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Queue', array('order_remove' => (string)$queueId), $adminCookies);
    $removedQueue = e2e_queue_row($queueId);
    $createdQueueIds = array_values(array_filter($createdQueueIds, fn($id) => $id !== $queueId));
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_freezes_unfreezes_and_deletes_queue_task',
        'checks' => array_merge(e2e_response_check($freezeResponse, 'admin queue freeze POST'), e2e_response_check($unfreezeResponse, 'admin queue unfreeze POST'), e2e_response_check($removeResponse, 'admin queue delete POST'), array(
            e2e_case($frozenQueue !== null && (int)$frozenQueue['freeze'] === 1 && (int)$frozenQueue['frozen'] > 0, 'admin freezes a queue task', $frozenQueue ?? array()),
            e2e_case($unfrozenQueue !== null && (int)$unfrozenQueue['freeze'] === 0 && (int)$unfrozenQueue['frozen'] === 0, 'admin unfreezes a queue task', $unfrozenQueue ?? array()),
            e2e_case($removedQueue === null, 'admin removes a queue task', $removedQueue ?? array()),
        )),
    ));

    dbquery("UPDATE {$db_prefix}users SET score1=0, score2=0, score3=0 WHERE player_id={$defenderId}");
    $recalcQueueId = AddQueue($defenderId, QTYP_RECALC_POINTS, 0, 0, 0, time(), 3600, QUEUE_PRIO_RECALC_POINTS);
    $createdQueueIds[] = $recalcQueueId;
    $completeResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Queue', array('order_end' => (string)$recalcQueueId), $adminCookies);
    $dueQueue = e2e_queue_row($recalcQueueId);
    $playerAuth = e2e_prepare_session($defenderId, USER_TYPE_PLAYER, 'admin-destructive-player-trigger');
    $playerCookies = $playerAuth['cookies'];
    $triggerResponse = e2e_http_request('GET', $gameBase . '/index.php?page=overview&session=' . rawurlencode($playerAuth['session']) . '&cp=' . $defenderPlanet, array(), $playerCookies);
    $completedQueue = e2e_queue_row($recalcQueueId);
    $scoreAfterForcedQueue = e2e_one_row("SELECT player_id, score1, score2, score3 FROM {$db_prefix}users WHERE player_id={$defenderId} LIMIT 1");
    $createdQueueIds = array_values(array_filter($createdQueueIds, fn($id) => $id !== $recalcQueueId));
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_forces_recalc_queue_completion',
        'checks' => array_merge(e2e_response_check($completeResponse, 'admin queue complete POST'), e2e_response_check($triggerResponse, 'post-complete admin trigger GET'), array(
            e2e_case($dueQueue !== null && (int)$dueQueue['end'] <= time(), 'admin complete moves queue task due immediately', $dueQueue ?? array()),
            e2e_case($completedQueue === null, 'next player page queue update completes and removes the recalc task', $completedQueue ?? array()),
            e2e_case($scoreAfterForcedQueue !== null && (int)$scoreAfterForcedQueue['score1'] > 0 && (int)$scoreAfterForcedQueue['score3'] > 0, 'forced recalc queue updates target scores', $scoreAfterForcedQueue ?? array()),
        )),
    ));

    dbquery("UPDATE {$db_prefix}users SET score1=0, score2=0, score3=0 WHERE player_id={$defenderId}");
    $recalcResponse = e2e_http_request('GET', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Users&action=recalc_stats&player_id=' . $defenderId, array(), $adminCookies);
    $scoreAfterAdminAction = e2e_one_row("SELECT player_id, score1, score2, score3 FROM {$db_prefix}users WHERE player_id={$defenderId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_recalc_stats_action_updates_user_scores',
        'checks' => array_merge(e2e_response_check($recalcResponse, 'admin recalc stats GET'), array(
            e2e_case($scoreAfterAdminAction !== null && (int)$scoreAfterAdminAction['score1'] > 0 && (int)$scoreAfterAdminAction['score3'] > 0, 'admin recalc_stats action recalculates user points', $scoreAfterAdminAction ?? array()),
        )),
    ));

    $scheduleResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Users&action=update&player_id=' . $defenderId, e2e_user_update_payload($defenderFull, array('deaktjava' => 'on', 'admin' => (string)USER_TYPE_PLAYER)), $adminCookies);
    $afterSchedule = e2e_one_row("SELECT player_id, disable, disable_until FROM {$db_prefix}users WHERE player_id={$defenderId} LIMIT 1");
    $cancelResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Users&action=update&player_id=' . $defenderId, e2e_user_update_payload($defenderFull, array('deaktjava' => '', 'admin' => (string)USER_TYPE_PLAYER)), $adminCookies);
    $afterCancel = e2e_one_row("SELECT player_id, disable, disable_until FROM {$db_prefix}users WHERE player_id={$defenderId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_schedules_and_cancels_account_deletion',
        'checks' => array_merge(e2e_response_check($scheduleResponse, 'admin schedule account deletion POST'), e2e_response_check($cancelResponse, 'admin cancel account deletion POST'), array(
            e2e_case($afterSchedule !== null && (int)$afterSchedule['disable'] === 1 && (int)$afterSchedule['disable_until'] > time(), 'admin schedules account deletion with a future deadline', $afterSchedule ?? array()),
            e2e_case($afterCancel !== null && (int)$afterCancel['disable'] === 0 && (int)$afterCancel['disable_until'] === 0, 'admin cancels account deletion schedule', $afterCancel ?? array()),
        )),
    ));

    $homePlanet = LoadPlanetById($defenderPlanet);
    if ($homePlanet === null) {
        throw new RuntimeException('Failed to load defender planet.');
    }
    $coords = e2e_find_empty_position($homePlanet);
    $createPlanetResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Users&action=create_planet&player_id=' . $defenderId, array('g' => (string)$coords[0], 's' => (string)$coords[1], 'p' => (string)$coords[2]), $adminCookies);
    $createdPlanet = e2e_one_row("SELECT planet_id, owner_id, type, g, s, p, prod1, prod2, prod3 FROM {$db_prefix}planets WHERE owner_id={$defenderId} AND g={$coords[0]} AND s={$coords[1]} AND p={$coords[2]} AND type=" . PTYP_PLANET . " ORDER BY planet_id DESC LIMIT 1");
    if ($createdPlanet !== null) {
        $createdPlanetIds[] = (int)$createdPlanet['planet_id'];
    }
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_creates_planet_and_stops_mine_production',
        'checks' => array_merge(e2e_response_check($createPlanetResponse, 'admin create planet POST'), array(
            e2e_case($createdPlanet !== null && (int)$createdPlanet['owner_id'] === $defenderId, 'admin creates a planet for the target user', $createdPlanet ?? array()),
            e2e_case($createdPlanet !== null && (float)$createdPlanet['prod1'] === 0.0 && (float)$createdPlanet['prod2'] === 0.0 && (float)$createdPlanet['prod3'] === 0.0, 'created admin planet starts with mine production stopped', $createdPlanet ?? array()),
        )),
    ));

    dbquery("UPDATE {$db_prefix}users SET vacation=0, vacation_until=0, lastclick=" . time() . " WHERE player_id={$defenderId}");
    InvalidateUserCache();
    $freezeResponse = e2e_http_request('POST', $gameBase . '/index.php?page=admin&session=' . rawurlencode($adminAuth['session']) . '&mode=Uni', e2e_uni_payload($uniSnapshot, true), $adminCookies);
    $frozenUni = LoadUniverse();
    $defenderAfterFreeze = e2e_one_row("SELECT player_id, vacation, vacation_until FROM {$db_prefix}users WHERE player_id={$defenderId} LIMIT 1");
    e2e_restore_uni($uniSnapshot);
    e2e_restore_user($defenderSnapshot);
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_universe_freeze_forces_active_players_into_vacation',
        'checks' => array_merge(e2e_response_check($freezeResponse, 'admin universe freeze POST'), array(
            e2e_case((int)$frozenUni['freeze'] === 1, 'admin universe settings can enable freeze', array('freeze' => $frozenUni['freeze'])),
            e2e_case($defenderAfterFreeze !== null && (int)$defenderAfterFreeze['vacation'] === 1 && (int)$defenderAfterFreeze['vacation_until'] > 0, 'freeze moves active non-admin players into vacation mode', $defenderAfterFreeze ?? array()),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'admin_destructive_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    foreach ($createdQueueIds as $queueId) {
        dbquery("DELETE FROM {$db_prefix}queue WHERE task_id=" . (int)$queueId);
    }
    foreach ($createdPlanetIds as $planetId) {
        dbquery("DELETE FROM {$db_prefix}queue WHERE sub_id=" . (int)$planetId . " OR owner_id={$defenderId}");
        dbquery("DELETE FROM {$db_prefix}buildqueue WHERE planet_id=" . (int)$planetId);
        dbquery("DELETE FROM {$db_prefix}planets WHERE planet_id=" . (int)$planetId . " AND owner_id={$defenderId}");
    }
    if ($attackerId > 0 && $attackerPlanet > 0) {
        e2e_reset_runtime_state($attackerId, $attackerPlanet);
    }
    if ($defenderId > 0 && $defenderPlanet > 0) {
        e2e_reset_runtime_state($defenderId, $defenderPlanet);
    }
    e2e_restore_uni($uniSnapshot);
    e2e_restore_user($attackerSnapshot);
    e2e_restore_user($defenderSnapshot);
}

echo json_encode(array(
    'case_group' => 'http_admin_destructive',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
