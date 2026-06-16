<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_account_actions_e2e.php';
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

function e2e_response_check(array $response): array
{
    $body = $response['body'];
    $hasError = preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read)|Notice:\s+Undefined/i', $body) === 1;
    return array(
        e2e_case(in_array($response['status'], array(200, 301, 302, 303), true), 'HTTP action returns an accepted status', array('status' => $response['status'], 'location' => $response['location'])),
        e2e_case(!$hasError, 'HTTP action body has no PHP error marker'),
    );
}

function e2e_prepare_session(int $userId): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5('actions-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5('actions-private-' . $userId . '-' . microtime(true));
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

function e2e_finalize_case(array $case): array
{
    $case['pass'] = array_reduce($case['checks'], fn($ok, $check) => $ok && $check['pass'], true);
    return $case;
}

function e2e_reset_planet_state(int $userId, int $planetId): void
{
    global $db_prefix;

    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$userId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$userId} OR planet_id={$planetId}");
    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000, " .
        "prod1=1, prod2=1, prod3=1, prod4=1, prod12=1, prod212=1, " .
        "fields=0, maxfields=200, type=" . PTYP_PLANET . ", owner_id={$userId} WHERE planet_id={$planetId}"
    );
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$attackerPlanet = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$runToken = 'e2e-' . substr(md5((string)microtime(true)), 0, 10);
$cases = array();

