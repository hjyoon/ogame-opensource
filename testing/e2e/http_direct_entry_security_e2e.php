<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_direct_entry_security_e2e.php';
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
loca_add('options', 'en');
loca_add('messages', 'en');

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
    $contentType = '';
    foreach ($responseHeaders as $header) {
        if (preg_match('/^HTTP\/\S+\s+(\d+)/', $header, $m)) {
            $status = (int)$m[1];
        } elseif (stripos($header, 'Location:') === 0) {
            $location = trim(substr($header, 9));
        } elseif (stripos($header, 'Content-Type:') === 0) {
            $contentType = trim(substr($header, 13));
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
        'content_type' => $contentType,
        'headers' => $responseHeaders,
        'body' => $body === false ? '' : $body,
    );
}

function e2e_case(bool $pass, string $message, array $context = array()): array
{
    return array('pass' => $pass, 'message' => $message, 'context' => $context);
}

function e2e_has_error_marker(string $body): bool
{
    return preg_match('/Fatal error|Parse error|Error-ID:|Warning:\s+(Undefined|Trying to access|Attempt to read|file_get_contents|Cannot modify header)|Notice:\s+Undefined|Unknown column|You have an error in your SQL syntax/i', $body) === 1;
}

function e2e_response_check(array $response, string $label, array $accepted = array(200, 301, 302, 303, 400)): array
{
    return array(
        e2e_case(in_array($response['status'], $accepted, true), $label . ' returns an accepted status', array('status' => $response['status'], 'location' => $response['location'])),
        e2e_case(!e2e_has_error_marker($response['body']), $label . ' body has no PHP or SQL error marker'),
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

function e2e_xml_ok(string $xml): bool
{
    $previous = libxml_use_internal_errors(true);
    libxml_clear_errors();
    $parsed = simplexml_load_string($xml);
    $ok = $parsed !== false;
    libxml_clear_errors();
    libxml_use_internal_errors($previous);
    return $ok;
}

function e2e_prepare_session(int $userId, string $label): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5($label . '-session-' . $userId . '-' . microtime(true)), 0, 12);
    $private = md5($label . '-private-' . $userId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . e2e_sql_escape($session) . "', private_session='" . e2e_sql_escape($private) . "', " .
        "admin=" . USER_TYPE_PLAYER . ", validated=1, deact_ip=1, vacation=0, vacation_until=0, banned=0, banned_until=0, " .
        "noattack=0, noattack_until=0, disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1 " .
        "WHERE player_id={$userId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private' => $private,
        'cookies' => array('prsess_' . $userId . '_' . $GlobalUni['num'] => $private),
    );
}

function e2e_user_snapshot(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row(
        "SELECT player_id, session, private_session, admin, validated, deact_ip, vacation, vacation_until, banned, banned_until, " .
        "noattack, noattack_until, disable, disable_until, lang, skin, useskin, ally_id, allyrank, joindate, flags, feedid, lastfeed " .
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
        "UPDATE {$db_prefix}users SET admin=" . (int)$user['admin'] . ", session='" . e2e_sql_escape($user['session']) . "', " .
        "private_session='" . e2e_sql_escape($user['private_session']) . "', validated=" . (int)$user['validated'] . ", deact_ip=" . (int)$user['deact_ip'] . ", " .
        "vacation=" . (int)$user['vacation'] . ", vacation_until=" . (int)$user['vacation_until'] . ", banned=" . (int)$user['banned'] . ", " .
        "banned_until=" . (int)$user['banned_until'] . ", noattack=" . (int)$user['noattack'] . ", noattack_until=" . (int)$user['noattack_until'] . ", " .
        "disable=" . (int)$user['disable'] . ", disable_until=" . (int)$user['disable_until'] . ", lang='" . e2e_sql_escape($user['lang']) . "', " .
        "skin='" . e2e_sql_escape($user['skin']) . "', useskin=" . (int)$user['useskin'] . ", ally_id=" . (int)$user['ally_id'] . ", " .
        "allyrank=" . (int)$user['allyrank'] . ", joindate=" . (int)$user['joindate'] . ", flags=" . (int)$user['flags'] . ", " .
        "feedid='" . e2e_sql_escape($user['feedid']) . "', lastfeed=" . (int)$user['lastfeed'] . " WHERE player_id={$id}"
    );
    InvalidateUserCache();
}

