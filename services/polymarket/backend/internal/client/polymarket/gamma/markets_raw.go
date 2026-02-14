package polymarketgamma

import (
	"context"
	"fmt"
	"net/url"
)

// GetMarketRawByID returns the raw JSON body from Gamma for a market.
// This is used by ingestion jobs that need fields not currently modeled in the typed Market struct.
func (c *Client) GetMarketRawByID(ctx context.Context, marketID string, params *GetMarketByIDQueryParams) ([]byte, error) {
	path := fmt.Sprintf("/markets/%s", url.PathEscape(marketID))
	if params != nil && params.IncludeTag != nil {
		urlParams := url.Values{}
		urlParams.Add("include_tag", fmt.Sprintf("%t", *params.IncludeTag))
		path += "?" + urlParams.Encode()
	}
	return c.doRequest(ctx, "GET", path)
}
