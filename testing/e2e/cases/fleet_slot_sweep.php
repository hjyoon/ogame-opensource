<?php

ob_start();
error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = $_SERVER['HTTP_USER_AGENT'] ?? 'ogame-e2e';
$_SERVER['REQUEST_URI'] = $_SERVER['REQUEST_URI'] ?? '/testing/e2e';
$_SERVER['HTTP_USER_AGENT'] = 'codex-cli';
$_SERVER['REQUEST_URI'] = '/tmp/ogame_fleet_slot_sweep.php';
$_COOKIE['ogamelang'] = 'en';

chdir('/var/www/html/game');
require_once 'config.php';
require_once 'core/core.php';

InitDB();
$GlobalUni = LoadUniverse();
$GlobalUser = array('player_id' => 0);
$session = '';
ModsInit();

$ATTACKER_ID = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 100162);
$ATTACKER_PLANET = intval(getenv('OGAME_E2E_ATTACKER_PLANET') ?: 10163);
$DEFENDER_PLANET = intval(getenv('OGAME_E2E_DEFENDER_PLANET') ?: 10161);

function q(string $sql): mixed
{
    $res = dbquery($sql);
    if ($res === false) throw new RuntimeException('SQL failed: ' . $sql);
    return $res;
}

function cleanup_slot_sweep(int $userId, int $originId, int $targetId): void
{
    global $db_prefix;
    q("DELETE FROM {$db_prefix}queue WHERE owner_id={$userId} OR (type='" . QTYP_FLEET . "' AND sub_id IN (SELECT fleet_id FROM {$db_prefix}fleet WHERE owner_id={$userId} OR start_planet IN ({$originId},{$targetId}) OR target_planet IN ({$originId},{$targetId})))");
    q("DELETE FROM {$db_prefix}fleet WHERE owner_id={$userId} OR start_planet IN ({$originId},{$targetId}) OR target_planet IN ({$originId},{$targetId})");
}

function set_computer_level(int $userId, int $level): void
{
    global $db_prefix, $resmap;
    $parts = array();
    foreach ($resmap as $gid) $parts[] = "`{$gid}`=" . ($gid === GID_R_COMPUTER ? $level : 10);
    q("UPDATE {$db_prefix}users SET " . implode(',', $parts) . ", validated=1, adm_until=0 WHERE player_id={$userId}");
    InvalidateUserCache();
}

function add_slot_fleet(int $ownerId, int $mission): int
{
    global $ATTACKER_PLANET, $DEFENDER_PLANET;
    $fleet = array(
        'owner_id' => $ownerId,
        'union_id' => 0,
        'fuel' => 0,
        'mission' => $mission,
        'start_planet' => $ATTACKER_PLANET,
        'target_planet' => $DEFENDER_PLANET,
        'flight_time' => 3600,
        'deploy_time' => 0,
        GID_F_SC => $mission == FTYP_MISSILE ? 0 : 1,
        'ipm_amount' => $mission == FTYP_MISSILE ? 1 : 0,
        'ipm_target' => 0,
    );
    $fleetId = AddDBRow($fleet, 'fleet');
    AddQueue($ownerId, QTYP_FLEET, $fleetId, 0, 0, time(), 3600, QUEUE_PRIO_FLEET + $mission);
    return $fleetId;
}

function slot_state(int $userId): array
{
    global $ATTACKER_PLANET;
    $user = LoadUser($userId);
    $planet = LoadPlanetById($ATTACKER_PLANET);
    $max = $maxNoBonus = 0;
    GetMaxFleet($user, $planet, $max, $maxNoBonus);
    $normal = dbrows(EnumOwnFleetQueue($userId));
    $withIpm = dbrows(EnumOwnFleetQueue($userId, 1));
    return array(
        'computer' => intval($user[GID_R_COMPUTER]),
        'max' => intval($max),
        'max_no_bonus' => intval($maxNoBonus),
        'normal_queue_count' => intval($normal),
        'with_ipm_queue_count' => intval($withIpm),
        'blocked' => $normal >= $max,
    );
}

$levels = range(0, 10);
$cases = array();

try {
    foreach ($levels as $level) {
        cleanup_slot_sweep($ATTACKER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET);
        set_computer_level($ATTACKER_ID, $level);
        $start = slot_state($ATTACKER_ID);
        for ($i = 0; $i < max(0, $level); $i++) add_slot_fleet($ATTACKER_ID, FTYP_TRANSPORT);
        $beforeLimit = slot_state($ATTACKER_ID);
        add_slot_fleet($ATTACKER_ID, FTYP_TRANSPORT);
        $atLimit = slot_state($ATTACKER_ID);
        add_slot_fleet($ATTACKER_ID, FTYP_MISSILE);
        $withIpm = slot_state($ATTACKER_ID);
        $expectedMax = $level + 1;
        $cases[] = array(
            'level' => $level,
            'expected_max' => $expectedMax,
            'start' => $start,
            'before_limit' => $beforeLimit,
            'at_limit' => $atLimit,
            'with_ipm' => $withIpm,
            'pass' =>
                $start['max'] === $expectedMax &&
                $start['normal_queue_count'] === 0 &&
                $beforeLimit['normal_queue_count'] === $level &&
                $beforeLimit['blocked'] === false &&
                $atLimit['normal_queue_count'] === $expectedMax &&
                $atLimit['blocked'] === true &&
                $withIpm['normal_queue_count'] === $expectedMax &&
                $withIpm['with_ipm_queue_count'] === $expectedMax + 1,
        );
    }
} finally {
    cleanup_slot_sweep($ATTACKER_ID, $ATTACKER_PLANET, $DEFENDER_PLANET);
    set_computer_level($ATTACKER_ID, 10);
}

$captured = trim(ob_get_clean());
echo json_encode(array(
    'case' => 'computer_technology_fleet_slot_sweep',
    'levels_tested' => $levels,
    'cases' => $cases,
    'captured_output' => $captured,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
