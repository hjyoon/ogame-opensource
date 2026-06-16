<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_message_lifecycle_e2e.php';
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
loca_add('messages', 'en');
loca_add('espionage', 'en');
loca_add('battlereport', 'en');

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

function e2e_prepare_session(int $userId, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', " .
        "private_session='" . e2e_sql_escape($private) . "', validated=1, deact_ip=1, " .
        "lang='en', skin='/evolution/', useskin=1, flags=0, vacation=0, vacation_until=0, " .
        "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, ally_id=0 " .
        "WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookies' => array('prsess_' . $userId . '_' . $GlobalUni['num'] => $private),
    );
}

function e2e_cleanup_messages(array $userIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', $userIds));
    dbquery("DELETE FROM {$db_prefix}reports WHERE owner_id IN ({$userList}) OR msg_id IN (SELECT msg_id FROM {$db_prefix}messages WHERE owner_id IN ({$userList}))");
    dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$userList})");
    dbquery("UPDATE {$db_prefix}users SET flags=0, ally_id=0 WHERE player_id IN ({$userList})");
    InvalidateUserCache();
}

function e2e_message_row(int $msgId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT msg_id, owner_id, pm, subj, text, shown, date, planet_id FROM {$db_prefix}messages WHERE msg_id={$msgId} LIMIT 1");
}

