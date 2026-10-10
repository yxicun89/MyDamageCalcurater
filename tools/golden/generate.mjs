// Deterministic external oracle. Run npm ci && npm run generate in this directory.
//
// P2-1b(ADR-0002 §決定 5 の追記「ゴールデンの oracle を Champions へ切り替え」)。
// pin した @smogon/calc 0.12.0 の2つの世代を使う:
//   - genC = Generations.get(0)(Champions)。主のゴールデン。SP は evs にそのまま渡す。
//   - gen9 = Generations.get(9)。Champions の mechanics に無い効果(持ち物・特性)の
//     計算式を検証する legacy-effects.jsonl.gz だけに使う。SP は max(0,8*SP-4) に換算する。
import calc from '@smogon/calc';
import {readFileSync, writeFileSync, mkdirSync} from 'node:fs';
import {gzipSync} from 'node:zlib';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
const {Generations, Pokemon, Move, Field, calculate} = calc;
const {getFlingPower} = createRequire(import.meta.url)('@smogon/calc/dist/items');
const genC = Generations.get(0);
const gen9 = Generations.get(9);
const out = fileURLToPath(new URL('../../testdata/golden/', import.meta.url));
const version = JSON.parse(readFileSync(new URL('node_modules/@smogon/calc/package.json', import.meta.url))).version;
assert.equal(version, '0.12.0', 'Review oracle upgrades before regenerating');
const effects = JSON.parse(readFileSync(`${out}/effects.json`));
const keys = ['hp','atk','def','spa','spd','spe'];
const stats = (value = 0) => Object.fromEntries(keys.map(k => [k,value]));
const id = name => name.toLowerCase().replace(/[^a-z0-9]/g,'');
// テラスタイプ(ADR-0224)。oracle の18タイプ(??? とステラを除く)。ステラは Champions 世代に無い効果なので使わない。
const teraTypeNames = [...genC.types].filter(t => t.id !== '' && t.id !== 'stellar').map(t => t.name).sort();

// --- Champions 種族集合(内部フォーム除外) ----------------------------------
// Aegislash-Both は calc が攻守フォームを1つの計算で扱うための擬似フォームで、ゲーム内で選べる姿ではない。
const excludedSpeciesNames = ['Aegislash-Both'];
for (const n of excludedSpeciesNames) {
  assert(genC.species.get(id(n)), `除外種族 ${n} が Champions 世代に無い(改名された可能性。列挙を見直す)`);
}
const excludedSpeciesIds = new Set(excludedSpeciesNames.map(id));
const species = [...genC.species]
  .filter(s => s.baseStats.hp !== 1 && !excludedSpeciesIds.has(s.id))
  .sort((a,b) => a.id < b.id ? -1 : a.id > b.id ? 1 : 0);

// --- legacy 効果の導出(手で列挙しない) --------------------------------------
// effects.json の持ち物・特性のうち、Champions 世代(genC)に存在しないもの。
const legacyItemNames = Object.keys(effects.items).filter(n => !genC.items.get(id(n)));
const legacyAbilityNames = Object.keys(effects.abilities).filter(n => !genC.abilities.get(id(n)));
assert.deepEqual([...legacyItemNames].sort(), ['Assault Vest','Choice Band','Choice Specs','Eviolite'].sort(),
  'legacyEffects(items)の導出結果が想定と違う(ADR-0002 §4 / P2-1b)');
assert.deepEqual([...legacyAbilityNames].sort(), ['Steelworker'],
  'legacyEffects(abilities)の導出結果が想定と違う(ADR-0002 §4 / P2-1b)');
const legacyItems = new Set(legacyItemNames);
const legacyAbilities = new Set(legacyAbilityNames);
// 未対応の印(補正を計算できない持ち物・特性。issue #270 案 B / ADR-0123)。印だけの定義はベクタで使わない。
const unsupportedEffectKeys = ['UnsupportedAttacker','UnsupportedDefender'];
const isUnsupportedEffect = def => unsupportedEffectKeys.some(k => k in def);
// 技の機構(多段・一撃必殺)と組み合わさって初めて効く特性の効果(ADR-0142)。通常の技のダメージは変えないので、
// 上の「ダメージが変わるか」の調査(probe)には現れない。照合は mechanisms.json が担う。
const mechanismEffectKeys = ['MaxMultiHit','PreventsOHKO'];
const isMechanismEffect = def => mechanismEffectKeys.some(k => k in def);
// 技のフラグ(ADR-0178 §1)。oracle の技データ(flags・recoil・hasCrashDamage・secondaries)から engine の語彙(昇順)にする。
// 本番(Showdown 由来)と同じ規則で、importer の照合(move-value-mismatch/flags)が両者の一致を取り込みのたびに確かめる。
const sourceMoveFlags = ['bite','bullet','contact','pulse','punch','slicing','sound'];
const moveFlagsOf = m => [
  ...sourceMoveFlags.filter(f => m.flags && m.flags[f]),
  ...(m.recoil || m.hasCrashDamage ? ['recoil'] : []),
  ...(m.secondaries ? ['secondary'] : []),
].sort();

// Fixed-power, single-hit reference moves. These probe arithmetic, not learnset legality.
const moveNames = [
  ['Body Slam','Hyper Voice'], ['Fire Punch','Flamethrower'], ['Aqua Tail','Surf'],
  ['Thunder Punch','Thunderbolt'], ['Seed Bomb','Energy Ball'], ['Ice Punch','Ice Beam'],
  ['Drain Punch','Aura Sphere'], ['Poison Jab','Sludge Bomb'], ['Drill Run','Earth Power'],
  ['Drill Peck','Air Slash'], ['Zen Headbutt','Psychic'], ['X-Scissor','Bug Buzz'],
  ['Rock Slide','Power Gem'], ['Shadow Punch','Shadow Ball'], ['Dragon Claw','Dragon Pulse'],
  ['Crunch','Dark Pulse'], ['Iron Head','Flash Cannon'], ['Play Rough','Moonblast']
].flat();
const moves = moveNames.map(n => genC.moves.get(id(n)));
for (const m of moves) assert(m && m.basePower > 0 && !m.multihit && !m.willCrit && !m.overrideDefensiveStat);
const physical = moves.filter(m => m.category === 'Physical');
const special = moves.filter(m => m.category === 'Special');
const attackAnchors = [ ['Garchomp','Dragon Claw'], ['Charizard','Flamethrower'], ['Pikachu','Thunderbolt'], ['Scizor','Iron Head'], ['Gengar','Shadow Ball'] ];
// P2-1b: Blissey は Champions 世代に存在しない。役割(単ノーマルの高HP受け)が近い Snorlax に差し替える
// (critic レビュー: Wigglytuff はノーマル/フェアリーでドラゴン無効・格闘等倍など相性が変わるため不採用)。
const defenderAnchors = ['Snorlax','Tyranitar','Corviknight','Toxapex','Garchomp'];
const nature = (gen, name) => {const n = gen.natures.get(id(name));return n.plus === n.minus ? {} : {Plus:n.plus,Minus:n.minus};};

// individual は世代ごとに SP の渡し方を変える:
//   - Champions(genC, num===0): evs にそのまま渡す(spInput: direct)
//   - gen9: 努力値へ換算する max(0,8*SP-4)(spInput: max(0,8*SP-4))
// Champions のベクタで legacy 効果(genC に無い持ち物・特性)を使おうとしたら、ここで止める
// (ADR-0002 §決定2「Champions のベクタはこれらを使わない」)。
// 状態異常(ADR-0143。options.status は段階2のベクタだけが使う。options.burn は従来どおり)。oracle の記号 → engine の語彙。
const statusNames = {brn:'burn',par:'paralysis',psn:'poison',tox:'badly_poison',slp:'sleep',frz:'freeze'};
function individual(gen, name, options = {}) {
  const sp = {...stats(),...options.sp};
  assert(Object.values(sp).every(v => v >= 0 && v <= 32));
  assert(Object.values(sp).reduce((a,b) => a+b,0) <= 66);
  const ability = options.ability || '';
  const item = options.item || '';
  // options.stage2(技の機構の段階2。ADR-0143)のベクタは、素早さの補正(speedItems / speedAbilities)を持ち物・特性の効果に合わせ、
  // 種族の重さ(WeightHg)を入力に載せる。それ以外のベクタは従来と同じ入力(バイト列を変えない)。
  // stage2 は持ち主の側('a': 攻撃側 / 'd': 防御側)。未対応の印(ADR-0123)は、その側で効く印を持つ定義だけをベクタで使わない
  // (くろいてっきゅうは防御側の印だけ持つので、攻撃側なら使える)。
  const stage2 = options.stage2 || '';
  assert(['', 'a', 'd'].includes(stage2), `未知の側 ${stage2}`);
  const markedUnsupported = def => stage2 ? def[stage2 === 'a' ? 'UnsupportedAttacker' : 'UnsupportedDefender'] === true : isUnsupportedEffect(def);
  const effectOf = (damage, speed, key) => stage2 && (damage[key] || speed[key]) ? {...(damage[key] || {}), ...(speed[key] || {})} : damage[key];
  const abilityEffect = ability ? effectOf(effects.abilities, effects.speedAbilities, ability) : undefined;
  const itemEffect = item ? effectOf(effects.items, effects.speedItems, item) : undefined;
  if (ability) assert(abilityEffect && !markedUnsupported(abilityEffect));
  // options.fling(なげつける。ADR-0144)の持ち物だけは、効果定義を持たない持ち物(ダメージを変えない = 調査で未定義に出ない)も使える。
  if (item) assert((itemEffect || options.fling) && !(itemEffect && markedUnsupported(itemEffect)));
  assert(!(options.status && options.burn) && (!options.status || statusNames[options.status]), `未知の状態 ${options.status}`);
  const status = options.status || (options.burn ? 'brn' : '');
  if (gen.num === 0) {
    assert(!item || !legacyItems.has(item), `Champions ベクタが legacy 持ち物 ${item} を使おうとした`);
    assert(!ability || !legacyAbilities.has(ability), `Champions ベクタが legacy 特性 ${ability} を使おうとした`);
  }
  const evs = gen.num === 0 ? {...sp} : Object.fromEntries(keys.map(k => [k,Math.max(0,8*sp[k]-4)]));
  // options.tera はテラスのベクタ(tera*。ADR-0224)だけが使う。oracle は teraType を渡すだけで「テラスタル済み」と扱う。
  // Champions のテラスのベクタだけで使い、gen9(legacy)では使わない(gen9 は防御側の相性もテラスで見るため)。
  const tera = options.tera || '';
  if (tera) {
    assert.equal(gen.num, 0, 'テラスのベクタは Champions 世代だけ(ADR-0224)');
    assert(teraTypeNames.includes(tera), `未知のテラスタイプ ${tera}`);
  }
  // Empty ability alone is insufficient: Pokemon.clone() otherwise restores the species default.
  const p = new Pokemon(gen, name, {level:50, ivs:stats(31),
    evs, nature:options.nature || 'Serious', boosts:options.ranks || {}, ability, item,
    status, overrides:{abilities:{0:''}}, ...(tera ? {teraType:tera} : {}), ...(options.curHP ? {curHP:options.curHP} : {})});
  assert.equal(p.ability || '', ability);
  assert.equal(p.clone().ability || '', ability);
  assert.equal(p.clone().teraType || '', tera);
  const input={Species:{Key:p.species.id,Types:p.types.map(t => t.toLowerCase()),BaseStats:p.species.baseStats},
    Level:50, Nature:nature(gen, p.nature), SP:sp, Ranks:options.ranks || {}, Status:status ? statusNames[status] : 'none',
    Ability:{ID:ability,Effect:abilityEffect || null},
    Item:item ? {ID:item,Effect:itemEffect} : null};
  if (stage2) input.Species.WeightHg=Math.round(p.species.weightkg*10);
  // 技の機構の段階3(ADR-0144)。options.curHP は oracle の Pokemon の curHP(1..最大。0 は oracle が満タンと読むので渡さない)。
  // options.fling は持ち物のなげつける威力を oracle の getFlingPower から入力に載せる。どちらも指定したベクタだけ(他のバイト列を変えない)。
  if (options.curHP) assert(Number.isInteger(options.curHP) && options.curHP >= 1 && options.curHP <= p.rawStats.hp && p.originalCurHP === options.curHP, `curHP ${options.curHP} が 1..${p.rawStats.hp} の外`);
  if (options.fling) {
    assert(item && input.Item, 'fling には持ち物が要る');
    input.Item.FlingPower = getFlingPower(item);
    assert(input.Item.FlingPower > 0, `${item}: oracle の getFlingPower が 0`);
  }
  // テラス無しのベクタは従来と同じバイト列にするため、キー自体を出さない。
  if (tera) input.TeraType=tera.toLowerCase();
  return {p, input};
}
// Independent ADR-0006 probability: convolve shortfalls from max damage; discard outcomes
// whose shortfall exceeds n*max-HP. Inputs are exclusively the external damage rolls.
function ko(rolls,hp) {
  const max = rolls[15];
  if (!max) return {Hits:0,Guaranteed:false,ChancePercent:0};
  const n = Math.ceil(hp/max);
  if (rolls[0]*n >= hp) return {Hits:n,Guaranteed:true,ChancePercent:0};
  const budget = n*max-hp;
  let p = new Float64Array(budget+1); p[0]=1;
  for(let hit=0;hit<n;hit++) {
    const next = new Float64Array(budget+1);
    for(let short=0;short<=budget;short++) if(p[short]) {
      for(const roll of rolls) if(short+max-roll<=budget) next[short+max-roll]+=p[short]/16;
    }
    p=next;
  }
  return {Hits:n,Guaranteed:false,ChancePercent:p.reduce((a,b)=>a+b,0)*100};
}
const weatherNames = {none:undefined,sun:'Sun',rain:'Rain',sand:'Sand',snow:'Snow'};
const terrainNames = {none:undefined,electric:'Electric',grassy:'Grassy',misty:'Misty',psychic:'Psychic'};
// 形式(ADR-0222)。engine の Format → oracle の Field.gameType。
const gameTypeNames = {single:'Singles', double:'Doubles'};
// 技の対象の分類(ADR-0222)。oracle(champions.js)の isSpread = gameType !== 'Singles' && target が次のどれか。
// それ以外(oracle のデータで target 省略 = 'any' を含む)は単体技。
const spreadTargets = ['allAdjacent','allAdjacentFoes'];
const moveTargetOf = m => spreadTargets.includes(m.target) ? 'spread' : 'single';
// options.format / options.moveTarget はダブルのベクタ(ADR-0222)だけが使う。未指定のベクタは従来と同じバイト列を出す。
function vector(gen, label, a, d, moveName, options={}) {
  const attack=individual(gen, a, options.a), defend=individual(gen, d, options.d);
  const weather=options.weather || 'none', terrain=options.terrain || 'none';
  const screen=options.screen;
  const format=options.format || 'single';
  assert(gameTypeNames[format], `未知の形式 ${format}`);
  const m = new Move(gen, moveName, {isCrit:!!options.critical});
  assert(m.bp>0 && m.hits===1);
  const f = new Field({gameType:gameTypeNames[format],weather:weatherNames[weather],terrain:terrainNames[terrain],
    defenderSide:{isReflect:screen==='Reflect',isLightScreen:screen==='LightScreen',isAuroraVeil:screen==='AuroraVeil'}});
  const result=calculate(gen,attack.p,defend.p,m,f);
  const rolls=typeof result.damage==='number' ? Array(16).fill(result.damage) : result.damage;
  assert.equal(rolls.length,16); assert(rolls.every(Number.isInteger));
  const expectedKO=ko(rolls,defend.p.rawStats.hp);
  const move={ID:id(moveName),Type:m.type.toLowerCase(),Category:m.category.toLowerCase(),Power:m.bp,Priority:m.priority};
  if (options.moveTarget) move.Target=moveTargetOf(m);
  // ADR-0178: 技のフラグ。oracle の技データから engine の語彙で書く(既存のベクタのバイト列を変えないため、指定したときだけ)。
  if (options.moveFlags) { move.Flags=moveFlagsOf(m); move.FlagsKnown=true; }
  return {id:label, oracle:{attacker:a,defender:d,move:moveName}, input:{Format:format,Attacker:attack.input,Defender:defend.input,
    Move:move,
    Field:{Weather:weather,Terrain:terrain,DefenderScreens:{Reflect:screen==='Reflect',LightScreen:screen==='LightScreen',AuroraVeil:screen==='AuroraVeil'}},Critical:!!options.critical},
    expected:{rolls,attackerStats:attack.p.rawStats,defenderStats:defend.p.rawStats,ko:expectedKO}};
}

// --- Champions の固定シナリオ -----------------------------------------------
// choiceband / choicespecs / assaultvest / steelworker / sand-vest は legacy 効果を使うため
// legacyScenarios(gen9)へ分けた(ADR-0002 §決定2)。
const championsScenarios = [
  ['baseline',{}],['critical',{critical:true}],['burn',{a:{burn:true}}],
  ['sun',{weather:'sun'}],['rain',{weather:'rain'}],['sand',{weather:'sand'}],['snow',{weather:'snow'}],
  ['electric',{terrain:'electric'}],['grassy',{terrain:'grassy'}],['psychic',{terrain:'psychic'}],['misty',{terrain:'misty'}],
  ['reflect',{screen:'Reflect'}],['light-screen',{screen:'LightScreen'}],['aurora-veil',{screen:'AuroraVeil'}],
  ...['Life Orb','Expert Belt','Charcoal','Muscle Band','Wise Glasses'].map(item=>[id(item),{a:{item}}]),
  ...['Occa Berry','Chilan Berry'].map(item=>[id(item),{d:{item}}]),
  ['adaptability',{a:{ability:'Adaptability'}}],
  ['water-bubble',{a:{ability:'Water Bubble',burn:true}}],['thick-fat',{d:{ability:'Thick Fat'}}],
  ['filter',{d:{ability:'Filter'}}],['filter-life-orb',{a:{item:'Life Orb'},d:{ability:'Filter'}}],
  ['sun-charcoal-thick-fat',{weather:'sun',a:{item:'Charcoal'},d:{ability:'Thick Fat'}}],
  ['critical-ranks-screens',{critical:true,a:{ranks:{atk:-6,spa:-6}},d:{ranks:{def:6,spd:6}},screen:'AuroraVeil'}]
];
// ADR-0002 P2-1b の差し替え表。役割(タイプ・立ち位置)は変えず、Champions に居ない種族だけ差し替える。
const fixedPairs=[['Typhlosion','Abomasnow','Flamethrower'],['Snorlax','Wigglytuff','Body Slam'],['Raichu','Blastoise','Thunderbolt'],['Venusaur','Tyranitar','Energy Ball'],['Metagross','Clefable','Iron Head'],['Goodra','Garchomp','Dragon Pulse']];
const championsFixed=[];
for (const [label,options] of championsScenarios) for(const [a,d,m] of fixedPairs) championsFixed.push(vector(genC,`${label}/${a}/${m}`,a,d,m,options));
// water-bubble-defense: 攻撃側を Magmortar → Typhlosion(接地した炎単の特殊アタッカー)に差し替え。
championsFixed.push(vector(genC,'water-bubble-defense','Typhlosion','Blastoise','Flamethrower',{d:{ability:'Water Bubble'}}));
championsFixed.push(vector(genC,'water-bubble-attack','Blastoise','Snorlax','Surf',{a:{ability:'Water Bubble'}}));
championsFixed.push(vector(genC,'ko-four-hits','Snorlax','Snorlax','Body Slam',{d:{sp:{def:32}}}));
// misty-dragon: 攻撃側を Haxorus → Goodra(接地したドラゴン単タイプ)に差し替え。
championsFixed.push(vector(genC,'misty-dragon','Goodra','Snorlax','Dragon Claw',{terrain:'misty'}));

// --- 特性によるタイプの無効・吸収(ADR-0106 §決定8) ---------------------------------
// Champions の random の特性プールには足さない(プールを変えると乱数列が動き、
// random.jsonl.gz 10,000件すべての期待値が変わってしまうため)。専用ベクタだけを
// championsFixed に足す。特性ごとに「無効/吸収」「対照(別タイプ)」「組合せ
// (急所・壁・天候・ランク)」の3件。Dry Skin・Storm Drain は入れない(ADR-0106 §限界1・2)。
const immunityCases = [
  {slug:'levitate', ability:'Levitate', attacker:'Garchomp', defender:'Snorlax',
    blockedMove:'Earth Power', controlMove:'Flamethrower', comboOptions:{critical:true, terrain:'misty', d:{ability:'Levitate'}}},
  {slug:'earth-eater', ability:'Earth Eater', attacker:'Garchomp', defender:'Snorlax',
    blockedMove:'Drill Run', controlMove:'Flamethrower', comboOptions:{screen:'AuroraVeil', d:{ability:'Earth Eater'}}},
  {slug:'water-absorb', ability:'Water Absorb', attacker:'Charizard', defender:'Blastoise',
    blockedMove:'Surf', controlMove:'Body Slam', comboOptions:{weather:'sun', d:{ability:'Water Absorb'}}},
  {slug:'volt-absorb', ability:'Volt Absorb', attacker:'Charizard', defender:'Blastoise',
    blockedMove:'Thunderbolt', controlMove:'Body Slam',
    comboOptions:{a:{ranks:{atk:6,spa:6}}, d:{ability:'Volt Absorb'}}},
  {slug:'motor-drive', ability:'Motor Drive', attacker:'Charizard', defender:'Blastoise',
    blockedMove:'Thunderbolt', controlMove:'Body Slam', comboOptions:{critical:true, d:{ability:'Motor Drive'}}},
  {slug:'lightning-rod', ability:'Lightning Rod', attacker:'Charizard', defender:'Blastoise',
    blockedMove:'Thunderbolt', controlMove:'Body Slam', comboOptions:{screen:'Reflect', d:{ability:'Lightning Rod'}}},
  {slug:'flash-fire', ability:'Flash Fire', attacker:'Metagross', defender:'Goodra',
    blockedMove:'Flamethrower', controlMove:'Body Slam', comboOptions:{weather:'rain', d:{ability:'Flash Fire'}}},
  {slug:'sap-sipper', ability:'Sap Sipper', attacker:'Metagross', defender:'Garchomp',
    blockedMove:'Energy Ball', controlMove:'Body Slam',
    comboOptions:{a:{ranks:{atk:6,spa:6}}, d:{ability:'Sap Sipper'}}},
  // issue #270 / ADR-0120: Eelevate は oracle で Levitate と同じ扱い(地面無効・isGrounded で浮く)。
  {slug:'eelevate', ability:'Eelevate', attacker:'Garchomp', defender:'Snorlax',
    blockedMove:'Drill Run', controlMove:'Flamethrower', comboOptions:{critical:true, terrain:'misty', d:{ability:'Eelevate'}}},
];
for (const c of immunityCases) {
  championsFixed.push(vector(genC,`${c.slug}/immune`,c.attacker,c.defender,c.blockedMove,{d:{ability:c.ability}}));
  championsFixed.push(vector(genC,`${c.slug}/control`,c.attacker,c.defender,c.controlMove,{d:{ability:c.ability}}));
  championsFixed.push(vector(genC,`${c.slug}/combined`,c.attacker,c.defender,c.blockedMove,c.comboOptions));
}

// --- フィールドの接地判定(issue #231 / ADR-0116) ----------------------------------
// 威力を上げる補正(エレキ・グラス・サイコ)は攻撃側、ミストのドラゴン半減は防御側が接地しているときだけ
// (oracle: util.isGrounded = ひこうタイプでない かつ Levitate でない かつ Air Balloon でない)。
// 浮いている側(ひこうタイプ・ふゆう)と、反対側だけが浮いている対照を両方置く。
const groundingCases = [
  ['electric-flying-attacker','Charizard','Snorlax','Thunderbolt',{terrain:'electric'}],
  ['grassy-flying-attacker','Corviknight','Snorlax','Energy Ball',{terrain:'grassy'}],
  ['psychic-flying-attacker','Talonflame','Snorlax','Psychic',{terrain:'psychic'}],
  ['electric-levitate-attacker','Pikachu','Snorlax','Thunderbolt',{terrain:'electric',a:{ability:'Levitate'}}],
  ['psychic-levitate-attacker','Gengar','Snorlax','Psychic',{terrain:'psychic',a:{ability:'Levitate'}}],
  ['misty-flying-defender','Goodra','Dragonite','Dragon Claw',{terrain:'misty'}],
  ['misty-levitate-defender','Goodra','Snorlax','Dragon Pulse',{terrain:'misty',d:{ability:'Levitate'}}],
  // 対照: 反対側だけが浮いている(補正は掛かる)。
  ['electric-flying-defender','Pikachu','Corviknight','Thunderbolt',{terrain:'electric'}],
  ['grassy-levitate-defender','Garchomp','Snorlax','Energy Ball',{terrain:'grassy',d:{ability:'Levitate'}}],
  ['misty-flying-attacker','Dragonite','Snorlax','Dragon Claw',{terrain:'misty'}],
  ['misty-levitate-attacker','Garchomp','Snorlax','Dragon Claw',{terrain:'misty',a:{ability:'Levitate'}}],
  // issue #270 / ADR-0120: Eelevate も oracle の isGrounded で浮く。
  ['electric-eelevate-attacker','Pikachu','Snorlax','Thunderbolt',{terrain:'electric',a:{ability:'Eelevate'}}],
];
for (const [slug,a,d,m,options] of groundingCases) {
  const v=vector(genC,`terrain-grounding/${slug}`,a,d,m,options);
  // 意図した側が浮いている/接地していることを oracle 側でも確かめる(種族の差し替えで黙って崩れないように)。
  const airborne = side => {const p=individual(genC, side==='a'?a:d, options[side]).p; return p.hasType('Flying')||p.hasAbility('Levitate','Eelevate');};
  assert.equal(airborne('a')||airborne('d'), true, `terrain-grounding/${slug} に浮いている側が無い`);
  championsFixed.push(v);
}

