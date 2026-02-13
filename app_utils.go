package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
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
	var found bool
	cred.username, found = os.LookupEnv("USERNAME")
	if !found {
		log.Fatal("USERNAME environment variable not set")
	}
	cred.password, found = os.LookupEnv("PASSWORD")
	if !found {
		log.Fatal("PASSWORD environment variable not set")
	}
	cred.host, found = os.LookupEnv("HOST")
	if !found {
		log.Fatal("HOST environment variable not set")
	}
	cred.port, found = os.LookupEnv("PORT")
	if !found {
		log.Fatal("PORT environment variable not set")
	}
	cred.dbName, found = os.LookupEnv("DB_NAME")
	if !found {
		log.Fatal("DB_NAME environment variable not set")
	}
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
		fmt.Printf("%-4d : %-60s %-5s %-4d : %-60s\n", i, list[i].Name, "  ", i+mid, list[i+mid].Name)
	}
	if len(list)%2 != 0 {
		fmt.Printf("%-65s%-60s\n", " ", list[len(list)-1].Name)
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

// getProductIdByName retrieves the ProductId of a product by its name.
func getProductIdByName(products []datastore.Product, name string) int {
	for _, p := range products {
		if p.ProductName == name {
			return p.ProductId
		}
	}
	return 0
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
		if len(products) == 0 {
			log.Printf("Data Worker %d: No products found for set '%s'. Skipping.\n", id, dc.set.Name)
			continue
		}
		products = screenProducts(products)       // Screen products to remove those without ProductNumber and duplicates
		dc.UpdateSetCount(len(products))          // Update set count with number of products after screening
		dc.UpdateSearchResultsSize(len(products)) // Update set count with number of products after screening
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

		jobStatus := JobStatus{job: job}                        // Initialize job status
		err := store.AddSetData(ctx, &job.set, job.productList) // attempt to add products to the database
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

// imageWorker fetches and stores images for products received via the jobs channel.
func imageWorker(id int, ctx context.Context, imgIdChan chan []datastore.Product, wg *sync.WaitGroup, store UserDataStore) {
	defer wg.Done()

	// Fetch and store images for products from the image ID channel.
	// Images are fetched using the product Id assigned by the TCGPlayer API,
	// then renamed using the product id assigned by the user data store.
	for {
		prodList, open := <-imgIdChan
		if open {
			setName := prodList[0].SetName
			products, err := store.GetProductsBySetName(ctx, setName) // Get list of products for the specified set from user data store
			if err != nil {
				log.Printf("Error fetching products for set %s: %v\n", setName, err)
				continue
			}

			// Fetch and store images for each product in the job using the product Id from user data store
			imgFiles := make(map[string][]byte) // Map to hold image file data
			for _, elem := range prodList {
				imgData, err := tcapi.FetchProductImageById(ctx, elem.ProductId) // Fetch product image by product Id
				if err != nil {
					log.Printf("Error fetching image for product %s: %v\n", elem.ProductName, err)
					continue
				}
				id := getProductIdByName(products, elem.ProductName)                                 // Get product Id from product list from user data store
				fileName := fmt.Sprintf("%s%d_in_%s", CARD_IMAGE_DIR, id, tcapi.IMAGE_FORMAT_SUFFIX) // Construct file name using product Id
				imgFiles[fileName] = imgData                                                         // Store image data in map
			}
			for fileName, imgData := range imgFiles {
				err = os.WriteFile(fileName, imgData, 0644) // Save image data to file
				if err != nil {
					log.Printf("Error saving image in set %s: %v\n", setName, err)
				}
			}
		} else {
			break // Exit loop if image ID channel is closed
		}
	}
	// Print log message and exit when image Id channel is closed.
	fmt.Printf("Images Worker %d: No more images to fetch. Exiting.\n", id)
}

// statusWorker process job statuses, received via the job status channel, and handles them accordingly.
// It prints successful job information and re-queues failed jobs after removing the problematic product.
// (will handle TCGPlayer API fetch errors in the future)
func statusWorker(id int, ctx context.Context, jobStatChan <-chan JobStatus,
	jobChan chan<- Job, imgInfoChan chan<- []datastore.Product, wg *sync.WaitGroup) {
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
			fmt.Printf("%-5d %-70s %-5d\n", set.Id, set.Name, set.Count)
			imgInfoChan <- status.job.productList // Send product list to image data channel for image fetching
		} else {
			var pgErr *pgconn.PgError
			if errors.As(status.err, &pgErr) {
				switch pgErr.Code {
				case datastore.UniqueViolationError:
					duplicateKey := getDuplicateKey(pgErr.Detail)                                               // Extract duplicate key from error detail
					status.job.productList = removeProductByProductNumber(status.job.productList, duplicateKey) // Remove duplicate product
					jobChan <- status.job                                                                       // Re-queue job after removing duplicate product
				default:
					log.Printf("Unhandled Postgres error code %s for set %s: %v\n", pgErr.Code, status.job.productList[0].SetName, status.err)

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
func LaunchWorkerPool(wpConfig *WorkerPoolConfig) {
	// Launch job workers
	for i := 1; i <= wpConfig.poolSize; i++ {
		wpConfig.jobWaitGroup.Add(1)
		go jobWorker(i, context.Background(), wpConfig.jobsChan, wpConfig.jobStatChan, wpConfig.jobWaitGroup, wpConfig.store)
	}

	// Launch data context workers
	for j := 1; j <= wpConfig.poolSize; j++ {
		wpConfig.dataWaitGroup.Add(1)
		go dataWorker(j, wpConfig.ctx, wpConfig.dataCtxChan, wpConfig.jobsChan, wpConfig.dataWaitGroup)
	}

	// Launch status worker
	for k := 1; k <= wpConfig.poolSize; k++ {
		wpConfig.statusWaitGroup.Add(1)
		go statusWorker(k, wpConfig.ctx, wpConfig.jobStatChan, wpConfig.jobsChan, wpConfig.imgInfoChan, wpConfig.statusWaitGroup)
	}

	// Launch image worker
	for l := 1; l <= wpConfig.poolSize+2; l++ {
		wpConfig.imageWaitGroup.Add(1)
		go imageWorker(l, wpConfig.ctx, wpConfig.imgInfoChan, wpConfig.imageWaitGroup, wpConfig.store)
	}
}

// WorkerPoolConfig holds configuration for the worker pool
type WorkerPoolConfig struct {
	ctx             context.Context
	poolSize        int
	dataCtxChan     chan DataContext         // Channel for data contexts
	jobsChan        chan Job                 // Channel for jobs to be processed
	jobStatChan     chan JobStatus           // Channel for job statuses
	imgInfoChan     chan []datastore.Product // Channel for image data requests
	store           UserDataStore
	dataWaitGroup   *sync.WaitGroup
	jobWaitGroup    *sync.WaitGroup
	statusWaitGroup *sync.WaitGroup
	imageWaitGroup  *sync.WaitGroup
}

func NewWorkerPoolConfig(ctx context.Context, poolSize int, dataCtxChan chan DataContext, jobChan chan Job,
	jobStatusChan chan JobStatus, imgInfoChan chan []datastore.Product, store UserDataStore) *WorkerPoolConfig {
	return &WorkerPoolConfig{
		ctx:             ctx,
		poolSize:        poolSize,
		dataCtxChan:     dataCtxChan,
		jobsChan:        jobChan,
		jobStatChan:     jobStatusChan,
		imgInfoChan:     imgInfoChan,
		store:           store,
		dataWaitGroup:   &sync.WaitGroup{},
		jobWaitGroup:    &sync.WaitGroup{},
		statusWaitGroup: &sync.WaitGroup{},
		imageWaitGroup:  &sync.WaitGroup{},
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
