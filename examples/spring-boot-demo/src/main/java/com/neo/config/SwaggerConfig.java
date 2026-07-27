package com.neo.config;

import io.swagger.v3.oas.models.OpenAPI;
import io.swagger.v3.oas.models.info.Contact;
import io.swagger.v3.oas.models.info.Info;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class SwaggerConfig {

    @Bean
    public OpenAPI api() {
        return new OpenAPI().info(new Info()
                .title("客户管理")
                .description("客户管理中心 API 1.0 操作文档")
                .version("1.0")
                .contact(new Contact()
                        .name("KodeRover")
                        .url("https://www.koderover.com/")
                        .email("demo@koderover.com")));
    }
}
