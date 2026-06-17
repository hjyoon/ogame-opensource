<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_account_deletion_cleanup_e2e.php';
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

function e2e_sql_exec(string $sql): mixed
{
    $res = dbquery($sql);
    if ($res === false) {
        throw new RuntimeException('SQL failed: ' . $sql);
    }
    return $res;
}

function e2e_one_row(string $sql): ?array
{
    $res = e2e_sql_exec($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function e2e_count(string $sql): int
{
    $row = e2e_one_row($sql);
    return $row === null ? 0 : (int)$row['cnt'];
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

function e2e_user_row(int $userId): ?array
{
    global $db_prefix;
    return e2e_one_row("SELECT player_id, name, hplanetid FROM {$db_prefix}users WHERE player_id={$userId} LIMIT 1");
}

function e2e_create_temp_user(string $suffix): array
{
    global $db_prefix, $resmap;

    $name = 'e2edel_' . substr($suffix, 0, 12);
    $existing = e2e_one_row("SELECT player_id FROM {$db_prefix}users WHERE name='" . e2e_sql_escape($name) . "' LIMIT 1");
    if ($existing !== null) {
        RemoveUser((int)$existing['player_id'], time());
    }

    $id = CreateUser($name, 'E2E_test123', $name . '@example.local', false);
    $row = e2e_one_row("SELECT player_id, hplanetid FROM {$db_prefix}users WHERE player_id={$id} LIMIT 1");
    if ($row === null || (int)$row['hplanetid'] <= 0) {
        throw new RuntimeException("Failed to create cleanup fixture user {$name}");
    }

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=0";
    }
    e2e_sql_exec(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) .
        ", validated=1, validatemd='', deact_ip=1, lang='en', skin='/evolution/', useskin=1, " .
        "admin=0, vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, " .
        "disable=0, disable_until=0, dm=0, dmfree=0 WHERE player_id={$id}"
    );
    InvalidateUserCache();

    return array('player_id' => $id, 'planet_id' => (int)$row['hplanetid'], 'name' => $name);
}

function e2e_seed_fleetlog(int $ownerId, int $targetId, array $origin, array $target, int $now): int
{
    global $fleetmap, $transportableResources;

    $row = array(
        'owner_id' => $ownerId,
        'target_id' => $targetId,
        'union_id' => 0,
        'fuel' => 0,
        'mission' => FTYP_TRANSPORT,
        'flight_time' => 60,
        'deploy_time' => 0,
        'start' => $now,
        'end' => $now + 60,
        'origin_g' => (int)$origin['g'],
        'origin_s' => (int)$origin['s'],
        'origin_p' => (int)$origin['p'],
        'origin_type' => (int)$origin['type'],
        'target_g' => (int)$target['g'],
        'target_s' => (int)$target['s'],
        'target_p' => (int)$target['p'],
        'target_type' => (int)$target['type'],
    );
    foreach ($transportableResources as $rc) {
        $row['p' . $rc] = 0;
        $row[$rc] = 0;
    }
    foreach ($fleetmap as $gid) {
        $row[$gid] = 0;
    }

    return AddDBRow($row, 'fleetlogs');
}

function e2e_seed_auxiliary_rows(int $deletedUserId, int $deletedPlanetId, int $survivorUserId, int $survivorPlanetId, int $allyId, string $token): array
{
    global $db_prefix;

    $now = time();
    $deletedPlanet = LoadPlanetById($deletedPlanetId);
    $survivorPlanet = LoadPlanetById($survivorPlanetId);
    if ($deletedPlanet === null || $survivorPlanet === null) {
        throw new RuntimeException('Failed to load cleanup fixture planets.');
    }

    $messageId = SendMessage($deletedUserId, 'E2E', $token . '-message', $token . '-body', MTYP_MISC, $now, $deletedPlanetId);
    AddDBRow(array('owner_id' => $deletedUserId, 'subj' => $token . '-note', 'text' => $token, 'textsize' => strlen($token), 'prio' => 1, 'date' => $now), 'notes');
    AddDBRow(array('owner_id' => $survivorUserId, 'msg_id' => $messageId, 'msgfrom' => 'E2E', 'subj' => $token . '-report', 'text' => $token, 'date' => $now), 'reports');
    AddDBRow(array('owner_id' => $deletedUserId, 'url' => '/e2e-cleanup', 'method' => 'GET', 'getdata' => $token, 'postdata' => '', 'date' => $now), 'browse');
    AddDBRow(array('owner_id' => $deletedUserId, 'name' => $token . '-template', 'date' => $now), 'template');
    SetVar($deletedUserId, 'E2EDeleteCleanup', $token);
    UserLog($deletedUserId, 'E2E_DELETE_CLEANUP', $token, $now);
    LogIPAddress('127.0.0.2', $deletedUserId, 0);
    AddApplication($allyId, $deletedUserId, $token);
    AddDBRow(array('request_from' => $deletedUserId, 'request_to' => $survivorUserId, 'text' => $token, 'accepted' => 0), 'buddy');
    AddDBRow(array('fleet_id' => 0, 'target_player' => $deletedUserId, 'name' => substr($token, 0, 20), 'players' => (string)$deletedUserId), 'union');
    e2e_seed_fleetlog($deletedUserId, $survivorUserId, $deletedPlanet, $survivorPlanet, $now);
    e2e_seed_fleetlog($survivorUserId, $deletedUserId, $survivorPlanet, $deletedPlanet, $now);

    return e2e_auxiliary_counts($deletedUserId, $messageId);
}

function e2e_auxiliary_counts(int $deletedUserId, int $messageId = 0): array
{
    global $db_prefix;

    $messageClause = $messageId > 0 ? " OR msg_id={$messageId}" : '';
    return array(
        'users' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}users WHERE player_id={$deletedUserId}"),
        'planets' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}planets WHERE owner_id={$deletedUserId} AND type<>" . PTYP_DF),
        'messages' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}messages WHERE owner_id={$deletedUserId}"),
        'notes' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}notes WHERE owner_id={$deletedUserId}"),
        'reports' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}reports WHERE owner_id={$deletedUserId}{$messageClause}"),
        'browse' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}browse WHERE owner_id={$deletedUserId}"),
        'template' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}template WHERE owner_id={$deletedUserId}"),
        'botvars' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}botvars WHERE owner_id={$deletedUserId}"),
        'userlogs' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}userlogs WHERE owner_id={$deletedUserId}"),
        'fleetlogs' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleetlogs WHERE owner_id={$deletedUserId} OR target_id={$deletedUserId}"),
        'iplogs' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}iplogs WHERE user_id={$deletedUserId}"),
        'allyapps' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}allyapps WHERE player_id={$deletedUserId}"),
        'buddy' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}buddy WHERE request_from={$deletedUserId} OR request_to={$deletedUserId}"),
        'union' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}union WHERE target_player={$deletedUserId} OR players REGEXP '(^|,){$deletedUserId}(,|$)'"),
        'queue' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id={$deletedUserId}"),
        'buildqueue' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}buildqueue WHERE owner_id={$deletedUserId}"),
        'fleet' => e2e_count("SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id={$deletedUserId}"),
    );
}

