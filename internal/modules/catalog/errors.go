package catalog

import "errors"

var (
	ErrInvalidProductInput = errors.New("catalog invalid product input")
	ErrProductSKUExists    = errors.New("catalog product sku exists")
	ErrProductNotFound     = errors.New("catalog product not found")
)
