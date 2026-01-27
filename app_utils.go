package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/gurbos/tcd/datastore"
	"github.com/gurbos/tcd/tcapi"
	"github.com/jackc/pgx/v5/pgconn"
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

// printLists prints a formatted list of ValueType items in two columns.
func printLists(list []tcapi.ValueType) {
	mid := len(list) / 2
	for i := 0; i < mid; i++ {
		fmt.Printf("%-4d : %-60s %-5s %-4d : %-60s\n", i, list[i].UrlName, "  ", i+mid, list[i+mid].UrlName)
	}
	if len(list)%2 != 0 {
		fmt.Printf("%-65s%-60s\n", " ", list[len(list)-1].UrlName)
	}
}

type cmd_flags struct {
	product_lines     bool
	product_line_name string
	sets              bool
	write_data        bool
	pl                string
}

func initCmdFlags() *cmd_flags {
	var flags cmd_flags
	pflag.BoolVarP(&flags.product_lines, "product-lines", "p", false, "Fetch all product lines from the data source")
	pflag.BoolVarP(&flags.sets, "sets", "s", false, "Specify sets as target data")
	pflag.StringVarP(&flags.product_line_name, "product-line-name", "n", "", "Product line name to process data for")
	pflag.BoolVarP(&flags.write_data, "write-data", "", false, "Write product line products and sets to the database")
	pflag.StringVarP(&flags.pl, "pl", "", "yugioh", "Product line to fetch sets for")
	pflag.Parse()
	return &flags
}

func associateSetsWithProductLine(sets []datastore.Set, productLineId int) {
	for i := 0; i < len(sets); i++ {
		sets[i].ProductLineId = productLineId
	}
}

func assocProductsWithSetAndProductLine(products []datastore.Product, setId int, productLineId int) {
	for i := 0; i < len(products); i++ {
		products[i].SetId = setId
		products[i].ProductLineId = productLineId
	}
}

// screenProducts removes products without a ProductNumber and eliminates duplicates.
func screenProducts(producsts []datastore.Product) []datastore.Product {
	products := removeProductWithoutNumber(producsts)
	products = removeDuplicateProducts(products)
	return products
}

// removeDuplicateProducts removes duplicate products based on ProductNumber.
func removeDuplicateProducts(products []datastore.Product) []datastore.Product {
	seen := make(map[string]struct{})
	unique := []datastore.Product{}
	for _, p := range products {
		if _, exists := seen[p.ProductNumber]; !exists {
			seen[p.ProductNumber] = struct{}{}
			unique = append(unique, p)
		}
	}
	return unique
}

