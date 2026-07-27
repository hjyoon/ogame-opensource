<?php

chdir('/var/www/html/game');
require_once 'core/defs.php';
require_once 'core/techs.php';

$OracleRandomCalls = array();
$OracleRapidRollHigh = true;

function oracle_mt_rand(int $min, int $max): int
{
    global $OracleRandomCalls, $OracleRapidRollHigh;
    if ($min === 1 && $max === RF_DICE) {
        $value = $OracleRapidRollHigh ? $max : $min;
        $OracleRapidRollHigh = !$OracleRapidRollHigh;
    } else {
        $value = $min;
    }
    $OracleRandomCalls[] = array('max' => $max - $min + 1, 'value' => $value - $min);
    return $value;
}

$engineSource = file_get_contents('core/battle_engine.php');
if ($engineSource === false) {
    throw new RuntimeException('unable to read legacy battle engine');
}
$engineSource = str_replace('mt_rand (', 'oracle_mt_rand (', $engineSource, $replacementCount);
if ($replacementCount !== 4) {
    throw new RuntimeException("expected four legacy random call sites, replaced {$replacementCount}");
}
$instrumentedEngine = tempnam(sys_get_temp_dir(), 'ogame-battle-oracle-');
if ($instrumentedEngine === false || file_put_contents($instrumentedEngine, $engineSource) === false) {
    throw new RuntimeException('unable to instrument legacy battle engine');
}
try {
    require $instrumentedEngine;
} finally {
    @unlink($instrumentedEngine);
}

$UnitParamLocal = $UnitParam;
$RapidFireLocal = $RapidFire;

function oracle_slot(array $units): array
{
    return array('weap' => 0, 'shld' => 0, 'armr' => 0, 'units' => $units);
}

function oracle_units(array $slot): array|object
{
    return count($slot['units']) === 0 ? (object)array() : $slot['units'];
}

function oracle_case(string $name, array $attackers, array $defenders, bool $rapidFire = false): array
{
    global $exploded_counter, $already_exploded_counter, $OracleRandomCalls, $OracleRapidRollHigh;
    $exploded_counter = 0;
    $already_exploded_counter = 0;
    $OracleRandomCalls = array();
    $OracleRapidRollHigh = true;
    $result = array('before' => array('attackers' => $attackers, 'defenders' => $defenders));
    DoBattle($result, $rapidFire ? 1 : 0, 6);
    return array('name' => $name, 'rapidFire' => $rapidFire, 'randomCalls' => $OracleRandomCalls, 'outcome' => $result['result'], 'rounds' => array_map(
        static fn(array $round): array => array(
            'attackerShots' => $round['ashoot'],
            'attackerPower' => $round['apower'],
            'defenderAbsorbed' => $round['dabsorb'],
            'defenderShots' => $round['dshoot'],
            'defenderPower' => $round['dpower'],
            'attackerAbsorbed' => $round['aabsorb'],
            'attackers' => array_map('oracle_units', $round['attackers']),
            'defenders' => array_map('oracle_units', $round['defenders']),
        ),
        $result['rounds']
    ));
}

$cases = array(
    oracle_case('unguarded', array(oracle_slot(array(202 => 1))), array(oracle_slot(array()))),
    oracle_case('shield_fast_draw', array(oracle_slot(array(202 => 1))), array(oracle_slot(array(202 => 1)))),
    oracle_case('deathstar_win', array(oracle_slot(array(214 => 1))), array(oracle_slot(array(202 => 1)))),
    oracle_case('plasma_defence_win', array(oracle_slot(array(204 => 1))), array(oracle_slot(array(406 => 10)))),
);

$combatUnits = array_merge($fleetmap, array_slice($defmap, 0, 8));
foreach (array(false, true) as $rapidFire) {
    foreach ($combatUnits as $attackerID) {
        foreach ($combatUnits as $defenderID) {
            $cases[] = oracle_case(
                sprintf('pair_%d_vs_%d_rapid_%s', $attackerID, $defenderID, $rapidFire ? 'true' : 'false'),
                array(oracle_slot(array($attackerID => 1))),
                array(oracle_slot(array($defenderID => 1))),
                $rapidFire
            );
        }
    }
}

echo json_encode(array('cases' => $cases), JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR);
