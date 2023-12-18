package proxy

import "context"

// private type creates an interface key for Context that cannot be accessed by any other package
type contextKey int

const (
	// position counter of the TX in the block
	contextMmodelName contextKey = iota
)

// WithModelName sets the model name in the provided context and returns a new context with the updated value.
// The model name can be retrieved using the contextMmodelName key.
func WithModelName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, contextMmodelName, name)
}

// ModelName retrieves the model name from the provided context. It returns
// the model name as a string and a boolean value indicating whether the
// model name was found in the context or not.
func ModelName(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(contextMmodelName).(string)
	return val, ok
}
