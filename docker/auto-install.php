<?php

mysqli_report(MYSQLI_REPORT_OFF);

function env_value(string $name, string $default = ''): string
{
    $value = getenv($name);
    return $value === false ? $default : $value;
}

function env_bool(string $name, bool $default = false): bool
{
    $value = getenv($name);
    if ($value === false) {
        return $default;
    }

    return filter_var($value, FILTER_VALIDATE_BOOLEAN);
}

function fail(string $message): void
{
    fwrite(STDERR, $message . "\n");
    exit(1);
}

function quote_identifier(string $name): string
{
    return '`' . str_replace('`', '``', $name) . '`';
}

function mysql_password(string $explicitEnv): string
{
    $password = getenv($explicitEnv);
    if ($password !== false) {
        return $password;
    }

    return env_value('MYSQL_ROOT_PASSWORD', '123');
}

function connect_mysql(string $host, string $user, string $password): mysqli
{
    $timeout = (int) env_value('OGAME_DB_WAIT_TIMEOUT', '60');
    $deadline = time() + max(1, $timeout);
    $connectError = '';

    do {
        $mysqli = @mysqli_connect($host, $user, $password);
        if ($mysqli) {
            return $mysqli;
        }

        $connectError = mysqli_connect_error();
        sleep(1);
    } while (time() < $deadline);

    fail("Unable to connect to MySQL at {$host}: {$connectError}");
}

function run_queries(mysqli $mysqli, array $queries): void
{
    foreach ($queries as $query) {
        if (!mysqli_query($mysqli, $query)) {
            fail("Auto-install query failed: " . mysqli_error($mysqli) . "\n{$query}");
        }
    }
}

function ensure_directory(string $path): void
{
    if (!is_dir($path) && !mkdir($path, 0775, true)) {
        fail("Unable to create directory: {$path}");
    }
}

function write_php_config(string $path, array $values): void
{
    ensure_directory(dirname($path));

    $config = "<?php\r\n";
    $config .= "// DO NOT MODIFY!\r\n";
    foreach ($values as $name => $value) {
        $config .= '$' . $name . '=' . var_export($value, true) . ";\r\n";
    }
    $config .= "?>";

    if (file_put_contents($path, $config) === false) {
        fail("Unable to write config: {$path}");
    }
}

function install_master_database(): array
{
    $host = env_value('OGAME_MDB_HOST', 'mysql');
    $user = env_value('OGAME_MDB_USER', 'root');
    $password = mysql_password('OGAME_MDB_PASS');
    $name = env_value('OGAME_MDB_NAME', 'master');
    $configPath = '/var/www/html/persistent_configs/root_config.php';

    $mysqli = connect_mysql($host, $user, $password);
    $quotedName = quote_identifier($name);

    run_queries($mysqli, [
        "CREATE DATABASE IF NOT EXISTS {$quotedName} CHARACTER SET utf8 COLLATE utf8_general_ci",
        "USE {$quotedName}",
        "SET NAMES 'utf8'",
        "SET CHARACTER SET 'utf8'",
        "SET SESSION collation_connection = 'utf8_general_ci'",
        "CREATE TABLE IF NOT EXISTS unis (id INT AUTO_INCREMENT PRIMARY KEY, num INT, dbhost TEXT, dbuser TEXT, dbpass TEXT, dbname TEXT, uniurl TEXT) CHARACTER SET utf8 COLLATE utf8_general_ci",
        "CREATE TABLE IF NOT EXISTS coupons (id INT AUTO_INCREMENT PRIMARY KEY, code TEXT, amount INT UNSIGNED, used INT, user_uni INT, user_id INT, user_name TEXT) CHARACTER SET utf8 COLLATE utf8_general_ci",
    ]);

    if (file_exists($configPath) && !env_bool('OGAME_AUTO_INSTALL_OVERWRITE')) {
        fwrite(STDOUT, "OGame master config already exists; skipping config write.\n");
    } else {
        write_php_config($configPath, [
            'mdb_host' => $host,
            'mdb_user' => $user,
            'mdb_pass' => $password,
            'mdb_name' => $name,
        ]);
        fwrite(STDOUT, "OGame master database auto-install complete.\n");
    }

    return [
        'host' => $host,
        'user' => $user,
        'pass' => $password,
        'name' => $name,
    ];
}

function public_host(): string
{
    $url = env_value('OGAME_UNI_URL', 'localhost:8888');
    $url = preg_replace('#^https?://#', '', $url);
    return rtrim((string) $url, '/');
}

function create_database(string $host, string $user, string $password, string $name): void
{
    $mysqli = connect_mysql($host, $user, $password);
    $quotedName = quote_identifier($name);
    run_queries($mysqli, [
        "CREATE DATABASE IF NOT EXISTS {$quotedName} CHARACTER SET utf8 COLLATE utf8_general_ci",
    ]);
}

