package domain

// ItemStatus はタイムライン項目の状態。
type ItemStatus string

const (
	StatusIssued    ItemStatus = "issued"    // 発表・レベルが上がった
	StatusContinued ItemStatus = "continued" // レベルが下がったが表示の対象のまま
	StatusCleared   ItemStatus = "cleared"   // 解除・表示の対象を下回った
	StatusCorrected ItemStatus = "corrected" // 訂正報で内容が変わった
	StatusCancelled ItemStatus = "cancelled" // 取消報で取り消された
)

// Decision は、ある地点・種別の状態が old から new に変わったときの判定結果。
type Decision struct {
	AddItem bool       // タイムラインに項目を追加するか
	Status  ItemStatus // 追加する項目の状態
	Notify  bool       // 通知するか
}

// Decide は警報・注意報の状態の変化から、タイムラインへの追加と通知を判定する。
//
//   - 表示: 新しいレベルが表示のしきい値以上のとき、または表示の対象から外れたとき（解除）に項目を追加する
//   - 通知: 通知のしきい値を上回る方向にレベルが上がったとき、または通知の対象から外れたときだけ送る
//     （同じレベルの継続や、しきい値未満での変化では送らない。docs/05_system_architecture.md 3.3）
//
// しきい値が 0 のときは OFF として扱う。changed は、レベルが同じでも種類（例: 大雨警報に洪水警報が加わった）が
// 変わったかどうか。
func Decide(old, new Severity, changed bool, setting LevelSetting) Decision {
	if old == new && !changed {
		return Decision{}
	}
	var d Decision
	if setting.Display > 0 {
		switch {
		case new >= setting.Display && new > old:
			d = Decision{AddItem: true, Status: StatusIssued}
		case new >= setting.Display && (new < old || changed):
			d = Decision{AddItem: true, Status: StatusContinued}
		case new < setting.Display && old >= setting.Display:
			d = Decision{AddItem: true, Status: StatusCleared}
		}
	}
	if setting.Notify > 0 {
		up := new >= setting.Notify && new > old
		cleared := old >= setting.Notify && new < setting.Notify
		d.Notify = up || cleared
	}
	return d
}
