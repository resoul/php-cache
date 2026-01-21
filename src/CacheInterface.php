<?php

declare(strict_types=1);

namespace Resoul\Cache;

interface CacheInterface
{
    public function get(string $key): ?string;
    public function set(string $key, string $value, int $ttl = 3600): bool;
    public function has(string $key): bool;
    public function delete(string $key): bool;
    public function clear(): bool;
}
