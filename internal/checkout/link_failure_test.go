package checkout

import (
	"errors"
	"fmt"
	"testing"
)

func TestLinkFailureCodesRedactSensitiveErrors(t *testing.T) {
	for _, tc := range []struct{err error; want string}{
		{errors.New("X price is not exactly the allowed one-time amount"), "X_PRICE_CURRENCY_OR_PAYMENT_TYPE_MISMATCH"},
		{fmt.Errorf("%w: %w",ErrXReadFailure,errors.New("X UserByScreenName returned HTTP 403; no payment attempted")), "X_HTTP_403"},
		{errors.New("invalid Stripe merchant publishable key"), "STRIPE_PUBLIC_KEY_INVALID"},
		{&stripeError{HTTP:401,Message:"secret account details",RequestID:"private",Code:"unexpected secret"}, "STRIPE_HTTP_401"},
		{temporary(&stripeTransportFailure{}), "STRIPE_NETWORK_FAILED"},
		{errors.New("X price is not exactly the allowed one-time amount\nBearer PRIVATE"), "OTHER_ERROR_REDACTED"},
		{errors.New("https://checkout.stripe.com/c/pay/cs_live_PRIVATE?secret=PRIVATE"), "OTHER_ERROR_REDACTED"},
	} {
		if got:=LinkFailureCode(tc.err); got!=tc.want { t.Errorf("got %q, want %q",got,tc.want) }
	}
}
