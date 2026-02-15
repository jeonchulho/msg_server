package httpresp

const (
	ErrUnauthorized               = "unauthorized"
	ErrInvalidCredentials         = "invalid credentials"
	ErrCannotUpdateOtherUserState = "cannot update another user's status"
	ErrFromMustBeRFC3339          = "from must use RFC3339 format"
	ErrToMustBeRFC3339            = "to must use RFC3339 format"
	ErrMissingBearerToken         = "bearer token is required"
	ErrInvalidToken               = "invalid token"
	ErrForbidden                  = "forbidden"
	ErrInsufficientRole           = "insufficient permissions"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type OKResponse struct {
	OK bool `json:"ok"`
}

type IDResponse struct {
	ID string `json:"id"`
}

type URLResponse struct {
	URL string `json:"url"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	UserID      string `json:"user_id"`
	TenantID    string `json:"tenant_id"`
	Role        string `json:"role"`
}

func NewErrorResponse(message string) ErrorResponse {
	return ErrorResponse{Error: message}
}

func NewOKResponse() OKResponse {
	return OKResponse{OK: true}
}

func NewIDResponse(id string) IDResponse {
	return IDResponse{ID: id}
}

func NewURLResponse(url string) URLResponse {
	return URLResponse{URL: url}
}

func NewTokenResponse(accessToken string, userID string, tenantID string, role string) TokenResponse {
	return TokenResponse{AccessToken: accessToken, UserID: userID, TenantID: tenantID, Role: role}
}
