<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-authenticated-game-visual-fixture';
$_SERVER['REQUEST_URI'] = '/testing/e2e/prepare-authenticated-game-visual-fixture.php';
$_COOKIE['ogamelang'] = 'en';

chdir('/var/www/html/game');
require_once 'config.php';
require_once 'core/core.php';

InitDB();
$GlobalUni = LoadUniverse();
$GlobalUser = array('player_id' => 0);
$session = '';
ModsInit();

function auth_visual_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
}

function auth_visual_one_row(string $sql): ?array
{
    $res = dbquery($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function auth_visual_user_by_name(string $name): ?array
{
    global $db_prefix;
    $lower = mb_strtolower($name, 'UTF-8');
    return auth_visual_one_row("SELECT * FROM {$db_prefix}users WHERE name='" . auth_visual_sql_escape($lower) . "' LIMIT 1");
}

function auth_visual_prepare_user(string $name, string $password, int $adminLevel): array
{
    global $db_prefix, $db_secret;

    $lower = mb_strtolower($name, 'UTF-8');
    $displayName = ucfirst($lower);
    $user = auth_visual_user_by_name($lower);
    if ($user === null) {
        $playerId = CreateUser($displayName, $password, $lower . '@visual.local', false);
        $user = auth_visual_one_row("SELECT * FROM {$db_prefix}users WHERE player_id={$playerId} LIMIT 1");
        if ($user === null) {
            throw new RuntimeException('failed to create visual fixture user');
        }
    }

    $playerId = (int)$user['player_id'];
    $homePlanetId = (int)$user['hplanetid'];
    $home = $homePlanetId > 0 ? auth_visual_one_row("SELECT planet_id FROM {$db_prefix}planets WHERE planet_id={$homePlanetId} AND owner_id={$playerId} LIMIT 1") : null;
    if ($home === null) {
        $homePlanetId = CreateHomePlanet($playerId);
        if ($homePlanetId <= 0) {
            throw new RuntimeException('failed to create visual fixture home planet');
        }
    }

    $passwordHash = md5($password . $db_secret);
    $now = time();
    dbquery(
        "UPDATE {$db_prefix}users SET " .
        "name='" . auth_visual_sql_escape($lower) . "', oname='" . auth_visual_sql_escape($displayName) . "', " .
        "password='" . auth_visual_sql_escape($passwordHash) . "', pemail='" . auth_visual_sql_escape($lower . '@visual.local') . "', " .
        "email='" . auth_visual_sql_escape($lower . '@visual.local') . "', validated=1, validatemd='', deact_ip=1, " .
        "admin={$adminLevel}, vacation=0, vacation_until=0, banned=0, banned_until=0, disable=0, disable_until=0, " .
        "ally_id=0, allyrank=0, joindate=0, com_until=0, adm_until=0, eng_until=0, geo_until=0, tec_until=0, dm=0, dmfree=5000, trader=1, rate_m=3, rate_k=2, rate_d=1, " .
        "`" . GID_R_COMPUTER . "`=3, `" . GID_R_COMBUST_DRIVE . "`=2, " .
        "score1=10000, score2=0, score3=0, place1=1, place2=1, place3=1, " .
        "noattack=0, noattack_until=0, lang='en', skin='/evolution/', useskin=1, " .
        "hplanetid={$homePlanetId}, aktplanet={$homePlanetId}, lastclick={$now} WHERE player_id={$playerId}"
    );

    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$playerId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$playerId} OR planet_id={$homePlanetId}");
    dbquery("DELETE FROM {$db_prefix}fleet WHERE owner_id={$playerId} OR start_planet={$homePlanetId} OR target_planet={$homePlanetId}");
    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000, " .
        "`" . GID_B_SHIPYARD . "`=2, `" . GID_B_METAL_STOR . "`=10, `" . GID_B_CRYS_STOR . "`=10, `" . GID_B_DEUT_STOR . "`=10, " .
        "`" . GID_F_SC . "`=3, `" . GID_F_LC . "`=1, `" . GID_F_RECYCLER . "`=25, `" . GID_F_PROBE . "`=25, " .
        "prod1=1, prod2=1, prod3=1, prod4=1, prod12=1, prod212=1, fields=0, maxfields=200, lastpeek={$now} " .
        "WHERE planet_id={$homePlanetId} AND owner_id={$playerId}"
    );

    InvalidateUserCache();
    SelectPlanet($playerId, $homePlanetId);

    return array('player_id' => $playerId, 'name' => $displayName, 'home_planet_id' => $homePlanetId);
}

