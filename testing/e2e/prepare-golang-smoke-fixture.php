<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-go-smoke-fixture';
$_SERVER['REQUEST_URI'] = '/testing/e2e/prepare-golang-smoke-fixture.php';
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

function smoke_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

function smoke_one_row(string $sql): ?array
{
    $res = dbquery($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function smoke_user_by_name(string $name): ?array
{
    global $db_prefix;
    $lower = mb_strtolower($name, 'UTF-8');
    return smoke_one_row("SELECT * FROM {$db_prefix}users WHERE name='" . smoke_sql_escape($lower) . "' LIMIT 1");
}

function smoke_home_planet_exists(int $playerId, int $planetId): bool
{
    global $db_prefix;
    if ($planetId <= 0) {
        return false;
    }
    $row = smoke_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE planet_id={$planetId} AND owner_id={$playerId} LIMIT 1");
    return $row !== null;
}

function smoke_prepare_user(string $name, string $password, string $email, int $adminLevel): array
{
    global $db_prefix, $db_secret;

    $displayName = $name === mb_strtolower($name, 'UTF-8') ? ucfirst($name) : $name;
    $user = smoke_user_by_name($name);
    if ($user === null) {
        $playerId = CreateUser($displayName, $password, $email, false);
        $user = smoke_one_row("SELECT * FROM {$db_prefix}users WHERE player_id={$playerId} LIMIT 1");
        if ($user === null) {
            throw new RuntimeException('Failed to create Go smoke user ' . $name . '.');
        }
    }

    $playerId = (int)$user['player_id'];
    $homePlanetId = (int)$user['hplanetid'];
    if (!smoke_home_planet_exists($playerId, $homePlanetId)) {
        $homePlanetId = CreateHomePlanet($playerId);
        if ($homePlanetId <= 0) {
            throw new RuntimeException('Failed to create Go smoke home planet for ' . $name . '.');
        }
    }

    $passwordHash = md5($password . $db_secret);
    dbquery(
        "UPDATE {$db_prefix}users SET " .
        "name='" . smoke_sql_escape(mb_strtolower($name, 'UTF-8')) . "', " .
        "oname='" . smoke_sql_escape($displayName) . "', " .
        "password='" . smoke_sql_escape($passwordHash) . "', " .
        "pemail='" . smoke_sql_escape($email) . "', email='" . smoke_sql_escape($email) . "', " .
        "validated=1, validatemd='', deact_ip=1, admin={$adminLevel}, " .
        "vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, " .
        "lang='en', skin='/evolution/', useskin=1, hplanetid={$homePlanetId}, aktplanet={$homePlanetId}, " .
        "lastclick=" . time() . " WHERE player_id={$playerId}"
    );
    InvalidateUserCache();

    return array('player_id' => $playerId, 'name' => $displayName, 'home_planet_id' => $homePlanetId);
}

function smoke_cleanup_fleets(array $userIds, array $planetIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', array_filter($userIds, fn($id) => $id > 0)));
    $planetList = implode(',', array_map('intval', array_filter($planetIds, fn($id) => $id > 0)));
    if ($userList === '' || $planetList === '') {
        return;
    }

    $fleetIds = array();
    $res = dbquery("SELECT fleet_id FROM {$db_prefix}fleet WHERE owner_id IN ({$userList}) OR start_planet IN ({$planetList}) OR target_planet IN ({$planetList})");
    while ($row = dbarray($res)) {
        $fleetIds[] = (int)$row['fleet_id'];
    }
    if (!empty($fleetIds)) {
        $fleetList = implode(',', $fleetIds);
        dbquery("DELETE FROM {$db_prefix}queue WHERE type='" . QTYP_FLEET . "' AND (owner_id IN ({$userList}) OR sub_id IN ({$fleetList}))");
        dbquery("DELETE FROM {$db_prefix}fleet WHERE fleet_id IN ({$fleetList})");
    }
	dbquery("DELETE FROM {$db_prefix}queue WHERE type='" . QTYP_FLEET . "' AND owner_id IN ({$userList})");
}

function smoke_prepare_admin_queue_task(int $ownerId): int
{
	global $db_prefix;

	dbquery("DELETE FROM {$db_prefix}queue WHERE type='" . QTYP_DEBUG . "' AND owner_id={$ownerId}");
	return AddQueue($ownerId, QTYP_DEBUG, 0, 0, 0, time(), 3600, QUEUE_PRIO_DEBUG);
}

function smoke_fleet_queue_task_id(int $fleetId): int
{
	global $db_prefix;
	$row = smoke_one_row("SELECT task_id FROM {$db_prefix}queue WHERE type='" . QTYP_FLEET . "' AND sub_id={$fleetId} LIMIT 1");
	return $row === null ? 0 : (int)$row['task_id'];
}

function smoke_age_fleet_queue(int $fleetId, int $elapsedSeconds): void
{
    global $db_prefix;
    $start = time() - $elapsedSeconds;
    dbquery("UPDATE {$db_prefix}queue SET start={$start} WHERE type='" . QTYP_FLEET . "' AND sub_id={$fleetId}");
}

function smoke_cleanup_alliances(array $userIds): void
{
    global $db_prefix;

    $userList = implode(',', array_map('intval', array_filter($userIds, fn($id) => $id > 0)));
    if ($userList === '') {
        return;
    }

    $allyIds = array();
    $res = dbquery("SELECT DISTINCT ally_id FROM {$db_prefix}users WHERE player_id IN ({$userList}) AND ally_id > 0");
    while ($row = dbarray($res)) {
        $allyIds[] = (int)$row['ally_id'];
    }
    $res = dbquery("SELECT ally_id FROM {$db_prefix}ally WHERE tag LIKE 'GOSM%' OR name LIKE 'Go smoke alliance%'");
    while ($row = dbarray($res)) {
        $allyIds[] = (int)$row['ally_id'];
    }
    foreach (array_unique($allyIds) as $allyId) {
        DismissAlly($allyId);
    }

    dbquery("DELETE FROM {$db_prefix}allyapps WHERE player_id IN ({$userList})");
    dbquery("UPDATE {$db_prefix}users SET ally_id=0, allyrank=0, joindate=0 WHERE player_id IN ({$userList})");
}

function smoke_find_empty_position(array $near): array
{
    $positions = smoke_find_empty_positions($near, 1);
    return $positions[0];
}

function smoke_find_empty_positions(array $near, int $count): array
{
    $g = (int)$near['g'];
    $system = (int)$near['s'];
    $positions = array();
    for ($p = 1; $p <= 15; $p++) {
        if ($p === (int)$near['p']) {
            continue;
        }
        if (!HasPlanet($g, $system, $p)) {
            $positions[] = array($g, $system, $p);
            if (count($positions) >= $count) {
                return $positions;
            }
        }
    }
    for ($scanG = 1; $scanG <= (int)$GLOBALS['GlobalUni']['galaxies']; $scanG++) {
        for ($scanS = 1; $scanS <= (int)$GLOBALS['GlobalUni']['systems']; $scanS++) {
            if ($scanG === $g && $scanS === $system) {
                continue;
            }
            for ($p = 1; $p <= 15; $p++) {
                if (!HasPlanet($scanG, $scanS, $p)) {
                    $positions[] = array($scanG, $scanS, $p);
                    if (count($positions) >= $count) {
                        return $positions;
                    }
                }
            }
        }
    }
    throw new RuntimeException('Not enough empty planet positions found for Go smoke fixtures.');
}

function smoke_prepare_planet(int $planetId, int $ownerId, string $name, array $coords): void
{
    global $db_prefix, $buildmap, $fleetmap;

    [$g, $s, $p] = $coords;
    $buildings = array();
    foreach ($buildmap as $gid) {
        $buildings[] = "`{$gid}`=0";
    }
    $ships = array();
    foreach ($fleetmap as $gid) {
        $ships[] = "`{$gid}`=" . ($gid === GID_F_SC ? 3 : 0);
    }
    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "name='" . smoke_sql_escape($name) . "', g={$g}, s={$s}, p={$p}, type=" . PTYP_PLANET . ", owner_id={$ownerId}, " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000, " .
        implode(',', $buildings) . ", " . implode(',', $ships) . ", " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=0, maxfields=200, lastpeek=" . time() . " " .
        "WHERE planet_id={$planetId}"
    );
}

function smoke_prepare_moon(int $homePlanetId, int $ownerId): int
{
    global $db_prefix, $buildmap, $fleetmap;

    $home = LoadPlanetById($homePlanetId);
    if ($home === null) {
        throw new RuntimeException('Cannot prepare phalanx moon without a home planet.');
    }
    $moonId = PlanetHasMoon($homePlanetId);
    if ($moonId <= 0) {
        $moonId = CreatePlanet((int)$home['g'], (int)$home['s'], (int)$home['p'], $ownerId, 1, 1, 20, time());
    }
    if ($moonId <= 0) {
        throw new RuntimeException('Failed to prepare phalanx fixture moon.');
    }

    $buildings = array();
    foreach ($buildmap as $gid) {
        $level = 0;
        if ($gid === GID_B_LUNAR_BASE) {
            $level = 1;
        } elseif ($gid === GID_B_PHALANX) {
            $level = 3;
        }
        $buildings[] = "`{$gid}`={$level}";
    }
    $ships = array();
    foreach ($fleetmap as $gid) {
        $ships[] = "`{$gid}`=0";
    }
    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "name='Go Smoke Moon', g=" . (int)$home['g'] . ", s=" . (int)$home['s'] . ", p=" . (int)$home['p'] . ", type=" . PTYP_MOON . ", owner_id={$ownerId}, " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=20000, " .
        implode(',', $buildings) . ", " . implode(',', $ships) . ", " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=2, maxfields=4, lastpeek=" . time() . " " .
        "WHERE planet_id={$moonId}"
    );
    return $moonId;
}

