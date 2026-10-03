// public/apple-touch-icon.png(180x180)を生成する。外部素材なし。実行: node scripts/gen-icons.mjs
// 赤地に白いハート(public/icon.svg と同じ配色)。生成済みの PNG をコミットしてあるので、図案を変えるときだけ実行する。
import { writeFileSync } from "node:fs";
import { crc32, deflateSync } from "node:zlib";

const SIZE = 180;
const SS = 3; // 3x3 のスーパーサンプリング
const BG = [0xe1, 0x1d, 0x48];
const FG = [0xff, 0xff, 0xff];

// ハート曲線 (x²+y²-1)³ - x²y³ <= 0(x,y は -1.3..1.3 に正規化)
const inHeart = (px, py) => {
  const x = ((px / SIZE) * 2 - 1) * 1.35;
  const y = -((py / SIZE) * 2 - 1.08) * 1.35;
  return (x * x + y * y - 1) ** 3 - x * x * y ** 3 <= 0;
};

const raw = Buffer.alloc((SIZE * 3 + 1) * SIZE);
for (let y = 0; y < SIZE; y++) {
  raw[y * (SIZE * 3 + 1)] = 0; // filter: none
  for (let x = 0; x < SIZE; x++) {
    let hit = 0;
    for (let sy = 0; sy < SS; sy++)
      for (let sx = 0; sx < SS; sx++) if (inHeart(x + (sx + 0.5) / SS, y + (sy + 0.5) / SS)) hit++;
    const t = hit / (SS * SS);
    for (let c = 0; c < 3; c++) {
      raw[y * (SIZE * 3 + 1) + 1 + x * 3 + c] = Math.round(BG[c] * (1 - t) + FG[c] * t);
    }
  }
}

const chunk = (type, data) => {
  const body = Buffer.concat([Buffer.from(type, "ascii"), data]);
  const out = Buffer.alloc(body.length + 8);
  out.writeUInt32BE(data.length, 0);
  body.copy(out, 4);
  out.writeUInt32BE(crc32(body), body.length + 4);
  return out;
};
const ihdr = Buffer.alloc(13);
ihdr.writeUInt32BE(SIZE, 0);
ihdr.writeUInt32BE(SIZE, 4);
ihdr[8] = 8; // bit depth
ihdr[9] = 2; // RGB
const png = Buffer.concat([
  Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
  chunk("IHDR", ihdr),
  chunk("IDAT", deflateSync(raw, { level: 9 })),
  chunk("IEND", Buffer.alloc(0)),
]);
writeFileSync(new URL("../public/apple-touch-icon.png", import.meta.url), png);
