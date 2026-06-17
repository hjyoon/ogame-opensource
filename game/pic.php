<?php

// Script to display pictures and scan them for malware.

if ( !file_exists ("config.php"))
{
    echo "<html><head><meta http-equiv='refresh' content='0;url=install.php' /></head><body></body></html>";
    exit ();
}

require_once "config.php";
require_once "core/core.php";

$url = NormalizeExternalUrl($_GET['url'] ?? '', true);

$extList = array();
$extList['gif'] = 'image/gif';
$extList['jpg'] = 'image/jpeg';
$extList['jpeg'] = 'image/jpeg';
$extList['png'] = 'image/png';

$path = parse_url($url, PHP_URL_PATH);
$extension = strtolower(pathinfo($path ?: "", PATHINFO_EXTENSION));

if ($url !== "" && isset($extList[$extension]))
{
    $previousTimeout = ini_get('default_socket_timeout');
    ini_set('default_socket_timeout', '5');
    $imageSize = @getimagesize($url);
    if ($previousTimeout !== false) {
        ini_set('default_socket_timeout', $previousTimeout);
    }

    if ($imageSize !== false && (!isset($imageSize['mime']) || $imageSize['mime'] === $extList[$extension])) {
        header ('Content-type: '.$extList[$extension]);
        readfile ($url);
        exit();
    }
}

header ('Content-type: text/html; charset=UTF-8');
echo "<font color=red><b>Графика недоступна</b></font>";
?>
