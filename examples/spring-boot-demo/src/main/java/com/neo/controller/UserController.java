package com.neo.controller;

import com.neo.config.BaseResult;
import com.neo.model.User;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.responses.ApiResponse;
import io.swagger.v3.oas.annotations.responses.ApiResponses;
import io.swagger.v3.oas.annotations.tags.Tag;
import org.springdoc.core.annotations.ParameterObject;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestMethod;
import org.springframework.web.bind.annotation.RestController;

import java.util.*;


@Tag(name = "用户管理", description = "用户管理API")
@RestController
@RequestMapping(value = "/user")
public class UserController {
    static Map<Long, User> users = Collections.synchronizedMap(new HashMap<>());

    @Operation(summary = "获取用户列表", description = "查询用户列表")
    @RequestMapping(value = {""}, method = RequestMethod.GET)
    @ApiResponses({
            @ApiResponse(responseCode = "100", description = "异常数据")
    })
    public List<User> getUserList() {
        return new ArrayList<>(users.values());
    }

    @Operation(summary = "创建用户", description = "根据User对象创建用户")
    @RequestMapping(value = "", method = RequestMethod.POST)
    public BaseResult<User> postUser(@ParameterObject User user) {
        users.put(user.getId(), user);
        return BaseResult.successWithData(user);
    }

    @Operation(summary = "获取用户详细信息", description = "根据url的id来获取用户详细信息")
    @RequestMapping(value = "/{id}", method = RequestMethod.GET)
    public User getUser(@PathVariable Long id) {
        return users.get(id);
    }

    @Operation(summary = "更新用户信息", description = "根据用户ID更新信息")
    @RequestMapping(value = "/{id}", method = RequestMethod.PUT)
    public BaseResult<User> putUser(@PathVariable Long id, @ParameterObject User user) {
        User u = users.get(id);
        u.setName(user.getName());
        u.setAge(user.getAge());
        users.put(id, u);
        return BaseResult.successWithData(u);
    }

    @RequestMapping(value = "/{id}", method = RequestMethod.DELETE)
    public String deleteUser(@PathVariable Long id) {
        users.remove(id);
        return "success";
    }

    @RequestMapping(value = "/ignoreMe/{id}", method = RequestMethod.DELETE)
    public String ignoreMe(@PathVariable Long id) {
        users.remove(id);
        return "success";
    }
}
