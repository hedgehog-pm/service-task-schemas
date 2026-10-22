// Package topsort упорядочивает значения по их зависимостям.
package topsort

import (
	"errors"
	"fmt"
)

var (
	ErrDuplicateKey      = errors.New("duplicate key")
	ErrMissingDependency = errors.New("missing dependency")
	ErrCycle             = errors.New("dependency cycle")
)

// Sort выполняет топологическую сортировку элементов по зависимостям, заданным через dependencies.
// Возвращает все элементы в новом срезе, размещая зависимости перед зависимыми элементами.
// При цикле возвращает nil и ErrCycle, при других ошибках — nil и соответствующую ошибку.
// Сложность по времени и памяти — O(V + E), где V — число элементов, E — число зависимостей.
func Sort[T any, K comparable](
	items []T,
	key func(T) K,
	dependencies func(T) []K,
) ([]T, error) {
	indices := make(map[K]int, len(items))
	keys := make([]K, len(items))

	for i, item := range items {
		id := key(item)
		if _, exists := indices[id]; exists {
			return nil, fmt.Errorf("%w: %v", ErrDuplicateKey, id)
		}

		indices[id] = i
		keys[i] = id
	}

	dependents, pending, err := buildEdges(items, keys, indices, dependencies)
	if err != nil {
		return nil, err
	}

	ready := make([]int, 0, len(items))

	for i, count := range pending {
		if count == 0 {
			ready = append(ready, i)
		}
	}

	result := make([]T, 0, len(items))
	for head := 0; head < len(ready); head++ {
		i := ready[head]
		result = append(result, items[i])

		for _, dependent := range dependents[i] {
			pending[dependent]--
			if pending[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
	}

	if len(result) != len(items) {
		return nil, ErrCycle
	}

	return result, nil
}

func buildEdges[T any, K comparable](
	items []T,
	keys []K,
	indices map[K]int,
	dependencies func(T) []K,
) ([][]int, []int, error) {
	dependents := make([][]int, len(items))
	pending := make([]int, len(items))

	for i, item := range items {
		for _, dependency := range dependencies(item) {
			parent, exists := indices[dependency]
			if !exists {
				return nil, nil, fmt.Errorf(
					"item %v: %w: %v",
					keys[i],
					ErrMissingDependency,
					dependency,
				)
			}

			dependents[parent] = append(dependents[parent], i)
			pending[i]++
		}
	}

	return dependents, pending, nil
}
