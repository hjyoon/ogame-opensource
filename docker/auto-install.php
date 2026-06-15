<?php

mysqli_report(MYSQLI_REPORT_OFF);

$host = getenv('OGAME_MDB_HOST') ?: 'mysql';
$user = getenv('OGAME_MDB_USER') ?: 'root';
$pass = getenv('OGAME_MDB_PASS');
if ($pass === false) {
    $pass = getenv('MYSQL_ROOT_PASSWORD') ?: '123';
}
$name = getenv('OGAME_MDB_NAME') ?: 'master';
$timeout = (int) (getenv('OGAME_DB_WAIT_TIMEOUT') ?: 60);
$configPath = '/var/www/html/persistent_configs/root_config.php';

if (file_exists($configPath) && !filter_var(getenv('OGAME_AUTO_INSTALL_OVERWRITE'), FILTER_VALIDATE_BOOLEAN)) {
    fwrite(STDOUT, "OGame master config already exists; skipping auto-install.\n");
    exit(0);
}

$deadline = time() + max(1, $timeout);
$connectError = '';
$mysqli = false;

do {
    $mysqli = @mysqli_connect($host, $user, $pass);
    if ($mysqli) {
        break;
    }

    $connectError = mysqli_connect_error();
    sleep(1);
} while (time() < $deadline);

if (!$mysqli) {
    fwrite(STDERR, "Unable to connect to MySQL at {$host}: {$connectError}\n");
    exit(1);
}

$quotedName = '`' . str_replace('`', '``', $name) . '`';

$queries = [
    "CREATE DATABASE IF NOT EXISTS {$quotedName} CHARACTER SET utf8 COLLATE utf8_general_ci",
    "USE {$quotedName}",
    "SET NAMES 'utf8'",
    "SET CHARACTER SET 'utf8'",
    "SET SESSION collation_connection = 'utf8_general_ci'",
    "CREATE TABLE IF NOT EXISTS unis (id INT AUTO_INCREMENT PRIMARY KEY, num INT, dbhost TEXT, dbuser TEXT, dbpass TEXT, dbname TEXT, uniurl TEXT) CHARACTER SET utf8 COLLATE utf8_general_ci",
    "CREATE TABLE IF NOT EXISTS coupons (id INT AUTO_INCREMENT PRIMARY KEY, code TEXT, amount INT UNSIGNED, used INT, user_uni INT, user_id INT, user_name TEXT) CHARACTER SET utf8 COLLATE utf8_general_ci",
];

foreach ($queries as $query) {
    if (!mysqli_query($mysqli, $query)) {
        fwrite(STDERR, "Auto-install query failed: " . mysqli_error($mysqli) . "\n{$query}\n");
        exit(1);
    }
}

if (!is_dir(dirname($configPath)) && !mkdir(dirname($configPath), 0775, true)) {
    fwrite(STDERR, "Unable to create config directory: " . dirname($configPath) . "\n");
    exit(1);
}

$config = "<?php\r\n";
$config .= "// DO NOT MODIFY!\r\n";
$config .= '$mdb_host=' . var_export($host, true) . ";\r\n";
$config .= '$mdb_user=' . var_export($user, true) . ";\r\n";
$config .= '$mdb_pass=' . var_export($pass, true) . ";\r\n";
$config .= '$mdb_name=' . var_export($name, true) . ";\r\n";
$config .= "?>";

if (file_put_contents($configPath, $config) === false) {
    fwrite(STDERR, "Unable to write master config: {$configPath}\n");
    exit(1);
}

fwrite(STDOUT, "OGame master database auto-install complete.\n");
