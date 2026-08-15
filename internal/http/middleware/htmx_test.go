package middleware

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// hxRequest returns a request carrying the given htmx headers.
func hxRequest(headers map[string]string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/styles/1/milestones", nil)
	for name, value := range headers {
		r.Header.Set(name, value)
	}
	return r
}

// capturesHX records what the HTMX middleware read off the request.
func capturesHX(hx *HX, fragment *bool) http.HandlerFunc {
	return func(_ http.ResponseWriter, r *http.Request) {
		*hx = HXFromContext(r.Context())
		*fragment = IsFragment(r.Context())
	}
}

func TestHTMXReadsTheRequestHeaders(t *testing.T) {
	var hx HX
	var fragment bool
	serve(HTMX(), hxRequest(map[string]string{
		headerHXRequest: "true",
		headerHXTarget:  "milestone-list",
		headerHXTrigger: "forecast-form",
	}), capturesHX(&hx, &fragment))

	want := HX{Active: true, Target: "milestone-list", Trigger: "forecast-form"}
	if hx != want {
		t.Errorf("hx = %+v, want %+v", hx, want)
	}
}

func TestIsFragment(t *testing.T) {
	testData := map[string]struct {
		headers map[string]string
		want    bool
	}{
		"plain navigation": {nil, false},
		"htmx swap": {map[string]string{
			headerHXRequest: "true",
		}, true},
		"boosted link": {map[string]string{
			headerHXRequest: "true",
			headerHXBoosted: "true",
		}, false},
		"history restore": {map[string]string{
			headerHXRequest: "true",
			headerHXRestore: "true",
		}, false},
		"header present but not true": {map[string]string{
			headerHXRequest: "yes",
		}, false},
	}
	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			var hx HX
			var fragment bool
			serve(HTMX(), hxRequest(tc.headers), capturesHX(&hx, &fragment))

			if fragment != tc.want {
				t.Errorf("IsFragment = %v, want %v (hx = %+v)", fragment, tc.want, hx)
			}
		})
	}
}

func TestIsHTMXCountsBoostedRequests(t *testing.T) {
	var seen bool
	serve(HTMX(), hxRequest(map[string]string{
		headerHXRequest: "true",
		headerHXBoosted: "true",
	}), func(_ http.ResponseWriter, r *http.Request) { seen = IsHTMX(r.Context()) })

	if !seen {
		t.Error("IsHTMX = false for a boosted request")
	}
}

func TestHTMXVariesOnTheRequestHeader(t *testing.T) {
	rec := serve(HTMX(), hxRequest(nil), func(http.ResponseWriter, *http.Request) {})

	if got := rec.Header().Values("Vary"); !slices.Contains(got, headerHXRequest) {
		t.Errorf("Vary = %v, want it to include %s", got, headerHXRequest)
	}
}

func TestHTMXOutsideARequestIsAPlainNavigation(t *testing.T) {
	if IsFragment(t.Context()) {
		t.Error("IsFragment = true with no request")
	}
	if IsHTMX(t.Context()) {
		t.Error("IsHTMX = true with no request")
	}
	if got := HXFromContext(t.Context()); got != (HX{}) {
		t.Errorf("hx = %+v, want the zero value", got)
	}
}
