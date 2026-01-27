package tcapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

var PRODUCT_LINES_URL = "https://mp-search-api.tcgplayer.com/v1/search/productLines"
var DATA_SEARCH_URL = "https://mp-search-api.tcgplayer.com/v1/search/request?q=&isList=false"

// Maximum number of product results returned by TCGPlayer API in a single response.
// Used by FetchProductsInParts to limit number of products requested per API call to
// FetchProducts.
const MAX_RESULT_SIZE = 50

func InitRequest(method string, url string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		log.Fatal(fmt.Errorf("Error creating HTTP request: %w", err))
	}
	InitRequestHeader(req)
	return req
}

// Initialize a new SearchCriteria and return the data in a format compatible
// with http.Request.Body (io.Reader).
func NewSearchFilter(sParams SearchParams) io.Reader {
	filter := InitSearchCriteria(sParams) // create search criteria struct
	data, err := json.Marshal(filter)     // marshal struct to JSON
	if err != nil {
		log.Fatal(fmt.Errorf("Error marshaling search criteria to JSON: %w", err))
	}

	// Return search criteria in form compatible with http.Request.Body (io.Reader)
	return bytes.NewReader(data)
}

// Initialize a new SearchCriteria struct with the values specified in sParams
func InitSearchCriteria(sParams SearchParams) SearchCriteria {
	var criteria SearchCriteria
	if sParams.ProductLine != "" {
		criteria.Filters.Term.ProductLineName = []string{sParams.ProductLine}
	}
	if sParams.SetName != "" {
		criteria.Filters.Term.SetName = []string{sParams.SetName}
	}
	if sParams.ProductType != "" {
		criteria.Filters.Term.ProductTypeName = []string{sParams.ProductType}
	}
	criteria.From = sParams.From
	criteria.Size = sParams.Size
	criteria.Algorithm = "sales_dismax"
	criteria.Context.ShippingCountry = "US"
	criteria.ListingSearch.Filters.Exclude.ChannelExclusion = 0
	criteria.ListingSearch.Filters.Range.Quantity.Gte = 1
	criteria.ListingSearch.Filters.Term.ChannelId = 0
	criteria.ListingSearch.Filters.Term.SellerStatus = "Live"
	criteria.Settings.UseFuzzySearch = true
	return criteria
}

// Initialize HTTP request headers according to request headers spcecified
// in https://www.tcgplayer.com request/response via browser developer tools
// file request?q=&isList=false in Network tab file field.
func InitRequestHeader(req *http.Request) {
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", "mp-search-api.tcgplayer.com")
	req.Header.Set("Origin", "https://www.tcgplayer.com")
	req.Header.Set("Referer", "https://www.tcgplayer.com/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("Sec-GPC", "1")
	req.Header.Set("TE", "trailers")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
}

// Return a SearchParams struct initialized with default values
func NewSearchParams(productLine string, setName string,
	productType string, from int, size int,
) SearchParams {
	params := SearchParams{
		From:        from,
		Size:        size,
		ProductLine: productLine,
		SetName:     setName,
		ProductType: productType,
	}
	return params
}
