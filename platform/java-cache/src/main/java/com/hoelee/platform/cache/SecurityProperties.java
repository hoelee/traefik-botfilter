package com.hoelee.platform.cache;

import jakarta.validation.constraints.NotBlank;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.validation.annotation.Validated;

/**
 * @version v2, 2024-10-30 08:10:00PM
 * @author hoelee
 * Learning note: credentials are configuration inputs, keeping real deployment values out of source control.
 */
@Validated
@ConfigurationProperties(prefix = "app.security")
public record SecurityProperties(@NotBlank String editorUsername, @NotBlank String editorPassword) {
}

