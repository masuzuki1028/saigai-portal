//go:build integration

package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/masuzuki1028/saigai-portal/api/internal/apigen"
	"github.com/masuzuki1028/saigai-portal/api/internal/area"
	"github.com/masuzuki1028/saigai-portal/api/internal/auth"
	"github.com/masuzuki1028/saigai-portal/api/internal/crypto"
	"github.com/masuzuki1028/saigai-portal/api/internal/httpapi"
	"github.com/masuzuki1028/saigai-portal/api/internal/store"
	"github.com/masuzuki1028/saigai-portal/api/internal/store/storetest"
)

// testVerifier は "Bearer <sub>" を受け付ける（テスト専用）。
type testVerifier struct{}

func (testVerifier) Verify(r *http.Request) (auth.Principal, error) {
	sub, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || sub == "" {
		return auth.Principal{}, auth.ErrUnauthenticated
	}
	return auth.Principal{Issuer: "test", Subject: sub}, nil
}

type noopSender struct{ sent int }

func (n *noopSender) SendTest(context.Context, store.User) error { n.sent++; return nil }

func newServer(t *testing.T) (*httptest.Server, *noopSender) {
	t.Helper()
	sealer, _ := crypto.NewAESGCM(bytes.Repeat([]byte{1}, 32))
	sender := &noopSender{}
	s := &httpapi.Server{
		Store: storetest.New(t), Resolver: area.StaticResolver{Points: area.DefaultStaticPoints},
		Sealer: sealer, Notifier: sender, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	h, err := s.Handler(testVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, sender
}

// call はリクエストを送り、レスポンスが OpenAPI のスキーマに合っているかも確認する（kin-openapi）。
func call(t *testing.T, srv *httptest.Server, method, path, token string, body any) (int, []byte) {
	t.Helper()
	var rd io.Reader
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, srv.URL+path, rd)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	resBody, _ := io.ReadAll(res.Body)
	validateResponse(t, method, path, raw, res, resBody)
	return res.StatusCode, resBody
}

func validateResponse(t *testing.T, method, path string, reqBody []byte, res *http.Response, body []byte) {
	t.Helper()
	spec, err := apigen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	spec.Servers = nil
	router, err := gorillamux.NewRouter(spec)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(method, "http://x"+strings.TrimPrefix(path, "/api"), bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	route, params, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("%s %s: 仕様にないパス: %v", method, path, err)
	}
	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route,
			Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}},
		Status: res.StatusCode, Header: res.Header, Body: io.NopCloser(bytes.NewReader(body)),
		Options: &openapi3filter.Options{IncludeResponseStatus: true},
	}
	if err := openapi3filter.ValidateResponse(context.Background(), in); err != nil {
		t.Errorf("%s %s → %d: レスポンスが OpenAPI と合わない: %v\n%s", method, path, res.StatusCode, err, body)
	}
}

var nagaoka = map[string]any{"name": "実家", "coord": map[string]any{"lat": 37.4462, "lon": 138.8512}, "radiusM": 3000, "address": "新潟県長岡市"}

func createLocation(t *testing.T, srv *httptest.Server, token string) apigen.Location {
	t.Helper()
	code, body := call(t, srv, http.MethodPost, "/api/locations", token, nagaoka)
	if code != http.StatusCreated {
		t.Fatalf("地点の登録: %d %s", code, body)
	}
	var l apigen.Location
	_ = json.Unmarshal(body, &l)
	return l
}