function smoke_prepare_phalanx_edge_fixture(string $password, array $near, int $targetPlanetId, int $ownTargetPlanetId): array
{
    global $db_prefix, $buildmap, $fleetmap;

    $user = smoke_prepare_user('gophalanxlow', $password, 'gophalanxlow@example.local', USER_TYPE_PLAYER);
    $playerId = (int)$user['player_id'];
    $homePlanetId = (int)$user['home_planet_id'];
    smoke_cleanup_alliances(array($playerId));

    $existingMoon = smoke_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE owner_id={$playerId} AND type=" . PTYP_MOON . " AND name='GoPhalanxLowMoon' LIMIT 1");
    $moonId = $existingMoon === null ? 0 : (int)$existingMoon['planet_id'];
    smoke_cleanup_fleets(array($playerId), array($homePlanetId, $moonId));
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$playerId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$playerId} OR planet_id IN ({$homePlanetId}," . max(0, $moonId) . ")");

    $coords = smoke_find_empty_position($near);
    smoke_prepare_planet($homePlanetId, $playerId, 'GoPhalanxLow', $coords);
    $home = LoadPlanetById($homePlanetId);
    if ($home === null) {
        throw new RuntimeException('Go phalanx edge home planet is missing.');
    }

    if ($moonId <= 0) {
        $moonId = CreatePlanet((int)$home['g'], (int)$home['s'], (int)$home['p'], $playerId, 1, 1, 20, time());
    }
    if ($moonId <= 0) {
        throw new RuntimeException('Failed to prepare Go phalanx edge fixture moon.');
    }

    $buildings = array();
    foreach ($buildmap as $gid) {
        $level = 0;
        if ($gid === GID_B_LUNAR_BASE) {
            $level = 1;
        } elseif ($gid === GID_B_PHALANX) {
            $level = 2;
        }
        $buildings[] = "`{$gid}`={$level}";
    }
    $ships = array();
    foreach ($fleetmap as $gid) {
        $ships[] = "`{$gid}`=0";
    }
    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "name='GoPhalanxLowMoon', g=" . (int)$home['g'] . ", s=" . (int)$home['s'] . ", p=" . (int)$home['p'] . ", type=" . PTYP_MOON . ", owner_id={$playerId}, " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=4000, " .
        implode(',', $buildings) . ", " . implode(',', $ships) . ", " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=2, maxfields=4, lastpeek=" . time() . " " .
        "WHERE planet_id={$moonId}"
    );
    InvalidateUserCache();

    return array(
        'low_login' => mb_strtolower($user['name'], 'UTF-8'),
        'low_deut_moon_id' => $moonId,
        'low_deut_target_planet_id' => $targetPlanetId,
        'low_deuterium' => 4000,
        'own_target_planet_id' => $ownTargetPlanetId,
    );
}

function smoke_dispatch_phalanx_fleet(int $ownerId, int $originPlanetId, int $targetPlanetId): int
{
    global $fleetmap, $transportableResources;

    $fleet = array();
    foreach ($fleetmap as $gid) {
        $fleet[$gid] = $gid === GID_F_SC ? 1 : 0;
    }
    $resources = array();
    foreach ($transportableResources as $gid) {
        $resources[$gid] = 0;
    }
    $origin = LoadPlanetById($originPlanetId);
    $target = LoadPlanetById($targetPlanetId);
    if ($origin === null || $target === null) {
        throw new RuntimeException('Cannot dispatch phalanx fixture fleet.');
    }
    AdjustShips($fleet, $originPlanetId, '-');
    return DispatchFleet($fleet, $origin, $target, FTYP_TRANSPORT, 3600, $resources, 0, time());
}

function smoke_prepare_feed_fixture(array $login, array $operator, array $target): array
{
    global $db_prefix;

    $loginId = (int)$login['player_id'];
    $operatorId = (int)$operator['player_id'];
    $targetId = (int)$target['player_id'];
    $now = time();
    $lastfeed = $now + 300;
    $rssFeed = substr(hash('sha256', 'go-smoke-rss-' . $targetId), 0, 32);
    $atomFeed = substr(hash('sha256', 'go-smoke-atom-' . $operatorId), 0, 32);
    $ownerSecret = 'GO_FEED_OWNER_' . $now;
    $foreignSecret = 'GO_FEED_FOREIGN_' . $now;
    $atomSecret = 'GO_FEED_ATOM_' . $now;

    dbquery("UPDATE {$db_prefix}uni SET feedage=5");
    dbquery("DELETE FROM {$db_prefix}messages WHERE msgfrom='Go Smoke Feed' AND owner_id IN ({$loginId},{$operatorId},{$targetId})");
    dbquery(
        "UPDATE {$db_prefix}users SET flags=((flags | " . USER_FLAG_FEED_ENABLE . ") & ~" . USER_FLAG_FEED_ATOM . "), " .
        "feedid='" . smoke_sql_escape($rssFeed) . "', lastfeed={$lastfeed} WHERE player_id={$targetId}"
    );
    dbquery(
        "UPDATE {$db_prefix}users SET flags=(flags | " . USER_FLAG_FEED_ENABLE . " | " . USER_FLAG_FEED_ATOM . "), " .
        "feedid='" . smoke_sql_escape($atomFeed) . "', lastfeed={$lastfeed} WHERE player_id={$operatorId}"
    );

    $ownerMessageId = SendMessage(
        $targetId,
        'Go Smoke Feed',
        $ownerSecret . ' <script>alert("subject")</script>',
        $ownerSecret . ' <img src=x onerror=alert("body")> </textarea><script>unsafe</script>',
        MTYP_MISC,
        $now
    );
    $foreignMessageId = SendMessage(
        $loginId,
        'Go Smoke Feed',
        $foreignSecret,
        $foreignSecret . ' foreign body',
        MTYP_MISC,
        $now
    );
    $atomMessageId = SendMessage(
        $operatorId,
        'Go Smoke Feed',
        $atomSecret,
        $atomSecret . ' atom body',
        MTYP_MISC,
        $now
    );

    return array(
        'rss_feed_id' => $rssFeed,
        'atom_feed_id' => $atomFeed,
        'owner_message_id' => $ownerMessageId,
        'foreign_message_id' => $foreignMessageId,
        'atom_message_id' => $atomMessageId,
        'owner_secret' => $ownerSecret,
        'foreign_secret' => $foreignSecret,
        'atom_secret' => $atomSecret,
    );
}

function smoke_prepare_password_recovery_fixture(string $password): array
{
    global $db_prefix, $db_secret;

    $permanent = smoke_prepare_user('gorecovery', $password, 'gorecovery@example.local', 0);
    $temporary = smoke_prepare_user('gorecoverytemp', $password, 'gorecoverytemp@example.local', 0);
    $temporaryEmail = 'gorecoverytemp.pending@example.local';
    $passwordHash = md5($password . $db_secret);

    dbquery(
        "UPDATE {$db_prefix}users SET password='" . smoke_sql_escape($passwordHash) . "', session='', private_session='', " .
        "pemail='gorecovery@example.local', email='gorecovery@example.local', validated=1, validatemd='' " .
        "WHERE player_id=" . (int)$permanent['player_id']
    );
    dbquery(
        "UPDATE {$db_prefix}users SET password='" . smoke_sql_escape($passwordHash) . "', session='', private_session='', " .
        "pemail='gorecoverytemp@example.local', email='" . smoke_sql_escape($temporaryEmail) . "', validated=0, " .
        "validatemd='" . smoke_sql_escape(md5('go-recovery-temp')) . "' " .
        "WHERE player_id=" . (int)$temporary['player_id']
    );
    InvalidateUserCache();

    return array(
        'password' => $password,
        'permanent' => array(
            'player_id' => (int)$permanent['player_id'],
            'name' => $permanent['name'],
            'email' => 'gorecovery@example.local',
        ),
        'temporary' => array(
            'player_id' => (int)$temporary['player_id'],
            'name' => $temporary['name'],
            'email' => 'gorecoverytemp@example.local',
            'temporary_email' => $temporaryEmail,
        ),
    );
}

function smoke_prepare_admin_operations_fixture(array $operator, array $target): array
{
    global $db_prefix;

    $operatorId = (int)$operator['player_id'];
    $targetId = (int)$target['player_id'];
    $token = 'Go smoke admin ops ' . substr(hash('sha256', (string)microtime(true)), 0, 10);
    dbquery(
        "DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$operatorId},{$targetId}) AND " .
        "(msgfrom LIKE 'Go smoke admin ops%' OR subj LIKE 'Go smoke admin ops%' OR text LIKE 'Go smoke admin ops%')"
    );
    dbquery(
        "DELETE FROM {$db_prefix}reports WHERE " .
        "msgfrom LIKE 'Go smoke admin ops%' OR subj LIKE 'Go smoke admin ops%' OR text LIKE 'Go smoke admin ops%'"
    );
    $reportId = AddDBRow(array(
        'owner_id' => $targetId,
        'msg_id' => 0,
        'msgfrom' => $token . ' reporter',
        'subj' => $token . ' report subject',
        'text' => $token . ' report text',
        'date' => time(),
    ), 'reports');

    return array(
        'token' => $token,
        'report_id' => (int)$reportId,
        'operator_player_id' => $operatorId,
        'target_player_id' => $targetId,
    );
}

