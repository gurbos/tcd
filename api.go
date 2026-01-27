package main

import (
	"context"

	"github.com/gurbos/tcd/datastore"
	ds "github.com/gurbos/tcd/datastore"
)

type application struct {
	store UserDataStore
}

type UserDataStore interface {
	GetProductLineByName(ctx context.Context, name string) (ds.Product_Line, error)
	GetSetsByProductLineName(ctx context.Context, name string) ([]ds.Set, error)
	AddProductLine(ctx context.Context, pl *datastore.Product_Line) (*datastore.Product_Line, error)
	AddSets(ctx context.Context, sets []ds.Set) ([]datastore.Set, error)
	AddProducts(ctx context.Context, products []datastore.Product) error
}
