<?php
header('Content-Type: text/plain');
echo "REMOTE_ADDR=" . ($_SERVER['REMOTE_ADDR'] ?? 'null') . "\n";
echo "XFF=" . ($_SERVER['HTTP_X_FORWARDED_FOR'] ?? 'null') . "\n";
echo "XRI=" . ($_SERVER['HTTP_X_REAL_IP'] ?? 'null') . "\n";
