package learning

import "context"

type Pagination struct {
	Limit     int  `json:"limit"`
	HasMore   bool `json:"hasMore"`
	NextLimit int  `json:"nextLimit"`
}
type workspaceLimitKey struct{}

func WithWorkspaceLimit(ctx context.Context, limit int) context.Context {
	return context.WithValue(ctx, workspaceLimitKey{}, limit)
}
func workspaceLimit(ctx context.Context) int {
	limit, _ := ctx.Value(workspaceLimitKey{}).(int)
	if limit < 1 {
		return 500
	}
	return limit
}
func bounded[T any](items []T, pagination *Pagination) []T {
	if len(items) > pagination.Limit {
		pagination.HasMore = true
		return items[:pagination.Limit]
	}
	return items
}
