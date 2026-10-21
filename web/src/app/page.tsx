import Link from "next/link";

import { Disclaimer } from "@/components/Disclaimer";

export default function Top() {
  return (
    <main className="main narrow">
      <h1>saigai-timeline</h1>
      <p className="lead">自宅・実家・学校など、登録した場所の周りで起きている災害の情報を、1本のタイムラインで確認できます。</p>
      <ul className="features">
        <li>気象庁の警報・注意報と地震の情報を、地点ごとにまとめて表示します</li>
        <li>災害の種類ごとに、表示と通知のレベルを選べます</li>
        <li>警報以上になったときや解除されたときに、メールで通知します</li>
      </ul>
      <Link href="/login" className="button">
        はじめる
      </Link>
      <Disclaimer />
    </main>
  );
}