function install_universe(array $master): void
{
    if (!env_bool('OGAME_UNI_AUTO_INSTALL', true)) {
        fwrite(STDOUT, "OGame universe auto-install disabled.\n");
        return;
    }

    $configPath = '/var/www/html/persistent_configs/game_config.php';
    if (file_exists($configPath) && !env_bool('OGAME_UNI_AUTO_INSTALL_OVERWRITE')) {
        fwrite(STDOUT, "OGame universe config already exists; skipping auto-install.\n");
        return;
    }

    if (file_exists($configPath) && env_bool('OGAME_UNI_AUTO_INSTALL_OVERWRITE')) {
        unlink($configPath);
    }

    $dbHost = env_value('OGAME_UNI_DB_HOST', 'mysql');
    $dbUser = env_value('OGAME_UNI_DB_USER', 'root');
    $dbPass = mysql_password('OGAME_UNI_DB_PASS');
    $dbName = env_value('OGAME_UNI_DB_NAME', 'uni');
    $publicHost = public_host();
    $startPage = env_value('OGAME_STARTPAGE', 'http://' . $publicHost);

    create_database($dbHost, $dbUser, $dbPass, $dbName);

    $_SERVER['REQUEST_METHOD'] = 'POST';
    $_SERVER['HTTP_HOST'] = $publicHost;
    $_SERVER['SCRIPT_NAME'] = '/game/install.php';
    $_SERVER['HTTPS'] = str_starts_with($startPage, 'https://') ? 'on' : '';
    $_COOKIE['ogamelang'] = env_value('OGAME_UNI_LANG', 'en');

    $_POST = [
        'install' => '1',
        'uni_lang' => env_value('OGAME_UNI_LANG', 'en'),
        'startpage' => rtrim($startPage, '/'),
        'db_host' => $dbHost,
        'db_user' => $dbUser,
        'db_pass' => $dbPass,
        'db_name' => $dbName,
        'db_prefix' => env_value('OGAME_UNI_DB_PREFIX', 'uni1_'),
        'db_secret' => env_value('OGAME_UNI_DB_SECRET', 'docker-secret'),
        'mdb_host' => env_value('OGAME_MDB_HOST', $master['host']),
        'mdb_user' => env_value('OGAME_MDB_USER', $master['user']),
        'mdb_pass' => mysql_password('OGAME_MDB_PASS'),
        'mdb_name' => env_value('OGAME_MDB_NAME', $master['name']),
        'uni_num' => env_value('OGAME_UNI_NUM', '1'),
        'uni_speed' => env_value('OGAME_UNI_SPEED', '1'),
        'uni_fspeed' => env_value('OGAME_UNI_FLEET_SPEED', '1'),
        'uni_galaxies' => env_value('OGAME_UNI_GALAXIES', '9'),
        'uni_systems' => env_value('OGAME_UNI_SYSTEMS', '499'),
        'uni_maxusers' => env_value('OGAME_UNI_MAX_USERS', '12500'),
        'start_dm' => env_value('OGAME_UNI_START_DM', '0'),
        'uni_acs' => env_value('OGAME_UNI_ACS', '4'),
        'uni_fid' => env_value('OGAME_UNI_FID', '30'),
        'uni_did' => env_value('OGAME_UNI_DID', '0'),
        'uni_battle_engine' => env_value('OGAME_UNI_BATTLE_ENGINE', '../cgi-bin/battle'),
        'battle_max' => env_value('OGAME_UNI_BATTLE_MAX', '1000000'),
        'max_werf' => env_value('OGAME_UNI_MAX_WERF', '999'),
        'feedage' => env_value('OGAME_UNI_FEED_AGE', '60'),
        'ext_board' => env_value('OGAME_EXT_BOARD', ''),
        'ext_discord' => env_value('OGAME_EXT_DISCORD', ''),
        'ext_tutorial' => env_value('OGAME_EXT_TUTORIAL', ''),
        'ext_rules' => env_value('OGAME_EXT_RULES', ''),
        'ext_impressum' => env_value('OGAME_EXT_IMPRESSUM', ''),
        'admin_email' => env_value('OGAME_ADMIN_EMAIL', 'admin@example.local'),
        'admin_pass' => env_value('OGAME_ADMIN_PASSWORD', 'admin'),
    ];

    foreach ([
        'mdb_enable' => env_bool('OGAME_UNI_MDB_ENABLE', true),
        'uni_rapid' => env_bool('OGAME_UNI_RAPID', true),
        'uni_moons' => env_bool('OGAME_UNI_MOONS', true),
        'php_battle' => env_bool('OGAME_UNI_PHP_BATTLE', true),
        'force_lang' => env_bool('OGAME_UNI_FORCE_LANG', false),
    ] as $name => $enabled) {
        if ($enabled) {
            $_POST[$name] = 'on';
        }
    }

    chdir('/var/www/html/game');

    global $CoreVersion, $DefaultLanguage, $GlobalUser, $InstallError, $Languages, $LOCA, $MDB_link;
    global $db_connect, $db_host, $db_name, $db_pass, $db_prefix, $db_user;
    global $from_cron, $from_reg, $loca_lang, $modlist, $query_counter, $query_log, $tabs;

    ob_start();
    include '/var/www/html/game/install.php';
    ob_end_clean();

    if (!file_exists($configPath)) {
        fail("OGame universe install did not create config: {$configPath}");
    }

    fwrite(STDOUT, "OGame universe auto-install complete.\n");
}

$master = install_master_database();
install_universe($master);
