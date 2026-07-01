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
        "noattack=0, noattack_until=0, lang='en', skin='/evolution/', useskin=1, " .
        "hplanetid={$homePlanetId}, aktplanet={$homePlanetId}, lastclick={$now} WHERE player_id={$playerId}"
    );

    dbquery("DELETE FROM {$db_prefix}queue WHERE owner_id={$playerId} AND type IN ('" . QTYP_BUILD . "','" . QTYP_DEMOLISH . "','" . QTYP_SHIPYARD . "','" . QTYP_FLEET . "')");
    dbquery("DELETE FROM {$db_prefix}buildqueue WHERE owner_id={$playerId} OR planet_id={$homePlanetId}");
    dbquery("DELETE FROM {$db_prefix}fleet WHERE owner_id={$playerId} OR start_planet={$homePlanetId} OR target_planet={$homePlanetId}");
    dbquery(
        "UPDATE {$db_prefix}planets SET " .
        "`" . GID_RC_METAL . "`=1000000, `" . GID_RC_CRYSTAL . "`=1000000, `" . GID_RC_DEUTERIUM . "`=1000000, " .
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
            if (auth_visual_position_is_clear($g, $s, 1) && auth_visual_position_is_clear($g, $s, 2)) {
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
    $viewerId = (int)$user['player_id'];
    $viewerPlanetId = (int)$user['home_planet_id'];
    $targetId = (int)$target['player_id'];
    $targetPlanetId = (int)$target['home_planet_id'];
    [$g, $s] = auth_visual_find_empty_hover_system();
    $now = time();

    auth_visual_place_planet($targetPlanetId, $targetId, 'Visual Hover Planet', $g, $s, 1);
    auth_visual_place_planet($viewerPlanetId, $viewerId, 'Visual Home', $g, $s, 2);
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$viewerPlanetId}, aktplanet={$viewerPlanetId}, lastclick={$now} WHERE player_id={$viewerId}");
    dbquery("UPDATE {$db_prefix}users SET hplanetid={$targetPlanetId}, aktplanet={$targetPlanetId}, lastclick={$now}, score1=1, score2=0, score3=0, place1=1, place2=1, place3=1 WHERE player_id={$targetId}");

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
    dbquery("UPDATE {$db_prefix}ally SET place1=1, place2=1, place3=1, score1=1, score2=0, score3=0 WHERE ally_id={$allyId}");
    InvalidateUserCache();
    SelectPlanet($viewerId, $viewerPlanetId);

    return array(
        'galaxy' => $g,
        'system' => $s,
        'target_position' => 1,
        'viewer_position' => 2,
        'target_player_id' => $targetId,
        'target_planet_id' => $targetPlanetId,
        'moon_id' => $moonId,
        'debris_id' => $debrisId,
        'ally_id' => $allyId,
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
    $user = auth_visual_prepare_user($name, $password, $adminLevel);
    $galaxyHover = auth_visual_prepare_galaxy_hover_fixture($user, $password);
    $auth = auth_visual_prepare_session((int)$user['player_id']);
    echo json_encode(array(
        'login_user' => $user['name'],
        'player_id' => (int)$user['player_id'],
        'home_planet_id' => (int)$user['home_planet_id'],
        'galaxy_hover' => $galaxyHover,
        'session' => $auth['session'],
        'private_session' => $auth['private_session'],
        'cookies' => $auth['cookies'],
    ), JSON_PRETTY_PRINT) . "\n";
} catch (Throwable $e) {
    fwrite(STDERR, $e->getMessage() . "\n");
    exit(1);
}
