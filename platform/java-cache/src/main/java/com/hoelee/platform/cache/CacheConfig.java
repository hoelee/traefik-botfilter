package com.hoelee.platform.cache;

import com.github.benmanes.caffeine.cache.Caffeine;
import org.springframework.cache.CacheManager;
import org.springframework.cache.annotation.EnableCaching;
import org.springframework.cache.caffeine.CaffeineCacheManager;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * @version v2, 2024-10-29 10:05:00PM
 * @author hoelee
 * Learning note: a bounded local cache improves hot-read latency while its statistics make the benefit observable.
 */
@Configuration
@EnableCaching
public class CacheConfig {

    @Bean
    CacheManager cacheManager(CacheProperties properties) {
        CaffeineCacheManager cacheManager = new CaffeineCacheManager("posts");
        cacheManager.setCaffeine(Caffeine.newBuilder()
                .recordStats()
                .maximumSize(properties.maximumSize())
                .expireAfterWrite(properties.ttl()));
        return cacheManager;
    }
}

