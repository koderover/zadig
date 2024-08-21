package org.example;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/greeting")
public class GreetingController {

    @Autowired
    private GreetingClient greetingClient;

    @GetMapping
    public String getGreeting(@RequestHeader("version") String version) {
        return greetingClient.getGreeting(version) + " - Processed by Service 3";
    }
}
