<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_idor_sweep_e2e.php';
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
loca_add('infos', 'en');
loca_add('renameplanet', 'en');
loca_add('resources', 'en');

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
        "lang='en', skin='/evolution/', useskin=1, flags=0, vacation=0, vacation_until=0, " .
        "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, ally_id=0, allyrank=0 " .
        "WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookies' => array('prsess_' . $userId . '_' . $GlobalUni['num'] => $private),
    );
}

function e2e_cleanup_idor(array $userIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', $userIds));
    if ($userList === '') {
        return;
    }

    dbquery("DELETE FROM {$db_prefix}reports WHERE owner_id IN ({$userList}) OR msg_id IN (SELECT msg_id FROM {$db_prefix}messages WHERE owner_id IN ({$userList}))");
    dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$userList})");
    dbquery("UPDATE {$db_prefix}users SET flags=0, ally_id=0, allyrank=0, vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0 WHERE player_id IN ({$userList})");
    InvalidateUserCache();
}

function e2e_prepare_home_planet(int $userId, int $planetId, string $name): void
{
    global $db_prefix;

    $safeName = e2e_sql_escape($name);
    dbquery(
        "UPDATE {$db_prefix}planets SET owner_id={$userId}, type=" . PTYP_PLANET . ", name='{$safeName}', remove=0, " .
        "prod1=1, prod2=1, prod3=1, prod4=1, prod12=1, prod212=1, " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000, " .
        "`" . GID_B_METAL_MINE . "`=10, `" . GID_B_CRYS_MINE . "`=10, `" . GID_B_DEUT_SYNTH . "`=10, `" . GID_B_SOLAR . "`=10, `" . GID_B_FUSION . "`=0, `" . GID_F_SAT . "`=0, " .
        "`" . GID_B_MISS_SILO . "`=0, `" . GID_D_ABM . "`=0, `" . GID_D_IPM . "`=0 " .
        "WHERE planet_id={$planetId}"
    );
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$planetId}, aktplanet={$planetId} WHERE player_id={$userId}");
    InvalidateUserCache();
}

function e2e_planet_snapshot(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, type, name, remove, prod1, prod2, prod3, prod4, prod12, prod212 " .
        "FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1"
    );
}

function e2e_message_row(int $msgId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT msg_id, owner_id, pm, subj, text, shown FROM {$db_prefix}messages WHERE msg_id={$msgId} LIMIT 1");
}

function e2e_set_missile_state(int $planetId, int $ownerId, int $silo, int $abm, int $ipm): void
{
    global $db_prefix;
    dbquery(
        "UPDATE {$db_prefix}planets SET owner_id={$ownerId}, type=" . PTYP_PLANET . ", remove=0, " .
        "`" . GID_B_MISS_SILO . "`={$silo}, `" . GID_D_ABM . "`={$abm}, `" . GID_D_IPM . "`={$ipm}, " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000 " .
        "WHERE planet_id={$planetId}"
    );
}

function e2e_missile_snapshot(int $planetId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT planet_id, owner_id, type, remove, `" . GID_B_MISS_SILO . "` AS silo, `" . GID_D_ABM . "` AS abm, `" . GID_D_IPM . "` AS ipm " .
        "FROM {$db_prefix}planets WHERE planet_id={$planetId} LIMIT 1"
    );
}