// --- サイコフィールドの先制技(ADR-0121 §5 / ADR-0123) ----------------------------------
// 優先度が正の攻撃技は、サイコフィールドで接地した防御側に当たらない(oracle: move.priority > 0 &&
// field.hasTerrain('Psychic') && isGrounded(defender))。浮いている防御側・他のフィールドは対照。
// 当たらないケースは oracle のダメージが 0、対照は 0 でないことをここでも確かめる。
const psychicPriorityCases = [
  ['grounded-defender','Garchomp','Snorlax','Quick Attack',{terrain:'psychic'},true],
  ['grounded-defender-special','Gengar','Snorlax','Vacuum Wave',{terrain:'psychic'},true],
  ['grounded-defender-priority2','Garchomp','Snorlax','Extreme Speed',{terrain:'psychic',critical:true},true],
  ['flying-defender','Garchomp','Corviknight','Quick Attack',{terrain:'psychic'},false],
  ['levitate-defender','Garchomp','Snorlax','Quick Attack',{terrain:'psychic',d:{ability:'Levitate'}},false],
  ['electric-terrain','Garchomp','Snorlax','Quick Attack',{terrain:'electric'},false],
];
for (const [slug,a,d,m,options,blocked] of psychicPriorityCases) {
  assert(genC.moves.get(id(m)).priority > 0, `psychic-priority/${slug}: ${m} の優先度が正でない`);
  const v=vector(genC,`psychic-priority/${slug}`,a,d,m,options);
  assert.equal(v.expected.rolls.every(r => r === 0), blocked, `psychic-priority/${slug}: oracle の当たる/当たらないが想定と違う`);
  championsFixed.push(v);
}

// --- 効果定義の1種ずつの照合(issue #270 / ADR-0120) --------------------------------
// タイプ・相性で効く効果(タイプ強化・半減きのみ・特定タイプの攻撃実数値補正・抜群軽減)は、
// effects.json の定義から「効く」ケースと「効かない対照」を1組ずつ作る。どちらも、同じ条件で
// 効果を外したときの oracle のダメージと比べ、効く方は変わり・対照は変わらないことを確かめる
// (定義の型を取り違えて、効かないケースだけを照合してしまうのを防ぐ)。
// Champions の random のプールには足さない(乱数列が動き、既存の期待値がすべて変わるため)。
const typeNameById = Object.fromEntries([...genC.types].map(t => [t.id, t.name]));
const effectiveness = (moveType, s) => s.types.reduce((e, t) => e * genC.types.get(id(moveType)).effectiveness[t], 1);
const physicalMoveOfType = t => {
  const m = physical.find(m => m.type === typeNameById[t]);
  assert(m, `代表技に ${t} タイプの物理技が無い(moveNames を確認)`);
  return m.name;
};
const defenderFor = (t, pred, label) => {
  const s = species.find(s => pred(effectiveness(typeNameById[t], s)));
  assert(s, `${label}: ${t} 技の相手が種族集合に無い`);
  return s.name;
};
const neutral = e => e === 1;
const superEffective = e => e > 1;
// 対照に使う別タイプ(ノーマル自身が対象の効果だけ、かくとうにする)。
const otherType = t => t === 'normal' ? 'fighting' : 'normal';
const effectAttacker = 'Snorlax';
function withoutEffect(options) {
  const strip = side => side && Object.fromEntries(Object.entries(side).filter(([k]) => k !== 'item' && k !== 'ability'));
  return {...options, a:strip(options.a), d:strip(options.d)};
}
function effectCase(label, t, pred, options, expectChange) {
  const move = physicalMoveOfType(t);
  const d = defenderFor(t, pred, label);
  const v = vector(genC, label, effectAttacker, d, move, options);
  const base = vector(genC, `${label}/base`, effectAttacker, d, move, withoutEffect(options));
  assert(v.expected.rolls.some(r => r > 0), `${label}: ダメージが0(相性で無効な組を選んだ)`);
  const changed = JSON.stringify(v.expected.rolls) !== JSON.stringify(base.expected.rolls);
  assert.equal(changed, expectChange, `${label}: 効果の有無で oracle のダメージが${expectChange ? '変わらない' : '変わる'}`);
  championsFixed.push(v);
}
// --- 特性の段階1(ADR-0176) -----------------------------------------------------------------
// effectPair は options と baseOptions(効果を外した、または比べる相手の条件)の oracle のダメージを比べ、
// 変わる/変わらないが想定どおりであることを確かめてから、options のベクタだけを fixed.json に足す。
const sameExpectedRolls = (x, y) => JSON.stringify(x.expected.rolls) === JSON.stringify(y.expected.rolls);
function effectPair(label, a, d, moveName, options, baseOptions, expectChange) {
  const v = vector(genC, label, a, d, moveName, options);
  const base = vector(genC, `${label}/base`, a, d, moveName, baseOptions);
  assert(v.expected.rolls.some(r => r > 0), `${label}: ダメージが0(相性で無効な組を選んだ)`);
  assert.equal(!sameExpectedRolls(v, base), expectChange,
    `${label}: 比べた条件と oracle のダメージが${expectChange ? '変わらない' : '変わる'}`);
  championsFixed.push(v);
}
const stripKey = (side, key) => side && Object.fromEntries(Object.entries(side).filter(([k]) => k !== key));
// 防御側の特性を無視する特性(IgnoresDefenderAbility)。効果データから引く(名前を書かない)。
const defenderAbilityIgnorers = Object.keys(effects.abilities).filter(n => effects.abilities[n].IgnoresDefenderAbility);
assert.equal(defenderAbilityIgnorers.length, 1, 'IgnoresDefenderAbility を持つ特性がちょうど1つでない(ベクタの作り方を見直す)');
const ignorer = defenderAbilityIgnorers[0];
// breakableCase は、防御側の Breakable な特性 name が効くケース(options)で、攻撃側に ignorer を持たせると
// 「防御側の特性なし」と同じダメージになることを確かめる。
function breakableCase(label, a, d, moveName, options) {
  const withIgnorer = {...options, a:{...(options.a || {}), ability:ignorer}};
  effectPair(label, a, d, moveName, withIgnorer, {...withIgnorer, d:stripKey(options.d, 'ability')}, false);
}
const normalPhysical = physical.find(m => m.type === 'Normal');
const normalSpecial = special.find(m => m.type === 'Normal');
assert(normalPhysical && normalSpecial, '代表技にノーマルの物理・特殊が無い');
// 威力の条件の境界を調べる技(威力固定・単発・優先度 0・反動なし)。
const fixedPowerMoves = probeMoveCandidates();
function probeMoveCandidates() {
  const names = ['Water Pulse','Thunder Fang','Fire Fang','Leaf Blade','Psycho Cut'];
  return [...moves, ...names.map(n => genC.moves.get(id(n)))].filter(m => m && m.basePower > 0 && !m.multihit &&
    !m.priority && !m.recoil && !m.willCrit);
}
const stage1AbilityCases = (name, def) => {
  const slug = `effects/${id(name)}`;
  const nd = defenderFor('normal', neutral, slug);
  if (def.TypeConvert) {
    const {From, To} = def.TypeConvert;
    assert.equal(From, 'normal', `${slug}: ノーマル以外からの変換はベクタの作り方を見直す`);
    const d = defenderFor(To, neutral, slug);
    effectPair(`${slug}/convert/physical/apply`, effectAttacker, d, normalPhysical.name, {a:{ability:name}}, {}, true);
    effectPair(`${slug}/convert/special/apply`, effectAttacker, d, normalSpecial.name, {a:{ability:name}}, {}, true);
    // 変換後のタイプで一致する攻撃側(元のタイプの一致を失わない)。
    const stabAttacker = species.find(s => s.types.includes(typeNameById[To]) && !s.types.includes('Normal'));
    assert(stabAttacker, `${slug}: ${To} タイプの攻撃側が種族集合に無い`);
    effectPair(`${slug}/convert/stab/apply`, stabAttacker.name, d, normalPhysical.name, {a:{ability:name}}, {}, true);
    // ノーマルのままなら当たらない相手(ゴースト)にも当たる。
    const ghost = species.find(s => effectiveness('Normal', s) === 0 && effectiveness(typeNameById[To], s) > 0);
    if (ghost) effectPair(`${slug}/convert/ghost/apply`, effectAttacker, ghost.name, normalPhysical.name, {a:{ability:name}}, {}, true);
    const c = otherType(From);
    effectPair(`${slug}/convert/${c}/control`, effectAttacker, defenderFor(c, neutral, slug), physicalMoveOfType(c), {a:{ability:name}}, {}, false);
  }
  for (const [field, label] of [['StatMods','stat'], ['SeparateStatMods','separate']]) {
    if (!def[field]) continue;
    for (const k of Object.keys(def[field]).sort()) {
      const offensive = k === 'atk' || k === 'spa';
      const usesPhysical = k === 'atk' || k === 'def';
      const side = offensive ? 'a' : 'd';
      const m = usesPhysical ? normalPhysical : normalSpecial;
      const cm = usesPhysical ? normalSpecial : normalPhysical;
      effectPair(`${slug}/${label}/${k}/apply`, effectAttacker, nd, m.name, {[side]:{ability:name}}, {}, true);
      effectPair(`${slug}/${label}/${k}/control`, effectAttacker, nd, cm.name, {[side]:{ability:name}}, {}, false);
      if (!offensive && def.Breakable) breakableCase(`${slug}/${label}/${k}/breakable`, effectAttacker, nd, m.name, {d:{ability:name}});
    }
  }
  for (const pm of def.PowerMods || []) {
    if (pm.Condition === 'max_base_power') {
      const under = fixedPowerMoves.filter(m => m.basePower <= pm.MaxPower).sort((x, y) => y.basePower - x.basePower)[0];
      const over = fixedPowerMoves.filter(m => m.basePower > pm.MaxPower).sort((x, y) => x.basePower - y.basePower)[0];
      assert(under && over, `${slug}: 威力 ${pm.MaxPower} の境界の技が無い`);
      effectPair(`${slug}/power/max${pm.MaxPower}/apply`, effectAttacker, defenderFor(id(under.type), neutral, slug), under.name, {a:{ability:name}}, {}, true);
      effectPair(`${slug}/power/max${pm.MaxPower}/control`, effectAttacker, defenderFor(id(over.type), neutral, slug), over.name, {a:{ability:name}}, {}, false);
    } else if (pm.Condition === 'move_type') {
      const t = pm.MoveType, c = otherType(t);
      effectPair(`${slug}/power/${t}/apply`, effectAttacker, defenderFor(t, neutral, slug), physicalMoveOfType(t), {a:{ability:name}}, {}, true);
      effectPair(`${slug}/power/${c}/control`, effectAttacker, defenderFor(c, neutral, slug), physicalMoveOfType(c), {a:{ability:name}}, {}, false);
    } else if (pm.Condition !== 'move_flag') { // move_flag は段階2(stage2AbilityCases。ADR-0178)
      assert.fail(`${slug}: 未知の威力の条件 ${pm.Condition}`);
    }
  }
  if (def.AuraType) {
    const t = def.AuraType, c = otherType(t), d = defenderFor(t, neutral, slug), m = physicalMoveOfType(t);
    effectPair(`${slug}/aura/${t}/attacker/apply`, effectAttacker, d, m, {a:{ability:name}}, {}, true);
    effectPair(`${slug}/aura/${t}/defender/apply`, effectAttacker, d, m, {d:{ability:name}}, {}, true);
    // 両側が持っても1回だけ(片側だけと同じ)。
    effectPair(`${slug}/aura/${t}/both/apply`, effectAttacker, d, m, {a:{ability:name}, d:{ability:name}}, {a:{ability:name}}, false);
    effectPair(`${slug}/aura/${c}/control`, effectAttacker, defenderFor(c, neutral, slug), physicalMoveOfType(c), {a:{ability:name}, d:{ability:name}}, {}, false);
  }
  if (def.CritDamageMod) {
    effectPair(`${slug}/crit/apply`, effectAttacker, nd, normalPhysical.name, {critical:true, a:{ability:name}}, {critical:true}, true);
    effectPair(`${slug}/crit/control`, effectAttacker, nd, normalPhysical.name, {a:{ability:name}}, {}, false);
  }
  if (def.PreventsCritical) {
    effectPair(`${slug}/nocrit/apply`, effectAttacker, nd, normalPhysical.name, {critical:true, d:{ability:name}}, {critical:true}, true);
    effectPair(`${slug}/nocrit/control`, effectAttacker, nd, normalPhysical.name, {d:{ability:name}}, {}, false);
    if (def.Breakable) breakableCase(`${slug}/nocrit/breakable`, effectAttacker, nd, normalPhysical.name, {critical:true, d:{ability:name}});
  }
  if (def.IgnoresOpponentRanks) {
    effectPair(`${slug}/ranks/attacker/apply`, effectAttacker, nd, normalPhysical.name, {a:{ability:name}, d:{ranks:{def:2}}}, {d:{ranks:{def:2}}}, true);
    effectPair(`${slug}/ranks/defender/apply`, effectAttacker, nd, normalPhysical.name, {a:{ranks:{atk:2}}, d:{ability:name}}, {a:{ranks:{atk:2}}}, true);
    effectPair(`${slug}/ranks/control`, effectAttacker, nd, normalPhysical.name, {a:{ability:name}}, {}, false);
    if (def.Breakable) breakableCase(`${slug}/ranks/breakable`, effectAttacker, nd, normalPhysical.name, {a:{ranks:{atk:2}}, d:{ability:name}});
  }
  if (def.IgnoresDefenderAbility) {
    // 防御側の Breakable な特性(タイプで効く半減)を無視する。Breakable でない特性(オーラ)は無視しない。
    const breakable = Object.keys(effects.abilities).sort().find(n => effects.abilities[n].Breakable && effects.abilities[n].DefResistType);
    assert(breakable, `${slug}: 比べる Breakable な半減の特性が無い`);
    const t = Object.keys(effects.abilities[breakable].DefResistType).sort()[0];
    const d = defenderFor(t, neutral, slug), m = physicalMoveOfType(t);
    effectPair(`${slug}/ignore/${id(breakable)}/apply`, effectAttacker, d, m, {a:{ability:name}, d:{ability:breakable}}, {d:{ability:breakable}}, true);
    const unbreakable = Object.keys(effects.abilities).sort().find(n => effects.abilities[n].AuraType && !effects.abilities[n].Breakable);
    if (unbreakable) {
      const at = effects.abilities[unbreakable].AuraType;
      effectPair(`${slug}/ignore/${id(unbreakable)}/control`, effectAttacker, defenderFor(at, neutral, slug), physicalMoveOfType(at),
        {a:{ability:name}, d:{ability:unbreakable}}, {d:{ability:unbreakable}}, false);
    }
    effectPair(`${slug}/ignore/control`, effectAttacker, nd, normalPhysical.name, {a:{ability:name}}, {}, false);
  }
};

// --- 特性の段階2(技のフラグ。ADR-0178) ------------------------------------------------------
// ベクタはすべて moveFlags を付ける(oracle の技のフラグを engine に渡す。FlagsKnown)。
// 威力固定・単発・優先度 0 の技から、フラグを持つ技と持たない技を選ぶ(名前ではなくフラグで選ぶ)。
const stage2ExtraMoveNames = ['Boomburst','Fire Fang','Water Pulse','Leaf Blade','Double-Edge','Flare Blitz','High Jump Kick'];
const stage2Moves = [...moves, ...stage2ExtraMoveNames.map(n => {
  const m = genC.moves.get(id(n));
  assert(m && m.basePower > 0, `段階2の技 ${n} が Champions 世代に無い(改名された可能性。列挙を見直す)`);
  return m;
})].filter(m => !m.multihit && !m.priority && !m.willCrit);
const stage2MoveWith = (f, label, pred = () => true) => {
  const m = stage2Moves.find(m => moveFlagsOf(m).includes(f) && pred(m));
  assert(m, `${label}: フラグ ${f} を持つ技が無い(stage2ExtraMoveNames を見直す)`);
  return m;
};
const stage2MoveWithout = (f, label, pred = () => true) => {
  const m = stage2Moves.find(m => !moveFlagsOf(m).includes(f) && pred(m));
  assert(m, `${label}: フラグ ${f} を持たない技が無い`);
  return m;
};
const withFlags = options => ({...options, moveFlags:true});
// 特性による無効(ダメージ 0)は effectPair の「ダメージが0」の検査に掛かるので、専用に比べる。
function immuneCase(label, a, d, moveName, options) {
  const v = vector(genC, label, a, d, moveName, withFlags(options));
  const base = vector(genC, `${label}/base`, a, d, moveName, withFlags(withoutEffect(options)));
  assert(v.expected.rolls.every(r => r === 0), `${label}: 無効のはずなのにダメージがある`);
  assert(base.expected.rolls.some(r => r > 0), `${label}: 特性なしでもダメージが0(相性で無効な組を選んだ)`);
  championsFixed.push(v);
}
const stage2AbilityCases = (name, def) => {
  const slug = `effects/${id(name)}`;
  const powerCases = (mods, stage) => {
    for (const pm of mods || []) {
      if (pm.Condition !== 'move_flag') continue;
      const f = pm.Flag;
      const m = stage2MoveWith(f, slug), c = stage2MoveWithout(f, slug);
      effectPair(`${slug}/${stage}/${f}/apply`, effectAttacker, defenderFor(id(m.type), neutral, slug), m.name,
        withFlags({a:{ability:name}}), withFlags({}), true);
      effectPair(`${slug}/${stage}/${f}/control`, effectAttacker, defenderFor(id(c.type), neutral, slug), c.name,
        withFlags({a:{ability:name}}), withFlags({}), false);
    }
  };
  powerCases(def.PowerMods, 'power');
  powerCases(def.PostAuraPowerMods, 'postaura');
  if (def.FlagTypeConvert) {
    const {Flag, To} = def.FlagTypeConvert;
    const m = stage2MoveWith(Flag, slug, m => m.type === 'Normal');
    const toName = typeNameById[To];
    // 変換で相性が変わる相手(ノーマルは等倍・変換後は抜群)。
    const d = species.find(s => effectiveness('Normal', s) === 1 && effectiveness(toName, s) > 1);
    assert(d, `${slug}: ${To} が抜群でノーマルが等倍の相手が無い`);
    effectPair(`${slug}/flagconvert/${Flag}/apply`, effectAttacker, d.name, m.name, withFlags({a:{ability:name}}), withFlags({}), true);
    const c = stage2MoveWithout(Flag, slug, m => m.type === 'Normal');
    effectPair(`${slug}/flagconvert/${Flag}/control`, effectAttacker, d.name, c.name, withFlags({a:{ability:name}}), withFlags({}), false);
  }
  for (const f of def.DefImmuneFlags || []) {
    const m = stage2MoveWith(f, slug), c = stage2MoveWithout(f, slug);
    const d = defenderFor(id(m.type), neutral, slug);
    immuneCase(`${slug}/immune/${f}/apply`, effectAttacker, d, m.name, {d:{ability:name}});
    effectPair(`${slug}/immune/${f}/control`, effectAttacker, defenderFor(id(c.type), neutral, slug), c.name,
      withFlags({d:{ability:name}}), withFlags({}), false);
    if (def.Breakable) breakableCase(`${slug}/immune/${f}/breakable`, effectAttacker, d, m.name, withFlags({d:{ability:name}}));
  }
  for (const f of Object.keys(def.DefFinalModsByFlag || {}).sort()) {
    // 炎の ×2 等のタイプの補正と混ざらないよう、タイプの補正の対象でない技を選ぶ。
    const typed = new Set(Object.keys(def.DefFinalModsByType || {}).map(t => typeNameById[t]));
    const m = stage2MoveWith(f, slug, m => !typed.has(m.type)), c = stage2MoveWithout(f, slug, m => !typed.has(m.type));
    const d = defenderFor(id(m.type), neutral, slug);
    effectPair(`${slug}/final/${f}/apply`, effectAttacker, d, m.name, withFlags({d:{ability:name}}), withFlags({}), true);
    effectPair(`${slug}/final/${f}/control`, effectAttacker, defenderFor(id(c.type), neutral, slug), c.name,
      withFlags({d:{ability:name}}), withFlags({}), false);
    if (def.Breakable) breakableCase(`${slug}/final/${f}/breakable`, effectAttacker, d, m.name, withFlags({d:{ability:name}}));
  }
  for (const t of Object.keys(def.DefFinalModsByType || {}).sort()) {
    // フラグの補正と混ざらないよう、その特性のフラグを持たない技を選ぶ。
    const flags = Object.keys(def.DefFinalModsByFlag || {});
    const m = stage2Moves.find(m => id(m.type) === t && !moveFlagsOf(m).some(f => flags.includes(f)));
    assert(m, `${slug}: ${t} タイプでフラグの補正の対象でない技が無い`);
    const ct = otherType(t);
    const c = stage2Moves.find(m => id(m.type) === ct && !moveFlagsOf(m).some(f => flags.includes(f)));
    assert(c, `${slug}: ${ct} タイプでフラグの補正の対象でない技が無い`);
    effectPair(`${slug}/finaltype/${t}/apply`, effectAttacker, defenderFor(t, neutral, slug), m.name, withFlags({d:{ability:name}}), withFlags({}), true);
    effectPair(`${slug}/finaltype/${t}/control`, effectAttacker, defenderFor(ct, neutral, slug), c.name, withFlags({d:{ability:name}}), withFlags({}), false);
    if (def.Breakable) breakableCase(`${slug}/finaltype/${t}/breakable`, effectAttacker, defenderFor(t, neutral, slug), m.name, withFlags({d:{ability:name}}));
  }
  if (def.NoContact) {
    // 接触の最終補正を持つ防御側の特性(効果データから引く)に対して、接触しない扱いで補正が外れる。
    const halver = Object.keys(effects.abilities).sort().find(n => (effects.abilities[n].DefFinalModsByFlag || {}).contact);
    assert(halver, `${slug}: 接触の最終補正を持つ特性が effects.json に無い`);
    const typed = new Set(Object.keys(effects.abilities[halver].DefFinalModsByType || {}).map(t => typeNameById[t]));
    const m = stage2MoveWith('contact', slug, m => !typed.has(m.type));
    const d = defenderFor(id(m.type), neutral, slug);
    effectPair(`${slug}/nocontact/${id(halver)}/apply`, effectAttacker, d, m.name,
      withFlags({a:{ability:name}, d:{ability:halver}}), withFlags({d:{ability:halver}}), true);
    effectPair(`${slug}/nocontact/control`, effectAttacker, d, m.name, withFlags({a:{ability:name}}), withFlags({}), false);
  }
};

const typedEffectCases = (kind, name, def) => {
  const slug = `effects/${id(name)}`;
  if (kind === 'abilities') stage1AbilityCases(name, def);
  if (kind === 'abilities') stage2AbilityCases(name, def);
  if (kind === 'items' && def.BoostType) {
    const t = def.BoostType;
    effectCase(`${slug}/boost/${t}/apply`, t, neutral, {a:{item:name}}, true);
    effectCase(`${slug}/boost/${otherType(t)}/control`, otherType(t), neutral, {a:{item:name}}, false);
  }
  if (kind === 'items' && def.ResistBerryType) {
    const t = def.ResistBerryType;
    effectCase(`${slug}/berry/${t}/apply`, t, t === 'normal' ? neutral : superEffective, {d:{item:name}}, true);
    // 半減きのみは抜群のときだけ効く(ノーマルは例外で常に効くので、別タイプを対照にする)。
    if (t === 'normal') effectCase(`${slug}/berry/${otherType(t)}/control`, otherType(t), neutral, {d:{item:name}}, false);
    else effectCase(`${slug}/berry/${t}/control`, t, neutral, {d:{item:name}}, false);
  }
  if (kind === 'abilities' && def.OffBoostType) {
    const t = def.OffBoostType;
    effectCase(`${slug}/offboost/${t}/apply`, t, neutral, {a:{ability:name}}, true);
    effectCase(`${slug}/offboost/${otherType(t)}/control`, otherType(t), neutral, {a:{ability:name}}, false);
  }
  if (kind === 'abilities' && def.DefResistType) {
    const types = Object.keys(def.DefResistType).sort();
    for (const t of types) effectCase(`${slug}/defresist/${t}/apply`, t, neutral, {d:{ability:name}}, true);
    if (def.Breakable) {
      const t = types[0];
      breakableCase(`${slug}/defresist/${t}/breakable`, effectAttacker, defenderFor(t, neutral, slug), physicalMoveOfType(t), {d:{ability:name}});
    }
    const c = types.includes('normal') ? 'fighting' : 'normal';
    assert(!types.includes(c), `${slug}: 対照のタイプ ${c} も軽減の対象になっている`);
    effectCase(`${slug}/defresist/${c}/control`, c, neutral, {d:{ability:name}}, false);
  }
  if (kind === 'abilities' && def.ReduceSuperEffective) {
    effectCase(`${slug}/reduce/fighting/apply`, 'fighting', superEffective, {d:{ability:name}}, true);
    effectCase(`${slug}/reduce/fighting/control`, 'fighting', neutral, {d:{ability:name}}, false);
    if (def.Breakable) breakableCase(`${slug}/reduce/fighting/breakable`, effectAttacker,
      defenderFor('fighting', superEffective, slug), physicalMoveOfType('fighting'), {d:{ability:name}});
  }
};
// 特性による無効・吸収(ADR-0106)のうち Breakable なものは、防御側の特性を無視する攻撃側には効かない。
for (const c of immunityCases) {
  if (!effects.abilities[c.ability].Breakable) continue;
  breakableCase(`${c.slug}/breakable`, c.attacker, c.defender, c.blockedMove, {d:{ability:c.ability}});
}
for (const kind of ['items','abilities']) {
  for (const name of Object.keys(effects[kind]).sort()) {
    if (kind === 'items' ? legacyItems.has(name) : legacyAbilities.has(name)) continue;
    if (isUnsupportedEffect(effects[kind][name])) continue;
    typedEffectCases(kind, name, effects[kind][name]);
  }
}

