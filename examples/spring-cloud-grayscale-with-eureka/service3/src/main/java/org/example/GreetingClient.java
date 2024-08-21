package org.example;

import org.springframework.cloud.openfeign.FeignClient;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestHeader;

@FeignClient(name = "gateway", url="${gateway.url}")
public interface GreetingClient {
    @GetMapping("/service2/greeting")
    String getGreeting(@RequestHeader("version") String version);
}