function auth_visual_position_is_clear(int $g, int $s, int $p): bool
{
    if (HasPlanet($g, $s, $p)) {
        return false;
    }
    $moon = LoadPlanet($g, $s, $p, 3);
    $debris = LoadPlanet($g, $s, $p, 2);
    return ($moon === null || $moon === false) && ($debris === null || $debris === false);
}

function auth_visual_find_empty_hover_system(): array
{
    global $GlobalUni;

    for ($g = 1; $g <= (int)$GlobalUni['galaxies']; $g++) {
        for ($s = 1; $s <= (int)$GlobalUni['systems']; $s++) {
            if (auth_visual_position_is_clear($g, $s, 1) && auth_visual_position_is_clear($g, $s, 2) && auth_visual_position_is_clear($g, $s, 3) && auth_visual_position_is_clear($g, $s, 4) && auth_visual_position_is_clear($g, $s, 5) && auth_visual_position_is_clear($g, $s, 6)) {
                return array($g, $s);
            }
        }
    }
    throw new RuntimeException('failed to find an empty visual hover galaxy system');
}

function auth_visual_place_planet(int $planetId, int $ownerId, string $name, int $g, int $s, int $p): void
{
    global $db_prefix;

    $now = time();
    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "name='" . auth_visual_sql_escape($name) . "', g={$g}, s={$s}, p={$p}, type=" . PTYP_PLANET . ", owner_id={$ownerId}, " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000, " .
        "fields=0, maxfields=200, prod1=1, prod2=1, prod3=1, prod4=1, prod12=1, prod212=1, lastpeek={$now}, lastakt={$now} " .
        "WHERE planet_id={$planetId}"
    );
}