func TestLocationCRUD(t *testing.T) {
	srv, sender := newServer(t)
	l := createLocation(t, srv, "alice")
	if l.AreaName != "長岡市" || l.Address == nil || *l.Address != "新潟県長岡市" || l.Coord.Lat != 37.4462 || l.Version != 1 {
		t.Errorf("登録した地点: %+v", l)
	}
	if l.HazardSettings.Earthquake.NotifyMinIntensity != apigen.Int5Lower {
		t.Errorf("初期値の設定が入っていない: %+v", l.HazardSettings)
	}

	upd := map[string]any{"name": "実家（長岡）", "coord": map[string]any{"lat": 37.4462, "lon": 138.8512}, "radiusM": 5000, "version": 1}
	code, body := call(t, srv, http.MethodPut, "/api/locations/"+l.Id, "alice", upd)
	if code != http.StatusOK || !strings.Contains(string(body), "実家（長岡）") {
		t.Fatalf("更新: %d %s", code, body)
	}
	// 古い version での更新は 409
	if code, _ := call(t, srv, http.MethodPut, "/api/locations/"+l.Id, "alice", upd); code != http.StatusConflict {
		t.Errorf("古い version での更新: %d", code)
	}

	code, body = call(t, srv, http.MethodGet, "/api/locations", "alice", nil)
	if code != http.StatusOK || strings.Count(string(body), `"id"`) != 1 {
		t.Errorf("一覧: %d %s", code, body)
	}
	if code, _ := call(t, srv, http.MethodGet, "/api/locations/"+l.Id+"/timeline", "alice", nil); code != http.StatusOK {
		t.Errorf("タイムライン: %d", code)
	}
	if code, _ := call(t, srv, http.MethodPost, "/api/me/notification-settings/test", "alice", nil); code != http.StatusAccepted || sender.sent != 1 {
		t.Errorf("テスト通知: %d sent=%d", code, sender.sent)
	}
	if code, _ := call(t, srv, http.MethodPut, "/api/me/notification-settings", "alice", map[string]any{"emailEnabled": false}); code != http.StatusOK {
		t.Errorf("通知設定: %d", code)
	}
	if code, _ := call(t, srv, http.MethodPost, "/api/me/notification-settings/test", "alice", nil); code != http.StatusConflict {
		t.Errorf("通知オフでのテスト通知: %d", code)
	}
	if code, _ := call(t, srv, http.MethodDelete, "/api/locations/"+l.Id, "alice", nil); code != http.StatusNoContent {
		t.Errorf("削除: %d", code)
	}
	if code, _ := call(t, srv, http.MethodGet, "/api/locations/"+l.Id, "alice", nil); code != http.StatusNotFound {
		t.Errorf("削除後の取得: %d", code)
	}
}

func TestValidation(t *testing.T) {
	srv, _ := newServer(t)
	bad := []map[string]any{
		{"name": "", "coord": map[string]any{"lat": 37.4, "lon": 138.8}, "radiusM": 3000},                      // 名前が空
		{"name": "x", "coord": map[string]any{"lat": 37.4, "lon": 138.8}, "radiusM": 2000},                     // 半径が選択肢にない
		{"name": "x", "coord": map[string]any{"lat": 10, "lon": 138.8}, "radiusM": 3000},                       // 日本の範囲外
		{"name": strings.Repeat("あ", 41), "coord": map[string]any{"lat": 37.4, "lon": 138.8}, "radiusM": 3000}, // 長すぎる
		{"name": "x", "coord": map[string]any{"lat": 26.2, "lon": 127.7}, "radiusM": 3000},                     // 市区町村が見つからない
	}
	for i, b := range bad {
		if code, body := call(t, srv, http.MethodPost, "/api/locations", "alice", b); code != http.StatusBadRequest {
			t.Errorf("%d: %d %s", i, code, body)
		}
	}
	if code, _ := call(t, srv, http.MethodGet, "/api/locations/not-a-ulid", "alice", nil); code != http.StatusBadRequest {
		t.Errorf("不正な ID: %d", code)
	}
}

func TestLocationLimit(t *testing.T) {
	srv, _ := newServer(t)
	for i := 0; i < httpapi.PlanLimits["free"]; i++ {
		createLocation(t, srv, "alice")
	}
	if code, _ := call(t, srv, http.MethodPost, "/api/locations", "alice", nagaoka); code != http.StatusConflict {
		t.Errorf("上限を超えて登録できた: %d", code)
	}
}

// authzCase は1つのエンドポイントの認可テスト。
type authzCase struct {
	method, path string
	body         any
}

