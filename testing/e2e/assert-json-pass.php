<?php

if ($argc < 2) {
    fwrite(STDERR, "Usage: php assert-json-pass.php <result.json>\n");
    exit(2);
}

$raw = file_get_contents($argv[1]);
$data = json_decode($raw, true);
if (!is_array($data)) {
    fwrite(STDERR, "Invalid JSON in {$argv[1]}\n");
    exit(2);
}

if (($data['all_pass'] ?? false) === true) {
    exit(0);
}

$group = $data['case_group'] ?? basename($argv[1]);
fwrite(STDERR, "Case group failed: {$group}\n");
foreach (($data['cases'] ?? array()) as $case) {
    if (($case['pass'] ?? true) === true) {
        continue;
    }
    fwrite(STDERR, " - " . ($case['case'] ?? '(unnamed case)') . "\n");
    foreach (($case['checks'] ?? array()) as $check) {
        if (is_array($check) && ($check['pass'] ?? true) !== true) {
            fwrite(STDERR, "   * " . ($check['message'] ?? '(failed check)') . "\n");
        }
    }
}
exit(1);