// --- ダメージに効く持ち物・特性と効果定義の差(issue #270 / ADR-0120) --------------------
// Champions 世代の全持ち物・全特性を1つずつ攻撃側/防御側に持たせ、持たせないときとダメージが
// 変わるものを数える(手で列挙しない)。そのうち effects.json に定義が無いものは、
// unsupported-effects.json に理由付きで登録したものだけを許す(差分は両方向とも失敗にする)。
// 条件付きの効果も拾えるよう、天候・急所・状態異常・HP・フィールドを変えた4条件で調べる。
// 技は代表技(先制度 0・単発・反動なし)に加え、効果の条件になる技の性質を持つ技も使う(ADR-0123 追記):
// 先制技(優先度で防ぐ特性・優先度を上げる特性)・反動技・連続技・低威力(威力 60 以下で効く特性)・
// 接触/非接触・パンチ・かみつき・音・波動・弾・切る技・追加効果のある技。oracle の champions.js が
// 条件に使う move.flags / priority / recoil / hits / bp / secondaries を網羅するように選ぶ(ベクタには使わない)。
const probeExtraMoveNames = [
  'Quick Attack','Extreme Speed','Aqua Jet','Bullet Punch','Mach Punch','Ice Shard','Shadow Sneak','Sucker Punch',
  'Vacuum Wave','Double-Edge','Flare Blitz','Brave Bird','Wild Charge','Head Smash','Bullet Seed','Rock Blast',
  'Icicle Spear','Boomburst','Leaf Blade','Psycho Cut','Water Pulse','Thunder Fang','Fire Fang',
];
// 重さで威力が決まる技(威力は 0 で登録されている。ADR-0143)。重さの補正の特性(ヘヴィメタル・ライトメタル)を拾う。
const probeWeightMoveNames = ['Low Kick','Grass Knot','Heavy Slam','Heat Crash'];
const probeMoves = [...moves, ...probeExtraMoveNames.map(n => {
  const m = genC.moves.get(id(n));
  assert(m && m.basePower > 0, `調査用の技 ${n} が Champions 世代に無い(改名された可能性。列挙を見直す)`);
  return m;
}), ...probeWeightMoveNames.map(n => {
  const m = genC.moves.get(id(n));
  assert(m && m.basePower === 0, `調査用の重さの技 ${n} が Champions 世代に無い、または威力が 0 でない(列挙を見直す)`);
  return m;
})];
{
  const has = pred => probeMoves.some(pred);
  for (const [label, pred] of [
    ['先制技', m => m.priority > 0], ['反動技', m => !!m.recoil], ['連続技', m => !!m.multihit],
    ['威力 60 以下', m => m.basePower <= 60], ['非接触の物理技', m => m.category === 'Physical' && !m.flags.contact],
    ...['contact','punch','bite','sound','pulse','bullet','slicing'].map(f => [`flags.${f}`, m => !!m.flags[f]]),
    ['追加効果', m => !!(m.secondaries || m.secondary)],
  ]) assert(has(pred), `調査用の技に${label}が無い(条件付きの効果を取りこぼす)`);
}
const probeAttackers = ['Pikachu','Garchomp'];
const probeDefenders = ['Snorlax','Tyranitar','Corviknight','Toxapex','Garchomp','Charizard','Gengar','Clefable','Venusaur'];
for (const m of probeMoves) {
  assert(probeDefenders.some(d => effectiveness(m.type, genC.species.get(id(d))) > 1) || m.type === 'Normal',
    `調査用の防御側に ${m.type} 技が抜群になる種族が無い(半減きのみを取りこぼす)`);
}
const probeConditions = [
  {field:{}, a:{}, d:{}},
  {field:{weather:'Sand'}, crit:true, a:{status:'brn', hpFraction:3}, d:{}},
  {field:{weather:'Sun', terrain:'Grassy'}, a:{status:'par'}, d:{status:'par'}},
  // サイコフィールド: 優先度を上げる特性(先制技にした技が接地した相手に防がれる)・シード系の持ち物を拾う。
  {field:{weather:'Rain', terrain:'Psychic'}, a:{}, d:{}},
  // ADR-0176: 相手の状態に掛かる特性を拾う。ランク(相手のランクを無視する特性)・毒の相手(急所になる特性)。
  {field:{}, a:{boosts:{atk:2, spa:2}}, d:{boosts:{def:2, spd:2}}},
  {field:{}, a:{boosts:{atk:-2, spa:-2}}, d:{status:'psn', boosts:{def:-2, spd:-2}}},
  // ADR-0176: 防御側の特性と組み合わさって効く攻撃側の特性(防御側の特性を無視する・接触の半減を受けない等)を拾う。
  // 防御側に特性を持たせる条件なので、持ち主が防御側の調査(holder = 'd')では使わない(attackerOnly)。
  ...['Thick Fat','Fur Coat','Fluffy','Multiscale','Levitate'].map(ability => ({field:{}, attackerOnly:true, a:{}, d:{ability}})),
  {field:{}, crit:true, attackerOnly:true, a:{}, d:{ability:'Shell Armor'}},
];
function probePokemon(name, side, ability, item) {
  const p = new Pokemon(genC, name, {level:50, ivs:stats(31), evs:stats(), nature:'Serious', ability, item,
    status:side.status || '', boosts:side.boosts || {}, overrides:{abilities:{0:''}}});
  if (side.hpFraction) p.originalCurHP = Math.floor(p.maxHP() / side.hpFraction);
  return p;
}
// probeSignature は holder 側に ability / item を持たせた全条件のダメージを並べる。other は持ち主でない側の特性
// (Breakable の導出で攻撃側に防御側の特性を無視する特性を持たせる)。持ち主でない側は、条件が特性を指定していればそれを使う。
function probeSignature(holder, ability, item, other = '') {
  const out = [];
  for (const cond of probeConditions) {
    if (cond.attackerOnly && holder === 'd') continue;
    for (const a of probeAttackers) for (const d of probeDefenders) for (const m of probeMoves) {
      const A = probePokemon(a, cond.a, holder === 'a' ? ability : (other || cond.a.ability || ''), holder === 'a' ? item : '');
      const D = probePokemon(d, cond.d, holder === 'd' ? ability : (other || cond.d.ability || ''), holder === 'd' ? item : '');
      const r = calculate(genC, A, D, new Move(genC, m.name, {isCrit:!!cond.crit}), new Field({gameType:'Singles', ...cond.field}));
      out.push(JSON.stringify(r.damage));
    }
  }
  return out.join('|');
}
// 持ち主の側ごとの基準(attackerOnly の条件があるので、攻撃側の調査と防御側の調査で条件の集合が違う)。
const probeBaselines = {a: probeSignature('a', '', ''), d: probeSignature('d', '', '')};
const damageChanging = {items:[], abilities:[]};
// changingSides は、持たせるとダメージが変わった側(a: 攻撃側 / d: 防御側。issue #270 案 B / ADR-0123)。
const changingSides = {items:{}, abilities:{}};
const probeSides = (kind, list, probe) => {
  for (const x of list) {
    const sides = {a: probe('a', x.name) !== probeBaselines.a, d: probe('d', x.name) !== probeBaselines.d};
    if (sides.a || sides.d) {
      damageChanging[kind].push(x.id);
      changingSides[kind][x.id] = sides;
    }
  }
};
probeSides('items', genC.items, (side, name) => probeSignature(side, '', name));
probeSides('abilities', genC.abilities, (side, name) => probeSignature(side, name, ''));
const unsupportedEffects = JSON.parse(readFileSync(new URL('unsupported-effects.json', import.meta.url)));
const effectCoverage = {};
for (const kind of ['items','abilities']) {
  const legacy = kind === 'items' ? legacyItems : legacyAbilities;
  const names = Object.keys(effects[kind]).filter(n => !legacy.has(n));
  // 「未対応」の印だけの定義(UnsupportedAttacker / UnsupportedDefender。ADR-0123)は補正の定義に数えない。
  const marked = names.filter(n => isUnsupportedEffect(effects[kind][n]));
  const defined = new Set(names.filter(n => !isUnsupportedEffect(effects[kind][n]) && !isMechanismEffect(effects[kind][n])).map(id));
  const changing = new Set(damageChanging[kind]);
  const undefinedChanging = [...changing].filter(x => !defined.has(x)).sort();
  assert.deepEqual(undefinedChanging, Object.keys(unsupportedEffects[kind]).sort(),
    `${kind}: ダメージに効くのに効果定義が無いものが unsupported-effects.json と一致しない(定義を足すか、理由付きで登録する)`);
  // 未対応の一覧(理由)と、効果定義の「未対応」の印は同じ集合で、印の側は oracle でダメージが変わった側と一致する。
  assert.deepEqual(marked.map(id).sort(), undefinedChanging,
    `${kind}: effects.json の未対応の印(UnsupportedAttacker / UnsupportedDefender)が unsupported-effects.json と一致しない`);
  for (const n of marked) {
    const def = effects[kind][n], sides = changingSides[kind][id(n)];
    // 持ち物による接地(Grounds。ADR-0144)は、フィールドがあるときだけ効く機構の効果で、上の probe(フィールドなし)には現れない。
    // 防御側の印を持つくろいてっきゅうが攻撃側では接地として効くので、印と同じ定義に置く。照合は mechanisms-stage3.json が担う。
    assert.deepEqual(Object.keys(def).sort().filter(k => !unsupportedEffectKeys.includes(k) && k !== 'Grounds'), [],
      `${kind} ${n}: 未対応の印と補正の定義を同じ項目に混ぜない`);
    assert.deepEqual({a:def.UnsupportedAttacker === true, d:def.UnsupportedDefender === true}, sides,
      `${kind} ${n}: 未対応の印の側が oracle でダメージが変わる側と一致しない(a=攻撃側 / d=防御側)`);
  }
  const deadDefinitions = [...defined].filter(x => !changing.has(x)).sort();
  assert.deepEqual(deadDefinitions, [], `${kind}: 効果定義があるのに oracle のダメージが変わらない(定義の誤りか調査条件の不足)`);
  effectCoverage[kind] = {damageChanging:changing.size, defined:defined.size, unsupported:undefinedChanging.length};
}

// oracle(champions.js)の defenderAbilityIgnored: 攻撃側の特性が Mold Breaker のとき、防御側の特性として無視される名前の一覧。
function oracleDefenderAbilityIgnored() {
  const src = readFileSync(new URL('node_modules/@smogon/calc/dist/mechanics/champions.js', import.meta.url), 'utf8');
  const m = /var defenderAbilityIgnored = defender\.hasAbility\(([^)]*)\);/.exec(src);
  assert(m, 'champions.js の defenderAbilityIgnored を読めない(oracle の版を確認する)');
  return [...m[1].matchAll(/'([^']+)'/g)].map(x => x[1]);
}

// --- Breakable の導出(ADR-0176) ------------------------------------------------------------
// 防御側でダメージを変える特性のうち、攻撃側に防御側の特性を無視する特性(effects.json の IgnoresDefenderAbility)を
// 持たせると「防御側の特性なし」と同じダメージになるものが Breakable。手で列挙せず oracle から導き、
// effects.json の Breakable と両方向で一致させる。段階1では未対応の印の定義には Breakable を付けない
// (印の定義は印だけを持つ。上の検査)。Breakable は防御側でダメージを変える特性にだけ付ける。
const breakableCoverage = {derived:[], defined:[]};
{
  const ignorerId = id(ignorer);
  const withIgnorerNone = probeSignature('d', '', '', ignorer);
  for (const ab of [...genC.abilities]) {
    const sides = changingSides.abilities[ab.id];
    if (!sides || !sides.d || ab.id === ignorerId) continue;
    if (probeSignature('d', ab.name, '', ignorer) === withIgnorerNone) breakableCoverage.derived.push(ab.id);
  }
  breakableCoverage.derived.sort();
  const names = Object.keys(effects.abilities).filter(n => !legacyAbilities.has(n));
  // 技の機構と組み合わさって効く特性(がんじょう等)は通常のダメージを変えないので上の導出に現れない。
  // oracle の defenderAbilityIgnored(防御側の特性を無視する攻撃側に効かない特性の一覧)に入っていることで確かめる。
  const oracleIgnored = oracleDefenderAbilityIgnored();
  for (const n of names.filter(n => isMechanismEffect(effects.abilities[n]) && effects.abilities[n].Breakable)) {
    assert(oracleIgnored.includes(n), `${n}: Breakable だが oracle の defenderAbilityIgnored に無い`);
  }
  breakableCoverage.defined = names.filter(n => effects.abilities[n].Breakable && !isMechanismEffect(effects.abilities[n])).map(id).sort();
  const derivedDefined = breakableCoverage.derived.filter(x => names.some(n => id(n) === x && !isUnsupportedEffect(effects.abilities[n])));
  assert.deepEqual(breakableCoverage.defined, derivedDefined,
    'abilities: effects.json の Breakable が oracle(防御側の特性を無視する攻撃側に効かない特性)と一致しない');
}

// --- legacy-effects の固定部分(gen9。元の種族のまま) --------------------------
const legacyScenarios = [
  ['choiceband',{a:{item:'Choice Band'}}],
  ['choicespecs',{a:{item:'Choice Specs'}}],
  ['assaultvest',{d:{item:'Assault Vest'}}],
  ['steelworker',{a:{ability:'Steelworker'}}],
  ['sand-vest',{weather:'sand',d:{item:'Assault Vest'}}],
];
// P2-1b 以前の fixedPairs(Champions に存在しない種族を含む、元の組合せ)。legacy はこれをそのまま使う。
const legacyFixedPairs=[['Magmortar','Abomasnow','Flamethrower'],['Snorlax','Blissey','Body Slam'],['Raichu','Blastoise','Thunderbolt'],['Tangrowth','Tyranitar','Energy Ball'],['Metagross','Clefable','Iron Head'],['Haxorus','Garchomp','Dragon Pulse']];
const legacyFixed=[];
for (const [label,options] of legacyScenarios) for(const [a,d,m] of legacyFixedPairs) legacyFixed.push(vector(gen9,`${label}/${a}/${m}`,a,d,m,options));
legacyFixed.push(vector(gen9,'eviolite-snow','Snorlax','Snover','Body Slam',{weather:'snow',d:{item:'Eviolite'}}));

// A fixed external KO cross-check for all matching 1..4-hit cases, without residual/berry model differences.
// そのケースが生成された世代(gen)で呼ぶ(Champions は genC、legacy は gen9)。
function koCrossCheck(gen, vectors) {
  let n=0;
  for(const v of vectors) {
    if(v.input.Field.Weather!=='none'||v.input.Field.Terrain!=='none'||v.input.Attacker.Status!=='none'||v.input.Attacker.Item||v.input.Defender.Item||v.input.Attacker.Ability.ID||v.input.Defender.Ability.ID||v.expected.ko.Hits<1||v.expected.ko.Hits>4) continue;
    const a=individual(gen,v.oracle.attacker),d=individual(gen,v.oracle.defender);
    // Call the oracle KO implementation with already externally computed rolls and the same HP.
    const result=calculate(gen,a.p,d.p,new Move(gen,v.oracle.move),new Field());
    result.damage=v.expected.rolls;
    const external=result.kochance();
    assert.equal(external.n,v.expected.ko.Hits);
    assert(Math.abs(external.chance-(v.expected.ko.Guaranteed?1:v.expected.ko.ChancePercent/100))<1e-12);
    v.expected.smogonKO=external;n++;
  }
  return n;
}
const championsKoCrossChecks = koCrossCheck(genC, championsFixed);
const legacyKoCrossChecks = koCrossCheck(gen9, legacyFixed);
assert.deepEqual([...new Set([...championsFixed,...legacyFixed].flatMap(v=>v.expected.smogonKO ? [v.expected.smogonKO.n] : []))].sort(),[1,2,3,4]);

// --- Champions のランダムケース(legacy 効果を含めない) ------------------------
const seed=0x504f4b45;
let state=seed;
function random(){state^=state<<13;state^=state>>>17;state^=state<<5;return (state>>>0)/4294967296;}
const pick=xs=>xs[Math.floor(random()*xs.length)];
function randomSP(rng){
  const sp=stats(), order=[...keys]; let budget=66;
  for(let i=order.length-1;i>0;i--){const j=Math.floor(rng()*(i+1));[order[i],order[j]]=[order[j],order[i]];}
  for(const k of order){sp[k]=Math.floor(rng()*(Math.min(32,budget)+1));budget-=sp[k];}
  return sp;
}
const randomCases=[];
for(let i=0;i<10000;i++) {
  const terrain=pick(['none','electric','grassy','misty','psychic']);
  // issue #231 / ADR-0116: 地形ありでもひこうタイプを除外しない(接地判定を検証する)。
  const a=pick(species),d=pick(species),m=pick(moves);
  // legacy 効果(Choice Band / Choice Specs / Assault Vest / Steelworker)はプールから除外(ADR-0002 §決定2)。
  const attackItem=pick(['','Life Orb','Expert Belt','Charcoal','Muscle Band','Wise Glasses']);
  const defendItem=pick(['','Occa Berry','Chilan Berry']);
  const rank=()=>Object.fromEntries(keys.slice(1).map(k=>[k,Math.floor(random()*13)-6]));
  randomCases.push(vector(genC,`random/${String(i).padStart(5,'0')}`,a.name,d.name,m.name,{terrain,weather:pick(Object.keys(weatherNames)),
    critical:random()<0.1,screen:pick(['','Reflect','LightScreen','AuroraVeil']),
    a:{sp:randomSP(random),nature:pick(['Serious','Adamant','Modest','Bold','Calm']),item:attackItem,ability:pick(['','Adaptability','Water Bubble']),burn:random()<0.2,ranks:rank()},
    d:{sp:randomSP(random),nature:pick(['Serious','Adamant','Modest','Bold','Calm']),item:defendItem,ability:pick(['','Thick Fat','Filter','Solid Rock','Water Bubble']),ranks:rank()}}));
}

// --- legacy-effects のランダム部分(gen9。旧 random が持っていた組合せの検証を残す) -----------------
// 種族プールは Champions 集合 ∩ gen9 で、種族値・タイプが同一のもの(P2-1b)。
function sameBaseStatsAndTypes(a, b) {
  return keys.every(k => a.baseStats[k] === b.baseStats[k]) &&
    a.types.length === b.types.length && a.types.every((t,i) => t === b.types[i]);
}
const legacySpeciesPool = species.filter(s => {
  const s9 = gen9.species.get(s.id);
  return s9 && sameBaseStatsAndTypes(s, s9);
});
assert(legacySpeciesPool.length > 0, 'legacy-effects 用の種族プールが空(Champions と gen9 の交差が無い)');
// Champions のメイン random(seed=0x504f4b45)とは別の乱数列(metadata.oracles[].seed に記録)。
const legacyRandomSeed = 0x4c454741; // "LEGA"
let legacyState = legacyRandomSeed;
function legacyRandom(){legacyState^=legacyState<<13;legacyState^=legacyState>>>17;legacyState^=legacyState<<5;return (legacyState>>>0)/4294967296;}
const legacyPick=xs=>xs[Math.floor(legacyRandom()*xs.length)];
// 各ケースが legacy 効果を1つ以上「実際に効く」形で使うようにする。Eviolite はランダムには入れない(固定 eviolite-snow のみ)。
//
// critic レビュー(P2-1b): 単に持ち物・特性を入れるだけでは engine の補正(modifiers.go)が無効なケースが
// 大量に混ざる(こだわりハチマキで特殊技を打つ、とつげきチョッキで物理技を受ける、はがねつかいで鋼以外の技、等。
// atkKey/defKey は技の分類で決まり、はがねつかいは技タイプで決まるため)。効果ごとに層を分け、
// その効果が実際に乗る技の集合(分類・タイプ)からだけ技を選ぶことで、全ケースで効果が確実に働くようにする。
const physicalMoves = moves.filter(m => m.category === 'Physical');
const specialMoves = moves.filter(m => m.category === 'Special');
const steelMoves = moves.filter(m => m.type === 'Steel');
assert(physicalMoves.length > 0 && specialMoves.length > 0 && steelMoves.length > 0,
  'legacy-random の層に技が無い(moveNames の代表技の分類/タイプ構成を確認)');
// 件数は「旧 random(Champions と legacy を分離する前)で各効果が実際に効いていた件数」
// (Choice Band 623 / Choice Specs 611 / Assault Vest 1181 / Steelworker 151。critic レビューで実測)を
// 下回らないよう、十分な余裕を持たせる(絶対ルール6: 弱体化しない)。
const legacyRandomLayers = [
  {label:'choiceband', count:1200, moves:physicalMoves, force:{a:{item:'Choice Band'}}},
  {label:'choicespecs', count:1200, moves:specialMoves, force:{a:{item:'Choice Specs'}}},
  {label:'assaultvest', count:1200, moves:specialMoves, force:{d:{item:'Assault Vest'}}},
  {label:'steelworker', count:220, moves:steelMoves, force:{a:{ability:'Steelworker'}}},
];
const legacyRandomCases=[];
let legacyCaseIndex=0;
for (const layer of legacyRandomLayers) {
  for(let i=0;i<layer.count;i++) {
    const terrain=legacyPick(['none','electric','grassy','misty','psychic']);
    const a=legacyPick(legacySpeciesPool),d=legacyPick(legacySpeciesPool),m=legacyPick(layer.moves);
    let attackItem=legacyPick(['','Life Orb','Expert Belt','Charcoal','Muscle Band','Wise Glasses']);
    let defendItem=legacyPick(['','Occa Berry','Chilan Berry']);
    let attackAbility=legacyPick(['','Adaptability','Water Bubble']);
    if (layer.force.a?.item) attackItem=layer.force.a.item;
    if (layer.force.d?.item) defendItem=layer.force.d.item;
    if (layer.force.a?.ability) attackAbility=layer.force.a.ability;
    const rank=()=>Object.fromEntries(keys.slice(1).map(k=>[k,Math.floor(legacyRandom()*13)-6]));
    legacyRandomCases.push(vector(gen9,`legacy-random/${String(legacyCaseIndex).padStart(5,'0')}`,a.name,d.name,m.name,{terrain,weather:legacyPick(Object.keys(weatherNames)),
      critical:legacyRandom()<0.1,screen:legacyPick(['','Reflect','LightScreen','AuroraVeil']),
      a:{sp:randomSP(legacyRandom),nature:legacyPick(['Serious','Adamant','Modest','Bold','Calm']),item:attackItem,ability:attackAbility,burn:legacyRandom()<0.2,ranks:rank()},
      d:{sp:randomSP(legacyRandom),nature:legacyPick(['Serious','Adamant','Modest','Bold','Calm']),item:defendItem,ability:legacyPick(['','Thick Fat','Filter','Solid Rock','Water Bubble']),ranks:rank()}}));
    legacyCaseIndex++;
  }
}
const legacyEffectsCases=[...legacyFixed,...legacyRandomCases];

// --- ダブル(壁・全体技。issue #232 案B のダブル分・#288。ADR-0222) ------------------------------
// Champions 世代(genC)の oracle が反映するもの(champions.js: 壁 2732/4096・全体技 3072/4096)だけを照合する。
// テラスタルは扱わない(ポケモンチャンピオンズに無い。ユーザー確認 2026-10-03)。
// 既存ファイルには足さない(乱数列とバイト列を変えないため)。各ケースは「対照と比べて oracle のダメージが
// 変わる/変わらない」をここでも確かめ、oracle の版やデータの変更で前提が黙って崩れないようにする。
const doubleFixed=[];
const sameRolls=(x,y)=>JSON.stringify(x.expected.rolls)===JSON.stringify(y.expected.rolls);
function doubleCase(label, a, d, moveName, options, control, expectChange) {
  const v=vector(genC,`doubles/${label}`,a,d,moveName,{moveTarget:true,...options});
  if (control) {
    const base=vector(genC,`doubles/${label}/control`,a,d,moveName,{moveTarget:true,...control});
    assert.equal(!sameRolls(v,base), expectChange, `doubles/${label}: 対照と比べた oracle の変化が想定(${expectChange ? '変わる' : '変わらない'})と違う`);
  }
  doubleFixed.push(v);
}
const sgl={}, dbl={format:'double'};
doubleCase('single-target/no-screen','Garchomp','Snorlax','Dragon Claw',dbl,sgl,false);
for (const [slug,a,d,m,screen] of [
  ['reflect','Garchomp','Snorlax','Dragon Claw','Reflect'],
  ['light-screen','Charizard','Snorlax','Flamethrower','LightScreen'],
  ['aurora-veil-physical','Garchomp','Snorlax','Dragon Claw','AuroraVeil'],
  ['aurora-veil-special','Charizard','Snorlax','Flamethrower','AuroraVeil'],
]) {
  doubleCase(`screen/${slug}`,a,d,m,{format:'double',screen},{screen},true);
  doubleCase(`screen/${slug}/critical`,a,d,m,{format:'double',screen,critical:true},{screen,critical:true},false); // 急所は壁を無視
}
doubleCase('screen/light-screen-vs-physical','Garchomp','Snorlax','Dragon Claw',{format:'double',screen:'LightScreen'},sgl,false);
for (const [slug,a,d,m] of [
  ['all-adjacent/earthquake','Garchomp','Snorlax','Earthquake'],
  ['all-adjacent/surf','Blastoise','Snorlax','Surf'],
  ['all-adjacent/discharge','Pikachu','Snorlax','Discharge'],
  ['all-adjacent-foes/rock-slide','Tyranitar','Charizard','Rock Slide'],
  ['all-adjacent-foes/hyper-voice','Snorlax','Snorlax','Hyper Voice'],
  ['all-adjacent-foes/heat-wave','Charizard','Snorlax','Heat Wave'],
  ['all-adjacent-foes/dazzling-gleam','Clefable','Garchomp','Dazzling Gleam'],
]) {
  assert(spreadTargets.includes(genC.moves.get(id(m)).target), `${m} が全体技でない`);
  doubleCase(`spread/${slug}`,a,d,m,dbl,sgl,true);
  doubleCase(`single-control/spread/${slug}`,a,d,m,{},null,false); // シングルの全体技は等倍(Target=spread でも 0.75 を掛けない)
}
doubleCase('spread/rain','Blastoise','Snorlax','Surf',{format:'double',weather:'rain'},{weather:'rain'},true);
doubleCase('spread/critical','Garchomp','Snorlax','Earthquake',{format:'double',critical:true},{critical:true},true);
// 全体技 × 壁: 0.75 × 2732/4096 はシングルの壁 0.5 とロールが一致することがある(この組がそう)。対照はダブルの壁なし。
doubleCase('spread/reflect','Garchomp','Snorlax','Earthquake',{format:'double',screen:'Reflect'},dbl,true);
doubleCase('spread/light-screen','Charizard','Snorlax','Heat Wave',{format:'double',screen:'LightScreen'},dbl,true);
doubleCase('spread/aurora-veil-critical','Garchomp','Snorlax','Earthquake',{format:'double',screen:'AuroraVeil',critical:true},{screen:'AuroraVeil',critical:true},true);
doubleCase('spread/burn-life-orb','Garchomp','Snorlax','Earthquake',{format:'double',a:{burn:true,item:'Life Orb'}},{a:{burn:true,item:'Life Orb'}},true);
doubleCase('spread/immune','Garchomp','Corviknight','Earthquake',dbl,sgl,false); // 無効は 0 のまま
doubleCase('spread/levitate','Garchomp','Snorlax','Earthquake',{format:'double',d:{ability:'Levitate'}},{d:{ability:'Levitate'}},false);
doubleCase('spread/bulky-defender','Pikachu','Snorlax','Surf',{format:'double',d:{sp:{hp:32,spd:32},nature:'Calm',ranks:{spd:6}}},{d:{sp:{hp:32,spd:32},nature:'Calm',ranks:{spd:6}}},true);

