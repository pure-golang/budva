package graphql

// StatusResponse — результат запроса status.
type StatusResponse struct {
	TDLibVersion string `json:"tdlibVersion"`
	UserID       int64  `json:"userId"`
}
