package middleware

import "context"

// Context keys for storing values in request context
type contextKey string

const (
	// ContextKeyUser is the context key for storing user information
	ContextKeyUser contextKey = "user"

	// ContextKeyAccessToken is the context key for storing the access token
	ContextKeyAccessToken contextKey = "access_token"
)

// Helper functions for context operations

// SetUserInContext adds user information to the context
func SetUserInContext(ctx context.Context, userInfo *UserInfo) context.Context {
	return context.WithValue(ctx, ContextKeyUser, userInfo)
}

// GetUserFromContext retrieves user information from the context
func GetUserFromContext(ctx context.Context) *UserInfo {
	if user := ctx.Value(ContextKeyUser); user != nil {
		if userInfo, ok := user.(*UserInfo); ok {
			return userInfo
		}
	}
	return nil
}

// SetAccessTokenInContext adds access token to the context
func SetAccessTokenInContext(ctx context.Context, accessToken string) context.Context {
	return context.WithValue(ctx, ContextKeyAccessToken, accessToken)
}

// GetAccessTokenFromContext retrieves access token from the context
func GetAccessTokenFromContext(ctx context.Context) string {
	if token := ctx.Value(ContextKeyAccessToken); token != nil {
		if tokenStr, ok := token.(string); ok {
			return tokenStr
		}
	}
	return ""
}
