<?php

// Working with MySQL database.

$query_counter = 0;
$query_log = "";
$db_connect = 0;

function dbconnect (string $db_host, string $db_user, string $db_pass, string $db_name) : void
{
    global  $query_counter, $query_log, $db_connect;
    mysqli_report(MYSQLI_REPORT_OFF);
    $db_connect = @mysqli_connect($db_host, $db_user, $db_pass);
    $db_select = @mysqli_select_db($db_connect, $db_name);
    if (!$db_connect) {
        die("<div style='font-family:Verdana;font-size:11px;text-align:center;'><b>Unable to establish connection to MySQL</b></div>");
    } elseif (!$db_select) {
        die("<div style='font-family:Verdana;font-size:11px;text-align:center;'><b>Unable to select MySQL database</b></div>");
    }

    $query_counter = 0;
    $query_log = "";
}

function dbquery (string $query, bool $mute=false) : mixed
{
    global  $query_counter, $query_log, $db_connect;
    $query_counter ++;
    $query_log .= $query . "<br>\n";
    $result = @mysqli_query($db_connect, $query);
    if (!$result && $mute==false) {
        error_log("SQL error: ".mysqli_error ($db_connect)." Query: ".$query);
        //Debug ( mysqli_error($db_connect) . "<br>" . $query . "<br>" . BackTrace () ) ;
        return false;
    }
    else return $result;
}

function dbrows (mixed $result) : int
{
    $rows = @mysqli_num_rows($result);
    return $rows;
}

function dbarray (mixed $result) : mixed
{
    if ($result === false || $result === null || $result === true) {
        return false;
    }
    $arr = @mysqli_fetch_assoc($result);
    return $arr === null ? false : $arr;
}

function dbfree (mixed $result) : void {
    @mysqli_free_result ($result);
}

// Connect to the database
function InitDB (bool $apply_migrations = true) : void
{
    global $db_host, $db_user, $db_pass, $db_name;
    dbconnect ($db_host, $db_user, $db_pass, $db_name);
    dbquery("SET NAMES 'utf8';");
    dbquery("SET CHARACTER SET 'utf8';");
    dbquery("SET SESSION collation_connection = 'utf8_general_ci';");
    if ($apply_migrations) {
        ApplyCoreSchemaMigrations();
    }
}

function DbIdent (string $name) : string
{
    return "`".str_replace("`", "``", $name)."`";
}

function CoreIndexDefinitions () : array
{
    return array(
        'users' => array(
            'idx_users_name' => array('name'),
            'idx_users_session' => array('session'),
            'idx_users_email' => array('email'),
            'idx_users_pemail' => array('pemail'),
            'idx_users_feedid' => array('feedid'),
            'idx_users_disable_until' => array('disable', 'disable_until', 'admin'),
            'idx_users_lastclick_cleanup' => array('lastclick', 'admin', 'dm'),
        ),
        'planets' => array(
            'idx_planets_coords_type' => array('g', 's', 'p', 'type'),
            'idx_planets_owner_type' => array('owner_id', 'type'),
            'idx_planets_remove_type' => array('remove', 'type'),
        ),
        'queue' => array(
            'idx_queue_due' => array('end', 'freeze', 'prio'),
            'idx_queue_owner_type_end' => array('owner_id', 'type', 'end'),
            'idx_queue_type_sub' => array('type', 'sub_id'),
        ),
        'buildqueue' => array(
            'idx_buildqueue_planet_list' => array('planet_id', 'list_id'),
            'idx_buildqueue_owner' => array('owner_id'),
        ),
        'fleet' => array(
            'idx_fleet_owner_mission' => array('owner_id', 'mission'),
            'idx_fleet_target_mission' => array('target_planet', 'mission'),
            'idx_fleet_start_planet' => array('start_planet'),
        ),
        'messages' => array(
            'idx_messages_owner_date' => array('owner_id', 'date'),
            'idx_messages_owner_pm_date' => array('owner_id', 'pm', 'date'),
        ),
        'reports' => array(
            'idx_reports_owner_date' => array('owner_id', 'date'),
            'idx_reports_msg' => array('msg_id'),
        ),
        'notes' => array(
            'idx_notes_owner_date' => array('owner_id', 'date'),
        ),
        'buddy' => array(
            'idx_buddy_from_to' => array('request_from', 'request_to'),
            'idx_buddy_to_accepted' => array('request_to', 'accepted'),
        ),
        'allyapps' => array(
            'idx_allyapps_ally_player' => array('ally_id', 'player_id'),
        ),
        'template' => array(
            'idx_template_owner_date' => array('owner_id', 'date'),
        ),
        'userlogs' => array(
            'idx_userlogs_owner_date' => array('owner_id', 'date'),
        ),
        'fleetlogs' => array(
            'idx_fleetlogs_owner_start' => array('owner_id', 'start'),
            'idx_fleetlogs_target_start' => array('target_id', 'start'),
        ),
    );
}