function smoke_prepare_admin_audit_fixture(array $target): array
{
    global $db_prefix;

    $targetId = (int)$target['player_id'];
    $token = 'Go smoke admin audit ' . substr(hash('sha256', (string)microtime(true)), 0, 10);
    dbquery("DELETE FROM {$db_prefix}userlogs WHERE owner_id={$targetId} AND (type='GO_SMOKE_AUDIT' OR text LIKE 'Go smoke admin audit%')");
    dbquery("DELETE FROM {$db_prefix}debug WHERE text LIKE 'Go smoke admin audit%' OR url LIKE '/go-smoke/audit/%'");
    dbquery("DELETE FROM {$db_prefix}errors WHERE text LIKE 'Go smoke admin audit%' OR url LIKE '/go-smoke/audit/%'");

    $now = time();
    UserLog($targetId, 'GO_SMOKE_AUDIT', $token . ' user log marker', $now);
    AddDBRow(array(
        'owner_id' => $targetId,
        'ip' => '127.0.0.1',
        'agent' => 'go-smoke',
        'url' => '/go-smoke/audit/' . $token,
        'text' => $token . ' debug marker',
        'date' => $now,
    ), 'debug');
    AddDBRow(array(
        'owner_id' => $targetId,
        'ip' => '127.0.0.1',
        'agent' => 'go-smoke',
        'url' => '/go-smoke/audit/' . $token,
        'text' => $token . ' error marker',
        'date' => $now,
    ), 'errors');

    return array(
        'token' => $token,
        'target_player_id' => $targetId,
    );
}

function smoke_set_fleet_restriction_user_state(array $user, int $score, array $options = array()): void
{
    global $db_prefix, $resmap;

    $playerId = (int)$user['player_id'];
    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=10";
    }
    $now = time();
    $admin = (int)($options['admin'] ?? USER_TYPE_PLAYER);
    $vacation = (int)($options['vacation'] ?? 0);
    $banned = (int)($options['banned'] ?? 0);
    $noattack = (int)($options['noattack'] ?? 0);
    $vacationUntil = $vacation ? $now + 3600 : 0;
    $bannedUntil = $banned ? $now + 3600 : 0;
    $noattackUntil = $noattack ? $now + 3600 : 0;
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", admin={$admin}, ally_id=0, allyrank=0, " .
        "validated=1, validatemd='', deact_ip=1, vacation={$vacation}, vacation_until={$vacationUntil}, " .
        "banned={$banned}, banned_until={$bannedUntil}, noattack={$noattack}, noattack_until={$noattackUntil}, " .
        "disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, score1={$score}, score2=0, score3=0, " .
        "place1=1, place2=1, place3=1, flags=" . USER_FLAG_DEFAULT . ", lastclick={$now} WHERE player_id={$playerId}"
    );
    InvalidateUserCache();
}

function smoke_prepare_fleet_restriction_fixture(string $password, array $near): array
{
    global $db_prefix;

    $attacker = smoke_prepare_user('gofleetattacker', $password, 'gofleetattacker@example.local', USER_TYPE_PLAYER);
    $weak = smoke_prepare_user('gofleetweak', $password, 'gofleetweak@example.local', USER_TYPE_PLAYER);
    $blocked = smoke_prepare_user('gofleetblocked', $password, 'gofleetblocked@example.local', USER_TYPE_PLAYER);
    $noob = smoke_prepare_user('gofleetnoob', $password, 'gofleetnoob@example.local', USER_TYPE_PLAYER);
    $strong = smoke_prepare_user('gofleetstrong', $password, 'gofleetstrong@example.local', USER_TYPE_PLAYER);
    $vacation = smoke_prepare_user('gofleetvacation', $password, 'gofleetvacation@example.local', USER_TYPE_PLAYER);
    $operator = smoke_prepare_user('gofleetoperator', $password, 'gofleetoperator@example.local', USER_TYPE_GO);
    $comparable = smoke_prepare_user('gofleetcomparable', $password, 'gofleetcomparable@example.local', USER_TYPE_PLAYER);
    $users = array($attacker, $weak, $blocked, $noob, $strong, $vacation, $operator, $comparable);
    $planetIds = array_map(fn($user) => (int)$user['home_planet_id'], $users);
    smoke_cleanup_fleets(array_map(fn($user) => (int)$user['player_id'], $users), $planetIds);
    $positions = smoke_find_empty_positions($near, count($users));

    $specs = array(
        array('key' => 'attacker', 'user' => $attacker, 'score' => 100000, 'options' => array()),
        array('key' => 'weak_attacker', 'user' => $weak, 'score' => 1000, 'options' => array()),
        array('key' => 'blocked_attacker', 'user' => $blocked, 'score' => 10000, 'options' => array('noattack' => 1)),
        array('key' => 'noob', 'user' => $noob, 'score' => 1000, 'options' => array()),
        array('key' => 'strong', 'user' => $strong, 'score' => 100000, 'options' => array()),
        array('key' => 'vacation', 'user' => $vacation, 'score' => 10000, 'options' => array('vacation' => 1)),
        array('key' => 'operator', 'user' => $operator, 'score' => 10000, 'options' => array('admin' => USER_TYPE_GO)),
        array('key' => 'comparable', 'user' => $comparable, 'score' => 10000, 'options' => array()),
    );

    $fixture = array();
    foreach ($specs as $index => $spec) {
        $user = $spec['user'];
        $planetId = (int)$user['home_planet_id'];
        $coords = $positions[$index];
        smoke_set_fleet_restriction_user_state($user, (int)$spec['score'], $spec['options']);
        smoke_prepare_planet($planetId, (int)$user['player_id'], 'GoFleet' . $index, $coords);
        dbquery(
            "UPDATE {$db_prefix}planets SET `" . GID_F_SC . "`=10, `" . GID_F_LF . "`=10, `" . GID_F_PROBE . "`=10, " .
            "`" . GID_RC_DEUTERIUM . "`=1000000 WHERE planet_id={$planetId}"
        );
        $fixture[$spec['key']] = array(
            'player_id' => (int)$user['player_id'],
            'login' => mb_strtolower($user['name'], 'UTF-8'),
            'home_planet_id' => $planetId,
            'coordinates' => array(
                'galaxy' => (int)$coords[0],
                'system' => (int)$coords[1],
                'position' => (int)$coords[2],
            ),
        );
    }
    return $fixture;
}

function smoke_prepare_premium_dm_fixture(string $password, array $near): array
{
    global $db_prefix;

    $oldGeologistUntil = time() + 3 * 24 * 60 * 60;
    $specs = array(
        'insufficient' => array('name' => 'gopremiumlow', 'email' => 'gopremiumlow@example.local', 'dm' => 9999, 'dmfree' => 0, 'geo_until' => 0),
        'mixed' => array('name' => 'gopremiummixed', 'email' => 'gopremiummixed@example.local', 'dm' => 4000, 'dmfree' => 7000, 'geo_until' => 0),
        'extend' => array('name' => 'gopremiumextend', 'email' => 'gopremiumextend@example.local', 'dm' => 20000, 'dmfree' => 0, 'geo_until' => $oldGeologistUntil),
        'invalid' => array('name' => 'gopremiuminvalid', 'email' => 'gopremiuminvalid@example.local', 'dm' => 50000, 'dmfree' => 500, 'geo_until' => 0),
    );

    $users = array();
    foreach ($specs as $key => $spec) {
        $users[$key] = smoke_prepare_user($spec['name'], $password, $spec['email'], USER_TYPE_PLAYER);
    }
    smoke_cleanup_alliances(array_map(fn($user) => (int)$user['player_id'], $users));
    smoke_cleanup_fleets(
        array_map(fn($user) => (int)$user['player_id'], $users),
        array_map(fn($user) => (int)$user['home_planet_id'], $users)
    );

    $positions = smoke_find_empty_positions($near, count($users));
    $fixture = array();
    $index = 0;
    foreach ($specs as $key => $spec) {
        $user = $users[$key];
        $playerId = (int)$user['player_id'];
        $planetId = (int)$user['home_planet_id'];
        smoke_prepare_planet($planetId, $playerId, 'GoPremium' . $index, $positions[$index]);
        dbquery(
            "UPDATE {$db_prefix}users SET admin=0, validated=1, deact_ip=1, vacation=0, vacation_until=0, " .
            "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, " .
            "dm=" . (int)$spec['dm'] . ", dmfree=" . (int)$spec['dmfree'] . ", " .
            "com_until=0, adm_until=0, eng_until=0, geo_until=" . (int)$spec['geo_until'] . ", tec_until=0, " .
            "hplanetid={$planetId}, aktplanet={$planetId}, lastclick=" . time() . " WHERE player_id={$playerId}"
        );
        $fixture[$key] = array(
            'player_id' => $playerId,
            'login' => mb_strtolower($user['name'], 'UTF-8'),
            'home_planet_id' => $planetId,
        );
        if ($key === 'extend') {
            $fixture[$key]['old_geologist_until'] = $oldGeologistUntil;
        }
        $index++;
    }
    InvalidateUserCache();
    return $fixture;
}

