package datastore

import "encoding/json"

/* This package contains data types that map to the database schema */

type Product_Line struct {
	Id      int
	Name    string
	UrlName string
}

type Set struct {
	Id            int
	Name          string
	UrlName       string
	Count         int
	ReleaseDate   string
	ProductLineId int
}

type Product struct {
	ProductId          int
	ProductLineName    string          `json:"productLineName"`
	ProductLineUrlName string          `json:"productLineUrlName"`
	ProductName        string          `json:"productName"`
	ProductUrlName     string          `json:"productUrlName"`
	CustomAttributes   json.RawMessage `json:"customAttributes"`
	SetName            string          `json:"setName"`
	SetUrlName         string          `json:"setUrlName"`
	RarityName         string          `json:"rarityName"`
	ProductNumber      string
	PrintEdition       string
	ReleaseDate        string
	ProductLineId      int
	SetId              int
}