function CoreTableExists (string $table) : bool
{
    global $db_connect, $db_name, $db_prefix;
    $schema = mysqli_real_escape_string($db_connect, $db_name);
    $tableName = mysqli_real_escape_string($db_connect, $db_prefix.$table);
    $result = dbquery("SELECT 1 FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA='{$schema}' AND TABLE_NAME='{$tableName}' LIMIT 1", true);
    return $result !== false && dbarray($result) !== false;
}

function CoreIndexExists (string $table, string $indexName) : bool
{
    global $db_connect, $db_name, $db_prefix;
    $schema = mysqli_real_escape_string($db_connect, $db_name);
    $tableName = mysqli_real_escape_string($db_connect, $db_prefix.$table);
    $index = mysqli_real_escape_string($db_connect, $indexName);
    $result = dbquery("SELECT 1 FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA='{$schema}' AND TABLE_NAME='{$tableName}' AND INDEX_NAME='{$index}' LIMIT 1", true);
    return $result !== false && dbarray($result) !== false;
}

function EnsureCoreIndex (string $table, string $indexName, array $columns) : bool
{
    global $db_prefix;
    if (!CoreTableExists($table) || CoreIndexExists($table, $indexName)) {
        return true;
    }

    $columnSql = implode(',', array_map(fn($column) => DbIdent((string)$column), $columns));
    $query = "ALTER TABLE ".DbIdent($db_prefix.$table)." ADD INDEX ".DbIdent($indexName)." (".$columnSql.")";
    return dbquery($query, true) !== false;
}

function EnsureCoreIndexes () : bool
{
    foreach (CoreIndexDefinitions() as $table => $indexes) {
        foreach ($indexes as $indexName => $columns) {
            if (!EnsureCoreIndex($table, $indexName, $columns)) {
                return false;
            }
        }
    }
    return true;
}

function ApplyCoreSchemaMigrations () : void
{
    static $applied = false;
    global $db_prefix;
    if ($applied) return;
    $applied = true;

    $table = $db_prefix."schema_migrations";
    $created = dbquery(
        "CREATE TABLE IF NOT EXISTS ".DbIdent($table)." (".
        "id VARCHAR(80) PRIMARY KEY, applied_at INT UNSIGNED NOT NULL".
        ") CHARACTER SET utf8 COLLATE utf8_general_ci",
        true
    );
    if ($created === false) return;

    $migrationId = "20260617_core_indexes";
    $safeId = mysqli_real_escape_string($GLOBALS['db_connect'], $migrationId);
    $result = dbquery("SELECT id FROM ".DbIdent($table)." WHERE id='{$safeId}' LIMIT 1", true);
    if ($result !== false && dbarray($result) !== false) return;

    if (EnsureCoreIndexes()) {
        dbquery("INSERT INTO ".DbIdent($table)." (id, applied_at) VALUES ('{$safeId}', ".time().")", true);
    }
}