function smoke_prepare_vacation_freeze_fixture(string $password, array $near): array
{
    global $db_prefix, $fleetmap, $transportableResources, $resmap;

    $buildUser = smoke_prepare_user('govacbuild', $password, 'govacbuild@example.local', USER_TYPE_PLAYER);
    $fleetUser = smoke_prepare_user('govacfleet', $password, 'govacfleet@example.local', USER_TYPE_PLAYER);
    $mutationUser = smoke_prepare_user('govacmutate', $password, 'govacmutate@example.local', USER_TYPE_PLAYER);
    $defender = smoke_prepare_user('govacdefender', $password, 'govacdefender@example.local', USER_TYPE_PLAYER);
    $users = array($buildUser, $fleetUser, $mutationUser, $defender);
    $userIds = array_map(fn($user) => (int)$user['player_id'], $users);
    $planetIds = array_map(fn($user) => (int)$user['home_planet_id'], $users);
    smoke_cleanup_alliances($userIds);
    smoke_cleanup_fleets($userIds, $planetIds);
    foreach ($userIds as $userId) {
        dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$userId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "')");
    }
    foreach ($planetIds as $planetId) {
        dbquery("DELETE FROM {$db_prefix}buildqueue WHERE planet_id={$planetId}");
    }

    $positions = smoke_find_empty_positions($near, count($users));
    foreach ($users as $index => $user) {
        smoke_prepare_planet((int)$user['home_planet_id'], (int)$user['player_id'], 'GoVac' . $index, $positions[$index]);
    }

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=10";
    }
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", admin=0, validated=1, deact_ip=1, vacation=0, vacation_until=0, " .
        "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, lastclick=" . time() .
        " WHERE player_id IN (" . implode(',', $userIds) . ")"
    );
    dbquery(
        "UPDATE {$db_prefix}planets SET `" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=12, `" . GID_F_SC . "`=3, " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000 " .
        "WHERE planet_id IN (" . implode(',', $planetIds) . ")"
    );

    $buildError = BuildEnque(LoadUser((int)$buildUser['player_id']), (int)$buildUser['home_planet_id'], GID_B_METAL_MINE, 0, time());
    $buildQueue = smoke_one_row("SELECT task_id, sub_id FROM {$db_prefix}queue WHERE owner_id=" . (int)$buildUser['player_id'] . " AND type='" . QTYP_BUILD . "' ORDER BY task_id DESC LIMIT 1");
    if ($buildQueue !== null) {
        $futureBuildEnd = time() + 600;
        dbquery("UPDATE {$db_prefix}queue SET end={$futureBuildEnd} WHERE task_id=" . (int)$buildQueue['task_id']);
        dbquery("UPDATE {$db_prefix}buildqueue SET end={$futureBuildEnd} WHERE id=" . (int)$buildQueue['sub_id']);
    }

    $fleet = array();
    foreach ($fleetmap as $gid) {
        $fleet[$gid] = $gid === GID_F_SC ? 1 : 0;
    }
    $resources = array();
    foreach ($transportableResources as $gid) {
        $resources[$gid] = 0;
    }
    AdjustShips($fleet, (int)$fleetUser['home_planet_id'], '-');
    $fleetId = DispatchFleet(
        $fleet,
        LoadPlanetById((int)$fleetUser['home_planet_id']),
        LoadPlanetById((int)$defender['home_planet_id']),
        FTYP_TRANSPORT,
        600,
        $resources,
        0,
        time()
    );

    dbquery("UPDATE {$db_prefix}users SET vacation=1, vacation_until=" . (time() + 3600) . " WHERE player_id=" . (int)$mutationUser['player_id']);
    InvalidateUserCache();

    return array(
        'build' => array(
            'login' => mb_strtolower($buildUser['name'], 'UTF-8'),
            'player_id' => (int)$buildUser['player_id'],
            'home_planet_id' => (int)$buildUser['home_planet_id'],
            'build_error' => $buildError,
            'queue_task_id' => $buildQueue === null ? 0 : (int)$buildQueue['task_id'],
        ),
        'fleet' => array(
            'login' => mb_strtolower($fleetUser['name'], 'UTF-8'),
            'player_id' => (int)$fleetUser['player_id'],
            'home_planet_id' => (int)$fleetUser['home_planet_id'],
            'fleet_id' => $fleetId,
        ),
        'mutation' => array(
            'login' => mb_strtolower($mutationUser['name'], 'UTF-8'),
            'player_id' => (int)$mutationUser['player_id'],
            'home_planet_id' => (int)$mutationUser['home_planet_id'],
        ),
    );
}

function smoke_prepare_merchant_fixture(string $password, array $near): array
{
    global $db_prefix;

    $specs = array(
        'insufficient' => array('name' => 'gomerchantlow', 'email' => 'gomerchantlow@example.local', 'dm' => 0, 'dmfree' => 0, 'trader' => 0, 'metal' => 100000, 'crystal' => 100000, 'deuterium' => 100000),
        'call' => array('name' => 'gomerchantcall', 'email' => 'gomerchantcall@example.local', 'dm' => 1000, 'dmfree' => 2000, 'trader' => 0, 'metal' => 100000, 'crystal' => 100000, 'deuterium' => 100000),
        'trade' => array('name' => 'gomerchanttrade', 'email' => 'gomerchanttrade@example.local', 'dm' => 0, 'dmfree' => 0, 'trader' => 1, 'metal' => 1000000, 'crystal' => 100000, 'deuterium' => 100000),
        'reject' => array('name' => 'gomerchantreject', 'email' => 'gomerchantreject@example.local', 'dm' => 0, 'dmfree' => 0, 'trader' => 1, 'metal' => 1000, 'crystal' => 100000, 'deuterium' => 100000),
    );
    $users = array();
    foreach ($specs as $key => $spec) {
        $users[$key] = smoke_prepare_user($spec['name'], $password, $spec['email'], USER_TYPE_PLAYER);
    }
    $userIds = array_map(fn($user) => (int)$user['player_id'], $users);
    $planetIds = array_map(fn($user) => (int)$user['home_planet_id'], $users);
    smoke_cleanup_alliances($userIds);
    smoke_cleanup_fleets($userIds, $planetIds);
    $positions = smoke_find_empty_positions($near, count($users));

    $fixture = array();
    $index = 0;
    foreach ($specs as $key => $spec) {
        $user = $users[$key];
        $playerId = (int)$user['player_id'];
        $planetId = (int)$user['home_planet_id'];
        smoke_prepare_planet($planetId, $playerId, 'GoMerchant' . $index, $positions[$index]);
        dbquery(
            "UPDATE {$db_prefix}users SET admin=0, validated=1, deact_ip=1, vacation=0, vacation_until=0, " .
            "dm=" . (int)$spec['dm'] . ", dmfree=" . (int)$spec['dmfree'] . ", trader=" . (int)$spec['trader'] . ", " .
            "rate_m=3, rate_k=2, rate_d=1, lastclick=" . time() . " WHERE player_id={$playerId}"
        );
        dbquery(
            "UPDATE {$db_prefix}planets SET `" . GID_B_METAL_STOR . "`=10, `" . GID_B_CRYS_STOR . "`=10, `" . GID_B_DEUT_STOR . "`=10, " .
            "`" . GID_RC_METAL . "`=" . (int)$spec['metal'] . ", `" . GID_RC_CRYSTAL . "`=" . (int)$spec['crystal'] . ", `" . GID_RC_DEUTERIUM . "`=" . (int)$spec['deuterium'] . ", " .
            "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, lastpeek=" . time() . " WHERE planet_id={$planetId}"
        );
        $fixture[$key] = array(
            'login' => mb_strtolower($user['name'], 'UTF-8'),
            'player_id' => $playerId,
            'home_planet_id' => $planetId,
        );
        $index++;
    }
    InvalidateUserCache();
    return $fixture;
}

function smoke_prepare_moon_build_fixture(string $password, array $near): array
{
    global $db_prefix, $buildmap, $fleetmap;

    $user = smoke_prepare_user('gomoonbuilder', $password, 'gomoonbuilder@example.local', USER_TYPE_PLAYER);
    $playerId = (int)$user['player_id'];
    $homePlanetId = (int)$user['home_planet_id'];
    smoke_cleanup_alliances(array($playerId));
    smoke_cleanup_fleets(array($playerId), array($homePlanetId));
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$playerId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$playerId} OR planet_id={$homePlanetId}");

    $coords = smoke_find_empty_position($near);
    smoke_prepare_planet($homePlanetId, $playerId, 'GoMoonHome', $coords);
    $home = LoadPlanetById($homePlanetId);
    if ($home === null) {
        throw new RuntimeException('Go moon build home planet is missing.');
    }

    $moon = smoke_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE owner_id={$playerId} AND type=" . PTYP_MOON . " AND name='GoMoonBuild' LIMIT 1");
    $moonId = $moon === null ? 0 : (int)$moon['planet_id'];
    if ($moonId <= 0) {
        $moonId = CreatePlanet((int)$home['g'], (int)$home['s'], (int)$home['p'], $playerId, 1, 1, 20, time());
    }
    if ($moonId <= 0) {
        throw new RuntimeException('Failed to prepare Go moon build fixture moon.');
    }

    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$playerId} AND sub_id IN (SELECT id FROM {$db_prefix}buildqueue WHERE planet_id={$moonId})");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$playerId} OR planet_id={$moonId}");
    $buildings = array();
    foreach ($buildmap as $gid) {
        $buildings[] = "`{$gid}`=0";
    }
    $ships = array();
    foreach ($fleetmap as $gid) {
        $ships[] = "`{$gid}`=0";
    }
    dbquery(
        "UPDATE {$db_prefix}planets SET name='GoMoonBuild', g=" . (int)$home['g'] . ", s=" . (int)$home['s'] . ", p=" . (int)$home['p'] . ", type=" . PTYP_MOON . ", owner_id={$playerId}, " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000, " .
        implode(',', $buildings) . ", " . implode(',', $ships) . ", " .
        "prod1=0, prod2=0, prod3=0, prod4=0, prod12=0, prod212=0, fields=0, maxfields=1, lastpeek=" . time() . " WHERE planet_id={$moonId}"
    );

    $buildError = BuildEnque(LoadUser($playerId), $moonId, GID_B_LUNAR_BASE, 0, time() - 120);
    $queue = smoke_one_row("SELECT task_id, sub_id FROM {$db_prefix}queue WHERE owner_id={$playerId} AND type='" . QTYP_BUILD . "' AND obj_id=" . GID_B_LUNAR_BASE . " ORDER BY task_id DESC LIMIT 1");
    if ($queue !== null) {
        dbquery("UPDATE {$db_prefix}queue SET start=" . (time() - 120) . ", end=" . (time() - 30) . ", freeze=0, frozen=0 WHERE task_id=" . (int)$queue['task_id']);
        dbquery("UPDATE {$db_prefix}buildqueue SET start=" . (time() - 120) . ", end=" . (time() - 30) . " WHERE id=" . (int)$queue['sub_id']);
    }
    InvalidateUserCache();

    return array(
        'login' => mb_strtolower($user['name'], 'UTF-8'),
        'player_id' => $playerId,
        'home_planet_id' => $homePlanetId,
        'moon_id' => $moonId,
        'queue_task_id' => $queue === null ? 0 : (int)$queue['task_id'],
        'build_error' => $buildError,
    );
}

