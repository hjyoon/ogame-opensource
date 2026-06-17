<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_admin_operations_e2e.php';
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
            'timeout' => 20,
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
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read|file_get_contents|unserialize|scandir)|Notice:\s+Undefined|Unknown column|You have an error in your SQL syntax/i', $body) === 1;
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
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', private_session='" . e2e_sql_escape($private) . "', admin={$adminLevel}, " .
        "validated=1, deact_ip=1, vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, " .
        "disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1 WHERE player_id={$userId}"
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
        "noattack, noattack_until, disable, disable_until, lang, skin, useskin FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1"
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
        "UPDATE {$db_prefix}users SET admin=" . (int)$user['admin'] . ", session='" . e2e_sql_escape($user['session']) . "', " .
        "private_session='" . e2e_sql_escape($user['private_session']) . "', validated=" . (int)$user['validated'] . ", deact_ip=" . (int)$user['deact_ip'] . ", " .
        "vacation=" . (int)$user['vacation'] . ", vacation_until=" . (int)$user['vacation_until'] . ", banned=" . (int)$user['banned'] . ", " .
        "banned_until=" . (int)$user['banned_until'] . ", noattack=" . (int)$user['noattack'] . ", noattack_until=" . (int)$user['noattack_until'] . ", " .
        "disable=" . (int)$user['disable'] . ", disable_until=" . (int)$user['disable_until'] . ", lang='" . e2e_sql_escape($user['lang']) . "', " .
        "skin='" . e2e_sql_escape($user['skin']) . "', useskin=" . (int)$user['useskin'] . " WHERE player_id={$id}"
    );
    InvalidateUserCache();
}

function e2e_admin_url(string $gameBase, string $session, string $mode, string $suffix = ''): string
{
    return $gameBase . '/index.php?page=admin&session=' . rawurlencode($session) . '&mode=' . rawurlencode($mode) . $suffix;
}

function e2e_cleanup_token_rows(int $attackerId, int $defenderId, string $token, int $battleMessageMax): void
{
    global $db_prefix;
    $safeToken = e2e_sql_escape($token);
    dbquery("DELETE FROM {$db_prefix}messages WHERE (owner_id IN ({$attackerId},{$defenderId}) AND (subj LIKE '%{$safeToken}%' OR text LIKE '%{$safeToken}%' OR msgfrom LIKE '%{$safeToken}%')) OR (owner_id={$attackerId} AND pm=" . MTYP_BATTLE_REPORT_TEXT . " AND msg_id>{$battleMessageMax})");
    dbquery("DELETE FROM {$db_prefix}reports WHERE msgfrom LIKE '%{$safeToken}%' OR subj LIKE '%{$safeToken}%' OR text LIKE '%{$safeToken}%'");
}

function e2e_battle_sim_payload(): array
{
    return array(
        'anum' => '1',
        'dnum' => '1',
        'a0_weap' => '0',
        'a0_shld' => '0',
        'a0_armor' => '0',
        'd0_weap' => '0',
        'd0_shld' => '0',
        'd0_armor' => '0',
        'a0_' . GID_F_SC => '1',
        'd0_' . GID_F_SC => '1',
        'rapid' => 'on',
        'fid' => '30',
        'did' => '30',
        'max_round' => '6',
        'battle_source' => '',
    );
}