// Add a row to the table.
// This method now takes into account that the table may have additional columns added by the mod that do not need to be touched.
function AddDBRow ( array $row, string $tabname ) : int
{
    global $db_connect, $db_prefix;
    ModsExecRefStr ( 'add_db_row', $row, $tabname );
    $values = "(";
    $columns = "(";
    $first = true;
    foreach ($row as $col=>$value)
    {
        if (!$first) {
            $values .= ", ";
            $columns .= ", ";
        }
        $values .= "'".mysqli_real_escape_string($db_connect, (string)$value)."'";
        $columns .= "`".$col."`";
        $first = false;
    }
    $values .= ");";
    $columns .= ")";
    $query = "INSERT INTO ".$db_prefix."$tabname $columns VALUES ".$values;
    dbquery( $query);
    return mysqli_insert_id ($db_connect);
}

// ---
// Working with the master database, where information common to all universes (e.g. coupons) is stored.
// The master database can be accessed from any universe

// Link to connect to the master database
$MDB_link = 0;

function MDBConnect () : bool
{
    global $MDB_link, $mdb_host, $mdb_user, $mdb_pass, $mdb_name, $mdb_enable;
    if (!$mdb_enable) return false;
    mysqli_report(MYSQLI_REPORT_OFF);
    $MDB_link = @mysqli_connect ($mdb_host, $mdb_user, $mdb_pass );
    if (!$MDB_link) return false;
    if ( ! @mysqli_select_db ($MDB_link, $mdb_name) ) return false;

    MDBQuery ("SET NAMES 'utf8';");
    MDBQuery ("SET CHARACTER SET 'utf8';");
    MDBQuery ("SET SESSION collation_connection = 'utf8_general_ci';");

    return true;
}

function MDBQuery (string $query) : mixed
{
    global $MDB_link;
    $result = @mysqli_query ($MDB_link, $query);
    if (!$result) return null;
    else return $result;
}

function MDBRows (mixed $result) : int
{
    $rows = @mysqli_num_rows($result);
    return $rows;
}

function MDBArray (mixed $result) : mixed
{
    $arr = @mysqli_fetch_assoc($result);
    if (!$arr) return null;
    else return $arr;
}


// Table locking is critical in a multi-user environment. It is protection against simultaneous work with the database from several users.
// Think of it as analogous to multitasking lock (mutex).

function LockTables () : void
{
    global $db_prefix;
    $tabs = array ('users','planets','ally','allyranks','allyapps','buddy','messages','notes','errors','debug','reports','browse','queue','buildqueue','fleet','union','battledata','fleetlogs','iplogs','pranger','exptab','coltab','template','botvars','userlogs','botstrat');
    ModsExecRef ('lock_tables', $tabs);
    $query = "LOCK TABLES ".$db_prefix."uni WRITE";
    foreach ( $tabs as $i=>$name ) 
    {
        $query .= ", ".$db_prefix.$name." WRITE";
    }
    dbquery ($query);
}

function UnlockTables () : void
{
    dbquery ( "UNLOCK TABLES" );
}

function SerializeTable (string $name) : array
{
    global $db_name;
    global $db_prefix;

    $tab = array();

    // Get table autoincrement value (or null, if the table has no autoincrement)
    $query = "SELECT `AUTO_INCREMENT` FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = '".$db_name."' AND TABLE_NAME = '".$db_prefix.$name."';";
    $res = dbquery ($query);
    $arr = dbarray($res);
    $auto_incr = empty($arr['AUTO_INCREMENT']) ? null : intval($arr['AUTO_INCREMENT']);
    $tab['auto_increment'] = $auto_incr;

    // Get the list of table columns
    $query = "SHOW COLUMNS FROM $db_prefix$name;";
    $res = dbquery($query);
    $rows = dbrows ($res);
    $tab['cols'] = array();
    $i = 0;
    while ($rows--) {
        $arr = dbarray($res);
        $tab['cols'][$i++] = $arr['Field'];
    }

    // Get table rows
    $tab['values'] = array();
    $query = "SELECT * FROM ".$db_prefix.$name;
    $res = dbquery ($query);
    $rows = dbrows($res);
    $i = 0;
    while ($rows--) {
        $arr = dbarray($res);
        $tab['values'][$i] = array();
        $n = 0;
        foreach ($arr as $j=>$value) {
            $tab['values'][$i][$n++] = $value;
        }
        $i++;
    }

    return $tab;
}

