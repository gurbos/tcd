package tcapi

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Fetch list of product lines from TCGPlayer API
/*func FetchProductLines() []ValueType {
	client := &http.Client{Timeout: 5 * time.Second}
	req := InitRequest(http.MethodGet, PRODUCT_LINES_URL, nil)
	res, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	var resBody bytes.Buffer
	resBody.ReadFrom(res.Body)

	dataBytes := resBody.Bytes()
	var productLines []ValueType
	err = json.Unmarshal(dataBytes, &productLines)
	if err != nil {
		log.Fatal()
	}
	return productLines
}*/

// Fetch product line data from TCGPlayer API.
// Search parameters are specified in sParams.
func FetchProductLineData(sParams SearchParams) (results SearchResults) {
	client := http.Client{Timeout: 5 * time.Second}
	reqBody := NewSearchFilter(sParams)                           // Create search criteria in io.Reader format
	req := InitRequest(http.MethodPost, DATA_SEARCH_URL, reqBody) // Create HTTP request with search criteria
	res, err := client.Do(req)                                    // Execute HTTP request
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	var resData bytes.Buffer                  // buffer to hold raw json response data
	resData.ReadFrom(res.Body)                // Read response body into buffer
	json.Unmarshal(resData.Bytes(), &results) // Unmarshal JSON data into SearchResults struct
	return results
}

// Return list of card sets for the specified product linefrom TCGPlayer API
func FetchSetsByProductLine(productLine string) []ValueType {
	sParams := NewSearchParams()
	sParams.ProductLine = productLine
	respData := FetchProductLineData(sParams)
	return respData.Results[0].Aggregations.SetName
}

// Return list of all product lines from TCGPlayer API
func FetchProductLines() []ValueType {
	sParams := NewSearchParams()
	respData := FetchProductLineData(sParams)
	return respData.Results[0].Aggregations.ProductLineName
}

// Return just the search results from the response data from TCGPlayer API
func FetchSearchResults(sParams SearchParams) []Product {
	respData := FetchProductLineData(sParams)
	return respData.Results[0].Results
}

// The TCGPlayer API limits the maximum number of results returned in a single response.
// This function fetches results in chunks of that maximum; it repeatedly calls
// FetchSearchResults until the total size specified in sParams.Size is reached.
func FetchResultsInParts(sParams SearchParams, partSize int) []Product {
	var allResults []Product
	size := sParams.Size

	sParams.Size = partSize
	for from := 0; from < size; from += partSize {
		sParams.From = from
		if from+partSize > size {
			sParams.Size = size - from
		}
		res := FetchSearchResults(sParams)
		allResults = append(allResults, res...)
	}

	extractProductAttributes(allResults) // Populate product info from raw JSON data
	return allResults
}

// Extract custom product attributes from JSON raw message and populate Product struct fields.
// Used to populate 'Number' and 'ReleaseDate' fields in Product struct from raw JSON data in
// 'CustomAttributes' field.
func extractProductAttributes(products []Product) {
	var attrs customAttrs
	for i := 0; i < len(products); i++ {
		elem := &products[i]
		json.Unmarshal(elem.CustomAttributes, &attrs)
		elem.Number = attrs.Number
		elem.ReleaseDate = attrs.ReleaseDate
	}
}

// SearchParams method to update SetName and Size from ValueType set info
func (sp *SearchParams) UpdateFromSetInfo(set ValueType) {
	sp.SetName = set.UrlName
	sp.Size = int(set.Count)
}
