<?php

chdir('/var/www/html/game');
require_once 'core/battle_engine.php';

$UnitParamLocal = array(
    202 => array(4000, 10, 5, 5000, 5000, 10),
    204 => array(4000, 10, 50, 50, 12500, 20),
    214 => array(9000000, 50000, 200000, 1000000, 100, 1),
    401 => array(2000, 20, 80, 0, 0, 0),
    406 => array(100000, 300, 3000, 0, 0, 0),
);
$RapidFireLocal = array();

function oracle_slot(array $units): array
{
    return array('weap' => 0, 'shld' => 0, 'armr' => 0, 'units' => $units);
}

function oracle_units(array $slot): array|object
{
    return count($slot['units']) === 0 ? (object)array() : $slot['units'];
}

function oracle_case(string $name, array $attackers, array $defenders): array
{
    global $exploded_counter, $already_exploded_counter;
    $exploded_counter = 0;
    $already_exploded_counter = 0;
    mt_srand(12345);
    $result = array('before' => array('attackers' => $attackers, 'defenders' => $defenders));
    DoBattle($result, 0, 6);
    return array('name' => $name, 'outcome' => $result['result'], 'rounds' => array_map(
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

echo json_encode(array('cases' => $cases), JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR);
