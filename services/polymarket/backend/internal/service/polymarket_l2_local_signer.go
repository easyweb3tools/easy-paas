package service

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/shopspring/decimal"

	"polymarket/internal/models"
)

func (e *CLOBExecutor) signOrderLocally(cfg liveBrokerConfig, order models.Order, leg orderLeg) (any, string, string, *bool, error) {
	pk := strings.TrimSpace(cfg.PrivateKey)
	if pk == "" {
		return nil, "", "", nil, fmt.Errorf("trading.live.private_key is required for auth_mode=polymarket_l2_local")
	}
	key, err := parseECDSAPrivateKeyHex(pk)
	if err != nil {
		return nil, "", "", nil, err
	}
	owner := strings.TrimSpace(leg.Owner)
	if owner == "" {
		owner = strings.TrimSpace(cfg.Address)
	}
	if owner == "" {
		owner = strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	}

	orderMap, err := buildUnsignedOrderPayload(order, leg, owner)
	if err != nil {
		return nil, "", "", nil, err
	}
	if leg.UnsignedOrder != nil {
		override, err := toMap(leg.UnsignedOrder)
		if err != nil {
			return nil, "", "", nil, err
		}
		for k, v := range override {
			orderMap[k] = v
		}
	}

	hash, err := resolveSigningHash(strings.TrimSpace(leg.SigningHash), orderMap)
	if err != nil {
		return nil, "", "", nil, err
	}
	sig, err := crypto.Sign(hash, key)
	if err != nil {
		return nil, "", "", nil, err
	}
	sigHex := "0x" + hex.EncodeToString(sig)

	sigField := strings.TrimSpace(leg.SignatureField)
	if sigField == "" {
		sigField = "signature"
	}
	ownerField := strings.TrimSpace(leg.OwnerField)
	if ownerField == "" {
		ownerField = "owner"
	}
	orderMap[sigField] = sigHex

	if ownerField != "" {
		orderMap[ownerField] = owner
	}

	orderType := strings.TrimSpace(leg.OrderType)
	if orderType == "" {
		orderType = "GTC"
	}
	return orderMap, owner, orderType, leg.PostOnly, nil
}

func buildUnsignedOrderPayload(order models.Order, leg orderLeg, owner string) (map[string]any, error) {
	price := order.Price
	if price.LessThanOrEqual(decimal.Zero) {
		price = decimal.NewFromFloat(0.5)
	}
	sizeUSD := order.SizeUSD
	if leg.SizeUSD != nil && *leg.SizeUSD > 0 {
		sizeUSD = decimal.NewFromFloat(*leg.SizeUSD)
	}
	if sizeUSD.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("order size_usd must be > 0")
	}
	side := normalizeOrderSide(order.Side, leg.Direction)
	if side == "" {
		side = "BUY"
	}
	shares := sizeUSD.Div(price)
	if shares.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("computed share size must be > 0")
	}

	usdcBase := decimal.NewFromInt(1_000_000)
	makerAmount := sizeUSD.Mul(usdcBase)
	takerAmount := shares.Mul(usdcBase)
	if side == "SELL" {
		makerAmount, takerAmount = takerAmount, makerAmount
	}

	exp := time.Now().UTC().Add(24 * time.Hour).Unix()
	expiration := int64ToStringWithDefault(firstInt64(leg.UnsignedOrder, "expiration", "expirationTime"), exp)
	nonce := int64ToStringWithDefault(firstInt64(leg.UnsignedOrder, "nonce"), 0)
	salt := int64ToStringWithDefault(firstInt64(leg.UnsignedOrder, "salt"), time.Now().UTC().UnixNano())
	taker := firstStringFromAny(leg.UnsignedOrder, "taker")
	if taker == "" {
		taker = "0x0000000000000000000000000000000000000000"
	}
	signatureType := firstInt64(leg.UnsignedOrder, "signatureType", "signature_type")
	feeRateBps := firstInt64(leg.UnsignedOrder, "feeRateBps", "fee_rate_bps")

	out := map[string]any{
		"salt":          salt,
		"maker":         owner,
		"signer":        owner,
		"taker":         taker,
		"tokenId":       strings.TrimSpace(order.TokenID),
		"makerAmount":   decimalToIntString(makerAmount),
		"takerAmount":   decimalToIntString(takerAmount),
		"expiration":    expiration,
		"nonce":         nonce,
		"feeRateBps":    strconv.FormatInt(feeRateBps, 10),
		"side":          side,
		"signatureType": signatureType,
	}
	if out["tokenId"] == "" {
		return nil, fmt.Errorf("order token_id is required")
	}
	return out, nil
}

