package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/gurbos/tcd/datastore"
	"github.com/gurbos/tcd/tcapi"
	"github.com/jackc/pgx/v5/pgconn"
)

const CARD_IMAGE_DIR = "/home/gurbos/card_images/" // Directory to store card images

func main() {

	cmdFlags := initCmdFlags()

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
		sets := tcapi.FetchSetsByProductLine(productLine.Name)

		if cmdFlags.write_data {
			pool, err := datastore.NewDBPool(context.Background(), config) // Create DB connection pool
			if err != nil {
				log.Fatal(fmt.Errorf("Error creating DB connection pool: %w", err))
			}
			defer pool.Close()
			store := datastore.NewPostgresDataStore(pool) // Create DataStore

			// Add Product Line to the database
			productLine, err = store.AddProductLine(context.Background(), productLine)
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				switch pgErr.Code {
				case datastore.UniqueViolationError:
					*productLine, err = store.GetProductLineByName(context.Background(), productLine.UrlName)
				default:
					log.Fatal(fmt.Errorf("Error adding Product Line: %w", err))
				}
			}

			// Associate sets with the product line and add to the database
			associateSetsWithProductLine(sets, productLine.Id)

			_, err = store.AddSets(context.Background(), sets)
			if errors.As(err, &pgErr) {
				switch pgErr.Code {
				case datastore.UniqueViolationError:
					// Sets already exist, proceed without error
				default:
					log.Fatal(fmt.Errorf("Error adding sets: %w", err))
				}
			}

			// Initialize worker pool configuration struct and launch worker pool
			maxProcs := runtime.GOMAXPROCS(0) / 3 // Determine number of workers to use
			wpConf := NewWorkerPoolConfig(
				context.Background(),
				maxProcs,                                   // pool size
				make(chan DataContext, maxProcs*10),        // data context channel
				make(chan Job, maxProcs*3),                 // job channel
				make(chan JobStatus, maxProcs*3),           // job status channel
				make(chan []datastore.Product, maxProcs*3), // image data request channel
				store,
			)

			// Launch the worker pool
			LaunchWorkerPool(wpConf)

			// Send data contexts to data context channel
			for _, set := range sets {
				sParams := tcapi.NewSearchParams(
					productLine.UrlName,
					set.UrlName,
					"Cards", 0,
					set.Count)
				dataCtx := DataContext{
					searchParams: sParams,
					set:          set,
					productLine:  *productLine,
				}
				wpConf.dataCtxChan <- dataCtx // Send data context to data context channel
			}

			close(wpConf.dataCtxChan)     // Close data context channel to signal data workers no more data contexts will be sent
			wpConf.dataWaitGroup.Wait()   // Wait for all data workers to finish
			close(wpConf.jobsChan)        // Close job channel to signal workers no more jobs will be sent
			wpConf.jobWaitGroup.Wait()    // Wait for all job workers to finish
			close(wpConf.jobStatChan)     // Close error channel to signal error worker no more errors will be sent
			wpConf.statusWaitGroup.Wait() // Wait for status worker to finish
			close(wpConf.imgInfoChan)     // Close image info channel to signal image worker no more image requests will be sent
			wpConf.imageWaitGroup.Wait()  // Wait for image worker to finish

			os.Exit(0)
		}

	}
}
