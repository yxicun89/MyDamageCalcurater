// CB-6 段階 C2(ADR-0359 追記): 素早さ画面の「この相手で連続技を計算」から計算画面の連続技へ渡す要求。
// お気に入りの復元要求(FavoriteRestoreRequest)と同じ流儀: token が変わったときだけ計算画面が受け取る。

import type { SpeedComboRequest } from "../speed/SpeedScreen";

/** 計算画面に渡す、連続技を開く要求(token は App が進める)。 */
export interface ComboOpenRequest extends SpeedComboRequest {
  readonly token: number;
}