try {
    if ($attackerId <= 0 || $attackerPlanet <= 0 || $defenderId <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    e2e_reset_planet_state($attackerId, $attackerPlanet);
    $auth = e2e_prepare_session($attackerId);
    $cookies = $auth['cookies'];
    $sess = $auth['session'];
    $user = LoadUser($attackerId);

    $noteSubject = $runToken . '-note';
    $noteText = 'E2E note body ' . $runToken;
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=notizen&session=' . rawurlencode($sess), array(
        's' => 1,
        'u' => 2,
        'betreff' => $noteSubject,
        'text' => $noteText,
    ), $cookies);
    $note = e2e_one_row("SELECT note_id, owner_id, subj, text, textsize, prio FROM {$db_prefix}notes WHERE owner_id={$attackerId} AND subj='" . e2e_sql_escape($noteSubject) . "' ORDER BY note_id DESC LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'create_note',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($note !== null, 'note row is created'),
            e2e_case($note !== null && (int)$note['prio'] === 2, 'note priority is persisted', $note ?? array()),
            e2e_case($note !== null && (int)$note['textsize'] === strlen($noteText), 'note text size is persisted', $note ?? array()),
        )),
    ));

    $messageSubject = $runToken . '-message';
    $messageText = 'E2E private message body ' . $runToken;
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=writemessages&session=' . rawurlencode($sess) . '&gesendet=1&messageziel=' . $attackerId, array(
        'betreff' => $messageSubject,
        'text' => $messageText,
    ), $cookies);
    $message = e2e_one_row(
        "SELECT msg_id, owner_id, subj, text, shown FROM {$db_prefix}messages " .
        "WHERE owner_id={$attackerId} AND subj LIKE '" . e2e_sql_escape($messageSubject) . "%' ORDER BY msg_id DESC LIMIT 1"
    );
    $cases[] = e2e_finalize_case(array(
        'case' => 'send_private_message',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($message !== null, 'private message row is created'),
            e2e_case($message !== null && strpos($message['text'], $messageText) !== false, 'private message body is persisted', $message ?? array()),
            e2e_case($message !== null && (int)$message['shown'] === 0, 'private message starts unread', $message ?? array()),
        )),
    ));

    $newPlanetName = 'E2E ' . substr($runToken, -8);
    $response = e2e_http_request('POST', $gameBase . '/index.php?page=renameplanet&session=' . rawurlencode($sess) . '&pl=' . $attackerPlanet, array(
        'page' => 'renameplanet',
        'newname' => $newPlanetName,
        'aktion' => 'Rename',
    ), $cookies);
    $planet = e2e_one_row("SELECT planet_id, name FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'rename_planet',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($planet !== null && $planet['name'] === $newPlanetName, 'planet name is updated', $planet ?? array()),
        )),
    ));

    $response = e2e_http_request('POST', $gameBase . '/index.php?page=options&session=' . rawurlencode($sess) . '&mode=change', array(
        'db_character' => $user['oname'],
        'db_password' => '',
        'newpass1' => '',
        'newpass2' => '',
        'db_email' => $user['pemail'],
        'dpath' => '/evolution/',
        'design' => 'on',
        'lang' => 'en',
        'settings_sort' => 1,
        'settings_order' => 1,
        'noipcheck' => 'on',
        'spio_anz' => 7,
        'settings_fleetactions' => 8,
    ), $cookies);
    $updatedUser = e2e_one_row("SELECT player_id, maxspy, maxfleetmsg, sortby, sortorder, deact_ip, lang, skin, useskin FROM {$db_prefix}users WHERE player_id={$attackerId} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'save_options',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($updatedUser !== null && (int)$updatedUser['maxspy'] === 7, 'spy probe count setting is saved', $updatedUser ?? array()),
            e2e_case($updatedUser !== null && (int)$updatedUser['maxfleetmsg'] === 8, 'fleet message count setting is saved', $updatedUser ?? array()),
            e2e_case($updatedUser !== null && (int)$updatedUser['sortby'] === 1 && (int)$updatedUser['sortorder'] === 1, 'sort settings are saved', $updatedUser ?? array()),
            e2e_case($updatedUser !== null && (int)$updatedUser['deact_ip'] === 1, 'IP check disabled setting is saved for stable E2E login', $updatedUser ?? array()),
        )),
    ));

    $response = e2e_http_request('POST', $gameBase . '/index.php?page=resources&session=' . rawurlencode($sess), array(
        'last1' => 80,
        'last2' => 70,
        'last3' => 60,
        'last4' => 100,
        'last12' => 90,
        'last212' => 50,
        'action' => 'Recalculate',
    ), $cookies);
    $resourcePlanet = e2e_one_row("SELECT planet_id, prod1, prod2, prod3, prod4, prod12, prod212 FROM {$db_prefix}planets WHERE planet_id={$attackerPlanet} LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'save_resource_settings',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($resourcePlanet !== null && abs((float)$resourcePlanet['prod1'] - 0.8) < 0.001, 'metal mine production factor is saved', $resourcePlanet ?? array()),
            e2e_case($resourcePlanet !== null && abs((float)$resourcePlanet['prod2'] - 0.7) < 0.001, 'crystal mine production factor is saved', $resourcePlanet ?? array()),
            e2e_case($resourcePlanet !== null && abs((float)$resourcePlanet['prod3'] - 0.6) < 0.001, 'deuterium synthesizer production factor is saved', $resourcePlanet ?? array()),
            e2e_case($resourcePlanet !== null && abs((float)$resourcePlanet['prod212'] - 0.5) < 0.001, 'solar satellite production factor is saved', $resourcePlanet ?? array()),
        )),
    ));

    e2e_reset_planet_state($attackerId, $attackerPlanet);
    $response = e2e_http_request('GET', $gameBase . '/index.php?page=b_building&session=' . rawurlencode($sess) . '&modus=add&techid=' . GID_B_METAL_MINE . '&planet=' . $attackerPlanet, array(), $cookies);
    $buildRow = e2e_one_row("SELECT id, owner_id, planet_id, list_id, tech_id, level, destroy FROM {$db_prefix}buildqueue WHERE owner_id={$attackerId} AND planet_id={$attackerPlanet} ORDER BY id DESC LIMIT 1");
    $buildQueueCount = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type='" . QTYP_BUILD . "'");
    $cases[] = e2e_finalize_case(array(
        'case' => 'enqueue_building',
        'checks' => array_merge(e2e_response_check($response), array(
            e2e_case($buildRow !== null, 'building queue row is created'),
            e2e_case($buildRow !== null && (int)$buildRow['tech_id'] === GID_B_METAL_MINE && (int)$buildRow['level'] >= 1, 'metal mine build target is queued', $buildRow ?? array()),
            e2e_case($buildQueueCount >= 1, 'global build queue task is created', array('queue_count' => $buildQueueCount)),
        )),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'account_actions_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    if ($attackerId > 0) {
        dbquery("DELETE FROM {$db_prefix}notes WHERE owner_id={$attackerId} AND subj LIKE '" . e2e_sql_escape($runToken) . "%'");
        dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id={$attackerId} AND subj LIKE '" . e2e_sql_escape($runToken) . "%'");
        dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$attackerId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "')");
        dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$attackerId}");
    }
    if ($attackerId > 0 && $attackerPlanet > 0) {
        dbquery("UPDATE {$db_prefix}planets SET prod1=1, prod2=1, prod3=1, prod4=1, prod12=1, prod212=1 WHERE planet_id={$attackerPlanet}");
    }
}

echo json_encode(array(
    'case_group' => 'http_account_actions',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
