// Deterministic external oracle. Run npm ci && npm run generate in this directory.
import calc from '@smogon/calc';
import {readFileSync, writeFileSync, mkdirSync} from 'node:fs';
import {gzipSync} from 'node:zlib';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';
import assert from 'node:assert/strict';
const {Generations, Pokemon, Move, Field, calculate} = calc;
const gen = Generations.get(9);
const out = fileURLToPath(new URL('../../testdata/golden/', import.meta.url));
const version = JSON.parse(readFileSync(new URL('node_modules/@smogon/calc/package.json', import.meta.url))).version;
assert.equal(version, '0.10.0', 'Review oracle upgrades before regenerating');
const effects = JSON.parse(readFileSync(`${out}/effects.json`));
const keys = ['hp','atk','def','spa','spd','spe'];
const stats = (value = 0) => Object.fromEntries(keys.map(k => [k,value]));
const species = [...gen.species].filter(s => s.baseStats.hp !== 1).sort((a,b) => a.id < b.id ? -1 : a.id > b.id ? 1 : 0);
const id = name => name.toLowerCase().replace(/[^a-z0-9]/g,'');
// Fixed-power, single-hit reference moves. These probe arithmetic, not learnset legality.
const moveNames = [
  ['Body Slam','Hyper Voice'], ['Fire Punch','Flamethrower'], ['Aqua Tail','Surf'],
  ['Thunder Punch','Thunderbolt'], ['Seed Bomb','Energy Ball'], ['Ice Punch','Ice Beam'],
  ['Drain Punch','Aura Sphere'], ['Poison Jab','Sludge Bomb'], ['Drill Run','Earth Power'],
  ['Drill Peck','Air Slash'], ['Zen Headbutt','Psychic'], ['X-Scissor','Bug Buzz'],
  ['Rock Slide','Power Gem'], ['Shadow Punch','Shadow Ball'], ['Dragon Claw','Dragon Pulse'],
  ['Crunch','Dark Pulse'], ['Iron Head','Flash Cannon'], ['Play Rough','Moonblast']
].flat();
const moves = moveNames.map(n => gen.moves.get(id(n)));
for (const m of moves) assert(m && m.basePower > 0 && !m.multihit && !m.willCrit && !m.overrideDefensiveStat);
const physical = moves.filter(m => m.category === 'Physical');
const special = moves.filter(m => m.category === 'Special');
const attackAnchors = [ ['Garchomp','Dragon Claw'], ['Charizard','Flamethrower'], ['Pikachu','Thunderbolt'], ['Scizor','Iron Head'], ['Gengar','Shadow Ball'] ];
const defenderAnchors = ['Blissey','Tyranitar','Corviknight','Toxapex','Garchomp'];
const nature = name => {const n = gen.natures.get(id(name));return n.plus === n.minus ? {} : {Plus:n.plus,Minus:n.minus};};
function individual(name, options = {}) {
  const sp = {...stats(),...options.sp};
  assert(Object.values(sp).every(v => v >= 0 && v <= 32));
  assert(Object.values(sp).reduce((a,b) => a+b,0) <= 66);
  const ability = options.ability || '';
  const item = options.item || '';
  if (ability) assert(effects.abilities[ability]);
  if (item) assert(effects.items[item]);
  // Empty ability alone is insufficient: Pokemon.clone() otherwise restores the species default.
  const p = new Pokemon(gen, name, {level:50, ivs:stats(31),
    evs:Object.fromEntries(keys.map(k => [k,Math.max(0,8*sp[k]-4)])),
    nature:options.nature || 'Serious', boosts:options.ranks || {}, ability, item,
    status:options.burn ? 'brn' : '', overrides:{abilities:{0:''}}});
  assert.equal(p.ability || '', ability);
  assert.equal(p.clone().ability || '', ability);
  return {p, input:{Species:{Key:p.species.id,Types:p.types.map(t => t.toLowerCase()),BaseStats:p.species.baseStats},
    Level:50, Nature:nature(p.nature), SP:sp, Ranks:options.ranks || {}, Status:options.burn ? 'burn':'none',
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
function vector(label, a, d, moveName, options={}) {
  const attack=individual(a,options.a), defend=individual(d,options.d);
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
const fixed=[];
const scenarios = [
  ['baseline',{}],['critical',{critical:true}],['burn',{a:{burn:true}}],
  ['sun',{weather:'sun'}],['rain',{weather:'rain'}],['sand',{weather:'sand'}],['snow',{weather:'snow'}],
  ['electric',{terrain:'electric'}],['grassy',{terrain:'grassy'}],['psychic',{terrain:'psychic'}],['misty',{terrain:'misty'}],
  ['reflect',{screen:'Reflect'}],['light-screen',{screen:'LightScreen'}],['aurora-veil',{screen:'AuroraVeil'}],
  ...['Choice Band','Choice Specs','Life Orb','Expert Belt','Charcoal','Muscle Band','Wise Glasses'].map(item=>[id(item),{a:{item}}]),
  ...['Assault Vest','Occa Berry','Chilan Berry'].map(item=>[id(item),{d:{item}}]),
  ['adaptability',{a:{ability:'Adaptability'}}],['steelworker',{a:{ability:'Steelworker'}}],
  ['water-bubble',{a:{ability:'Water Bubble',burn:true}}],['thick-fat',{d:{ability:'Thick Fat'}}],
  ['filter',{d:{ability:'Filter'}}],['filter-life-orb',{a:{item:'Life Orb'},d:{ability:'Filter'}}],
  ['sun-charcoal-thick-fat',{weather:'sun',a:{item:'Charcoal'},d:{ability:'Thick Fat'}}],
  ['sand-vest',{weather:'sand',d:{item:'Assault Vest'}}],
  ['critical-ranks-screens',{critical:true,a:{ranks:{atk:-6,spa:-6}},d:{ranks:{def:6,spd:6}},screen:'AuroraVeil'}]
];
const fixedPairs=[['Magmortar','Abomasnow','Flamethrower'],['Snorlax','Blissey','Body Slam'],['Raichu','Blastoise','Thunderbolt'],['Tangrowth','Tyranitar','Energy Ball'],['Metagross','Clefable','Iron Head'],['Haxorus','Garchomp','Dragon Pulse']];
for (const [label,options] of scenarios) for(const [a,d,m] of fixedPairs) fixed.push(vector(`${label}/${a}/${m}`,a,d,m,options));
fixed.push(vector('eviolite-snow','Snorlax','Snover','Body Slam',{weather:'snow',d:{item:'Eviolite'}}));
fixed.push(vector('water-bubble-defense','Magmortar','Blastoise','Flamethrower',{d:{ability:'Water Bubble'}}));
fixed.push(vector('water-bubble-attack','Blastoise','Snorlax','Surf',{a:{ability:'Water Bubble'}}));
fixed.push(vector('ko-four-hits','Snorlax','Snorlax','Body Slam',{d:{sp:{def:32}}}));
fixed.push(vector('misty-dragon','Haxorus','Snorlax','Dragon Claw',{terrain:'misty'}));
// A fixed external KO cross-check for all matching 1..4-hit cases, without residual/berry model differences.
let koCrossChecks=0;
for(const v of fixed) {
  if(v.input.Field.Weather!=='none'||v.input.Field.Terrain!=='none'||v.input.Attacker.Status!=='none'||v.input.Attacker.Item||v.input.Defender.Item||v.input.Attacker.Ability.ID||v.input.Defender.Ability.ID||v.expected.ko.Hits<1||v.expected.ko.Hits>4) continue;
  const a=individual(v.oracle.attacker),d=individual(v.oracle.defender);
  // Call the oracle KO implementation with already externally computed rolls and the same HP.
  const result=calculate(gen,a.p,d.p,new Move(gen,v.oracle.move),new Field());
  result.damage=v.expected.rolls;
  const external=result.kochance();
  assert.equal(external.n,v.expected.ko.Hits);
  assert(Math.abs(external.chance-(v.expected.ko.Guaranteed?1:v.expected.ko.ChancePercent/100))<1e-12);
  v.expected.smogonKO=external;koCrossChecks++;
}
assert.deepEqual([...new Set(fixed.flatMap(v=>v.expected.smogonKO ? [v.expected.smogonKO.n] : []))].sort(),[1,2,3,4]);
const seed=0x504f4b45;
let state=seed;
function random(){state^=state<<13;state^=state>>>17;state^=state<<5;return (state>>>0)/4294967296;}
const pick=xs=>xs[Math.floor(random()*xs.length)];
function randomSP(){
  const sp=stats(), order=[...keys]; let budget=66;
  for(let i=order.length-1;i>0;i--){const j=Math.floor(random()*(i+1));[order[i],order[j]]=[order[j],order[i]];}
  for(const k of order){sp[k]=Math.floor(random()*(Math.min(32,budget)+1));budget-=sp[k];}
  return sp;
}
const randomCases=[];
const grounded=species.filter(s=>!s.types.includes('Flying'));
for(let i=0;i<10000;i++) {
  const terrain=pick(['none','electric','grassy','misty','psychic']);
  const pool=terrain==='none'?species:grounded;
  const a=pick(pool),d=pick(pool),m=pick(moves);
  const attackItem=pick(['','Choice Band','Choice Specs','Life Orb','Expert Belt','Charcoal','Muscle Band','Wise Glasses']);
  const defendItem=pick(['','Assault Vest','Occa Berry','Chilan Berry']);
  const rank=()=>Object.fromEntries(keys.slice(1).map(k=>[k,Math.floor(random()*13)-6]));
  randomCases.push(vector(`random/${String(i).padStart(5,'0')}`,a.name,d.name,m.name,{terrain,weather:pick(Object.keys(weatherNames)),
    critical:random()<0.1,screen:pick(['','Reflect','LightScreen','AuroraVeil']),
    a:{sp:randomSP(),nature:pick(['Serious','Adamant','Modest','Bold','Calm']),item:attackItem,ability:pick(['','Adaptability','Steelworker','Water Bubble']),burn:random()<0.2,ranks:rank()},
    d:{sp:randomSP(),nature:pick(['Serious','Adamant','Modest','Bold','Calm']),item:defendItem,ability:pick(['','Thick Fat','Filter','Solid Rock','Water Bubble']),ranks:rank()}}));
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
  // All gen9 data species/forms are reference inputs; Champions legality awaits P2.
  for(const category of [physical,special]) {
    const m=category.find(m=>s.types.includes(m.type));assert(m,`missing representative move: ${s.name}`);
    const atk=m.category==='Physical'?'atk':'spa';
    for(const boosted of [false,true]) for(const d of defenderAnchors)
      attacks.push(vector(`attack/${s.id}/${m.id}/${boosted?'max':'zero'}/${id(d)}`,s.name,d,m.name,{a:boosted?{sp:{[atk]:32},nature:atk==='atk'?'Adamant':'Modest'}:{}}));
  }
  for(const [a,m] of attackAnchors) for(const [preset,opt] of defensePresets)
    defenses.push(vector(`defense/${s.id}/${id(a)}/${preset}`,a,s.name,m,{d:opt}));
  for(const k of keys) for(const sp of [0,1,31,32]) for(const modifier of ['neutral','plus','minus']) {
    const n=[...gen.natures].find(n=>modifier==='neutral'?n.plus===n.minus:modifier==='plus'?n.plus===k&&n.minus!==k:n.minus===k&&n.plus!==k);
    // HP cannot receive nature changes. Three neutral nature cases still verify HP invariance.
    const p=individual(s.name,{sp:{[k]:sp},nature:n?.name || 'Serious'});
    statCases.push({id:`stats/${s.id}/${k}/${sp}/${modifier}`,individual:p.input,expected:p.p.rawStats});
  }
}
mkdirSync(out,{recursive:true});
const files={};
function save(name,data,compressed=false){const raw=compressed?data.map(v=>JSON.stringify(v)).join('\n')+'\n':JSON.stringify(data,null,2)+'\n';const bytes=compressed?gzipSync(raw,{level:9}):Buffer.from(raw);writeFileSync(`${out}/${name}`,bytes);files[name]={count:data.length,sha256:createHash('sha256').update(bytes).digest('hex')};}
save('fixed.json',fixed);save('random.jsonl.gz',randomCases,true);save('attack-species.jsonl.gz',attacks,true);save('defense-species.jsonl.gz',defenses,true);save('stats-species.jsonl.gz',statCases,true);
// ADR-0013 §P1-13.5: oracle のタイプ相性表を engine に渡す入力として出力する。表の正しさは oracle の責務。
// 倍率は oracle の値(0/0.5/1/2)を2倍した整数コード(0=無効/1=いまひとつ/2=等倍/4=抜群)。
const excludedTypes=['???','stellar']; // oracle の type.id では '' と 'stellar'
const engineTypes=['bug','dark','dragon','electric','fairy','fighting','fire','flying','ghost','grass','ground','ice','normal','poison','psychic','rock','steel','water'];
const chartTypes=[...gen.types].filter(t=>t.id!==''&&t.id!=='stellar');
assert.deepEqual(chartTypes.map(t=>t.id).sort(),engineTypes,'oracle types differ from the 18 engine types; review before regenerating');
const typeNameById=Object.fromEntries([...gen.types].map(t=>[t.id,t.name]));
const typeChart={};
for(const atk of [...chartTypes].sort((a,b)=>a.id<b.id?-1:1)) {
  const row={};
  for(const def of engineTypes) {
    const multiplier=atk.effectiveness[typeNameById[def]];
    assert([0,0.5,1,2].includes(multiplier),`unexpected effectiveness ${atk.id} -> ${def}: ${multiplier}`);
    row[def]=multiplier*2;
  }
  typeChart[atk.id]=row;
}
assert.equal(Object.keys(typeChart).length*engineTypes.length,324);
{
  const raw=JSON.stringify({schemaVersion:1,source:'@smogon/calc',version,generation:9,
    note:'Multiplier codes are the effectiveness x2 as integers (0=immune, 1=not very effective, 2=neutral, 4=super effective). ADR-0013',
    excludedTypes,types:engineTypes,effectiveness:typeChart},null,2)+'\n';
  writeFileSync(`${out}/typechart.json`,raw);
  files['typechart.json']={count:engineTypes.length*engineTypes.length,sha256:createHash('sha256').update(raw).digest('hex')};
}
writeFileSync(`${out}/metadata.json`,JSON.stringify({schemaVersion:1,source:'@smogon/calc',version,generation:9,seed,randomAlgorithm:'xorshift32',level:50,ivs:31,spToEV:'max(0,8*SP-4)',speciesScope:'gen9 library reference species/forms (including CAP fan species), NOT Champions availability; P2 source investigation pending',speciesCount:species.length,representativeMoves:moveNames,learnsets:'Not asserted: arithmetic type/category coverage only',koModel:'ADR-0006: independent uniform rolls, full HP, identical hit repeated without residuals/recovery or consumable state transitions',koCrossChecks,exclusions:[{scope:'species',names:[...gen.species].filter(s=>s.baseStats.hp===1).map(s=>s.name),reason:'HP=1 special mechanic is outside Champions SP formula'},{scope:'moves',reason:'Only the listed fixed-power single-hit moves; excludes variable/fixed damage, multi-hit, forced criticals, alternate attack/defense stats, screen removal, terrain-specific move mechanics, tera/Z/Max moves'},{scope:'abilities/items',reason:'Only effects.json adapters; no default species ability; Eviolite only on Snover fixed case'},{scope:'terrain',reason:'Flying species excluded from terrain-enabled random/fixed cases; ADR-0005 assumes grounded, no Levitate/Air Balloon admitted'},{scope:'battle',reason:'No double/tera/Dynamax/form transformations or unsupported status effects'},{scope:'KO',reason:'Smogon residual/consumable multi-turn model differs from ADR-0006; direct smogonKO cross-check only residual/consumable-free fixed cases with 1-4 hits'}],files},null,2)+'\n');
console.log(JSON.stringify({species:species.length,koCrossChecks,files},null,2));
