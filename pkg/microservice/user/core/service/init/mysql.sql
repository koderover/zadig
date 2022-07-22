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
) ENGINE = InnoDB AUTO_INCREMENT = 59 CHARACTER SET = utf8 COLLATE = utf8_general_ci COMMENT = '账号登陆表' ROW_FORMAT = Compact;

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
) ENGINE = InnoDB AUTO_INCREMENT = 59 CHARACTER SET = utf8 COLLATE = utf8_general_ci COMMENT = '用户信息表' ROW_FORMAT = Compact;