// TestAuthorization は全エンドポイントについて、未認証は 401、他人の地点は 404 になることを確認する（docs/08 5.3）。
// 新しいエンドポイントを追加して、ここに追加し忘れると TestAuthzCoversAllOperations が失敗する。
func TestAuthorization(t *testing.T) {
	srv, _ := newServer(t)
	alice := createLocation(t, srv, "alice")
	upd := map[string]any{"name": "乗っ取り", "coord": map[string]any{"lat": 37.4462, "lon": 138.8512}, "radiusM": 3000, "version": 1}

	ownResource := []authzCase{
		{http.MethodGet, "/api/locations/" + alice.Id, nil},
		{http.MethodPut, "/api/locations/" + alice.Id, upd},
		{http.MethodDelete, "/api/locations/" + alice.Id, nil},
		{http.MethodGet, "/api/locations/" + alice.Id + "/timeline", nil},
	}
	for _, c := range ownResource {
		if code, body := call(t, srv, c.method, c.path, "bob", c.body); code != http.StatusNotFound {
			t.Errorf("bob が alice の地点に %s %s: %d %s", c.method, c.path, code, body)
		}
	}
	// bob の一覧に alice の地点は出ない
	if _, body := call(t, srv, http.MethodGet, "/api/locations", "bob", nil); strings.Contains(string(body), alice.Id) {
		t.Errorf("bob の一覧に alice の地点が出ている: %s", body)
	}
	// alice の地点は変わっていない
	if _, body := call(t, srv, http.MethodGet, "/api/locations/"+alice.Id, "alice", nil); !strings.Contains(string(body), "実家") {
		t.Errorf("alice の地点が変わった: %s", body)
	}

	for _, c := range allCases(alice.Id, upd) {
		if code, _ := call(t, srv, c.method, c.path, "", c.body); code != http.StatusUnauthorized {
			t.Errorf("未認証で %s %s: %d", c.method, c.path, code)
		}
	}
}

func allCases(locID string, upd any) []authzCase {
	return []authzCase{
		{http.MethodGet, "/api/me", nil},
		{http.MethodGet, "/api/me/notification-settings", nil},
		{http.MethodPut, "/api/me/notification-settings", map[string]any{"emailEnabled": true}},
		{http.MethodPost, "/api/me/notification-settings/test", nil},
		{http.MethodGet, "/api/locations", nil},
		{http.MethodPost, "/api/locations", nagaoka},
		{http.MethodGet, "/api/locations/" + locID, nil},
		{http.MethodPut, "/api/locations/" + locID, upd},
		{http.MethodDelete, "/api/locations/" + locID, nil},
		{http.MethodGet, "/api/locations/" + locID + "/timeline", nil},
		{http.MethodPost, "/api/me/delete", map[string]any{"confirm": "delete"}},
	}
}

func TestDeleteAccount(t *testing.T) {
	srv, _ := newServer(t)
	alice := createLocation(t, srv, "alice")
	bob := createLocation(t, srv, "bob")
	if code, _ := call(t, srv, http.MethodPost, "/api/me/delete", "alice", map[string]any{"confirm": "yes"}); code != http.StatusBadRequest {
		t.Errorf("確認の文字列が違うのに削除できた: %d", code)
	}
	if code, body := call(t, srv, http.MethodPost, "/api/me/delete", "alice", map[string]any{"confirm": "delete"}); code != http.StatusNoContent {
		t.Fatalf("退会: %d %s", code, body)
	}
	// 同じ sub でログインし直すと、新しい利用者として扱われ、以前の地点は見えない
	if _, body := call(t, srv, http.MethodGet, "/api/locations", "alice", nil); strings.Contains(string(body), alice.Id) {
		t.Errorf("退会後も地点が残っている: %s", body)
	}
	// 他人のデータは消えない
	if code, _ := call(t, srv, http.MethodGet, "/api/locations/"+bob.Id, "bob", nil); code != http.StatusOK {
		t.Errorf("bob の地点が消えた: %d", code)
	}
}

// TestAuthzCoversAllOperations は、OpenAPI の全操作（認証不要のものを除く）が allCases に含まれていることを確認する。
func TestAuthzCoversAllOperations(t *testing.T) {
	spec, err := apigen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	for _, c := range allCases("{locationId}", nil) {
		covered[c.method+" "+strings.TrimPrefix(c.path, "/api")] = true
	}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op.Security != nil && len(*op.Security) == 0 {
				continue // 認証不要（/health）
			}
			if !covered[method+" "+path] {
				t.Errorf("%s %s が認可テストの対象に入っていない", method, path)
			}
		}
	}
}
