package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mmcloughlin/geohash"
	"github.com/oklog/ulid/v2"

	"github.com/masuzuki1028/saigai-portal/api/internal/apigen"
	"github.com/masuzuki1028/saigai-portal/api/internal/area"
	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
	"github.com/masuzuki1028/saigai-portal/api/internal/store"
)

func (s *Server) GetHealth(context.Context, apigen.GetHealthRequestObject) (apigen.GetHealthResponseObject, error) {
	return apigen.GetHealth200JSONResponse{Status: apigen.Ok}, nil
}

func (s *Server) GetMe(ctx context.Context, _ apigen.GetMeRequestObject) (apigen.GetMeResponseObject, error) {
	u, err := mustUser(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetMe200JSONResponse{UserId: u.ID, Plan: apigen.Plan(u.Plan), LocationLimit: PlanLimits[u.Plan]}, nil
}

func (s *Server) GetNotificationSettings(ctx context.Context, _ apigen.GetNotificationSettingsRequestObject) (apigen.GetNotificationSettingsResponseObject, error) {
	u, err := mustUser(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetNotificationSettings200JSONResponse{EmailEnabled: u.EmailEnabled}, nil
}

func (s *Server) PutNotificationSettings(ctx context.Context, req apigen.PutNotificationSettingsRequestObject) (apigen.PutNotificationSettingsResponseObject, error) {
	u, err := mustUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.Store.SetEmailEnabled(ctx, u.ID, req.Body.EmailEnabled); err != nil {
		return nil, err
	}
	return apigen.PutNotificationSettings200JSONResponse{EmailEnabled: req.Body.EmailEnabled}, nil
}

func (s *Server) SendTestNotification(ctx context.Context, _ apigen.SendTestNotificationRequestObject) (apigen.SendTestNotificationResponseObject, error) {
	u, err := mustUser(ctx)
	if err != nil {
		return nil, err
	}
	if !u.EmailEnabled {
		return apigen.SendTestNotification409JSONResponse(errBody("email_disabled", "メール通知がオフになっています")), nil
	}
	if err := s.Notifier.SendTest(ctx, u); err != nil {
		return nil, err
	}
	return apigen.SendTestNotification202Response{}, nil
}

// DeleteAccount は退会する。認証基盤（Cognito）のユーザーの削除は AWS への構築時に追加する。
func (s *Server) DeleteAccount(ctx context.Context, _ apigen.DeleteAccountRequestObject) (apigen.DeleteAccountResponseObject, error) {
	u, err := mustUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.Store.DeleteUserData(ctx, u); err != nil {
		return nil, err
	}
	s.Log.Info("account deleted", "userId", u.ID)
	return apigen.DeleteAccount204Response{}, nil
}

func (s *Server) ListLocations(ctx context.Context, _ apigen.ListLocationsRequestObject) (apigen.ListLocationsResponseObject, error) {
	u, err := mustUser(ctx)
	if err != nil {
		return nil, err
	}
	ls, err := s.Store.ListLocations(ctx, u.OwnerKey())
	if err != nil {
		return nil, err
	}
	out := apigen.ListLocations200JSONResponse{Items: make([]apigen.Location, 0, len(ls))}
	for _, l := range ls {
		al, err := s.toAPI(ctx, l)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, al)
	}
	return out, nil
}

func (s *Server) GetLocation(ctx context.Context, req apigen.GetLocationRequestObject) (apigen.GetLocationResponseObject, error) {
	l, err := s.ownLocation(ctx, req.LocationId)
	if errors.Is(err, store.ErrNotFound) {
		return apigen.GetLocation404JSONResponse{NotFoundJSONResponse: notFound()}, nil
	}
	if err != nil {
		return nil, err
	}
	al, err := s.toAPI(ctx, l)
	if err != nil {
		return nil, err
	}
	return apigen.GetLocation200JSONResponse(al), nil
}

func (s *Server) CreateLocation(ctx context.Context, req apigen.CreateLocationRequestObject) (apigen.CreateLocationResponseObject, error) {
	u, err := mustUser(ctx)
	if err != nil {
		return nil, err
	}
	n, err := s.Store.CountLocations(ctx, u.OwnerKey())
	if err != nil {
		return nil, err
	}
	if n >= PlanLimits[u.Plan] {
		return apigen.CreateLocation409JSONResponse(errBody("limit_exceeded", fmt.Sprintf("登録できる地点は %d 件までです", PlanLimits[u.Plan]))), nil
	}
	now := time.Now().UTC()
	l := store.Location{OwnerKey: u.OwnerKey(), ID: ulid.Make().String(), Version: 1, CreatedAt: now}
	if msg, err := s.fill(ctx, &l, *req.Body, now, false); err != nil {
		return nil, err
	} else if msg != "" {
		return apigen.CreateLocation400JSONResponse{BadRequestJSONResponse: apigen.BadRequestJSONResponse(errBody("bad_request", msg))}, nil
	}
	if err := s.Store.CreateLocation(ctx, l); err != nil {
		return nil, err
	}
	al, err := s.toAPI(ctx, l)
	if err != nil {
		return nil, err
	}
	return apigen.CreateLocation201JSONResponse(al), nil
}

func (s *Server) UpdateLocation(ctx context.Context, req apigen.UpdateLocationRequestObject) (apigen.UpdateLocationResponseObject, error) {
	old, err := s.ownLocation(ctx, req.LocationId)
	if errors.Is(err, store.ErrNotFound) {
		return apigen.UpdateLocation404JSONResponse{NotFoundJSONResponse: notFound()}, nil
	}
	if err != nil {
		return nil, err
	}
	if req.Body.Version != old.Version {
		return conflict(), nil
	}
	b := req.Body
	in := apigen.LocationInput{Name: b.Name, Coord: b.Coord, Address: b.Address, RadiusM: b.RadiusM, HazardSettings: b.HazardSettings}
	l := old
	l.Version = old.Version + 1
	// 座標が変わっていなければ、逆ジオコーダを呼び直さない
	sameCoord := false
	if pt, err := s.Sealer.Open(ctx, old.CoordEnc, []byte(old.ID)); err == nil {
		sameCoord = string(pt) == formatCoord(b.Coord.Lat, b.Coord.Lon)
	}
	if msg, err := s.fill(ctx, &l, in, time.Now().UTC(), sameCoord); err != nil {
		return nil, err
	} else if msg != "" {
		return apigen.UpdateLocation400JSONResponse{BadRequestJSONResponse: apigen.BadRequestJSONResponse(errBody("bad_request", msg))}, nil
	}
	if err := s.Store.UpdateLocation(ctx, old, l); errors.Is(err, store.ErrConflict) {
		return conflict(), nil
	} else if err != nil {
		return nil, err
	}
	al, err := s.toAPI(ctx, l)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateLocation200JSONResponse(al), nil
}

func conflict() apigen.UpdateLocation409JSONResponse {
	return apigen.UpdateLocation409JSONResponse(errBody("conflict", "ほかの画面で更新されています。読み込み直してください"))
}

func (s *Server) DeleteLocation(ctx context.Context, req apigen.DeleteLocationRequestObject) (apigen.DeleteLocationResponseObject, error) {
	l, err := s.ownLocation(ctx, req.LocationId)
	if errors.Is(err, store.ErrNotFound) {
		return apigen.DeleteLocation404JSONResponse{NotFoundJSONResponse: notFound()}, nil
	}
	if err != nil {
		return nil, err
	}
	if err := s.Store.DeleteLocation(ctx, l); err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	if err := s.Store.DeleteTimeline(ctx, l.ID); err != nil {
		return nil, err
	}
	return apigen.DeleteLocation204Response{}, nil
}

func (s *Server) GetTimeline(ctx context.Context, req apigen.GetTimelineRequestObject) (apigen.GetTimelineResponseObject, error) {
	l, err := s.ownLocation(ctx, req.LocationId)
	if errors.Is(err, store.ErrNotFound) {
		return apigen.GetTimeline404JSONResponse{NotFoundJSONResponse: notFound()}, nil
	}
	if err != nil {
		return nil, err
	}
	limit := 30
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	cursor := ""
	if req.Params.Cursor != nil {
		cursor = *req.Params.Cursor
	}
	items, next, err := s.Store.QueryTimeline(ctx, l.ID, limit, cursor)
	if errors.Is(err, store.ErrBadCursor) {
		return apigen.GetTimeline400JSONResponse{BadRequestJSONResponse: apigen.BadRequestJSONResponse(errBody("bad_cursor", "カーソルが正しくありません"))}, nil
	}
	if err != nil {
		return nil, err
	}
	out := apigen.GetTimeline200JSONResponse{Items: make([]apigen.TimelineItem, 0, len(items))}
	for _, it := range items {
		notified := it.Notified
		ti := apigen.TimelineItem{
			Id: it.ID, Kind: apigen.TimelineItemKind(it.Kind), Hazard: apigen.Hazard(it.Hazard), Severity: int(it.Severity),
			Reliability: apigen.TimelineItemReliability(it.Reliability), Title: it.Title, Summary: it.Summary,
			Status: apigen.TimelineItemStatus(it.Status), Source: it.Source, IssuedAt: it.IssuedAt, Notified: &notified,
		}
		if it.SourceURL != "" {
			u := it.SourceURL
			ti.SourceUrl = &u
		}
		out.Items = append(out.Items, ti)
	}
	if next != "" {
		out.NextCursor = &next
	}
	return out, nil
}

// ownLocation はログイン中の利用者の地点を取得する。所有者のキーはトークンから引いた userId で作るため、
// 他人の地点の ID を指定しても ErrNotFound になる（IDOR 対策。存在の有無も漏らさないよう 404 で返す）。
func (s *Server) ownLocation(ctx context.Context, id string) (store.Location, error) {
	u, err := mustUser(ctx)
	if err != nil {
		return store.Location{}, err
	}
	return s.Store.GetLocation(ctx, u.OwnerKey(), id)
}

// fill は入力値を検証して地点に反映する。入力の誤りは msg で返す。
func (s *Server) fill(ctx context.Context, l *store.Location, in apigen.LocationInput, now time.Time, keepArea bool) (msg string, err error) {
	name := trimName(in.Name)
	if name == "" || len([]rune(name)) > 40 {
		return "名前は1〜40文字で入力してください", nil
	}
	if !in.RadiusM.Valid() {
		return "半径が正しくありません", nil
	}
	hs := domain.DefaultHazardSettings()
	if in.HazardSettings != nil {
		hs = fromAPISettings(*in.HazardSettings)
	}
	if err := hs.Validate(); err != nil {
		return "災害の設定が正しくありません", nil
	}
	a := area.Area{MuniCode: l.MuniCode, Name: l.AreaName, WarnKey: l.WarnKey, QuakeKey: l.QuakeKey, IsCoastal: l.IsCoastal}
	tsunamiKey, tsunamiName := l.TsunamiKey, l.TsunamiName
	if !keepArea {
		// 海岸線から一定の距離内なら沿岸の地点とし、一番近い津波予報区に結び付ける
		z, _, coastal := area.NearestTsunamiZone(in.Coord.Lat, in.Coord.Lon)
		tsunamiKey, tsunamiName = z.Code, z.Name
		a, err = s.Resolver.Resolve(ctx, in.Coord.Lat, in.Coord.Lon)
		if errors.Is(err, area.ErrNotFound) {
			return "この場所の市区町村が見つかりません", nil
		}
		if err != nil {
			return "", fmt.Errorf("resolve area: %w", err)
		}
		a.IsCoastal = coastal
	}
	coord, err := s.Sealer.Seal(ctx, []byte(formatCoord(in.Coord.Lat, in.Coord.Lon)), []byte(l.ID))
	if err != nil {
		return "", err
	}
	l.AddressEnc = nil
	if in.Address != nil && strings.TrimSpace(*in.Address) != "" {
		if l.AddressEnc, err = s.Sealer.Seal(ctx, []byte(strings.TrimSpace(*in.Address)), []byte(l.ID)); err != nil {
			return "", err
		}
	}
	l.Name, l.RadiusM, l.HazardSettings = name, int(in.RadiusM), hs
	l.Geohash6 = geohash.EncodeWithPrecision(in.Coord.Lat, in.Coord.Lon, 6)
	l.CoordEnc = coord
	l.MuniCode, l.AreaName, l.WarnKey, l.QuakeKey, l.IsCoastal = a.MuniCode, a.Name, a.WarnKey, a.QuakeKey, a.IsCoastal
	l.TsunamiKey, l.TsunamiName = tsunamiKey, tsunamiName
	l.UpdatedAt = now
	l.WrittenBy = s.Store.WrittenBy
	return "", nil
}

func formatCoord(lat, lon float64) string {
	return strconv.FormatFloat(lat, 'f', 6, 64) + "," + strconv.FormatFloat(lon, 'f', 6, 64)
}

func parseCoord(s string) (lat, lon float64, err error) {
	a, b, ok := strings.Cut(s, ",")
	if !ok {
		return 0, 0, errors.New("bad coord")
	}
	if lat, err = strconv.ParseFloat(a, 64); err != nil {
		return 0, 0, err
	}
	lon, err = strconv.ParseFloat(b, 64)
	return lat, lon, err
}

// toAPI は地点を API の形にする。正確な座標・住所は本人の画面表示のときだけ復号する。
func (s *Server) toAPI(ctx context.Context, l store.Location) (apigen.Location, error) {
	pt, err := s.Sealer.Open(ctx, l.CoordEnc, []byte(l.ID))
	if err != nil {
		return apigen.Location{}, fmt.Errorf("open coord: %w", err)
	}
	lat, lon, err := parseCoord(string(pt))
	if err != nil {
		return apigen.Location{}, err
	}
	out := apigen.Location{
		Id: l.ID, Name: l.Name, Coord: apigen.LatLon{Lat: lat, Lon: lon}, RadiusM: apigen.RadiusM(l.RadiusM),
		Geohash6: l.Geohash6, AreaName: l.AreaName, IsCoastal: l.IsCoastal, HazardSettings: toAPISettings(l.HazardSettings),
		Version: l.Version, CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt, CurrentLevels: []apigen.CurrentLevel{},
	}
	if l.TsunamiName != "" {
		n := l.TsunamiName
		out.TsunamiAreaName = &n
	}
	if len(l.AddressEnc) > 0 {
		addr, err := s.Sealer.Open(ctx, l.AddressEnc, []byte(l.ID))
		if err != nil {
			return apigen.Location{}, fmt.Errorf("open address: %w", err)
		}
		a := string(addr)
		out.Address = &a
	}
	states, err := s.Store.ListStates(ctx, l.ID)
	if err != nil {
		return apigen.Location{}, err
	}
	out.CurrentLevels = currentLevels(states, l.HazardSettings, time.Now())
	return out, nil
}

// currentLevels は種別ごとに、表示の対象になっている現在のレベルを返す。
// 警報・注意報は発表中のもの、地震は24時間以内に表示のしきい値以上を観測したもの。
func currentLevels(states []store.HazardState, hs domain.HazardSettings, now time.Time) []apigen.CurrentLevel {
	best := map[domain.Hazard]store.HazardState{}
	for _, st := range states {
		if st.Severity == 0 {
			continue
		}
		if st.Hazard == domain.Earthquake {
			if now.Sub(st.ReportTime) > 24*time.Hour || !st.Intensity.AtLeast(hs.Earthquake.DisplayMinIntensity) {
				continue
			}
		} else if d := hs.Level(st.Hazard).Display; d == 0 || st.Severity < d {
			continue
		}
		if cur, ok := best[st.Hazard]; !ok || st.Severity > cur.Severity {
			best[st.Hazard] = st
		}
	}
	out := []apigen.CurrentLevel{}
	for _, hz := range domain.Hazards {
		if st, ok := best[hz]; ok {
			out = append(out, apigen.CurrentLevel{Hazard: apigen.Hazard(hz), Severity: int(st.Severity), Title: st.Title})
		}
	}
	return out
}

func notFound() apigen.NotFoundJSONResponse {
	return apigen.NotFoundJSONResponse(errBody("not_found", "見つかりません"))
}

func fromAPISettings(h apigen.HazardSettings) domain.HazardSettings {
	lv := func(l apigen.LevelSetting) domain.LevelSetting {
		return domain.LevelSetting{Display: domain.Severity(l.Display), Notify: domain.Severity(l.Notify)}
	}
	return domain.HazardSettings{
		Rain: lv(h.Rain), Flood: lv(h.Flood), Landslide: lv(h.Landslide), Tsunami: lv(h.Tsunami),
		Earthquake: domain.EarthquakeSetting{
			DisplayMinIntensity: domain.Intensity(h.Earthquake.DisplayMinIntensity),
			NotifyMinIntensity:  domain.Intensity(h.Earthquake.NotifyMinIntensity),
		},
	}
}

func toAPISettings(h domain.HazardSettings) apigen.HazardSettings {
	lv := func(l domain.LevelSetting) apigen.LevelSetting {
		return apigen.LevelSetting{Display: int(l.Display), Notify: int(l.Notify)}
	}
	return apigen.HazardSettings{
		Rain: lv(h.Rain), Flood: lv(h.Flood), Landslide: lv(h.Landslide), Tsunami: lv(h.Tsunami),
		Earthquake: apigen.EarthquakeSetting{
			DisplayMinIntensity: apigen.Intensity(h.Earthquake.DisplayMinIntensity),
			NotifyMinIntensity:  apigen.Intensity(h.Earthquake.NotifyMinIntensity),
		},
	}
}