function e2e_message_ids_for_subject_prefix(int $ownerId, string $prefix): array
{
    global $db_prefix;

    $ids = array();
    $res = dbquery("SELECT msg_id FROM {$db_prefix}messages WHERE owner_id={$ownerId} AND subj LIKE '" . e2e_sql_escape($prefix) . "%' ORDER BY date DESC, msg_id DESC");
    while ($row = dbarray($res)) {
        $ids[] = (int)$row['msg_id'];
    }
    return $ids;
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderPlanet = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$runToken = 'msg-e2e-' . substr(md5((string)microtime(true)), 0, 10);
$cases = array();

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $attackerAuth = e2e_prepare_session($attackerId, 'message-life-attacker');
    $defenderAuth = e2e_prepare_session($defenderId, 'message-life-defender');

    e2e_cleanup_messages(array($attackerId, $defenderId));
    $readIds = array(
        SendMessage($attackerId, 'E2E System', $runToken . '-read-private', 'Private message body ' . $runToken, MTYP_PM, time() + 1),
        SendMessage($attackerId, 'E2E Spy', $runToken . '-read-spy', '<table><tr><td>Spy message body ' . $runToken . '</td></tr></table>', MTYP_SPY_REPORT, time() + 2, $defenderPlanet),
        SendMessage($attackerId, 'E2E Expedition', $runToken . '-read-expedition', 'Expedition message body ' . $runToken, MTYP_EXP, time() + 3),
        SendMessage($attackerId, 'E2E Misc', $runToken . '-read-misc', 'Misc message body ' . $runToken, MTYP_MISC, time() + 4),
    );
    $unreadBefore = UnreadMessages($attackerId);
    $messagesResponse = e2e_http_request('GET', $gameBase . '/index.php?page=messages&dsp=1&session=' . rawurlencode($attackerAuth['session']), array(), $attackerAuth['cookies']);
    $shownAfter = array();
    foreach ($readIds as $msgId) {
        $row = e2e_message_row($msgId);
        $shownAfter[$msgId] = $row === null ? null : (int)$row['shown'];
    }
    $cases[] = e2e_finalize_case(array(
        'case' => 'messages_page_marks_visible_messages_read',
        'checks' => array_merge(e2e_response_check($messagesResponse), array(
            e2e_case($unreadBefore === count($readIds), 'messages start unread before opening inbox', array('unread_before' => $unreadBefore, 'expected' => count($readIds))),
            e2e_case(strpos($messagesResponse['body'], $runToken . '-read-private') !== false && strpos($messagesResponse['body'], $runToken . '-read-spy') !== false, 'message page renders newly created messages'),
            e2e_case(count(array_filter($shownAfter, fn($shown) => $shown === 1)) === count($readIds), 'opening inbox marks displayed messages as read', array('shown_after' => $shownAfter)),
            e2e_case(UnreadMessages($attackerId) === 0, 'unread count is cleared after opening inbox', array('unread_after' => UnreadMessages($attackerId))),
        )),
    ));

    e2e_cleanup_messages(array($attackerId, $defenderId));
    $keepId = SendMessage($attackerId, 'E2E System', $runToken . '-keep', 'Keep this message', MTYP_MISC, time() + 1);
    $deleteId = SendMessage($attackerId, 'E2E System', $runToken . '-delete-marked', 'Delete this message', MTYP_MISC, time() + 2);
    $deleteResponse = e2e_http_request('POST', $gameBase . '/index.php?page=messages&dsp=1&session=' . rawurlencode($attackerAuth['session']), array(
        'deletemessages' => 'deletemarked',
        'delmes' . $deleteId => 'on',
        'messages' => '1',
    ), $attackerAuth['cookies']);
    $cases[] = e2e_finalize_case(array(
        'case' => 'delete_marked_message_removes_only_selected',
        'checks' => array_merge(e2e_response_check($deleteResponse), array(
            e2e_case(e2e_message_row($deleteId) === null, 'selected message is deleted', array('deleted_id' => $deleteId)),
            e2e_case(e2e_message_row($keepId) !== null, 'unselected message remains', array('kept_id' => $keepId)),
        )),
    ));

    e2e_cleanup_messages(array($attackerId, $defenderId));
    $bulkPrefix = $runToken . '-bulk-';
    for ($i = 0; $i < 30; $i++) {
        SendMessage($attackerId, 'E2E Bulk', $bulkPrefix . sprintf('%02d', $i), 'Bulk message ' . $i, MTYP_MISC, time() + $i);
    }
    $bulkBefore = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}messages WHERE owner_id={$attackerId}");
    $bulkDeleteResponse = e2e_http_request('POST', $gameBase . '/index.php?page=messages&dsp=1&session=' . rawurlencode($attackerAuth['session']), array(
        'deletemessages' => 'deleteallshown',
        'messages' => '1',
    ), $attackerAuth['cookies']);
    $remainingBulkIds = e2e_message_ids_for_subject_prefix($attackerId, $bulkPrefix);
    $remainingSubjects = array();
    $res = dbquery("SELECT subj FROM {$db_prefix}messages WHERE owner_id={$attackerId} AND subj LIKE '" . e2e_sql_escape($bulkPrefix) . "%' ORDER BY subj ASC");
    while ($row = dbarray($res)) {
        $remainingSubjects[] = $row['subj'];
    }
    $cases[] = e2e_finalize_case(array(
        'case' => 'delete_all_shown_respects_visible_message_limit',
        'checks' => array_merge(e2e_response_check($bulkDeleteResponse), array(
            e2e_case($bulkBefore === 30, 'bulk setup creates thirty messages', array('before' => $bulkBefore)),
            e2e_case(count($remainingBulkIds) === 5, 'delete all displayed messages removes only the visible page size for non-Commander users', array('remaining_count' => count($remainingBulkIds))),
            e2e_case($remainingSubjects === array($bulkPrefix . '00', $bulkPrefix . '01', $bulkPrefix . '02', $bulkPrefix . '03', $bulkPrefix . '04'), 'oldest messages remain after newest visible page is deleted', array('remaining_subjects' => $remainingSubjects)),
        )),
    ));

    e2e_cleanup_messages(array($attackerId, $defenderId));
    $pmSubject = $runToken . '-report-private';
    $pmResponse = e2e_http_request('POST', $gameBase . '/index.php?page=writemessages&session=' . rawurlencode($attackerAuth['session']) . '&gesendet=1&messageziel=' . $defenderId, array(
        'betreff' => $pmSubject,
        'text' => 'Private reportable message ' . $runToken,
    ), $attackerAuth['cookies']);
    $pmMessage = e2e_one_row("SELECT msg_id, owner_id, pm, subj, text FROM {$db_prefix}messages WHERE owner_id={$defenderId} AND subj LIKE '" . e2e_sql_escape($pmSubject) . "%' ORDER BY msg_id DESC LIMIT 1");
    $reportResponse = $pmMessage === null ? array('status' => 0, 'location' => '', 'body' => '') : e2e_http_request('POST', $gameBase . '/index.php?page=messages&dsp=1&session=' . rawurlencode($defenderAuth['session']), array(
        'deletemessages' => 'deletemarked',
        'sneak' . (int)$pmMessage['msg_id'] => 'on',
        'messages' => '1',
    ), $defenderAuth['cookies']);
    $reportCountAfterFirst = $pmMessage === null ? 0 : e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}reports WHERE owner_id={$defenderId} AND msg_id=" . (int)$pmMessage['msg_id']);
    $duplicateReportResponse = $pmMessage === null ? array('status' => 0, 'location' => '', 'body' => '') : e2e_http_request('POST', $gameBase . '/index.php?page=messages&dsp=1&session=' . rawurlencode($defenderAuth['session']), array(
        'deletemessages' => 'deletemarked',
        'sneak' . (int)$pmMessage['msg_id'] => 'on',
        'messages' => '1',
    ), $defenderAuth['cookies']);
    $reportCountAfterSecond = $pmMessage === null ? 0 : e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}reports WHERE owner_id={$defenderId} AND msg_id=" . (int)$pmMessage['msg_id']);
    $cases[] = e2e_finalize_case(array(
        'case' => 'private_message_report_creates_operator_report_once',
        'checks' => array_merge(e2e_response_check($pmResponse), e2e_response_check($reportResponse), e2e_response_check($duplicateReportResponse), array(
            e2e_case($pmMessage !== null && (int)$pmMessage['pm'] === MTYP_PM, 'HTTP private message send creates a PM for the recipient', $pmMessage ?? array()),
            e2e_case($reportCountAfterFirst === 1, 'reporting a private message creates one operator report', array('report_count' => $reportCountAfterFirst)),
            e2e_case($reportCountAfterSecond === 1, 'reporting the same private message twice does not duplicate operator reports', array('report_count' => $reportCountAfterSecond)),
        )),
    ));

    e2e_cleanup_messages(array($attackerId, $defenderId));
    $spySecret = $runToken . '-spy-secret';
    $battleSecret = $runToken . '-battle-secret';
    $spyId = SendMessage($attackerId, 'E2E Spy', $runToken . '-spy-report', '<table><tr><td>' . $spySecret . '</td></tr></table>', MTYP_SPY_REPORT, time() + 1, $defenderPlanet);
    $battleTextId = SendMessage($attackerId, 'E2E Battle', $runToken . '-battle-text', '<table><tr><td>' . $battleSecret . '</td></tr></table>', MTYP_BATTLE_REPORT_TEXT, time() + 2);
    $ownerSpyResponse = e2e_http_request('GET', $gameBase . '/index.php?page=bericht&session=' . rawurlencode($attackerAuth['session']) . '&bericht=' . $spyId, array(), $attackerAuth['cookies']);
    $foreignSpyResponse = e2e_http_request('GET', $gameBase . '/index.php?page=bericht&session=' . rawurlencode($defenderAuth['session']) . '&bericht=' . $spyId, array(), $defenderAuth['cookies']);
    dbquery("UPDATE {$db_prefix}users SET ally_id=424242 WHERE player_id IN ({$attackerId},{$defenderId})");
    InvalidateUserCache();
    $alliedSpyResponse = e2e_http_request('GET', $gameBase . '/index.php?page=bericht&session=' . rawurlencode($defenderAuth['session']) . '&bericht=' . $spyId, array(), $defenderAuth['cookies']);
    $foreignBattleResponse = e2e_http_request('GET', $gameBase . '/index.php?page=bericht&session=' . rawurlencode($defenderAuth['session']) . '&bericht=' . $battleTextId, array(), $defenderAuth['cookies']);
    dbquery("UPDATE {$db_prefix}users SET ally_id=0 WHERE player_id IN ({$attackerId},{$defenderId})");
    InvalidateUserCache();
    $ownerBattleResponse = e2e_http_request('GET', $gameBase . '/index.php?page=bericht&session=' . rawurlencode($attackerAuth['session']) . '&bericht=' . $battleTextId, array(), $attackerAuth['cookies']);
    $cases[] = e2e_finalize_case(array(
        'case' => 'report_popup_access_controls_owner_and_alliance_spy_reports',
        'checks' => array_merge(
            e2e_response_check($ownerSpyResponse),
            e2e_response_check($foreignSpyResponse),
            e2e_response_check($alliedSpyResponse),
            e2e_response_check($foreignBattleResponse),
            e2e_response_check($ownerBattleResponse),
            array(
                e2e_case(strpos($ownerSpyResponse['body'], $spySecret) !== false, 'spy report owner can read report popup'),
                e2e_case(strpos($foreignSpyResponse['body'], $spySecret) === false, 'foreign user outside alliance cannot read spy report popup'),
                e2e_case(strpos($alliedSpyResponse['body'], $spySecret) !== false, 'same-alliance user can read shared spy report popup'),
                e2e_case(strpos($ownerBattleResponse['body'], $battleSecret) !== false, 'battle report owner can read report popup'),
                e2e_case(strpos($foreignBattleResponse['body'], $battleSecret) === false, 'foreign user cannot read battle report text popup even when allied'),
            )
        ),
    ));

    e2e_cleanup_messages(array($attackerId, $defenderId));
    $deletedSecret = $runToken . '-deleted-report-secret';
    $deletedId = SendMessage($attackerId, 'E2E Spy', $runToken . '-deleted-report', '<table><tr><td>' . $deletedSecret . '</td></tr></table>', MTYP_SPY_REPORT, time() + 1, $defenderPlanet);
    DeleteMessage($attackerId, $deletedId);
    $deletedResponse = e2e_http_request('GET', $gameBase . '/index.php?page=bericht&session=' . rawurlencode($attackerAuth['session']) . '&bericht=' . $deletedId, array(), $attackerAuth['cookies']);
    $cases[] = e2e_finalize_case(array(
        'case' => 'deleted_report_link_renders_without_php_error',
        'checks' => array_merge(e2e_response_check($deletedResponse), array(
            e2e_case(strpos($deletedResponse['body'], $deletedSecret) === false, 'deleted report popup does not expose deleted report body'),
        )),
    ));

    e2e_cleanup_messages(array($attackerId, $defenderId));
    $oldId = SendMessage($attackerId, 'E2E Old', $runToken . '-expired', 'Expired message body', MTYP_MISC, time() - 3 * 24 * 60 * 60);
    $freshId = SendMessage($attackerId, 'E2E New', $runToken . '-fresh', 'Fresh message body', MTYP_MISC, time() + 1);
    $expireResponse = e2e_http_request('GET', $gameBase . '/index.php?page=messages&dsp=1&session=' . rawurlencode($attackerAuth['session']), array(), $attackerAuth['cookies']);
    $cases[] = e2e_finalize_case(array(
        'case' => 'opening_messages_expires_old_non_commander_messages',
        'checks' => array_merge(e2e_response_check($expireResponse), array(
            e2e_case(e2e_message_row($oldId) === null, 'old non-Commander message is deleted when inbox opens', array('old_id' => $oldId)),
            e2e_case(e2e_message_row($freshId) !== null, 'fresh message remains after expiry cleanup', array('fresh_id' => $freshId)),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'message_lifecycle_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($attackerId > 0 && $defenderId > 0) {
        e2e_cleanup_messages(array($attackerId, $defenderId));
    }
}

echo json_encode(array(
    'case_group' => 'http_message_lifecycle',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