function e2e_all_zero(array $counts): bool
{
    foreach ($counts as $count) {
        if ((int)$count !== 0) {
            return false;
        }
    }
    return true;
}

function e2e_force_cleanup_user(int $userId): void
{
    global $db_prefix;

    if ($userId <= 0) {
        return;
    }
    if (e2e_user_row($userId) !== null) {
        RemoveUser($userId, time());
    }
    e2e_sql_exec("DELETE FROM {$db_prefix}messages WHERE owner_id={$userId}");
    e2e_sql_exec("DELETE FROM {$db_prefix}notes WHERE owner_id={$userId}");
    e2e_sql_exec("DELETE FROM {$db_prefix}reports WHERE owner_id={$userId}");
    e2e_sql_exec("DELETE FROM {$db_prefix}browse WHERE owner_id={$userId}");
    e2e_sql_exec("DELETE FROM {$db_prefix}template WHERE owner_id={$userId}");
    e2e_sql_exec("DELETE FROM {$db_prefix}botvars WHERE owner_id={$userId}");
    e2e_sql_exec("DELETE FROM {$db_prefix}userlogs WHERE owner_id={$userId}");
    e2e_sql_exec("DELETE FROM {$db_prefix}fleetlogs WHERE owner_id={$userId} OR target_id={$userId}");
    e2e_sql_exec("DELETE FROM {$db_prefix}iplogs WHERE user_id={$userId}");
    e2e_sql_exec("DELETE FROM {$db_prefix}allyapps WHERE player_id={$userId}");
    e2e_sql_exec("DELETE FROM {$db_prefix}buddy WHERE request_from={$userId} OR request_to={$userId}");
    e2e_sql_exec("DELETE FROM {$db_prefix}union WHERE target_player={$userId} OR players REGEXP '(^|,){$userId}(,|$)'");
    e2e_sql_exec("DELETE FROM {$db_prefix}queue WHERE owner_id={$userId}");
    e2e_sql_exec("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$userId}");
    e2e_sql_exec("DELETE FROM {$db_prefix}fleet WHERE owner_id={$userId}");
}

$cases = array();
$createdUsers = array();
$createdAlliances = array();

