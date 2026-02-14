package clob

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type Decimal struct {
	decimal.Decimal
}

func (d *Decimal) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		d.Decimal = decimal.Zero
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		val, err := decimal.NewFromString(s)
		if err != nil {
			return err
		}
		d.Decimal = val
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err == nil {
		d.Decimal = decimal.NewFromFloat(f)
		return nil
	}
	return fmt.Errorf("invalid decimal: %s", string(b))
}

type Order struct {
	Price decimal.Decimal
	Size  decimal.Decimal
}

func (o *Order) UnmarshalJSON(b []byte) error {
	var arr []json.RawMessage
	if err := json.Unmarshal(b, &arr); err == nil && len(arr) >= 2 {
		price, err := parseDecimalRaw(arr[0])
		if err != nil {
			return err
		}
		size, err := parseDecimalRaw(arr[1])
		if err != nil {
			return err
		}
		o.Price = price
		o.Size = size
		return nil
	}
	var obj struct {
		Price json.RawMessage `json:"price"`
		Size  json.RawMessage `json:"size"`
		Qty   json.RawMessage `json:"qty"`
	}
	if err := json.Unmarshal(b, &obj); err == nil {
		price, err := parseDecimalRaw(obj.Price)
		if err != nil {
			return err
		}
		sizeRaw := obj.Size
		if len(sizeRaw) == 0 {
			sizeRaw = obj.Qty
		}
		size, err := parseDecimalRaw(sizeRaw)
		if err != nil {
			return err
		}
		o.Price = price
		o.Size = size
		return nil
	}
	return fmt.Errorf("invalid order: %s", string(b))
}

type OrderBook struct {
	Bids []Order `json:"bids"`
	Asks []Order `json:"asks"`
}

type PricePoint struct {
	TS    time.Time
	Price decimal.Decimal
}

func parsePrice(body []byte) (Decimal, error) {
	var resp struct {
		Price Decimal `json:"price"`
	}
	if err := json.Unmarshal(body, &resp); err == nil && !resp.Price.Decimal.IsZero() {
		return resp.Price, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return Decimal{}, err
	}
	if priceRaw, ok := raw["price"]; ok {
		val, err := parseDecimalRaw(priceRaw)
		if err != nil {
			return Decimal{}, err
		}
		return Decimal{Decimal: val}, nil
	}
	return Decimal{}, fmt.Errorf("price not found in response")
}

func parseOrderBook(body []byte) (*OrderBook, error) {
	var book OrderBook
	if err := json.Unmarshal(body, &book); err == nil {
		return &book, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if bidsRaw, ok := raw["bids"]; ok {
		_ = json.Unmarshal(bidsRaw, &book.Bids)
	}
	if asksRaw, ok := raw["asks"]; ok {
		_ = json.Unmarshal(asksRaw, &book.Asks)
	}
	return &book, nil
}

func parsePriceHistory(body []byte) ([]PricePoint, error) {
	var raw struct {
		Prices []json.RawMessage `json:"prices"`
		Data   []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err == nil {
		if len(raw.Prices) > 0 {
			return parsePricePointList(raw.Prices)
		}
		if len(raw.Data) > 0 {
			return parsePricePointList(raw.Data)
		}
	}
	var list []json.RawMessage
	if err := json.Unmarshal(body, &list); err == nil {
		return parsePricePointList(list)
	}
	return nil, fmt.Errorf("unknown price history format")
}

func parsePricePointList(items []json.RawMessage) ([]PricePoint, error) {
	points := make([]PricePoint, 0, len(items))
	for _, item := range items {
		if len(item) == 0 {
			continue
		}
		if point, ok := parsePricePointArray(item); ok {
			points = append(points, point)
			continue
		}
		if point, ok := parsePricePointObject(item); ok {
			points = append(points, point)
		}
	}
	return points, nil
}

func parsePricePointArray(item json.RawMessage) (PricePoint, bool) {
	var arr []json.RawMessage
	if err := json.Unmarshal(item, &arr); err != nil || len(arr) < 2 {
		return PricePoint{}, false
	}
	ts, err := parseTimeRaw(arr[0])
	if err != nil {
		return PricePoint{}, false
	}
	price, err := parseDecimalRaw(arr[1])
	if err != nil {
		return PricePoint{}, false
	}
	return PricePoint{TS: ts, Price: price}, true
}

func parsePricePointObject(item json.RawMessage) (PricePoint, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(item, &obj); err != nil {
		return PricePoint{}, false
	}
	tsRaw := firstRaw(obj, "ts", "t", "timestamp", "time")
	priceRaw := firstRaw(obj, "price", "p")
	if len(tsRaw) == 0 || len(priceRaw) == 0 {
		return PricePoint{}, false
	}
	ts, err := parseTimeRaw(tsRaw)
	if err != nil {
		return PricePoint{}, false
	}
	price, err := parseDecimalRaw(priceRaw)
	if err != nil {
		return PricePoint{}, false
	}
	return PricePoint{TS: ts, Price: price}, true
}

func parseDecimalRaw(b json.RawMessage) (decimal.Decimal, error) {
	var d Decimal
	if err := json.Unmarshal(b, &d); err != nil {
		return decimal.Zero, err
	}
	return d.Decimal, nil
}

func parseTimeRaw(b json.RawMessage) (time.Time, error) {
	var i int64
	if err := json.Unmarshal(b, &i); err == nil {
		return unixToTime(i), nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err == nil {
		return unixToTime(int64(f)), nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil && s != "" {
		if ts, err := time.Parse(time.RFC3339, s); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time: %s", string(b))
}

func unixToTime(v int64) time.Time {
	if v > 1_000_000_000_000 {
		return time.UnixMilli(v).UTC()
	}
	return time.Unix(v, 0).UTC()
}

func firstRaw(m map[string]json.RawMessage, keys ...string) json.RawMessage {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return nil
}