// ランダム(別の乱数列。メインの random と legacy-random の列は変えない)。
const doubleRandomSeed = 0x44424c45; // "DBLE"
let doubleState = doubleRandomSeed;
function doubleRandom(){doubleState^=doubleState<<13;doubleState^=doubleState>>>17;doubleState^=doubleState<<5;return (doubleState>>>0)/4294967296;}
const doublePick=xs=>xs[Math.floor(doubleRandom()*xs.length)];
// 全体技のプール: 威力固定・単発で、技名を名指しする処理が champions.js に無いもの。
// Earthquake / Bulldoze(グラスフィールドで半減)・Misty Explosion(ミストフィールドで強化)・Eruption / Water Spout(HP で威力変動)・
// Explosion / Self-Destruct は入れない(固定ベクタの Earthquake は地形なしで使う)。
const doubleSpreadMoveNames=['Rock Slide','Surf','Hyper Voice','Heat Wave','Dazzling Gleam','Discharge','Sludge Wave','Muddy Water','Blizzard','Icy Wind','Snarl','Lava Plume','Petal Blizzard','Boomburst','Breaking Swipe'];
const doubleSpreadMoves=doubleSpreadMoveNames.map(n=>{const m=genC.moves.get(id(n));assert(m&&m.basePower>0&&!m.multihit&&spreadTargets.includes(m.target),`全体技 ${n} が前提を満たさない`);return m;});
const doubleMoves=[...moves,...doubleSpreadMoves];
const doubleRandomCases=[];
for(let i=0;i<3000;i++) {
  const terrain=doublePick(['none','electric','grassy','misty','psychic']);
  const a=doublePick(species),d=doublePick(species),m=doublePick(doubleMoves);
  const rank=()=>Object.fromEntries(keys.slice(1).map(k=>[k,Math.floor(doubleRandom()*13)-6]));
  doubleRandomCases.push(vector(genC,`doubles-random/${String(i).padStart(5,'0')}`,a.name,d.name,m.name,{moveTarget:true,
    format:doublePick(['single','double','double']),terrain,weather:doublePick(Object.keys(weatherNames)),
    critical:doubleRandom()<0.1,screen:doublePick(['','Reflect','LightScreen','AuroraVeil']),
    a:{sp:randomSP(doubleRandom),nature:doublePick(['Serious','Adamant','Modest','Bold','Calm']),item:doublePick(['','Life Orb','Expert Belt','Charcoal','Muscle Band','Wise Glasses']),ability:doublePick(['','Adaptability','Water Bubble']),burn:doubleRandom()<0.2,ranks:rank()},
    d:{sp:randomSP(doubleRandom),nature:doublePick(['Serious','Adamant','Modest','Bold','Calm']),item:doublePick(['','Occa Berry','Chilan Berry']),ability:doublePick(['','Thick Fat','Filter','Solid Rock','Water Bubble']),ranks:rank()}}));
}
{
  // 層の網羅(弱い生成で黙って偏らないように)。
  const count=pred=>doubleRandomCases.filter(pred).length;
  for (const [label,pred,min] of [
    ['double × 全体技', v=>v.input.Format==='double'&&v.input.Move.Target==='spread',200],
    ['double × 壁', v=>v.input.Format==='double'&&Object.values(v.input.Field.DefenderScreens).some(Boolean),200],
    ['single × 全体技', v=>v.input.Format==='single'&&v.input.Move.Target==='spread',200],
    ['double × 全体技 × 急所', v=>v.input.Format==='double'&&v.input.Move.Target==='spread'&&v.input.Critical,20],
  ]) assert(count(pred)>=min, `doubles-random の層「${label}」が ${count(pred)} 件(下限 ${min})`);
}

// --- テラスタル(オプションの機能。ADR-0224) ------------------------------------------------------
// ポケモンチャンピオンズ本編にテラスタルは無いが、指定したときだけ反映する機能として照合する(ユーザー決定 2026-10-03)。
// oracle(Champions 世代)は Pokemon.teraType を渡すだけで「テラスタル済み」と扱い、次を反映する(ADR-0224 §1 の表):
//   - 攻撃側のタイプ一致(util.getStabMod): 元タイプ一致 +2048、テラス=技 +2048、てきおうりょくは hasType(技)のとき
//     テラスが元タイプなら +1024・そうでなければ +2048(hasType はテラス中ならテラスだけを見る)
//   - 「そのタイプを持つか」(Pokemon.hasType): 接地(ひこう)・サイコフィールドの先制技・すなあらし(いわ)・ゆき(こおり)
// 防御側のタイプ相性はテラスを見ない(champions.js は defender.types で相性を引く。gen9 の mechanics とは違う oracle の癖)。
// 既存ファイルには足さない(バイト列を変えないため)。各ケースは対照(テラス無し)と比べて oracle のダメージが
// 変わる/変わらないをここでも確かめ、oracle の版やデータの変更で前提が黙って崩れないようにする。
const teraFixed=[];
function teraCase(label, a, d, moveName, options, expectChange) {
  const v=vector(genC,`tera/${label}`,a,d,moveName,options);
  const strip=o=>o?Object.fromEntries(Object.entries(o).filter(([k])=>k!=='tera')):o;
  const base=vector(genC,`tera/${label}/control`,a,d,moveName,{...options,a:strip(options.a),d:strip(options.d)});
  assert(v.input.Attacker.TeraType || v.input.Defender.TeraType, `tera/${label}: テラスが無い`);
  assert.equal(!sameRolls(v,base), expectChange, `tera/${label}: 対照と比べた oracle の変化が想定(${expectChange ? '変わる' : '変わらない'})と違う`);
  teraFixed.push(v);
}
// T1〜T11(ADR-0224 §1 の表と同じ番号)。
teraCase('t1/stab-tera-is-original','Charizard','Snorlax','Flamethrower',{a:{tera:'Fire'}},true);           // ×1.5 → ×2.0
teraCase('t1/stab-tera-is-original-dual','Garchomp','Snorlax','Earth Power',{a:{tera:'Ground'}},true);
teraCase('t2/original-move-other-tera','Charizard','Snorlax','Flamethrower',{a:{tera:'Water'}},false);      // ×1.5 のまま
teraCase('t2/other-original-move-dual','Garchomp','Snorlax','Earth Power',{a:{tera:'Dragon'}},false);
teraCase('t3/tera-only','Charizard','Snorlax','Thunderbolt',{a:{tera:'Electric'}},true);                   // ×1.0 → ×1.5
teraCase('t3/tera-only-physical','Snorlax','Snorlax','Drain Punch',{a:{tera:'Fighting'}},true);
teraCase('t4/no-match','Charizard','Snorlax','Thunderbolt',{a:{tera:'Water'}},false);
teraCase('t5/adaptability/tera-is-original','Charizard','Snorlax','Flamethrower',{a:{tera:'Fire',ability:'Adaptability'}},true);     // ×2.0 → ×2.25
teraCase('t5/adaptability/tera-only','Charizard','Snorlax','Thunderbolt',{a:{tera:'Electric',ability:'Adaptability'}},true);       // ×1.0 → ×2.0
teraCase('t5/adaptability/original-move-other-tera','Charizard','Snorlax','Flamethrower',{a:{tera:'Water',ability:'Adaptability'}},true); // ×2.0 → ×1.5
teraCase('t5/adaptability/other-original-move-dual','Garchomp','Snorlax','Earth Power',{a:{tera:'Dragon',ability:'Adaptability'}},true); // ×2.0 → ×1.5
teraCase('t5/adaptability/no-match','Charizard','Snorlax','Thunderbolt',{a:{tera:'Water',ability:'Adaptability'}},false);
teraCase('t6/attacker-tera-flying-not-grounded','Pikachu','Snorlax','Thunderbolt',{terrain:'electric',a:{tera:'Flying'}},true);   // 補正が外れる
teraCase('t6/flying-attacker-tera-grounded','Charizard','Snorlax','Thunderbolt',{terrain:'electric',a:{tera:'Fire'}},true);       // 元ひこうでもテラス中は接地
teraCase('t6/flying-attacker-tera-flying','Charizard','Snorlax','Thunderbolt',{terrain:'electric',a:{tera:'Flying'}},false);
teraCase('t6/levitate-attacker-stays-airborne','Pikachu','Snorlax','Thunderbolt',{terrain:'electric',a:{tera:'Normal',ability:'Levitate'}},false);
teraCase('t6/tera-stab-and-grounded','Charizard','Snorlax','Thunderbolt',{terrain:'electric',a:{tera:'Electric'}},true);
teraCase('t7/defender-tera-flying-not-grounded','Goodra','Snorlax','Dragon Claw',{terrain:'misty',d:{tera:'Flying'}},true);
teraCase('t7/flying-defender-tera-grounded','Goodra','Corviknight','Dragon Claw',{terrain:'misty',d:{tera:'Steel'}},true);
teraCase('t8/psychic-priority-defender-tera-flying','Garchomp','Snorlax','Quick Attack',{terrain:'psychic',d:{tera:'Flying'}},true);   // 当たる
teraCase('t8/psychic-priority-flying-defender-tera-normal','Garchomp','Corviknight','Quick Attack',{terrain:'psychic',d:{tera:'Normal'}},true); // 当たらない
teraCase('t9/sand-defender-tera-rock','Charizard','Snorlax','Flamethrower',{weather:'sand',d:{tera:'Rock'}},true);
teraCase('t9/sand-rock-defender-tera-other','Charizard','Tyranitar','Flamethrower',{weather:'sand',d:{tera:'Fire'}},true);
teraCase('t9/sand-physical-unchanged','Snorlax','Snorlax','Body Slam',{weather:'sand',d:{tera:'Rock'}},false);
teraCase('t10/snow-defender-tera-ice','Snorlax','Snorlax','Body Slam',{weather:'snow',d:{tera:'Ice'}},true);
teraCase('t10/snow-ice-defender-tera-other','Snorlax','Abomasnow','Body Slam',{weather:'snow',d:{tera:'Grass'}},true);
teraCase('t10/snow-special-unchanged','Charizard','Snorlax','Thunderbolt',{weather:'snow',d:{tera:'Ice'}},false);
teraCase('t11/defender-tera-ghost-not-immune','Snorlax','Snorlax','Body Slam',{d:{tera:'Ghost'}},false);
teraCase('t11/defender-tera-fairy-dragon','Garchomp','Garchomp','Dragon Claw',{d:{tera:'Fairy'}},false);
teraCase('t11/defender-tera-flying-ground','Garchomp','Snorlax','Earth Power',{d:{tera:'Flying'}},false);
teraCase('t11/defender-tera-grass-fire','Charizard','Snorlax','Flamethrower',{d:{tera:'Grass'}},false);
teraCase('t11/defender-tera-expert-belt','Charizard','Snorlax','Flamethrower',{a:{item:'Expert Belt'},d:{tera:'Grass'}},false);
// 組合せ(急所・やけど・持ち物・天候・ランク)の中でもテラスの補正の位置(タイプ一致の段)が合うこと。
teraCase('combo/critical-life-orb','Charizard','Snorlax','Flamethrower',{critical:true,a:{tera:'Fire',item:'Life Orb'}},true);
teraCase('combo/sun-adaptability','Charizard','Snorlax','Flamethrower',{weather:'sun',a:{tera:'Fire',ability:'Adaptability'}},true);
teraCase('combo/burn-ranks-screen','Snorlax','Garchomp','Drain Punch',{screen:'Reflect',a:{tera:'Fighting',burn:true,ranks:{atk:2}}},true);
teraCase('combo/both-sides','Garchomp','Garchomp','Dragon Claw',{a:{tera:'Dragon'},d:{tera:'Fairy'}},true);
// 総当たり: 18タイプそれぞれを、攻撃側・防御側・両側に置く(どのタイプでも上の規則から外れないこと)。
for (const t of teraTypeNames) {
  const tid=id(t);
  teraFixed.push(vector(genC,`tera/all/atk/${tid}/charizard-flamethrower`,'Charizard','Snorlax','Flamethrower',{a:{tera:t}}));
  teraFixed.push(vector(genC,`tera/all/atk/${tid}/charizard-thunderbolt-electric`,'Charizard','Snorlax','Thunderbolt',{terrain:'electric',a:{tera:t}}));
  teraFixed.push(vector(genC,`tera/all/atk/${tid}/adaptability-garchomp-dragon-claw`,'Garchomp','Snorlax','Dragon Claw',{a:{tera:t,ability:'Adaptability'}}));
  teraFixed.push(vector(genC,`tera/all/def/${tid}/tyranitar-flamethrower-sand`,'Charizard','Tyranitar','Flamethrower',{weather:'sand',d:{tera:t}}));
  teraFixed.push(vector(genC,`tera/all/def/${tid}/abomasnow-body-slam-snow`,'Snorlax','Abomasnow','Body Slam',{weather:'snow',d:{tera:t}}));
  teraFixed.push(vector(genC,`tera/all/def/${tid}/corviknight-dragon-claw-misty`,'Goodra','Corviknight','Dragon Claw',{terrain:'misty',d:{tera:t}}));
  teraFixed.push(vector(genC,`tera/all/def/${tid}/corviknight-quick-attack-psychic`,'Garchomp','Corviknight','Quick Attack',{terrain:'psychic',d:{tera:t}}));
  teraFixed.push(vector(genC,`tera/all/both/${tid}/garchomp-earth-power`,'Garchomp','Garchomp','Earth Power',{a:{tera:t},d:{tera:t}}));
}

// ランダム(別の乱数列。他のファイルの列は変えない)。形式はシングルだけ(ダブルとの組合せは対象外。ユーザー決定 2026-10-03)。
const teraRandomSeed = 0x54455241; // "TERA"
let teraState = teraRandomSeed;
function teraRandom(){teraState^=teraState<<13;teraState^=teraState>>>17;teraState^=teraState<<5;return (teraState>>>0)/4294967296;}
const teraPick=xs=>xs[Math.floor(teraRandom()*xs.length)];
const teraRandomCases=[];
for(let i=0;i<3000;i++) {
  const terrain=teraPick(['none','electric','grassy','misty','psychic']);
  const a=teraPick(species),d=teraPick(species);
  // 4割は元タイプの技(タイプ一致とテラスの重なりを十分に出す)。代表技は18タイプすべてを持つ。
  const m=teraRandom()<0.4 ? teraPick(moves.filter(x=>a.types.includes(x.type))) : teraPick(moves);
  // 攻撃側のテラスは「技のタイプ」「元のタイプ」「任意」「無し」から選び、タイプ一致の各分岐が十分に出るようにする。
  const attackTera=teraPick(['',m.type,a.types[0],a.types[a.types.length-1],teraPick(teraTypeNames),teraPick(teraTypeNames)]);
  const defendTera=teraPick(['',teraPick(teraTypeNames),teraPick(teraTypeNames)]);
  const rank=()=>Object.fromEntries(keys.slice(1).map(k=>[k,Math.floor(teraRandom()*13)-6]));
  teraRandomCases.push(vector(genC,`tera-random/${String(i).padStart(5,'0')}`,a.name,d.name,m.name,{
    terrain,weather:teraPick(Object.keys(weatherNames)),
    critical:teraRandom()<0.1,screen:teraPick(['','Reflect','LightScreen','AuroraVeil']),
    a:{sp:randomSP(teraRandom),nature:teraPick(['Serious','Adamant','Modest','Bold','Calm']),item:teraPick(['','Life Orb','Expert Belt','Charcoal','Muscle Band','Wise Glasses']),ability:teraPick(['','Adaptability','Adaptability','Water Bubble','Levitate']),burn:teraRandom()<0.2,ranks:rank(),tera:attackTera},
    d:{sp:randomSP(teraRandom),nature:teraPick(['Serious','Adamant','Modest','Bold','Calm']),item:teraPick(['','Occa Berry','Chilan Berry']),ability:teraPick(['','Thick Fat','Filter','Solid Rock','Water Bubble','Levitate']),ranks:rank(),tera:defendTera}}));
}
{
  // 層の網羅(弱い生成で黙って偏らないように)。
  const count=pred=>teraRandomCases.filter(pred).length;
  const at=v=>v.input.Attacker.TeraType, dt=v=>v.input.Defender.TeraType;
  const orig=(v,t)=>v.input.Attacker.Species.Types.includes(t);
  const adapt=v=>v.input.Attacker.Ability.ID==='Adaptability';
  for (const [label,pred,min] of [
    ['攻撃側テラス=技=元タイプ', v=>at(v)&&at(v)===v.input.Move.Type&&orig(v,at(v)),200],
    ['攻撃側テラス=技(元タイプでない)', v=>at(v)&&at(v)===v.input.Move.Type&&!orig(v,at(v)),200],
    ['攻撃側テラス≠技・技=元タイプ', v=>at(v)&&at(v)!==v.input.Move.Type&&orig(v,v.input.Move.Type),200],
    ['てきおうりょく × 攻撃側テラス', v=>at(v)&&adapt(v),300],
    ['攻撃側テラス × フィールド', v=>at(v)&&v.input.Field.Terrain!=='none',300],
    ['防御側テラス × すなあらし/ゆき', v=>dt(v)&&['sand','snow'].includes(v.input.Field.Weather),200],
    ['防御側テラス × ミスト/サイコ', v=>dt(v)&&['misty','psychic'].includes(v.input.Field.Terrain),200],
    ['両側テラス', v=>at(v)&&dt(v),500],
    ['テラス無し(対照)', v=>!at(v)&&!dt(v),100],
  ]) assert(count(pred)>=min, `tera-random の層「${label}」が ${count(pred)} 件(下限 ${min})`);
}

// ADR-0009 §1/§6: engine の DefenderPresetCatalog と同じ8件。名前は engine の PresetKey と同一文字列。
const defensePresets=[
  ['none',{}],
  ['hp',{sp:{hp:32}}],
  ['hb_boost',{sp:{hp:32},nature:'Bold'}],
  ['hb',{sp:{hp:32,def:32}}],
  ['hb_full',{sp:{hp:32,def:32},nature:'Bold'}],
  ['hd_boost',{sp:{hp:32},nature:'Calm'}],
  ['hd',{sp:{hp:32,spd:32}}],
  ['hd_full',{sp:{hp:32,spd:32},nature:'Calm'}],
];
const attacks=[],defenses=[],statCases=[];
for(const s of species) {
  // Champions species set (ADR-0002 §決定5)。
  for(const category of [physical,special]) {
    const m=category.find(m=>s.types.includes(m.type));assert(m,`missing representative move: ${s.name}`);
    const atk=m.category==='Physical'?'atk':'spa';
    for(const boosted of [false,true]) for(const d of defenderAnchors)
      attacks.push(vector(genC,`attack/${s.id}/${m.id}/${boosted?'max':'zero'}/${id(d)}`,s.name,d,m.name,{a:boosted?{sp:{[atk]:32},nature:atk==='atk'?'Adamant':'Modest'}:{}}));
  }
  for(const [a,m] of attackAnchors) for(const [preset,opt] of defensePresets)
    defenses.push(vector(genC,`defense/${s.id}/${id(a)}/${preset}`,a,s.name,m,{d:opt}));
  for(const k of keys) for(const sp of [0,1,31,32]) for(const modifier of ['neutral','plus','minus']) {
    const n=[...genC.natures].find(n=>modifier==='neutral'?n.plus===n.minus:modifier==='plus'?n.plus===k&&n.minus!==k:n.minus===k&&n.plus!==k);
    // HP cannot receive nature changes. Three neutral nature cases still verify HP invariance.
    const p=individual(genC, s.name,{sp:{[k]:sp},nature:n?.name || 'Serious'});
    statCases.push({id:`stats/${s.id}/${k}/${sp}/${modifier}`,individual:p.input,expected:p.p.rawStats});
  }
}

