package datastore

import (
	"github.com/gurbos/tcd/tcapi"
)

type Product_Line struct {
	Id int
	tcapi.ValueType
}

type Set struct {
	Id            int
	ProductLineId int
	tcapi.ValueType
}
