<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_alliance_management_e2e.php';
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

function e2e_cleanup_alliance_management(array $userIds, string $token): void
{
    global $db_prefix;

    $users = implode(',', array_map('intval', $userIds));
    if ($users === '') {
        return;
    }

    $allyIds = array();
    $res = dbquery("SELECT DISTINCT ally_id FROM {$db_prefix}users WHERE player_id IN ({$users}) AND ally_id > 0");
    while ($row = dbarray($res)) {
        $allyIds[] = (int)$row['ally_id'];
    }
    $res = dbquery("SELECT ally_id FROM {$db_prefix}ally WHERE tag LIKE 'E2EAM%' OR name LIKE 'E2EAM%'");
    while ($row = dbarray($res)) {
        $allyIds[] = (int)$row['ally_id'];
    }
    foreach (array_unique($allyIds) as $allyId) {
        DismissAlly($allyId);
    }

    dbquery("DELETE FROM {$db_prefix}allyapps WHERE player_id IN ({$users})");
    dbquery("UPDATE {$db_prefix}users SET ally_id=0, allyrank=0, joindate=0 WHERE player_id IN ({$users})");

    $safe = e2e_sql_escape($token);
    dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$users}) AND (subj LIKE '%{$safe}%' OR text LIKE '%{$safe}%' OR msgfrom LIKE '%{$safe}%')");
}

function e2e_user_snapshot(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT player_id, oname, ally_id, allyrank, joindate FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1");
}

function e2e_ally_snapshot(int $allyId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT ally_id, tag, name, owner_id, homepage, imglogo, open, insertapp, exttext, inttext, apptext, nextrank " .
        "FROM {$db_prefix}ally WHERE ally_id={$allyId} LIMIT 1"
    );
}

function e2e_rank_by_name(int $allyId, string $rankName): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT rank_id, ally_id, name, rights FROM {$db_prefix}allyranks " .
        "WHERE ally_id={$allyId} AND name='" . e2e_sql_escape($rankName) . "' ORDER BY rank_id DESC LIMIT 1"
    );
}

function e2e_rank_by_id(int $allyId, int $rankId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT rank_id, ally_id, name, rights FROM {$db_prefix}allyranks WHERE ally_id={$allyId} AND rank_id={$rankId} LIMIT 1");
}

function e2e_message_count(int $ownerId, string $needle): int
{
    global $db_prefix;
    $safe = e2e_sql_escape($needle);
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}messages WHERE owner_id={$ownerId} AND (subj LIKE '%{$safe}%' OR text LIKE '%{$safe}%' OR msgfrom LIKE '%{$safe}%')");
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$token = 'E2EAM' . substr(md5((string)microtime(true)), 0, 8);
$cases = array();
$allyId = 0;