function auth_visual_prepare_galaxy_hover_fixture(array $user, string $password): array
{
    global $db_prefix;

    $target = auth_visual_prepare_user('visualhover', $password, USER_TYPE_PLAYER);
    $noobTarget = auth_visual_prepare_user('visualnoob', $password, USER_TYPE_PLAYER);
    $vacationTarget = auth_visual_prepare_user('visualvacation', $password, USER_TYPE_PLAYER);
    $maxFleetUser = auth_visual_prepare_user('visualmax', $password, USER_TYPE_PLAYER);
    $maxFleetTarget = auth_visual_prepare_user('visualmaxtarget', $password, USER_TYPE_PLAYER);
    $viewerId = (int)$user['player_id'];
    $viewerPlanetId = (int)$user['home_planet_id'];
    $targetId = (int)$target['player_id'];
    $targetPlanetId = (int)$target['home_planet_id'];
    $noobTargetId = (int)$noobTarget['player_id'];
    $noobTargetPlanetId = (int)$noobTarget['home_planet_id'];
    $vacationTargetId = (int)$vacationTarget['player_id'];
    $vacationTargetPlanetId = (int)$vacationTarget['home_planet_id'];
    $maxFleetUserId = (int)$maxFleetUser['player_id'];
    $maxFleetPlanetId = (int)$maxFleetUser['home_planet_id'];
    $maxFleetTargetId = (int)$maxFleetTarget['player_id'];
    $maxFleetTargetPlanetId = (int)$maxFleetTarget['home_planet_id'];
    [$g, $s] = auth_visual_find_empty_hover_system();
    $now = time();

    auth_visual_place_planet($targetPlanetId, $targetId, 'Visual Hover Planet', $g, $s, 1);
    auth_visual_place_planet($viewerPlanetId, $viewerId, 'Visual Home', $g, $s, 2);
    auth_visual_place_planet($noobTargetPlanetId, $noobTargetId, 'Visual Noob Planet', $g, $s, 3);
    auth_visual_place_planet($vacationTargetPlanetId, $vacationTargetId, 'Visual Vacation', $g, $s, 4);
    auth_visual_place_planet($maxFleetTargetPlanetId, $maxFleetTargetId, 'Visual Max Target', $g, $s, 5);
    auth_visual_place_planet($maxFleetPlanetId, $maxFleetUserId, 'Visual Max Home', $g, $s, 6);
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$viewerPlanetId}, aktplanet={$viewerPlanetId}, lastclick={$now} WHERE player_id={$viewerId}");
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$targetPlanetId}, aktplanet={$targetPlanetId}, lastclick={$now}, score1=10000, score2=0, score3=0, place1=1, place2=1, place3=1 WHERE player_id={$targetId}");
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$noobTargetPlanetId}, aktplanet={$noobTargetPlanetId}, lastclick={$now}, score1=1, score2=0, score3=0, place1=1, place2=1, place3=1 WHERE player_id={$noobTargetId}");
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$vacationTargetPlanetId}, aktplanet={$vacationTargetPlanetId}, lastclick={$now}, score1=10000, score2=0, score3=0, place1=1, place2=1, place3=1, vacation=1, vacation_until=" . ($now + 86400) . " WHERE player_id={$vacationTargetId}");
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$maxFleetPlanetId}, aktplanet={$maxFleetPlanetId}, lastclick={$now}, score1=10000, score2=0, score3=0, place1=1, place2=1, place3=1, `" . GID_R_COMPUTER . "`=1 WHERE player_id={$maxFleetUserId}");
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$maxFleetTargetPlanetId}, aktplanet={$maxFleetTargetPlanetId}, lastclick={$now}, score1=10000, score2=0, score3=0, place1=1, place2=1, place3=1 WHERE player_id={$maxFleetTargetId}");

    $maxOrigin = LoadPlanetById($maxFleetPlanetId);
    $maxTarget = LoadPlanetById($maxFleetTargetPlanetId);
    $maxResources = array();
    foreach ($GLOBALS['transportableResources'] as $rc) {
        $maxResources[$rc] = 0;
    }
    $maxFleet = array();
    foreach ($GLOBALS['fleetmap'] as $gid) {
        $maxFleet[$gid] = 0;
    }
    $maxFleet[GID_F_PROBE] = 1;
    DispatchFleet($maxFleet, $maxOrigin, $maxTarget, FTYP_SPY, 3600, $maxResources, 0, $now - 120);
    DispatchFleet($maxFleet, $maxOrigin, $maxTarget, FTYP_SPY, 3600, $maxResources, 0, $now - 60);
    $maxFleetAuth = auth_visual_prepare_session($maxFleetUserId);

    $moonId = PlanetHasMoon($targetPlanetId);
    if ($moonId <= 0) {
        $moonId = CreatePlanet($g, $s, 1, $targetId, 1, 1, 20, $now);
    }
    if ($moonId <= 0) {
        throw new RuntimeException('failed to create visual hover moon');
    }
    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "name='Visual Hover Moon', g={$g}, s={$s}, p=1, type=" . PTYP_MOON . ", owner_id={$targetId}, " .
        "`" . GID_RC_METAL . "`=100000, `" . GID_RC_CRYSTAL . "`=100000, `" . GID_RC_DEUTERIUM . "`=20000, " .
        "diameter=8888, temp=-42, fields=2, maxfields=4, lastpeek={$now}, lastakt={$now} WHERE planet_id={$moonId}"
    );

    $debrisId = CreateDebris($g, $s, 1, USER_SPACE);
    if ($debrisId <= 0) {
        throw new RuntimeException('failed to create visual hover debris');
    }
    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "name='Debris', g={$g}, s={$s}, p=1, type=" . PTYP_DF . ", owner_id=" . USER_SPACE . ", " .
        "`" . GID_RC_METAL . "`=120000, `" . GID_RC_CRYSTAL . "`=80000, `" . GID_RC_DEUTERIUM . "`=0, " .
        "lastpeek={$now}, lastakt={$now} WHERE planet_id={$debrisId}"
    );

    $existingAlly = auth_visual_one_row("SELECT ally_id FROM {$db_prefix}ally WHERE tag='VGHT' LIMIT 1");
    if ($existingAlly !== null) {
        DismissAlly((int)$existingAlly['ally_id']);
    }
    $allyId = CreateAlly($targetId, 'VGHT', 'Visual Hover Alliance');
    dbquery("UPDATE {$db_prefix}ally SET place1=1, place2=1, place3=1, score1=10000, score2=0, score3=0 WHERE ally_id={$allyId}");
    InvalidateUserCache();
    SelectPlanet($viewerId, $viewerPlanetId);

    return array(
        'galaxy' => $g,
        'system' => $s,
        'target_position' => 1,
        'viewer_position' => 2,
        'target_player_id' => $targetId,
        'target_planet_id' => $targetPlanetId,
        'noob_target_position' => 3,
        'noob_target_player_id' => $noobTargetId,
        'noob_target_planet_id' => $noobTargetPlanetId,
        'vacation_target_position' => 4,
        'vacation_target_player_id' => $vacationTargetId,
        'vacation_target_planet_id' => $vacationTargetPlanetId,
        'max_fleet_target_position' => 5,
        'max_fleet_player_id' => $maxFleetUserId,
        'max_fleet_planet_id' => $maxFleetPlanetId,
        'max_fleet_target_player_id' => $maxFleetTargetId,
        'max_fleet_target_planet_id' => $maxFleetTargetPlanetId,
        'moon_id' => $moonId,
        'debris_id' => $debrisId,
        'ally_id' => $allyId,
        'max_fleet' => array(
            'login_user' => $maxFleetUser['name'],
            'player_id' => $maxFleetUserId,
            'home_planet_id' => $maxFleetPlanetId,
            'session' => $maxFleetAuth['session'],
            'private_session' => $maxFleetAuth['private_session'],
            'cookies' => $maxFleetAuth['cookies'],
        ),
    );
}

