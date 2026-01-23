package main

import (
	"context"
	"log"
	"os"

	ds "github.com/gurbos/tcd/datastore"
	"github.com/gurbos/tcd/tcapi"
	"github.com/joho/godotenv"
)

func main() {

	cmdFlags := initCmdFlags()

	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	// Load DB credentials from environment variables
	var creds DBCredentials
	creds.LoadCredentials()
	config := ds.Config(creds.ConnectString())

	if cmdFlags.product_lines {
		pls := tcapi.FetchProductLines()
		printLists(pls)
		os.Exit(0)
	}

	if cmdFlags.sets {
		if cmdFlags.product_line_id >= 1 {
			pool, err := ds.NewDBPool(context.Background(), config)
			if err != nil {
				log.Fatal("Error creating DB connection pool: %w", err)
			}
			defer pool.Close()

			store := ds.NewPostgresDataStore(pool)

			index := cmdFlags.product_line_id - 1
			pls := tcapi.FetchProductLines()
			sets := toSets(
				tcapi.FetchSetsByProductLine(pls[index].UrlName),
				1,
			)
			if err := store.UpdateSets(sets); err != nil {

				log.Fatal("Error updating Sets: %w", err)
			}
			os.Exit(0)
		}
	}
}