function smoke_prepare_fleet_template_fixture(string $password, array $near): array
{
    global $db_prefix;

    $commander = smoke_prepare_user('gotemplatecommander', $password, 'gotemplatecommander@example.local', USER_TYPE_PLAYER);
    $nonCommander = smoke_prepare_user('gotemplatenocom', $password, 'gotemplatenocom@example.local', USER_TYPE_PLAYER);
    $foreign = smoke_prepare_user('gotemplateforeign', $password, 'gotemplateforeign@example.local', USER_TYPE_PLAYER);
    $users = array($commander, $nonCommander, $foreign);
    $userIds = array_map(fn($user) => (int)$user['player_id'], $users);
    $planetIds = array_map(fn($user) => (int)$user['home_planet_id'], $users);

    smoke_cleanup_alliances($userIds);
    smoke_cleanup_fleets($userIds, $planetIds);
    dbquery("DELETE FROM {$db_prefix}template WHERE owner_id IN (" . implode(',', $userIds) . ")");
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id IN (" . implode(',', $userIds) . ") AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "')");

    $positions = smoke_find_empty_positions($near, count($users));
    foreach ($users as $index => $user) {
        smoke_prepare_planet((int)$user['home_planet_id'], (int)$user['player_id'], 'GoTemplate' . $index, $positions[$index]);
    }

    $now = time();
    $commanderUntil = $now + 7 * 24 * 60 * 60;
    dbquery(
        "UPDATE {$db_prefix}users SET admin=0, validated=1, deact_ip=1, vacation=0, vacation_until=0, " .
        "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, " .
        "`" . GID_R_COMPUTER . "`=1, com_until={$commanderUntil}, lastclick={$now} " .
        "WHERE player_id IN (" . (int)$commander['player_id'] . "," . (int)$foreign['player_id'] . ")"
    );
    dbquery(
        "UPDATE {$db_prefix}users SET admin=0, validated=1, deact_ip=1, vacation=0, vacation_until=0, " .
        "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, " .
        "`" . GID_R_COMPUTER . "`=1, com_until=0, lastclick={$now} " .
        "WHERE player_id=" . (int)$nonCommander['player_id']
    );
    InvalidateUserCache();

    return array(
        'commander' => array(
            'login' => mb_strtolower($commander['name'], 'UTF-8'),
            'player_id' => (int)$commander['player_id'],
            'home_planet_id' => (int)$commander['home_planet_id'],
        ),
        'non_commander' => array(
            'login' => mb_strtolower($nonCommander['name'], 'UTF-8'),
            'player_id' => (int)$nonCommander['player_id'],
            'home_planet_id' => (int)$nonCommander['home_planet_id'],
        ),
        'foreign' => array(
            'login' => mb_strtolower($foreign['name'], 'UTF-8'),
            'player_id' => (int)$foreign['player_id'],
            'home_planet_id' => (int)$foreign['home_planet_id'],
        ),
        'expected_max' => 2,
    );
}

function smoke_prepare_galaxy_remote_fixture(string $password, array $near): array
{
    global $db_prefix, $GlobalUni;

    $enough = smoke_prepare_user('gogalaxyremote', $password, 'gogalaxyremote@example.local', USER_TYPE_PLAYER);
    $low = smoke_prepare_user('gogalaxynodeut', $password, 'gogalaxynodeut@example.local', USER_TYPE_PLAYER);
    $users = array($enough, $low);
    $userIds = array_map(fn($user) => (int)$user['player_id'], $users);
    $planetIds = array_map(fn($user) => (int)$user['home_planet_id'], $users);

    smoke_cleanup_alliances($userIds);
    smoke_cleanup_fleets($userIds, $planetIds);
    $positions = smoke_find_empty_positions($near, count($users));
    foreach ($users as $index => $user) {
        smoke_prepare_planet((int)$user['home_planet_id'], (int)$user['player_id'], 'GoGalaxy' . $index, $positions[$index]);
    }

    $now = time();
    $systems = max(1, (int)$GlobalUni['systems']);
    $enoughHome = LoadPlanetById((int)$enough['home_planet_id']);
    $lowHome = LoadPlanetById((int)$low['home_planet_id']);
    if ($enoughHome === null || $lowHome === null) {
        throw new RuntimeException('Go galaxy remote home planet is missing.');
    }
    $remoteSystemFor = function (array $home) use ($systems): int {
        return (int)$home['s'] < $systems ? (int)$home['s'] + 1 : max(1, (int)$home['s'] - 1);
    };
    dbquery(
        "UPDATE {$db_prefix}users SET admin=0, validated=1, deact_ip=1, vacation=0, vacation_until=0, " .
        "banned=0, banned_until=0, noattack=0, noattack_until=0, disable=0, disable_until=0, lastclick={$now} " .
        "WHERE player_id IN (" . implode(',', $userIds) . ")"
    );
    dbquery("UPDATE {$db_prefix}planets SET `" . GID_RC_DEUTERIUM . "`=25, lastpeek={$now} WHERE planet_id=" . (int)$enough['home_planet_id']);
    dbquery("UPDATE {$db_prefix}planets SET `" . GID_RC_DEUTERIUM . "`=0, lastpeek={$now} WHERE planet_id=" . (int)$low['home_planet_id']);
    InvalidateUserCache();

    return array(
        'enough' => array(
            'login' => mb_strtolower($enough['name'], 'UTF-8'),
            'player_id' => (int)$enough['player_id'],
            'home_planet_id' => (int)$enough['home_planet_id'],
            'initial_deuterium' => 25,
            'remote_galaxy' => (int)$enoughHome['g'],
            'remote_system' => $remoteSystemFor($enoughHome),
        ),
        'low' => array(
            'login' => mb_strtolower($low['name'], 'UTF-8'),
            'player_id' => (int)$low['player_id'],
            'home_planet_id' => (int)$low['home_planet_id'],
            'initial_deuterium' => 0,
            'remote_galaxy' => (int)$lowHome['g'],
            'remote_system' => $remoteSystemFor($lowHome),
        ),
        'cost' => GALAXY_DEUTERIUM_CONS,
    );
}

function smoke_prepare_galaxy_missile_fixture(string $password, array $near): array
{
    global $db_prefix, $defmap;

    $attacker = smoke_prepare_user('gogmissile', $password, 'gogmissile@example.local', USER_TYPE_PLAYER);
    $target = smoke_prepare_user('gogmissilet', $password, 'gogmissilet@example.local', USER_TYPE_PLAYER);
    $users = array($attacker, $target);
    $userIds = array_map(fn($user) => (int)$user['player_id'], $users);
    $planetIds = array_map(fn($user) => (int)$user['home_planet_id'], $users);

    smoke_cleanup_alliances($userIds);
    smoke_cleanup_fleets($userIds, $planetIds);
    $positions = smoke_find_empty_positions($near, count($users));

    $defenseZero = array();
    foreach ($defmap as $gid) {
        $defenseZero[] = "`{$gid}`=0";
    }
    $now = time();

    smoke_set_fleet_restriction_user_state($attacker, 10000);
    smoke_set_fleet_restriction_user_state($target, 10000);
    smoke_prepare_planet((int)$attacker['home_planet_id'], (int)$attacker['player_id'], 'GoGalaxyMissileA', $positions[0]);
    smoke_prepare_planet((int)$target['home_planet_id'], (int)$target['player_id'], 'GoGalaxyMissileT', $positions[1]);

    dbquery(
        "UPDATE {$db_prefix}planets SET " . implode(',', $defenseZero) . ", " .
        "`" . GID_B_MISS_SILO . "`=6, `" . GID_D_IPM . "`=3, `" . GID_RC_DEUTERIUM . "`=1000000, lastpeek={$now} " .
        "WHERE planet_id=" . (int)$attacker['home_planet_id']
    );
    dbquery(
        "UPDATE {$db_prefix}planets SET " . implode(',', $defenseZero) . ", " .
        "`" . GID_D_RL . "`=20, `" . GID_D_LL . "`=5, `" . GID_D_ABM . "`=0, lastpeek={$now} " .
        "WHERE planet_id=" . (int)$target['home_planet_id']
    );
    InvalidateUserCache();

    return array(
        'attacker' => array(
            'login' => mb_strtolower($attacker['name'], 'UTF-8'),
            'player_id' => (int)$attacker['player_id'],
            'home_planet_id' => (int)$attacker['home_planet_id'],
            'initial_missiles' => 3,
            'coordinates' => array(
                'galaxy' => (int)$positions[0][0],
                'system' => (int)$positions[0][1],
                'position' => (int)$positions[0][2],
            ),
        ),
        'target' => array(
            'player_id' => (int)$target['player_id'],
            'home_planet_id' => (int)$target['home_planet_id'],
            'coordinates' => array(
                'galaxy' => (int)$positions[1][0],
                'system' => (int)$positions[1][1],
                'position' => (int)$positions[1][2],
            ),
        ),
        'launch_amount' => 2,
        'target_defense_id' => GID_D_RL,
    );
}

