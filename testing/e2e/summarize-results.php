<?php

if ($argc < 2) {
    fwrite(STDERR, "Usage: php summarize-results.php <result-dir>\n");
    exit(2);
}

$outDir = rtrim($argv[1], '/');
if (!is_dir($outDir)) {
    fwrite(STDERR, "Result directory not found: {$outDir}\n");
    exit(2);
}

function e2e_summary_read_json(string $path): ?array
{
    $raw = @file_get_contents($path);
    if ($raw === false || trim($raw) === '') {
        return null;
    }

    $data = json_decode($raw, true);
    return is_array($data) ? $data : null;
}

function e2e_summary_check_count(array $case): int
{
    $checks = $case['checks'] ?? array();
    return is_array($checks) ? count($checks) : 0;
}

function e2e_summary_failed_checks(array $case): array
{
    $failed = array();
    foreach (($case['checks'] ?? array()) as $check) {
        if (is_array($check) && ($check['pass'] ?? true) !== true) {
            $failed[] = array(
                'message' => $check['message'] ?? '(failed check)',
                'context' => $check['context'] ?? null,
            );
        }
    }
    return $failed;
}

function e2e_summary_stderr_preview(string $path): string
{
    $raw = @file_get_contents($path);
    if ($raw === false) {
        return '';
    }

    $lines = preg_split('/\r\n|\r|\n/', trim($raw));
    if (!is_array($lines)) {
        return '';
    }

    return implode("\n", array_slice($lines, 0, 20));
}

function e2e_summary_markdown(array $summary): string
{
    $lines = array();
    $lines[] = '# E2E Summary';
    $lines[] = '';
    $lines[] = '- Generated: ' . $summary['generated_at'];
    $lines[] = '- Result directory: `' . $summary['result_dir'] . '`';
    $lines[] = '- Overall: ' . ($summary['all_pass'] ? 'PASS' : 'FAIL');
    $lines[] = '- Case result files: ' . $summary['case_result_files'];
    $lines[] = '- Cases: ' . $summary['case_count'];
    $lines[] = '- Checks: ' . $summary['check_count'];
    $lines[] = '- Failed groups: ' . count($summary['failed_groups']);
    $lines[] = '- Non-empty stderr files: ' . count($summary['stderr_failures']);
    $lines[] = '';

    if (!empty($summary['failed_groups'])) {
        $lines[] = '## Failed Cases';
        foreach ($summary['failed_groups'] as $group) {
            $lines[] = '';
            $lines[] = '### ' . $group['name'];
            if (isset($group['error'])) {
                $lines[] = '- Error: ' . $group['error'];
            }
            foreach (($group['failed_cases'] ?? array()) as $case) {
                $lines[] = '- ' . $case['case'];
                foreach ($case['failed_checks'] as $check) {
                    $lines[] = '  - ' . $check['message'];
                }
            }
        }
        $lines[] = '';
    }

    if (!empty($summary['stderr_failures'])) {
        $lines[] = '## Runtime Stderr';
        foreach ($summary['stderr_failures'] as $stderr) {
            $lines[] = '';
            $lines[] = '### ' . $stderr['name'];
            $lines[] = '```text';
            $lines[] = $stderr['preview'];
            $lines[] = '```';
        }
        $lines[] = '';
    }

    if (!empty($summary['performance'])) {
        $lines[] = '## Performance Baseline';
        $perf = $summary['performance'];
        if (isset($perf['recorded_at'])) {
            $lines[] = '- Recorded: ' . $perf['recorded_at'];
        }
        if (isset($perf['baseline_file'])) {
            $lines[] = '- Baseline file: `' . $perf['baseline_file'] . '`';
        }
        if (!empty($perf['metrics'])) {
            $lines[] = '';
            $lines[] = '| Metric | Elapsed ms | Threshold ms | Bytes |';
            $lines[] = '| --- | ---: | ---: | ---: |';
            foreach ($perf['metrics'] as $label => $metric) {
                $lines[] = '| ' . $label . ' | ' . ($metric['elapsed_ms'] ?? '') . ' | ' . ($metric['threshold_ms'] ?? '') . ' | ' . ($metric['bytes'] ?? '') . ' |';
            }
        }
        $lines[] = '';
    }

    $lines[] = '## Result Files';
    foreach ($summary['groups'] as $group) {
        $lines[] = '- `' . $group['file'] . '`: ' . ($group['pass'] ? 'PASS' : 'FAIL') . ' (' . $group['case_count'] . ' cases, ' . $group['check_count'] . ' checks)';
    }
    $lines[] = '';

    return implode("\n", $lines);
}

