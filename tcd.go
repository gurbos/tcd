package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"

	"github.com/gurbos/tcd/datastore"
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
	config := datastore.Config(creds.ConnectString())

	// Print product lines and exit if product-lines flag is set
	if cmdFlags.product_lines {
		pls := tcapi.FetchProductLines()
		printLists(pls)
		os.Exit(0)
	}

	if cmdFlags.product_line_name != "" {
		productLine := tcapi.FetchProductLineByName(cmdFlags.product_line_name)
		if productLine == nil {
			log.Fatalf("Product line '%s' not found", cmdFlags.product_line_name)
		}
		sets := tcapi.FetchSetsByProductLine(productLine.UrlName)

		if cmdFlags.write_data {
			pool, err := datastore.NewDBPool(context.Background(), config) // Create DB connection pool
			if err != nil {
				log.Fatal(fmt.Errorf("Error creating DB connection pool: %w", err))
			}
			defer pool.Close()
			store := datastore.NewPostgresDataStore(pool) // Create DataStore

			// Add Product Line to the database
			productLine, err = store.AddProductLine(context.Background(), productLine)
			if err != nil {
				log.Fatal(fmt.Errorf("Error adding Product Line: %w", err))
			}

			// Associate sets with the product line and add to the database
			associateSetsWithProductLine(sets, productLine.Id)
			if _, err := store.AddSets(context.Background(), sets); err != nil {
				log.Fatal(fmt.Errorf("Error adding sets: %w", err))
			}

			// Initialize worker pool configuration struct and launch worker pool
			maxProcs := runtime.GOMAXPROCS(0) / 2 // Determine number of workers to use
			wpConf := NewWorkerPoolConfig(
				context.Background(),
				maxProcs,
				make(chan DataContext, maxProcs), // data context channel
				make(chan Job, maxProcs),         // job channel
				make(chan JobStatus, maxProcs),   // job status channel
				store,
			)
			dataWaitGroup := &sync.WaitGroup{}
			jobWaitGroup := &sync.WaitGroup{}
			statusWaitGroup := &sync.WaitGroup{}

			// Launch job workers
			var i int
			for i = 1; i <= wpConf.poolSize; i++ {
				jobWaitGroup.Add(1)
				go jobWorker(i, context.Background(), wpConf.jobChan, wpConf.jobStatChan, jobWaitGroup, store)
			}

			// Launch data context workers
			var j int
			for j = i; j <= wpConf.poolSize*2; j++ {
				dataWaitGroup.Add(1)
				go dataWorker(j, wpConf.ctx, wpConf.dataCtxChan, wpConf.jobChan, dataWaitGroup)
			}

			// Launch status worker
			var k int
			for k = j; k <= wpConf.poolSize*2+2; k++ {
				statusWaitGroup.Add(1)
				go statusWorker(k, wpConf.ctx, wpConf.jobStatChan, wpConf.jobChan, statusWaitGroup)
			}

			// Send data contexts to data context channel
			for _, set := range sets {
				sParams := tcapi.NewSearchParams(productLine.UrlName, set.UrlName, "", 0, set.Count)
				dataCtx := DataContext{
					searchParams: sParams,
					set:          set,
					productLine:  *productLine,
				}
				wpConf.dataCtxChan <- dataCtx // Send data context to data context channel
			}

			close(wpConf.dataCtxChan) // Close data context channel to signal data workers no more data contexts will be sent
			dataWaitGroup.Wait()      // Wait for all data workers to finish
			close(wpConf.jobChan)     // Close job channel to signal workers no more jobs will be sent
			jobWaitGroup.Wait()       // Wait for all job workers to finish
			close(wpConf.jobStatChan) // Close error channel to signal error worker no more errors will be sent
			statusWaitGroup.Wait()    // Wait for status worker to finish

			os.Exit(0)
		}

	}
}
