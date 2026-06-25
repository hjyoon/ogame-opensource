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
$moonId = smoke_prepare_moon((int)$login['home_planet_id'], (int)$login['player_id']);
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
    'feed' => $feedFixture,
    'password_recovery' => $passwordRecoveryFixture,
    'admin_operations' => $adminOperationsFixture,
    'admin_audit' => $adminAuditFixture,
    'admin_universe' => array(
        'freeze_victim_player_id' => (int)$freezeVictim['player_id'],
        'freeze_victim_name' => $freezeVictim['name'],
    ),
    'premium_dm' => $premiumDmFixture,
    'fleet_restrictions' => $fleetRestrictionFixture,
), JSON_UNESCAPED_SLASHES) . PHP_EOL;
