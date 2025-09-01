package service

// Service 实例变量
var (
	AuthServiceInstance          *AuthService
	PermissionServiceInstance    *PermissionService
	SystemSettingServiceInstance *SystemSettingService
)

// dao层初始化完成后，调用Init函数
func Init() error {
	// 初始化核心服务
	PermissionServiceInstance = NewPermissionService()
	AuthServiceInstance = NewAuthService(PermissionServiceInstance)
	SystemSettingServiceInstance = NewSystemSettingService()

	return nil
}
