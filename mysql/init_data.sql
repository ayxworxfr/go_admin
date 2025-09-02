-- 初始化数据

-- 插入默认用户 (密码: admin123，使用bcrypt加密)
INSERT INTO `user` (`id`, `username`, `password`, `email`, `phone`, `status`) VALUES
(1, 'admin', 'ad298ae64eadb2c73cebe7c059139d4cb22069f93a237d978b195f81803f3a1e40aa8677cdffa3835a8f64db2678920e', 'admin@example.com', '13800000000', 1),
(2, 'demo', 'ad298ae64eadb2c73cebe7c059139d4cb22069f93a237d978b195f81803f3a1e40aa8677cdffa3835a8f64db2678920e', 'demo@example.com', '13800000001', 1);

-- 插入默认角色
INSERT INTO `role` (`id`, `name`, `code`, `description`, `status`) VALUES
(1, '系统管理员', 'ADMIN', '系统超级管理员，拥有所有权限', 1),
(2, '普通用户', 'USER', '普通用户角色，基础权限', 1);

-- 插入基础权限
INSERT INTO `permission` (`id`, `name`, `code`, `description`, `parent_id`, `type`, `path`, `method`, `status`) VALUES
-- 系统管理
(1, '系统管理', 'SYSTEM', '系统管理模块', 0, 1, '/api/protected/system', '', 1),
(2, '用户管理', 'USER_MANAGE', '用户管理', 1, 1, '/api/protected/system/user', '', 1),
(3, '角色管理', 'ROLE_MANAGE', '角色管理', 1, 1, '/api/protected/system/role', '', 1),
(4, '权限管理', 'PERMISSION_MANAGE', '权限管理', 1, 1, '/api/protected/system/permission', '', 1),
(5, '系统设置', 'SYSTEM_SETTING', '系统设置', 1, 1, '/api/protected/system/setting', '', 1),
-- 用户管理权限
(10, '查看用户', 'USER_VIEW', '查看用户列表', 2, 3, '/api/protected/user', 'GET', 1),
(11, '创建用户', 'USER_CREATE', '创建新用户', 2, 3, '/api/protected/user', 'POST', 1),
(12, '编辑用户', 'USER_UPDATE', '编辑用户信息', 2, 3, '/api/protected/user/*', 'PUT', 1),
(13, '删除用户', 'USER_DELETE', '删除用户', 2, 3, '/api/protected/user/*', 'DELETE', 1),
-- 角色管理权限
(20, '查看角色', 'ROLE_VIEW', '查看角色列表', 3, 3, '/api/protected/role', 'GET', 1),
(21, '创建角色', 'ROLE_CREATE', '创建新角色', 3, 3, '/api/protected/role', 'POST', 1),
(22, '编辑角色', 'ROLE_UPDATE', '编辑角色信息', 3, 3, '/api/protected/role/*', 'PUT', 1),
(23, '删除角色', 'ROLE_DELETE', '删除角色', 3, 3, '/api/protected/role/*', 'DELETE', 1),
-- 权限管理权限
(30, '查看权限', 'PERMISSION_VIEW', '查看权限列表', 4, 3, '/api/protected/permission', 'GET', 1),
(31, '创建权限', 'PERMISSION_CREATE', '创建新权限', 4, 3, '/api/protected/permission', 'POST', 1),
(32, '编辑权限', 'PERMISSION_UPDATE', '编辑权限信息', 4, 3, '/api/protected/permission/*', 'PUT', 1),
(33, '删除权限', 'PERMISSION_DELETE', '删除权限', 4, 3, '/api/protected/permission/*', 'DELETE', 1),
-- 系统设置权限
(40, '查看设置', 'SETTING_VIEW', '查看系统设置', 5, 3, '/api/protected/system/setting', 'GET', 1),
(41, '修改设置', 'SETTING_UPDATE', '修改系统设置', 5, 3, '/api/protected/system/setting', 'PUT', 1),
-- 基础权限
(100, '个人信息', 'PROFILE', '查看和修改个人信息', 0, 1, '/api/protected/profile', '', 1),
(101, '查看个人信息', 'PROFILE_VIEW', '查看个人信息', 100, 3, '/api/protected/profile', 'GET', 1),
(102, '修改个人信息', 'PROFILE_UPDATE', '修改个人信息', 100, 3, '/api/protected/profile', 'PUT', 1);

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
