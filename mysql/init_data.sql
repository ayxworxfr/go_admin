-- 初始化数据
-- 权限 path 必须与 @route 注册路径一致（最终为 /api/protected + 相对路径）。
-- 鉴权只认 method+path 非空的记录（type=3 接口权限）；type=1 菜单 path 供前端树展示。

-- 插入默认用户 (密码: admin123，使用 Argon2id 加密，自描述格式见 pkg/crypter.Argon2Hasher)
INSERT INTO `user` (`id`, `username`, `password_hash`, `email`, `phone`, `status`) VALUES
(1, 'admin', '$argon2id$v=19$m=65536,t=3,p=2$l2/H7PBMkR7oLTQDFl/QYA$d8zUw8p/xnrrHTe2iFjpjVt+x0+Pzgq0GMNSHurHWuc', 'admin@example.com', '13800000000', 1),
(2, 'demo', '$argon2id$v=19$m=65536,t=3,p=2$sRZMfGnjdrKH9nEW4Dmqsw$dRlNLutxEzDm4z0wKJvW2kzv2sB0ZyxdKwX/IAse/mw', 'demo@example.com', '13800000001', 1);

-- 插入默认角色
INSERT INTO `role` (`id`, `name`, `code`, `description`, `status`) VALUES
(1, '系统管理员', 'ADMIN', '系统超级管理员，拥有所有权限', 1),
(2, '普通用户', 'USER', '普通用户角色，基础权限', 1);

-- 插入基础权限
INSERT INTO `permission` (`id`, `name`, `code`, `description`, `parent_id`, `type`, `path`, `method`, `status`) VALUES
-- 菜单（type=1）：path 与模块 API 前缀对齐，method 为空不参与接口鉴权
(1, '系统管理', 'SYSTEM', '系统管理模块', 0, 1, '/api/protected', '', 1),
(2, '用户管理', 'USER_MANAGE', '用户管理', 1, 1, '/api/protected/user', '', 1),
(3, '角色管理', 'ROLE_MANAGE', '角色管理', 1, 1, '/api/protected/role', '', 1),
(4, '权限管理', 'PERMISSION_MANAGE', '权限管理', 1, 1, '/api/protected/permission', '', 1),
(5, '系统设置', 'SYSTEM_SETTING', '系统设置', 1, 1, '/api/protected/system-setting', '', 1),
-- 用户管理接口（type=3）↔ @route /user*
(10, '查看用户', 'USER_VIEW', '查看用户列表', 2, 3, '/api/protected/user/*', 'GET', 1),
(11, '创建用户', 'USER_CREATE', '创建新用户', 2, 3, '/api/protected/user/*', 'POST', 1),
(12, '编辑用户', 'USER_UPDATE', '编辑用户信息', 2, 3, '/api/protected/user/*', 'PUT', 1),
(13, '删除用户', 'USER_DELETE', '删除用户', 2, 3, '/api/protected/user/*', 'DELETE', 1),
-- 角色管理接口 ↔ @route /role*
(20, '查看角色', 'ROLE_VIEW', '查看角色列表', 3, 3, '/api/protected/role/*', 'GET', 1),
(21, '创建角色', 'ROLE_CREATE', '创建新角色', 3, 3, '/api/protected/role/*', 'POST', 1),
(22, '编辑角色', 'ROLE_UPDATE', '编辑角色信息', 3, 3, '/api/protected/role/*', 'PUT', 1),
(23, '删除角色', 'ROLE_DELETE', '删除角色', 3, 3, '/api/protected/role/*', 'DELETE', 1),
-- 权限管理接口 ↔ @route /permission*
(30, '查看权限', 'PERMISSION_VIEW', '查看权限列表', 4, 3, '/api/protected/permission/*', 'GET', 1),
(31, '创建权限', 'PERMISSION_CREATE', '创建新权限', 4, 3, '/api/protected/permission/*', 'POST', 1),
(32, '编辑权限', 'PERMISSION_UPDATE', '编辑权限信息', 4, 3, '/api/protected/permission/*', 'PUT', 1),
(33, '删除权限', 'PERMISSION_DELETE', '删除权限', 4, 3, '/api/protected/permission/*', 'DELETE', 1),
-- 系统设置接口 ↔ @route /system-setting*（注意是连字符，不是 /system/setting）
(40, '查看设置', 'SETTING_VIEW', '查看系统设置', 5, 3, '/api/protected/system-setting/*', 'GET', 1),
(41, '创建设置', 'SETTING_CREATE', '创建系统设置', 5, 3, '/api/protected/system-setting/*', 'POST', 1),
(42, '修改设置', 'SETTING_UPDATE', '修改系统设置', 5, 3, '/api/protected/system-setting/*', 'PUT', 1),
(43, '删除设置', 'SETTING_DELETE', '删除系统设置', 5, 3, '/api/protected/system-setting/*', 'DELETE', 1),
-- 个人信息（预留菜单；当前无独立 /profile 路由时不影响鉴权）
(100, '个人信息', 'PROFILE', '查看和修改个人信息', 0, 1, '/api/protected/user/current', '', 1),
(101, '查看个人信息', 'PROFILE_VIEW', '查看个人信息', 100, 3, '/api/protected/user/current', 'GET', 1),
(102, '修改个人信息', 'PROFILE_UPDATE', '修改个人信息', 100, 3, '/api/protected/user/*', 'PUT', 1);

-- 分配用户角色
INSERT INTO `user_role` (`user_id`, `role_id`) VALUES
(1, 1), -- admin用户分配系统管理员角色
(2, 2); -- demo用户分配普通用户角色

-- 分配角色权限
-- 系统管理员拥有所有权限
INSERT INTO `role_permission` (`role_id`, `permission_id`) 
SELECT 1, `id` FROM `permission` WHERE `status` = 1;

-- 普通用户只有基础权限
INSERT INTO `role_permission` (`role_id`, `permission_id`) VALUES
(2, 100), (2, 101), (2, 102); -- 个人信息相关权限

-- 插入默认系统设置
INSERT INTO `system_setting` (`category`, `key`, `value`, `type`, `description`, `create_by`) VALUES
('basic', 'system_name', 'Go Admin Scaffold', 1, '系统名称', 1),
('basic', 'system_version', '1.0.0', 1, '系统版本', 1),
('basic', 'page_size', '100', 2, '默认分页大小', 1),
('security', 'session_timeout', '7200', 2, '会话超时时间(秒)', 1),
('security', 'password_policy', '{"min_length": 8, "require_special": true}', 4, '密码策略配置', 1);
