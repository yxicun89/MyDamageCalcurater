# ADR-0210: 私設サービスの境界(issue #148)

- 状態: 採用(2026-09-23。issue #148 の API レーン担当分。critic PASS。cloud overlay から gateway の Ingress を除去する削除 patch を実装し、AC-B1〜B3 は緑。`kubectl kustomize deploy/k8s/overlays/cloud` に Ingress・LoadBalancer・NodePort が無いことを確認済み)
- 日付: 2026-09-23
- 関連: issue #148(監査ベース `705183e30f6a2afb9d4e322a20cab1079b86e615`)、
  docs/ai-shared/DECISIONS.md 2026-09-23「issue #148(クラウド公開前のアクセス境界・認証方針)をユーザーが決定」、
  ADR-0202(gateway のルーティング・ヘッダ検証・CORS)、ADR-0203(k3d デプロイと `deploytest` の静的検査)、
  ADR-0209(M2 保存データの保持・削除・端末 ID 境界。§1 の公開範囲・§2 の端末 ID の位置づけ)、
  ADR-0012(サービス境界。`/api/balance` は balance 自身の Ingress)、ADR-0104 §8(cloud overlay で CronJob を suspend)、
  docs/requirements.md §1(利用者は自分1人・認証不要)・§3(認証なし・Tailscale)・§4(アーキテクチャ)・§5(`tailscale serve` で HTTPS)、
  docs/coding-rules.md(秘密値を Git に置かない)

## 背景

クラウドへ移す前に「自分だけが使う私設サービス」か「インターネット公開サービス」かを決める必要がある(issue #148)。
現状は次のようになっている。

- `deploy/k8s/overlays/cloud/kustomization.yaml` は `../../base` をそのまま読む。
- `deploy/k8s/base/gateway/ingress.yaml` は **host 指定なし・HTTP の `/` Prefix・TLS 設定なし**の Ingress で、
  `ingressClassName: traefik`(k3d の同梱 Traefik)を指す。クラウドの一般的な Ingress Controller に同じ
  マニフェストを渡すと、公開ロードバランサーと公開 IP ができうる。
- `services/gateway/internal/httpapi/headers.go` はクライアントが自己生成する `X-Device-Id` / `X-Session-Id` の
  **形式だけ**を検証する(署名も秘密も所有者照合もない)。認証ではない。
- `services/gateway/internal/httpapi/cors.go` の CORS はブラウザの約束であり、curl やネイティブ iOS クライアントに対する
  到達制御にはならない。
- `api/openapi.yaml` に `securitySchemes` は無く、401 / 403 を返す操作も無い。

ユーザー決定(DECISIONS.md 2026-09-23、issue #148 の選択肢1・既定案):
**私設サービスを維持する。Tailscale 等の private overlay network だけから gateway へ到達させ、
public LoadBalancer / Ingress を作らない。端末 ID は引き続き認証ではない。**

この ADR は、その決定を API レーンの範囲(gateway・Ingress・OpenAPI・API 契約)で具体的な方針・受け入れ条件・
テストにする。Web / iOS の接続方法の詳細、provider 別 overlay の具体構成、監視・ログ・失効 runbook の本体は
それぞれのレーンの担当で、この ADR では「他レーンへの依頼」として §7 に置く。

## 決定

### 1. 選択の記録(私設サービスを維持する)

v1 も、クラウドへ移した後も、**到達経路は private overlay network(Tailscale の tailnet)だけ**にする。
public な IP・DNS 名・ロードバランサーを作らない。ADR-0209 §1 の「v1 は個人利用 + Tailscale 内に固定する」を、
**クラウドを含む全環境の恒久的な方針**に引き上げる(§6)。

#### 1.1 対象利用者

自分1人(requirements.md §1)。端末は「自分の iPhone」「自分の Mac のブラウザ」だけで、
同時に使う端末が2〜3台を超えることは想定しない。他人にアカウントを配る計画は無い。

#### 1.2 脅威モデル

**想定する攻撃者**

| # | 攻撃者 | 能力 | この境界での扱い |
|---|---|---|---|
| T1 | インターネット上の無差別スキャナ / ボット | 公開 IP とポートを総当たりで叩く。認証のない API を見つけたら使う | **public IP を作らない**ことで到達そのものを消す(§2)。唯一の防御線 |
| T2 | 端末 ID を推測・総当たりしようとする者 | UUID を大量に試す | **到達できない**ので成立しない。端末 ID 自体は防御に数えない(§4) |
| T3 | ブラウザ経由のクロスオリジンの読み出し | 悪意のあるページを利用者に開かせ、fetch で API を読む | CORS の許可オリジン完全一致(ADR-0202 §6)で**ブラウザに限っては**防ぐ。CORS を到達制御には数えない(§4) |
| T4 | tailnet 内の別ノード | tailnet に参加している別の端末から gateway へ到達する | tailnet の ACL で「この利用者の端末 tag」だけに絞る(運用レーン。§7)。利用者は自分1人なので、ここは越境の「事故」を防ぐ層 |
| T5 | 経路上の盗聴・改ざん | 公衆 Wi-Fi 等でパケットを見る | tailnet 内は WireGuard で暗号化される(§3) |

**想定しない攻撃者(この境界で守らない)**

- Mac を物理的に操作できる人(クラスタ・DB のすべてに届く。家庭内の前提)。
- Tailscale / Apple / クラウド事業者(基盤の事業者を信頼する前提で使う)。
- 利用者自身の端末上のマルウェア(端末 ID も含め、端末の中身は端末の安全性に依存する)。

**守るもの**

1. **可用性と計算資源**: 認証のない計算 API が無差別に叩かれないこと。issue #110(ADR-0208)で本文サイズの
   上限は入れたが、レート制限・同時実行数は未実装で、public に置けば CPU を消費させられる。
2. **将来の個人データ(M2)の秘匿と完全性**: 履歴・お気に入り・構築(ADR-0209 の #1〜#4)。
   これらは端末 ID で分けるだけで、認証も認可も無い(§4)。public に置くと任意の端末 ID を名乗って読み書きできる。
3. **第三者データを再配布しないこと**: マスタ(ポケモンのデータ・画像)は Git にも置かない(ADR-0002)。
   public な API として配れば、実質的に第三者データの再配布になる(requirements.md §3「知財」)。

**守らないもの(明示)**

- **端末 ID の秘匿**。端末 ID は Web の localStorage・iOS の UserDefaults に平文で置き、失われたら作り直す値で、
  秘密として守れない(ADR-0209 §2)。この ADR でもそれを変えない。
- **tailnet の内側での「利用者ごとの分離」**。利用者は1人なので、分離すべき主体が無い。

#### 1.3 端末紛失・端末 ID 漏えい時の扱い(ADR-0209 §2 との整合)

ADR-0209 §2 は「端末 ID は秘密でないので、失効・再発行の概念を持たない」と決めている。この ADR はそれを変えず、
**失効はアプリ層ではなくネットワーク層(tailnet)で行う**と位置づける。整合を確認した結果は次のとおり。

| 事象 | 対応する層 | 具体的な手当て | 残るリスク |
|---|---|---|---|
| 端末を紛失した | **tailnet** | その端末を tailnet から外す(Tailscale の admin console で device を remove / key を失効)。以後その端末は gateway に到達できない | 端末内に残る端末 ID とローカルのキャッシュ。端末のロック・リモート消去に依存する(この境界の外) |
| 端末 ID が漏れた(ログ・スクリーンショット等) | **どちらの層でもない(手当てしない)** | 何もしない。端末 ID を知っていても tailnet の外からは到達できない(T2)。tailnet の中で別端末がその端末 ID を名乗れるが、利用者は自分1人なので実害は無い | 「同じ端末 ID を名乗る別端末が自分のデータを見る」= tailnet に入れた端末が自分の端末だけである限り起きない |
| 端末 ID を作り直したい(端末の初期化など) | **アプリ層。ただし失効ではない** | 新しい端末 ID になり、前のデータには戻れない(ADR-0209 §2 の通り)。古いデータは保持期間で自然に消える(ADR-0209 §3・§4) | 意図せず端末 ID を失うとデータに戻れない。ADR-0209 §8 の文言で利用者に伝える |
| 運用者の資格情報(クラスタ・DB)が漏れた | **運用レーン** | tailnet とクラウド側の資格情報の失効手順。runbook(§7) | この ADR の範囲外 |

**結論**: 端末 ID に失効・再発行・ローテーションの仕組みを**足さない**(ADR-0209 §2 の却下案どおり)。
「失効」に相当する操作は「tailnet からその端末を外す」であり、端末 ID とは別のレイヤーにある。
この分離が成り立つのは public IP が無いからで、§2 が崩れると同時に崩れる。

### 2. `overlays/cloud` の方針: 公開の入口を作らない

#### 2.1 決定

`deploy/k8s/overlays/cloud` の描画結果に、**`kind: Ingress` を1つも含めない**。
Service の `spec.type` は `ClusterIP`(既定)だけで、**`LoadBalancer` も `NodePort` も作らない**。
同じ理由で、`Service.spec.externalIPs`・Pod の `hostNetwork: true`・コンテナの `hostPort` も使わない
(いずれも Service の type に関わらずノードの外部 IP やネットワークを直接公開しうる経路のため)。
クラウドでの到達経路は Ingress Controller ではなく、**tailnet に直接参加する経路**にする。

到達経路の第一候補(実装は別タスク・運用レーンと共同):

1. **Tailscale Kubernetes Operator の `tailscale` ingressClass**、または
2. **tailnet に参加した subnet router / `tailscale serve` を前段に置き、gateway の `ClusterIP` Service へ転送する**。

どちらも tailnet の内側にだけ名前と到達性を作り、クラウドの public な L4/L7 ロードバランサーを作らない。
どちらを採るかは provider(AWS/GCP)を決めるときに運用レーンと決める(§7)。この ADR は
「**public な入口を作らない**」「**cloud overlay に hostless HTTP Ingress を残さない**」の2点だけを固定する。

**候補1(Tailscale Operator の `tailscale` ingressClass)を採る場合の注意**: この方式は `kind: Ingress` を
Kubernetes リソースとして作る(ただし `ingressClassName: tailscale` で、public な Ingress Controller には
渡らない)。§5 の AC-B1・AC-B2 は現状「`kind: Ingress` を1つも含めない」という条件で書いており、
候補1を採用する時点でこの条件とテストは**そのままでは矛盾する**。候補1に進むときは、AC-B1・AC-B2 を
「`ingressClassName` が `tailscale`(または private であることが明らかな値)以外の Ingress が無い」という
条件に改めるか、`tailscale` ingressClass の Ingress を検査から除外する追記をこの ADR に行うこと。
テストの条件を先に緩めてはいけない(絶対ルール6)。

#### 2.2 却下した案: Ingress に host 制約と TLS を足す

`base` の Ingress に `host:` と `tls:` を足すだけでは、ユーザー決定「public LoadBalancer / Ingress を作らない」を
満たさない。理由:

- Ingress Controller が public なロードバランサーと public IP を作るかどうかは **ingressClass と
  Controller の設定**で決まる。`host` はその上でのルーティング条件にすぎず、**公開 IP は作られたまま**になる。
- host 不一致を 404 にしても、公開 IP は T1(無差別スキャナ)に見えており、DNS 名が分かれば届く。
  「守る層が1枚しかない」(§1.2 の T1)という前提と噛み合わない。
- TLS を足すと証明書と鍵の管理が発生する(§3 で却下する理由と同じ)。

つまり host / TLS は「公開した上で絞る」ための道具で、「公開しない」の実現手段ではない。

#### 2.3 `base` の Ingress は残す(local 専用と明記する)

`deploy/k8s/base/gateway/ingress.yaml` の hostless HTTP Ingress は、k3d 同梱の Traefik で
`localhost:8080` から受けるための**現役の local 専用の経路**で、ADR-0012(`/api/balance` が最長一致で balance の
Ingress に届く)の前提でもある。`base` から取り除くと `overlays/local` と `overlays/local-api` の両方に
重複して置くことになり、最長一致の前提が2か所に散る。

そこで `base` には残し、次の2つで「無検討で公開されない」ことを担保する。

1. **`ingress.yaml` の先頭コメントに、この Ingress が local(k3d / Traefik)専用であること、cloud overlay では
   使わないこと、ADR-0210 を書く**。読んだ人が cloud にそのまま持ち出さないようにする。
2. **静的テストで、cloud overlay の描画結果に Ingress が残らないことを機械的に固定する**(§5)。

実装手段(cloud overlay で `$patch: delete` の削除 patch を置く / `resources` を `../../base` から
Ingress を含まない粒度に分解する)は implementer の判断に任せる。§5 のテストはどちらでも通る条件を見る。

### 3. TLS 終端: gateway では終端しない

#### 3.1 決定

**gateway 自身でも、クラスタ内の Ingress でも TLS を終端しない。** 外部からの入口で HTTPS を終端するのは
**Tailscale**(`tailscale serve` / Tailscale Operator)で、tailnet 内の通信は WireGuard で暗号化される。
証明書は Tailscale が tailnet の MagicDNS 名(Tailscale が管理するドメインサフィックス配下)に対して自動発行・自動更新し、
**証明書と鍵は Git にもクラスタの Secret にも入らない**(coding-rules「秘密値を Git に置かない」を構造的に満たす)。

issue #148 の共通の受け入れ条件「全経路で TLS を終端し、HTTP から HTTPS へ転送し、証明書更新と
秘密値を Git へ置かない方法を決める」は、private 案では次の形で満たす。

| 区間 | 暗号化 | 終端・鍵の持ち主 |
|---|---|---|
| クライアント(iOS / ブラウザ)→ tailnet の入口 | **HTTPS**(`tailscale serve` の自動証明書) | Tailscale。鍵はノードのローカル(Git にもクラスタにも無い) |
| tailnet 内(入口 → クラスタ) | **WireGuard**(平文の HTTP をトンネルの中で運ぶ) | Tailscale。鍵は各ノードの tailscaled |
| クラスタ内(Ingress / Service → gateway → 上流) | 平文 HTTP | ― (クラスタ内。mTLS は導入しない) |

**HTTP → HTTPS の転送**: `tailscale serve` は HTTPS のリスナだけを公開し、平文 HTTP の入口を作らない
(「転送する」のではなく「平文の入口が存在しない」形で満たす)。クラスタ内の hostless HTTP Ingress は
§2 のとおり cloud には出さないので、外から触れる平文の入口は無い。

#### 3.2 却下した案

- **gateway に TLS を直接持たせる**: 証明書と鍵を環境変数か Secret で渡すことになり、クラスタが秘密値を持つ。
  gateway の責務(ルーティングとヘッダ検証。ADR-0202)も増える。守るものは増えない(§3.1 の表のとおり
  すでに暗号化されている区間を二重にするだけ)。
- **cert-manager + Let's Encrypt(DNS-01)**: public な DNS 名を持たない私設サービスでは HTTP-01 が使えず、
  DNS-01 のために DNS プロバイダの API トークンをクラスタに置くことになる。public IP を作らない方針(§2)と
  比べて守るものが増えないのに、秘密値が1つ増える。
- **自己署名 CA + cert-manager**: iOS / ブラウザに CA を入れる運用が増え、無料 Apple ID の7日再署名
  (requirements.md §3)と合わせて手間が増える。
- **クラスタ内 mTLS(service mesh)**: 学習目的としては面白いが、脅威モデル(§1.2)に対応する項目が無い。
  必要になったら別 ADR。

### 4. CORS と端末 ID を認証として扱わない(既存決定の再確認と回帰テスト)

#### 4.1 再確認した既存の決定(変えない)

| 決定 | 出典 | この ADR での扱い |
|---|---|---|
| `X-Device-Id` は保存データの**分割キー**で、秘密・認証・所有権の証明・ベアラトークンではない | ADR-0209 §2 | 変えない。§1.3 で失効の層を明確にしただけ |
| `X-Session-Id` は分割キーにしない | ADR-0209 §2 | 変えない |
| gateway は UUID の**形式だけ**を検証し、通れば書き換えずに上流へ転送する | ADR-0202 §4 | 変えない |
| 形式が不正なら 400 `missing_header` / `invalid_header`(401 / 403 ではない) | ADR-0202 §4・ADR-0200 | 変えない。**認証の語彙を契約に持ち込まない**ことを §4.3 でテストに固定する |
| CORS は許可オリジン完全一致のときだけ ACAO を付ける。`Access-Control-Allow-Credentials` は付けない、`*` は使わない | ADR-0202 §6 | 変えない |
| 契約に `securitySchemes` は無い | `api/openapi.yaml`(現状) | 変えない。**無いことをテストで固定する**(§4.3) |

#### 4.2 既存テストの調査結果(何が足りないか)

既にあるもの:

- `services/internal/api/client_id_semantics_test.go` の `TestClientIDParameterSemantics`:
  契約の `DeviceId` / `SessionId` の description に「分割キー」「認証ではない」「ADR-0209」があることを検査する。
- `services/gateway/internal/httpapi/headers_test.go` の `TestHeaderValidationAccepts`:
  任意の形式の正しい UUID(版を問わない・大文字小文字を問わない)がそのまま上流へ届くことを検査する。
  結果として「事前登録・払い出しの概念が無い」ことを部分的に固定している。
- `TestHeaderValidationRejects`: 欠落・不正が 400 `missing_header` / `invalid_header` であること。
- `services/gateway/internal/httpapi/cors_test.go` の `TestCORSSimpleRequests`:
  許可外オリジン・`Origin` 無しのときに CORS ヘッダが**付かない**ことを検査する。

**足りないもの(この ADR で足す)**

| # | 足りない検査 | なぜ必要か |
|---|---|---|
| G1 | gateway が `Authorization` / `Cookie` / `X-Api-Key` を**一切見ない**(付けても付けなくても結果が変わらない・これらを理由に 401 / 403 を返さない) | 将来「`Authorization` があれば通す」等の半端な認証が入ると、端末 ID が認証に見える状態が生まれる。今は誰も検査していない |
| G2 | 端末 ID を変えても、同じ本文が同じ上流へ同じ内容で届く(端末 ID が gateway の分岐に使われない) | 端末 ID による「認可の分岐」が gateway に生えないようにする(分割キーは保存サービスの責務。ADR-0209 §6) |
| G3 | 不正・欠落の端末 ID は 400 で、**401 / 403 ではない** ことを明示的に固定 | 認証の語彙(401 / 403)が入ると「端末 ID = 資格情報」と読めてしまう |
| G4 | 許可外オリジン・`Origin` 無しでも **200 で上流に届く**(CORS はブラウザの約束であって到達制御ではない) | 既存テストは「ヘッダが付かない」だけを見て、到達したかを見ていない。CORS を到達制御と誤認すると、public に置いても大丈夫だと誤読しうる |
| G5 | 契約(`api/openapi.yaml`)に認証の語彙が無い: `securitySchemes` 無し・`security` 無し・401 / 403 の応答無し・`ErrorCode` に `unauthorized` / `forbidden` 等が無い・`Authorization` ヘッダのパラメータが無い | 「認証を足す」変更が**無言では入らない**ようにする。足すときはこのテストが落ち、ADR(§6 の条件)に戻る |

#### 4.3 新しいテストの設計

- `services/gateway/internal/httpapi/private_boundary_test.go`(新規)
  - `TestDeviceIDIsNotAuthentication`: G1・G2・G3。
  - `TestCORSIsNotAccessControl`: G4。
- `services/internal/api/no_authentication_test.go`(新規)
  - `TestContractHasNoAuthentication`: G5。

これらは**現在の挙動を固定する回帰テスト**なので、書いた時点で緑になる(§5 の AC-B1〜B3 だけが赤で始まる)。

### 5. 静的テスト(`overlays/cloud` が無検討で公開されない)

`services/gateway/deploytest`(ADR-0203 §3。kubectl に依存しない YAML の静的検査)を踏襲する。
`deploytest` は Kustomize を描画しないので、**2層**で確かめる。

1. **構造の検査(常に走る)**: `deploytest` に `OverlayObjects`(`resources` / `components` を再帰的に辿り、
   `$patch: delete` の削除 patch を適用して「残るオブジェクト」を返す)を足し、cloud overlay に
   Ingress が残らないこと・`LoadBalancer` / `NodePort` の Service が無いこと・Service の `externalIPs`・
   Deployment の `hostNetwork: true`・コンテナの `hostPort` が無いことを検査する。
   さらに overlay 配下のファイルを本文検索して、patch でこれらを後付けしていないことも見る
   (`OverlayObjects` は削除以外の patch を適用しないため、その穴を本文検索で塞ぐ)。
2. **描画の検査(`kubectl` があるときだけ。無ければ skip)**: `kubectl kustomize deploy/k8s/overlays/cloud` の
   出力に `kind: Ingress`・`type: LoadBalancer`・`type: NodePort`・`externalIPs`・`hostNetwork: true`・
   `hostPort` が無いことを検査する。
   実装手段に依存しない「意図そのもの」の検査で、1 の再現の限界(削除以外の patch を見ない)を補う。
   `smoke_test.go` が curl の無い環境で skip する前例に合わせる。
   (critic のレビューで見つかった穴: `externalIPs`・`hostNetwork`・`hostPort` は当初この2層をすり抜けた。
   `Service`/`PodSpec`/`ContainerPort` にフィールドを足し、変異テスト(値を注入して赤になることを確認)で
   検出を確かめた上でこの版に反映した)

加えて `base/gateway/ingress.yaml` の先頭コメントが §2.3 の内容(local / k3d 専用・cloud では使わない・ADR-0210)を
持つことを検査する(`web_docs_test.go` / `pokedex_docs_test.go` と同じ文書検査のパターン)。

`make k8s-render`(`Makefile`。`make lint` から呼ばれる)は cloud overlay を `kubectl kustomize` で描画できることを
既に確かめているので、これは変えない。

### 6. ADR-0209 §1 との関係(「公開へ進む判断」は「公開しない」で確定)

ADR-0209 §1 は「v1 は個人利用 + Tailscale 内に固定する」とし、末尾の「人間の確認が必要なこと」に
**「クラウド公開へ進む判断」**を人間確認待ちとして残していた。issue #148 のユーザー決定(DECISIONS.md 2026-09-23)で
この判断は **「公開しない」で確定**した。

したがって:

- ADR-0209 §1 の「v1 は…に固定する」は、**v1 に限らず恒久的な方針**になった(この ADR §1)。
- ADR-0209 §1 の「インターネットに公開する前に認証方式を別 ADR で必須決定とする」は**生きている**。
  ただし前提条件が変わり、「公開に進むとき」は**この決定(issue #148)を明示的に覆すとき**だけになった。
- ADR-0209 は採用済みなので大きく書き換えない。§1 と末尾に**追記の1〜2行**を足すだけにする(この ADR で実施済み)。

公開へ進み直すときに満たすべき条件(将来の別 ADR の入口。ここでは列挙だけ):
認証主体とデータ所有境界の決定(ADR-0209 §2 の分割キーを格下げする)、`api/openapi.yaml` への `securitySchemes` の追加
(§4.3 の `TestContractHasNoAuthentication` が落ちる)、gateway のレート制限・同時実行数・監査ログ、
Web / iOS のトークン保管、認証なしで許す endpoint の列挙。

### 7. 他レーンへの依頼(DECISIONS.md への転記は 2026-09-23 の追記エントリ)

| レーン | 依頼 |
|---|---|
| 運用 | tailnet の ACL(利用端末の tag と運用者を分ける)、端末の失効手順(§1.3)、クラウドでの到達経路(§2.1 の1か2)の選定と導入、監視・ログの経路が public IP を作らないこと |
| Web | API の base URL を tailnet の MagicDNS 名にし、public な既定値を持たない。CORS 許可オリジンも tailnet 上の名前だけ |
| iOS | 同上。`tailscale serve` が HTTPS を終端するので ATS の例外(平文許可)を作らない |
| データ / タイプバランス / 素早さ | 各レーンの `services/*/deploy/k8s/base/ingress.yaml`(balance・speed・judge)を root の cloud overlay に入れる日が来たら、同じ制約(public Ingress / LoadBalancer を作らない)を適用する。**今は root の `deploy/k8s/base` に入っていないので追加対応は無い** |

### 8. 決めないこと(この ADR の範囲外)

- provider(AWS / GCP)の選定と provider 別 overlay の具体構成。
- レート制限・同時実行数(ADR-0208 の残存リスク。private でも学習目的で入れてよいが別タスク)。
- 監視・ログ・バックアップの runbook 本体(運用レーン・P7-4)。
- Web / iOS の接続設定の実装。
- **`make gen` の生成物ドリフト検査**: `TestContractHasNoAuthentication`(§4.3・G5)は `api.GetSwagger()` 経由で
  仕様を読む。これは `api/openapi.yaml` を編集した直後・`make gen` 前の一時的な状態には効かないが、
  この契約テストの弱点は本 ADR 固有ではなく、`services/*/internal/httpapi/contract_test.go` など既存の契約テスト
  全てに共通する前提(絶対ルール1「API変更は必ず openapi.yaml から」で `make gen` が徹底される前提)。
  この ADR だけこの1テストを直読みに変えるのは既存パターンと不整合になるため見送り、リポジトリ共通の
  生成物ドリフト検査(CI での `make gen` 後 `git diff --exit-code` 等)は別タスクとして残す。

## 受け入れ条件

| AC | 内容 | 担当テスト / 確認 |
|---|---|---|
| **AC-B1** | `deploy/k8s/overlays/cloud` から到達するオブジェクトに `kind: Ingress` が1つも無く、Service の `spec.type` は `ClusterIP`(既定)だけ(`LoadBalancer` / `NodePort` が無い)。Service の `externalIPs`・Deployment の `hostNetwork: true`・コンテナの `hostPort` も無い | `deploytest.TestCloudOverlayHasNoPublicEntrypoint`(緑) |
| **AC-B2** | `kubectl kustomize deploy/k8s/overlays/cloud` の描画結果に `kind: Ingress`・`type: LoadBalancer`・`type: NodePort`・`externalIPs`・`hostNetwork: true`・`hostPort` が無い(kubectl が無ければ skip) | `deploytest.TestCloudOverlayRenderHasNoPublicEntrypoint`(緑) |
| **AC-B3** | `deploy/k8s/base/gateway/ingress.yaml` の先頭コメントに「local / k3d 専用」「cloud overlay では使わない」「ADR-0210」がある | `deploytest.TestBaseGatewayIngressIsDocumentedAsLocalOnly`(緑) |
| **AC-B4** | `deploy/k8s/overlays/local` / `local-api` の到達経路は変わらない(gateway の Ingress は local では従来どおり存在する) | 既存 `cmd/gateway.TestManifestGatewayIngress` が緑のまま |
| **AC-B5** | gateway は `Authorization` / `Cookie` / `X-Api-Key` を見ない(付けても付けなくても同じ結果。これらを理由に 401 / 403 を返さない)。端末 ID を変えても同じ本文が同じ上流へ同じ内容で届く。不正・欠落の端末 ID は 400 で 401 / 403 ではない | `httpapi.TestDeviceIDIsNotAuthentication`(緑) |
| **AC-B6** | 許可外オリジン・`Origin` 無しでも 200 で上流に届く(CORS ヘッダは付かない) | `httpapi.TestCORSIsNotAccessControl`(緑) |
| **AC-B7** | 契約に `securitySchemes` / `security` が無く、401 / 403 の応答が無く、`ErrorCode` に認証系の code が無く、`Authorization` のパラメータが無い | `api.TestContractHasNoAuthentication`(緑) |
| **AC-B8** | `api/openapi.yaml` を変えない(API 契約の変更は無い。`make gen` の差分も無い) | `git diff --stat` |
| **AC-B9** | `make test` / `make lint`(`k8s-render` を含む)が成功する | 手動 |

## 自己矛盾チェックリスト(この ADR を変更したら通し直す)

- [x] ユーザー決定「public LoadBalancer / Ingress を作らない」と §2.1 が一致する(host / TLS を足す案は §2.2 で却下)。
- [x] ユーザー決定「端末 ID は引き続き認証ではない」と §4.1 が一致し、ADR-0209 §2 を変えていない。
- [x] §1.3 の「失効はネットワーク層」は ADR-0209 §2 の「端末 ID に失効・再発行を持ち込まない」と矛盾しない
      (失効の対象が端末 ID ではなく tailnet の device である)。
- [x] §3(gateway で TLS を終端しない)が成立する前提は §2(public な入口が無い)。§2 が崩れれば §3 も見直す、と §3.1 に書いてある。
- [x] §2.3(base に Ingress を残す)と AC-B1(cloud に Ingress が無い)は両立する(cloud overlay で消す)。
- [x] §5 の1(構造)は削除以外の patch を見ないという限界があり、それを §5 の本文検索と2(描画)で塞いでいる。
- [x] requirements.md §3「Mac 起動中は Tailscale 経由で外出先からも利用可」・§5「`tailscale serve` で HTTPS」と §3.1 が一致する。
- [x] ADR-0012(`/api/balance` は balance の Ingress に最長一致で届く)は local の話で、cloud に Ingress を出さない決定と衝突しない
      (cloud に balance を入れるときは §7 の依頼どおり同じ制約を適用する)。
- [x] ADR-0104 §8(cloud で CronJob を suspend)と §2 の cloud overlay の変更は同じファイルを触るが、目的が独立している。
- [x] §2.1 の到達経路候補1(Tailscale Operator の `tailscale` ingressClass)は `kind: Ingress` を作るため、
      現状の AC-B1・AC-B2(Ingress が1つも無い)と将来衝突する。候補1に進むときの対応(条件の改定。テストを先に
      緩めない)を §2.1 に明記した。
- [x] 絶対ルール1(API 変更は openapi.yaml から)に触れない(この ADR は API 契約を変えない。AC-B8)。
- [x] 絶対ルール6(テストを消さない・弱めない)を守る: 既存テストは1つも変えず、AC-B4 で local の経路が変わらないことを固定する。
- [x] §5・AC-B1・AC-B2 の検査範囲(`externalIPs`・`hostNetwork`・`hostPort`)は §2.1 の決定文と一致する
      (critic レビューで見つかった穴。`hostNetwork: true` を注入する変異テストで実際に赤くなることを確認した上でこの版に反映)。

## 却下した案(§2.2・§3.2 以外)

- **選択肢2(インターネット公開 + OIDC)**: ユーザー決定に反する。利用者1人に対して認証基盤・トークン保管・
  失効手順・監査ログを持つコストが釣り合わない。学習目的としては魅力があるが、その場合は「学習のための別環境」として
  本番の個人データから切り離すべきで、この ADR の境界とは別問題。
- **選択肢3(計算 API と Web 資産だけ公開し、保存 API は private)**: 境界が二重になり、
  「どの endpoint がどちらか」を gateway・OpenAPI・クライアントの3か所で一致させ続ける必要がある。
  計算 API は認証が無いままレート制限も無いので(§1.2 の守るもの1)、公開する側にも守りが必要になり、
  「public にする分だけ守りが増える」だけになる。
- **`base` から Ingress を取り除く**: §2.3 のとおり local の経路が2か所に重複し、ADR-0012 の最長一致の前提が散る。
- **cloud overlay を今の時点で丸ごと削除する**: `make k8s-render`(`make lint` から呼ばれる)と
  ADR-0104 §8 の suspend patch の置き場所が無くなる。cloud へ移す判断はまだしていないが、雛形は残しておきたい。
- **NetworkPolicy で外部からの ingress を落とす**: クラスタ内の多層防御としては有効だが、
  「public IP を作らない」の代替にはならない(LoadBalancer ができてしまえば、それは Pod への正当な ingress に見える)。
  必要なら別タスクで足す。

## 影響

- `deploy/k8s/overlays/cloud/kustomization.yaml`(+ 削除 patch か resources の分解)と
  `deploy/k8s/base/gateway/ingress.yaml`(コメント)は implementer が変える。`base` の Ingress 自体は残る。
- `services/gateway/deploytest` に overlay の到達オブジェクトを辿る helper(`OverlayObjects`)が増える。
  テスト専用で、本体コードからは使わない(ADR-0203 §3 と同じ扱い)。
- `api/openapi.yaml` は変えない。生成物の差分も無い。
- ADR-0209 は §1 と末尾に追記の1〜2行が入る(§6)。
- クラウドへ移すときの到達経路(§2.1)は運用レーンと決める。それまで cloud overlay は「描画できるが入口を持たない」状態になる。

## 人間の確認が必要なこと

- **クラウドでの到達経路の選択**(§2.1 の1: Tailscale Operator の ingressClass / 2: subnet router + `tailscale serve`)。
  既定案は 2(現在の自宅 Mac の運用と同じ仕組みで、Operator の追加学習が不要)。provider を決めるときに合わせて確認したい。
- **この決定を将来覆すとき**(公開へ進むとき)は §6 の条件を満たす別 ADR が必要。自動では進めない。
