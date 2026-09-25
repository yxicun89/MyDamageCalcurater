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
const {Generations, Pokemon, Move, Field, calculate} = calc;
const genC = Generations.get(0);
const gen9 = Generations.get(9);
const out = fileURLToPath(new URL('../../testdata/golden/', import.meta.url));
const version = JSON.parse(readFileSync(new URL('node_modules/@smogon/calc/package.json', import.meta.url))).version;
assert.equal(version, '0.12.0', 'Review oracle upgrades before regenerating');
const effects = JSON.parse(readFileSync(`${out}/effects.json`));
const keys = ['hp','atk','def','spa','spd','spe'];
const stats = (value = 0) => Object.fromEntries(keys.map(k => [k,value]));
const id = name => name.toLowerCase().replace(/[^a-z0-9]/g,'');

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
function individual(gen, name, options = {}) {
  const sp = {...stats(),...options.sp};
  assert(Object.values(sp).every(v => v >= 0 && v <= 32));
  assert(Object.values(sp).reduce((a,b) => a+b,0) <= 66);
  const ability = options.ability || '';
  const item = options.item || '';
  if (ability) assert(effects.abilities[ability] && !isUnsupportedEffect(effects.abilities[ability]));
  if (item) assert(effects.items[item] && !isUnsupportedEffect(effects.items[item]));
  if (gen.num === 0) {
    assert(!item || !legacyItems.has(item), `Champions ベクタが legacy 持ち物 ${item} を使おうとした`);
    assert(!ability || !legacyAbilities.has(ability), `Champions ベクタが legacy 特性 ${ability} を使おうとした`);
  }
  const evs = gen.num === 0 ? {...sp} : Object.fromEntries(keys.map(k => [k,Math.max(0,8*sp[k]-4)]));
  // Empty ability alone is insufficient: Pokemon.clone() otherwise restores the species default.
  const p = new Pokemon(gen, name, {level:50, ivs:stats(31),
    evs, nature:options.nature || 'Serious', boosts:options.ranks || {}, ability, item,
    status:options.burn ? 'brn' : '', overrides:{abilities:{0:''}}});
  assert.equal(p.ability || '', ability);
  assert.equal(p.clone().ability || '', ability);
  return {p, input:{Species:{Key:p.species.id,Types:p.types.map(t => t.toLowerCase()),BaseStats:p.species.baseStats},
    Level:50, Nature:nature(gen, p.nature), SP:sp, Ranks:options.ranks || {}, Status:options.burn ? 'burn':'none',
    Ability:{ID:ability,Effect:effects.abilities[ability] || null},
    Item:item ? {ID:item,Effect:effects.items[item]} : null}};
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
function vector(gen, label, a, d, moveName, options={}) {
  const attack=individual(gen, a, options.a), defend=individual(gen, d, options.d);
  const weather=options.weather || 'none', terrain=options.terrain || 'none';
  const screen=options.screen;
  const m = new Move(gen, moveName, {isCrit:!!options.critical});
  assert(m.bp>0 && m.hits===1);
  const f = new Field({gameType:'Singles',weather:weatherNames[weather],terrain:terrainNames[terrain],
    defenderSide:{isReflect:screen==='Reflect',isLightScreen:screen==='LightScreen',isAuroraVeil:screen==='AuroraVeil'}});
  const result=calculate(gen,attack.p,defend.p,m,f);
  const rolls=typeof result.damage==='number' ? Array(16).fill(result.damage) : result.damage;
  assert.equal(rolls.length,16); assert(rolls.every(Number.isInteger));
  const expectedKO=ko(rolls,defend.p.rawStats.hp);
  return {id:label, oracle:{attacker:a,defender:d,move:moveName}, input:{Format:'single',Attacker:attack.input,Defender:defend.input,
    Move:{ID:id(moveName),Type:m.type.toLowerCase(),Category:m.category.toLowerCase(),Power:m.bp,Priority:m.priority},
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
const typedEffectCases = (kind, name, def) => {
  const slug = `effects/${id(name)}`;
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
    const c = types.includes('normal') ? 'fighting' : 'normal';
    assert(!types.includes(c), `${slug}: 対照のタイプ ${c} も軽減の対象になっている`);
    effectCase(`${slug}/defresist/${c}/control`, c, neutral, {d:{ability:name}}, false);
  }
  if (kind === 'abilities' && def.ReduceSuperEffective) {
    effectCase(`${slug}/reduce/fighting/apply`, 'fighting', superEffective, {d:{ability:name}}, true);
    effectCase(`${slug}/reduce/fighting/control`, 'fighting', neutral, {d:{ability:name}}, false);
  }
};
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
const probeMoves = [...moves, ...probeExtraMoveNames.map(n => {
  const m = genC.moves.get(id(n));
  assert(m && m.basePower > 0, `調査用の技 ${n} が Champions 世代に無い(改名された可能性。列挙を見直す)`);
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
];
function probePokemon(name, side, ability, item) {
  const p = new Pokemon(genC, name, {level:50, ivs:stats(31), evs:stats(), nature:'Serious', ability, item,
    status:side.status || '', overrides:{abilities:{0:''}}});
  if (side.hpFraction) p.originalCurHP = Math.floor(p.maxHP() / side.hpFraction);
  return p;
}
function probeSignature(holder, ability, item) {
  const out = [];
  for (const cond of probeConditions) for (const a of probeAttackers) for (const d of probeDefenders) for (const m of probeMoves) {
    const A = probePokemon(a, cond.a, holder === 'a' ? ability : '', holder === 'a' ? item : '');
    const D = probePokemon(d, cond.d, holder === 'd' ? ability : '', holder === 'd' ? item : '');
    const r = calculate(genC, A, D, new Move(genC, m.name, {isCrit:!!cond.crit}), new Field({gameType:'Singles', ...cond.field}));
    out.push(JSON.stringify(r.damage));
  }
  return out.join('|');
}
const probeBaseline = probeSignature('a', '', '');
const damageChanging = {items:[], abilities:[]};
// changingSides は、持たせるとダメージが変わった側(a: 攻撃側 / d: 防御側。issue #270 案 B / ADR-0123)。
const changingSides = {items:{}, abilities:{}};
const probeSides = (kind, list, probe) => {
  for (const x of list) {
    const sides = {a: probe('a', x.name) !== probeBaseline, d: probe('d', x.name) !== probeBaseline};
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
  const defined = new Set(names.filter(n => !isUnsupportedEffect(effects[kind][n])).map(id));
  const changing = new Set(damageChanging[kind]);
  const undefinedChanging = [...changing].filter(x => !defined.has(x)).sort();
  assert.deepEqual(undefinedChanging, Object.keys(unsupportedEffects[kind]).sort(),
    `${kind}: ダメージに効くのに効果定義が無いものが unsupported-effects.json と一致しない(定義を足すか、理由付きで登録する)`);
  // 未対応の一覧(理由)と、効果定義の「未対応」の印は同じ集合で、印の側は oracle でダメージが変わった側と一致する。
  assert.deepEqual(marked.map(id).sort(), undefinedChanging,
    `${kind}: effects.json の未対応の印(UnsupportedAttacker / UnsupportedDefender)が unsupported-effects.json と一致しない`);
  for (const n of marked) {
    const def = effects[kind][n], sides = changingSides[kind][id(n)];
    assert.deepEqual(Object.keys(def).sort().filter(k => !unsupportedEffectKeys.includes(k)), [],
      `${kind} ${n}: 未対応の印と補正の定義を同じ項目に混ぜない`);
    assert.deepEqual({a:def.UnsupportedAttacker === true, d:def.UnsupportedDefender === true}, sides,
      `${kind} ${n}: 未対応の印の側が oracle でダメージが変わる側と一致しない(a=攻撃側 / d=防御側)`);
  }
  const deadDefinitions = [...defined].filter(x => !changing.has(x)).sort();
  assert.deepEqual(deadDefinitions, [], `${kind}: 効果定義があるのに oracle のダメージが変わらない(定義の誤りか調査条件の不足)`);
  effectCoverage[kind] = {damageChanging:changing.size, defined:defined.size, unsupported:undefinedChanging.length};
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
mkdirSync(out,{recursive:true});
const files={};
function save(name,data,compressed=false){const raw=compressed?data.map(v=>JSON.stringify(v)).join('\n')+'\n':JSON.stringify(data,null,2)+'\n';const bytes=compressed?gzipSync(raw,{level:9}):Buffer.from(raw);writeFileSync(`${out}/${name}`,bytes);files[name]={count:data.length,sha256:createHash('sha256').update(bytes).digest('hex')};}
save('fixed.json',championsFixed);save('random.jsonl.gz',randomCases,true);save('attack-species.jsonl.gz',attacks,true);save('defense-species.jsonl.gz',defenses,true);save('stats-species.jsonl.gz',statCases,true);
save('legacy-effects.jsonl.gz',legacyEffectsCases,true);

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
const championsFiles=['fixed.json','random.jsonl.gz','attack-species.jsonl.gz','defense-species.jsonl.gz','stats-species.jsonl.gz','typechart.json'];
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
    {scope:'moves',reason:'Only the listed fixed-power single-hit moves; excludes variable/fixed damage, multi-hit, forced criticals, alternate attack/defense stats, screen removal, terrain-specific move mechanics, tera/Z/Max moves'},
    {scope:'abilities/items',reason:'Only effects.json adapters; no default species ability; Eviolite/Choice Band/Choice Specs/Assault Vest/Steelworker moved to legacy-effects (gen9), not present in the Champions vectors. Champions vectors additionally cover ability-based type immunity/absorption (Levitate, Water Absorb, Volt Absorb, Earth Eater, Flash Fire, Sap Sipper, Motor Drive, Lightning Rod; ADR-0106); Dry Skin (also boosts Fire move power while absorbing Water, not representable yet) and Storm Drain (absent from the Champions generation) are excluded (ADR-0106 limits 1-2). Every non-legacy effects.json entry with a type-dependent effect has an apply/control pair (effects/<id>/...; issue #270 / ADR-0120). Champions items/abilities that change damage but are not representable by the effect schema are listed with reasons in tools/golden/unsupported-effects.json and never appear in vectors'},
    {scope:'terrain',reason:'Grounding (ADR-0116) covers Flying type and Levitate (Airborne ability effect) only; Gravity, Iron Ball and Air Balloon are not modeled and never appear; the Psychic Terrain priority block is covered by psychic-priority/* (ADR-0123); terrain-specific moves (Grassy Terrain Earthquake/Bulldoze halving, Terrain Pulse etc.) are outside the move list and carry an unsupported mark in the engine (ADR-0123)'},
    {scope:'battle',reason:'No double/tera/Dynamax/form transformations or unsupported status effects'},
    {scope:'KO',reason:'Smogon residual/consumable multi-turn model differs from ADR-0006; direct smogonKO cross-check only residual/consumable-free fixed cases with 1-4 hits'},
  ],
  oracles:[
    {
      id:'champions', source:'@smogon/calc', version, generation:'champions', generationNum:0,
      spInput:'direct', files:championsFiles, koCrossChecks:championsKoCrossChecks,
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
console.log(JSON.stringify({species:species.length,effectCoverage,championsKoCrossChecks,legacyKoCrossChecks,legacyRandomCount:legacyRandomCases.length,files},null,2));
