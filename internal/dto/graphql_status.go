package dto

// GraphQLStatusResponse — результат GraphQL-запроса status.
type GraphQLStatusResponse struct {
	ReleaseVersion string `json:"releaseVersion"`
	TDLibVersion   string `json:"tdlibVersion"`
	UserID         int64  `json:"userId"`
}
