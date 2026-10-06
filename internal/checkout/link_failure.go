package checkout

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strconv"
)

// LinkFailureCode returns only fixed categories and bounded HTTP status codes.
// Never return err.Error(): it may include credentials or a checkout URL.
func LinkFailureCode(err error) string {
	if err == nil { return "NONE" }
	if errors.Is(err, context.DeadlineExceeded) { return "TIMEOUT" }
	if errors.Is(err, context.Canceled) { return "REQUEST_CANCELED" }
	var network *stripeTransportFailure
	if errors.As(err, &network) { return "STRIPE_NETWORK_FAILED" }
	var stripe *stripeError
	if errors.As(err, &stripe) {
		switch stripe.Code {
		case "api_key_expired", "api_key_invalid": return "STRIPE_PUBLIC_KEY_REJECTED"
		case "checkout_not_active_session": return "STRIPE_SESSION_INACTIVE"
		case "resource_missing": return "STRIPE_RESOURCE_MISSING"
		}
		if stripe.HTTP >= 400 && stripe.HTTP <= 599 { return "STRIPE_HTTP_" + strconv.Itoa(stripe.HTTP) }
		return "STRIPE_REQUEST_REJECTED"
	}
	if code := knownLinkFailure(err, 0); code != "" { return code }
	if errors.Is(err, sql.ErrNoRows) { return "CONFIG_OR_ORDER_RECORD_MISSING" }
	if errors.Is(err, ErrXReadFailure) { return "X_READ_FAILED" }
	return "OTHER_ERROR_REDACTED"
}

var xFailureHTTP = regexp.MustCompile(`^X [A-Za-z][A-Za-z0-9_]{0,80} returned HTTP ([45][0-9]{2}); no payment attempted$`)

func knownLinkFailure(err error, depth int) string {
	if err == nil || depth > 12 { return "" }
	switch err.Error() {
	case "unexpected X product or price list": return "X_PRODUCT_OR_PRICE_LIST_MISMATCH"
	case "X price is not exactly the allowed one-time amount": return "X_PRICE_CURRENCY_OR_PAYMENT_TYPE_MISMATCH"
	case "invalid Stripe merchant publishable key": return "STRIPE_PUBLIC_KEY_INVALID"
	case "X API authentication metadata is missing or unreadable", "invalid X API authentication metadata": return "X_AUTH_CONFIGURATION_INVALID"
	case "required X cookies missing", "unexpected cookie domain", "unexpected cookie name", "invalid X cookie": return "X_COOKIES_INVALID"
	case "recipient identity or gift eligibility could not be verified": return "X_RECIPIENT_UNVERIFIED"
	case "X regional checkout proxy is unavailable": return "X_CHECKOUT_PROXY_UNAVAILABLE"
	case "X returned non-JSON data": return "X_NON_JSON_RESPONSE"
	case "X checkout status is not Unpaid; payment was not submitted": return "X_CHECKOUT_NOT_UNPAID"
	case "X checkout session ID is missing or not a live session; payment was not submitted": return "X_CHECKOUT_SESSION_INVALID"
	case "X checkout URL is unsupported or does not match its session; payment was not submitted": return "X_CHECKOUT_URL_INVALID"
	case "Stripe merchant, session, recipient return URLs, currency or payment mode mismatch": return "STRIPE_MERCHANT_OR_SESSION_MISMATCH"
	case "Stripe product, duration, quantity or unit amount mismatch": return "STRIPE_PRODUCT_OR_AMOUNT_MISMATCH"
	case "Stripe payment intent amount or currency mismatch": return "STRIPE_INTENT_AMOUNT_MISMATCH"
	case "Stripe returned non-JSON data": return "STRIPE_NON_JSON_RESPONSE"
	case "saved payment route is invalid; refusing to select another node", "saved payment route card binding is invalid", "saved payment outbound is invalid": return "SAVED_STRIPE_ROUTE_INVALID"
	case "catalog record is missing or unreadable; write it with setup or put --name catalog", "invalid catalog JSON", "invalid catalog merchant", "invalid catalog currency", "invalid catalog plan product", "no catalog plan allows this duration": return "CATALOG_CONFIGURATION_INVALID"
	}
	if match := xFailureHTTP.FindStringSubmatch(err.Error()); match != nil { return "X_HTTP_"+match[1] }
	if children, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range children.Unwrap() { if code:=knownLinkFailure(child,depth+1); code!="" { return code } }
	}
	return knownLinkFailure(errors.Unwrap(err),depth+1)
}
