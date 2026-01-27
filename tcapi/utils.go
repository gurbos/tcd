package tcapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gurbos/tcd/datastore"
)

// Fetch product line data from TCGPlayer API.
// Search parameters are specified in sParams.
func FetchProductLineData(sParams SearchParams) (results SearchResults) {
	client := http.Client{Timeout: 60 * time.Second}
	reqBody := NewSearchFilter(sParams)                           // Create search criteria in io.Reader format
	req := InitRequest(http.MethodPost, DATA_SEARCH_URL, reqBody) // Create HTTP request with search criteria
	res, err := client.Do(req)                                    // Execute HTTP request
	if err != nil {
		log.Fatal(
			fmt.Errorf("Error fetching product line data from TCGPlayer API: %w", err),
		)
	}
	defer res.Body.Close()

	var resData bytes.Buffer                  // buffer to hold raw json response data
	resData.ReadFrom(res.Body)                // Read response body into buffer
	json.Unmarshal(resData.Bytes(), &results) // Unmarshal JSON data into SearchResults struct
	return results
}

// Return list of card sets for the specified product linefrom TCGPlayer API
func FetchSetsByProductLine(productLine string) []datastore.Set {
	sParams := NewSearchParams("", "", "", 0, 0)
	sParams.ProductLine = productLine
	respData := FetchProductLineData(sParams)
	return toSets(respData.Results[0].Aggregations.SetName)
}

// Return list of all product lines from TCGPlayer API
func FetchProductLines() []ValueType {
	sParams := NewSearchParams("", "", "", 0, 0)
	respData := FetchProductLineData(sParams)
	return respData.Results[0].Aggregations.ProductLineName
}

func FetchProductLineByName(urlName string) *datastore.Product_Line {
	pl := FetchProductLines()
	for _, elem := range pl {
		if elem.UrlName == urlName {
			return &datastore.Product_Line{
				Id:      0,
				Name:    elem.Name,
				UrlName: elem.UrlName,
			}
		}
	}

	return nil
}

// Return just the search results from the response data from TCGPlayer API
func FetchProducts(sParams SearchParams) []datastore.Product {
	respData := FetchProductLineData(sParams)
	return respData.Results[0].Results
}

// The TCGPlayer API limits the maximum number of results returned in a single response.
// This function fetches results in chunks of that maximum; it repeatedly calls
// FetchProducts until the total size specified in sParams.Size is reached.
func FetchProductsInParts(sParams SearchParams) []datastore.Product {
	var allResults []datastore.Product
	size := sParams.Size

	sParams.Size = MAX_RESULT_SIZE
	for from := 0; from < size; from += MAX_RESULT_SIZE {
		sParams.From = from
		if from+MAX_RESULT_SIZE > size {
			sParams.Size = size - from
		}
		res := FetchProducts(sParams)
		allResults = append(allResults, res...)
	}

	extractProductAttributes(allResults) // Populate product info from raw JSON data
	return allResults
}

// Extract custom product attributes from JSON raw message and populate Product struct fields.
// Used to populate 'Number' and 'ReleaseDate' fields in Product struct from raw JSON data in
// 'CustomAttributes' field.
func extractProductAttributes(products []datastore.Product) {
	var attrs customAttrs
	for i := 0; i < len(products); i++ {
		elem := &products[i]
		json.Unmarshal(elem.CustomAttributes, &attrs)
		elem.ProductNumber = attrs.Number
		elem.ReleaseDate = attrs.ReleaseDate
	}
}

// ToSets converts a slice of data.ValueType to a slice of datastore.Set
func toSets(setsData []ValueType) (sets []datastore.Set) {
	sets = make([]datastore.Set, len(setsData))
	for i, elem := range setsData {
		sets[i].Name = elem.Name
		sets[i].UrlName = elem.UrlName
		sets[i].Count = int(elem.Count)
	}
	return sets
}
