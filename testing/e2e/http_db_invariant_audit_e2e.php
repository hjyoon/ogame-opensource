<?php

error_reporting(E_ALL & ~E_DEPRECATED);
ini_set('display_errors', '1');

$_SERVER['REMOTE_ADDR'] = '127.0.0.1';
$_SERVER['HTTP_HOST'] = '127.0.0.1:8888';
$_SERVER['REQUEST_METHOD'] = 'CLI';
$_SERVER['HTTP_USER_AGENT'] = 'ogame-e2e';
$_SERVER['REQUEST_URI'] = '/testing/e2e/http_db_invariant_audit_e2e.php';
$_COOKIE['ogamelang'] = 'en';

chdir('/var/www/html/game');
require_once 'config.php';
require_once 'core/core.php';

InitDB();
$GlobalUni = LoadUniverse();
$GlobalUser = array('player_id' => 0);
$session = '';
ModsInit();

function e2e_sql_escape(string $value): string
{
    global $db_connect;
    return mysqli_real_escape_string($db_connect, $value);
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

function e2e_one_row(string $sql): ?array
{
    $res = dbquery($sql);
    $row = dbarray($res);
    return $row === false ? null : $row;
}

function e2e_rows(string $sql, int $limit = 10): array
{
    $res = dbquery($sql . " LIMIT {$limit}");
    $rows = array();
    while ($row = dbarray($res)) {
        $rows[] = $row;
    }
    return $rows;
}

function e2e_count(string $sql): int
{
    $row = e2e_one_row($sql);
    return $row === null ? 0 : (int)$row['cnt'];
}

function e2e_violation_check(string $message, string $countSql, string $sampleSql): array
{
    $count = e2e_count($countSql);
    return e2e_case($count === 0, $message, array(
        'violations' => $count,
        'samples' => $count === 0 ? array() : e2e_rows($sampleSql),
    ));
}

function e2e_backtick_cols(array $cols): array
{
    $unique = array_values(array_unique(array_map('intval', $cols)));
    return array_map(fn($col) => "`{$col}`", $unique);
}

function e2e_negative_conditions(array $cols): string
{
    $checks = array_map(fn($col) => "{$col} < 0", e2e_backtick_cols($cols));
    return implode(' OR ', $checks);
}

function e2e_type_list(array $types): string
{
    return implode(',', array_map(fn($type) => "'" . e2e_sql_escape($type) . "'", $types));
}

function e2e_baseline_id(string $envName): int
{
    $value = getenv($envName);
    return $value === false ? 0 : max(0, intval($value));
}

$cases = array();

try {
    global $db_prefix, $buildmap, $fleetmap, $defmap, $resmap, $transportableResources;

    $planetNonNegativeCols = array_merge(
        array(GID_RC_METAL, GID_RC_CRYSTAL, GID_RC_DEUTERIUM),
        $buildmap,
        $fleetmap,
        $defmap
    );
    $planetNegativeWhere = e2e_negative_conditions($planetNonNegativeCols) . " OR fields < 0 OR maxfields < 0";
    $userResearchNegativeWhere = e2e_negative_conditions($resmap);
    $fleetNegativeWhere = e2e_negative_conditions(array_merge($fleetmap, $transportableResources)) . " OR fuel < 0 OR ipm_amount < 0";

    $cases[] = e2e_finalize_case(array(
        'case' => 'numeric_state_has_no_negative_counts',
        'checks' => array(
            e2e_violation_check(
                'planet resources, buildings, fleets, defenses, and fields are non-negative',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}planets WHERE {$planetNegativeWhere}",
                "SELECT planet_id, owner_id, type, g, s, p, `" . GID_RC_METAL . "` AS metal, `" . GID_RC_CRYSTAL . "` AS crystal, `" . GID_RC_DEUTERIUM . "` AS deuterium, fields, maxfields FROM {$db_prefix}planets WHERE {$planetNegativeWhere}"
            ),
            e2e_violation_check(
                'user research levels are non-negative',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}users WHERE {$userResearchNegativeWhere}",
                "SELECT player_id, name FROM {$db_prefix}users WHERE {$userResearchNegativeWhere}"
            ),
            e2e_violation_check(
                'active fleet resources and ship counts are non-negative',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE {$fleetNegativeWhere}",
                "SELECT fleet_id, owner_id, mission, start_planet, target_planet FROM {$db_prefix}fleet WHERE {$fleetNegativeWhere}"
            ),
            e2e_violation_check(
                'queue and buildqueue levels are non-negative',
                "SELECT COUNT(*) AS cnt FROM (SELECT task_id AS id FROM {$db_prefix}queue WHERE level < 0 UNION ALL SELECT id FROM {$db_prefix}buildqueue WHERE level < 0) AS bad_levels",
                "SELECT 'queue' AS source, task_id AS id, owner_id, type, level FROM {$db_prefix}queue WHERE level < 0 UNION ALL SELECT 'buildqueue' AS source, id, owner_id, 'BuildQueue' AS type, level FROM {$db_prefix}buildqueue WHERE level < 0"
            ),
        ),
    ));

    $cases[] = e2e_finalize_case(array(
        'case' => 'core_foreign_keys_are_not_orphaned',
        'checks' => array(
            e2e_violation_check(
                'planets reference an existing owner user',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}planets p LEFT JOIN {$db_prefix}users u ON u.player_id=p.owner_id WHERE u.player_id IS NULL",
                "SELECT p.planet_id, p.owner_id, p.type, p.g, p.s, p.p FROM {$db_prefix}planets p LEFT JOIN {$db_prefix}users u ON u.player_id=p.owner_id WHERE u.player_id IS NULL"
            ),
            e2e_violation_check(
                'normal users reference an existing owned home planet',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}users u LEFT JOIN {$db_prefix}planets p ON p.planet_id=u.hplanetid WHERE u.player_id<>" . USER_SPACE . " AND (p.planet_id IS NULL OR p.owner_id<>u.player_id)",
                "SELECT u.player_id, u.name, u.hplanetid, p.owner_id AS planet_owner FROM {$db_prefix}users u LEFT JOIN {$db_prefix}planets p ON p.planet_id=u.hplanetid WHERE u.player_id<>" . USER_SPACE . " AND (p.planet_id IS NULL OR p.owner_id<>u.player_id)"
            ),
            e2e_violation_check(
                'normal users reference an existing owned active planet',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}users u LEFT JOIN {$db_prefix}planets p ON p.planet_id=u.aktplanet WHERE u.player_id<>" . USER_SPACE . " AND (p.planet_id IS NULL OR p.owner_id<>u.player_id)",
                "SELECT u.player_id, u.name, u.aktplanet, p.owner_id AS planet_owner FROM {$db_prefix}users u LEFT JOIN {$db_prefix}planets p ON p.planet_id=u.aktplanet WHERE u.player_id<>" . USER_SPACE . " AND (p.planet_id IS NULL OR p.owner_id<>u.player_id)"
            ),
            e2e_violation_check(
                'fleet rows reference existing owner, origin, and target records',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet f LEFT JOIN {$db_prefix}users u ON u.player_id=f.owner_id LEFT JOIN {$db_prefix}planets sp ON sp.planet_id=f.start_planet LEFT JOIN {$db_prefix}planets tp ON tp.planet_id=f.target_planet WHERE u.player_id IS NULL OR sp.planet_id IS NULL OR tp.planet_id IS NULL",
                "SELECT f.fleet_id, f.owner_id, f.start_planet, f.target_planet, u.player_id AS user_ref, sp.planet_id AS start_ref, tp.planet_id AS target_ref FROM {$db_prefix}fleet f LEFT JOIN {$db_prefix}users u ON u.player_id=f.owner_id LEFT JOIN {$db_prefix}planets sp ON sp.planet_id=f.start_planet LEFT JOIN {$db_prefix}planets tp ON tp.planet_id=f.target_planet WHERE u.player_id IS NULL OR sp.planet_id IS NULL OR tp.planet_id IS NULL"
            ),
            e2e_violation_check(
                'queue and buildqueue owner references exist',
                "SELECT COUNT(*) AS cnt FROM (SELECT q.task_id AS id FROM {$db_prefix}queue q LEFT JOIN {$db_prefix}users u ON u.player_id=q.owner_id WHERE u.player_id IS NULL UNION ALL SELECT b.id FROM {$db_prefix}buildqueue b LEFT JOIN {$db_prefix}users u ON u.player_id=b.owner_id WHERE u.player_id IS NULL) AS orphaned_queue_owners",
                "SELECT 'queue' AS source, q.task_id AS id, q.owner_id FROM {$db_prefix}queue q LEFT JOIN {$db_prefix}users u ON u.player_id=q.owner_id WHERE u.player_id IS NULL UNION ALL SELECT 'buildqueue' AS source, b.id, b.owner_id FROM {$db_prefix}buildqueue b LEFT JOIN {$db_prefix}users u ON u.player_id=b.owner_id WHERE u.player_id IS NULL"
            ),
            e2e_violation_check(
                'buildqueue rows reference existing planets',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}buildqueue b LEFT JOIN {$db_prefix}planets p ON p.planet_id=b.planet_id WHERE p.planet_id IS NULL",
                "SELECT b.id, b.owner_id, b.planet_id, b.tech_id FROM {$db_prefix}buildqueue b LEFT JOIN {$db_prefix}planets p ON p.planet_id=b.planet_id WHERE p.planet_id IS NULL"
            ),
        ),
    ));

    $fleetQueueType = QTYP_FLEET;
    $buildTypes = e2e_type_list(array(QTYP_BUILD, QTYP_DEMOLISH));
    $cases[] = e2e_finalize_case(array(
        'case' => 'queue_relationships_are_consistent',
        'checks' => array(
            e2e_violation_check(
                'each active fleet has exactly one fleet queue row',
                "SELECT COUNT(*) AS cnt FROM (SELECT f.fleet_id, COUNT(q.task_id) AS queue_rows FROM {$db_prefix}fleet f LEFT JOIN {$db_prefix}queue q ON q.type='" . e2e_sql_escape($fleetQueueType) . "' AND q.sub_id=f.fleet_id GROUP BY f.fleet_id HAVING queue_rows<>1) AS fleet_queue_counts",
                "SELECT f.fleet_id, f.owner_id, f.mission, COUNT(q.task_id) AS queue_rows FROM {$db_prefix}fleet f LEFT JOIN {$db_prefix}queue q ON q.type='" . e2e_sql_escape($fleetQueueType) . "' AND q.sub_id=f.fleet_id GROUP BY f.fleet_id, f.owner_id, f.mission HAVING queue_rows<>1"
            ),
            e2e_violation_check(
                'fleet queue rows point to existing fleet rows',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}queue q LEFT JOIN {$db_prefix}fleet f ON f.fleet_id=q.sub_id WHERE q.type='" . e2e_sql_escape($fleetQueueType) . "' AND f.fleet_id IS NULL",
                "SELECT q.task_id, q.owner_id, q.sub_id FROM {$db_prefix}queue q LEFT JOIN {$db_prefix}fleet f ON f.fleet_id=q.sub_id WHERE q.type='" . e2e_sql_escape($fleetQueueType) . "' AND f.fleet_id IS NULL"
            ),
            e2e_violation_check(
                'build and demolish queue rows point to existing buildqueue rows',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}queue q LEFT JOIN {$db_prefix}buildqueue b ON b.id=q.sub_id WHERE q.type IN ({$buildTypes}) AND b.id IS NULL",
                "SELECT q.task_id, q.owner_id, q.type, q.sub_id FROM {$db_prefix}queue q LEFT JOIN {$db_prefix}buildqueue b ON b.id=q.sub_id WHERE q.type IN ({$buildTypes}) AND b.id IS NULL"
            ),
            e2e_violation_check(
                'each buildqueue row has exactly one active build/demolish queue row',
                "SELECT COUNT(*) AS cnt FROM (SELECT b.id, COUNT(q.task_id) AS queue_rows FROM {$db_prefix}buildqueue b LEFT JOIN {$db_prefix}queue q ON q.type IN ({$buildTypes}) AND q.sub_id=b.id GROUP BY b.id HAVING queue_rows<>1) AS buildqueue_queue_counts",
                "SELECT b.id, b.owner_id, b.planet_id, b.tech_id, COUNT(q.task_id) AS queue_rows FROM {$db_prefix}buildqueue b LEFT JOIN {$db_prefix}queue q ON q.type IN ({$buildTypes}) AND q.sub_id=b.id GROUP BY b.id, b.owner_id, b.planet_id, b.tech_id HAVING queue_rows<>1"
            ),
            e2e_violation_check(
                'no buildqueue row is driven by multiple active build/demolish queues',
                "SELECT COUNT(*) AS cnt FROM (SELECT sub_id, COUNT(*) AS queue_rows FROM {$db_prefix}queue WHERE type IN ({$buildTypes}) GROUP BY sub_id HAVING queue_rows>1) AS duplicate_build_queue_refs",
                "SELECT sub_id, COUNT(*) AS queue_rows FROM {$db_prefix}queue WHERE type IN ({$buildTypes}) GROUP BY sub_id HAVING queue_rows>1"
            ),
            e2e_violation_check(
                'buildqueue rows are owned by the same player as their planets',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}buildqueue b JOIN {$db_prefix}planets p ON p.planet_id=b.planet_id WHERE p.owner_id<>b.owner_id",
                "SELECT b.id, b.owner_id, b.planet_id, p.owner_id AS planet_owner, b.tech_id FROM {$db_prefix}buildqueue b JOIN {$db_prefix}planets p ON p.planet_id=b.planet_id WHERE p.owner_id<>b.owner_id"
            ),
            e2e_violation_check(
                'build and demolish queue owners match their buildqueue owners',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}queue q JOIN {$db_prefix}buildqueue b ON b.id=q.sub_id WHERE q.type IN ({$buildTypes}) AND q.owner_id<>b.owner_id",
                "SELECT q.task_id, q.owner_id, b.owner_id AS buildqueue_owner, q.type, q.sub_id FROM {$db_prefix}queue q JOIN {$db_prefix}buildqueue b ON b.id=q.sub_id WHERE q.type IN ({$buildTypes}) AND q.owner_id<>b.owner_id"
            ),
            e2e_violation_check(
                'each planet has at most one active research queue',
                "SELECT COUNT(*) AS cnt FROM (SELECT owner_id, COUNT(*) AS queue_rows FROM {$db_prefix}queue WHERE type='" . e2e_sql_escape(QTYP_RESEARCH) . "' GROUP BY owner_id HAVING queue_rows>1) AS duplicate_research",
                "SELECT owner_id, COUNT(*) AS queue_rows FROM {$db_prefix}queue WHERE type='" . e2e_sql_escape(QTYP_RESEARCH) . "' GROUP BY owner_id HAVING queue_rows>1"
            ),
            e2e_violation_check(
                'research and shipyard queue rows reference existing planets',
                "SELECT COUNT(*) AS cnt FROM (SELECT q.task_id FROM {$db_prefix}queue q LEFT JOIN {$db_prefix}planets p ON p.planet_id=q.sub_id WHERE q.type IN ('" . e2e_sql_escape(QTYP_RESEARCH) . "','" . e2e_sql_escape(QTYP_SHIPYARD) . "') AND p.planet_id IS NULL) AS bad_planet_queues",
                "SELECT q.task_id, q.owner_id, q.type, q.sub_id FROM {$db_prefix}queue q LEFT JOIN {$db_prefix}planets p ON p.planet_id=q.sub_id WHERE q.type IN ('" . e2e_sql_escape(QTYP_RESEARCH) . "','" . e2e_sql_escape(QTYP_SHIPYARD) . "') AND p.planet_id IS NULL"
            ),
            e2e_violation_check(
                'research and shipyard queue owners match their planet owners',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}queue q JOIN {$db_prefix}planets p ON p.planet_id=q.sub_id WHERE q.type IN ('" . e2e_sql_escape(QTYP_RESEARCH) . "','" . e2e_sql_escape(QTYP_SHIPYARD) . "') AND p.owner_id<>q.owner_id",
                "SELECT q.task_id, q.owner_id, p.owner_id AS planet_owner, q.type, q.sub_id FROM {$db_prefix}queue q JOIN {$db_prefix}planets p ON p.planet_id=q.sub_id WHERE q.type IN ('" . e2e_sql_escape(QTYP_RESEARCH) . "','" . e2e_sql_escape(QTYP_SHIPYARD) . "') AND p.owner_id<>q.owner_id"
            ),
            e2e_violation_check(
                'fleet queue owners match their fleet owners',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}queue q JOIN {$db_prefix}fleet f ON f.fleet_id=q.sub_id WHERE q.type='" . e2e_sql_escape($fleetQueueType) . "' AND f.owner_id<>q.owner_id",
                "SELECT q.task_id, q.owner_id, f.owner_id AS fleet_owner, q.sub_id FROM {$db_prefix}queue q JOIN {$db_prefix}fleet f ON f.fleet_id=q.sub_id WHERE q.type='" . e2e_sql_escape($fleetQueueType) . "' AND f.owner_id<>q.owner_id"
            ),
        ),
    ));

    $planetTypes = implode(',', array(PTYP_PLANET, PTYP_DEST_PLANET, PTYP_ABANDONED));
    $moonTypes = implode(',', array(PTYP_MOON, PTYP_DEST_MOON));
    $cases[] = e2e_finalize_case(array(
        'case' => 'coordinate_and_social_references_are_consistent',
        'checks' => array(
            e2e_violation_check(
                'planet coordinate slots are unique across planet/destroyed/abandoned records',
                "SELECT COUNT(*) AS cnt FROM (SELECT g, s, p, COUNT(*) AS slot_count FROM {$db_prefix}planets WHERE type IN ({$planetTypes}) GROUP BY g, s, p HAVING slot_count>1) AS duplicate_planet_slots",
                "SELECT g, s, p, COUNT(*) AS slot_count FROM {$db_prefix}planets WHERE type IN ({$planetTypes}) GROUP BY g, s, p HAVING slot_count>1"
            ),
            e2e_violation_check(
                'moon coordinate slots are unique across moon/destroyed-moon records',
                "SELECT COUNT(*) AS cnt FROM (SELECT g, s, p, COUNT(*) AS slot_count FROM {$db_prefix}planets WHERE type IN ({$moonTypes}) GROUP BY g, s, p HAVING slot_count>1) AS duplicate_moon_slots",
                "SELECT g, s, p, COUNT(*) AS slot_count FROM {$db_prefix}planets WHERE type IN ({$moonTypes}) GROUP BY g, s, p HAVING slot_count>1"
            ),
            e2e_violation_check(
                'debris coordinate slots are unique',
                "SELECT COUNT(*) AS cnt FROM (SELECT g, s, p, COUNT(*) AS slot_count FROM {$db_prefix}planets WHERE type=" . PTYP_DF . " GROUP BY g, s, p HAVING slot_count>1) AS duplicate_debris_slots",
                "SELECT g, s, p, COUNT(*) AS slot_count FROM {$db_prefix}planets WHERE type=" . PTYP_DF . " GROUP BY g, s, p HAVING slot_count>1"
            ),
            e2e_violation_check(
                'alliance membership and founder references exist',
                "SELECT COUNT(*) AS cnt FROM (SELECT u.player_id AS id FROM {$db_prefix}users u LEFT JOIN {$db_prefix}ally a ON a.ally_id=u.ally_id WHERE u.ally_id>0 AND a.ally_id IS NULL UNION ALL SELECT a.ally_id FROM {$db_prefix}ally a LEFT JOIN {$db_prefix}users u ON u.player_id=a.owner_id WHERE u.player_id IS NULL) AS bad_alliance_refs",
                "SELECT 'member' AS source, u.player_id AS id, u.ally_id AS ref_id FROM {$db_prefix}users u LEFT JOIN {$db_prefix}ally a ON a.ally_id=u.ally_id WHERE u.ally_id>0 AND a.ally_id IS NULL UNION ALL SELECT 'founder' AS source, a.ally_id AS id, a.owner_id AS ref_id FROM {$db_prefix}ally a LEFT JOIN {$db_prefix}users u ON u.player_id=a.owner_id WHERE u.player_id IS NULL"
            ),
            e2e_violation_check(
                'buddy relationship endpoints exist',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}buddy b LEFT JOIN {$db_prefix}users rf ON rf.player_id=b.request_from LEFT JOIN {$db_prefix}users rt ON rt.player_id=b.request_to WHERE rf.player_id IS NULL OR rt.player_id IS NULL",
                "SELECT b.buddy_id, b.request_from, b.request_to FROM {$db_prefix}buddy b LEFT JOIN {$db_prefix}users rf ON rf.player_id=b.request_from LEFT JOIN {$db_prefix}users rt ON rt.player_id=b.request_to WHERE rf.player_id IS NULL OR rt.player_id IS NULL"
            ),
        ),
    ));

    $messageBaseline = e2e_baseline_id('OGAME_E2E_AUDIT_BASE_MESSAGE_ID');
    $noteBaseline = e2e_baseline_id('OGAME_E2E_AUDIT_BASE_NOTE_ID');
    $reportBaseline = e2e_baseline_id('OGAME_E2E_AUDIT_BASE_REPORT_ID');
    $templateBaseline = e2e_baseline_id('OGAME_E2E_AUDIT_BASE_TEMPLATE_ID');
    $botvarBaseline = e2e_baseline_id('OGAME_E2E_AUDIT_BASE_BOTVAR_ID');
    $allyappBaseline = e2e_baseline_id('OGAME_E2E_AUDIT_BASE_ALLYAPP_ID');
    $unionBaseline = e2e_baseline_id('OGAME_E2E_AUDIT_BASE_UNION_ID');
    $battleBaseline = e2e_baseline_id('OGAME_E2E_AUDIT_BASE_BATTLE_ID');

    $cases[] = e2e_finalize_case(array(
        'case' => 'message_and_auxiliary_references_are_consistent',
        'baselines' => array(
            'message_id' => $messageBaseline,
            'note_id' => $noteBaseline,
            'report_id' => $reportBaseline,
            'template_id' => $templateBaseline,
            'botvar_id' => $botvarBaseline,
            'allyapp_id' => $allyappBaseline,
            'union_id' => $unionBaseline,
            'battle_id' => $battleBaseline,
        ),
        'checks' => array(
            e2e_violation_check(
                'messages reference existing owner users',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}messages m LEFT JOIN {$db_prefix}users u ON u.player_id=m.owner_id WHERE m.msg_id>{$messageBaseline} AND u.player_id IS NULL",
                "SELECT m.msg_id, m.owner_id, m.pm, m.subj FROM {$db_prefix}messages m LEFT JOIN {$db_prefix}users u ON u.player_id=m.owner_id WHERE m.msg_id>{$messageBaseline} AND u.player_id IS NULL"
            ),
            e2e_violation_check(
                'message planet references point to existing planets when present',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}messages m LEFT JOIN {$db_prefix}planets p ON p.planet_id=m.planet_id WHERE m.msg_id>{$messageBaseline} AND m.planet_id>0 AND p.planet_id IS NULL",
                "SELECT m.msg_id, m.owner_id, m.pm, m.planet_id FROM {$db_prefix}messages m LEFT JOIN {$db_prefix}planets p ON p.planet_id=m.planet_id WHERE m.msg_id>{$messageBaseline} AND m.planet_id>0 AND p.planet_id IS NULL"
            ),
            e2e_violation_check(
                'notes reference existing owner users',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}notes n LEFT JOIN {$db_prefix}users u ON u.player_id=n.owner_id WHERE n.note_id>{$noteBaseline} AND u.player_id IS NULL",
                "SELECT n.note_id, n.owner_id, n.subj FROM {$db_prefix}notes n LEFT JOIN {$db_prefix}users u ON u.player_id=n.owner_id WHERE n.note_id>{$noteBaseline} AND u.player_id IS NULL"
            ),
            e2e_violation_check(
                'reported-message audit rows reference existing owner users',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}reports r LEFT JOIN {$db_prefix}users u ON u.player_id=r.owner_id WHERE r.id>{$reportBaseline} AND u.player_id IS NULL",
                "SELECT r.id, r.owner_id, r.msg_id, r.subj FROM {$db_prefix}reports r LEFT JOIN {$db_prefix}users u ON u.player_id=r.owner_id WHERE r.id>{$reportBaseline} AND u.player_id IS NULL"
            ),
            e2e_violation_check(
                'fleet templates reference existing owner users',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}template t LEFT JOIN {$db_prefix}users u ON u.player_id=t.owner_id WHERE t.id>{$templateBaseline} AND u.player_id IS NULL",
                "SELECT t.id, t.owner_id, t.name FROM {$db_prefix}template t LEFT JOIN {$db_prefix}users u ON u.player_id=t.owner_id WHERE t.id>{$templateBaseline} AND u.player_id IS NULL"
            ),
            e2e_violation_check(
                'bot variables reference existing owner users',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}botvars b LEFT JOIN {$db_prefix}users u ON u.player_id=b.owner_id WHERE b.id>{$botvarBaseline} AND u.player_id IS NULL",
                "SELECT b.id, b.owner_id, b.var FROM {$db_prefix}botvars b LEFT JOIN {$db_prefix}users u ON u.player_id=b.owner_id WHERE b.id>{$botvarBaseline} AND u.player_id IS NULL"
            ),
            e2e_violation_check(
                'alliance applications reference existing alliances and users',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}allyapps aa LEFT JOIN {$db_prefix}ally a ON a.ally_id=aa.ally_id LEFT JOIN {$db_prefix}users u ON u.player_id=aa.player_id WHERE aa.app_id>{$allyappBaseline} AND (a.ally_id IS NULL OR u.player_id IS NULL)",
                "SELECT aa.app_id, aa.ally_id, aa.player_id FROM {$db_prefix}allyapps aa LEFT JOIN {$db_prefix}ally a ON a.ally_id=aa.ally_id LEFT JOIN {$db_prefix}users u ON u.player_id=aa.player_id WHERE aa.app_id>{$allyappBaseline} AND (a.ally_id IS NULL OR u.player_id IS NULL)"
            ),
            e2e_violation_check(
                'alliance ranks reference existing alliances',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}allyranks ar LEFT JOIN {$db_prefix}ally a ON a.ally_id=ar.ally_id WHERE a.ally_id IS NULL",
                "SELECT ar.rank_id, ar.ally_id, ar.name FROM {$db_prefix}allyranks ar LEFT JOIN {$db_prefix}ally a ON a.ally_id=ar.ally_id WHERE a.ally_id IS NULL"
            ),
            e2e_violation_check(
                'ACS unions reference existing owner fleet and target player rows',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}union u LEFT JOIN {$db_prefix}fleet f ON f.fleet_id=u.fleet_id LEFT JOIN {$db_prefix}users tu ON tu.player_id=u.target_player WHERE u.union_id>{$unionBaseline} AND (f.fleet_id IS NULL OR tu.player_id IS NULL)",
                "SELECT u.union_id, u.fleet_id, u.target_player, u.players FROM {$db_prefix}union u LEFT JOIN {$db_prefix}fleet f ON f.fleet_id=u.fleet_id LEFT JOIN {$db_prefix}users tu ON tu.player_id=u.target_player WHERE u.union_id>{$unionBaseline} AND (f.fleet_id IS NULL OR tu.player_id IS NULL)"
            ),
            e2e_violation_check(
                'completed battledata rows have renderable title and report text',
                "SELECT COUNT(*) AS cnt FROM {$db_prefix}battledata WHERE battle_id>{$battleBaseline} AND (TRIM(COALESCE(title, ''))='' OR TRIM(COALESCE(report, ''))='')",
                "SELECT battle_id, date, LENGTH(title) AS title_len, LENGTH(report) AS report_len FROM {$db_prefix}battledata WHERE battle_id>{$battleBaseline} AND (TRIM(COALESCE(title, ''))='' OR TRIM(COALESCE(report, ''))='')"
            ),
        ),
    ));

    $attackerId = intval(getenv('OGAME_E2E_ATTACKER_ID') ?: 0);
    $defenderId = intval(getenv('OGAME_E2E_DEFENDER_ID') ?: 0);
    $fixtureChecks = array();
    if ($attackerId > 0 && $defenderId > 0) {
        $fixtureUserList = $attackerId . ',' . $defenderId;
        $volatileTypes = e2e_type_list(array(QTYP_BUILD, QTYP_DEMOLISH, QTYP_RESEARCH, QTYP_SHIPYARD, QTYP_FLEET));
        $fixtureChecks[] = e2e_violation_check(
            'fixture users have no active build/research/shipyard/fleet queues after case cleanup',
            "SELECT COUNT(*) AS cnt FROM {$db_prefix}queue WHERE owner_id IN ({$fixtureUserList}) AND type IN ({$volatileTypes})",
            "SELECT task_id, owner_id, type, sub_id, obj_id, level FROM {$db_prefix}queue WHERE owner_id IN ({$fixtureUserList}) AND type IN ({$volatileTypes})"
        );
        $fixtureChecks[] = e2e_violation_check(
            'fixture users have no active fleet rows after case cleanup',
            "SELECT COUNT(*) AS cnt FROM {$db_prefix}fleet WHERE owner_id IN ({$fixtureUserList})",
            "SELECT fleet_id, owner_id, mission, start_planet, target_planet FROM {$db_prefix}fleet WHERE owner_id IN ({$fixtureUserList})"
        );
    }
    $fixtureChecks[] = e2e_violation_check(
        'temporary E2E users from self-cleaning cases do not remain after case cleanup',
        "SELECT COUNT(*) AS cnt FROM {$db_prefix}users WHERE name REGEXP '^(e2eh|e2epw_|e2emq_|e2ebulk_|e2erg_|e2ebad_|e2emiss_|e2eagb|e2edel_)'",
        "SELECT player_id, name, email, hplanetid FROM {$db_prefix}users WHERE name REGEXP '^(e2eh|e2epw_|e2emq_|e2ebulk_|e2erg_|e2ebad_|e2emiss_|e2eagb|e2edel_)'"
    );
    $fleetLocks = array_map('basename', glob('temp/fleetlock_*') ?: array());
    $fixtureChecks[] = e2e_case(count($fleetLocks) === 0, 'no temporary fleet lock files remain after E2E cases', array('fleet_locks' => $fleetLocks));
    $battleScratch = array_merge(
        array_map('basename', glob('battledata/battle_*.txt') ?: array()),
        array_map('basename', glob('battleresult/battle_*.txt') ?: array())
    );
    $fixtureChecks[] = e2e_case(count($battleScratch) === 0, 'no temporary battle engine scratch files remain after E2E cases', array('battle_scratch' => $battleScratch));
    $cases[] = e2e_finalize_case(array(
        'case' => 'e2e_runtime_artifacts_are_clean',
        'checks' => $fixtureChecks,
    ));
} catch (Throwable $e) {
    $cases[] = array(
        'case' => 'db_invariant_audit_exception',
        'checks' => array(e2e_case(false, $e->getMessage(), array('exception' => get_class($e)))),
        'pass' => false,
    );
}

echo json_encode(array(
    'case_group' => 'http_db_invariant_audit',
    'cases' => $cases,
    'all_pass' => array_reduce($cases, fn($ok, $case) => $ok && $case['pass'], true),
), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