function SerializeDB () : string
{
    include "install_tabs.php";
    ModsExecRef ('install_tabs_included', $tabs);

    $db_tabs = array();

    foreach ($tabs as $i=>$cols) {
        $db_tabs[$i] = SerializeTable ($i);
    }

    return json_encode ($db_tabs, JSON_UNESCAPED_UNICODE|JSON_PRETTY_PRINT);
}

function DeserExecQuery (string $query) : void
{
    //echo $query . "\n";
    dbquery ($query);
}

function DeserializeTable (string $name, array $tab) : void
{
    global $db_prefix;
    global $db_connect;

    // Clean up the old rows
    $query = "TRUNCATE TABLE `".$db_prefix.$name."`;";
    DeserExecQuery ($query);

    if (count($tab['values']) != 0) {

        $query = "INSERT INTO `".$db_prefix.$name."` (";
        $first = true;
        foreach ($tab['cols'] as $col) {
            if (!$first) $query .= ", ";
            $query .= "`".$col."`";
            if ($first) $first = false;
        }
        $query .= ") VALUES\n";

        $first = true;
        foreach ($tab['values'] as $row) {
            if (!$first) $query .= ",\n";
            $query .= "(";
            $first_val = true;
            foreach ($row as $value) {
                if (!$first_val) $query .= ", ";
                $query .= "\"".mysqli_escape_string($db_connect, $value)."\"";
                if ($first_val) $first_val = false;
            }
            $query .= ")";
            if ($first) $first = false;
        }
        $query .= ";";
        DeserExecQuery ($query);
    }

    // Actualize autoincrement. The column for autoincrement in the game tables is always the first one.
    if ($tab['auto_increment'] != null) {
        $query = "ALTER TABLE `".$db_prefix.$name."` MODIFY `".$tab['cols'][0]."` INT AUTO_INCREMENT, AUTO_INCREMENT=".$tab['auto_increment'].";";
        DeserExecQuery ($query);
    }
}

function DeserializeDB (string $text) : void
{
    $tabs = json_decode ($text, true);

    foreach ($tabs as $i=>$tab) {
        DeserializeTable ($i, $tab);
    }
}

function IsSerializedDBBackup (string $text) : bool
{
    $db_tabs = json_decode ($text, true);
    if (!is_array($db_tabs) || json_last_error() !== JSON_ERROR_NONE) return false;

    include "install_tabs.php";
    ModsExecRef ('install_tabs_included', $tabs);

    foreach ($tabs as $name=>$cols) {
        if (!isset($db_tabs[$name]) || !is_array($db_tabs[$name])) return false;
        if (!isset($db_tabs[$name]['cols']) || !is_array($db_tabs[$name]['cols'])) return false;
        if (!isset($db_tabs[$name]['values']) || !is_array($db_tabs[$name]['values'])) return false;
        if (!array_key_exists('auto_increment', $db_tabs[$name])) return false;

        $expected_cols = array_map('strval', array_keys($cols));
        if ($db_tabs[$name]['cols'] !== $expected_cols) return false;

        $col_count = count($expected_cols);
        foreach ($db_tabs[$name]['values'] as $row) {
            if (!is_array($row) || count($row) !== $col_count) return false;
        }
    }

    return true;
}

?>