function auth_visual_prepare_commander_fixture(array $user): void
{
    global $db_prefix;

    $playerId = (int)$user['player_id'];
    $now = time();
    $commanderUntil = $now + 60 * 60 * 24 * 30;

    dbquery("UPDATE {$db_prefix}users SET com_until={$commanderUntil}, `" . GID_R_COMPUTER . "`=3 WHERE player_id={$playerId}");
    dbquery("DELETE FROM {$db_prefix}template WHERE owner_id={$playerId}");
    AddDBRow(
        array(
            'owner_id' => $playerId,
            'name' => 'Visual Template',
            'date' => $now,
            GID_F_SC => 3,
            GID_F_LC => 1,
            GID_F_LF => 0,
            GID_F_HF => 0,
            GID_F_CRUISER => 0,
            GID_F_BATTLESHIP => 0,
            GID_F_COLON => 0,
            GID_F_RECYCLER => 2,
            GID_F_PROBE => 4,
            GID_F_BOMBER => 0,
            GID_F_SAT => 0,
            GID_F_DESTRO => 0,
            GID_F_DEATHSTAR => 0,
            GID_F_BATTLECRUISER => 0,
        ),
        'template'
    );
}

function auth_visual_prepare_alliance_fixture(array $user, string $password): array
{
    global $db_prefix;

    $viewerId = (int)$user['player_id'];
    $now = time();

    $existingAlly = auth_visual_one_row("SELECT ally_id FROM {$db_prefix}ally WHERE tag='VQA' LIMIT 1");
    if ($existingAlly !== null) {
        DismissAlly((int)$existingAlly['ally_id']);
    }
    dbquery("UPDATE {$db_prefix}users SET ally_id=0, allyrank=0, joindate=0 WHERE player_id={$viewerId}");

    $allyId = CreateAlly($viewerId, 'VQA', 'Visual QA Alliance');
    dbquery(
        "UPDATE {$db_prefix}ally SET " .
        "homepage='https://visual.example.local', imglogo='', open=1, insertapp=1, " .
        "exttext='Welcome to the visual QA alliance.', inttext='Internal visual QA notice.', apptext='Sample application text', " .
        "place1=1, place2=1, place3=1, score1=1000, score2=0, score3=0 WHERE ally_id={$allyId}"
    );

    $member = auth_visual_prepare_user('visualmember', $password, USER_TYPE_PLAYER);
    $memberId = (int)$member['player_id'];
    dbquery("UPDATE {$db_prefix}users SET ally_id={$allyId}, allyrank=1, joindate={$now}, score1=2000, place1=2 WHERE player_id={$memberId}");

    $applicant = auth_visual_prepare_user('visualapp', $password, USER_TYPE_PLAYER);
    $applicantId = (int)$applicant['player_id'];
    dbquery("UPDATE {$db_prefix}users SET ally_id=0, allyrank=0, joindate=0, score1=500, place1=3 WHERE player_id={$applicantId}");
    dbquery("DELETE FROM {$db_prefix}allyapps WHERE ally_id={$allyId} OR player_id={$applicantId}");
    $applicationId = AddApplication($allyId, $applicantId, "Visual application statement");

    InvalidateUserCache();
    SelectPlanet($viewerId, (int)$user['home_planet_id']);

    return array(
        'ally_id' => $allyId,
        'member_player_id' => $memberId,
        'applicant_player_id' => $applicantId,
        'application_id' => $applicationId,
    );
}