func normalizeOrderSide(orderSide, legDirection string) string {
	choose := strings.ToUpper(strings.TrimSpace(orderSide))
	if choose == "" {
		choose = strings.ToUpper(strings.TrimSpace(legDirection))
	}
	switch {
	case strings.HasPrefix(choose, "BUY"):
		return "BUY"
	case strings.HasPrefix(choose, "SELL"):
		return "SELL"
	default:
		return ""
	}
}

func decimalToIntString(v decimal.Decimal) string {
	if v.LessThan(decimal.Zero) {
		v = decimal.Zero
	}
	return v.Round(0).StringFixed(0)
}

func int64ToStringWithDefault(v int64, d int64) string {
	if v == 0 {
		v = d
	}
	return strconv.FormatInt(v, 10)
}

func firstStringFromAny(v any, keys ...string) string {
	m, err := toMap(v)
	if err != nil || m == nil {
		return ""
	}
	for _, k := range keys {
		raw, ok := m[k]
		if !ok {
			continue
		}
		out := strings.TrimSpace(fmt.Sprintf("%v", raw))
		if out != "" && out != "<nil>" {
			return out
		}
	}
	return ""
}

func firstInt64(v any, keys ...string) int64 {
	m, err := toMap(v)
	if err != nil || m == nil {
		return 0
	}
	for _, k := range keys {
		raw, ok := m[k]
		if !ok || raw == nil {
			continue
		}
		switch t := raw.(type) {
		case int:
			return int64(t)
		case int8:
			return int64(t)
		case int16:
			return int64(t)
		case int32:
			return int64(t)
		case int64:
			return t
		case uint:
			return int64(t)
		case uint8:
			return int64(t)
		case uint16:
			return int64(t)
		case uint32:
			return int64(t)
		case uint64:
			if t > math.MaxInt64 {
				return math.MaxInt64
			}
			return int64(t)
		case float32:
			return int64(t)
		case float64:
			return int64(t)
		case json.Number:
			if i, err := t.Int64(); err == nil {
				return i
			}
		case string:
			s := strings.TrimSpace(t)
			s = strings.TrimPrefix(s, "0x")
			if s == "" {
				continue
			}
			if i, ok := new(big.Int).SetString(s, 10); ok && i.IsInt64() {
				return i.Int64()
			}
			if i, ok := new(big.Int).SetString(s, 16); ok && i.IsInt64() {
				return i.Int64()
			}
		}
	}
	return 0
}

func parseECDSAPrivateKeyHex(raw string) (*ecdsa.PrivateKey, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "0x")
	if raw == "" {
		return nil, fmt.Errorf("empty private key")
	}
	key, err := crypto.HexToECDSA(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	return key, nil
}

func parseHash32Hex(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "0x")
	b, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid signing_hash: %w", err)
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("signing_hash must be 32 bytes")
	}
	return b, nil
}

func resolveSigningHash(raw string, orderMap map[string]any) ([]byte, error) {
	if strings.TrimSpace(raw) != "" {
		return parseHash32Hex(raw)
	}
	canonical, err := canonicalJSON(orderMap)
	if err != nil {
		return nil, err
	}
	hash := crypto.Keccak256(canonical)
	if len(hash) != 32 {
		return nil, fmt.Errorf("invalid keccak hash size")
	}
	return hash, nil
}

func canonicalJSON(v any) ([]byte, error) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]json.RawMessage, len(keys))
		for _, k := range keys {
			b, err := canonicalJSON(t[k])
			if err != nil {
				return nil, err
			}
			out[k] = b
		}
		return json.Marshal(out)
	case []any:
		arr := make([]json.RawMessage, 0, len(t))
		for _, item := range t {
			b, err := canonicalJSON(item)
			if err != nil {
				return nil, err
			}
			arr = append(arr, b)
		}
		return json.Marshal(arr)
	default:
		return json.Marshal(t)
	}
}

func toMap(v any) (map[string]any, error) {
	if m, ok := v.(map[string]any); ok {
		cpy := make(map[string]any, len(m))
		for k, val := range m {
			cpy[k] = val
		}
		return cpy, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