// --- 技の機構の段階1(ADR-0142 §9): mechanisms.json -----------------------------------------------
// 多段・固定ダメージ(レベル)・必ず急所・防御ランク無視・攻撃に使う能力値(ボディプレス・イカサマ)・防御に使う能力値
// (サイコショック)を、oracle(Champions 世代)の値と照合する。一撃必殺は oracle が扱わない(威力 0 のままダメージ 0)ので
// 入れない。既存のファイル・プール・乱数列には足さない(新しいファイルだけ。乱数も使わない)。
// 技の機構と中身は oracle の技データ(Showdown 由来)から導く。技の名前で機構を決めない。ただし oracle が技名で
// 分岐する固定ダメージ(レベル)だけは、oracle のデータに無いので技名の一覧で与える。
const levelDamageMoves = ['Seismic Toss','Night Shade'];
function mechanismsOf(moveName) {
  const data = genC.moves.get(id(moveName));
  assert(data, `機構の調査用の技 ${moveName} が Champions 世代に無い`);
  const mechanisms = [];
  const params = {};
  if (data.multihit) {
    mechanisms.push('multi_hit');
    params.MultiHit = Array.isArray(data.multihit) ? {Min:data.multihit[0],Max:data.multihit[1]} : {Min:data.multihit,Max:data.multihit};
  }
  if (levelDamageMoves.includes(moveName)) {
    mechanisms.push('fixed_damage');
    params.FixedDamage = {Level:true,Value:0};
  }
  if (data.willCrit) mechanisms.push('always_crit');
  if (data.ignoreDefensive) mechanisms.push('ignore_defense_ranks');
  if (data.overrideOffensiveStat || data.overrideOffensivePokemon) {
    mechanisms.push('alt_offense_stat');
    if (data.overrideOffensiveStat) params.OffenseStat = data.overrideOffensiveStat;
    if (data.overrideOffensivePokemon) {
      assert.equal(data.overrideOffensivePokemon, 'target', `${moveName}: 想定外の overrideOffensivePokemon`);
      params.OffensePokemon = 'defender';
    }
  }
  if (data.overrideDefensiveStat) {
    mechanisms.push('alt_defense_stat');
    params.DefenseStat = data.overrideDefensiveStat;
  }
  assert(mechanisms.length > 0, `${moveName}: 段階1の機構を持たない`);
  return {mechanisms: mechanisms.sort(), params};
}
// 多段技の確定数(ADR-0142 §3): 1回の使用 = 発ごとの16段階の独立な乱数。n = ceil(HP / Σ最大)、n 回の使用の合計が HP 以上になる確率。
function koUses(hitRolls, hp) {
  const max = hitRolls.reduce((s, r) => s + r[15], 0), min = hitRolls.reduce((s, r) => s + r[0], 0);
  if (!max) return {Hits:0,Guaranteed:false,ChancePercent:0};
  const n = Math.ceil(hp/max);
  if (min*n >= hp) return {Hits:n,Guaranteed:true,ChancePercent:0};
  const budget = n*max-hp;
  let p = new Float64Array(budget+1); p[0] = 1;
  for (let use = 0; use < n; use++) for (const rolls of hitRolls) {
    const hmax = rolls[15];
    const next = new Float64Array(budget+1);
    for (let short = 0; short <= budget; short++) if (p[short]) {
      for (const roll of rolls) if (short+hmax-roll <= budget) next[short+hmax-roll] += p[short]/16;
    }
    p = next;
  }
  return {Hits:n,Guaranteed:false,ChancePercent:p.reduce((a,b) => a+b,0)*100};
}
// options は vector と同じ(a / d / critical / screen / weather / terrain)。攻撃側の特性は oracle の Move にも渡す
// (スキルリンクは Move の構築時に回数を決める。calculate() は決め直さない)。
function mechVector(label, a, d, moveName, options = {}) {
  const attack = individual(genC, a, options.a), defend = individual(genC, d, options.d);
  const weather = options.weather || 'none', terrain = options.terrain || 'none', screen = options.screen;
  const {mechanisms, params} = mechanismsOf(moveName);
  const m = new Move(genC, moveName, {isCrit:!!options.critical, ability:attack.p.ability || undefined});
  const f = new Field({gameType:'Singles',weather:weatherNames[weather],terrain:terrainNames[terrain],
    defenderSide:{isReflect:screen==='Reflect',isLightScreen:screen==='LightScreen',isAuroraVeil:screen==='AuroraVeil'}});
  const result = calculate(genC, attack.p, defend.p, m, f);
  const matrix = typeof result.damage === 'number' ? [Array(16).fill(result.damage)]
    : Array.isArray(result.damage[0]) ? result.damage : [result.damage];
  assert(matrix.every(r => r.length === 16 && r.every(Number.isInteger)));
  assert.equal(matrix.length, m.hits || 1, `${label}: 回数が oracle の Move と違う`);
  const rolls = matrix[0].map((_, i) => matrix.reduce((s, r) => s + r[i], 0));
  const hp = defend.p.rawStats.hp;
  const move = {ID:id(moveName),Type:m.type.toLowerCase(),Category:m.category.toLowerCase(),Power:m.bp,Priority:m.priority,Mechanisms:mechanisms};
  if (Object.keys(params).length) move.mechanismParams = params;
  const expected = {rolls,attackerStats:attack.p.rawStats,defenderStats:defend.p.rawStats,ko:koUses(matrix,hp)};
  if (matrix.length > 1) expected.hitRolls = matrix;
  return {id:label, oracle:{attacker:a,defender:d,move:moveName,ability:attack.p.ability || ''}, input:{Format:'single',Attacker:attack.input,Defender:defend.input,
    Move:move,
    Field:{Weather:weather,Terrain:terrain,DefenderScreens:{Reflect:screen==='Reflect',LightScreen:screen==='LightScreen',AuroraVeil:screen==='AuroraVeil'}},Critical:!!options.critical},
    expected};
}
const mechanismsFixed = [];
const addMech = (label, a, d, moveName, options) => {
  const v = mechVector(label, a, d, moveName, options);
  mechanismsFixed.push(v);
  return v;
};
// 比べる条件との oracle のダメージが変わる/変わらないを確かめてから足す(定義の取り違えで効かないケースだけを照合しない)。
function mechPair(label, a, d, moveName, options, baseOptions, expectChange) {
  const v = addMech(label, a, d, moveName, options);
  const base = mechVector(`${label}/base`, a, d, moveName, baseOptions);
  assert(v.expected.rolls.some(r => r > 0), `${label}: ダメージが0`);
  assert.equal(JSON.stringify(v.expected.rolls) !== JSON.stringify(base.expected.rolls), expectChange,
    `${label}: 比べた条件と oracle のダメージが${expectChange ? '変わらない' : '変わる'}`);
}
{
  // 多段: 固定回数(2回)・範囲 [2,5] の既定(最小+1=3)・スキルリンク(最大 5)・10 回。
  const multiPairs = [['Snorlax','Corviknight'],['Garchomp','Snorlax'],['Tyranitar','Metagross']];
  const multiVariants = [['', {}], ['crit', {critical:true}], ['reflect', {screen:'Reflect'}], ['light-screen', {screen:'LightScreen'}],
    ['burn', {a:{burn:true}}], ['life-orb', {a:{item:'Life Orb'}}], ['sun', {weather:'sun'}], ['ranks', {a:{ranks:{atk:2,spa:2}}, d:{ranks:{def:-1,spd:-1}}}]];
  const addAll = (label, moveName, pairs, variants, extra = {}) => {
    for (const [a, d] of pairs) for (const [vl, o] of variants) {
      addMech(`${label}/${a}/${d}/${vl || 'plain'}`, a, d, moveName, {...o, a:{...(o.a || {}),...(extra.a || {})}, d:{...(o.d || {}),...(extra.d || {})}});
    }
  };
  addAll('multi-hit-fixed', 'Dragon Darts', multiPairs, multiVariants);
  addAll('multi-hit-fixed', 'Twin Beam', multiPairs.slice(0, 2), multiVariants.slice(0, 3));
  addAll('multi-hit-range', 'Bullet Seed', multiPairs, multiVariants);
  addAll('multi-hit-range', 'Water Shuriken', multiPairs.slice(0, 2), multiVariants.slice(0, 3));
  addAll('multi-hit-skill-link', 'Bullet Seed', multiPairs, multiVariants, {a:{ability:'Skill Link'}});
  addAll('multi-hit-skill-link', 'Icicle Spear', multiPairs.slice(0, 2), multiVariants.slice(0, 3), {a:{ability:'Skill Link'}});
  // スキルリンクは固定回数の技を変えない。
  addAll('multi-hit-skill-link-fixed', 'Dragon Darts', multiPairs.slice(0, 2), multiVariants.slice(0, 2), {a:{ability:'Skill Link'}});
  addAll('multi-hit-ten', 'Population Bomb', multiPairs, multiVariants.slice(0, 4));
}
{
  // 必ず急所: 急所の指定なしで急所 / 急所に当たらない特性 / 急所の無効を無視する特性 / 急所のときに無視されるランク。
  const critMoves = ['Frost Breath','Storm Throw'];
  const pairs = [['Typhlosion','Snorlax'],['Scizor','Garchomp']];
  for (const mv of critMoves) for (const [a, d] of pairs) {
    addMech(`always-crit/${a}/${d}/${id(mv)}`, a, d, mv, {});
    addMech(`always-crit/${a}/${d}/${id(mv)}/ranks`, a, d, mv, {a:{ranks:{atk:-2,spa:-2}}, d:{ranks:{def:2,spd:2}}});
    addMech(`always-crit/${a}/${d}/${id(mv)}/reflect`, a, d, mv, {screen:'Reflect'});
    addMech(`always-crit-shell-armor/${a}/${d}/${id(mv)}`, a, d, mv, {d:{ability:'Shell Armor'}});
    addMech(`always-crit-shell-armor/${a}/${d}/${id(mv)}/mold-breaker`, a, d, mv, {a:{ability:ignorer}, d:{ability:'Shell Armor'}});
    addMech(`always-crit/${a}/${d}/${id(mv)}/sniper`, a, d, mv, {a:{ability:'Sniper'}});
  }
  // 防御ランク無視: 防御側の +6 / -6(物理)。
  for (const mv of ['Sacred Sword','Darkest Lariat']) for (const [a, d] of [['Garchomp','Snorlax'],['Scizor','Tyranitar']]) {
    for (const rank of [6, -6, 2, 0]) addMech(`ignore-defense-ranks/${a}/${d}/${id(mv)}/def${rank}`, a, d, mv, {d:{ranks:{def:rank}}});
    addMech(`ignore-defense-ranks/${a}/${d}/${id(mv)}/crit`, a, d, mv, {critical:true, d:{ranks:{def:6}}});
    addMech(`ignore-defense-ranks/${a}/${d}/${id(mv)}/reflect`, a, d, mv, {screen:'Reflect', d:{ranks:{def:-3}}});
    addMech(`ignore-defense-ranks/${a}/${d}/${id(mv)}/unaware`, a, d, mv, {d:{ability:'Unaware', ranks:{def:3}}});
  }
}
{
  // 攻撃に使う能力値: ボディプレス(攻撃側の防御とそのランク)・イカサマ(防御側の攻撃とそのランク)。
  const bodyPress = [['Corviknight','Snorlax'],['Toxapex','Garchomp']];
  for (const [a, d] of bodyPress) {
    const base = {a:{sp:{def:20,atk:4}}};
    addMech(`alt-offense-def/${a}/${d}/plain`, a, d, 'Body Press', base);
    addMech(`alt-offense-def/${a}/${d}/def-plus`, a, d, 'Body Press', {a:{sp:{def:20}, ranks:{def:2,atk:-3}}});
    addMech(`alt-offense-def/${a}/${d}/def-minus-crit`, a, d, 'Body Press', {critical:true, a:{sp:{def:20}, ranks:{def:-2,atk:4}}});
    addMech(`alt-offense-def/${a}/${d}/def-minus`, a, d, 'Body Press', {a:{sp:{def:20}, ranks:{def:-2}}});
    addMech(`alt-offense-def/${a}/${d}/burn`, a, d, 'Body Press', {a:{sp:{def:20}, burn:true}});
    addMech(`alt-offense-def/${a}/${d}/huge-power`, a, d, 'Body Press', {a:{sp:{def:20}, ability:'Huge Power'}});
    addMech(`alt-offense-def/${a}/${d}/muscle-band`, a, d, 'Body Press', {a:{sp:{def:20}, item:'Muscle Band'}});
    addMech(`alt-offense-def/${a}/${d}/unaware`, a, d, 'Body Press', {a:{sp:{def:20}, ranks:{def:2}}, d:{ability:'Unaware'}});
    addMech(`alt-offense-def/${a}/${d}/reflect`, a, d, 'Body Press', {screen:'Reflect', a:{sp:{def:20}}});
  }
  for (const [a, d] of [['Gengar','Snorlax'],['Scizor','Tyranitar']]) {
    const tgt = {d:{sp:{atk:32}, nature:'Adamant'}};
    addMech(`alt-offense-target/${a}/${d}/plain`, a, d, 'Foul Play', tgt);
    addMech(`alt-offense-target/${a}/${d}/target-plus`, a, d, 'Foul Play', {d:{...tgt.d, ranks:{atk:2}}, a:{ranks:{atk:-3}}});
    addMech(`alt-offense-target/${a}/${d}/target-minus-crit`, a, d, 'Foul Play', {critical:true, d:{...tgt.d, ranks:{atk:-2}}});
    addMech(`alt-offense-target/${a}/${d}/attacker-rank`, a, d, 'Foul Play', {...tgt, a:{ranks:{atk:6}}});
    addMech(`alt-offense-target/${a}/${d}/huge-power`, a, d, 'Foul Play', {...tgt, a:{ability:'Huge Power'}});
    addMech(`alt-offense-target/${a}/${d}/unaware`, a, d, 'Foul Play', {d:{...tgt.d, ranks:{atk:3}, ability:'Unaware'}});
    addMech(`alt-offense-target/${a}/${d}/burn`, a, d, 'Foul Play', {...tgt, a:{burn:true}});
  }
}
{
  // 防御に使う能力値: サイコショック(特殊技が防御を参照)。壁・やけどは分類(特殊)で決まる。
  const shockPairs = [['Gengar','Snorlax'],['Pikachu','Tyranitar'],['Gengar','Abomasnow']];
  for (const [a, d] of shockPairs) {
    const sp = {d:{sp:{def:20}, ranks:{def:1,spd:-2}}};
    addMech(`alt-defense-def/${a}/${d}/plain`, a, d, 'Psyshock', sp);
    addMech(`alt-defense-def/${a}/${d}/light-screen`, a, d, 'Psyshock', {...sp, screen:'LightScreen'});
    addMech(`alt-defense-def/${a}/${d}/reflect`, a, d, 'Psyshock', {...sp, screen:'Reflect'});
    addMech(`alt-defense-def/${a}/${d}/aurora-veil`, a, d, 'Psyshock', {...sp, screen:'AuroraVeil'});
    addMech(`alt-defense-def/${a}/${d}/crit`, a, d, 'Psyshock', {...sp, critical:true});
    addMech(`alt-defense-def/${a}/${d}/sand`, a, d, 'Psyshock', {...sp, weather:'sand'});
    addMech(`alt-defense-def/${a}/${d}/snow`, a, d, 'Psyshock', {...sp, weather:'snow'});
    addMech(`alt-defense-def/${a}/${d}/fur-coat`, a, d, 'Psyshock', {d:{...sp.d, ability:'Fur Coat'}});
    addMech(`alt-defense-def/${a}/${d}/unaware`, a, d, 'Psyshock', {a:{ranks:{spa:2}}, d:{...sp.d, ability:'Unaware'}});
  }
  // 氷タイプの防御側 × ゆきは防御が上がる(分類でなく参照する能力値で決まる)。
  assert(species.some(s => s.name === 'Abomasnow' && s.types.includes('Ice')), 'サイコショックのゆきの対照に使う氷タイプが居ない');
}
{
  // 固定ダメージ(レベル): Lv50 のダメージ。一致・相性・急所・やけど・壁・持ち物・ランクで変わらない。タイプ相性で無効。
  for (const mv of levelDamageMoves) {
    const [a, d, im] = mv === 'Seismic Toss' ? ['Snorlax','Garchomp','Gengar'] : ['Gengar','Snorlax','Snorlax'];
    addMech(`fixed-damage-level/${a}/${d}/${id(mv)}/plain`, a, d, mv, {});
    addMech(`fixed-damage-level/${a}/${d}/${id(mv)}/crit-burn-reflect`, a, d, mv, {critical:true, screen:'Reflect', a:{burn:true, item:'Life Orb', ranks:{atk:6,spa:6}}, d:{ranks:{def:-6,spd:-6}}});
    addMech(`fixed-damage-level/${a}/${d}/${id(mv)}/stab-weather`, a, d, mv, {weather:'sun'});
    addMech(`fixed-damage-level/${a}/${d}/${id(mv)}/tough`, a, d, mv, {d:{sp:{hp:32,def:17,spd:17}}});
    // タイプ相性の無効(かくとう → ゴースト・ゴースト → ノーマル)。
    const imm = mv === 'Seismic Toss' ? 'Gengar' : 'Snorlax';
    const v = addMech(`fixed-damage-immune/${a}/${imm}/${id(mv)}`, a, imm, mv, {});
    assert(v.expected.rolls.every(r => r === 0), `fixed-damage-immune/${mv}: oracle が無効になっていない`);
    // 特性による無効・吸収は技のタイプで決まる(かくとう技を無効にする特性は Champions に無いのでゴースト技で確かめる)。
  }
}
// --- 機構と組み合わさる特性の効果(ADR-0142 §3・§4): effects.json の定義から apply / control / breakable を作る ----------------
for (const name of Object.keys(effects.abilities).sort()) {
  const def = effects.abilities[name];
  if (!isMechanismEffect(def)) continue;
  const slug = `effects/${id(name)}`;
  if (def.MaxMultiHit) {
    // 範囲のある多段技で最大回数になる。固定回数の技は変えない。
    mechPair(`${slug}/range/apply`, 'Snorlax', 'Corviknight', 'Bullet Seed', {a:{ability:name}}, {}, true);
    mechPair(`${slug}/fixed/control`, 'Snorlax', 'Corviknight', 'Dragon Darts', {a:{ability:name}}, {}, false);
    // 防御側が持っても効かない。
    mechPair(`${slug}/defender/control`, 'Snorlax', 'Corviknight', 'Bullet Seed', {d:{ability:name}}, {}, false);
  }
  if (def.PreventsOHKO) {
    // 一撃必殺は oracle が扱わない。ここでは通常の多段技のダメージを変えないこと(対照)と、Breakable の照合だけを置く。
    mechPair(`${slug}/control`, 'Snorlax', 'Corviknight', 'Bullet Seed', {d:{ability:name}}, {}, false);
    if (def.Breakable) {
      const withIgnorer = {a:{ability:ignorer}, d:{ability:name}};
      mechPair(`${slug}/breakable`, 'Snorlax', 'Corviknight', 'Bullet Seed', withIgnorer, {a:{ability:ignorer}}, false);
    }
  }
}

// --- 技の機構の段階2(ADR-0143 §7): mechanisms-stage2.json ---------------------------------------------
// 既存の入力(状態異常・持ち物・天候・フィールド・ランク・壁)と種族の重さ・素早さだけで決まる技の処理を、
// effects.json の moveRules(技 → engine の MoveRule)をベクタの入力(Move.Rule)に載せて oracle(Champions 世代)と照合する。
// 技の名前で機構や結果を決めない(機構は moveRules の中身から導く)。既存のファイルのバイト列・乱数列は変えない
// (新しいファイルだけ。乱数も使わない)。ベクタは種族の重さ(WeightHg = oracle の weightkg × 10)と、
// 持ち物・特性の効果(素早さの補正 speedItems / speedAbilities を含む)を入力に載せる。
const stage2Rules = effects.moveRules;
// 段階3の語彙(ADR-0144 §1)を使う定義。段階2の網羅(moveRules の全技に1件以上)はこの技を数えず、段階3の節が数える。
const stage3PowerFormulas = ['attacker_hp_scaled','attacker_hp_low','defender_hp_ratio','attacker_item_fling'];
const isStage3Rule = r => stage3PowerFormulas.includes(r.PowerFormula) || !!r.FixedDamageFormula || !!r.CategoryByStats;
assert(stage2Rules && Object.keys(stage2Rules).length > 0, 'testdata/golden/effects.json に moveRules が無い');
assert(effects.speedItems && effects.speedAbilities, 'testdata/golden/effects.json に speedItems / speedAbilities が無い');
const stage2Fixed = [];
const stage2CoveredMoves = new Set();
// moveRules の中身から、engine の検証(機構との対応)を満たしつつ未対応の印が付かない機構の一覧を作る。
function stage2Mechanisms(rule) {
  const mechanisms = [];
  if (rule.PowerFormula || rule.PowerBoosts || rule.IgnoresBurn) mechanisms.push('variable_power');
  if (rule.PowerFormula === 'hit_index') mechanisms.push('multi_hit');
  if (rule.MoveSpecificResolved) mechanisms.push('move_specific');
  if (rule.TypeByWeather || rule.TypeByTerrain) mechanisms.push('type_change');
  if (rule.ExtraEffectivenessType || rule.SuperEffectiveAgainst) mechanisms.push('effectiveness_change');
  if (rule.TerrainPowerMods) mechanisms.push('field_specific');
  if (rule.PriorityBoost) mechanisms.push('priority_change');
  if (rule.FixedDamageFormula) mechanisms.push('fixed_damage'); // 段階3(ADR-0144)
  if (rule.CategoryByStats) mechanisms.push('move_specific');
  assert(mechanisms.length > 0, '技の処理の定義が機構を持たない');
  return mechanisms.sort();
}
// options は vector と同じ(a / d / critical / screen / weather / terrain / format)。技のデータ(タイプ・威力・対象)は
// calculate() が書き換える(ウェザーボールのタイプ等)ので、呼ぶ前に写しておく。
function stage2Vector(label, a, d, moveName, options = {}) {
  const rule = stage2Rules[moveName];
  assert(rule, `${label}: moveRules に ${moveName} が無い`);
  const attack = individual(genC, a, {...options.a, stage2:'a'}), defend = individual(genC, d, {...options.d, stage2:'d'});
  const weather = options.weather || 'none', terrain = options.terrain || 'none', screen = options.screen;
  const format = options.format || 'single';
  assert(gameTypeNames[format], `未知の形式 ${format}`);
  const m = new Move(genC, moveName, {isCrit:!!options.critical, ability:attack.p.ability || undefined});
  const data = {type:m.type, category:m.category, bp:m.bp, priority:m.priority, hits:m.hits || 1, target:moveTargetOf(m), multihit:genC.moves.get(id(moveName)).multihit};
  const f = new Field({gameType:gameTypeNames[format],weather:weatherNames[weather],terrain:terrainNames[terrain],
    defenderSide:{isReflect:screen==='Reflect',isLightScreen:screen==='LightScreen',isAuroraVeil:screen==='AuroraVeil'}});
  const result = calculate(genC, attack.p, defend.p, m, f);
  const matrix = typeof result.damage === 'number' ? [Array(16).fill(result.damage)]
    : Array.isArray(result.damage[0]) ? result.damage : [result.damage];
  assert(matrix.every(r => r.length === 16 && r.every(Number.isInteger)), `${label}: oracle のダメージが16段階の整数でない`);
  assert.equal(matrix.length, data.hits, `${label}: 回数が oracle の Move と違う`);
  const rolls = matrix[0].map((_, i) => matrix.reduce((s, r) => s + r[i], 0));
  const hp = defend.p.rawStats.hp;
  const move = {ID:id(moveName),Type:data.type.toLowerCase(),Category:data.category.toLowerCase(),Power:data.bp,Priority:data.priority,
    Mechanisms:stage2Mechanisms(rule),Rule:rule};
  if (rule.PowerFormula === 'hit_index') {
    assert(data.multihit, `${label}: hit_index の技が多段でない`);
    move.mechanismParams = {MultiHit:Array.isArray(data.multihit) ? {Min:data.multihit[0],Max:data.multihit[1]} : {Min:data.multihit,Max:data.multihit}};
  }
  if (format === 'double') move.Target = data.target; // 未指定だとダブルで「対象が不明」の印が付く
  const expected = {rolls,attackerStats:attack.p.rawStats,defenderStats:defend.p.rawStats,ko:matrix.length > 1 ? koUses(matrix,hp) : ko(rolls,hp)};
  if (matrix.length > 1) expected.hitRolls = matrix;
  return {id:label, oracle:{attacker:a,defender:d,move:moveName}, input:{Format:format,Attacker:attack.input,Defender:defend.input,
    Move:move,
    Field:{Weather:weather,Terrain:terrain,DefenderScreens:{Reflect:screen==='Reflect',LightScreen:screen==='LightScreen',AuroraVeil:screen==='AuroraVeil'}},Critical:!!options.critical},
    expected};
}
const addStage2 = (label, a, d, moveName, options) => {
  const v = stage2Vector(label, a, d, moveName, options);
  stage2Fixed.push(v);
  stage2CoveredMoves.add(moveName);
  return v;
};
const hasDamage = v => v.expected.rolls.some(r => r > 0);
// pair は apply(label)と control(controlLabel。省略時は足さず比べるだけ)を作り、oracle のダメージが
// 変わる/変わらないが想定どおりであることを確かめる(定義の取り違えで効かない・効きすぎるケースを照合しない)。
function stage2Pair(label, controlLabel, a, d, moveName, options, controlOptions, expectChange = true) {
  const v = addStage2(label, a, d, moveName, options);
  const control = controlLabel ? addStage2(controlLabel, a, d, moveName, controlOptions) : stage2Vector(`${label}/base`, a, d, moveName, controlOptions);
  assert(hasDamage(v) || hasDamage(control), `${label}: ダメージが0(相性で無効な組を選んだ)`);
  assert.equal(JSON.stringify(v.expected.rolls) !== JSON.stringify(control.expected.rolls), expectChange,
    `${label}: 比べた条件と oracle のダメージが${expectChange ? '変わらない' : '変わる'}`);
  return v;
}
// 比べる相手のベクタを足さない pair(同じ control を複数の apply で共有するとき、control は1度だけ addStage2 で足す)。
const compareStage2 = (label, a, d, moveName, options, control, expectChange = true) => {
  const v = addStage2(label, a, d, moveName, options);
  assert(hasDamage(v) || hasDamage(control), `${label}: ダメージが0(相性で無効な組を選んだ)`);
  if (expectChange !== 'any') {
    assert.equal(JSON.stringify(v.expected.rolls) !== JSON.stringify(control.expected.rolls), expectChange,
      `${label}: 比べた条件と oracle のダメージが${expectChange ? '変わらない' : '変わる'}`);
  }
  return v;
};
// グループ単位の保証: 比べた条件で oracle のダメージが変わった組が1つ以上あること(素早さの補正は表の段をまたがないと変わらないため、
// 組ごとには問わない)。
const changedGroups = {};
const trackChange = (group, v, control) => {
  changedGroups[group] ||= false;
  if (JSON.stringify(v.expected.rolls) !== JSON.stringify(control.expected.rolls)) changedGroups[group] = true;
};
const slugOf = name => id(name);

