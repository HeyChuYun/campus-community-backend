package privacy

const (
	ScopeDM   = "dm"
	ScopePost = "post"
)

type BlacklistStatus struct {
	DMBlocked   bool
	PostBlocked bool
}

type PostSettings struct {
	AllowComment bool
}
