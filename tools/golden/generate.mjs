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
  if (ability) assert(effects.abilities[ability]);
  if (item) assert(effects.items[item]);
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
  // ADR-0005 assumes grounded combatants. Restrict terrain cases in advance accordingly.
  if (terrain!=='none') assert(!attack.p.hasType('Flying') && !defend.p.hasType('Flying'));
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
const grounded=species.filter(s=>!s.types.includes('Flying'));
for(let i=0;i<10000;i++) {
  const terrain=pick(['none','electric','grassy','misty','psychic']);
  const pool=terrain==='none'?species:grounded;
  const a=pick(pool),d=pick(pool),m=pick(moves);
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
const legacyGrounded = legacySpeciesPool.filter(s=>!s.types.includes('Flying'));
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
    const pool=terrain==='none'?legacySpeciesPool:legacyGrounded;
    const a=legacyPick(pool),d=legacyPick(pool),m=legacyPick(layer.moves);
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
    {scope:'abilities/items',reason:'Only effects.json adapters; no default species ability; Eviolite/Choice Band/Choice Specs/Assault Vest/Steelworker moved to legacy-effects (gen9), not present in the Champions vectors'},
    {scope:'terrain',reason:'Flying species excluded from terrain-enabled random/fixed cases; ADR-0005 assumes grounded, no Levitate/Air Balloon admitted'},
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
console.log(JSON.stringify({species:species.length,championsKoCrossChecks,legacyKoCrossChecks,legacyRandomCount:legacyRandomCases.length,files},null,2));
