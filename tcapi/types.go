package tcapi

import (
	"encoding/json"

	"github.com/gurbos/tcd/datastore"
)

/*-------------------------------------------------------------------------------------------------*/

type SearchCriteria struct {
	Algorithm     string        `json:"algorithm"`
	Context       Context       `json:"context"`
	Filters       filters       `json:"filters"`
	From          int           `json:"from"`
	ListingSearch listingSearch `json:"listingSearch"`
	Settings      settings      `json:"settings"`
	Size          int           `json:"size"`
	Sort          sort          `json:"sort"`
}

/******************************************************************/
type Context struct {
	Cart            _cart        `json:"cart"`
	ShippingCountry string       `json:"shippingCountry"`
	UserProfile     _userProfile `json:"userProfile"`
}

/******************************************************************/
type _cart struct{}
type _userProfile struct{}

/******************************************************************/

type filters struct {
	Match _match `json:"match"`
	Range _range `json:"range"`
	Term  _term  `json:"term"`
}

type _match struct{}
type _range struct{}

type _term struct {
	ProductLineName []string `json:"productLineName,omitempty"`
	SetName         []string `json:"setName,omitempty"`
	ProductTypeName []string `json:"productTypeName,omitempty"`
}

/******************************************************************/

/******************************************************************/
type listingSearch struct {
	Context _context `json:"context"`
	Filters _filters `json:"filters"`
}

type _context struct {
	Cart _cart `json:"cart"`
}

type _filters struct {
	Exclude __exclude `json:"exclude"`
	Range   __range   `json:"range"`
	Term    __term    `json:"term"`
}

type __exclude struct {
	ChannelExclusion int `json:"channelExclusion"`
}

type __range struct {
	Quantity ___quantity `json:"quantity"`
}

type ___quantity struct {
	Gte int `json:"gte"`
}

type __term struct {
	ChannelId    int    `json:"channelId"`
	SellerStatus string `json:"sellerStatus"`
}

/******************************************************************/

/******************************************************************/
type settings struct {
	DidYouMean     _didYouMean `json:"didYouMean"`
	UseFuzzySearch bool        `json:"useFuzzySearch"`
}

type _didYouMean struct{}

/******************************************************************/
type sort struct{}

/******************************************************************/

/*-------------------------------------------------------------------------------------------------*/

type SearchResults struct {
	Errors  []Error   `json:"errors"`
	Results []Results `json:"results"`
}

type Error struct{}

type Results struct {
	Aggregations aggregations `json:"aggregations"`
	Results      []Product    `json:"results"`
}

/******************************************************************/

type aggregations struct {
	CardType        []ValueType `json:"cardType"`
	RarityName      []ValueType `json:"rarityName"`
	SetName         []ValueType `json:"setName"`
	ProductTypeName []ValueType `json:"productTypeName"`
	ProductLineName []ValueType `json:"productLineName"`
	Condition       []ValueType `json:"condition"`
}

type ValueType struct {
	Name    string  `json:"value"`
	UrlName string  `json:"urlValue"`
	Count   float64 `json:"count"`
}

type Product struct {
	ProductId          float64         `json:"productId"`
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

/******************************************************************/
/*type Product struct {
	ProductLineUrlName string          `json:"productLineUrlName"`
	ProductUrlName     string          `json:"productUrlName"`
	RarityName         string          `json:"rarityName"`
	CustomAttributes   json.RawMessage `json:"customAttributes"`
	ProductName        string          `json:"productName"`
	SetName            string          `json:"setName"`
	FoilOnly           bool            `json:"foilOnluy"`
	SetUrlName         string          `json:"setUrlName"`
	ProductLineName    string          `json:"productLineName"`
	ProductTypeId      int             `json:"productTypeId"`
	Number             string
	ReleaseDate        string
}*/

/******************************************************************/

/*-------------------------------------------------------------------------------------------------*/

// Structure for holding search parameters
type SearchParams struct {
	ProductLine string
	SetName     string
	ProductType string
	From        int
	Size        int
}

// SearchParams method to update SetName and Size from ValueType set info
func (sp *SearchParams) UpdateFromSetInfo(set datastore.Set) {
	sp.SetName = set.UrlName
	sp.Size = int(set.Count)
}

/*-------------------------------------------------------------------------------------------------*/

// Structure for holding custom product attributes
type customAttrs struct {
	Number      string `json:"number"`
	ReleaseDate string `json:"releaseDate"`
}
