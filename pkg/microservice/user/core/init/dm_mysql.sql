CREATE TABLE IF NOT EXISTS user_login(
    id bigint NOT NULL AUTO_INCREMENT,
    uid varchar(64) NOT NULL DEFAULT '0' COMMENT '用户id',
    login_id varchar(64) NOT NULL DEFAULT '0' COMMENT '用户登录id,如账号名',
    login_type int NOT NULL DEFAULT '0' COMMENT '登录类型,0.账号名',
    password varchar(64) DEFAULT '' COMMENT '密码',
    last_login_time int NOT NULL DEFAULT '0' COMMENT '最后登陆时间',
    created_at int NOT NULL DEFAULT '0' COMMENT '创建时间',
    updated_at int NOT NULL DEFAULT '0' COMMENT '更新时间',
    PRIMARY KEY (id)
) ;

CREATE INDEX IF NOT EXISTS idx_uid ON user_login(uid);

CREATE UNIQUE INDEX IF NOT EXISTS "login" ON user_login(uid,login_id,login_type);

CREATE TABLE IF NOT EXISTS "user"(
    uid varchar(64) NOT NULL COMMENT '用户ID',
    account varchar(32) NOT NULL DEFAULT '' COMMENT '用户账号',
    name varchar(32) NOT NULL DEFAULT '' COMMENT '用户名',
    identity_type varchar(32) NOT NULL DEFAULT 'unknown' COMMENT '用户来源',
    phone varchar(16) NOT NULL DEFAULT '' COMMENT '手机号码',
    email varchar(100) NOT NULL DEFAULT '' COMMENT '邮箱',
    api_token varchar(1024) NOT NULL DEFAULT '' COMMENT 'openAPIToken',
    created_at int NOT NULL COMMENT '创建时间',
    updated_at int NOT NULL COMMENT '修改时间',
    PRIMARY KEY (uid)
) ;

CREATE UNIQUE INDEX IF NOT EXISTS account ON "user"(account,identity_type);

CREATE TABLE IF NOT EXISTS user_group (
    group_id    varchar(64) NOT NULL COMMENT '用户组ID',
    group_name  varchar(32) NOT NULL UNIQUE COMMENT '用户组名',
    description varchar(64) NOT NULL COMMENT '简介',
    type        int NOT NULL COMMENT '资源范围，1-系统自带， 2-用户自定义',
    created_at  int NOT NULL DEFAULT '0' COMMENT '创建时间',
    updated_at  int NOT NULL DEFAULT '0' COMMENT '更新时间',
    PRIMARY KEY (group_id)
) ;

CREATE TABLE IF NOT EXISTS group_binding (
    id bigint NOT NULL AUTO_INCREMENT,
    group_id varchar(64) NOT NULL COMMENT '用户组ID',
    uid varchar(64) NOT NULL COMMENT '用户ID',
    PRIMARY KEY (id),
    FOREIGN KEY (uid) REFERENCES "user"(uid) ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES user_group(group_id) ON DELETE CASCADE
) ;

CREATE TABLE IF NOT EXISTS action (
    id       bigint NOT NULL AUTO_INCREMENT,
    name     varchar(32) NOT NULL COMMENT '权限项名称',
    action   varchar(32) NOT NULL COMMENT '权限项',
    resource varchar(32) NOT NULL COMMENT '资源类别',
    scope    int NOT NULL COMMENT '资源范围，1-项目范围， 2-全局',
    PRIMARY KEY (id)
) ;

CREATE UNIQUE INDEX IF NOT EXISTS resource_action ON action(action,resource);

CREATE TABLE IF NOT EXISTS role (
    id          bigint NOT NULL AUTO_INCREMENT,
    name        varchar(32) NOT NULL COMMENT '角色名称',
    description varchar(64) NOT NULL COMMENT '描述',
    type        int NOT NULL COMMENT '资源范围，1-系统自带， 2-用户自定义',
    namespace   varchar(32) NOT NULL COMMENT '所属项目，*为全局角色标记',
    PRIMARY KEY (id)
) ;

CREATE UNIQUE INDEX IF NOT EXISTS namespaced_role ON role(namespace, name);

CREATE TABLE IF NOT EXISTS role_template (
    id          bigint NOT NULL AUTO_INCREMENT,
    name        varchar(32) NOT NULL COMMENT '角色名称',
    description varchar(64) NOT NULL COMMENT '描述',
    PRIMARY KEY (id)
) ;

CREATE UNIQUE INDEX IF NOT EXISTS name_role_template ON role_template(name);

CREATE TABLE IF NOT EXISTS role_action_binding (
    id        bigint NOT NULL AUTO_INCREMENT,
    action_id bigint NOT NULL COMMENT '用户组ID',
    role_id   bigint NOT NULL COMMENT '角色ID',
    PRIMARY KEY (id),
    FOREIGN KEY (action_id) REFERENCES action(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES role(id) ON DELETE CASCADE
) ;

CREATE TABLE IF NOT EXISTS role_template_action_binding (
    id                 bigint   NOT NULL AUTO_INCREMENT,
    action_id          bigint   NOT NULL COMMENT '用户组ID',
    role_template_id   bigint NOT NULL COMMENT '全局角色ID',
    PRIMARY KEY (id),
    FOREIGN KEY (action_id) REFERENCES action(id) ON DELETE CASCADE,
    FOREIGN KEY (role_template_id) REFERENCES role_template(id) ON DELETE CASCADE
) ;

CREATE TABLE IF NOT EXISTS role_binding (
    id        bigint NOT NULL AUTO_INCREMENT,
    uid   varchar(64) NOT NULL COMMENT '用户ID',
    role_id   bigint NOT NULL COMMENT '角色ID',
    PRIMARY KEY (id),
    FOREIGN KEY (uid) REFERENCES "user"(uid) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES role(id) ON DELETE CASCADE
) ;

CREATE TABLE IF NOT EXISTS group_role_binding (
    id        bigint NOT NULL AUTO_INCREMENT,
    group_id  varchar(64) NOT NULL COMMENT '用户组ID',
    role_id   bigint NOT NULL COMMENT '角色ID',
    PRIMARY KEY (id),
    FOREIGN KEY (group_id) REFERENCES user_group(group_id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES role(id) ON DELETE CASCADE
) ;

CREATE TABLE IF NOT EXISTS role_template_binding (
    id                 bigint NOT NULL AUTO_INCREMENT,
    role_id            varchar(64) NOT NULL COMMENT '角色ID',
    role_template_id   bigint NOT NULL COMMENT '全局角色ID',
    PRIMARY KEY (id),
    FOREIGN KEY (role_id) REFERENCES role(id) ON DELETE CASCADE,
    FOREIGN KEY (role_template_id) REFERENCES role_template(id) ON DELETE CASCADE
) ;