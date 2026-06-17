<?php

// Check if the configuration file is missing - redirect to the game installation page.
if ( !file_exists ("config.php"))
{
    echo "<html><head><meta http-equiv='refresh' content='0;url=install.php' /></head><body></body></html>";
    exit ();
}

require_once "config.php";
require_once "core/core.php";
SendSecurityHeaders();

// All links from the game to the outside go through this script.
// Supposedly there could be filters for undesirable websites here.

$url = NormalizeExternalUrl($_GET['url'] ?? '', true);
if ($url === "") {
    http_response_code(400);
    header('Content-Type: text/html; charset=UTF-8');
    echo "<html><head><title>Invalid URL</title></head><body>Invalid URL</body></html>";
    exit();
}

$safe_url = htmlsafe($url);

?>

<HTML>
<HEAD>
<META HTTP-EQUIV="refresh" content="0;URL=<?=$safe_url;?>">
<TITLE>Page has moved</TITLE>
</HEAD>
<BODY>
Page has moved
</BODY>
</HTML>

