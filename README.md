# go-ssa-atomic-store

Zenn記事へのリンク(TBD)


# タイトル: ARMアーキテクチャがメモリアクセスを省略することで意図したベンチマークになっていなかった話

## 概要
とあるサンプルコードでベンチマークを測ったところ、期待する18倍の速度が出力されました。バイナリを見るとCPU命令は1つしか違っていませんでした。調べてみると、ARMアーキテクチャは連続したメモリへの書き込みを1つにマージするため、実行が省略され正しいベンチマークが測れていませんでした。

パフォーマンスを解釈するにはアーキテクチャを理解する必要があった事例として記事にしてみました。Go言語で説明しますが、言語に依存しない話題だと思います。

## 得られた教訓
- バイナリを見ても説明できない性能差は、アーキテクチャの理解によって説明できることがある
- マイクロベンチマークは、何を計測しているのかを理解した上で実施する必要がある

## 不思議なベンチマーク結果

Go言語 100Tips「No.89: 不正確なベンチマークを書く」(p.313; [web版](https://100go.co/89-benchmarks/#making-wrong-assumptions-about-micro-benchmarks))に、以下のサンプルコードがあります。atomic.StoreInt32関数は、アトミックなストアを保証するためにメモリアクセスを行います。
```go
func BenchmarkAtomicStoreInt32(b *testing.B) {
	var v int32
	for i := 0; i < b.N; i++ {
		atomic.StoreInt32(&v, 1)
	}
}
```

Goでは、`go test -bench .`でベンチマークを取ることができます。(`ns/op`は 1op(今回の例ではatomic~の1行)あたりの実行時間です。)
```
% go test -bench .
goos: darwin
goarch: arm64
pkg: github.com/haruyama480/go-ssa-atomic-store
BenchmarkAtomicStoreInt32-10            1000000000               0.3120 ns/op
```

自分の環境では`0.3165 ns/op` という結果になりました。
しかし、書籍では`5.682 ns/op`と記載されており、18倍もの速度差がありました。

この速度差の原因は何でしょうか？
- プロセッサに違いがあるものの(書籍はIntel(AMD64)で自分の環境はApple Silicon(AArch64))、それだけで10倍以上速くなることはあるのでしょうか？
- もしかして、書籍執筆後にコンパイラが賢くなり、最適化によって命令が省略されるようになったのでしょうか？

結論として、コンパイラによる最適化は発生しておらず、ARMアーキテクチャによるメモリアクセスの高速化により、想定通りのベンチマーク結果が得られていませんでした。

## 調査
バイナリを調査することで、コンパイラの最適化の有無を確認しました。
サンプルコードは [github](https://github.com/haruyama480/go-ssa-atomic-store)に公開しています。実行環境はMBP M1 Maxです。

### ベンチマークを増やす

とりあえずベンチマークを増やして実験してみました。書籍と同等の性能になるケースを見つければ、高速化の要因を探っていけるはずです。

```go
func BenchmarkAtomicStoreInt32Add(b *testing.B) {
	var v int32
	for i := 0; i < b.N; i++ {
		atomic.StoreInt32(&v, int32(i)+1)
	}
}

func BenchmarkAtomicStoreInt32Inc(b *testing.B) {
	var v int32
	for i := 0; i < b.N; i++ {
		atomic.StoreInt32(&v, v+1)
	}
}

func BenchmarkEmpty(b *testing.B) {
	for i := 0; i < b.N; i++ {}
}
```

各テストを、Addテスト、Incテスト、Emptyテストと呼ぶことにします。
Addテストは過去の値を使わず、足し算の結果をvに代入します。
Incテストは過去のvの値を使って、足し算の結果をvに代入します。
Emptyテストは何もしませんが、他のテスト結果と比較するための指標となります。

ベンチマークの結果は次のとおりです。
```shell
BenchmarkAtomicStoreInt32-10            1000000000               0.3119 ns/op
BenchmarkAtomicStoreInt32Add-10         1000000000               0.3139 ns/op
BenchmarkAtomicStoreInt32Inc-10         202822557                5.910 ns/op
BenchmarkEmpty-10                       1000000000               0.3110 ns/op
```

過去の値を使わないAddテストは、Emptyテストと同等の結果となりました。
Incテストは書籍と似た結果になりました。

EmptyテストとAddテストの結果を見ると、最適化によって高速化されているようです。実際に計算処理に該当する命令が削られているのでしょうか？

コンパイラが生成する中間表現であるSSAと、最終的に出力されるバイナリを調査することで、詳細を確認しましょう。

### SSAの差分
Go言語はコンパイル時にコードをSSA形式に変換します。
各関数のSSAは以下のコマンドで出力できます。出力された`ssa.html`をブラウザで開くと、コードがASTに変換され最適化される過程をGUIで確認できます。
```shell
GOSSAFUNC=AtomicEmpty go build main.go
```

(GUIの例)
![](https://storage.googleapis.com/zenn-user-upload/50772c803561-20240222.png)

各ベンチマークと同等の関数を定義し、SSAを比較してみました。

```
// Empty
MOVD $0, R0
JMP 6
ADD $1, R0, R0
CMP $100, R0
BLT 5
MOVD $0, R0
RET
END

// Add
MOVD $type:int32(SB), R0
PCDATA $1, $0
CALL runtime.newobject(SB)
MOVD $0, R1
JMP 11
ADD $1, R1, R2  // 差分
STLRW R2, (R0)  //
ADD $1, R1, R1
CMP $100, R1
BLT 8
MOVW (R0), R0
RET
END

// Inc
MOVD $type:int32(SB), R0
PCDATA $1, $0
CALL runtime.newobject(SB)
MOVD $0, R1
JMP 12
MOVW (R0), R2   // 差分
ADD $1, R2, R2  //
STLRW R2, (R0)  //
ADD $1, R1, R1
CMP $100, R1
BLT 8
MOVW (R0), R0
RET
END
```

見ての通り、差分は最大でも3行でした。

意外だったのが、EmptyとAddのSSAが異なっていることです。
AddはEmptyと同等に速かったので、最適化されていれば同じSSAに変換されるという推測は間違っていたようです。

### バイナリの差分

各関数のSSAを見ても、実際にはリンクタイム最適化やインライン展開等の影響を受けさらに変換される可能性があります。

次のコマンドで出力されたバイナリを逆アセンブルします。速度差の原因となっている命令を特定していきましょう。
```shell
go build -o main main.go
otool -tvV main
```

以下は、逆アセンブル結果の抜粋です。forループの中身のみを示しています。

```
_main.main:

// AtomicStoreInt32Addのfor文の中身
add	x2, x1, #0x1
stlr    w2, [x0]
add x1, x1, #0x1

// AtomicStoreInt32Incのfor文の中身
ldrsw   x2, [x0] // 差分
add x2, x2, #0x1
stlr    w2, [x0]
add x1, x1, #0x1
...
```

差分は、`ldrsw x2, [x0]` という1命令のみのようです。
[LDRSWのドキュメント](https://developer.arm.com/documentation/ddi0602/2023-12/Base-Instructions/LDRSW--register---Load-Register-Signed-Word--register--?lang=en#XtOrXZR__11)を読んでみると、メモリからx0レジスタで指定するアドレスの値を読み取り、x2レジスタに格納する命令のようです。

メモリ読み出し命令1つだけで、18倍もの性能差が生じているようです。
### どうしてこんなにも性能差が出るのか？

ARMアーキテクチャとそのメモリアクセスの順序を理解する必要があります。

[Memory access ordering](https://developer.arm.com/documentation/102376/0100/Memory-access-ordering)というarmのドキュメントを読むと以下のような内容が書かれています。
- メモリアクセス順序(memory access ordering) と命令順序(instructions ordering)は異なる概念
- 見かけ上の命令の実行順序はSSEモデル(割愛)により保証される
- しかし、メモリアクセスの順序はライトバッファーやキャッシュメモリのようなメカニズムがあるため保証されない

さらに[Normarl Memory](https://developer.arm.com/documentation/102376/0100/Normal-memory) を読むと、以下の記載があります。(ChatGPTによる翻訳)

> コードは、複数回同じ場所にアクセスしたり、連続する複数の場所にアクセスしたりすることがあります。効率のため、プロセッサはこれらのアクセスを検出して単一のアクセスにマージすることが許可されています。例えば、ソフトウェアが変数に複数回書き込む場合、プロセッサはメモリシステムに最後の書き込みのみを提示するかもしれません。

メモリアクセスはマージされ、プロセッサにより省略されることがあるようです。
つまり、Addテストは同一アドレスへの書き込み(`stlr`)が連続していたため、それらはアーキテクチャによって省略され、最後の書き込みのみが実行される可能性があります。[^reorder]

(再掲)
```go
	for i := 0; i < b.N; i++ {
		atomic.StoreInt32(&v, 1)
	}
```

具体的には、forループでN回回していたはずのStoreInt32()が実際には最後の1回しか実行されない可能性がある、ということです。L1キャッシュへのアクセスといえど数CPUサイクルを消費してしまうので、数サイクル×Nのオーダーの省略は、マイクロベンチマークにおいて大きな速度差を生みそうです。

このようにイテレーションをまたいだ最適化が起き、実際の実行回数が本来のNではなくなり、期待以上のベンチマーク結果がでてしまったのでしょう。

### メモリアクセスの省略を検証

実際にメモリアクセスの省略が起きているかを検証してみます。
本来であれば、CPUがメモリアクセスを省略したことをログに吐いてくれればいいのですが、残念ながらそのような機能はなさそうなので実験から考察してみます。

以下のベンチマークを定義します。
```go
func BenchmarkAtomicStoreInt32(b *testing.B) {
	var v int32
	for i := 0; i < b.N; i++ {
		atomic.StoreInt32(&v, 1)
	}
}
func BenchmarkAtomicLoadInt32(b *testing.B) {
	var v int32
	for i := 0; i < b.N; i++ {
		_ = atomic.LoadInt32(&v)
	}
}
func BenchmarkAtomicStoreLoadInt32(b *testing.B) {
	var v int32
	for i := 0; i < b.N; i++ {
		atomic.StoreInt32(&v, 1)
		_ = atomic.LoadInt32(&v)
	}
}
```
- Storeテストは、メモリの書き込み
- Loadテストは、メモリの読み取り
- StoreLoadテストは、書き込みと読み取りどちらも行います

詳細は省きますが、StoreLoadのforループに該当する命令列は、StoreとLoadを足し合わせたものより小さくなります。もしStoreとLoadテストでメモリアクセスの省略が起きていれば、それが起き得ないStoreLoadの平均実行速度は、StoreとLoadの速度の和より大きくなるでしょう。[^disclaimer]

結果は以下です。
```
% go test -bench 'Int32$'
...
BenchmarkAtomicStoreInt32-10            1000000000               0.3137 ns/op
BenchmarkAtomicLoadInt32-10             1000000000               0.5355 ns/op
BenchmarkAtomicStoreLoadInt32-10        256485384                4.672 ns/op
```

想定通り StoreLoad >>> Store + Load となったので、メモリアクセスがマージされたと見てよいでしょう

[^disclaimer]: CPUアーキテクチャに精通しているわけではないので、もしかしたら他の要因があるかもしれません。


## 何が問題だったのか
意図したベンチマークが取れていなかった問題について、主に2つの観点から考察します
### イテレーション間の干渉
ほとんどのベンチマークは、ばらつきを防ぐためにforループでN回実行し、合計実行時間/Nによって平均実行時間(`ns/op`)を算出します。しかし、イテレーションをまたいだ最適化が起きると、実行が省略され、平均実行時間を過小評価してしまう可能性があります。Goはデフォルトで合計実行時間が1秒に収まるように Nを自動的に設定するため、良い環境ほどNが大きくなり、さらに過小評価につながる可能性もあります。

今回の場合、メモリアクセスするアドレスを毎回変えてあげれば、イテレーション間の干渉は解消されるでしょう。
```go
func BenchmarkAtomicStoreInt32Fixed(b *testing.B) {
	v := make([]int32, b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		atomic.StoreInt32(&v[i], 1)
	}
}
// 結果: 0.5345 ns/op
```
だたし、この例だと `v[i]`へのアクセスは連続的(シーケンシャル)なため、L1キャッシュヒット率が高い前提があります。問題設定によっては`v := make([]*int32, b.N)`とするとよさそうです。
### マイクロベンチマークが何を計測しているのかを注意深く理解する
すべてのベンチマークや一般的な評価に通じて言えることなのですが、特にマイクロベンチマークにおいて重要です。

Go言語100Tipsにも書かれている通り、マクロベンチマークは多くの要因が結果に影響を与え、間違った仮定へ導く可能性があります。今回の例も、一見するとコンパイラの最適化が要因と考えられましたが、実際はアーキテクチャの仕様でした。ベンチマークを取った後も、それは正しい計測になっているかを様々な角度で検証・考察できるとよいでしょう。
### その他
その他にも問題があげられると思います。ベンチマークの一般的な問題は『詳解システム・パフォーマンス』12章にまとまっています。

## まとめ
- 命令が1つ増えるだけで、18倍も速度が遅くなることがあった
- その速度差は、アーキテクチャとメモリアクセスの理解を必要としていた
- 具体的には、ARMアーキテクチャは同一アドレスへの連続したメモリアクセスをマージする可能性があった
- メモリアクセスの省略により本来の実行回数分の実行されておらず、平均実行速度が過小評価されていた
- ベンチマークはイテレーション間が干渉しないように書くべき
- 特にマイクロベンチマークは何を計測しているのかを注意深く理解すべき



[^reorder]: 
ちなみに、Incテストの場合、同一アドレスへの書き込みと読み込みを交互に行っているため、並び替えは発生せず、プロセッサはメモリアクセスを順序通り実行してくれるようです。
