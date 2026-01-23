package main

import (
	"fmt"
	"os"

	"github.com/gurbos/tcd/datastore"
	"github.com/gurbos/tcd/tcapi"
	"github.com/spf13/pflag"
)

type DBCredentials struct {
	username string
	password string
	host     string
	port     string
	dbName   string
}

// LoadCredentials loads database credentials from environment variables.
func (cred *DBCredentials) LoadCredentials() {
	cred.username = os.Getenv("USERNAME")
	cred.password = os.Getenv("PASSWORD")
	cred.host = os.Getenv("HOST")
	cred.port = os.Getenv("PORT")
	cred.dbName = os.Getenv("DB_NAME")
}

// ConnectString constructs a PostgreSQL connection string from the credentials.

func (cred *DBCredentials) ConnectString() string {
	return "postgres://" + cred.username + ":" + cred.password + "@" + cred.host +
		":" + cred.port + "/" + cred.dbName
}

// ConnectStringer defines an interface for types that can provide a database connection string.
type ConnectStringer interface {
	ConnectString() string
}

// ToSets converts a slice of data.ValueType to a slice of datastore.Set, associating them with the given product line ID.
func toSets(setsData []tcapi.ValueType, productLineId int) (sets []datastore.Set) {
	sets = make([]datastore.Set, len(setsData))
	for i, elem := range setsData {
		sets[i].Name = elem.Name
		sets[i].UrlName = elem.UrlName
		sets[i].Count = elem.Count
		sets[i].ProductLineId = productLineId
	}
	return sets
}

func printLists(list []tcapi.ValueType) {
	mid := len(list) / 2
	for i := 0; i < mid; i++ {
		fmt.Printf("%-4d : %-60s %-5s %-4d : %-60s\n", i+1, list[i].UrlName, "  ", i+mid+1, list[i+mid].UrlName)
	}
	if len(list)%2 != 0 {
		fmt.Printf("%-65s%-60s\n", " ", list[len(list)-1].UrlName)
	}
}

type cmd_flags struct {
	product_lines   bool
	sets            bool
	store_data      bool
	product_line_id int
	pl              string
}

func initCmdFlags() *cmd_flags {
	var flags cmd_flags
	pflag.BoolVarP(&flags.product_lines, "product-lines", "p", false, "Fetch all product lines from the data source")
	pflag.BoolVarP(&flags.sets, "sets", "s", false, "Specify sets as target data")
	pflag.IntVarP(&flags.product_line_id, "product-line-id", "", 0, "Product line ID to fetch sets for")
	pflag.BoolVarP(&flags.store_data, "write-db", "", false, "Write product line products and sets to the database")
	pflag.StringVarP(&flags.pl, "pl", "", "yugioh", "Product line to fetch sets for")
	pflag.Parse()
	return &flags
}

/*func writeSets(sets []datastore.Set, store UserDataStore) error {
	if err := store.UpdateSets(sets); err != nil {
		return fmt.Errorf("Error writing sets to datastore: %w", err)
	}
	return nil
}*/
