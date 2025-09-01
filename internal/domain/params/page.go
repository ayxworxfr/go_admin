package params

type Page struct {
	Offset int `query:"offset" vd:"$>=0"`
	Limit  int `query:"limit" vd:"$>=1 && $<=1000"`
}
