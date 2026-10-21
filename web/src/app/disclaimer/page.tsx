import Link from "next/link";

import { Disclaimer } from "@/components/Disclaimer";

export default function Page() {
  return (
    <main className="main narrow">
      <h1>免責・出典</h1>
      <Disclaimer />
      <h2>出典</h2>
      <ul>
        <li>
          警報・注意報、地震の情報: 気象庁防災情報 XML（
          <a href="https://xml.kishou.go.jp/" target="_blank" rel="noopener noreferrer">
            気象庁
          </a>
          ）を加工して作成
        </li>
        <li>
          地図: 
          <a href="https://maps.gsi.go.jp/development/ichiran.html" target="_blank" rel="noopener noreferrer">
            地理院タイル
          </a>
          、市区町村の判定: 国土地理院 逆ジオコーダ、住所検索: 国土地理院 住所検索
        </li>
        <li>
          津波予報区の判定: 気象庁 防災情報XMLで用いるコードが示す地域の GIS データ（津波予報区）を加工して作成
        </li>
      </ul>
      <Link href="/">トップへ</Link>
    </main>
  );
}
