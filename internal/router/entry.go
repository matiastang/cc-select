// entry.go 定义请求上下文里传递的路由条目（鉴权中间件 → 转发层）。
package router

import (
	"context"

	"github.com/cc-select/cc-select/internal/routes"
)

type entryCtxKey struct{}

// withEntry 把命中的路由条目挂进请求上下文。
func withEntry(ctx context.Context, e routes.Entry) context.Context {
	return context.WithValue(ctx, entryCtxKey{}, e)
}

// entryFrom 取出请求对应的路由条目；不存在返回零值（仅理论可能：未经 auth 中间件）。
func entryFrom(ctx context.Context) routes.Entry {
	if e, ok := ctx.Value(entryCtxKey{}).(routes.Entry); ok {
		return e
	}
	return routes.Entry{}
}