function auth_visual_prepare_report_fixture(array $user): array
{
    global $db_prefix;

    $playerId = (int)$user['player_id'];
    $homePlanetId = (int)$user['home_planet_id'];
    dbquery(
        "DELETE FROM {$db_prefix}messages WHERE owner_id={$playerId} AND subj='Visual Spy Report' " .
        "AND msgfrom='Visual Control'"
    );

    $text =
        "Visual Spy Report<br>" .
        "<table>" .
        "<tr><th>Metal</th><th>1.000.000</th></tr>" .
        "<tr><th>Crystal</th><th>1.000.000</th></tr>" .
        "<tr><th>Deuterium</th><th>1.000.000</th></tr>" .
        "</table>";
    $messageId = SendMessage($playerId, 'Visual Control', 'Visual Spy Report', $text, MTYP_SPY_REPORT, time(), $homePlanetId);
    dbquery("UPDATE {$db_prefix}messages SET shown=1 WHERE msg_id={$messageId}");

    return array('report_id' => $messageId);
}

function auth_visual_prepare_phalanx_fixture(array $galaxyHover): array
{
    return array(
        'target_planet_id' => (int)$galaxyHover['target_planet_id'],
        'state' => 'missing_sensor',
    );
}

function auth_visual_prepare_session(int $playerId): array
{
    global $db_prefix, $GlobalUni;

    $session = substr(md5('auth-game-visual-session-' . $playerId . '-' . microtime(true)), 0, 12);
    $private = md5('auth-game-visual-private-' . $playerId . '-' . microtime(true));
    dbquery(
        "UPDATE {$db_prefix}users SET session='" . auth_visual_sql_escape($session) . "', " .
        "private_session='" . auth_visual_sql_escape($private) . "', lastclick=" . time() . " WHERE player_id={$playerId}"
    );
    InvalidateUserCache();

    return array(
        'session' => $session,
        'private_session' => $private,
        'cookies' => array('prsess_' . $playerId . '_' . $GlobalUni['num'] => $private),
    );
}

try {
    $name = getenv('OGAME_GAME_VISUAL_USER') ?: 'legor';
    $password = getenv('OGAME_GAME_VISUAL_PASS') ?: 'admin';
    $adminLevel = intval(getenv('OGAME_GAME_VISUAL_ADMIN') ?: USER_TYPE_ADMIN);
    $useCommander = getenv('OGAME_GAME_VISUAL_COMMANDER_FIXTURE') === '1';
    $useAlliance = getenv('OGAME_GAME_VISUAL_ALLIANCE_FIXTURE') === '1';
    $useReport = getenv('OGAME_GAME_VISUAL_REPORT_FIXTURE') === '1';
    $usePhalanx = getenv('OGAME_GAME_VISUAL_PHALANX_FIXTURE') === '1';
    $user = auth_visual_prepare_user($name, $password, $adminLevel);
    $galaxyHover = auth_visual_prepare_galaxy_hover_fixture($user, $password);
    $maxFleet = $galaxyHover['max_fleet'] ?? null;
    unset($galaxyHover['max_fleet']);
    $alliance = null;
    $report = null;
    $phalanx = null;
    if ($useCommander) {
        auth_visual_prepare_commander_fixture($user);
    }
    if ($useAlliance) {
        $alliance = auth_visual_prepare_alliance_fixture($user, $password);
    }
    if ($useReport) {
        $report = auth_visual_prepare_report_fixture($user);
    }
    if ($usePhalanx) {
        $phalanx = auth_visual_prepare_phalanx_fixture($galaxyHover);
    }
    $auth = auth_visual_prepare_session((int)$user['player_id']);
    echo json_encode(array(
        'login_user' => $user['name'],
        'player_id' => (int)$user['player_id'],
        'home_planet_id' => (int)$user['home_planet_id'],
        'galaxy_hover' => $galaxyHover,
        'max_fleet' => $maxFleet,
        'alliance' => $alliance,
        'report' => $report,
        'phalanx' => $phalanx,
        'features' => array(
            'commander' => $useCommander,
            'alliance' => $useAlliance,
            'report' => $useReport,
            'phalanx' => $usePhalanx,
        ),
        'session' => $auth['session'],
        'private_session' => $auth['private_session'],
        'cookies' => $auth['cookies'],
    ), JSON_PRETTY_PRINT) . "\n";
} catch (Throwable $e) {
    fwrite(STDERR, $e->getMessage() . "\n");
    exit(1);
}
