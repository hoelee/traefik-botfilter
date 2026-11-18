# java-cache

Bounded Caffeine cache plus role-based security configuration, extracted from
the Spring Boot service in `hoelee/springboot-hoelee-demo`.

`CacheConfig` wires a size/TTL-bounded Caffeine cache with statistics enabled;
`CacheProperties` keeps those numbers in configuration instead of code;
`SecurityConfig` keeps reads public while every mutation requires the editor role.

```
mvn -q test
```
