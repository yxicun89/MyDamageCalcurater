import glob,re,subprocess
exec(open("../rep.py").read().split("for f,rs")[0])
pairs=[]
for rs in R.values(): pairs+=rs
pairs+=[('"観測を追加"','"ダメージを追加"'),("観測を追加","ダメージを追加"),("この観測を削除","このダメージを削除"),("観測と一致","入力したダメージと一致"),
 ("のメガストーン","専用のメガストーン"),("API に接続できません","サーバーに接続できません"),("マスタにありません","データにありません"),("マスタに見つかりません","データに見つかりません"),("サーバーのマスタ","サーバーのデータ"),("ポケモンのマスタを","ポケモンのデータを")]
files=glob.glob("PokeCalcKit/Tests/**/*.swift",recursive=True)+glob.glob("PokeCalcUITests/**/*.swift",recursive=True)
for f in files:
    if "Judge" in f: continue
    s=o=open(f).read()
    # only touch string literals: apply per line where line contains a quote
    out=[]
    for ln in s.split("\n"):
        if ln.lstrip().startswith("//"): out.append(ln); continue
        for a,b in pairs: ln=ln.replace(a,b)
        out.append(ln)
    s="\n".join(out)
    if s!=o: open(f,"w").write(s); print(f)
