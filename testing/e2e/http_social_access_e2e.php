<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_social_access_e2e.php';
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
loca_add('ally', 'en');
loca_add('buddy', 'en');
loca_add('notes', 'en');
loca_add('build', 'en');
loca_add('battlereport', 'en');
loca_add('espionage', 'en');

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

function e2e_response_check(array $response): array
{
    $body = $response['body'];
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined/i', $body) === 1;
    return array(
        e2e_case(in_array($response['status'], array(200, 301, 302, 303), true), 'HTTP request returns an accepted status', array('status' => $response['status'], 'location' => $response['location'])),
        e2e_case(!$hasError, 'HTTP body has no PHP error marker'),
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
        "lang='en', skin='/evolution/', useskin=1, vacation=0, vacation_until=0, " .
        "disable=0, disable_until=0 WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookies' => array('prsess_' . $userId . '_' . $GlobalUni['num'] => $private),
    );
}

function e2e_cleanup_social(int $attackerId, int $defenderId, string $token = ''): void
{
    global $db_prefix;

    $users = "{$attackerId},{$defenderId}";
    $allyIds = array();
    $res = dbquery("SELECT DISTINCT ally_id FROM {$db_prefix}users WHERE player_id IN ({$users}) AND ally_id > 0");
    while ($row = dbarray($res)) {
        $allyIds[] = (int)$row['ally_id'];
    }
    $res = dbquery("SELECT ally_id FROM {$db_prefix}ally WHERE tag LIKE 'E2ESOC%'");
    while ($row = dbarray($res)) {
        $allyIds[] = (int)$row['ally_id'];
    }
    foreach (array_unique($allyIds) as $allyId) {
        DismissAlly($allyId);
    }

    dbquery("DELETE FROM {$db_prefix}allyapps WHERE player_id IN ({$users})");
    dbquery("DELETE FROM {$db_prefix}buddy WHERE request_from IN ({$users}) OR request_to IN ({$users})");
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id IN ({$users}) AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id IN ({$users})");
    dbquery("UPDATE {$db_prefix}users SET ally_id=0, allyrank=0, joindate=0 WHERE player_id IN ({$users})");

    if ($token !== '') {
        $safe = e2e_sql_escape($token);
        dbquery("DELETE FROM {$db_prefix}notes WHERE owner_id IN ({$users}) AND (subj LIKE '{$safe}%' OR text LIKE '%{$safe}%')");
        dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$users}) AND (subj LIKE '%{$safe}%' OR text LIKE '%{$safe}%' OR msgfrom LIKE '%{$safe}%')");
    }
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderPlanet = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$token = 'E2ESOC' . substr(md5((string)microtime(true)), 0, 8);
$cases = array();
$allyId = 0;

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    e2e_cleanup_social($attackerId, $defenderId, $token);
    $attackerAuth = e2e_prepare_session($attackerId, 'social-attacker');
    $defenderAuth = e2e_prepare_session($defenderId, 'social-defender');
    $attackerCookies = $attackerAuth['cookies'];
    $defenderCookies = $defenderAuth['cookies'];
    $attackerSession = $attackerAuth['session'];
    $defenderSession = $defenderAuth['session'];

    $publicCookies = array();
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=overview', array(), $publicCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'private_page_requires_session',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(stripos($response['body'], 'http-equiv') !== false && stripos($response['body'], 'refresh') !== false, 'unauthenticated private page returns a redirect shell'),
            e2e_case(stripos($response['body'], 'var session=""') === false, 'unauthenticated private page does not render an empty game session'),
            e2e_case(stripos($response['body'], 'reloadImages') === false, 'unauthenticated private page does not render game content scripts'),
        )),
    ));

    $secretReport = $token . '-secret-report';
    $reportId = SendMessage($attackerId, $token, $token . ' report', '<table><tr><th>' . $secretReport . '</th></tr></table>', MTYP_SPY_REPORT, time(), $attackerPlanet);
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=bericht&session=' . rawurlencode($defenderSession) . '&bericht=' . $reportId, array(), $defenderCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'report_owner_access_control',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case(strpos($response['body'], $secretReport) === false, 'non-owner cannot read another user report body'),
        )),
    ));

    AddNote($attackerId, $token . '-note', $token . '-original-note-body', 1);
    $note = e2e_one_row("SELECT note_id, text FROM {$db_prefix}notes WHERE owner_id={$attackerId} AND subj='" . e2e_sql_escape($token . '-note') . "' ORDER BY note_id DESC LIMIT 1");
    $noteId = $note === null ? 0 : (int)$note['note_id'];
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=notizen&session=' . rawurlencode($defenderSession), array(
        's' => 2,
        'n' => $noteId,
        'u' => 2,
        'betreff' => $token . '-stolen-note',
        'text' => $token . '-tampered-note-body',
    ), $defenderCookies);
    $noteAfter = e2e_one_row("SELECT note_id, owner_id, subj, text, prio FROM {$db_prefix}notes WHERE note_id={$noteId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'note_owner_access_control',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($noteAfter !== null && (int)$noteAfter['owner_id'] === $attackerId, 'note remains owned by original user', $noteAfter ?? array()),
            e2e_case($noteAfter !== null && $noteAfter['text'] === $token . '-original-note-body', 'non-owner cannot modify note text', $noteAfter ?? array()),
        )),
    ));

    $buildCountBefore = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}buildqueue WHERE planet_id={$attackerPlanet}");
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=b_building&session=' . rawurlencode($defenderSession) . '&modus=add&techid=' . GID_B_METAL_MINE . '&planet=' . $attackerPlanet, array(), $defenderCookies);
    $buildCountAfter = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}buildqueue WHERE planet_id={$attackerPlanet}");
    $cases[] = e2e_finalize_case(array(
        'case' => 'foreign_planet_build_access_control',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($buildCountAfter === $buildCountBefore, 'foreign user cannot enqueue construction on another player planet', array('before' => $buildCountBefore, 'after' => $buildCountAfter)),
        )),
    ));

    $allyTag = substr($token, 0, 8);
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($attackerSession) . '&a=1&weiter=1', array(
        'tag' => $allyTag,
        'name' => $token . ' Alliance',
    ), $attackerCookies);
    $ally = e2e_one_row("SELECT ally_id, tag, name, owner_id FROM {$db_prefix}ally WHERE tag='" . e2e_sql_escape($allyTag) . "' ORDER BY ally_id DESC LIMIT 1");
    $allyId = $ally === null ? 0 : (int)$ally['ally_id'];
    $attackerAfterCreate = e2e_one_row("SELECT player_id, ally_id, allyrank FROM {$db_prefix}users WHERE player_id={$attackerId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'alliance_create',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($ally !== null && (int)$ally['owner_id'] === $attackerId, 'alliance row is created with attacker as owner', $ally ?? array()),
            e2e_case($attackerAfterCreate !== null && (int)$attackerAfterCreate['ally_id'] === $allyId && (int)$attackerAfterCreate['allyrank'] === 0, 'founder is attached as alliance head', $attackerAfterCreate ?? array()),
        )),
    ));

    $response = e2e_http_request('POST', $gameBase . '/index.php?page=bewerben&session=' . rawurlencode($defenderSession) . '&allyid=' . $allyId, array(
        'weiter' => loca('ALLY_APPU_SUBMIT'),
        'text' => $token . ' application text',
    ), $defenderCookies);
    $app = e2e_one_row("SELECT app_id, ally_id, player_id, text FROM {$db_prefix}allyapps WHERE ally_id={$allyId} AND player_id={$defenderId} ORDER BY app_id DESC LIMIT 1");
    $appId = $app === null ? 0 : (int)$app['app_id'];
    $cases[] = e2e_finalize_case(array(
        'case' => 'alliance_apply',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($app !== null && strpos($app['text'], $token) !== false, 'defender application row is created', $app ?? array()),
        )),
    ));

    $response = e2e_http_request('POST', $gameBase . '/index.php?page=bewerbungen&session=' . rawurlencode($attackerSession) . '&show=' . $appId . '&sort=1', array(
        'aktion' => loca('ALLY_APPA_ACCEPT'),
        'text' => '',
    ), $attackerCookies);
    $defenderAfterAccept = e2e_one_row("SELECT player_id, ally_id, allyrank FROM {$db_prefix}users WHERE player_id={$defenderId} LIMIT 1");
    $appAfterAccept = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}allyapps WHERE app_id={$appId}");
    $cases[] = e2e_finalize_case(array(
        'case' => 'alliance_accept_application',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($defenderAfterAccept !== null && (int)$defenderAfterAccept['ally_id'] === $allyId && (int)$defenderAfterAccept['allyrank'] === 1, 'accepted player joins alliance as newcomer', $defenderAfterAccept ?? array()),
            e2e_case($appAfterAccept === 0, 'accepted application is removed', array('remaining_app_count' => $appAfterAccept)),
        )),
    ));

    $response = e2e_http_request('GET', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($defenderSession) . '&a=13&u=' . $attackerId, array(), $defenderCookies);
    $attackerAfterUnauthorizedKick = e2e_one_row("SELECT player_id, ally_id, allyrank FROM {$db_prefix}users WHERE player_id={$attackerId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'alliance_kick_requires_permission',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($attackerAfterUnauthorizedKick !== null && (int)$attackerAfterUnauthorizedKick['ally_id'] === $allyId && (int)$attackerAfterUnauthorizedKick['allyrank'] === 0, 'newcomer cannot kick the alliance founder by direct URL', $attackerAfterUnauthorizedKick ?? array()),
        )),
    ));

    $response = e2e_http_request('POST', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($defenderSession) . '&a=3&weiter=1', array(), $defenderCookies);
    $defenderAfterLeave = e2e_one_row("SELECT player_id, ally_id FROM {$db_prefix}users WHERE player_id={$defenderId} LIMIT 1");
    $attackerStillFounder = e2e_one_row("SELECT player_id, ally_id FROM {$db_prefix}users WHERE player_id={$attackerId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'alliance_member_leave',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($defenderAfterLeave !== null && (int)$defenderAfterLeave['ally_id'] === 0, 'member can leave alliance', $defenderAfterLeave ?? array()),
            e2e_case($attackerStillFounder !== null && (int)$attackerStillFounder['ally_id'] === $allyId, 'founder remains in alliance after member leaves', $attackerStillFounder ?? array()),
        )),
    ));

    $response = e2e_http_request('POST', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($attackerSession) . '&a=12&weiter=1', array(), $attackerCookies);
    $allyAfterDismiss = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}ally WHERE ally_id={$allyId}");
    $attackerAfterDismiss = e2e_one_row("SELECT player_id, ally_id FROM {$db_prefix}users WHERE player_id={$attackerId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'alliance_dismiss',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($allyAfterDismiss === 0, 'founder can dismiss alliance', array('remaining_ally_count' => $allyAfterDismiss)),
            e2e_case($attackerAfterDismiss !== null && (int)$attackerAfterDismiss['ally_id'] === 0, 'founder is detached after dismissal', $attackerAfterDismiss ?? array()),
        )),
    ));
    $allyId = 0;

    $response = e2e_http_request('POST', $gameBase . '/index.php?page=buddy&session=' . rawurlencode($attackerSession) . '&action=1&buddy_id=' . $defenderId, array(
        'text' => $token . ' rejectable buddy request',
    ), $attackerCookies);
    $rejectBuddy = e2e_one_row("SELECT buddy_id, request_from, request_to, text, accepted FROM {$db_prefix}buddy WHERE request_from={$attackerId} AND request_to={$defenderId} ORDER BY buddy_id DESC LIMIT 1");
    $rejectBuddyId = $rejectBuddy === null ? 0 : (int)$rejectBuddy['buddy_id'];
    $cases[] = e2e_finalize_case(array(
        'case' => 'buddy_request_create',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($rejectBuddy !== null && (int)$rejectBuddy['accepted'] === 0, 'outgoing buddy request is created pending', $rejectBuddy ?? array()),
        )),
    ));

    $response = e2e_http_request('GET', $gameBase . '/index.php?page=buddy&session=' . rawurlencode($attackerSession) . '&action=2&buddy_id=' . $rejectBuddyId, array(), $attackerCookies);
    $selfAcceptedBuddy = e2e_one_row("SELECT buddy_id, accepted FROM {$db_prefix}buddy WHERE buddy_id={$rejectBuddyId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'buddy_accept_requires_recipient',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($selfAcceptedBuddy !== null && (int)$selfAcceptedBuddy['accepted'] === 0, 'sender cannot accept their own outgoing buddy request', $selfAcceptedBuddy ?? array()),
        )),
    ));

    $response = e2e_http_request('GET', $gameBase . '/index.php?page=buddy&session=' . rawurlencode($defenderSession) . '&action=3&buddy_id=' . $rejectBuddyId, array(), $defenderCookies);
    $rejectBuddyCount = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}buddy WHERE buddy_id={$rejectBuddyId}");
    $cases[] = e2e_finalize_case(array(
        'case' => 'buddy_request_reject',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($rejectBuddyCount === 0, 'rejected buddy request is removed', array('remaining_buddy_count' => $rejectBuddyCount)),
        )),
    ));

    $response = e2e_http_request('POST', $gameBase . '/index.php?page=buddy&session=' . rawurlencode($attackerSession) . '&action=1&buddy_id=' . $defenderId, array(
        'text' => $token . ' accepted buddy request',
    ), $attackerCookies);
    $acceptBuddy = e2e_one_row("SELECT buddy_id, request_from, request_to, text, accepted FROM {$db_prefix}buddy WHERE request_from={$attackerId} AND request_to={$defenderId} ORDER BY buddy_id DESC LIMIT 1");
    $acceptBuddyId = $acceptBuddy === null ? 0 : (int)$acceptBuddy['buddy_id'];
    $responseAccept = e2e_http_request('GET', $gameBase . '/index.php?page=buddy&session=' . rawurlencode($defenderSession) . '&action=2&buddy_id=' . $acceptBuddyId, array(), $defenderCookies);
    $acceptedBuddy = e2e_one_row("SELECT buddy_id, request_from, request_to, accepted FROM {$db_prefix}buddy WHERE buddy_id={$acceptBuddyId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'buddy_request_accept',
        'checks' => array_merge(e2e_response_check($response), e2e_response_check($responseAccept), array(
            e2e_case($acceptedBuddy !== null && (int)$acceptedBuddy['accepted'] === 1, 'accepted buddy request becomes a buddy relation', $acceptedBuddy ?? array()),
        )),
    ));

    $response = e2e_http_request('GET', $gameBase . '/index.php?page=buddy&session=' . rawurlencode($attackerSession) . '&action=8&buddy_id=' . $acceptBuddyId, array(), $attackerCookies);
    $acceptedBuddyCount = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}buddy WHERE buddy_id={$acceptBuddyId}");
    $cases[] = e2e_finalize_case(array(
        'case' => 'buddy_delete',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($acceptedBuddyCount === 0, 'accepted buddy relation can be deleted', array('remaining_buddy_count' => $acceptedBuddyCount)),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'social_access_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($allyId > 0) {
        DismissAlly($allyId);
    }
    if ($attackerId > 0 && $defenderId > 0) {
        e2e_cleanup_social($attackerId, $defenderId, $token);
    }
}

echo json_encode(array(
    'case_group' => 'http_social_access',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
