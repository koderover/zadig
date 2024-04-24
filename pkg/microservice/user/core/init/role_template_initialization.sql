INSERT INTO `role_template` (name, description, type) VALUES
("project-admin", "拥有执行项目中任何操作的权限", 1),
("read-only", "拥有指定项目中所有资源的读权限", 1),
("read-project-only", "拥有指定项目本身的读权限，无权限查看和操作项目内资源", 1)
on duplicate key update id=id;
