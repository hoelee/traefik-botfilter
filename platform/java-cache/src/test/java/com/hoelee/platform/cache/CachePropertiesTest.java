package com.hoelee.platform.cache;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;

import java.time.Duration;
import org.junit.jupiter.api.Test;

class CachePropertiesTest {

    @Test
    void keepsConfiguredSizeAndTtl() {
        CacheProperties props = new CacheProperties(500, Duration.ofMinutes(5));
        assertEquals(500, props.maximumSize());
        assertEquals(Duration.ofMinutes(5), props.ttl());
    }

    @Test
    void ttlIsRequired() {
        assertNotNull(new CacheProperties(1, Duration.ofSeconds(1)).ttl());
    }
}