function smoke_prepare_buddy_lifecycle_fixture(string $password, array $near): array
{
    global $db_prefix;

    $requester = smoke_prepare_user('gobuddya', $password, 'gobuddya@example.local', USER_TYPE_PLAYER);
    $recipient = smoke_prepare_user('gobuddyb', $password, 'gobuddyb@example.local', USER_TYPE_PLAYER);
    $users = array($requester, $recipient);
    $userIds = array_map(fn($user) => (int)$user['player_id'], $users);
    $planetIds = array_map(fn($user) => (int)$user['home_planet_id'], $users);
    $userList = implode(',', $userIds);

    smoke_cleanup_alliances($userIds);
    smoke_cleanup_fleets($userIds, $planetIds);
    dbquery("DELETE FROM {$db_prefix}buddy WHERE request_from IN ({$userList}) OR request_to IN ({$userList})");
    dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$userList}) AND subj IN ('Buddy request', 'confirm')");

    $positions = smoke_find_empty_positions($near, count($users));
    foreach ($users as $index => $user) {
        smoke_set_fleet_restriction_user_state($user, 10000);
        smoke_prepare_planet((int)$user['home_planet_id'], (int)$user['player_id'], 'GoBuddy' . $index, $positions[$index]);
    }

    return array(
        'requester' => array(
            'login' => mb_strtolower($requester['name'], 'UTF-8'),
            'player_id' => (int)$requester['player_id'],
            'home_planet_id' => (int)$requester['home_planet_id'],
        ),
        'recipient' => array(
            'login' => mb_strtolower($recipient['name'], 'UTF-8'),
            'player_id' => (int)$recipient['player_id'],
            'home_planet_id' => (int)$recipient['home_planet_id'],
        ),
    );
}

function smoke_prepare_message_scope_fixture(string $password, array $near): array
{
    global $db_prefix;

    $owner = smoke_prepare_user('gomsgown', $password, 'gomsgown@example.local', USER_TYPE_PLAYER);
    $foreign = smoke_prepare_user('gomsgfor', $password, 'gomsgfor@example.local', USER_TYPE_PLAYER);
    $users = array($owner, $foreign);
    $userIds = array_map(fn($user) => (int)$user['player_id'], $users);
    $planetIds = array_map(fn($user) => (int)$user['home_planet_id'], $users);
    $userList = implode(',', $userIds);

    smoke_cleanup_alliances($userIds);
    smoke_cleanup_fleets($userIds, $planetIds);
    dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$userList})");

    $positions = smoke_find_empty_positions($near, count($users));
    foreach ($users as $index => $user) {
        smoke_set_fleet_restriction_user_state($user, 10000);
        smoke_prepare_planet((int)$user['home_planet_id'], (int)$user['player_id'], 'GoMsgScope' . $index, $positions[$index]);
    }

    $now = time();
    return array(
        'owner' => array(
            'login' => mb_strtolower($owner['name'], 'UTF-8'),
            'player_id' => (int)$owner['player_id'],
            'home_planet_id' => (int)$owner['home_planet_id'],
        ),
        'foreign' => array(
            'login' => mb_strtolower($foreign['name'], 'UTF-8'),
            'player_id' => (int)$foreign['player_id'],
            'home_planet_id' => (int)$foreign['home_planet_id'],
        ),
        'owner_selected_id' => SendMessage((int)$owner['player_id'], 'Go Msg Scope', 'GoMsgScope owner selected', 'owner selected body', MTYP_MISC, $now + 4),
        'owner_bulk_id' => SendMessage((int)$owner['player_id'], 'Go Msg Scope', 'GoMsgScope owner bulk', 'owner bulk body', MTYP_MISC, $now + 3),
        'foreign_selected_id' => SendMessage((int)$foreign['player_id'], 'Go Msg Scope', 'GoMsgScope foreign selected', 'foreign selected body', MTYP_MISC, $now + 2),
        'foreign_bulk_id' => SendMessage((int)$foreign['player_id'], 'Go Msg Scope', 'GoMsgScope foreign bulk', 'foreign bulk body', MTYP_MISC, $now + 1),
        'owner_report_id' => SendMessage((int)$owner['player_id'], 'Go Msg Scope', 'GoMsgScope owner report', 'owner report body', MTYP_PM, $now + 6),
        'foreign_report_id' => SendMessage((int)$foreign['player_id'], 'Go Msg Scope', 'GoMsgScope foreign report', 'foreign report body', MTYP_PM, $now + 5),
    );
}

function smoke_prepare_message_retention_fixture(string $password, array $near): array
{
    global $db_prefix;

    $regular = smoke_prepare_user('gomsgret', $password, 'gomsgret@example.local', USER_TYPE_PLAYER);
    $operator = smoke_prepare_user('gomsgadm', $password, 'gomsgadm@example.local', USER_TYPE_GO);
    $users = array($regular, $operator);
    $userIds = array_map(fn($user) => (int)$user['player_id'], $users);
    $planetIds = array_map(fn($user) => (int)$user['home_planet_id'], $users);
    $userList = implode(',', $userIds);

    smoke_cleanup_alliances($userIds);
    smoke_cleanup_fleets($userIds, $planetIds);
    dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$userList})");
    dbquery("UPDATE {$db_prefix}users SET com_until=0 WHERE player_id IN ({$userList})");

    $positions = smoke_find_empty_positions($near, count($users));
    foreach ($users as $index => $user) {
        $options = $index === 1 ? array('admin' => USER_TYPE_GO) : array();
        smoke_set_fleet_restriction_user_state($user, 10000, $options);
        smoke_prepare_planet((int)$user['home_planet_id'], (int)$user['player_id'], 'GoMsgRet' . $index, $positions[$index]);
    }

    $now = time();
    return array(
        'regular' => array(
            'login' => mb_strtolower($regular['name'], 'UTF-8'),
            'player_id' => (int)$regular['player_id'],
            'home_planet_id' => (int)$regular['home_planet_id'],
        ),
        'operator' => array(
            'login' => mb_strtolower($operator['name'], 'UTF-8'),
            'player_id' => (int)$operator['player_id'],
            'home_planet_id' => (int)$operator['home_planet_id'],
        ),
        'regular_old_id' => SendMessage((int)$regular['player_id'], 'Go Msg Retention', 'GoMsgRetention old regular', 'old regular body', MTYP_MISC, $now - 3 * 24 * 60 * 60),
        'regular_fresh_id' => SendMessage((int)$regular['player_id'], 'Go Msg Retention', 'GoMsgRetention fresh regular', 'fresh regular body', MTYP_MISC, $now + 1),
        'operator_old_id' => SendMessage((int)$operator['player_id'], 'Go Msg Retention', 'GoMsgRetention old operator', 'old operator body', MTYP_MISC, $now - 3 * 24 * 60 * 60),
    );
}

function smoke_prepare_message_bulk_delete_fixture(string $password, array $near): array
{
    global $db_prefix;

    $user = smoke_prepare_user('gomsgbulk', $password, 'gomsgbulk@example.local', USER_TYPE_PLAYER);
    $playerId = (int)$user['player_id'];
    $planetId = (int)$user['home_planet_id'];

    smoke_cleanup_alliances(array($playerId));
    smoke_cleanup_fleets(array($playerId), array($planetId));
    dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id={$playerId}");
    dbquery("UPDATE {$db_prefix}users SET com_until=0 WHERE player_id={$playerId}");
    smoke_set_fleet_restriction_user_state($user, 10000);
    smoke_prepare_planet($planetId, $playerId, 'GoMsgBulk', smoke_find_empty_position($near));

    $now = time();
    $prefix = 'GoMsgBulk ';
    $expectedRemaining = array();
    for ($i = 0; $i < 30; $i++) {
        $subject = $prefix . sprintf('%02d', $i);
        SendMessage($playerId, 'Go Msg Bulk', $subject, 'bulk delete body ' . $i, MTYP_MISC, $now + $i);
        if ($i < 5) {
            $expectedRemaining[] = $subject;
        }
    }

    return array(
        'user' => array(
            'login' => mb_strtolower($user['name'], 'UTF-8'),
            'player_id' => $playerId,
            'home_planet_id' => $planetId,
        ),
        'prefix' => $prefix,
        'total_messages' => 30,
        'visible_limit' => 25,
        'expected_remaining_subjects' => $expectedRemaining,
    );
}

function smoke_prepare_message_nonmarked_delete_fixture(string $password, array $near): array
{
    global $db_prefix;

    $user = smoke_prepare_user('gomsgnon', $password, 'gomsgnon@example.local', USER_TYPE_PLAYER);
    $playerId = (int)$user['player_id'];
    $planetId = (int)$user['home_planet_id'];

    smoke_cleanup_alliances(array($playerId));
    smoke_cleanup_fleets(array($playerId), array($planetId));
    dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id={$playerId}");
    dbquery("UPDATE {$db_prefix}users SET com_until=0 WHERE player_id={$playerId}");
    smoke_set_fleet_restriction_user_state($user, 10000);
    smoke_prepare_planet($planetId, $playerId, 'GoMsgNon', smoke_find_empty_position($near));

    $now = time();
    return array(
        'user' => array(
            'login' => mb_strtolower($user['name'], 'UTF-8'),
            'player_id' => $playerId,
            'home_planet_id' => $planetId,
        ),
        'selected_id' => SendMessage($playerId, 'Go Msg Nonmarked', 'GoMsgNon selected', 'selected body', MTYP_MISC, $now + 3),
        'unselected_a_id' => SendMessage($playerId, 'Go Msg Nonmarked', 'GoMsgNon unselected A', 'unselected a body', MTYP_MISC, $now + 2),
        'unselected_b_id' => SendMessage($playerId, 'Go Msg Nonmarked', 'GoMsgNon unselected B', 'unselected b body', MTYP_MISC, $now + 1),
    );
}