function e2e_report_count(int $ownerId, int $msgId): int
{
    global $db_prefix;
    return e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}reports WHERE owner_id={$ownerId} AND msg_id={$msgId}");
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$defenderPlanet = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 0);
$defenderPassword = getenv('OGAME_E2E_DEFENDER_PASSWORD') ?: '';
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$token = 'idor-e2e-' . substr(md5((string)microtime(true)), 0, 10);
$cases = array();

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0 || $defenderPlanet <= 0 || $defenderPassword === '') {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $attackerAuth = e2e_prepare_session($attackerId, 'idor-attacker');
    $defenderAuth = e2e_prepare_session($defenderId, 'idor-defender');
    e2e_prepare_home_planet($attackerId, $attackerPlanet, 'E2E IDOR Attacker');
    e2e_prepare_home_planet($defenderId, $defenderPlanet, 'E2E IDOR Defender');

    e2e_cleanup_idor(array($attackerId, $defenderId));
    $foreignDeleteId = SendMessage($attackerId, 'E2E IDOR', $token . '-foreign-delete', 'foreign delete body', MTYP_MISC, time() + 1);
    $ownDeleteId = SendMessage($defenderId, 'E2E IDOR', $token . '-own-delete', 'own delete body', MTYP_MISC, time() + 2);
    $deleteResponse = e2e_http_request('POST', $gameBase . '/index.php?page=messages&dsp=1&session=' . rawurlencode($defenderAuth['session']), array(
        'deletemessages' => 'deletemarked',
        'delmes' . $foreignDeleteId => 'on',
        'delmes' . $ownDeleteId => 'on',
        'messages' => '1',
    ), $defenderAuth['cookies']);
    $foreignAfterDelete = e2e_message_row($foreignDeleteId);
    $ownAfterDelete = e2e_message_row($ownDeleteId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'selected_message_delete_is_scoped_to_current_owner',
        'checks' => array_merge(e2e_response_check($deleteResponse), array(
            e2e_case($foreignAfterDelete !== null && (int)$foreignAfterDelete['owner_id'] === $attackerId, 'foreign selected message id is ignored', $foreignAfterDelete ?? array()),
            e2e_case($ownAfterDelete === null, 'current user selected message is deleted'),
        )),
    ));

    e2e_cleanup_idor(array($attackerId, $defenderId));
    $foreignBulkId = SendMessage($attackerId, 'E2E IDOR', $token . '-foreign-bulk', 'foreign bulk body', MTYP_MISC, time() + 1);
    $ownBulkId = SendMessage($defenderId, 'E2E IDOR', $token . '-own-bulk', 'own bulk body', MTYP_MISC, time() + 2);
    $bulkResponse = e2e_http_request('POST', $gameBase . '/index.php?page=messages&dsp=1&session=' . rawurlencode($defenderAuth['session']), array(
        'deletemessages' => 'deleteall',
        'messages' => '1',
    ), $defenderAuth['cookies']);
    $cases[] = e2e_finalize_case(array(
        'case' => 'bulk_message_delete_is_scoped_to_current_owner',
        'checks' => array_merge(e2e_response_check($bulkResponse), array(
            e2e_case(e2e_message_row($foreignBulkId) !== null, 'foreign message remains after current user delete-all', array('foreign_id' => $foreignBulkId)),
            e2e_case(e2e_message_row($ownBulkId) === null, 'current user message is removed by delete-all', array('own_id' => $ownBulkId)),
        )),
    ));

    e2e_cleanup_idor(array($attackerId, $defenderId));
    $foreignReportId = SendMessage($attackerId, 'E2E IDOR', $token . '-foreign-report', 'foreign report body', MTYP_PM, time() + 1);
    $ownReportId = SendMessage($defenderId, 'E2E IDOR', $token . '-own-report', 'own report body', MTYP_PM, time() + 2);
    $reportResponse = e2e_http_request('POST', $gameBase . '/index.php?page=messages&dsp=1&session=' . rawurlencode($defenderAuth['session']), array(
        'deletemessages' => 'deletemarked',
        'sneak' . $foreignReportId => 'on',
        'sneak' . $ownReportId => 'on',
        'messages' => '1',
    ), $defenderAuth['cookies']);
    $cases[] = e2e_finalize_case(array(
        'case' => 'message_report_is_scoped_to_current_owner',
        'checks' => array_merge(e2e_response_check($reportResponse), array(
            e2e_case(e2e_report_count($defenderId, $foreignReportId) === 0, 'foreign private-message id is not reported by current user', array('foreign_id' => $foreignReportId)),
            e2e_case(e2e_report_count($defenderId, $ownReportId) === 1, 'current user private-message report is created', array('own_id' => $ownReportId)),
            e2e_case(e2e_message_row($foreignReportId) !== null, 'foreign private message remains readable only by its owner', array('foreign_id' => $foreignReportId)),
        )),
    ));

    e2e_cleanup_idor(array($attackerId, $defenderId));
    e2e_prepare_home_planet($attackerId, $attackerPlanet, 'E2E IDOR Attacker');
    e2e_prepare_home_planet($defenderId, $defenderPlanet, 'E2E IDOR Defender');
    SelectPlanet($defenderId, $defenderPlanet);
    $attackerBeforeResources = e2e_planet_snapshot($attackerPlanet);
    $defenderBeforeResources = e2e_planet_snapshot($defenderPlanet);
    $resourceResponse = e2e_http_request('POST', $gameBase . '/index.php?page=resources&session=' . rawurlencode($defenderAuth['session']) . '&cp=' . $attackerPlanet, array(
        'last1' => 10,
        'last2' => 20,
        'last3' => 30,
        'last4' => 40,
        'last12' => 50,
        'last212' => 60,
        'action' => 'Recalculate',
    ), $defenderAuth['cookies']);
    $attackerAfterResources = e2e_planet_snapshot($attackerPlanet);
    $defenderAfterResources = e2e_planet_snapshot($defenderPlanet);
    $defenderSelectedAfterResources = GetSelectedPlanet($defenderId);
    $cases[] = e2e_finalize_case(array(
        'case' => 'foreign_cp_resource_settings_do_not_mutate_foreign_planet',
        'checks' => array_merge(e2e_response_check($resourceResponse), array(
            e2e_case($attackerBeforeResources !== null && $attackerAfterResources !== null && abs((float)$attackerAfterResources['prod1'] - (float)$attackerBeforeResources['prod1']) < 0.001 && abs((float)$attackerAfterResources['prod2'] - (float)$attackerBeforeResources['prod2']) < 0.001, 'foreign planet production settings are unchanged', array('before' => $attackerBeforeResources, 'after' => $attackerAfterResources)),
            e2e_case($defenderAfterResources !== null && abs((float)$defenderAfterResources['prod1'] - 0.1) < 0.001 && abs((float)$defenderAfterResources['prod2'] - 0.2) < 0.001, 'request applies only to the current owned planet', array('before' => $defenderBeforeResources, 'after' => $defenderAfterResources)),
            e2e_case($defenderSelectedAfterResources === $defenderPlanet, 'foreign cp does not change the selected planet', array('selected' => $defenderSelectedAfterResources, 'expected' => $defenderPlanet)),
        )),
    ));

    e2e_prepare_home_planet($attackerId, $attackerPlanet, 'E2E IDOR Attacker');
    e2e_prepare_home_planet($defenderId, $defenderPlanet, 'E2E IDOR Defender');
    e2e_set_missile_state($attackerPlanet, $attackerId, 2, 2, 4);
    e2e_set_missile_state($defenderPlanet, $defenderId, 2, 2, 4);
    SelectPlanet($defenderId, $defenderPlanet);
    $attackerBeforeSilo = e2e_missile_snapshot($attackerPlanet);
    $defenderBeforeSilo = e2e_missile_snapshot($defenderPlanet);
    $siloResponse = e2e_http_request('POST', $gameBase . '/index.php?page=infos&session=' . rawurlencode($defenderAuth['session']) . '&cp=' . $attackerPlanet . '&gid=' . GID_B_MISS_SILO, array(
        'ab' . GID_D_ABM => 2,
        'ab' . GID_D_IPM => 3,
        'aktion' => '1',
    ), $defenderAuth['cookies']);
    $attackerAfterSilo = e2e_missile_snapshot($attackerPlanet);
    $defenderAfterSilo = e2e_missile_snapshot($defenderPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'foreign_cp_missile_silo_demolition_is_scoped_to_current_owner',
        'checks' => array_merge(e2e_response_check($siloResponse), array(
            e2e_case($attackerBeforeSilo !== null && $attackerAfterSilo !== null && (int)$attackerAfterSilo['abm'] === (int)$attackerBeforeSilo['abm'] && (int)$attackerAfterSilo['ipm'] === (int)$attackerBeforeSilo['ipm'], 'foreign planet missiles are unchanged', array('before' => $attackerBeforeSilo, 'after' => $attackerAfterSilo)),
            e2e_case($defenderBeforeSilo !== null && $defenderAfterSilo !== null && (int)$defenderAfterSilo['abm'] === 0 && (int)$defenderAfterSilo['ipm'] === 1, 'current owner ABM and IPM demolition counts are applied independently', array('before' => $defenderBeforeSilo, 'after' => $defenderAfterSilo)),
            e2e_case(GetSelectedPlanet($defenderId) === $defenderPlanet, 'foreign cp does not move missile silo action off the selected owned planet', array('selected' => GetSelectedPlanet($defenderId), 'expected' => $defenderPlanet)),
        )),
    ));

    e2e_prepare_home_planet($attackerId, $attackerPlanet, 'E2E IDOR Attacker');
    e2e_prepare_home_planet($defenderId, $defenderPlanet, 'E2E IDOR Defender');
    SelectPlanet($defenderId, $defenderPlanet);
    $attackerBeforeDelete = e2e_planet_snapshot($attackerPlanet);
    $deletePlanetResponse = e2e_http_request('POST', $gameBase . '/index.php?page=renameplanet&session=' . rawurlencode($defenderAuth['session']) . '&cp=' . $defenderPlanet . '&pl=' . $attackerPlanet, array(
        'page' => 'renameplanet',
        'deleteid' => $attackerPlanet,
        'pw' => $defenderPassword,
        'aktion' => loca('REN_DELETE_PLANET'),
    ), $defenderAuth['cookies']);
    $attackerAfterDelete = e2e_planet_snapshot($attackerPlanet);
    $cases[] = e2e_finalize_case(array(
        'case' => 'foreign_planet_deleteid_is_rejected',
        'checks' => array_merge(e2e_response_check($deletePlanetResponse), array(
            e2e_case($attackerBeforeDelete !== null && $attackerAfterDelete !== null && (int)$attackerAfterDelete['owner_id'] === $attackerId && (int)$attackerAfterDelete['type'] === PTYP_PLANET && (int)$attackerAfterDelete['remove'] === 0, 'foreign deleteid does not abandon the target planet', array('before' => $attackerBeforeDelete, 'after' => $attackerAfterDelete)),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'idor_sweep_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    $ids = array_filter(array($attackerId, $defenderId), fn($id) => $id > 0);
    if (!empty($ids)) {
        e2e_cleanup_idor($ids);
    }
    if ($attackerId > 0 && $attackerPlanet > 0) {
        e2e_prepare_home_planet($attackerId, $attackerPlanet, 'E2E Fixture Home');
        SelectPlanet($attackerId, $attackerPlanet);
    }
    if ($defenderId > 0 && $defenderPlanet > 0) {
        e2e_prepare_home_planet($defenderId, $defenderPlanet, 'E2E Fixture Defender');
        SelectPlanet($defenderId, $defenderPlanet);
    }
}

echo json_encode(array(
    'case_group' => 'http_idor_sweep',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
