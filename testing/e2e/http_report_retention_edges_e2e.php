<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_report_retention_edges_e2e.php';
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
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined|SQL syntax/i', $body) === 1;
    return array(
        e2e_case(in_array($response['status'], array(200, 301, 302, 303), true), 'HTTP request returns an accepted status', array('status' => $response['status'], 'location' => $response['location'])),
        e2e_case(!$hasError, 'HTTP body has no PHP or SQL error marker'),
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

function e2e_prepare_session(int $userId, int $adminLevel, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', private_session='" . e2e_sql_escape($private) . "', " .
        "admin={$adminLevel}, validated=1, deact_ip=1, vacation=0, vacation_until=0, banned=0, banned_until=0, " .
        "noattack=0, noattack_until=0, disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, ally_id=0 " .
        "WHERE player_id={$userId}"
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
    return e2e_one_row("SELECT player_id, admin, session, private_session, validated, deact_ip, vacation, vacation_until, banned, banned_until, noattack, noattack_until, disable, disable_until, lang, skin, useskin, ally_id FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1");
}

function e2e_restore_user(?array $user): void
{
    global $db_prefix;
    if ($user === null) {
        return;
    }

    $id = (int)$user['player_id'];
    dbquery(
        "UPDATE {$db_prefix}users SET admin=" . (int)$user['admin'] . ", session='" . e2e_sql_escape($user['session']) . "', private_session='" . e2e_sql_escape($user['private_session']) . "', " .
        "validated=" . (int)$user['validated'] . ", deact_ip=" . (int)$user['deact_ip'] . ", vacation=" . (int)$user['vacation'] . ", vacation_until=" . (int)$user['vacation_until'] . ", " .
        "banned=" . (int)$user['banned'] . ", banned_until=" . (int)$user['banned_until'] . ", noattack=" . (int)$user['noattack'] . ", noattack_until=" . (int)$user['noattack_until'] . ", " .
        "disable=" . (int)$user['disable'] . ", disable_until=" . (int)$user['disable_until'] . ", lang='" . e2e_sql_escape($user['lang']) . "', skin='" . e2e_sql_escape($user['skin']) . "', " .
        "useskin=" . (int)$user['useskin'] . ", ally_id=" . (int)$user['ally_id'] . " WHERE player_id={$id}"
    );
    InvalidateUserCache();
}

function e2e_cleanup_messages(array $userIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', $userIds));
    if ($userList === '') {
        return;
    }
    dbquery("DELETE FROM {$db_prefix}reports WHERE owner_id IN ({$userList}) OR msg_id IN (SELECT msg_id FROM {$db_prefix}messages WHERE owner_id IN ({$userList}))");
    dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$userList})");
    dbquery("UPDATE {$db_prefix}users SET ally_id=0 WHERE player_id IN ({$userList})");
    InvalidateUserCache();
}

function e2e_message_row(int $msgId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT msg_id, owner_id, pm, subj, shown, date FROM {$db_prefix}messages WHERE msg_id={$msgId} LIMIT 1");
}

