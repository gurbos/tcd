package main

import (
	ds "github.com/gurbos/tcd/datastore"
)

type application struct {
	store UserDataStore
}

type UserDataStore interface {
	GetProductLineByName(name string) (*ds.Product_Line, error)
	GetSetsByProductLineName(name string) ([]ds.Set, error)
	UpdateSets(sets []ds.Set) error
	//UpdateProducts(products []data.Product) error
}
