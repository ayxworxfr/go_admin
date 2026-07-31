-- 03-init_user.sql：收紧 go_user 权限（账号由 MYSQL_USER/MYSQL_PASSWORD 创建）
-- 不在此 CREATE/IDENTIFIED，避免覆盖 .env 密码。
-- 不对 information_schema 做 GRANT：MySQL 8 初始化阶段会报 Access denied 并中断 entrypoint。

-- 先收回入口脚本默认的 ALL，再授予最小业务权限
REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'go_user'@'%';

GRANT USAGE ON *.* TO 'go_user'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE, SHOW VIEW ON go_admin.* TO 'go_user'@'%';

FLUSH PRIVILEGES;

SHOW GRANTS FOR 'go_user'@'%';
