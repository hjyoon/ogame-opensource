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
    $g = (int)$near['g'];
    $system = (int)$near['s'];
    for ($p = 1; $p <= 15; $p++) {
        if ($p === (int)$near['p']) {
            continue;
        }
        if (!HasPlanet($g, $system, $p)) {
            return array($g, $system, $p);
        }
    }
    throw new RuntimeException('No empty same-system phalanx fixture position found.');
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

$name = getenv('OGAME_GO_LOGIN_SMOKE_USER') ?: 'legor';
$password = getenv('OGAME_GO_LOGIN_SMOKE_PASS') ?: 'admin';
$email = getenv('OGAME_GO_LOGIN_SMOKE_EMAIL') ?: ($name . '@example.local');

$login = smoke_prepare_user($name, $password, $email, USER_TYPE_ADMIN);
$operator = smoke_prepare_user('gooperator', $password, 'gooperator@example.local', USER_TYPE_GO);
$target = smoke_prepare_user('gophalaxtarget', $password, 'gophalaxtarget@example.local', 0);
smoke_cleanup_alliances(array((int)$login['player_id'], (int)$operator['player_id'], (int)$target['player_id']));
$home = LoadPlanetById((int)$login['home_planet_id']);
if ($home === null) {
	throw new RuntimeException('Go smoke home planet is missing.');
}
$targetCoords = smoke_find_empty_position($home);
smoke_cleanup_fleets(array((int)$login['player_id'], (int)$operator['player_id'], (int)$target['player_id']), array((int)$login['home_planet_id'], (int)$operator['home_planet_id'], (int)$target['home_planet_id']));
smoke_prepare_planet((int)$login['home_planet_id'], (int)$login['player_id'], 'Go Smoke Home', array((int)$home['g'], (int)$home['s'], (int)$home['p']));
smoke_prepare_planet((int)$target['home_planet_id'], (int)$target['player_id'], 'Go Smoke Target', $targetCoords);
$moonId = smoke_prepare_moon((int)$login['home_planet_id'], (int)$login['player_id']);
$fleetId = smoke_dispatch_phalanx_fleet((int)$target['player_id'], (int)$target['home_planet_id'], (int)$login['home_planet_id']);
$recallFleetId = smoke_dispatch_phalanx_fleet((int)$target['player_id'], (int)$target['home_planet_id'], (int)$login['home_planet_id']);
$queueTaskId = smoke_prepare_admin_queue_task((int)$login['player_id']);
$fleetQueueTaskId = smoke_fleet_queue_task_id($fleetId);
$recallFleetQueueTaskId = smoke_fleet_queue_task_id($recallFleetId);
$feedFixture = smoke_prepare_feed_fixture($login, $operator, $target);
$passwordRecoveryFixture = smoke_prepare_password_recovery_fixture('E2E_reset123');
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
), JSON_UNESCAPED_SLASHES) . PHP_EOL;
