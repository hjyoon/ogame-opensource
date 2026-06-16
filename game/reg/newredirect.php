<?php

// Check if the configuration file is missing - redirect to the game installation page.
if ( !file_exists ("../config.php"))
{
    echo "<html><head><meta http-equiv='refresh' content='0;url=../install.php' /></head><body></body></html>";
    exit ();
}
else {
    require_once "../config.php";
}

require_once "../core/core.php";

loca_add ( "common", $loca_lang, "../" );
loca_add ( "debug", $loca_lang, "../" );
loca_add ( "reg", $loca_lang, "../" );

InitDB();

// Verify registration information.

$GlobalUni = LoadUniverse ();
$from_reg = true;

if ( $_SERVER['REQUEST_METHOD'] === "POST" )
{
    $agbValue = $_POST['agb'] ?? '';
    $character = $_POST['character'] ?? '';
    $password = $_POST['password'] ?? '';
    $email = $_POST['email'] ?? '';
    $universe = $_POST['universe'] ?? '';

    if ( $agbValue !== 'on' ) $AGB = 0;
    else $AGB = 1;

    $ip = $_SERVER['REMOTE_ADDR'];
    $now = time ();
    $last = GetLastRegistrationByIP ( $ip );
    $check_ip_reg = true;
    
    $RegError = 0;

    if ( $check_ip_reg && ( $now - $last ) < 10 * 60 && !localhost($ip) ) $RegError = 108;
    else if ( strlen ($password) < 8 ) $RegError = 107;
    else if ( mb_strlen ($character) < 3 || mb_strlen ($character) > 20 || preg_match ('/[;,<>()\`\"\']/', $character) ) $RegError = 103;
    else if ( IsUserExist ( $character)) $RegError = 101;
    else if ( !isValidEmail ($email) ) $RegError = 104;
    else if ( IsEmailExist ( $email)) $RegError = 102;
    else if ( GetUsersCount() >= $GlobalUni['maxusers']) $RegError = 109;

    $forbidden = explode ( ",", FORBIDDEN_LOGINS );
    $lower = mb_strtolower ($character, 'UTF-8');
    foreach ( $forbidden as $i=>$name) {
        if ( strpos($lower, $name) !== false ) {
            $RegError = 103;
            break;
        }
    }

    // If all parameters are correct - create a new user and log in to the game.
    if ($RegError == 0 && $AGB)
    {
        CreateUser ( $character, $password, $email, false );
        Login ( $character, $password );
        exit ();
    }

    echo "<html><head><meta http-equiv='refresh' content='0;url=$StartPage/register.php?errorCode=$RegError&agb=$AGB&character=".$character."&email=".$email."&universe=".$universe."' /></head><body></body></html>";
    exit ();
}

// Open new.php
echo "<html><head><meta http-equiv='refresh' content='0;url=new.php' /></head><body></body></html>";

?>