function smoke_prepare_message_send_fixture(string $password, array $near): array
{
    global $db_prefix;

    $sender = smoke_prepare_user('gomsgsnd', $password, 'gomsgsnd@example.local', USER_TYPE_PLAYER);
    $recipient = smoke_prepare_user('gomsgrcv', $password, 'gomsgrcv@example.local', USER_TYPE_PLAYER);
    $users = array($sender, $recipient);
    $userIds = array_map(fn($user) => (int)$user['player_id'], $users);
    $planetIds = array_map(fn($user) => (int)$user['home_planet_id'], $users);
    $userList = implode(',', $userIds);

    smoke_cleanup_alliances($userIds);
    smoke_cleanup_fleets($userIds, $planetIds);
    dbquery("DELETE FROM {$db_prefix}messages WHERE owner_id IN ({$userList})");

    $positions = smoke_find_empty_positions($near, count($users));
    foreach ($users as $index => $user) {
        smoke_set_fleet_restriction_user_state($user, 10000);
        smoke_prepare_planet((int)$user['home_planet_id'], (int)$user['player_id'], 'GoMsgSend' . $index, $positions[$index]);
    }

    return array(
        'sender' => array(
            'login' => mb_strtolower($sender['name'], 'UTF-8'),
            'player_id' => (int)$sender['player_id'],
            'home_planet_id' => (int)$sender['home_planet_id'],
        ),
        'recipient' => array(
            'login' => mb_strtolower($recipient['name'], 'UTF-8'),
            'player_id' => (int)$recipient['player_id'],
            'home_planet_id' => (int)$recipient['home_planet_id'],
        ),
        'subject' => 'GoMsgSend subject',
        'text' => 'GoMsgSend body',
    );
}

function smoke_prepare_resource_scope_fixture(string $password, array $near): array
{
    global $db_prefix;

    $owner = smoke_prepare_user('goresown', $password, 'goresown@example.local', USER_TYPE_PLAYER);
    $foreign = smoke_prepare_user('goresfor', $password, 'goresfor@example.local', USER_TYPE_PLAYER);
    $users = array($owner, $foreign);
    $userIds = array_map(fn($user) => (int)$user['player_id'], $users);
    $planetIds = array_map(fn($user) => (int)$user['home_planet_id'], $users);

    smoke_cleanup_alliances($userIds);
    smoke_cleanup_fleets($userIds, $planetIds);
    $positions = smoke_find_empty_positions($near, count($users));
    foreach ($users as $index => $user) {
        smoke_set_fleet_restriction_user_state($user, 10000);
        smoke_prepare_planet((int)$user['home_planet_id'], (int)$user['player_id'], 'GoResScope' . $index, $positions[$index]);
    }

    $resourceSetup =
        "`" . GID_B_METAL_MINE . "`=10, `" . GID_B_CRYS_MINE . "`=10, `" . GID_B_DEUT_SYNTH . "`=10, " .
        "`" . GID_B_SOLAR . "`=12, `" . GID_B_FUSION . "`=0, `" . GID_F_SAT . "`=0";
    dbquery("UPDATE {$db_prefix}planets SET {$resourceSetup}, prod1=1, prod2=1, prod3=1, prod4=1, prod12=0, prod212=0 WHERE planet_id=" . (int)$owner['home_planet_id']);
    dbquery("UPDATE {$db_prefix}planets SET {$resourceSetup}, prod1=0.8, prod2=0.7, prod3=1, prod4=1, prod12=0, prod212=0 WHERE planet_id=" . (int)$foreign['home_planet_id']);
    InvalidateUserCache();

    return array(
        'owner' => array(
            'login' => mb_strtolower($owner['name'], 'UTF-8'),
            'player_id' => (int)$owner['player_id'],
            'home_planet_id' => (int)$owner['home_planet_id'],
        ),
        'foreign' => array(
            'login' => mb_strtolower($foreign['name'], 'UTF-8'),
            'player_id' => (int)$foreign['player_id'],
            'home_planet_id' => (int)$foreign['home_planet_id'],
        ),
        'foreign_initial_metal_percent' => 80,
        'foreign_initial_crystal_percent' => 70,
    );
}

function smoke_prepare_input_hardening_fixture(string $password, array $near): array
{
    global $db_prefix, $fleetmap, $resmap, $GlobalUni;

    $attacker = smoke_prepare_user('gohardenatt', $password, 'gohardenatt@example.local', USER_TYPE_PLAYER);
    $defender = smoke_prepare_user('gohardendef', $password, 'gohardendef@example.local', USER_TYPE_PLAYER);
    $users = array($attacker, $defender);
    $userIds = array_map(fn($user) => (int)$user['player_id'], $users);
    $planetIds = array_map(fn($user) => (int)$user['home_planet_id'], $users);

    smoke_cleanup_alliances($userIds);
    smoke_cleanup_fleets($userIds, $planetIds);
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id IN (" . implode(',', $userIds) . ") AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id IN (" . implode(',', $userIds) . ") OR planet_id IN (" . implode(',', $planetIds) . ")");

    $positions = smoke_find_empty_positions($near, count($users));
    foreach ($users as $index => $user) {
        smoke_prepare_planet((int)$user['home_planet_id'], (int)$user['player_id'], 'GoHarden' . $index, $positions[$index]);
    }

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=10";
    }
    $now = time();
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", admin=0, validated=1, deact_ip=1, " .
        "vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, " .
        "disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, score1=10000, score2=0, score3=0, " .
        "place1=1, place2=1, place3=1, lastclick={$now} " .
        "WHERE player_id IN (" . implode(',', $userIds) . ")"
    );
    dbquery(
        "UPDATE {$db_prefix}planets SET `" . GID_B_ROBOTS . "`=10, `" . GID_B_SHIPYARD . "`=12, `" . GID_B_MISS_SILO . "`=2, " .
        "`" . GID_F_SC . "`=10, `" . GID_F_LF . "`=10, `" . GID_F_PROBE . "`=10, `" . GID_D_ABM . "`=5, `" . GID_D_IPM . "`=3, " .
        "`" . GID_RC_METAL . "`=10000000, `" . GID_RC_CRYSTAL . "`=10000000, `" . GID_RC_DEUTERIUM . "`=10000000, " .
        "lastpeek={$now}, lastakt={$now} WHERE planet_id=" . (int)$attacker['home_planet_id']
    );
    dbquery(
        "UPDATE {$db_prefix}planets SET `" . GID_D_RL . "`=20, `" . GID_D_LL . "`=5, lastpeek={$now}, lastakt={$now} " .
        "WHERE planet_id=" . (int)$defender['home_planet_id']
    );
    InvalidateUserCache();

    return array(
        'attacker' => array(
            'login' => mb_strtolower($attacker['name'], 'UTF-8'),
            'player_id' => (int)$attacker['player_id'],
            'home_planet_id' => (int)$attacker['home_planet_id'],
            'coordinates' => array(
                'galaxy' => (int)$positions[0][0],
                'system' => (int)$positions[0][1],
                'position' => (int)$positions[0][2],
            ),
        ),
        'defender' => array(
            'login' => mb_strtolower($defender['name'], 'UTF-8'),
            'player_id' => (int)$defender['player_id'],
            'home_planet_id' => (int)$defender['home_planet_id'],
            'coordinates' => array(
                'galaxy' => (int)$positions[1][0],
                'system' => (int)$positions[1][1],
                'position' => (int)$positions[1][2],
            ),
        ),
        'max_shipyard' => (int)$GlobalUni['max_werf'],
        'initial_abm' => 5,
        'initial_ipm' => 3,
    );
}

