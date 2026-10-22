package listschemaversions

import "context"

type repository interface {
	Versions(ctx context.Context, slug string) ([]int, error)
}
