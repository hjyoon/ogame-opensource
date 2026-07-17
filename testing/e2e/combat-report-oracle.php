<?php

date_default_timezone_set('UTC');
$from_reg = false;

chdir('/var/www/html/game');
require_once 'core/defs.php';
require_once 'core/utils.php';
require_once 'core/techs.php';
require_once 'core/loca.php';
require_once 'core/battle_report.php';

function report_slot(string $name, int $g, int $s, int $p, int $weap, int $shld, int $armr, array $units): array
{
    return array(
        'name' => $name,
        'g' => $g,
        's' => $s,
        'p' => $p,
        'pf' => 2,
        'weap' => $weap,
        'shld' => $shld,
        'armr' => $armr,
        'units' => $units,
    );
}

function report_round(array $attackerUnits): array
{
    return array(
        'ashoot' => 1,
        'apower' => 10,
        'dabsorb' => 10,
        'dshoot' => 5,
        'dpower' => 400,
        'aabsorb' => 42,
        'attackers' => array(report_slot('ghost', 1, 1, 4, 0, 0, 0, $attackerUnits)),
        'defenders' => array(report_slot('Piraten', 1, 1, 16, 0, 0, 0, array(GID_F_LF => 5))),
    );
}

function report_case(string $name, array $rounds, string $result): array
{
    $battle = array(
        'before' => array(
            'attackers' => array(report_slot('ghost', 1, 1, 4, 9, 7, 8, array(GID_F_LC => 1))),
            'defenders' => array(report_slot('Piraten', 1, 1, 16, 6, 4, 5, array(GID_F_LF => 5))),
        ),
        'rounds' => $rounds,
        'result' => $result,
    );
    return array(
        'name' => $name,
        'report' => BattleReport($battle, 1700000000, null, null, 0, false, null, null, 'en'),
    );
}

$sixRounds = array();
for ($round = 0; $round < BATTLE_MAX_ROUND; $round++) {
    $sixRounds[] = report_round(array(GID_F_LC => 1));
}

echo json_encode(array(
    'cases' => array(
        report_case(
            'pirate_two_round_defender_win',
            array(report_round(array(GID_F_LC => 1)), report_round(array())),
            'dwon'
        ),
        report_case('pirate_six_round_draw', $sixRounds, 'draw'),
    ),
), JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR);