function smoke_prepare_fleet_recall_fixture(string $password, array $near): array
{
    global $db_prefix, $fleetmap, $transportableResources, $resmap;

    $attacker = smoke_prepare_user('gorecallatt', $password, 'gorecallatt@example.local', USER_TYPE_PLAYER);
    $defender = smoke_prepare_user('gorecalldef', $password, 'gorecalldef@example.local', USER_TYPE_PLAYER);
    $users = array($attacker, $defender);
    $userIds = array_map(fn($user) => (int)$user['player_id'], $users);
    $planetIds = array_map(fn($user) => (int)$user['home_planet_id'], $users);

    smoke_cleanup_alliances($userIds);
    smoke_cleanup_fleets($userIds, $planetIds);
    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id IN (" . implode(',', $userIds) . ") AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_RESEARCH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id IN (" . implode(',', $userIds) . ") OR planet_id IN (" . implode(',', $planetIds) . ")");

    $positions = smoke_find_empty_positions($near, count($users));
    foreach ($users as $index => $user) {
        smoke_prepare_planet((int)$user['home_planet_id'], (int)$user['player_id'], 'GoRecall' . $index, $positions[$index]);
    }

    $research = array();
    foreach ($resmap as $gid) {
        $research[] = "`{$gid}`=10";
    }
    $now = time();
    dbquery(
        "UPDATE {$db_prefix}users SET " . implode(',', $research) . ", admin=0, validated=1, deact_ip=1, " .
        "vacation=0, vacation_until=0, banned=0, banned_until=0, noattack=0, noattack_until=0, " .
        "disable=0, disable_until=0, lang='en', skin='/evolution/', useskin=1, score1=10000, score2=0, score3=0, " .
        "place1=1, place2=1, place3=1, lastclick={$now} WHERE player_id IN (" . implode(',', $userIds) . ")"
    );
    dbquery(
        "UPDATE {$db_prefix}planets SET `" . GID_F_SC . "`=10, `" . GID_F_LF . "`=10, `" . GID_F_PROBE . "`=10, " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000, " .
        "lastpeek={$now}, lastakt={$now} WHERE planet_id IN (" . implode(',', $planetIds) . ")"
    );

    $fleet = array();
    foreach ($fleetmap as $gid) {
        $fleet[$gid] = $gid === GID_F_SC ? 1 : 0;
    }
    $ownResources = array();
    $foreignResources = array();
    foreach ($transportableResources as $gid) {
        $ownResources[$gid] = $gid === GID_RC_METAL ? 77 : 0;
        $foreignResources[$gid] = $gid === GID_RC_METAL ? 17 : 0;
    }

    $attackerPlanet = LoadPlanetById((int)$attacker['home_planet_id']);
    $defenderPlanet = LoadPlanetById((int)$defender['home_planet_id']);
    if ($attackerPlanet === null || $defenderPlanet === null) {
        throw new RuntimeException('Go fleet recall fixture planets are missing.');
    }
    AdjustShips($fleet, (int)$attacker['home_planet_id'], '-');
    AdjustResources($ownResources, (int)$attacker['home_planet_id'], '-');
    $ownFleetId = DispatchFleet($fleet, $attackerPlanet, $defenderPlanet, FTYP_TRANSPORT, 3600, $ownResources, 0, time());
    smoke_age_fleet_queue($ownFleetId, 120);

    AdjustShips($fleet, (int)$defender['home_planet_id'], '-');
    AdjustResources($foreignResources, (int)$defender['home_planet_id'], '-');
    $foreignFleetId = DispatchFleet($fleet, $defenderPlanet, $attackerPlanet, FTYP_TRANSPORT, 3600, $foreignResources, 0, time());
    smoke_age_fleet_queue($foreignFleetId, 120);
    InvalidateUserCache();

    return array(
        'attacker' => array(
            'login' => mb_strtolower($attacker['name'], 'UTF-8'),
            'player_id' => (int)$attacker['player_id'],
            'home_planet_id' => (int)$attacker['home_planet_id'],
        ),
        'defender' => array(
            'login' => mb_strtolower($defender['name'], 'UTF-8'),
            'player_id' => (int)$defender['player_id'],
            'home_planet_id' => (int)$defender['home_planet_id'],
        ),
        'own_fleet_id' => $ownFleetId,
        'foreign_fleet_id' => $foreignFleetId,
        'own_cargo_metal' => 77,
        'foreign_cargo_metal' => 17,
    );
}

$name = getenv('OGAME_GO_LOGIN_SMOKE_USER') ?: 'legor';
$password = getenv('OGAME_GO_LOGIN_SMOKE_PASS') ?: 'admin';
$email = getenv('OGAME_GO_LOGIN_SMOKE_EMAIL') ?: ($name . '@example.local');

$login = smoke_prepare_user($name, $password, $email, USER_TYPE_ADMIN);
$operator = smoke_prepare_user('gooperator', $password, 'gooperator@example.local', USER_TYPE_GO);
$target = smoke_prepare_user('gophalaxtarget', $password, 'gophalaxtarget@example.local', 0);
$freezeVictim = smoke_prepare_user('gofreezevictim', $password, 'gofreezevictim@example.local', 0);
smoke_cleanup_alliances(array((int)$login['player_id'], (int)$operator['player_id'], (int)$target['player_id'], (int)$freezeVictim['player_id']));
$home = LoadPlanetById((int)$login['home_planet_id']);
if ($home === null) {
	throw new RuntimeException('Go smoke home planet is missing.');
}
$targetCoords = smoke_find_empty_position($home);
smoke_cleanup_fleets(array((int)$login['player_id'], (int)$operator['player_id'], (int)$target['player_id'], (int)$freezeVictim['player_id']), array((int)$login['home_planet_id'], (int)$operator['home_planet_id'], (int)$target['home_planet_id'], (int)$freezeVictim['home_planet_id']));
smoke_prepare_planet((int)$login['home_planet_id'], (int)$login['player_id'], 'Go Smoke Home', array((int)$home['g'], (int)$home['s'], (int)$home['p']));
smoke_prepare_planet((int)$target['home_planet_id'], (int)$target['player_id'], 'Go Smoke Target', $targetCoords);
smoke_prepare_planet((int)$freezeVictim['home_planet_id'], (int)$freezeVictim['player_id'], 'Go Freeze', smoke_find_empty_position($home));
$premiumDmFixture = smoke_prepare_premium_dm_fixture($password, $home);
$vacationFreezeFixture = smoke_prepare_vacation_freeze_fixture($password, $home);
$merchantFixture = smoke_prepare_merchant_fixture($password, $home);
$moonBuildFixture = smoke_prepare_moon_build_fixture($password, $home);
$moonId = smoke_prepare_moon((int)$login['home_planet_id'], (int)$login['player_id']);
$targetHome = LoadPlanetById((int)$target['home_planet_id']);
if ($targetHome === null) {
    throw new RuntimeException('Go smoke target planet is missing.');
}
$phalanxEdgeFixture = smoke_prepare_phalanx_edge_fixture($password, $targetHome, (int)$target['home_planet_id'], (int)$login['home_planet_id']);
$fleetId = smoke_dispatch_phalanx_fleet((int)$target['player_id'], (int)$target['home_planet_id'], (int)$login['home_planet_id']);
$recallFleetId = smoke_dispatch_phalanx_fleet((int)$target['player_id'], (int)$target['home_planet_id'], (int)$login['home_planet_id']);
$queueTaskId = smoke_prepare_admin_queue_task((int)$login['player_id']);
$fleetQueueTaskId = smoke_fleet_queue_task_id($fleetId);
$recallFleetQueueTaskId = smoke_fleet_queue_task_id($recallFleetId);
$feedFixture = smoke_prepare_feed_fixture($login, $operator, $target);
$passwordRecoveryFixture = smoke_prepare_password_recovery_fixture('E2E_reset123');
$adminOperationsFixture = smoke_prepare_admin_operations_fixture($operator, $target);
$adminAuditFixture = smoke_prepare_admin_audit_fixture($target);
$fleetRestrictionFixture = smoke_prepare_fleet_restriction_fixture($password, $home);
$fleetTemplateFixture = smoke_prepare_fleet_template_fixture($password, $home);
$galaxyRemoteFixture = smoke_prepare_galaxy_remote_fixture($password, $home);
$galaxyMissileFixture = smoke_prepare_galaxy_missile_fixture($password, $home);
$buddyLifecycleFixture = smoke_prepare_buddy_lifecycle_fixture($password, $home);
$messageScopeFixture = smoke_prepare_message_scope_fixture($password, $home);
$messageRetentionFixture = smoke_prepare_message_retention_fixture($password, $home);
$messageBulkDeleteFixture = smoke_prepare_message_bulk_delete_fixture($password, $home);
$messageNonmarkedDeleteFixture = smoke_prepare_message_nonmarked_delete_fixture($password, $home);
$messageSendFixture = smoke_prepare_message_send_fixture($password, $home);
$resourceScopeFixture = smoke_prepare_resource_scope_fixture($password, $home);
$inputHardeningFixture = smoke_prepare_input_hardening_fixture($password, $home);
$fleetRecallFixture = smoke_prepare_fleet_recall_fixture($password, $home);
SelectPlanet((int)$login['player_id'], (int)$login['home_planet_id']);

echo json_encode(array(
	'player_id' => (int)$login['player_id'],
	'name' => $login['name'],
	'home_planet_id' => (int)$login['home_planet_id'],
	'operator' => array(
		'player_id' => (int)$operator['player_id'],
		'name' => $operator['name'],
		'home_planet_id' => (int)$operator['home_planet_id'],
	),
	'admin_queue' => array(
		'task_id' => $queueTaskId,
	),
	'admin_fleetlogs' => array(
		'task_id' => $fleetQueueTaskId,
		'recall_task_id' => $recallFleetQueueTaskId,
		'recall_fleet_id' => $recallFleetId,
	),
	'phalanx' => array(
		'source_moon_id' => $moonId,
		'target_planet_id' => (int)$target['home_planet_id'],
		'target_player_id' => (int)$target['player_id'],
		'fleet_id' => $fleetId,
		'initial_deuterium' => 20000,
		'cost' => 5000,
	),
	'phalanx_edges' => $phalanxEdgeFixture,
	'feed' => $feedFixture,
	'password_recovery' => $passwordRecoveryFixture,
	'admin_operations' => $adminOperationsFixture,
	'admin_audit' => $adminAuditFixture,
	'admin_universe' => array(
		'freeze_victim_player_id' => (int)$freezeVictim['player_id'],
		'freeze_victim_name' => $freezeVictim['name'],
	),
	'premium_dm' => $premiumDmFixture,
	'vacation_freeze' => $vacationFreezeFixture,
	'merchant' => $merchantFixture,
	'moon_build' => $moonBuildFixture,
		'fleet_restrictions' => $fleetRestrictionFixture,
		'fleet_templates' => $fleetTemplateFixture,
		'galaxy_remote' => $galaxyRemoteFixture,
		'galaxy_missile' => $galaxyMissileFixture,
		'buddy_lifecycle' => $buddyLifecycleFixture,
		'message_scope' => $messageScopeFixture,
		'message_retention' => $messageRetentionFixture,
		'message_bulk_delete' => $messageBulkDeleteFixture,
		'message_nonmarked_delete' => $messageNonmarkedDeleteFixture,
		'message_send' => $messageSendFixture,
		'resource_scope' => $resourceScopeFixture,
		'input_hardening' => $inputHardeningFixture,
		'fleet_recall' => $fleetRecallFixture,
	), JSON_UNESCAPED_SLASHES) . PHP_EOL;