// 状態異常: 攻撃側(からげんき)・防御側(たたりめ型・ベノムショック型・ベノムトラップ型)。
{
  const facadePairs = [['Snorlax','Garchomp'],['Typhlosion','Tyranitar']];
  for (const [a, d] of facadePairs) {
    const none = addStage2(`status-none/facade/${a}/${d}/none`, a, d, 'Facade', {});
    for (const st of ['brn','par','psn','tox']) compareStage2(`status-attacker/facade/${a}/${d}/${st}`, a, d, 'Facade', {a:{status:st}}, none);
    // こおり・ねむりは一覧に無い(Showdown はこおりを含むが oracle に従う)。状態なしと同じダメージ。
    for (const st of ['slp','frz']) compareStage2(`status-none/facade/${a}/${d}/${st}`, a, d, 'Facade', {a:{status:st}}, none, false);
    // やけどの攻撃半減を受けない: やけど × からげんきは、まひ × からげんきと同じダメージ(2倍のみ)。
    const par = stage2Vector(`status-attacker/facade/${a}/${d}/par-ref`, a, d, 'Facade', {a:{status:'par'}});
    compareStage2(`status-attacker/facade/${a}/${d}/brn-no-halving`, a, d, 'Facade', {a:{status:'brn'}}, par, false);
    // 補正の連鎖: 8192 とこだわり系でない持ち物(ライフオーブ)。
    addStage2(`status-attacker/facade/${a}/${d}/brn-life-orb`, a, d, 'Facade', {a:{status:'brn', item:'Life Orb'}});
    addStage2(`status-attacker/facade/${a}/${d}/tox-reflect-crit`, a, d, 'Facade', {a:{status:'tox'}, screen:'Reflect', critical:true});
  }
  const statusAll = ['brn','par','psn','tox','slp','frz'];
  const defenderMoves = [
    ['Hex', [['Gengar','Garchomp'],['Gengar','Metagross']], statusAll],
    ['Infernal Parade', [['Gengar','Garchomp'],['Gengar','Metagross']], statusAll],
    ['Venoshock', [['Gengar','Garchomp'],['Gengar','Snorlax']], ['psn','tox']],
    ['Barb Barrage', [['Garchomp','Snorlax'],['Tyranitar','Garchomp']], ['psn','tox']],
  ];
  for (const [moveName, pairs, statuses] of defenderMoves) for (const [a, d] of pairs) {
    const slug = slugOf(moveName);
    const none = addStage2(`status-none/${slug}/${a}/${d}/none`, a, d, moveName, {});
    for (const st of statuses) compareStage2(`status-defender/${slug}/${a}/${d}/${st}`, a, d, moveName, {d:{status:st}}, none);
    // 一覧に無い状態は状態なしと同じダメージ(ベノムショック・ベノムトラップ型は毒だけ)。
    if (statuses.length === 2) for (const st of ['brn','par']) compareStage2(`status-none/${slug}/${a}/${d}/${st}`, a, d, moveName, {d:{status:st}}, none, false);
    addStage2(`status-defender/${slug}/${a}/${d}/tox-technician`, a, d, moveName, {a:{ability:'Technician'}, d:{status:'tox'}});
    addStage2(`status-defender/${slug}/${a}/${d}/psn-crit-reflect`, a, d, moveName, {d:{status:'psn'}, critical:true, screen:moveName === 'Barb Barrage' ? 'Reflect' : 'LightScreen'});
  }
}
// 持ち物: アクロバット型(攻撃側が持ち物なし)・はたきおとす型(防御側が持ち物あり)・ポルターガイスト型(防御側が持ち物なしで失敗)。
{
  for (const [a, d] of [['Corviknight','Garchomp'],['Charizard','Snorlax']]) {
    const held = addStage2(`item-held/acrobatics/${a}/${d}/muscle-band`, a, d, 'Acrobatics', {a:{item:'Muscle Band'}});
    compareStage2(`item-none/acrobatics/${a}/${d}/none`, a, d, 'Acrobatics', {}, held);
    addStage2(`item-held/acrobatics/${a}/${d}/life-orb`, a, d, 'Acrobatics', {a:{item:'Life Orb'}});
    addStage2(`item-none/acrobatics/${a}/${d}/technician`, a, d, 'Acrobatics', {a:{ability:'Technician'}});
    addStage2(`item-none/acrobatics/${a}/${d}/crit-reflect`, a, d, 'Acrobatics', {critical:true, screen:'Reflect'});
  }
  for (const [a, d] of [['Tyranitar','Snorlax'],['Scizor','Garchomp'],['Weavile','Clefable']]) {
    const plain = addStage2(`item-none/knockoff/${a}/${d}/none`, a, d, 'Knock Off', {});
    compareStage2(`item-removable/knockoff/${a}/${d}/muscle-band`, a, d, 'Knock Off', {d:{item:'Muscle Band'}}, plain);
    compareStage2(`item-removable/knockoff/${a}/${d}/life-orb`, a, d, 'Knock Off', {d:{item:'Life Orb'}}, plain);
    addStage2(`item-removable/knockoff/${a}/${d}/muscle-band-technician`, a, d, 'Knock Off', {a:{ability:'Technician'}, d:{item:'Muscle Band'}});
    addStage2(`item-removable/knockoff/${a}/${d}/muscle-band-crit`, a, d, 'Knock Off', {d:{item:'Muscle Band'}, critical:true});
  }
  for (const [a, d] of [['Gengar','Garchomp'],['Gengar','Metagross']]) {
    const failed = addStage2(`item-failed/poltergeist/${a}/${d}/none`, a, d, 'Poltergeist', {});
    assert(!hasDamage(failed), `item-failed/poltergeist/${a}/${d}: 持ち物なしで失敗していない`);
    const held = addStage2(`item-held/poltergeist/${a}/${d}/muscle-band`, a, d, 'Poltergeist', {d:{item:'Muscle Band'}});
    assert(hasDamage(held), `item-held/poltergeist/${a}/${d}: 持ち物ありで当たっていない`);
    addStage2(`item-held/poltergeist/${a}/${d}/life-orb-crit`, a, d, 'Poltergeist', {a:{item:'Life Orb'}, d:{item:'Muscle Band'}, critical:true});
  }
}
// 天候: ウェザーボール型(タイプと威力)・ソーラービーム型(雨・砂・雪で半減)。
{
  for (const [a, d] of [['Typhlosion','Snorlax'],['Blastoise','Tyranitar']]) {
    const none = addStage2(`weather/weatherball/${a}/${d}/none`, a, d, 'Weather Ball', {});
    for (const w of ['sun','rain','sand','snow']) compareStage2(`weather/weatherball/${a}/${d}/${w}`, a, d, 'Weather Ball', {weather:w}, none);
    addStage2(`weather/weatherball/${a}/${d}/rain-reflect-crit`, a, d, 'Weather Ball', {weather:'rain', critical:true, screen:'LightScreen'});
    addStage2(`weather/weatherball/${a}/${d}/sun-life-orb`, a, d, 'Weather Ball', {weather:'sun', a:{item:'Life Orb'}});
  }
  for (const [moveName, a, d] of [['Solar Beam','Venusaur','Snorlax'],['Solar Beam','Sceptile','Garchomp'],['Solar Blade','Sceptile','Snorlax'],['Solar Blade','Venusaur','Tyranitar']]) {
    const slug = slugOf(moveName);
    const none = addStage2(`weather/${slug}/${a}/${d}/none`, a, d, moveName, {});
    for (const w of ['rain','sand','snow']) compareStage2(`weather/${slug}/${a}/${d}/${w}`, a, d, moveName, {weather:w}, none);
    compareStage2(`weather/${slug}/${a}/${d}/sun`, a, d, moveName, {weather:'sun'}, none, false);
    addStage2(`weather/${slug}/${a}/${d}/snow-technician`, a, d, moveName, {weather:'snow', a:{ability:'Technician'}});
  }
}
// フィールド: テラバースト型(攻撃側が接地のときだけタイプと威力)・ミストバースト型・ワイドフォース型・ライジングボルト型(防御側が接地)・
// じしん型(グラスフィールドで防御側が接地なら半減)。浮いている側(ひこう)は変わらない。
{
  const terrains = ['electric','grassy','misty','psychic'];
  for (const [a, d] of [['Gardevoir','Snorlax'],['Clefable','Tyranitar']]) {
    const none = addStage2(`terrain-attacker/terrainpulse/${a}/${d}/none`, a, d, 'Terrain Pulse', {});
    for (const t of terrains) compareStage2(`terrain-attacker/terrainpulse/${a}/${d}/${t}`, a, d, 'Terrain Pulse', {terrain:t}, none);
    addStage2(`terrain-attacker/terrainpulse/${a}/${d}/electric-crit-reflect`, a, d, 'Terrain Pulse', {terrain:'electric', critical:true, screen:'LightScreen'});
  }
  for (const [a, d] of [['Corviknight','Snorlax'],['Charizard','Garchomp']]) {
    const none = stage2Vector(`terrain-airborne/terrainpulse/${a}/${d}/none-ref`, a, d, 'Terrain Pulse', {});
    for (const t of terrains) compareStage2(`terrain-airborne/terrainpulse/${a}/${d}/${t}`, a, d, 'Terrain Pulse', {terrain:t}, none, false);
  }
  for (const [a, d] of [['Gardevoir','Garchomp'],['Clefable','Tyranitar']]) {
    const none = addStage2(`terrain-attacker/mistyexplosion/${a}/${d}/none`, a, d, 'Misty Explosion', {});
    compareStage2(`terrain-attacker/mistyexplosion/${a}/${d}/misty`, a, d, 'Misty Explosion', {terrain:'misty'}, none);
    compareStage2(`terrain-attacker/mistyexplosion/${a}/${d}/electric`, a, d, 'Misty Explosion', {terrain:'electric'}, none, false);
    addStage2(`terrain-attacker/mistyexplosion/${a}/${d}/misty-life-orb-crit`, a, d, 'Misty Explosion', {terrain:'misty', a:{item:'Life Orb'}, critical:true});
  }
  for (const [a, d] of [['Charizard','Garchomp'],['Corviknight','Tyranitar']]) {
    const none = stage2Vector(`terrain-airborne/mistyexplosion/${a}/${d}/none-ref`, a, d, 'Misty Explosion', {});
    compareStage2(`terrain-airborne/mistyexplosion/${a}/${d}/misty`, a, d, 'Misty Explosion', {terrain:'misty'}, none, false);
  }
  for (const [a, d] of [['Gardevoir','Snorlax'],['Gengar','Garchomp']]) {
    const none = addStage2(`terrain-attacker/expandingforce/${a}/${d}/none`, a, d, 'Expanding Force', {});
    compareStage2(`terrain-attacker/expandingforce/${a}/${d}/psychic`, a, d, 'Expanding Force', {terrain:'psychic'}, none);
    addStage2(`terrain-attacker/expandingforce/${a}/${d}/psychic-technician-crit`, a, d, 'Expanding Force', {terrain:'psychic', a:{ability:'Technician'}, critical:true});
    // 他のフィールドでは変わらない。
    compareStage2(`terrain-attacker/expandingforce/${a}/${d}/grassy`, a, d, 'Expanding Force', {terrain:'grassy'}, none, false);
    // ダブル: サイコフィールドで攻撃側が接地なら全体技。浮いている側は単体のまま。
    const doubleNone = addStage2(`terrain-spread/expandingforce/${a}/${d}/double-none`, a, d, 'Expanding Force', {format:'double'});
    compareStage2(`terrain-spread/expandingforce/${a}/${d}/double-psychic`, a, d, 'Expanding Force', {format:'double', terrain:'psychic'}, doubleNone);
    addStage2(`terrain-spread/expandingforce/${a}/${d}/double-psychic-reflect`, a, d, 'Expanding Force', {format:'double', terrain:'psychic', screen:'LightScreen'});
  }
  {
    const doubleNone = stage2Vector('terrain-airborne/expandingforce/Charizard/Snorlax/double-none-ref', 'Charizard', 'Snorlax', 'Expanding Force', {format:'double'});
    const airborne = addStage2('terrain-airborne/expandingforce/Charizard/Snorlax/double-psychic', 'Charizard', 'Snorlax', 'Expanding Force', {format:'double', terrain:'psychic'});
    // 浮いていて全体技にならず、サイコフィールドの威力の補正も受けない: 単体のまま(フィールドなしと同じダメージ)。
    assert.equal(JSON.stringify(airborne.expected.rolls), JSON.stringify(doubleNone.expected.rolls), '浮いた攻撃側のワイドフォース型がフィールドなしと違う');
  }
  for (const [a, d] of [['Pikachu','Snorlax'],['Jolteon','Blastoise']]) {
    const none = addStage2(`terrain-defender/risingvoltage/${a}/${d}/none`, a, d, 'Rising Voltage', {});
    compareStage2(`terrain-defender/risingvoltage/${a}/${d}/electric`, a, d, 'Rising Voltage', {terrain:'electric'}, none);
    addStage2(`terrain-defender/risingvoltage/${a}/${d}/electric-crit-reflect`, a, d, 'Rising Voltage', {terrain:'electric', critical:true, screen:'LightScreen'});
    compareStage2(`terrain-defender/risingvoltage/${a}/${d}/grassy`, a, d, 'Rising Voltage', {terrain:'grassy'}, none, false);
  }
  for (const [a, d] of [['Pikachu','Corviknight'],['Jolteon','Charizard']]) {
    const none = stage2Vector(`terrain-airborne/risingvoltage/${a}/${d}/none-ref`, a, d, 'Rising Voltage', {});
    // 防御側が浮いていれば ×2 を受けない。攻撃側のエレキフィールドの 1.3 倍は掛かるので、それだけの差。
    addStage2(`terrain-airborne/risingvoltage/${a}/${d}/electric`, a, d, 'Rising Voltage', {terrain:'electric'});
    assert(hasDamage(none), `terrain-airborne/risingvoltage/${a}/${d}: ダメージが0`);
  }
  for (const [moveName, pairs] of [['Earthquake', [['Garchomp','Snorlax'],['Excadrill','Typhlosion']]], ['Bulldoze', [['Garchomp','Tyranitar'],['Excadrill','Typhlosion']]]]) {
    for (const [a, d] of pairs) {
      const slug = slugOf(moveName);
      const none = addStage2(`terrain-defender/${slug}/${a}/${d}/none`, a, d, moveName, {});
      compareStage2(`terrain-defender/${slug}/${a}/${d}/grassy`, a, d, moveName, {terrain:'grassy'}, none);
      compareStage2(`terrain-defender/${slug}/${a}/${d}/electric`, a, d, moveName, {terrain:'electric'}, none, false);
      addStage2(`terrain-defender/${slug}/${a}/${d}/grassy-crit-reflect`, a, d, moveName, {terrain:'grassy', critical:true, screen:'Reflect'});
      addStage2(`terrain-defender/${slug}/${a}/${d}/double-grassy`, a, d, moveName, {format:'double', terrain:'grassy'});
    }
  }
  // 地面技を受けない浮いた防御側(ひこう)に当てる組は、タイプ相性で無効(oracle が 0)。グラスフィールドの半減は関係しない。
  for (const [a, d] of [['Garchomp','Corviknight']]) {
    const v = addStage2(`terrain-airborne/earthquake/${a}/${d}/grassy`, a, d, 'Earthquake', {terrain:'grassy'});
    assert(!hasDamage(v), `terrain-airborne/earthquake/${a}/${d}: 無効になっていない`);
  }
}
// 優先度: グラススライダー型はグラスフィールドで +1。サイコフィールドと同時には起きないので、どちらでも当たる。
{
  for (const [a, d] of [['Sceptile','Snorlax'],['Venusaur','Garchomp']]) {
    for (const t of ['none','psychic','grassy']) {
      const v = addStage2(`priority-terrain/grassyglide/${a}/${d}/${t}`, a, d, 'Grassy Glide', {terrain:t});
      assert(hasDamage(v), `priority-terrain/grassyglide/${a}/${d}/${t}: ダメージが0`);
    }
    addStage2(`priority-terrain/grassyglide/${a}/${d}/psychic-crit`, a, d, 'Grassy Glide', {terrain:'psychic', critical:true});
  }
}
// ランク: アシストパワー型(攻撃側の正のランクだけを数える)。
{
  for (const [moveName, a, d] of [['Stored Power','Gardevoir','Snorlax'],['Stored Power','Gengar','Garchomp'],['Power Trip','Tyranitar','Snorlax'],['Power Trip','Weavile','Gardevoir']]) {
    const slug = slugOf(moveName);
    const none = addStage2(`ranks/${slug}/${a}/${d}/none`, a, d, moveName, {});
    compareStage2(`ranks/${slug}/${a}/${d}/plus2`, a, d, moveName, {a:{ranks:{atk:2}}}, none);
    compareStage2(`ranks/${slug}/${a}/${d}/mixed`, a, d, moveName, {a:{ranks:{atk:2,def:1,spa:-1,spe:3}}}, none);
    compareStage2(`ranks/${slug}/${a}/${d}/max`, a, d, moveName, {a:{ranks:{atk:6,def:6,spa:6,spd:6,spe:6}}}, none);
    // 負のランクは数えない: 威力は変わらず、攻撃の実数値だけが変わる。ランクなしと違うのは実数値の分。
    addStage2(`ranks/${slug}/${a}/${d}/negative-only`, a, d, moveName, {a:{ranks:{def:-2,spd:-6}}});
    addStage2(`ranks/${slug}/${a}/${d}/plus1-defender-ranks`, a, d, moveName, {a:{ranks:{spe:1}}, d:{ranks:{def:2,spd:2}}});
    addStage2(`ranks/${slug}/${a}/${d}/plus3-crit-reflect`, a, d, moveName, {a:{ranks:{def:3}}, critical:true, screen:'Reflect'});
  }
}
// 素早さ: エレキボール型(比の表)・ジャイロボール型(式)。ランク・まひ・素早さの補正(特性・持ち物)を入れた素早さで決まる。
{
  const speedPairs = [['Jolteon','Snorlax'],['Pikachu','Tyranitar'],['Gengar','Garchomp'],['Snorlax','Weavile']];
  const ball = (moveName, slug, label) => {
    for (const [a, d] of speedPairs) {
      // a が電気・鋼でなくても威力の式は同じ(タイプ相性・一致の有無はダメージの差になるだけ)。無効の組は除く。
      const probe = stage2Vector(`${label}/${slug}/${a}/${d}/probe`, a, d, moveName, {});
      if (!hasDamage(probe)) continue;
      addStage2(`${label}/${slug}/${a}/${d}/plain`, a, d, moveName, {});
      addStage2(`${label}/${slug}/${a}/${d}/atk-spe-plus1`, a, d, moveName, {a:{ranks:{spe:1}}});
      addStage2(`${label}/${slug}/${a}/${d}/atk-spe-minus2`, a, d, moveName, {a:{ranks:{spe:-2}}});
      addStage2(`${label}/${slug}/${a}/${d}/def-spe-minus1`, a, d, moveName, {d:{ranks:{spe:-1}}});
      addStage2(`${label}/${slug}/${a}/${d}/def-spe-plus6`, a, d, moveName, {d:{ranks:{spe:6}}});
      addStage2(`${label}/${slug}/${a}/${d}/atk-max-spe-sp`, a, d, moveName, {a:{sp:{spe:32}, nature:'Jolly'}, d:{sp:{spe:0}, nature:'Brave'}});
      addStage2(`${label}/${slug}/${a}/${d}/crit-reflect`, a, d, moveName, {critical:true, screen:'Reflect'});
    }
  };
  ball('Electro Ball', 'electroball', 'speed-ratio');
  ball('Gyro Ball', 'gyroball', 'speed-inverse');
  for (const [moveName, slug, pair] of [['Electro Ball','electroball',['Jolteon','Snorlax']],['Gyro Ball','gyroball',['Snorlax','Weavile']],['Gyro Ball','gyroball',['Scizor','Garchomp']]]) {
    const [a, d] = pair;
    const plain = addStage2(`speed-paralysis/${slug}/${a}/${d}/none`, a, d, moveName, {});
    if (!hasDamage(plain)) continue;
    trackChange('speed-paralysis', compareStage2(`speed-paralysis/${slug}/${a}/${d}/atk-par`, a, d, moveName, {a:{status:'par'}}, plain, 'any'), plain);
    trackChange('speed-paralysis', compareStage2(`speed-paralysis/${slug}/${a}/${d}/def-par`, a, d, moveName, {d:{status:'par'}}, plain, 'any'), plain);
    addStage2(`speed-paralysis/${slug}/${a}/${d}/both-par`, a, d, moveName, {a:{status:'par'}, d:{status:'par'}});
    addStage2(`speed-paralysis/${slug}/${a}/${d}/atk-par-spe-plus2`, a, d, moveName, {a:{status:'par', ranks:{spe:2}}});
    // まひの半減は切り捨て(素早さが奇数になる組)。
    addStage2(`speed-paralysis/${slug}/${a}/${d}/atk-par-odd`, a, d, moveName, {a:{status:'par', sp:{spe:1}}, d:{sp:{spe:3}}});
  }
  assert.equal(changedGroups['speed-paralysis'], true, 'speed-paralysis: まひで oracle のダメージが変わった組が1つも無い');
  // 素早さの補正(特性): ようりょくそ・すいすい・すなかき・ゆきかき・サーフテール(エレキフィールド)・はやあし(状態異常)・かるわざ(item_lost は入力に無いので効かない)。
  const speedAbilityCases = [
    ['Chlorophyll', {weather:'sun'}, true], ['Swift Swim', {weather:'rain'}, true], ['Sand Rush', {weather:'sand'}, true],
    ['Slush Rush', {weather:'snow'}, true], ['Surge Surfer', {terrain:'electric'}, true],
  ];
  for (const [moveName, slug, a, d] of [['Gyro Ball','gyroball','Scizor','Garchomp'],['Gyro Ball','gyroball','Snorlax','Weavile'],['Electro Ball','electroball','Pikachu','Tyranitar'],['Electro Ball','electroball','Snorlax','Weavile']]) {
    for (const [ability, cond] of speedAbilityCases) {
      const slug2 = slugOf(ability);
      // 攻撃側が持つ。条件が成立する場(on)と成立しない場(off)。
      const onBase = stage2Vector(`speed-ability/${slug}/${a}/${d}/${slug2}/on-base`, a, d, moveName, cond);
      const on = compareStage2(`speed-ability/${slug}/${a}/${d}/${slug2}/on`, a, d, moveName, {...cond, a:{ability}}, onBase, 'any');
      trackChange(`speed-ability/${slug2}`, on, onBase);
      const offBase = stage2Vector(`speed-ability/${slug}/${a}/${d}/${slug2}/off-base`, a, d, moveName, {});
      compareStage2(`speed-ability/${slug}/${a}/${d}/${slug2}/off`, a, d, moveName, {a:{ability}}, offBase, false);
      // 防御側が持つ。
      const defBase = stage2Vector(`speed-ability/${slug}/${a}/${d}/${slug2}/defender-base`, a, d, moveName, cond);
      const def = compareStage2(`speed-ability/${slug}/${a}/${d}/${slug2}/defender`, a, d, moveName, {...cond, d:{ability}}, defBase, 'any');
      trackChange(`speed-ability/${slug2}`, def, defBase);
    }
    // はやあし: 状態異常で ×1.5・まひの半減を受けない。
    const qf = 'Quick Feet';
    const parBase = stage2Vector(`speed-ability/${slug}/${a}/${d}/quickfeet/par-base`, a, d, moveName, {a:{status:'par'}});
    trackChange('speed-ability/quickfeet', compareStage2(`speed-ability/${slug}/${a}/${d}/quickfeet/par`, a, d, moveName, {a:{ability:qf, status:'par'}}, parBase, 'any'), parBase);
    const brnBase = stage2Vector(`speed-ability/${slug}/${a}/${d}/quickfeet/brn-base`, a, d, moveName, {a:{status:'brn'}});
    trackChange('speed-ability/quickfeet', compareStage2(`speed-ability/${slug}/${a}/${d}/quickfeet/brn`, a, d, moveName, {a:{ability:qf, status:'brn'}}, brnBase, 'any'), brnBase);
    const defPsnBase = stage2Vector(`speed-ability/${slug}/${a}/${d}/quickfeet/def-psn-base`, a, d, moveName, {d:{status:'psn'}});
    trackChange('speed-ability/quickfeet', compareStage2(`speed-ability/${slug}/${a}/${d}/quickfeet/def-psn`, a, d, moveName, {d:{ability:qf, status:'psn'}}, defPsnBase, 'any'), defPsnBase);
    addStage2(`speed-ability/${slug}/${a}/${d}/quickfeet/none`, a, d, moveName, {a:{ability:qf}});
    // かるわざ: 入力に「持ち物を失った」状態が無いので素早さは変わらない(oracle も既定は効かない)。
    compareStage2(`speed-ability/${slug}/${a}/${d}/unburden`, a, d, moveName, {a:{ability:'Unburden'}}, stage2Vector(`speed-ability/${slug}/${a}/${d}/unburden-base`, a, d, moveName, {}), false);
    // くろいてっきゅう: 素早さ半減。
    const base = stage2Vector(`speed-item/${slug}/${a}/${d}/base`, a, d, moveName, {});
    trackChange('speed-item/ironball', compareStage2(`speed-item/${slug}/${a}/${d}/ironball`, a, d, moveName, {a:{item:'Iron Ball'}}, base, 'any'), base);
    addStage2(`speed-item/${slug}/${a}/${d}/ironball-par-swiftswim`, a, d, moveName, {weather:'rain', a:{item:'Iron Ball', ability:'Swift Swim', status:'par'}});
    addStage2(`speed-item/${slug}/${a}/${d}/ironball-ranks`, a, d, moveName, {a:{item:'Iron Ball', ranks:{spe:3}}});
  }
  for (const group of ['speed-ability/chlorophyll','speed-ability/swiftswim','speed-ability/sandrush','speed-ability/slushrush','speed-ability/surgesurfer','speed-ability/quickfeet','speed-item/ironball']) {
    assert.equal(changedGroups[group], true, `${group}: 素早さの補正で oracle のダメージが変わった組が1つも無い(組を選び直す)`);
  }
}
// 2タイプの相性(フライングプレス型)・特定タイプへの相性(フリーズドライ型)。
{
  const defenders = ['Garchomp','Venusaur','Gengar','Corviknight','Azumarill','Gyarados','Charizard','Snorlax','Scizor','Tyranitar','Blastoise','Typhlosion','Abomasnow'];
  for (const d of defenders) {
    addStage2(`effectiveness/flyingpress/Lucario/${d}`, 'Lucario', d, 'Flying Press', {});
    addStage2(`effectiveness/freezedry/Weavile/${d}`, 'Weavile', d, 'Freeze-Dry', {});
  }
  addStage2('effectiveness/flyingpress/Lucario/Venusaur/crit-reflect', 'Lucario', 'Venusaur', 'Flying Press', {critical:true, screen:'Reflect'});
  addStage2('effectiveness/freezedry/Weavile/Gyarados/light-screen', 'Weavile', 'Gyarados', 'Freeze-Dry', {screen:'LightScreen'});
  addStage2('effectiveness/freezedry/Weavile/Blastoise/life-orb', 'Weavile', 'Blastoise', 'Freeze-Dry', {a:{item:'Life Orb'}});
  addStage2('effectiveness/flyingpress/Lucario/Azumarill/life-orb', 'Lucario', 'Azumarill', 'Flying Press', {a:{item:'Life Orb'}});
}
// 壁を壊す: かわらわり型・サイコファング型は防御側の壁を外してから計算する。
{
  for (const [moveName, a, d] of [['Brick Break','Lucario','Snorlax'],['Brick Break','Tyranitar','Garchomp'],['Psychic Fangs','Gardevoir','Snorlax'],['Psychic Fangs','Gengar','Garchomp']]) {
    const slug = slugOf(moveName);
    const none = addStage2(`screens-broken/${slug}/${a}/${d}/none`, a, d, moveName, {});
    for (const screen of ['Reflect','AuroraVeil','LightScreen']) {
      compareStage2(`screens-broken/${slug}/${a}/${d}/${id(screen)}`, a, d, moveName, {screen}, none, false);
    }
    compareStage2(`screens-broken/${slug}/${a}/${d}/reflect-double`, a, d, moveName, {screen:'Reflect', format:'double'},
      stage2Vector(`screens-broken/${slug}/${a}/${d}/double-none`, a, d, moveName, {format:'double'}), false);
    addStage2(`screens-broken/${slug}/${a}/${d}/reflect-crit`, a, d, moveName, {screen:'Reflect', critical:true});
  }
}
// 何発目か: トリプルアクセル型は h 発目の威力が 威力 × h。特性の条件(テクニシャン)は発ごとの威力で判定する。
{
  for (const [a, d] of [['Weavile','Garchomp'],['Weavile','Snorlax'],['Gardevoir','Tyranitar']]) {
    for (const [vl, o] of [['plain', {}], ['technician', {a:{ability:'Technician'}}], ['crit', {critical:true}], ['reflect', {screen:'Reflect'}],
      ['life-orb', {a:{item:'Life Orb'}}], ['burn', {a:{status:'brn'}}], ['sun', {weather:'sun'}], ['ranks', {a:{ranks:{atk:2}}, d:{ranks:{def:-1}}}]]) {
      const v = addStage2(`hit-index/tripleaxel/${a}/${d}/${vl}`, a, d, 'Triple Axel', o);
      assert.equal(v.expected.hitRolls.length, 3, `hit-index/${a}/${d}/${vl}: 3 発でない`);
    }
  }
}
// ハンドラがダメージに効かない技(命中・味方への処理・天候による命中・眠りを覚ます等)は通常の式と同じ。
{
  for (const [moveName, pairs] of [['Blizzard', [['Weavile','Garchomp'],['Abomasnow','Snorlax']]], ['Hurricane', [['Charizard','Venusaur'],['Corviknight','Garchomp']]],
    ['Thunder', [['Jolteon','Blastoise'],['Pikachu','Snorlax']]], ['Pollen Puff', [['Scizor','Gardevoir'],['Venusaur','Snorlax']]], ['Uproar', [['Snorlax','Garchomp'],['Gardevoir','Tyranitar']]]]) {
    for (const [a, d] of pairs) {
      const slug = slugOf(moveName);
      addStage2(`resolved/${slug}/${a}/${d}/plain`, a, d, moveName, {});
      addStage2(`resolved/${slug}/${a}/${d}/crit-reflect`, a, d, moveName, {critical:true, screen:'LightScreen'});
      addStage2(`resolved/${slug}/${a}/${d}/rain`, a, d, moveName, {weather:'rain'});
      addStage2(`resolved/${slug}/${a}/${d}/double`, a, d, moveName, {format:'double'});
    }
  }
}
// 重さ: けたぐり型(防御側の重さの表)・ヘビーボンバー型(攻撃側 / 防御側の比。整数で比べる)・特性の重さの補正。
// 重さの比がちょうど整数(2〜5)になる組・浮動小数で整数の比較とずれる組は使わない(oracle は kg の浮動小数で割る。ADR-0143)。
const weightHgOf = (name, ability = '') => {
  const w = Math.round(genC.species.get(id(name)).weightkg * 10);
  if (ability === 'Heavy Metal') return Math.max(Math.trunc(w * 2), 1);
  if (ability === 'Light Metal') return Math.max(Math.trunc(w * 0.5), 1);
  return w;
};
function ratioSafe(attackerHg, defenderHg) {
  for (let k = 2; k <= 5; k++) {
    if (attackerHg === defenderHg * k) return false;
    if ((attackerHg / defenderHg >= k) !== (attackerHg >= defenderHg * k)) return false;
  }
  return true;
}
{
  const targets = ['Rotom-Wash','Pikachu','Toxapex','Azumarill','Clefable','Excadrill','Gardevoir','Lucario','Typhlosion','Blastoise','Charizard','Garchomp','Venusaur','Scizor','Goodra','Tyranitar','Dragonite','Gyarados','Snorlax','Metagross'];
  for (const [moveName, a] of [['Low Kick','Lucario'],['Grass Knot','Venusaur']]) {
    const slug = slugOf(moveName);
    for (const d of targets) {
      if (d === a) continue;
      const v = addStage2(`weight-target/${slug}/${a}/${d}`, a, d, moveName, {});
      if (!hasDamage(v)) stage2Fixed.pop(); // 無効になる組は照合にならないので足さない
    }
    addStage2(`weight-target/${slug}/${a}/Garchomp/crit-reflect`, a, 'Garchomp', moveName, {critical:true, screen:moveName === 'Low Kick' ? 'Reflect' : 'LightScreen'});
    addStage2(`weight-target/${slug}/${a}/Tyranitar/life-orb`, a, 'Tyranitar', moveName, {a:{item:'Life Orb'}});
  }
  // 比の境界を避けながら、段(40・60・80・100・120)をまたぐ組を集める。
  const heavy = ['Metagross','Snorlax','Aggron','Tyranitar','Scizor','Gyarados'];
  const ratioTargets = ['Rotom-Wash','Pikachu','Toxapex','Azumarill','Gengar','Gardevoir','Lucario','Typhlosion','Garchomp','Goodra','Snorlax','Aggron'];
  for (const [moveName, attackers] of [['Heavy Slam', heavy], ['Heat Crash', ['Typhlosion','Charizard','Goodra','Snorlax','Aggron']]]) {
    const slug = slugOf(moveName);
    const seenBands = new Set();
    for (const a of attackers) for (const d of ratioTargets) {
      if (a === d) continue;
      const aw = weightHgOf(a), dw = weightHgOf(d);
      if (!ratioSafe(aw, dw)) continue;
      const band = `${a}:${aw >= dw * 5 ? 5 : aw >= dw * 4 ? 4 : aw >= dw * 3 ? 3 : aw >= dw * 2 ? 2 : 1}`;
      if (seenBands.has(band)) continue; // 攻撃側ごとに、表の段(5以上・4・3・2・それ未満)につき1組
      seenBands.add(band);
      const v = addStage2(`weight-ratio/${slug}/${a}/${d}`, a, d, moveName, {});
      if (!hasDamage(v)) stage2Fixed.pop();
    }
    addStage2(`weight-ratio/${slug}/${attackers[0]}/Pikachu/crit-reflect`, attackers[0], 'Pikachu', moveName, {critical:true, screen:'Reflect'});
    addStage2(`weight-ratio/${slug}/${attackers[0]}/Pikachu/life-orb`, attackers[0], 'Pikachu', moveName, {a:{item:'Life Orb'}});
  }
  // ヘヴィメタル(×2)・ライトメタル(×0.5)。段をまたぐ組を選ぶ。かたやぶりで防御側の補正を無視する(Breakable)。
  const weightAbilityCases = [
    // 防御側が持つ(けたぐり型): Garchomp 95.0kg → 190.0kg(80 → 100)・47.5kg(80 → 60)。
    {kind:'target', moveName:'Low Kick', a:'Lucario', d:'Garchomp', holder:'d'},
    {kind:'target', moveName:'Grass Knot', a:'Venusaur', d:'Typhlosion', holder:'d'},
    // 攻撃側が持つ(ヘビーボンバー型)・防御側が持つ(比が変わる)。
    {kind:'ratio', moveName:'Heavy Slam', a:'Scizor', d:'Garchomp', holder:'a'},
    {kind:'ratio', moveName:'Heavy Slam', a:'Scizor', d:'Garchomp', holder:'d'},
    {kind:'ratio', moveName:'Heat Crash', a:'Goodra', d:'Gardevoir', holder:'a'},
    {kind:'ratio', moveName:'Heat Crash', a:'Goodra', d:'Gardevoir', holder:'d'},
  ];
  for (const abilityName of Object.keys(effects.abilities).filter(n => effects.abilities[n].WeightMod).sort()) {
    const def = effects.abilities[abilityName];
    const slug = id(abilityName);
    for (const c of weightAbilityCases) {
      const holderName = c[c.holder];
      const withAbility = {[c.holder]:{ability:abilityName}};
      const label = `${c.kind === 'target' ? 'weight-target' : 'weight-ratio'}/${slugOf(c.moveName)}/${c.a}/${c.d}`;
      const aw = c.holder === 'a' ? weightHgOf(c.a, abilityName) : weightHgOf(c.a);
      const dw = c.holder === 'd' ? weightHgOf(c.d, abilityName) : weightHgOf(c.d);
      if (c.kind === 'ratio') {
        assert(ratioSafe(aw, dw) && ratioSafe(weightHgOf(c.a), weightHgOf(c.d)), `${abilityName} ${label}: 重さの比が境界にある(組を選び直す)`);
      }
      const plain = stage2Vector(`${label}/base`, c.a, c.d, c.moveName, {});
      const apply = addStage2(`weight-ability/${slug}/${slugOf(c.moveName)}/${c.a}/${c.d}/${holderName}`, c.a, c.d, c.moveName, withAbility);
      trackChange(`weight-ability/${slug}/${c.kind}`, apply, plain);
    }
    for (const kind of ['target','ratio']) {
      assert.equal(changedGroups[`weight-ability/${slug}/${kind}`], true, `weight-ability/${slug}/${kind}: 重さの補正で oracle のダメージが変わった組が1つも無い(組を選び直す)`);
    }
    // effects/<id>/weight/apply・control・breakable(ADR-0143 §7)。
    const [moveName, a, d] = ['Low Kick', 'Lucario', 'Garchomp']; // Garchomp 95.0kg: ×2 で 100(80 → 100)・×0.5 で 47.5(80 → 60)
    const base = stage2Vector(`effects/${slug}/weight/base`, a, d, moveName, {});
    compareStage2(`effects/${slug}/weight/apply`, a, d, moveName, {d:{ability:abilityName}}, base);
    // 攻撃側が持っても防御側の重さは変わらない(けたぐり型)。
    compareStage2(`effects/${slug}/weight/control`, a, d, moveName, {a:{ability:abilityName}}, base, false);
    if (def.Breakable) {
      const withIgnorer = {a:{ability:ignorer}};
      compareStage2(`effects/${slug}/weight/breakable`, a, d, moveName, {a:{ability:ignorer}, d:{ability:abilityName}},
        stage2Vector(`effects/${slug}/weight/breakable-base`, a, d, moveName, withIgnorer), false);
    }
  }
  // 比の式の整数比較: 攻撃側 / 防御側が整数倍に近い組(境界を避けた近傍)で、段の境目に近い値を確かめる。
  for (const [a, d, moveName] of [['Metagross','Garchomp','Heavy Slam'],['Snorlax','Typhlosion','Heavy Slam'],['Aggron','Blastoise','Heavy Slam']]) {
    if (ratioSafe(weightHgOf(a), weightHgOf(d))) addStage2(`weight-ratio/${slugOf(moveName)}/${a}/${d}/near-boundary`, a, d, moveName, {});
  }
}

