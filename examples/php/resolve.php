<?php
require __DIR__ . '/vendor/autoload.php';

use Net_DNS2\Resolver;

$domain = '8.8.8.8.geons';
$resolver = new Resolver([
    'nameservers' => ['127.0.0.1'],
    'port' => 5300,
]);

try {
    $response = $resolver->query($domain, 'TXT');
    foreach ($response->answer as $record) {
        if ($record->type === 'TXT') {
            echo implode('', $record->text) . PHP_EOL;
        }
    }
} catch (Exception $e) {
    fwrite(STDERR, 'DNS lookup error: ' . $e->getMessage() . "\n");
    exit(1);
}