function e2e_report_row(int $msgId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT id, owner_id, msg_id, subj, date FROM {$db_prefix}reports WHERE msg_id={$msgId} ORDER BY id DESC LIMIT 1");
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$runToken = 'report-retention-' . substr(md5((string)microtime(true)), 0, 10);
$cases = array();
$attackerSnapshot = null;
$defenderSnapshot = null;

try {
    if ($attackerId <= 0 || $defenderId <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $attackerSnapshot = e2e_snapshot_user($attackerId);
    $defenderSnapshot = e2e_snapshot_user($defenderId);
    $attackerAuth = e2e_prepare_session($attackerId, USER_TYPE_PLAYER, 'report-retention-attacker');
    $defenderAuth = e2e_prepare_session($defenderId, USER_TYPE_PLAYER, 'report-retention-defender');

    e2e_cleanup_messages(array($attackerId, $defenderId));
    $pmId = SendMessage($defenderId, 'E2E PM', $runToken . '-audited-pm', 'Audited private message ' . $runToken, MTYP_PM, time() + 1);
    $reportResponse = e2e_http_request('POST', $gameBase . '/index.php?page=messages&dsp=1&session=' . rawurlencode($defenderAuth['session']), array(
        'deletemessages' => 'deletemarked',
        'sneak' . $pmId => 'on',
        'messages' => '1',
    ), $defenderAuth['cookies']);
    $reportedRow = e2e_report_row($pmId);
    $foreignReportResponse = e2e_http_request('POST', $gameBase . '/index.php?page=messages&dsp=1&session=' . rawurlencode($attackerAuth['session']), array(
        'deletemessages' => 'deletemarked',
        'sneak' . $pmId => 'on',
        'messages' => '1',
    ), $attackerAuth['cookies']);
    $foreignReportCount = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}reports WHERE msg_id={$pmId} AND owner_id={$attackerId}");
    $deleteResponse = e2e_http_request('POST', $gameBase . '/index.php?page=messages&dsp=1&session=' . rawurlencode($defenderAuth['session']), array(
        'deletemessages' => 'deletemarked',
        'delmes' . $pmId => 'on',
        'messages' => '1',
    ), $defenderAuth['cookies']);
    $messageAfterDelete = e2e_message_row($pmId);
    $reportAfterDelete = e2e_report_row($pmId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'reported_pm_audit_report_survives_source_message_deletion_and_foreign_report_post',
        'checks' => array_merge(e2e_response_check($reportResponse), e2e_response_check($foreignReportResponse), e2e_response_check($deleteResponse), array(
            e2e_case($reportedRow !== null && (int)$reportedRow['owner_id'] === $defenderId, 'owner can create one operator report for their PM', $reportedRow ?? array()),
            e2e_case($foreignReportCount === 0, 'foreign user cannot create a report for another user message via crafted POST', array('foreign_report_count' => $foreignReportCount)),
            e2e_case($messageAfterDelete === null, 'source PM is deleted from the user inbox', array('message_id' => $pmId)),
            e2e_case($reportAfterDelete !== null && (int)$reportAfterDelete['owner_id'] === $defenderId, 'operator report row is retained after source PM deletion', $reportAfterDelete ?? array()),
        )),
    ));

    e2e_cleanup_messages(array($attackerId, $defenderId));
    $oldDefenderId = SendMessage($defenderId, 'E2E Old', $runToken . '-old-defender', 'expired defender message', MTYP_MISC, time() - 3 * 24 * 60 * 60);
    $freshDefenderId = SendMessage($defenderId, 'E2E Fresh', $runToken . '-fresh-defender', 'fresh defender message', MTYP_MISC, time() + 1);
    $oldAttackerAdminId = SendMessage($attackerId, 'E2E Admin Old', $runToken . '-old-admin', 'old admin message', MTYP_MISC, time() - 3 * 24 * 60 * 60);
    $defenderExpireResponse = e2e_http_request('GET', $gameBase . '/index.php?page=messages&dsp=1&session=' . rawurlencode($defenderAuth['session']), array(), $defenderAuth['cookies']);
    $oldDefenderAfter = e2e_message_row($oldDefenderId);
    $freshDefenderAfter = e2e_message_row($freshDefenderId);
    $oldAdminBeforeAdminOpen = e2e_message_row($oldAttackerAdminId);
    $attackerAdminAuth = e2e_prepare_session($attackerId, USER_TYPE_GO, 'report-retention-admin');
    $adminExpireResponse = e2e_http_request('GET', $gameBase . '/index.php?page=messages&dsp=1&session=' . rawurlencode($attackerAdminAuth['session']), array(), $attackerAdminAuth['cookies']);
    $oldAdminAfter = e2e_message_row($oldAttackerAdminId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'message_expiry_cleanup_is_owner_scoped_and_skips_admin_users',
        'checks' => array_merge(e2e_response_check($defenderExpireResponse), e2e_response_check($adminExpireResponse), array(
            e2e_case($oldDefenderAfter === null, 'regular user old message is expired when that owner opens messages', array('old_defender_id' => $oldDefenderId)),
            e2e_case($freshDefenderAfter !== null, 'regular user fresh message remains after expiry cleanup', $freshDefenderAfter ?? array()),
            e2e_case($oldAdminBeforeAdminOpen !== null, 'another owner old message is not deleted by defender cleanup', $oldAdminBeforeAdminOpen ?? array()),
            e2e_case($oldAdminAfter !== null, 'admin/operator old message is retained when admin opens messages', $oldAdminAfter ?? array()),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'report_retention_edges_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e), 'trace' => $e->getTraceAsString()))),
        'pass' => false,
    );
} finally {
    if ($attackerId > 0 && $defenderId > 0) {
        e2e_cleanup_messages(array($attackerId, $defenderId));
    }
    e2e_restore_user($attackerSnapshot);
    e2e_restore_user($defenderSnapshot);
}

echo json_encode(array(
    'case_group' => 'http_report_retention_edges',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