// removeProductWithoutNumber filters out products that do not have a ProductNumber.
func removeProductWithoutNumber(products []datastore.Product) []datastore.Product {
	var filtered []datastore.Product
	for _, p := range products {
		if p.ProductNumber != "" {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// removeProductByProductNumber removes a product with the specified ProductNumber from the list.
func removeProductByProductNumber(products []datastore.Product, number string) []datastore.Product {
	filtered := make([]datastore.Product, len(products)-1)
	var i int // index of element to remove
	for i = 0; i < len(products); i++ {
		if products[i].ProductNumber != number {
			filtered = append(filtered, products[i])
		}
	}
	return filtered
}

// dataWorker fetches products, based search parameters sent via the data context channel, from
// the TCGPlayer API, initializes a jobs with the fetched products, and sends the jobs, via the jobs channel,
// to the job workers for processing.
func dataWorker(id int, ctx context.Context, dcChan <-chan DataContext, jobsChan chan<- Job, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		dc, open := <-dcChan
		if !open {
			log.Printf("Data Worker %d: No more data contexts to process. Exiting.\n", id)
			return
		}
		products := tcapi.FetchProductsInParts(dc.searchParams) // Fetch products based on search parameters
		products = screenProducts(products)                     // Screen products to remove those without ProductNumber and duplicates
		dc.UpdateSetCount(len(products))                        // Update set count with number of products after screening
		dc.UpdateSearchResultsSize(len(products))               // Update set count with number of products after screening
		assocProductsWithSetAndProductLine(products, dc.set.Id, dc.productLine.Id)
		job := NewJob(dc.productLine, dc.set, products)
		jobsChan <- job
	}
}

// jobWorker processes jobs, received via the jobs channel, and adds them to the database using the
// provided UserDataStore. It reports job status, via the job status channel, to the status worker.
func jobWorker(id int, ctx context.Context, jobsChan <-chan Job, statChan chan<- JobStatus, wg *sync.WaitGroup, store UserDataStore) {
	defer wg.Done()

	// Process jobs from the jobs channel
	for {
		job, open := <-jobsChan
		// End worker if jobs channel is closed
		if !open {
			log.Printf("Job Worker %d: No more jobs to process. Exiting.\n", id)
			return
		}

		jobStatus := JobStatus{job: job}               // Initialize job status
		err := store.AddProducts(ctx, job.productList) // attempt to add products to the database
		if err != nil {
			jobStatus.success = false // Mark job as failed
		} else {
			jobStatus.success = true // Mark job as successful
			//log.Printf("%-4d %-40s\n", job.set.Count, job.set.Name)
		}
		jobStatus.err = err // Record any error encountered
		jobStatus.worker = id
		statChan <- jobStatus // Send job status to status channel
	}
}

// statusWorker process job statuses, received via the job status channel, and handles them accordingly.
// It prints successful job information and re-queues failed jobs after removing the problematic product.
// (will handle TCGPlayer API fetch errors in the future)
func statusWorker(id int, ctx context.Context, jobStatChan <-chan JobStatus, jobChan chan<- Job, wg *sync.WaitGroup) {
	defer wg.Done()
	// Process job statuses from the job status channel
	for {
		status, open := <-jobStatChan
		if !open {
			log.Printf("Status Worker %d: No more job statuses to process. Exiting.\n", id)
			return
		}

		set := status.job.set
		if status.success {
			fmt.Printf("%-5d %-60s %-5d  Worker: %d\n", set.Id, set.Name, set.Count, status.worker)
		} else {
			var pgErr *pgconn.PgError
			if errors.As(status.err, &pgErr) {
				errCode, err := strconv.Atoi(pgErr.Code)
				if err != nil {
					log.Printf("Error converting error code to int: %v\n", err)
					continue
				}
				switch errCode {
				case 23505:
					duplicateKey := getDuplicateKey(pgErr.Detail)                                               // Extract duplicate key from error detail
					status.job.productList = removeProductByProductNumber(status.job.productList, duplicateKey) // Remove duplicate product
					jobChan <- status.job                                                                       // Re-queue job after removing duplicate product
				default:
					log.Printf("Unhandled Postgres error code %d for set %s: %v\n", errCode, status.job.productList[0].SetName, status.err)

				}
			}
		}

	}
}

// newJob creates a new Job instance
func NewJob(productLine datastore.Product_Line, set datastore.Set, products []datastore.Product) Job {
	return Job{
		productLine: productLine,
		set:         set,
		productList: products,
	}
}

type DataContext struct {
	productLine  datastore.Product_Line
	set          datastore.Set
	searchParams tcapi.SearchParams
}

// UpdateSetCount updates the count of products in the set within the DataContext
func (dc *DataContext) UpdateSetCount(count int) {
	dc.set.Count = count
}

// UpdateSearchResultsSize updates the size of search results in the SearchParams within the DataContext
func (dc *DataContext) UpdateSearchResultsSize(size int) {
	dc.searchParams.Size = size
}

// Job represents a job to be processed by a worker
type Job struct {
	productLine datastore.Product_Line
	set         datastore.Set
	productList []datastore.Product
}

// JobStatus represents the status of a processed job
type JobStatus struct {
	success bool
	err     error
	worker  int
	job     Job
}

// LaunchWorkerPool initializes and starts the worker pool (job workers, status worker, and data workers)
func LaunchWorkerPool(workerPoolConfig *WorkerPoolConfig) {
	/*var i int
	for i = 1; i <= workerPoolConfig.poolSize; i++ {
		workerPoolConfig.wg.Add(1)
		go jobWorker(
			i,
			workerPoolConfig.ctx,
			workerPoolConfig.jobChan,
			workerPoolConfig.jobStatChan,
			workerPoolConfig.wg,
			workerPoolConfig.store,
		)
	}*/
}

// WorkerPoolConfig holds configuration for the worker pool
type WorkerPoolConfig struct {
	ctx         context.Context
	poolSize    int
	dataCtxChan chan DataContext // Channel for data contexts
	jobChan     chan Job         // Channel for jobs to be processed
	jobStatChan chan JobStatus   // Channel for job statuses
	store       UserDataStore
}

func NewWorkerPoolConfig(ctx context.Context, poolSize int, dataCtxChan chan DataContext, jobChan chan Job,
	jobStatusChan chan JobStatus, s UserDataStore) *WorkerPoolConfig {
	return &WorkerPoolConfig{
		poolSize:    poolSize,
		dataCtxChan: dataCtxChan,
		jobChan:     jobChan,
		jobStatChan: jobStatusChan,
		store:       s,
	}
}

func getDuplicateKey(errDetail string) string {
	rgx := regexp.MustCompile(`\((.*?)\)`)
	rs := rgx.FindStringSubmatch(errDetail)
	var number string
	if len(rs) > 1 {
		number = strings.Split(rs[3], ",")[0]
	}
	return number
}