function e2e_set_feed_user(int $userId, string $feedId, int $flags, int $lastfeed): void
{
    global $db_prefix;
    dbquery("UPDATE {$db_prefix}users SET feedid='" . e2e_sql_escape($feedId) . "', flags={$flags}, lastfeed={$lastfeed} WHERE player_id={$userId}");
    InvalidateUserCache();
}

$attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
$defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
$base = rtrim(getenv('OGAME_E2E_HTTP_BASE') ?: 'http://127.0.0.1', '/');
$gameBase = $base . '/game';
$token = 'E2EDIR' . substr(md5((string)microtime(true)), 0, 8);
$cases = array();
$attackerSnapshot = null;
$defenderSnapshot = null;
$uniSnapshot = null;
$createdAlly = 0;

try {
    if ($attackerId <= 0 || $defenderId <= 0) {
        throw new RuntimeException('Fixture environment variables are missing.');
    }

    $attackerSnapshot = e2e_user_snapshot($attackerId);
    $defenderSnapshot = e2e_user_snapshot($defenderId);
    $uniSnapshot = e2e_one_row("SELECT feedage FROM {$db_prefix}uni LIMIT 1");
    if ($attackerSnapshot === null || $defenderSnapshot === null || $uniSnapshot === null) {
        throw new RuntimeException('Fixture users or universe row are missing.');
    }

    $checks = array();
    $unsafeRedirects = array(
        'javascript:alert(1)',
        'data:text/html,<script>' . $token . '</script>',
        'file:///etc/passwd',
        'http://127.0.0.1:8888/game/index.php',
        'http://localhost:8888/game/index.php',
        'http://[::1]/game/index.php',
        'http://[::ffff:127.0.0.1]/game/index.php',
        'http://169.254.169.254/latest/meta-data/',
        'http://example.com@127.0.0.1/image.png',
    );
    foreach ($unsafeRedirects as $unsafeUrl) {
        $response = e2e_http_request('GET', $gameBase . '/redir.php?url=' . rawurlencode($unsafeUrl));
        $checks = array_merge($checks, e2e_response_check($response, 'redir unsafe target'));
        $checks[] = e2e_case($response['location'] === '', 'redir does not issue Location for unsafe URL', array('url' => $unsafeUrl, 'location' => $response['location']));
        $checks[] = e2e_case(strpos($response['body'], $unsafeUrl) === false, 'redir body does not echo unsafe URL', array('url' => $unsafeUrl));
    }
    $unsafeImages = array(
        $gameBase . '/img/preload.gif',
        'file:///etc/passwd',
        'javascript:alert(1)',
        'http://127.0.0.1:8888/game/img/preload.gif',
        'http://[::1]/game/img/preload.gif',
        'http://[::ffff:127.0.0.1]/game/img/preload.gif',
        'http://169.254.169.254/latest/meta-data/iam/security-credentials.png',
        'http://example.com/image.svg',
    );
    foreach ($unsafeImages as $unsafeUrl) {
        $response = e2e_http_request('GET', $gameBase . '/pic.php?url=' . rawurlencode($unsafeUrl));
        $checks = array_merge($checks, e2e_response_check($response, 'pic unsafe target'));
        $checks[] = e2e_case($response['location'] === '', 'pic does not redirect to unsafe URL', array('url' => $unsafeUrl, 'location' => $response['location']));
        $checks[] = e2e_case(stripos($response['content_type'], 'image/') !== 0, 'pic does not return image content for rejected URL', array('url' => $unsafeUrl, 'content_type' => $response['content_type']));
    }
    $cases[] = e2e_finalize_case(array(
        'case' => 'redirect_and_image_proxy_reject_unsafe_direct_urls',
        'checks' => $checks,
    ));

    $now = time();
    $attackerFeed = substr(md5($token . '-attacker-feed'), 0, 32);
    $defenderFeed = substr(md5($token . '-defender-feed'), 0, 32);
    $feedLastSeen = $now + 60;
    dbquery("UPDATE {$db_prefix}uni SET feedage=5");
    e2e_set_feed_user($attackerId, $attackerFeed, ((int)$attackerSnapshot['flags'] | USER_FLAG_FEED_ENABLE) & ~USER_FLAG_FEED_ATOM, $feedLastSeen);
    e2e_set_feed_user($defenderId, $defenderFeed, ((int)$defenderSnapshot['flags'] | USER_FLAG_FEED_ENABLE) & ~USER_FLAG_FEED_ATOM, $feedLastSeen);
    $attackerSecret = $token . '-attacker-feed-secret';
    $defenderSecret = $token . '-defender-feed-secret';
    $attackerMsg = SendMessage($attackerId, $token, $attackerSecret . ' <script>alert("subject")</script>', $attackerSecret . ' <img src=x onerror=alert("body")> </textarea><script>' . $token . '</script>', MTYP_MISC, $now);
    $defenderMsg = SendMessage($defenderId, $token, $defenderSecret, $defenderSecret . ' <script>alert("foreign")</script>', MTYP_MISC, $now);

    $rss = e2e_http_request('GET', $gameBase . '/feed/show.php?feedid=' . rawurlencode($attackerFeed));
    $ownItem = e2e_http_request('GET', $gameBase . '/feed/viewitem.php?feedid=' . rawurlencode($attackerFeed) . '&mid=' . $attackerMsg . '&type=i');
    $foreignItem = e2e_http_request('GET', $gameBase . '/feed/viewitem.php?feedid=' . rawurlencode($attackerFeed) . '&mid=' . $defenderMsg . '&type=i');
    $badFeed = e2e_http_request('GET', $gameBase . '/feed/show.php?feedid=' . rawurlencode($attackerFeed . 'x<script>'));
    $badItem = e2e_http_request('GET', $gameBase . '/feed/viewitem.php?feedid=' . rawurlencode($attackerFeed) . '&mid=abc');

    e2e_set_feed_user($attackerId, $attackerFeed, ((int)$attackerSnapshot['flags'] | USER_FLAG_FEED_ENABLE | USER_FLAG_FEED_ATOM), $feedLastSeen);
    $atom = e2e_http_request('GET', $gameBase . '/feed/show.php?feedid=' . rawurlencode($attackerFeed));

    e2e_set_feed_user($attackerId, $attackerFeed, ((int)$attackerSnapshot['flags'] & ~USER_FLAG_FEED_ENABLE) & ~USER_FLAG_FEED_ATOM, $feedLastSeen);
    $disabledFeed = e2e_http_request('GET', $gameBase . '/feed/show.php?feedid=' . rawurlencode($attackerFeed));

    e2e_set_feed_user($attackerId, $attackerFeed, ((int)$attackerSnapshot['flags'] | USER_FLAG_FEED_ENABLE) & ~USER_FLAG_FEED_ATOM, $feedLastSeen);
    dbquery("UPDATE {$db_prefix}uni SET feedage=-1");
    $prohibitedFeed = e2e_http_request('GET', $gameBase . '/feed/show.php?feedid=' . rawurlencode($attackerFeed));
    dbquery("UPDATE {$db_prefix}uni SET feedage=5");

    $combinedFeedBodies = $rss['body'] . $atom['body'] . $ownItem['body'];
    $cases[] = e2e_finalize_case(array(
        'case' => 'feed_endpoints_escape_output_and_enforce_owner_token_boundaries',
        'checks' => array_merge(
            e2e_response_check($rss, 'RSS feed render'),
            e2e_response_check($atom, 'Atom feed render'),
            e2e_response_check($ownItem, 'own feed item render'),
            e2e_response_check($foreignItem, 'foreign feed item request'),
            e2e_response_check($badFeed, 'malformed feed id request'),
            e2e_response_check($badItem, 'malformed feed item id request'),
            e2e_response_check($disabledFeed, 'disabled feed request'),
            e2e_response_check($prohibitedFeed, 'universe-prohibited feed request'),
            array(
                e2e_case(strpos($rss['body'], $attackerSecret) !== false, 'RSS feed contains the owner message marker'),
                e2e_case(strpos($atom['body'], $attackerSecret) !== false, 'Atom feed contains the owner message marker'),
                e2e_case(strpos($ownItem['body'], $attackerSecret) !== false, 'feed viewitem contains the owner message marker'),
                e2e_case(e2e_xml_ok($rss['body']), 'RSS feed is well-formed XML'),
                e2e_case(e2e_xml_ok($atom['body']), 'Atom feed is well-formed XML'),
                e2e_case(strpos($foreignItem['body'], $defenderSecret) === false, 'feed viewitem does not reveal a foreign user message'),
                e2e_case(strpos($rss['body'] . $atom['body'], $defenderSecret) === false, 'feed listing does not reveal foreign user messages'),
                e2e_case(stripos($combinedFeedBodies, '<script') === false && stripos($combinedFeedBodies, '<img') === false && stripos($combinedFeedBodies, '</textarea>') === false, 'feed output does not render raw executable HTML payloads'),
                e2e_case(strpos($disabledFeed['body'], $attackerSecret) === false, 'disabled feed does not reveal owner messages'),
                e2e_case(strpos($prohibitedFeed['body'], $attackerSecret) === false, 'universe-prohibited feed does not reveal owner messages'),
            )
        ),
    ));

    $attackerAuth = e2e_prepare_session($attackerId, 'direct-entry-method-attacker');
    $cookies = $attackerAuth['cookies'];
    $messageSubject = $token . '-method-message';
    $messageId = SendMessage($attackerId, $token, $messageSubject, $token . '-method-body', MTYP_MISC, time());
    $messageBefore = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}messages WHERE owner_id={$attackerId} AND msg_id={$messageId}");
    $messageGet = e2e_http_request('GET', $gameBase . '/index.php?page=messages&session=' . rawurlencode($attackerAuth['session']) . '&messages=1&deletemessages=deleteall&delmes' . $messageId . '=on', array(), $cookies);
    $messageAfter = e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}messages WHERE owner_id={$attackerId} AND msg_id={$messageId}");

    dbquery("UPDATE {$db_prefix}users SET disable=0, disable_until=0 WHERE player_id={$attackerId}");
    $optionsGet = e2e_http_request('GET', $gameBase . '/index.php?page=options&session=' . rawurlencode($attackerAuth['session']) . '&mode=change&db_deaktjava=on', array(), $cookies);
    $disableRow = e2e_one_row("SELECT disable, disable_until FROM {$db_prefix}users WHERE player_id={$attackerId} LIMIT 1");

    e2e_restore_user($attackerSnapshot);
    e2e_prepare_session($attackerId, 'direct-entry-method-ally');
    $allyAuth = e2e_prepare_session($attackerId, 'direct-entry-method-ally-session');
    $allyCookies = $allyAuth['cookies'];
    $createdAlly = CreateAlly($attackerId, 'D' . substr($token, -5), 'Direct Entry ' . substr($token, -6));
    $allyBefore = e2e_one_row("SELECT homepage, imglogo, open FROM {$db_prefix}ally WHERE ally_id={$createdAlly} LIMIT 1");
    $allyGet = e2e_http_request('GET', $gameBase . '/index.php?page=allianzen&session=' . rawurlencode($allyAuth['session']) . '&a=11&d=2&hp=' . rawurlencode('https://example.com/' . $token) . '&logo=' . rawurlencode('https://example.com/' . $token . '.png') . '&bew=0&fname=' . rawurlencode($token), array(), $allyCookies);
    $allyAfter = e2e_one_row("SELECT homepage, imglogo, open FROM {$db_prefix}ally WHERE ally_id={$createdAlly} LIMIT 1");

    $cases[] = e2e_finalize_case(array(
        'case' => 'get_requests_do_not_trigger_post_only_mutations',
        'checks' => array_merge(
            e2e_response_check($messageGet, 'GET message delete request'),
            e2e_response_check($optionsGet, 'GET account deletion request'),
            e2e_response_check($allyGet, 'GET alliance settings request'),
            array(
                e2e_case($messageBefore === 1 && $messageAfter === 1, 'GET messages request does not delete messages', array('before' => $messageBefore, 'after' => $messageAfter)),
                e2e_case($disableRow !== null && (int)$disableRow['disable'] === 0 && (int)$disableRow['disable_until'] === 0, 'GET options request does not schedule account deletion', $disableRow ?? array()),
                e2e_case($allyBefore !== null && $allyAfter !== null && $allyBefore['homepage'] === $allyAfter['homepage'] && $allyBefore['imglogo'] === $allyAfter['imglogo'] && (int)$allyBefore['open'] === (int)$allyAfter['open'], 'GET alliance settings request does not update persisted settings', array('before' => $allyBefore, 'after' => $allyAfter)),
            )
        ),
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'direct_entry_security_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
} finally {
    dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$attackerId},{$defenderId}) AND (subj LIKE '%" . e2e_sql_escape($token) . "%' OR text LIKE '%" . e2e_sql_escape($token) . "%' OR msgfrom LIKE '%" . e2e_sql_escape($token) . "%')");
    if ($createdAlly > 0) {
        DismissAlly($createdAlly);
    }
    if ($uniSnapshot !== null) {
        dbquery("UPDATE {$db_prefix}uni SET feedage=" . (int)$uniSnapshot['feedage']);
    }
    e2e_restore_user($attackerSnapshot);
    e2e_restore_user($defenderSnapshot);
}

echo json_encode(array(
    'case_group' => 'http_direct_entry_security',
    'base' => $base,
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