function e2e_expedition_payload(array $settings, array $overrides = array()): array
{
    $keys = array(
        'chance_success', 'depleted_min', 'depleted_med', 'depleted_max',
        'chance_depleted_min', 'chance_depleted_med', 'chance_depleted_max',
        'chance_alien', 'chance_pirates', 'chance_dm', 'chance_lost',
        'chance_delay', 'chance_accel', 'chance_res', 'chance_fleet',
        'dm_factor', 'score_cap1', 'score_cap2', 'score_cap3', 'score_cap4',
        'score_cap5', 'score_cap6', 'score_cap7', 'score_cap8',
        'limit_cap1', 'limit_cap2', 'limit_cap3', 'limit_cap4',
        'limit_cap5', 'limit_cap6', 'limit_cap7', 'limit_cap8', 'limit_max',
    );
    $payload = array();
    foreach ($keys as $key) {
        $payload[$key] = (string)$settings[$key];
    }
    foreach ($overrides as $key => $value) {
        $payload[$key] = (string)$value;
    }
    return $payload;
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$token = 'E2EOPS' . substr(md5((string)microtime(true)), 0, 8);
$cases = array();
$attackerSnapshot = null;
$defenderSnapshot = null;
$expeditionSnapshot = null;
$battleMessageMax = 0;

try {
    if ($attackerId <= 0 || $defenderId <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $attackerSnapshot = e2e_snapshot_user($attackerId);
    $defenderSnapshot = e2e_snapshot_user($defenderId);
    $expeditionSnapshot = LoadExpeditionSettings();
    $battleMaxRow = e2e_one_row("SELECT COALESCE(MAX(msg_id), 0) AS max_id FROM {$db_prefix}messages WHERE owner_id={$attackerId}");
    $battleMessageMax = $battleMaxRow === null ? 0 : (int)$battleMaxRow['max_id'];
    if ($attackerSnapshot === null || $defenderSnapshot === null || $expeditionSnapshot === null) {
        throw new RuntimeException('Fixture state is missing.');
    }

    e2e_cleanup_token_rows($attackerId, $defenderId, $token, $battleMessageMax);

    $regularAuth = e2e_prepare_session($attackerId, USER_TYPE_PLAYER, 'admin-ops-regular');
    $regularCookies = $regularAuth['cookies'];
    $checks = array();
    foreach (array('Broadcast', 'Reports', 'BattleSim', 'RakSim', 'Expedition') as $mode) {
        $response = e2e_http_request('GET', e2e_admin_url($gameBase, $regularAuth['session'], $mode), array(), $regularCookies);
        $checks = array_merge($checks, e2e_response_check($response, 'regular ' . $mode . ' request'), array(
            e2e_case(stripos($response['body'], 'http-equiv') !== false && stripos($response['body'], 'refresh') !== false, 'regular user is redirected away from ' . $mode),
        ));
    }
    $cases[] = e2e_finalize_case(array(
        'case' => 'regular_user_is_denied_for_admin_operation_modes',
        'checks' => $checks,
    ));

    $adminAuth = e2e_prepare_session($attackerId, USER_TYPE_ADMIN, 'admin-ops-admin');
    $adminCookies = $adminAuth['cookies'];
    e2e_prepare_session($defenderId, USER_TYPE_GO, 'admin-ops-broadcast-target');
    $beforeBroadcast = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}messages WHERE owner_id={$defenderId} AND (subj LIKE '%" . e2e_sql_escape($token) . "%' OR text LIKE '%" . e2e_sql_escape($token) . "%')");
    $broadcastResponse = e2e_http_request('POST', e2e_admin_url($gameBase, $adminAuth['session'], 'Broadcast'), array(
        'cat' => '3',
        'subj' => $token . ' broadcast subject',
        'text' => $token . ' broadcast body',
    ), $adminCookies);
    $afterBroadcast = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}messages WHERE owner_id={$defenderId} AND (subj LIKE '%" . e2e_sql_escape($token) . "%' OR text LIKE '%" . e2e_sql_escape($token) . "%')");
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_broadcast_to_operator_category_creates_message',
        'checks' => array_merge(e2e_response_check($broadcastResponse, 'admin Broadcast POST'), array(
            e2e_case($beforeBroadcast === 0 && $afterBroadcast >= 1, 'broadcast creates a marker message for an operator target', array('before' => $beforeBroadcast, 'after' => $afterBroadcast)),
            e2e_case(strpos($broadcastResponse['body'], 'Message sent to') !== false, 'broadcast response reports a successful send'),
        )),
    ));

    $reportId = AddDBRow(array(
        'owner_id' => $defenderId,
        'msg_id' => 0,
        'msgfrom' => $token . ' reporter',
        'subj' => $token . ' report subject',
        'text' => $token . ' report text',
        'date' => time(),
    ), 'reports');
    $reportsResponse = e2e_http_request('GET', e2e_admin_url($gameBase, $adminAuth['session'], 'Reports'), array(), $adminCookies);
    $operatorAuth = e2e_prepare_session($attackerId, USER_TYPE_GO, 'admin-ops-operator');
    $operatorCookies = $operatorAuth['cookies'];
    $operatorDeleteResponse = e2e_http_request('POST', e2e_admin_url($gameBase, $operatorAuth['session'], 'Reports'), array(
        'delmes' . $reportId => 'on',
        'deletemessages' => 'deletemarked',
    ), $operatorCookies);
    $reportAfterDelete = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}reports WHERE id={$reportId}");
    $cases[] = e2e_finalize_case(array(
        'case' => 'admin_reports_render_and_operator_can_delete_marked_report',
        'checks' => array_merge(
            e2e_response_check($reportsResponse, 'admin Reports GET'),
            e2e_response_check($operatorDeleteResponse, 'operator Reports delete POST'),
            array(
                e2e_case(strpos($reportsResponse['body'], $token . ' report subject') !== false, 'Reports page renders the seeded report marker'),
                e2e_case($reportAfterDelete === 0, 'operator marked delete removes only the seeded report', array('remaining' => $reportAfterDelete)),
            )
        ),
    ));

    $battleResponse = e2e_http_request('POST', e2e_admin_url($gameBase, $operatorAuth['session'], 'BattleSim'), e2e_battle_sim_payload(), $operatorCookies);
    $battleMessageCount = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}messages WHERE owner_id={$attackerId} AND pm=" . MTYP_BATTLE_REPORT_TEXT . " AND msg_id>{$battleMessageMax}");
    $rakResponse = e2e_http_request('POST', e2e_admin_url($gameBase, $operatorAuth['session'], 'RakSim'), array(
        'a_weap' => '10',
        'd_armor' => '10',
        'anz' => '3',
        'pziel' => (string)GID_D_RL,
        'd_' . GID_D_RL => '10',
        'd_' . GID_D_LL => '5',
    ), $operatorCookies);
    $expeditionSimResponse = e2e_http_request('POST', e2e_admin_url($gameBase, $operatorAuth['session'], 'Expedition', '&action=sim'), array(
        'expcount' => '5',
    ), $operatorCookies);
    $cases[] = e2e_finalize_case(array(
        'case' => 'operator_simulator_posts_render_results_without_mutating_game_state_unexpectedly',
        'checks' => array_merge(
            e2e_response_check($battleResponse, 'operator BattleSim POST'),
            e2e_response_check($rakResponse, 'operator RakSim POST'),
            e2e_response_check($expeditionSimResponse, 'operator Expedition sim POST'),
            array(
                e2e_case($battleMessageCount >= 1 && strpos($battleResponse['body'], 'Battle report') !== false, 'BattleSim creates and links a battle report message', array('battle_messages' => $battleMessageCount)),
                e2e_case(strpos($rakResponse['body'], 'Missile attack') !== false && strpos($rakResponse['body'], 'Defense') !== false, 'RakSim renders missile simulation controls after POST'),
                e2e_case(strpos($expeditionSimResponse['body'], 'Expedition simulation result') !== false && strpos($expeditionSimResponse['body'], 'myChart') !== false, 'Expedition simulator renders chart data after POST'),
            )
        ),
    ));

    $originalChance = (int)$expeditionSnapshot['chance_success'];
    $operatorAttemptChance = $originalChance === 99 ? 98 : $originalChance + 1;
    $adminChangeChance = $operatorAttemptChance === 99 ? 97 : $operatorAttemptChance + 1;
    $operatorSettingsResponse = e2e_http_request(
        'POST',
        e2e_admin_url($gameBase, $operatorAuth['session'], 'Expedition', '&action=settings'),
        e2e_expedition_payload($expeditionSnapshot, array('chance_success' => $operatorAttemptChance)),
        $operatorCookies
    );
    $afterOperatorSettings = LoadExpeditionSettings();
    $adminAuth = e2e_prepare_session($attackerId, USER_TYPE_ADMIN, 'admin-ops-expedition-settings-admin');
    $adminCookies = $adminAuth['cookies'];
    $adminSettingsResponse = e2e_http_request(
        'POST',
        e2e_admin_url($gameBase, $adminAuth['session'], 'Expedition', '&action=settings'),
        e2e_expedition_payload($expeditionSnapshot, array('chance_success' => $adminChangeChance)),
        $adminCookies
    );
    $afterAdminSettings = LoadExpeditionSettings();
    SaveExpeditionSettings($expeditionSnapshot);
    $cases[] = e2e_finalize_case(array(
        'case' => 'expedition_settings_mutation_is_admin_only',
        'checks' => array_merge(
            e2e_response_check($operatorSettingsResponse, 'operator Expedition settings POST'),
            e2e_response_check($adminSettingsResponse, 'admin Expedition settings POST'),
            array(
                e2e_case((int)$afterOperatorSettings['chance_success'] === $originalChance, 'operator settings POST does not mutate expedition settings', $afterOperatorSettings ?? array()),
                e2e_case((int)$afterAdminSettings['chance_success'] === $adminChangeChance, 'admin settings POST mutates expedition settings', $afterAdminSettings ?? array()),
            )
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'admin_operations_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($expeditionSnapshot !== null) {
        SaveExpeditionSettings($expeditionSnapshot);
    }
    if ($attackerId > 0 && $defenderId > 0) {
        e2e_cleanup_token_rows($attackerId, $defenderId, $token, $battleMessageMax);
    }
    e2e_restore_user($attackerSnapshot);
    e2e_restore_user($defenderSnapshot);
}

echo json_encode(array(
    'case_group' => 'http_admin_operations',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