try {
    if ($attackerId <= 0 || $defenderId <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    e2e_cleanup_alliance_management(array($attackerId, $defenderId), $token);
    $attackerAuth = e2e_prepare_session($attackerId, 'alliance-management-founder');
    $defenderAuth = e2e_prepare_session($defenderId, 'alliance-management-member');
    $attackerCookies = $attackerAuth['cookies'];
    $defenderCookies = $defenderAuth['cookies'];
    $attackerSession = $attackerAuth['session'];
    $defenderSession = $defenderAuth['session'];
    $attackerUser = e2e_user_snapshot($attackerId);
    $defenderUser = e2e_user_snapshot($defenderId);
    $attackerName = $attackerUser['oname'] ?? '';
    $defenderName = $defenderUser['oname'] ?? '';

    $allyTag = substr($token, 0, 8);
    $createResponse = e2e_http_request('POST', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($attackerSession) . '&a=1&weiter=1', array(
        'tag' => $allyTag,
        'name' => $token . ' Alliance',
    ), $attackerCookies);
    $ally = e2e_one_row("SELECT ally_id, tag, name, owner_id FROM {$db_prefix}ally WHERE tag='" . e2e_sql_escape($allyTag) . "' ORDER BY ally_id DESC LIMIT 1");
    $allyId = $ally === null ? 0 : (int)$ally['ally_id'];

    $applyResponse = e2e_http_request('POST', $gameBase . '/index.php?page=bewerben&session=' . rawurlencode($defenderSession) . '&allyid=' . $allyId, array(
        'weiter' => loca('ALLY_APPU_SUBMIT'),
        'text' => $token . ' application text',
    ), $defenderCookies);
    $app = e2e_one_row("SELECT app_id, ally_id, player_id, text FROM {$db_prefix}allyapps WHERE ally_id={$allyId} AND player_id={$defenderId} ORDER BY app_id DESC LIMIT 1");
    $appId = $app === null ? 0 : (int)$app['app_id'];

    $acceptResponse = e2e_http_request('POST', $gameBase . '/index.php?page=bewerbungen&session=' . rawurlencode($attackerSession) . '&show=' . $appId . '&sort=1', array(
        'aktion' => loca('ALLY_APPA_ACCEPT'),
        'text' => '',
    ), $attackerCookies);
    $founderAfterAccept = e2e_user_snapshot($attackerId);
    $memberAfterAccept = e2e_user_snapshot($defenderId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'alliance_management_fixture_founder_and_member',
        'checks' => array_merge(e2e_response_check($createResponse, 'alliance create'), e2e_response_check($applyResponse, 'alliance apply'), e2e_response_check($acceptResponse, 'alliance accept'), array(
            e2e_case($ally !== null && (int)$ally['owner_id'] === $attackerId, 'founder creates an alliance through HTTP', $ally ?? array()),
            e2e_case($app !== null && (int)$app['player_id'] === $defenderId, 'member application is created through HTTP', $app ?? array()),
            e2e_case($founderAfterAccept !== null && (int)$founderAfterAccept['ally_id'] === $allyId && (int)$founderAfterAccept['allyrank'] === 0, 'founder remains rank 0 after accepting member', $founderAfterAccept ?? array()),
            e2e_case($memberAfterAccept !== null && (int)$memberAfterAccept['ally_id'] === $allyId && (int)$memberAfterAccept['allyrank'] === 1, 'accepted member starts as newcomer rank 1', $memberAfterAccept ?? array()),
        )),
    ));

    $unauthorizedRankName = $token . 'BadRank';
    $badRankResponse = e2e_http_request('POST', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($defenderSession) . '&a=15', array(
        'newrangname' => $unauthorizedRankName,
    ), $defenderCookies);
    $badRank = e2e_rank_by_name($allyId, $unauthorizedRankName);

    $unauthorizedCircularText = $token . ' unauthorized circular';
    $unauthorizedCircularResponse = e2e_http_request('POST', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($defenderSession) . '&a=17&sendmail=1', array(
        'r' => 0,
        'text' => $unauthorizedCircularText,
    ), $defenderCookies);
    $unauthorizedFounderMessages = e2e_message_count($attackerId, $unauthorizedCircularText);
    $unauthorizedMemberMessages = e2e_message_count($defenderId, $unauthorizedCircularText);

    $unauthorizedSettingsText = $token . ' unauthorized internal text';
    $unauthorizedSettingsResponse = e2e_http_request('POST', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($defenderSession) . '&a=11&d=1&t=2', array(
        'text' => $unauthorizedSettingsText,
    ), $defenderCookies);
    $allyAfterUnauthorizedSettings = e2e_ally_snapshot($allyId);

    $unauthorizedKickResponse = e2e_http_request('GET', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($defenderSession) . '&a=13&u=' . $attackerId, array(), $defenderCookies);
    $founderAfterUnauthorizedKick = e2e_user_snapshot($attackerId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'newcomer_direct_url_permissions_are_denied',
        'checks' => array_merge(
            e2e_response_check($badRankResponse, 'newcomer rank POST'),
            e2e_response_check($unauthorizedCircularResponse, 'newcomer circular POST'),
            e2e_response_check($unauthorizedSettingsResponse, 'newcomer settings POST'),
            e2e_response_check($unauthorizedKickResponse, 'newcomer kick GET'),
            array(
                e2e_case($badRank === null, 'newcomer cannot create ranks by direct POST', $badRank ?? array()),
                e2e_case(stripos($badRankResponse['body'], 'Not enough permissions') !== false, 'rank POST renders permission denial'),
                e2e_case($unauthorizedFounderMessages === 0 && $unauthorizedMemberMessages === 0, 'newcomer circular POST sends no messages', array('founder_messages' => $unauthorizedFounderMessages, 'member_messages' => $unauthorizedMemberMessages)),
                e2e_case($allyAfterUnauthorizedSettings !== null && strpos($allyAfterUnauthorizedSettings['inttext'], $unauthorizedSettingsText) === false, 'newcomer cannot update internal text by direct POST', $allyAfterUnauthorizedSettings ?? array()),
                e2e_case($founderAfterUnauthorizedKick !== null && (int)$founderAfterUnauthorizedKick['ally_id'] === $allyId && (int)$founderAfterUnauthorizedKick['allyrank'] === 0, 'newcomer cannot kick founder by direct URL', $founderAfterUnauthorizedKick ?? array()),
            )
        ),
    ));

    $rankName = $token . 'Officer';
    $createRankResponse = e2e_http_request('POST', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($attackerSession) . '&a=15', array(
        'newrangname' => $rankName,
    ), $attackerCookies);
    $createdRank = e2e_rank_by_name($allyId, $rankName);
    $rankId = $createdRank === null ? 0 : (int)$createdRank['rank_id'];
    $rightsMask = ARANK_R_MEMBERS | ARANK_W_MEMBERS | ARANK_CIRCULAR;
    $rightsResponse = e2e_http_request('POST', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($attackerSession) . '&a=15', array(
        'u' . $rankId . 'r3' => 'on',
        'u' . $rankId . 'r5' => 'on',
        'u' . $rankId . 'r7' => 'on',
    ), $attackerCookies);
    $rankAfterRights = e2e_rank_by_id($allyId, $rankId);
    $assignRankResponse = e2e_http_request('POST', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($attackerSession) . '&a=16&u=' . $defenderId, array(
        'newrang' => $rankId,
    ), $attackerCookies);
    $memberAfterRank = e2e_user_snapshot($defenderId);
    $memberHomeResponse = e2e_http_request('GET', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($defenderSession), array(), $defenderCookies);
    $memberListResponse = e2e_http_request('GET', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($defenderSession) . '&a=4', array(), $defenderCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'founder_creates_rank_sets_rights_and_assigns_member',
        'checks' => array_merge(
            e2e_response_check($createRankResponse, 'founder rank create POST'),
            e2e_response_check($rightsResponse, 'founder rank rights POST'),
            e2e_response_check($assignRankResponse, 'founder member rank POST'),
            e2e_response_check($memberHomeResponse, 'member alliance home'),
            e2e_response_check($memberListResponse, 'member list page'),
            array(
                e2e_case($createdRank !== null && (int)$createdRank['rights'] === 0, 'custom rank is created with no rights by default', $createdRank ?? array()),
                e2e_case($rankAfterRights !== null && (int)$rankAfterRights['rights'] === $rightsMask, 'custom rank receives member-read, management, and circular rights', array('rank' => $rankAfterRights, 'expected_mask' => $rightsMask)),
                e2e_case($memberAfterRank !== null && (int)$memberAfterRank['allyrank'] === $rankId, 'founder assigns custom rank to member', $memberAfterRank ?? array()),
                e2e_case(stripos($memberHomeResponse['body'], 'alliance management') !== false && stripos($memberHomeResponse['body'], 'Send General Message') !== false, 'member home exposes links granted by custom rank'),
                e2e_case($attackerName !== '' && $defenderName !== '' && stripos($memberListResponse['body'], $attackerName) !== false && stripos($memberListResponse['body'], $defenderName) !== false, 'member can view alliance member list with granted right', array('founder' => $attackerName, 'member' => $defenderName)),
            )
        ),
    ));

    $circularText = $token . ' rank scoped circular';
    $founderMessagesBeforeCircular = e2e_message_count($attackerId, $circularText);
    $memberMessagesBeforeCircular = e2e_message_count($defenderId, $circularText);
    $circularResponse = e2e_http_request('POST', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($defenderSession) . '&a=17&sendmail=1', array(
        'r' => $rankId,
        'text' => $circularText,
    ), $defenderCookies);
    $founderMessagesAfterCircular = e2e_message_count($attackerId, $circularText);
    $memberMessagesAfterCircular = e2e_message_count($defenderId, $circularText);
    $cases[] = e2e_finalize_case(array(
        'case' => 'rank_scoped_circular_message_targets_only_selected_rank',
        'checks' => array_merge(e2e_response_check($circularResponse, 'rank circular POST'), array(
            e2e_case($memberMessagesAfterCircular === $memberMessagesBeforeCircular + 1, 'rank-scoped circular is delivered to the member with the selected rank', array('before' => $memberMessagesBeforeCircular, 'after' => $memberMessagesAfterCircular)),
            e2e_case($founderMessagesAfterCircular === $founderMessagesBeforeCircular, 'rank-scoped circular is not delivered to founder rank when a custom rank is selected', array('before' => $founderMessagesBeforeCircular, 'after' => $founderMessagesAfterCircular)),
            e2e_case($defenderName !== '' && stripos($circularResponse['body'], $defenderName) !== false, 'circular result lists the selected-rank recipient', array('recipient' => $defenderName)),
            e2e_case($attackerName === '' || stripos($circularResponse['body'], $attackerName) === false, 'circular result omits non-selected-rank founder', array('omitted' => $attackerName)),
        )),
    ));

    $internalText = $token . ' managed internal text';
    $settingsTextResponse = e2e_http_request('POST', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($defenderSession) . '&a=11&d=1&t=2', array(
        'text' => $internalText,
    ), $defenderCookies);
    $homepage = 'https://example.local/' . strtolower($token);
    $logo = 'https://example.local/' . strtolower($token) . '.png';
    $founderRankName = $token . 'Lead';
    $settingsMetaResponse = e2e_http_request('POST', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($defenderSession) . '&a=11&d=2', array(
        'hp' => $homepage,
        'logo' => $logo,
        'bew' => 1,
        'fname' => $founderRankName,
    ), $defenderCookies);
    $allyAfterSettings = e2e_ally_snapshot($allyId);
    $founderRankAfterSettings = e2e_rank_by_id($allyId, 0);
    $cases[] = e2e_finalize_case(array(
        'case' => 'management_rank_can_update_alliance_texts_and_settings',
        'checks' => array_merge(e2e_response_check($settingsTextResponse, 'managed text settings POST'), e2e_response_check($settingsMetaResponse, 'managed metadata settings POST'), array(
            e2e_case($allyAfterSettings !== null && $allyAfterSettings['inttext'] === $internalText, 'management-ranked member updates internal alliance text', $allyAfterSettings ?? array()),
            e2e_case($allyAfterSettings !== null && $allyAfterSettings['homepage'] === $homepage && $allyAfterSettings['imglogo'] === $logo, 'management-ranked member updates homepage and logo', $allyAfterSettings ?? array()),
            e2e_case($allyAfterSettings !== null && (int)$allyAfterSettings['open'] === 0, 'management-ranked member can close applications', $allyAfterSettings ?? array()),
            e2e_case($founderRankAfterSettings !== null && $founderRankAfterSettings['name'] === $founderRankName, 'management-ranked member can rename founder rank label', $founderRankAfterSettings ?? array()),
        )),
    ));

    $reassignResponse = e2e_http_request('POST', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($attackerSession) . '&a=16&u=' . $defenderId, array(
        'newrang' => 1,
    ), $attackerCookies);
    $deleteRankResponse = e2e_http_request('GET', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($attackerSession) . '&a=15&d=' . $rankId, array(), $attackerCookies);
    $rankAfterDelete = e2e_rank_by_id($allyId, $rankId);
    $memberAfterReassign = e2e_user_snapshot($defenderId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'founder_reassigns_member_and_deletes_custom_rank',
        'checks' => array_merge(e2e_response_check($reassignResponse, 'founder rank reset POST'), e2e_response_check($deleteRankResponse, 'founder rank delete GET'), array(
            e2e_case($memberAfterReassign !== null && (int)$memberAfterReassign['allyrank'] === 1, 'member is reset to newcomer before custom rank deletion', $memberAfterReassign ?? array()),
            e2e_case($rankAfterDelete === null, 'custom rank is deleted by founder', $rankAfterDelete ?? array()),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'alliance_management_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($allyId > 0) {
        DismissAlly($allyId);
    }
    if ($attackerId > 0 && $defenderId > 0) {
        e2e_cleanup_alliance_management(array($attackerId, $defenderId), $token);
    }
}

echo json_encode(array(
    'case_group' => 'http_alliance_management',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