$summary = array(
    'generated_at' => gmdate('c'),
    'result_dir' => $outDir,
    'all_pass' => true,
    'case_result_files' => 0,
    'case_count' => 0,
    'check_count' => 0,
    'groups' => array(),
    'failed_groups' => array(),
    'stderr_failures' => array(),
    'performance' => null,
);

foreach (glob($outDir . '/*.stderr') ?: array() as $stderrPath) {
    if (is_file($stderrPath) && filesize($stderrPath) > 0) {
        $summary['all_pass'] = false;
        $summary['stderr_failures'][] = array(
            'name' => basename($stderrPath, '.stderr'),
            'file' => basename($stderrPath),
            'preview' => e2e_summary_stderr_preview($stderrPath),
        );
    }
}

foreach (glob($outDir . '/*.json') ?: array() as $jsonPath) {
    $file = basename($jsonPath);
    if ($file === 'summary.json' || $file === 'performance-baseline-metrics.json') {
        continue;
    }

    $data = e2e_summary_read_json($jsonPath);
    if ($data === null) {
        $summary['all_pass'] = false;
        $summary['case_result_files']++;
        $group = array(
            'name' => basename($jsonPath, '.json'),
            'file' => $file,
            'pass' => false,
            'case_count' => 0,
            'check_count' => 0,
            'error' => 'Invalid JSON result file',
        );
        $summary['groups'][] = $group;
        $summary['failed_groups'][] = $group;
        continue;
    }

    if (!array_key_exists('case_group', $data) && !array_key_exists('all_pass', $data)) {
        continue;
    }

    $caseCount = 0;
    $checkCount = 0;
    $failedCases = array();
    foreach (($data['cases'] ?? array()) as $case) {
        if (!is_array($case)) {
            continue;
        }
        $caseCount++;
        $checkCount += e2e_summary_check_count($case);
        if (($case['pass'] ?? true) !== true) {
            $failedCases[] = array(
                'case' => $case['case'] ?? '(unnamed case)',
                'failed_checks' => e2e_summary_failed_checks($case),
            );
        }
    }

    $pass = ($data['all_pass'] ?? false) === true && empty($failedCases);
    $group = array(
        'name' => $data['case_group'] ?? basename($jsonPath, '.json'),
        'file' => $file,
        'pass' => $pass,
        'case_count' => $caseCount,
        'check_count' => $checkCount,
    );

    $summary['case_result_files']++;
    $summary['case_count'] += $caseCount;
    $summary['check_count'] += $checkCount;
    $summary['groups'][] = $group;

    if (!$pass) {
        $summary['all_pass'] = false;
        $group['failed_cases'] = $failedCases;
        $summary['failed_groups'][] = $group;
    }
}

$perfBaselinePath = $outDir . '/performance-baseline-metrics.json';
$perfBaseline = e2e_summary_read_json($perfBaselinePath);
if ($perfBaseline !== null && isset($perfBaseline['metrics']) && is_array($perfBaseline['metrics'])) {
    $summary['performance'] = array(
        'recorded_at' => $perfBaseline['recorded_at'] ?? null,
        'baseline_file' => basename($perfBaselinePath),
        'metrics' => $perfBaseline['metrics'],
    );
}

usort($summary['groups'], fn($a, $b) => strcmp($a['file'], $b['file']));

$summaryJson = json_encode($summary, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES);
if ($summaryJson === false || @file_put_contents($outDir . '/summary.json', $summaryJson . PHP_EOL) === false) {
    fwrite(STDERR, "Failed to write {$outDir}/summary.json\n");
    exit(2);
}

$summaryMarkdown = e2e_summary_markdown($summary);
if (@file_put_contents($outDir . '/summary.md', $summaryMarkdown . PHP_EOL) === false) {
    fwrite(STDERR, "Failed to write {$outDir}/summary.md\n");
    exit(2);
}

fwrite(STDOUT, "Summary written: {$outDir}/summary.json {$outDir}/summary.md\n");
exit(0);