// 生成器の保証(ADR-0143 §7): (a) moveRules の全技に1件以上のベクタ。(b) 重さの比のベクタは境界を避ける。
{
  const missing = Object.keys(stage2Rules).filter(n => !stage2CoveredMoves.has(n) && !isStage3Rule(stage2Rules[n]));
  assert.deepEqual(missing, [], `moveRules の技にベクタが無い: ${missing.join(', ')}`);
  for (const v of stage2Fixed) {
    const rule = v.input.Move.Rule;
    if (rule.PowerFormula !== 'weight_ratio') continue;
    const wa = effectiveWeightHg(v.input.Attacker), wd = effectiveWeightHg(v.input.Defender);
    assert(ratioSafe(wa, wd), `${v.id}: 重さの比が境界にある(攻撃側 ${wa} / 防御側 ${wd})`);
  }
  const ids = new Set();
  for (const v of stage2Fixed) {
    assert(!ids.has(v.id), `ベクタの id が重複: ${v.id}`);
    ids.add(v.id);
    assert(v.input.Attacker.Species.WeightHg > 0 && v.input.Defender.Species.WeightHg > 0, `${v.id}: 種族の重さが入力に無い`);
  }
}
function effectiveWeightHg(side) {
  const w = side.Species.WeightHg, mod = side.Ability.Effect && side.Ability.Effect.WeightMod;
  return mod ? Math.max(1, Math.trunc(w * mod / 4096)) : w;
}

// --- 技の機構の段階3(ADR-0144 §7): mechanisms-stage3.json ---------------------------------------------
// 対戦の状態(残り HP・多段の回数)・なげつける(持ち物の威力)・分類の切り替え・持ち物による接地を oracle(Champions 世代)と照合する。
// 残り HP は oracle の Pokemon の curHP(1..最大)、回数は Move の hits を、入力の battleState(DamageInput.State)に同じ値で載せる。
// 技の処理の定義は effects.json の moveRules(段階2と同じく Move.Rule に載せる)。定義を持たない技(hp-ko・hits・grounded-item)は
// Move.Rule を持たない。いかりのまえば型(Super Fang)・がむしゃら型(Endeavor)は oracle が計算しない(威力 0 として 0 を返す)ので
// 入れない(engine の単体テストが Showdown の規則を見る)。既存のファイルのバイト列・乱数列は変えない(新しいファイルだけ。乱数も使わない)。
const stage3Fixed = [];
const stage3CoveredMoves = new Set();
const oracleLacksFixedDamage = r => r.FixedDamageFormula === 'defender_current_hp_half' || r.FixedDamageFormula === 'defender_minus_attacker_hp';
const stage3RuleMoves = Object.keys(stage2Rules).filter(n => isStage3Rule(stage2Rules[n]));
assert.equal(stage3RuleMoves.length, 10, `段階3の語彙を使う moveRules の技は 10 技のはず: ${stage3RuleMoves.join(', ')}`);
const maxHpOf = (name, o = {}) => individual(genC, name, o).p.rawStats.hp;
function stage3Vector(label, a, d, moveName, options = {}) {
  const rule = stage2Rules[moveName] || null;
  if (rule) assert(isStage3Rule(rule), `${label}: ${moveName} は段階3の語彙の定義でない`);
  const attack = individual(genC, a, {...options.a, stage2:'a'}), defend = individual(genC, d, {...options.d, stage2:'d'});
  const weather = options.weather || 'none', terrain = options.terrain || 'none', screen = options.screen;
  const m = new Move(genC, moveName, {isCrit:!!options.critical, ability:attack.p.ability || undefined, ...(options.hits ? {hits:options.hits} : {})});
  const data = {type:m.type, category:m.category, bp:m.bp, priority:m.priority, hits:m.hits || 1, multihit:genC.moves.get(id(moveName)).multihit};
  const f = new Field({gameType:'Singles',weather:weatherNames[weather],terrain:terrainNames[terrain],
    defenderSide:{isReflect:screen==='Reflect',isLightScreen:screen==='LightScreen',isAuroraVeil:screen==='AuroraVeil'}});
  const result = calculate(genC, attack.p, defend.p, m, f);
  const matrix = typeof result.damage === 'number' ? [Array(16).fill(result.damage)]
    : Array.isArray(result.damage[0]) ? result.damage : [result.damage];
  assert(matrix.every(r => r.length === 16 && r.every(Number.isInteger)), `${label}: oracle のダメージが16段階の整数でない`);
  assert.equal(matrix.length, data.hits, `${label}: 回数が oracle の Move と違う`);
  const rolls = matrix[0].map((_, i) => matrix.reduce((s, r) => s + r[i], 0));
  const move = {ID:id(moveName),Type:data.type.toLowerCase(),Category:data.category.toLowerCase(),Power:data.bp,Priority:data.priority};
  if (rule) {
    move.Mechanisms = stage2Mechanisms(rule);
    move.Rule = rule;
  } else if (data.multihit) {
    // 定義を持たない多段技: 段階1の機構と中身(回数の範囲)。
    const mech = mechanismsOf(moveName);
    move.Mechanisms = mech.mechanisms;
    if (Object.keys(mech.params).length) move.mechanismParams = mech.params;
  }
  // 対戦の状態(残り HP は 1..最大。省略は満タン。回数は範囲の多段技だけ)。
  const state = {};
  if (options.a?.curHP) state.AttackerCurrentHP = options.a.curHP;
  if (options.d?.curHP) state.DefenderCurrentHP = options.d.curHP;
  if (options.hits) {
    assert(Array.isArray(data.multihit) && options.hits >= data.multihit[0] && options.hits <= data.multihit[1], `${label}: 回数 ${options.hits} が範囲の多段の外`);
    state.Hits = options.hits;
  }
  const input = {Format:'single',Attacker:attack.input,Defender:defend.input,Move:move,
    Field:{Weather:weather,Terrain:terrain,DefenderScreens:{Reflect:screen==='Reflect',LightScreen:screen==='LightScreen',AuroraVeil:screen==='AuroraVeil'}},Critical:!!options.critical};
  if (Object.keys(state).length) input.battleState = state;
  const hp = defend.p.curHP();
  const expected = {rolls,attackerStats:attack.p.rawStats,defenderStats:defend.p.rawStats,ko:matrix.length > 1 ? koUses(matrix,hp) : ko(rolls,hp)};
  if (matrix.length > 1) expected.hitRolls = matrix;
  return {id:label, oracle:{attacker:a,defender:d,move:moveName}, input, expected, finalCategory:result.move.category.toLowerCase()};
}
// ファイルに書くときは検査用の finalCategory を外す。
const stage3Out = v => { const {finalCategory, ...rest} = v; return rest; };
const addStage3 = (label, a, d, moveName, options) => {
  const v = stage3Vector(label, a, d, moveName, options);
  stage3Fixed.push(v);
  stage3CoveredMoves.add(moveName);
  return v;
};
const rollsKey = v => JSON.stringify(v.expected.rolls);
const stage3Hp = v => v.expected.rolls.some(r => r > 0);

// 残り HP: 候補(最大 HP に対する 1..最大の値。重複は除く)。
const uniqSorted = xs => [...new Set(xs)].sort((x, y) => x - y);
const hpLevels = max => uniqSorted([max, max - 1, Math.floor(max * 3 / 4), Math.floor(max / 2), Math.floor(max / 3), Math.floor(max / 10), 3, 2, 1].filter(v => v >= 1 && v <= max));

