package com.hoelee.platform.cache;

import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotNull;
import java.time.Duration;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.validation.annotation.Validated;

/**
 * @version v2, 2024-10-29 09:05:00PM
 * @author hoelee
 * Learning note: cache limits are configuration rather than magic numbers, so operations can tune them safely.
 */
@Validated
@ConfigurationProperties(prefix = "app.cache")
public record CacheProperties(@Min(1) int maximumSize, @NotNull Duration ttl) {
}

