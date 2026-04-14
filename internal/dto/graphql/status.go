package graphql

// StatusResponse — результат запроса status.
type StatusResponse struct {
	ReleaseVersion string `json:"releaseVersion"`
	TDLibVersion   string `json:"tdlibVersion"`
	UserID         int64  `json:"userId"`
}
