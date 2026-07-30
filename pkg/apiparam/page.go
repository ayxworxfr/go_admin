package apiparam

// Page 分页请求参数，供各模块的列表请求 DTO 组合复用。
type Page struct {
	Offset int `query:"offset" vd:"$>=0"`
	Limit  int `query:"limit" vd:"$>=1 && $<=1000"`
}
