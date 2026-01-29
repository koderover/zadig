package cache

import (
	"errors"
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/util"
	"github.com/redis/go-redis/v9"
)

func AppendBigStringToRedis(key string, value string) error {
	redisCache := NewRedisCache(config.RedisCommonCacheTokenDB())

	// 读取现有的日志内容
	existingCompressedLogStr, err := redisCache.GetString(key)
	if err != nil && !errors.Is(err, redis.Nil) {
		// 如果错误不是 key 不存在，则返回错误
		return fmt.Errorf("failed to read existing string from redis, key: %s, error: %s", key, err)
	}

	// 追加新内容到现有日志
	var newLogContent string
	if err == nil && existingCompressedLogStr != "" {
		// 解压缩现有日志（将字符串转换为字节数组）
		existingLog, decompressErr := util.DecompressBytes([]byte(existingCompressedLogStr))
		if decompressErr != nil {
			return fmt.Errorf("failed to decompress existing string, key: %s, error: %s", key, decompressErr)
		}
		newLogContent = existingLog + value
	} else {
		newLogContent = value
	}

	compressedLogBytes, err := util.CompressBytes(newLogContent)
	if err != nil {
		return fmt.Errorf("failed to compress string, key: %s, error: %s", key, err)
	}

	// 写回 Redis
	err = redisCache.Write(key, string(compressedLogBytes), 3*24*time.Hour)
	if err != nil {
		err = fmt.Errorf("failed to write string to redis, key: %s, error: %s", key, err)
		return err
	}

	return nil
}

func GetBigStringFromRedis(logKey string) (string, error) {
	redisCache := NewRedisCache(config.RedisCommonCacheTokenDB())

	compressedDataStr, err := redisCache.GetString(logKey)
	if err != nil {
		return "", err
	}

	decompressedData, err := util.DecompressBytes([]byte(compressedDataStr))
	if err != nil {
		return "", fmt.Errorf("failed to decompress job log, key: %s, error: %s", logKey, err)
	}

	return decompressedData, nil
}

func DeleteBigStringFromRedis(key string) error {
	redisCache := NewRedisCache(config.RedisCommonCacheTokenDB())
	return redisCache.Delete(key)
}
