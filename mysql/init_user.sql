-- 03-init_user.sql：授予 go_admin 所有表的增删改查权限
-- 1. 先删除旧用户（避免重复创建报错，首次执行可跳过但保留更安全）
DROP USER IF EXISTS 'go_user'@'%';

-- 2. 创建用户（使用 MySQL 8.0 默认插件 caching_sha2_password，消除过时警告）
CREATE USER 'go_user'@'%' 
IDENTIFIED BY 'go_user123';  -- 密码与 docker-compose 中配置一致

-- 3. 关键1：授予「访问 go_admin 数据库的基础权限」（必须，否则连 USE 数据库都报错）
-- USAGE 权限：仅允许“使用该数据库”，无其他额外权限，符合最小权限原则
GRANT USAGE ON go_admin.* TO 'go_user'@'%';

-- 4. 关键2：授予「go_admin 所有表的增删改查权限」
-- go_admin.* 表示“go_admin 数据库下的所有表”
GRANT SELECT, INSERT, UPDATE, DELETE ON go_admin.* TO 'go_user'@'%';

-- 5. 可选：授予「查看表结构的权限」（兼容 ORM 框架，如 Gorm/MyBatis）
-- SHOW VIEW：允许查看视图结构
-- INFORMATION_SCHEMA.COLUMNS：允许获取表字段信息（ORM 自动映射实体类需此权限）
GRANT SHOW VIEW ON go_admin.* TO 'go_user'@'%';
GRANT SELECT ON INFORMATION_SCHEMA.COLUMNS TO 'go_user'@'%';

-- 6. 刷新权限缓存，确保所有授权立即生效（必须执行，否则权限不更新）
FLUSH PRIVILEGES;

-- 7. 验证：查看 go_user 的完整权限（确认授权正确）
SHOW GRANTS FOR 'go_user'@'%';