// 攻撃側の残り HP の割合の威力(ふんか・しおふき型): 威力 × 残り / 最大 の切り捨て(最小 1)。
{
  for (const [moveName, pairs] of [['Eruption', [['Typhlosion','Snorlax'],['Charizard','Garchomp']]], ['Water Spout', [['Blastoise','Garchomp'],['Gyarados','Typhlosion']]]]) {
    for (const [a, d] of pairs) {
      const max = maxHpOf(a), slug = slugOf(moveName);
      // 切り上げ・切り捨ての境目を含める: max × k / 150 の前後と、最大・最小。
      const levels = uniqSorted([...hpLevels(max), Math.ceil(max / 150), Math.ceil(max * 2 / 150), Math.floor(max * 149 / 150), Math.floor(max * 75 / 150) + 1].filter(v => v >= 1 && v <= max));
      for (const cur of levels) addStage3(`hp-attacker-scaled/${slug}/${a}/${d}/cur${cur}`, a, d, moveName, {a:{curHP:cur}});
      const half = Math.floor(max / 2);
      addStage3(`hp-attacker-scaled/${slug}/${a}/${d}/half-crit`, a, d, moveName, {a:{curHP:half}, critical:true});
      addStage3(`hp-attacker-scaled/${slug}/${a}/${d}/half-reflect`, a, d, moveName, {a:{curHP:half}, screen:'LightScreen'});
      addStage3(`hp-attacker-scaled/${slug}/${a}/${d}/half-life-orb`, a, d, moveName, {a:{curHP:half, item:'Life Orb'}});
      addStage3(`hp-attacker-scaled/${slug}/${a}/${d}/half-technician`, a, d, moveName, {a:{curHP:half, ability:'Technician'}});
      // 防御側の残り HP は威力に効かない(確定数だけ)。
      const full = stage3Vector(`hp-attacker-scaled/${slug}/${a}/${d}/def-ref`, a, d, moveName, {a:{curHP:half}});
      const defLow = addStage3(`hp-attacker-scaled/${slug}/${a}/${d}/defender-low`, a, d, moveName, {a:{curHP:half}, d:{curHP:Math.floor(maxHpOf(d) / 3)}});
      assert.equal(rollsKey(defLow), rollsKey(full), `hp-attacker-scaled/${slug}/${a}/${d}: 防御側の残り HP でダメージが変わった`);
    }
  }
}
// 攻撃側の残り HP の 48 分率の威力(きしかいせい・じたばた型): p = floor(48 × 残り / 最大) の段(≤1:200 ≤4:150 ≤9:100 ≤16:80 ≤32:40 else 20)。
{
  const bandLevels = max => {
    const out = [max, 1];
    for (const p of [0, 1, 2, 4, 5, 9, 10, 16, 17, 32, 33, 48]) {
      const lo = Math.max(1, Math.ceil(p * max / 48)); // p 以上になる最小
      if (Math.floor(48 * lo / max) === p) out.push(lo);
      const hi = Math.ceil((p + 1) * max / 48) - 1;    // p のままの最大
      if (hi >= 1 && hi <= max && Math.floor(48 * hi / max) === p) out.push(hi);
    }
    return uniqSorted(out.filter(v => v >= 1 && v <= max));
  };
  for (const [moveName, pairs] of [['Flail', [['Snorlax','Garchomp'],['Tyranitar','Gardevoir']]], ['Reversal', [['Lucario','Snorlax'],['Garchomp','Tyranitar']]]]) {
    for (const [a, d] of pairs) {
      const max = maxHpOf(a), slug = slugOf(moveName);
      for (const cur of bandLevels(max)) addStage3(`hp-attacker-low/${slug}/${a}/${d}/cur${cur}`, a, d, moveName, {a:{curHP:cur}});
      assert(bandLevels(max).length >= 6, `hp-attacker-low/${slug}/${a}: 段をまたぐ残り HP が少ない`);
      const mid = bandLevels(max)[Math.floor(bandLevels(max).length / 2)];
      addStage3(`hp-attacker-low/${slug}/${a}/${d}/mid-crit-reflect`, a, d, moveName, {a:{curHP:mid}, critical:true, screen:'Reflect'});
      addStage3(`hp-attacker-low/${slug}/${a}/${d}/mid-life-orb`, a, d, moveName, {a:{curHP:mid, item:'Life Orb'}});
    }
  }
}
// 防御側の残り HP の割合の威力(ハードプレス型): 100 × floor(残り × 4096 / 最大) の 4096 の丸め。
{
  for (const [a, d] of [['Scizor','Snorlax'],['Metagross','Garchomp'],['Scizor','Tyranitar']]) {
    const max = maxHpOf(d), slug = 'hardpress';
    for (const cur of uniqSorted([...hpLevels(max), Math.ceil(max / 100), Math.floor(max * 99 / 100)].filter(v => v >= 1 && v <= max))) {
      addStage3(`hp-defender-ratio/${slug}/${a}/${d}/cur${cur}`, a, d, 'Hard Press', {d:{curHP:cur}});
    }
    const half = Math.floor(max / 2);
    addStage3(`hp-defender-ratio/${slug}/${a}/${d}/half-crit`, a, d, 'Hard Press', {d:{curHP:half}, critical:true});
    addStage3(`hp-defender-ratio/${slug}/${a}/${d}/half-reflect`, a, d, 'Hard Press', {d:{curHP:half}, screen:'Reflect'});
    // 攻撃側の残り HP は威力に効かない。
    const ref = stage3Vector(`hp-defender-ratio/${slug}/${a}/${d}/atk-ref`, a, d, 'Hard Press', {d:{curHP:half}});
    const atkLow = addStage3(`hp-defender-ratio/${slug}/${a}/${d}/attacker-low`, a, d, 'Hard Press', {d:{curHP:half}, a:{curHP:Math.floor(maxHpOf(a) / 3)}});
    assert.equal(rollsKey(atkLow), rollsKey(ref), `hp-defender-ratio/${slug}/${a}/${d}: 攻撃側の残り HP でダメージが変わった`);
  }
}
// 攻撃側の残り HP と同じダメージ(いのちがけ型)。タイプ相性の無効(ゴーストにかくとう)は先。
{
  for (const [a, d] of [['Lucario','Snorlax'],['Gardevoir','Garchomp'],['Snorlax','Tyranitar']]) {
    const max = maxHpOf(a);
    for (const cur of hpLevels(max)) {
      const v = addStage3(`hp-fixed/finalgambit/${a}/${d}/cur${cur}`, a, d, 'Final Gambit', {a:{curHP:cur}});
      assert(v.expected.rolls.every(r => r === cur), `hp-fixed/${a}/${d}/cur${cur}: oracle のダメージが残り HP と違う`);
    }
    // 一致・壁・急所・持ち物・天候・ランクで変わらない固定ダメージ。
    const cur = Math.floor(max / 2);
    addStage3(`hp-fixed/finalgambit/${a}/${d}/stab-crit-reflect`, a, d, 'Final Gambit', {a:{curHP:cur, item:'Life Orb', ranks:{spa:2}}, d:{ranks:{spd:3}}, critical:true, screen:'LightScreen', weather:'sun'});
  }
  for (const a of ['Lucario','Snorlax']) {
    const v = addStage3(`hp-fixed/finalgambit/${a}/Gengar/immune`, a, 'Gengar', 'Final Gambit', {a:{curHP:Math.floor(maxHpOf(a) / 2)}});
    assert(v.expected.rolls.every(r => r === 0), `hp-fixed/${a}/Gengar: oracle が無効になっていない`);
  }
}
// 防御側の残り HP は確定数に効く(定義を持たない通常の技・多段技)。ダメージの16段階は満タンと同じ。
{
  const hpKoPairs = [['Garchomp','Snorlax'],['Tyranitar','Gardevoir'],['Scizor','Blastoise']];
  for (const [a, d] of hpKoPairs) {
    const max = maxHpOf(d), full = stage3Vector(`hp-ko/dragonclaw/${a}/${d}/full-ref`, a, d, 'Dragon Claw', {});
    const dmg = full.expected.rolls[15];
    // 倒す回数が変わる残り HP: 1 発で倒せる・2 発・乱数を含む・満タン。
    const levels = uniqSorted([1, Math.max(1, dmg - 1), dmg, dmg + 1, Math.floor(full.expected.rolls[0] * 1.5), Math.floor(dmg * 2) - 1, Math.floor(dmg * 2) + 5, Math.floor(max / 2), max].filter(v => v >= 1 && v <= max));
    for (const cur of levels) {
      const v = addStage3(`hp-ko/dragonclaw/${a}/${d}/cur${cur}`, a, d, 'Dragon Claw', {d:{curHP:cur}});
      assert.equal(rollsKey(v), rollsKey(full), `hp-ko/${a}/${d}/cur${cur}: 残り HP でダメージの16段階が変わった`);
    }
    addStage3(`hp-ko/dragonclaw/${a}/${d}/crit-low`, a, d, 'Dragon Claw', {d:{curHP:Math.floor(max / 3)}, critical:true});
  }
  for (const [a, d] of [['Snorlax','Corviknight'],['Garchomp','Snorlax']]) {
    const max = maxHpOf(d), full = stage3Vector(`hp-ko/bulletseed/${a}/${d}/full-ref`, a, d, 'Bullet Seed', {});
    const dmg = full.expected.rolls[15];
    for (const cur of uniqSorted([1, Math.max(1, dmg - 1), dmg, dmg + 1, Math.floor(dmg * 1.5), Math.floor(max / 2), max].filter(v => v >= 1 && v <= max))) {
      addStage3(`hp-ko/bulletseed/${a}/${d}/cur${cur}`, a, d, 'Bullet Seed', {d:{curHP:cur}});
    }
  }
}
// 多段の回数の指定(範囲の多段技の最小・最大・途中。スキルリンクがあっても指定が勝つ)。
{
  for (const [moveName, pairs] of [['Bullet Seed', [['Snorlax','Corviknight'],['Garchomp','Snorlax']]], ['Icicle Spear', [['Weavile','Garchomp']]],
    ['Rock Blast', [['Tyranitar','Charizard']]], ['Water Shuriken', [['Blastoise','Garchomp']]]]) {
    for (const [a, d] of pairs) {
      const slug = slugOf(moveName), [min, max] = genC.moves.get(id(moveName)).multihit;
      const rolls = {};
      for (let n = min; n <= max; n++) rolls[n] = rollsKey(addStage3(`hits/${slug}/${a}/${d}/n${n}`, a, d, moveName, {hits:n}));
      assert.equal(new Set(Object.values(rolls)).size, max - min + 1, `hits/${slug}/${a}/${d}: 回数でダメージが変わらない`);
      addStage3(`hits/${slug}/${a}/${d}/max-crit-reflect`, a, d, moveName, {hits:max, critical:true, screen:'Reflect'});
      addStage3(`hits/${slug}/${a}/${d}/min-life-orb`, a, d, moveName, {hits:min, a:{item:'Life Orb'}});
      // スキルリンクより指定が勝つ(oracle の options.hits)。
      const linkMin = addStage3(`hits/${slug}/${a}/${d}/skill-link-min`, a, d, moveName, {hits:min, a:{ability:'Skill Link'}});
      assert.equal(linkMin.expected.hitRolls.length, min, `hits/${slug}: スキルリンクがあっても指定の回数にならない`);
      addStage3(`hits/${slug}/${a}/${d}/skill-link-max`, a, d, moveName, {hits:max, a:{ability:'Skill Link'}});
      // 回数と防御側の残り HP の組み合わせ(確定数は残りで数える)。
      addStage3(`hits/${slug}/${a}/${d}/n3-defender-low`, a, d, moveName, {hits:Math.min(max, 3), d:{curHP:Math.floor(maxHpOf(d) / 2)}});
    }
  }
}
// なげつける型: 攻撃側の持ち物の威力(oracle の getFlingPower)。持ち物の他の効果は計算中も残る。持ち物なしはダメージ 0。
{
  const noEffectItems = ['Leftovers','King\'s Rock','Quick Claw','Sitrus Berry','Focus Sash','Lum Berry']
    .filter(n => genC.items.get(id(n)) && !effects.items[n] && !(effects.speedItems && effects.speedItems[n]) && !unsupportedEffects.items[id(n)]);
  assert(noEffectItems.length >= 4, `効果を持たない持ち物の候補が少ない: ${noEffectItems}`);
  // 攻撃側の補正を持つ持ち物(いのちのたま等)を投げる組は、実機で効果が乗るか未確認なので engine が攻撃側の持ち物の印を残す
  // (ADR-0144 §結果)。印の付くベクタはゴールデンに入れない(印の有無は engine の単体テストが見る)。
  const flingPairs = [['Tyranitar','Snorlax'],['Weavile','Gardevoir']];
  const powers = new Set();
  for (const [a, d] of flingPairs) {
    for (const item of noEffectItems) {
      const v = addStage3(`fling/${id(item)}/${a}/${d}`, a, d, 'Fling', {a:{item, fling:true}});
      assert(stage3Hp(v), `fling/${id(item)}/${a}/${d}: ダメージが0`);
      powers.add(v.input.Attacker.Item.FlingPower);
    }
    addStage3(`fling/${id('Quick Claw')}/${a}/${d}/crit-reflect`, a, d, 'Fling', {a:{item:'Quick Claw', fling:true}, critical:true, screen:'Reflect'});
    // テクニシャン(威力 60 以下)は投げた持ち物の威力で判定される: 威力 30 の効果を持たない持ち物で確かめる。
    const tech = addStage3(`fling/${id("King's Rock")}/${a}/${d}/technician`, a, d, 'Fling', {a:{item:"King's Rock", fling:true, ability:'Technician'}});
    assert(tech.input.Attacker.Item.FlingPower <= 60, 'fling technician: 持ち物の威力が 60 以下でない');
    // 攻撃側がくろいてっきゅうを持つ(威力 130・接地)。
    const iron = addStage3(`fling/${id('Iron Ball')}/${a}/${d}`, a, d, 'Fling', {a:{item:'Iron Ball', fling:true}});
    assert.equal(iron.input.Attacker.Item.FlingPower, 130);
    // 持ち物なしは失敗(oracle はダメージ 0)。
    const none = addStage3(`fling-none/${a}/${d}`, a, d, 'Fling', {});
    assert(!stage3Hp(none), `fling-none/${a}/${d}: ダメージが0でない`);
  }
  assert(powers.size >= 3, `なげつけるの威力の種類が少ない: ${[...powers]}`);
}
// 分類の切り替え(シェルアームズ型): ランク補正後の 攻撃 / 防御側の防御 > 特攻 / 防御側の特防 なら物理、同じなら特殊。
{
  const sets = [];
  const attackers = ['Garchomp','Gardevoir','Snorlax','Lucario','Tyranitar','Blastoise'], defenders = ['Snorlax','Gardevoir','Corviknight','Toxapex','Charizard'];
  const rankVariants = [{}, {a:{ranks:{atk:1}}}, {a:{ranks:{spa:2}}}, {d:{ranks:{def:2}}}, {d:{ranks:{spd:2}}}, {a:{ranks:{atk:-2}}, d:{ranks:{spd:-1}}}];
  const seen = {physical:0, special:0, tie:0};
  for (const a of attackers) for (const d of defenders) {
    if (a === d) continue;
    for (const [i, rv] of rankVariants.entries()) {
      const opts = {a:{...(rv.a || {}), nature:'Serious'}, d:{...(rv.d || {}), nature:'Serious'}};
      const v = stage3Vector(`category-probe/${a}/${d}/${i}`, a, d, 'Shell Side Arm', opts);
      const label = v.finalCategory === 'physical' ? 'category-physical' : 'category-special';
      // 組ごとに物理 3 件・特殊 3 件まで(同じ組を使い回さない)。
      if (seen[v.finalCategory] >= 8) continue;
      seen[v.finalCategory]++;
      addStage3(`${label}/shellsidearm/${a}/${d}/r${i}`, a, d, 'Shell Side Arm', opts);
    }
  }
  assert(seen.physical >= 3 && seen.special >= 3, `分類の切り替えの物理/特殊のベクタが足りない: ${JSON.stringify(seen)}`);
  // 同値(攻撃 / 防御 = 特攻 / 特防)は特殊。実数値(無補正の性格: 種族値 + 20 + SP)の積が一致する組を総当たりで探す
  // (oracle の比較は浮動小数の除算。同じ比は同じ値になる)。見つけた組のベクタの分類は oracle の結果で確かめる。
  const spLevels = [0, 2, 4, 8, 12, 16, 20, 24, 28, 32];
  const baseOf = name => genC.species.get(id(name)).baseStats;
  let ties = 0;
  search: for (const a of attackers) for (const d of defenders) {
    if (a === d) continue;
    const ab = baseOf(a), db = baseOf(d);
    for (const atkSp of spLevels) for (const spaSp of spLevels) for (const defSp of spLevels) for (const spdSp of spLevels) {
      if ((ab.atk + 20 + atkSp) * (db.spd + 20 + spdSp) !== (ab.spa + 20 + spaSp) * (db.def + 20 + defSp)) continue;
      const opts = {a:{sp:{atk:atkSp, spa:spaSp}, nature:'Serious'}, d:{sp:{def:defSp, spd:spdSp}, nature:'Serious'}};
      const v = addStage3(`category-special/shellsidearm/${a}/${d}/tie-a${atkSp}-${spaSp}-d${defSp}-${spdSp}`, a, d, 'Shell Side Arm', opts);
      const ai = v.expected.attackerStats, di = v.expected.defenderStats;
      assert.equal(ai.atk * di.spd, ai.spa * di.def, `category-tie/${a}/${d}: 実数値の積が一致していない`);
      assert.equal(v.finalCategory, 'special', `category-tie/${a}/${d}: 同値が特殊でない`);
      ties++;
      continue search; // 組ごとに 1 件
    }
    if (ties >= 4) break;
  }
  assert(ties >= 2, `同値になる組が見つからない(${ties} 組。組を選び直す)`);
}
// 持ち物による接地(くろいてっきゅう型 Grounds): 浮いている攻撃側(ひこう・ふゆう)が接地し、フィールドの補正を受ける。
// 防御側が持つ組は未対応の印(UnsupportedDefender)があるので使わない。
const stage3GroundItems = Object.keys(effects.items).filter(n => effects.items[n].Grounds).sort();
assert.deepEqual(stage3GroundItems, ['Iron Ball'], 'Grounds を持つ持ち物が Iron Ball だけでない(ベクタを足す)');
{
  const cases = [
    ['Corviknight', 'Snorlax', 'Thunderbolt', 'electric', {}],
    ['Charizard', 'Garchomp', 'Energy Ball', 'grassy', {}],
    ['Gardevoir', 'Snorlax', 'Psychic', 'psychic', {a:{ability:'Levitate'}}],
    ['Pikachu', 'Corviknight', 'Thunderbolt', 'electric', {a:{ability:'Levitate'}}],
  ];
  for (const itemName of stage3GroundItems) {
    const slug = id(itemName);
    for (const [a, d, moveName, terrain, extra] of cases) {
      const base = {...extra, a:{...(extra.a || {})}};
      const withItem = {...base, a:{...base.a, item:itemName}};
      // 浮いている攻撃側はフィールドの補正を受けない(持ち物なし)が、持ち物で接地して受ける。ふゆう(Levitate)はひこうタイプでなくても浮く。
      const apply = addStage3(`grounded-item/${slug}/${a}/${d}/${moveName}/apply`, a, d, moveName, {...withItem, terrain});
      // 効かない対照: フィールドが無ければ持っていてもダメージは変わらない(oracle で確かめる)。
      const control = addStage3(`grounded-item/${slug}/${a}/${d}/${moveName}/control`, a, d, moveName, {...withItem});
      const noItem = addStage3(`grounded-item/${slug}/${a}/${d}/${moveName}/no-item`, a, d, moveName, {...base, terrain});
      const noItemNoField = stage3Vector(`grounded-item/${slug}/${a}/${d}/${moveName}/ref`, a, d, moveName, {...base});
      assert(stage3Hp(apply) && stage3Hp(noItem), `grounded-item/${slug}/${a}/${d}/${moveName}: ダメージが0`);
      assert.notEqual(rollsKey(apply), rollsKey(noItem), `grounded-item/${slug}/${a}/${d}/${moveName}: 接地でダメージが変わらない`);
      assert.equal(rollsKey(control), rollsKey(noItemNoField), `grounded-item/${slug}/${a}/${d}/${moveName}: フィールドなしで持ち物がダメージを変えた`);
      assert.equal(rollsKey(noItem), rollsKey(noItemNoField), `grounded-item/${slug}/${a}/${d}/${moveName}: 浮いた攻撃側がフィールドの補正を受けた`);
    }
    // 接地済みの攻撃側が持っても変わらない(飛行でも浮いてもいない)。
    const ground = stage3Vector('grounded-item/ground-ref', 'Garchomp', 'Snorlax', 'Thunderbolt', {terrain:'electric'});
    const groundItem = addStage3(`grounded-item/${id('Iron Ball')}/Garchomp/Snorlax/Thunderbolt/already-grounded`, 'Garchomp', 'Snorlax', 'Thunderbolt', {terrain:'electric', a:{item:'Iron Ball'}});
    assert.equal(rollsKey(groundItem), rollsKey(ground), 'grounded-item/already-grounded: 接地済みの攻撃側でダメージが変わった');
  }
}

// 生成器の保証(ADR-0144 §7): (a) 段階3の語彙の moveRules の全技(oracle が計算しない固定ダメージの式を除く)に1件以上のベクタ。
// (b) 状態の値域(残り HP は 1..最大、回数は範囲の多段の最小..最大)。(c) ラベルの網羅。(d) id の重複なし。
{
  const missing = stage3RuleMoves.filter(n => !oracleLacksFixedDamage(stage2Rules[n]) && !stage3CoveredMoves.has(n));
  assert.deepEqual(missing, [], `段階3の moveRules の技にベクタが無い: ${missing.join(', ')}`);
  const labels = new Set(), ids = new Set();
  for (const v of stage3Fixed) {
    assert(!ids.has(v.id), `ベクタの id が重複: ${v.id}`);
    ids.add(v.id);
    labels.add(v.id.split('/')[0]);
    const s = v.input.battleState || {};
    assert(!s.AttackerCurrentHP || (s.AttackerCurrentHP >= 1 && s.AttackerCurrentHP <= v.expected.attackerStats.hp), `${v.id}: 攻撃側の残り HP が値域外`);
    assert(!s.DefenderCurrentHP || (s.DefenderCurrentHP >= 1 && s.DefenderCurrentHP <= v.expected.defenderStats.hp), `${v.id}: 防御側の残り HP が値域外`);
    if (s.Hits) {
      const mh = v.input.Move.mechanismParams && v.input.Move.mechanismParams.MultiHit;
      assert(mh && mh.Min < mh.Max && s.Hits >= mh.Min && s.Hits <= mh.Max, `${v.id}: 回数が範囲の多段の外`);
    }
  }
  for (const l of ['hp-attacker-scaled','hp-attacker-low','hp-defender-ratio','hp-fixed','hp-ko','hits','fling','fling-none','category-physical','category-special','grounded-item']) {
    assert(labels.has(l), `ラベル ${l} のベクタが無い`);
  }
}
const stage3Saved = stage3Fixed.map(stage3Out);

mkdirSync(out,{recursive:true});
const files={};
function save(name,data,compressed=false){const raw=compressed?data.map(v=>JSON.stringify(v)).join('\n')+'\n':JSON.stringify(data,null,2)+'\n';const bytes=compressed?gzipSync(raw,{level:9}):Buffer.from(raw);writeFileSync(`${out}/${name}`,bytes);files[name]={count:data.length,sha256:createHash('sha256').update(bytes).digest('hex')};}
save('fixed.json',championsFixed);save('random.jsonl.gz',randomCases,true);save('attack-species.jsonl.gz',attacks,true);save('defense-species.jsonl.gz',defenses,true);save('stats-species.jsonl.gz',statCases,true);
save('legacy-effects.jsonl.gz',legacyEffectsCases,true);
save('doubles.json',doubleFixed);save('doubles-random.jsonl.gz',doubleRandomCases,true);
save('tera.json',teraFixed);save('tera-random.jsonl.gz',teraRandomCases,true);
save('mechanisms.json',mechanismsFixed);
save('mechanisms-stage2.json',stage2Fixed);
save('mechanisms-stage3.json',stage3Saved);

// ADR-0013 §P1-13.5: oracle のタイプ相性表を engine に渡す入力として出力する。表の正しさは oracle の責務。
// 倍率は oracle の値(0/0.5/1/2)を2倍した整数コード(0=無効/1=いまひとつ/2=等倍/4=抜群)。
// P2-1b: 相性表は Champions 世代(genC)から作る。gen9 と一致しなければ止める(mechanics の変更で表が動いていないことの確認)。
const excludedTypes=['???','stellar']; // oracle の type.id では '' と 'stellar'
const engineTypes=['bug','dark','dragon','electric','fairy','fighting','fire','flying','ghost','grass','ground','ice','normal','poison','psychic','rock','steel','water'];
function buildTypeChart(gen) {
  const chartTypes=[...gen.types].filter(t=>t.id!==''&&t.id!=='stellar').sort((a,b)=>a.id<b.id?-1:1);
  assert.deepEqual(chartTypes.map(t=>t.id).sort(),engineTypes,'oracle types differ from the 18 engine types; review before regenerating');
  const typeNameById=Object.fromEntries([...gen.types].map(t=>[t.id,t.name]));
  const chart={};
  for(const atk of chartTypes) {
    const row={};
    for(const def of engineTypes) {
      const multiplier=atk.effectiveness[typeNameById[def]];
      assert([0,0.5,1,2].includes(multiplier),`unexpected effectiveness ${atk.id} -> ${def}: ${multiplier}`);
      row[def]=multiplier*2;
    }
    chart[atk.id]=row;
  }
  return {chartTypes,chart};
}
const {chartTypes,chart:typeChart}=buildTypeChart(genC);
const {chart:typeChartGen9}=buildTypeChart(gen9);
assert.deepEqual(typeChart,typeChartGen9,'Champions と gen9 のタイプ相性表が一致しない(mechanics の変更を確認すること)');
assert.equal(Object.keys(typeChart).length*engineTypes.length,324);
{
  const raw=JSON.stringify({schemaVersion:1,source:'@smogon/calc',version,generation:0,
    note:'Multiplier codes are the effectiveness x2 as integers (0=immune, 1=not very effective, 2=neutral, 4=super effective). ADR-0013. Generated from the Champions generation (Generations.get(0)); confirmed identical to gen9.',
    excludedTypes,types:engineTypes,effectiveness:typeChart},null,2)+'\n';
  writeFileSync(`${out}/typechart.json`,raw);
  files['typechart.json']={count:engineTypes.length*engineTypes.length,sha256:createHash('sha256').update(raw).digest('hex')};
}

// --- metadata.json(schemaVersion 2。ADR-0002 §決定4 / P2-1b) -----------------
const championsFiles=['fixed.json','random.jsonl.gz','attack-species.jsonl.gz','defense-species.jsonl.gz','stats-species.jsonl.gz','typechart.json','doubles.json','doubles-random.jsonl.gz','tera.json','tera-random.jsonl.gz','mechanisms.json','mechanisms-stage2.json','mechanisms-stage3.json'];
const legacyFiles=['legacy-effects.jsonl.gz'];
const metadata={
  schemaVersion:2,
  source:'@smogon/calc',
  version,
  generation:'champions',
  seed,
  randomAlgorithm:'xorshift32',
  level:50,
  ivs:31,
  speciesScope:'@smogon/calc 0.12.0 Champions generation (Generations.get(0)) species/forms, minus internal forms not selectable in-game (see exclusions)',
  speciesCount:species.length,
  representativeMoves:moveNames,
  learnsets:'Not asserted: arithmetic type/category coverage only',
  koModel:'ADR-0006: independent uniform rolls, full HP, identical hit repeated without residuals/recovery or consumable state transitions',
  exclusions:[
    {scope:'species',names:excludedSpeciesNames,reason:'Internal calc-only pseudo-form; not a selectable in-game form (P2-1b)'},
    {scope:'species',names:[...genC.species].filter(s=>s.baseStats.hp===1).map(s=>s.name),reason:'HP=1 special mechanic is outside Champions SP formula; not present in the current Champions set'},
    {scope:'moves',reason:'The fixed/random/species files use only the listed fixed-power single-hit moves. Stage-1 move mechanisms (multi-hit, fixed damage equal to the level, forced criticals, defense-rank ignoring, alternate attack/defense stats) are checked in mechanisms.json (ADR-0142). Stage-2 move rules (ADR-0143: power formulas by weight / speed ratio / positive ranks / hit index, status / item / weather / terrain conditions, move type by weather or terrain, two-type effectiveness, Freeze-Dry, screen removal, priority boost, moves whose handlers do not affect damage) are checked in mechanisms-stage2.json (stage-2 moves are checked in mechanisms-stage2.json, not the other files). Stage-3 move rules and battle state (ADR-0144: current HP of the attacker / defender as the oracle curHP, the number of hits of a range multi-hit move as the oracle hits, power by current HP (Eruption / Water Spout / Flail / Reversal / Hard Press), damage equal to the attacker current HP (Final Gambit), Fling by the attacker item, Shell Side Arm category by stats) are checked in mechanisms-stage3.json; Endeavor and Super Fang are not computed by the oracle (it returns 0 damage) so they are covered by engine unit tests only. Excluded everywhere: OHKO (the oracle does not compute it), other fixed damage, moves that need battle history (not covered), tera/Z/Max moves'},
    {scope:'abilities/items',reason:'Only effects.json adapters; no default species ability; Eviolite/Choice Band/Choice Specs/Assault Vest/Steelworker moved to legacy-effects (gen9), not present in the Champions vectors. Champions vectors additionally cover ability-based type immunity/absorption (Levitate, Water Absorb, Volt Absorb, Earth Eater, Flash Fire, Sap Sipper, Motor Drive, Lightning Rod; ADR-0106); Dry Skin (also boosts Fire move power while absorbing Water, not representable yet) and Storm Drain (absent from the Champions generation) are excluded (ADR-0106 limits 1-2). Every non-legacy effects.json entry with a type-dependent effect (issue #270 / ADR-0120) or a stage-1 ability field (TypeConvert, PowerMods, AuraType/AuraMod, StatMods, SeparateStatMods, CritDamageMod, PreventsCritical, IgnoresOpponentRanks, IgnoresDefenderAbility; ADR-0176) has an apply/control pair (effects/<id>/...), and every Breakable definition has a breakable vector showing that an IgnoresDefenderAbility attacker (Mold Breaker) gets the same damage as against no ability (ADR-0176). The coverage survey also uses both-side +/-2 ranks, a poisoned defender and attacker-only probes against a defender ability (Thick Fat, Fur Coat, Fluffy, Multiscale, Levitate, critical hit x Shell Armor), so abilities that change damage only in combination (Mold Breaker, Unaware, Merciless, Long Reach) must be either defined or marked; Breakable is checked against the oracle in both directions, not listed by hand (ADR-0176). Champions items/abilities that change damage but are not representable by the effect schema (move-flag and HP-dependent abilities are later stages) are listed with reasons in tools/golden/unsupported-effects.json and never appear in vectors; unsupported-mark definitions carry no Breakable in stage 1, so Mold Breaker against a marked defender ability keeps the mark (ADR-0176)'},
    {scope:'terrain',reason:'Grounding (ADR-0116) covers Flying type and Levitate (Airborne ability effect) only; Gravity and Air Balloon are not modeled and never appear (Iron Ball grounding is modeled as the Grounds item effect and checked in mechanisms-stage3.json grounded-item/*, attacker side only because the defender side keeps an unsupported mark; ADR-0144); the Psychic Terrain priority block is covered by psychic-priority/* (ADR-0123); terrain-specific moves (Grassy Terrain Earthquake/Bulldoze halving, Terrain Pulse, Misty Explosion, Expanding Force, Rising Voltage) are checked in mechanisms-stage2.json (ADR-0143)'},
    {scope:'battle',reason:'Doubles are covered only by doubles.json and doubles-random.jsonl.gz (ADR-0222): screens 2732/4096 and spread 3072/4096 for allAdjacent/allAdjacentFoes moves. Tera is absent from Pokemon Champions and appears only in tera.json and tera-random.jsonl.gz as an optional feature (ADR-0224): singles only; attacker STAB and has-type checks (grounding, Psychic Terrain priority, Sand/Snow defense) use the tera type, while defender type effectiveness ignores it (Champions generation quirk); no Stellar, Tera Blast or 60 BP floor (absent from the Champions generation). Doubles have no tera; ally effects (Helping Hand, Friend Guard), Dynamax, form transformations or unsupported status effects'},
    {scope:'KO',reason:'Smogon residual/consumable multi-turn model differs from ADR-0006; direct smogonKO cross-check only residual/consumable-free fixed cases with 1-4 hits'},
  ],
  oracles:[
    {
      id:'champions', source:'@smogon/calc', version, generation:'champions', generationNum:0,
      spInput:'direct', files:championsFiles, koCrossChecks:championsKoCrossChecks,
      doublesRandomSeed:doubleRandomSeed,
      teraRandomSeed,
    },
    {
      id:'gen9-legacy-effects', source:'@smogon/calc', version, generation:'gen9', generationNum:9,
      spInput:'max(0,8*SP-4)', seed:legacyRandomSeed, files:legacyFiles, koCrossChecks:legacyKoCrossChecks,
      legacyEffects:{items:[...legacyItemNames], abilities:[...legacyAbilityNames]},
      reason:'These items/ability are not implemented in @smogon/calc 0.12.0 Champions generation mechanics. The calc-formula check (ADR-0002 decision 7) continues against gen9, independent of in-game availability.',
    },
  ],
  files,
};
writeFileSync(`${out}/metadata.json`,JSON.stringify(metadata,null,2)+'\n');
console.log(JSON.stringify({species:species.length,effectCoverage,breakableCoverage,championsKoCrossChecks,legacyKoCrossChecks,legacyRandomCount:legacyRandomCases.length,files},null,2));
