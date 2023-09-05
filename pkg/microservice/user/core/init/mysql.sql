CREATE DATABASE IF NOT EXISTS `%s`;
use `%s`;
CREATE TABLE IF NOT EXISTS `user_login`(
    `id` bigint(20) NOT NULL AUTO_INCREMENT,
    `uid` varchar(64) NOT NULL DEFAULT '0' COMMENT '用户id',
    `login_id` varchar(64) NOT NULL DEFAULT '0' COMMENT '用户登录id,如账号名',
    `login_type` int(4) unsigned NOT NULL DEFAULT '0' COMMENT '登录类型,0.账号名',
    `password` varchar(64) DEFAULT '' COMMENT '密码',
    `last_login_time` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '最后登陆时间',
    `created_at` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '创建时间',
    `updated_at` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '更新时间',
    UNIQUE KEY `login` (`uid`,`login_id`,`login_type`),
    PRIMARY KEY (`id`),
    KEY `idx_uid` (`uid`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8 COLLATE = utf8_general_ci COMMENT = '账号登陆表' ROW_FORMAT = Compact;

CREATE TABLE IF NOT EXISTS `user`(
    `uid` varchar(64) NOT NULL COMMENT '用户ID',
    `account` varchar(32) NOT NULL DEFAULT '' COMMENT '用户账号',
    `name` varchar(32) NOT NULL DEFAULT '' COMMENT '用户名',
    `identity_type` varchar(32) NOT NULL DEFAULT 'unknown' COMMENT '用户来源',
    `phone` varchar(16) NOT NULL DEFAULT '' COMMENT '手机号码',
    `email` varchar(100) NOT NULL DEFAULT '' COMMENT '邮箱',
    `api_token` varchar(1024) NOT NULL DEFAULT '' COMMENT 'openAPIToken',
    `created_at` int(11) unsigned NOT NULL COMMENT '创建时间',
    `updated_at` int(11) unsigned NOT NULL COMMENT '修改时间',
    UNIQUE KEY `account` (`account`,`identity_type`),
    PRIMARY KEY (`uid`)
) ENGINE = InnoDB CHARACTER SET = utf8 COLLATE = utf8_general_ci COMMENT = '用户信息表' ROW_FORMAT = Compact;

CREATE TABLE IF NOT EXISTS `user_group` (
    `group_id`    varchar(64) NOT NULL COMMENT '用户组ID',
    `group_name`  varchar(32) NOT NULL UNIQUE COMMENT '用户组名',
    `description` varchar(64) NOT NULL COMMENT '简介',
    `created_at`  int(11) unsigned NOT NULL DEFAULT '0' COMMENT '创建时间',
    `updated_at`  int(11) unsigned NOT NULL DEFAULT '0' COMMENT '更新时间',
    PRIMARY KEY (`group_id`)
) ENGINE = InnoDB CHARACTER SET = utf8 COLLATE = utf8_general_ci COMMENT = '用户组信息表' ROW_FORMAT = Compact;

CREATE TABLE IF NOT EXISTS `group_binding` (
    `id` bigint(20) NOT NULL AUTO_INCREMENT,
    `group_id` varchar(64) NOT NULL COMMENT '用户组ID',
    `uid` varchar(64) NOT NULL COMMENT '用户ID',
    PRIMARY KEY (`id`),
    FOREIGN KEY (`uid`) REFERENCES user(`uid`),
    FOREIGN KEY (`group_id`) REFERENCES user_group(`group_id`)
) ENGINE = InnoDB CHARACTER SET = utf8 COLLATE = utf8_general_ci COMMENT = '用户/用户组绑定信息' ROW_FORMAT = Compact;

CREATE TABLE IF NOT EXISTS `action` (
    `id`       bigint(20) NOT NULL AUTO_INCREMENT,
    `name`     varchar(32) NOT NULL COMMENT '权限项名称',
    `action`   varchar(32) NOT NULL COMMENT '权限项',
    `resource` varchar(32) NOT NULL COMMENT '资源类别',
    `scope`    int(11) NOT NULL COMMENT '资源范围，1-项目范围， 2-全局',
    PRIMARY KEY (`id`),
    UNIQUE KEY `resource_action` (`action`,`resource`)
) ENGINE = InnoDB CHARACTER SET = utf8 COLLATE = utf8_general_ci COMMENT = '权限项表' ROW_FORMAT = Compact;

CREATE TABLE IF NOT EXISTS `role` (
    `id`          bigint(20) NOT NULL AUTO_INCREMENT,
    `name`        varchar(32) NOT NULL COMMENT '角色名称',
    `description` varchar(64) NOT NULL COMMENT '描述',
    `type`        int(11) NOT NULL COMMENT '资源范围，1-系统自带， 2-用户自定义',
    `namespace`   varchar(32) NOT NULL COMMENT '所属项目，*为全局角色标记',
    PRIMARY KEY (`id`),
    UNIQUE KEY `namespaced_role` (`namespace`, `name`)
) ENGINE = InnoDB CHARACTER SET = utf8 COLLATE = utf8_general_ci COMMENT = '角色表' ROW_FORMAT = Compact;

CREATE TABLE IF NOT EXISTS `role_action_binding` (
    `id`        bigint(20) NOT NULL AUTO_INCREMENT,
    `action_id` bigint(20) NOT NULL COMMENT '用户组ID',
    `role_id`   bigint(20) NOT NULL COMMENT '角色ID',
    PRIMARY KEY (`id`),
    FOREIGN KEY (`action_id`) REFERENCES action(`id`),
    FOREIGN KEY (`role_id`) REFERENCES role(`id`)
) ENGINE = InnoDB CHARACTER SET = utf8 COLLATE = utf8_general_ci COMMENT = '角色/权限项绑定信息' ROW_FORMAT = Compact;

CREATE TABLE IF NOT EXISTS `role_binding` (
    `id`        bigint(20) NOT NULL AUTO_INCREMENT,
    `uid`   varchar(64) NOT NULL COMMENT '用户ID',
    `role_id`   bigint(20) NOT NULL COMMENT '角色ID',
    PRIMARY KEY (`id`),
    FOREIGN KEY (`uid`) REFERENCES user(`uid`),
    FOREIGN KEY (`role_id`) REFERENCES role(`id`)
) ENGINE = InnoDB CHARACTER SET = utf8 COLLATE = utf8_general_ci COMMENT = '角色/用户绑定信息' ROW_FORMAT = Compact;

CREATE TABLE IF NOT EXISTS `group_role_binding` (
    `id`        bigint(20) NOT NULL AUTO_INCREMENT,
    `group_id`  varchar(64) NOT NULL COMMENT '用户组ID',
    `role_id`   bigint(20) NOT NULL COMMENT '角色ID',
    PRIMARY KEY (`id`),
    FOREIGN KEY (`group_id`) REFERENCES user_group(`group_id`),
    FOREIGN KEY (`role_id`) REFERENCES role(`id`)
) ENGINE = InnoDB CHARACTER SET = utf8 COLLATE = utf8_general_ci COMMENT = '角色组/角色绑定信息' ROW_FORMAT = Compact;

