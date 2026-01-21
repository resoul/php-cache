<?php

declare(strict_types=1);

namespace Resoul\Cache;

class FileCache implements CacheInterface
{
    private string $cacheDir;

    public function __construct(string $cacheDir)
    {
        $this->cacheDir = rtrim($cacheDir, '/');

        if (!is_dir($this->cacheDir)) {
            mkdir($this->cacheDir, 0755, true);
        }
    }

    public function get(string $key): ?string
    {
        $file = $this->getFilePath($key);

        if (!file_exists($file)) {
            return null;
        }

        $content = file_get_contents($file);
        $data = unserialize($content);

        if ($data['expires_at'] !== null && $data['expires_at'] < time()) {
            $this->delete($key);
            return null;
        }

        return $data['value'];
    }

    public function set(string $key, string $value, int $ttl = 3600): bool
    {
        $file = $this->getFilePath($key);
        $dir = dirname($file);

        if (!is_dir($dir)) {
            mkdir($dir, 0755, true);
        }

        $data = [
            'value' => $value,
            'expires_at' => $ttl > 0 ? time() + $ttl : null,
            'created_at' => time(),
        ];

        return file_put_contents($file, serialize($data), LOCK_EX) !== false;
    }

    public function has(string $key): bool
    {
        return $this->get($key) !== null;
    }

    public function delete(string $key): bool
    {
        $file = $this->getFilePath($key);

        if (file_exists($file)) {
            return unlink($file);
        }

        return true;
    }

    public function clear(): bool
    {
        $files = glob($this->cacheDir . '/*/*.cache');

        foreach ($files as $file) {
            if (is_file($file)) {
                unlink($file);
            }
        }

        return true;
    }

    private function getFilePath(string $key): string
    {
        $hash = md5($key);

        return sprintf(
            '%s/%s/%s.cache',
            $this->cacheDir,
            substr($hash, 0, 2),
            $hash
        );
    }
}