try {
    $runToken = 'del' . substr(md5((string)microtime(true)), 0, 8);

    $survivor = e2e_create_temp_user($runToken . 's1');
    $deleted = e2e_create_temp_user($runToken . 'd1');
    $createdUsers[] = (int)$survivor['player_id'];
    $createdUsers[] = (int)$deleted['player_id'];
    $allyId = CreateAlly((int)$survivor['player_id'], 'ED' . substr($runToken, -4), 'E2E Delete ' . $runToken);
    $createdAlliances[] = $allyId;

    $beforeDirect = e2e_seed_auxiliary_rows((int)$deleted['player_id'], (int)$deleted['planet_id'], (int)$survivor['player_id'], (int)$survivor['planet_id'], $allyId, $runToken . '-direct');
    RemoveUser((int)$deleted['player_id'], time());
    $afterDirect = e2e_auxiliary_counts((int)$deleted['player_id']);
    $survivorAfterDirect = e2e_user_row((int)$survivor['player_id']);
    $cases[] = e2e_finalize_case(array(
        'case' => 'remove_user_cleans_auxiliary_player_rows',
        'checks' => array(
            e2e_case($beforeDirect['messages'] >= 2 && $beforeDirect['notes'] >= 1 && $beforeDirect['reports'] >= 1 && $beforeDirect['botvars'] >= 2, 'direct-delete fixture has representative auxiliary rows before deletion', $beforeDirect),
            e2e_case(e2e_all_zero($afterDirect), 'RemoveUser deletes user, planet, queue, fleet, social, log, and auxiliary rows', $afterDirect),
            e2e_case($survivorAfterDirect !== null, 'RemoveUser preserves unrelated survivor user', $survivorAfterDirect ?? array()),
        ),
    ));
    $createdUsers = array_values(array_filter($createdUsers, fn($id) => $id !== (int)$deleted['player_id']));

    $queueDeleted = e2e_create_temp_user($runToken . 'd2');
    $createdUsers[] = (int)$queueDeleted['player_id'];
    $beforeQueue = e2e_seed_auxiliary_rows((int)$queueDeleted['player_id'], (int)$queueDeleted['planet_id'], (int)$survivor['player_id'], (int)$survivor['planet_id'], $allyId, $runToken . '-queue');
    $now = time();
    e2e_sql_exec("UPDATE {$db_prefix}users SET disable=1, disable_until=" . ($now - 10) . ", admin=0, dm=0, lastclick={$now} WHERE player_id=" . (int)$queueDeleted['player_id']);
    $cleanTaskId = AddQueue(USER_SPACE, QTYP_CLEAN_PLAYERS, 0, 0, 0, $now, 0, QUEUE_PRIO_CLEAN_PLAYERS);
    $cleanQueue = LoadQueue($cleanTaskId);
    Queue_CleanPlayers_End($cleanQueue);
    $afterQueue = e2e_auxiliary_counts((int)$queueDeleted['player_id']);
    $futureCleanPlayers = e2e_one_row("SELECT task_id, end FROM {$db_prefix}queue WHERE type='" . QTYP_CLEAN_PLAYERS . "' ORDER BY end DESC LIMIT 1");
    $cases[] = e2e_finalize_case(array(
        'case' => 'clean_players_queue_reuses_remove_user_cleanup',
        'checks' => array(
            e2e_case($beforeQueue['messages'] >= 2 && $beforeQueue['notes'] >= 1 && $beforeQueue['reports'] >= 1 && $beforeQueue['botvars'] >= 2, 'clean-players fixture has representative auxiliary rows before deletion', $beforeQueue),
            e2e_case(e2e_all_zero($afterQueue), 'CleanPlayers deletion removes the same auxiliary rows as direct RemoveUser', $afterQueue),
            e2e_case($futureCleanPlayers !== null && (int)$futureCleanPlayers['task_id'] !== $cleanTaskId, 'CleanPlayers schedules its next future cleanup task', $futureCleanPlayers ?? array()),
        ),
    ));
    $createdUsers = array_values(array_filter($createdUsers, fn($id) => $id !== (int)$queueDeleted['player_id']));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'account_deletion_cleanup_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array(
            'exception' => get_class($e),
            'file' => $e->getFile(),
            'line' => $e->getLine(),
        ))),
        'pass' => false,
    );
} finally {
    foreach ($createdAlliances as $allyId) {
        if ($allyId > 0 && e2e_one_row("SELECT ally_id FROM {$db_prefix}ally WHERE ally_id={$allyId} LIMIT 1") !== null) {
            DismissAlly($allyId);
        }
    }
    foreach ($createdUsers as $userId) {
        e2e_force_cleanup_user((int)$userId);
    }
}

$pass = array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true);
echo json_encode(array(
    'case_group' => 'http_account_deletion_cleanup',
    'all_pass' => $pass,
    'cases' => $cases,
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
