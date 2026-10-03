package account

import (
	"context"
	"net/http"
	"testing"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/testutil"
)

func TestApplyProfileUpdate(t *testing.T) {
	str := func(s string) *string { return &s }
	long := func(n int) *string {
		b := make([]rune, n)
		for i := range b {
			b[i] = '字'
		}
		s := string(b)
		return &s
	}
	cases := []struct {
		name    string
		req     updateMeRequest
		wantErr string
	}{
		{"trims and sets", updateMeRequest{DisplayName: str("  Ada  "), Bio: str(" hi "), Region: str("AU")}, ""},
		{"empty name rejected", updateMeRequest{DisplayName: str("   ")}, "invalid_display_name"},
		{"name at limit ok", updateMeRequest{DisplayName: long(maxDisplayNameRunes)}, ""},
		{"name over limit", updateMeRequest{DisplayName: long(maxDisplayNameRunes + 1)}, "invalid_display_name"},
		{"bio over limit", updateMeRequest{Bio: long(maxBioRunes + 1)}, "invalid_bio"},
		{"region over limit", updateMeRequest{Region: long(maxRegionRunes + 1)}, "invalid_region"},
		{"clear bio", updateMeRequest{Bio: str("")}, ""},
	}
	for _, c := range cases {
		u := &auth.User{DisplayName: "old", Bio: "old bio", Region: "US"}
		err := applyProfileUpdate(u, c.req)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != c.wantErr {
			t.Errorf("%s: err=%q want %q", c.name, got, c.wantErr)
		}
	}
	u := &auth.User{DisplayName: "old", Bio: "keep", Region: "US"}
	_ = applyProfileUpdate(u, updateMeRequest{DisplayName: str("  Ada  ")})
	if u.DisplayName != "Ada" || u.Bio != "keep" || u.Region != "US" {
		t.Errorf("partial update changed wrong fields: %+v", u)
	}
}

func TestUpdateMe_Persists(t *testing.T) {
	pool := testutil.Pool(t)
	u := testutil.NewUser(t, pool, "user")
	deps := Handler{Pool: pool}

	rr := testutil.Call(t, http.HandlerFunc(deps.updateMe), http.MethodPatch, "/api/me", nil, u,
		`{"display_name":"  Grace  ","bio":"Debugs things","region":"NZ"}`)
	testutil.WantStatus(t, rr, http.StatusOK)
	var name, bio, region string
	if err := pool.QueryRow(context.Background(),
		`SELECT display_name, COALESCE(bio,''), COALESCE(region,'') FROM users WHERE id = $1`, u.ID).
		Scan(&name, &bio, &region); err != nil {
		t.Fatal(err)
	}
	if name != "Grace" || bio != "Debugs things" || region != "NZ" {
		t.Errorf("persisted %q %q %q", name, bio, region)
	}

	rr = testutil.Call(t, http.HandlerFunc(deps.updateMe), http.MethodPatch, "/api/me", nil, u, `{"role":"admin"}`)
	testutil.WantStatus(t, rr, http.StatusBadRequest)
	rr = testutil.Call(t, http.HandlerFunc(deps.updateMe), http.MethodPatch, "/api/me", nil, u, `{"display_name":""}`)
	testutil.WantStatus(t, rr, http.StatusBadRequest)
}

func TestAccount_RequiresLogin(t *testing.T) {
	h := Handler{}
	for _, fn := range []http.HandlerFunc{h.me, h.updateMe} {
		rr := testutil.Call(t, fn, http.MethodGet, "/api/me", nil, nil, "")
		testutil.WantStatus(t, rr, http.StatusUnauthorized)
	}
}
