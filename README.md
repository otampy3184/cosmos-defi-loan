# Cosmos SDK Loan Module

IgniteCLIが用意しているCosmosSDK Tutorialの[Advanced Module: DeFi Loan](https://docs.ignite.com/guide/loan)を試し、IBCでやり取りされるMsgの中身やKeeper処理などの理解を深める

## 初期設定

CosmosSDKが提供してくれる必要なModule群を一括でIgniteCLIで用意する

```:
% ignite scaffold chain github.com/username/loan --no-module
```

loanに移動する

```;
% cd loan
```

モジュール作成のために必要なファイルを用意する

```;
% ignite scaffold module loan --dep bank
```

上のコマンドを実行することで”loan”というモジュールを作成してくれる
ユーザーカスタマイズのモジュールは”x”ディレクトリ以下に作成される
”---dep bank”オプションを利用することで”bank”モジュールを利用することになる
さらに”loan”モジュールがローンの情報を保存できるようにするため、以下のコマンドを実行する

```;
% ignite scaffold list loan amount fee collateral deadline state borrower lender --no-message
```

実行後、loan/proto/loan/loan.protoというファイルに、上のコマンドで指定した要素を持った型が定義されている

```proto:loan/proto/loan/loan.proto
syntax = "proto3";
package username.loan.loan;

option go_package = "github.com/username/loan/x/loan/types";

message Loan {
  uint64 id = 1;
  string amount = 2; 
  string fee = 3; 
  string collateral = 4; 
  string deadline = 5; 
  string state = 6; 
  string borrower = 7; 
  string lender = 8; 
}
```

## Request Loan Messageの設定

Borrowerがローンを作成して依頼するためのメッセージとして**Request Loan Message**を実装する
Messageはユーザーがトランザクションを実行する際に利用される
Messageの内容としては以下を用意する

* いくら借りたいかの**amount**
* Lenderへ払う手数料の**fee**
* 担保としていくら預けるかの**collateral**
* いつまでに返済を行うかの**deadline**

これらの要素を持つMessageの型をproto-buffで用意する

```;
% ignite scaffold message request-loan amount fee collateral deadline
```

x/loan/keeper/msg_server_request_loan.goにrequrest_loanの関数が設定されている(以下初期状態)

```go:x/loan/keeper/msg_server_request_loan.go
package keeper

import (
 "context"

 sdk "github.com/cosmos/cosmos-sdk/types"
 "github.com/username/loan/x/loan/types"
)

func (k msgServer) RequestLoan(goCtx context.Context, msg *types.MsgRequestLoan) (*types.MsgRequestLoanResponse, error) {
 ctx := sdk.UnwrapSDKContext(goCtx)

 // TODO: Handling the message
 _ = ctx

 return &types.MsgRequestLoanResponse{}, nil
}
```

以上の処理はBorrowerがトランザクションを発行し、request loan msgを送った際に実行される処理になる
以下のように書き換えることで、新しくLoanを作成し、トランザクションが持っているmsgから情報を受け取り、stateをRequestedにする
また、Borrowerのアドレスと担保とする資産の値を定義し、各アカウントの残高を管理しているbankモジュールに対して、borrowerの持つ資産をこのloanモジュールに対して送金させるう処理を実行させる

```go:x/loan/keeper/msg_server_request_loan.go
package keeper

import (
 "context"

 "github.com/username/loan/x/loan/types"
 sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k msgServer) RequestLoan(goCtx context.Context, msg *types.MsgRequestLoan) (*types.MsgRequestLoanResponse, error) {
 ctx := sdk.UnwrapSDKContext(goCtx)

 // Create a new Loan with the following user input
 var loan = types.Loan{
  Amount:     msg.Amount,
  Fee:        msg.Fee,
  Collateral: msg.Collateral,
  Deadline:   msg.Deadline,
  State:      "requested",
  Borrower:   msg.Creator,
 }

 // TODO: collateral has to be more than the amount (+fee?)

 // moduleAcc := sdk.AccAddress(crypto.AddressHash([]byte(types.ModuleName)))
 // Get the borrower address
 borrower, _ := sdk.AccAddressFromBech32(msg.Creator)

 // Get the collateral as sdk.Coins
 collateral, err := sdk.ParseCoinsNormalized(loan.Collateral)
 if err != nil {
  panic(err)
 }

 // Use the module account as escrow account
 sdkError := k.bankKeeper.SendCoinsFromAccountToModule(ctx, borrower, types.ModuleName, collateral)
 if sdkError != nil {
  return nil, sdkError
 }

 // Add the loan to the keeper
 k.AppendLoan(
  ctx,
  loan,
 )

 return &types.MsgRequestLoanResponse{}, nil
}
```

また、k.bankKeeperでSendCoinsFromAccountToModuleを利用しているため、KeeperのInterfaceも更新する

```go:loan/x/loan/types/expected_keepers.go
package types

import (
 sdk "github.com/cosmos/cosmos-sdk/types"
 "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// AccountKeeper defines the expected account keeper used for simulations (noalias)
type AccountKeeper interface {
 GetAccount(ctx sdk.Context, addr sdk.AccAddress) types.AccountI
 // Methods imported from account should be defined here
}

// BankKeeper defines the expected interface needed to retrieve account balances.
type BankKeeper interface {
 SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
}
```

また、Loan作成時に受け取ったMsgに問題ないかの検証を行う処理として、ValidateBasic()を更新する

```go:loan/x/loan/types/message_approve_loan.go
func (msg *MsgRequestLoan) ValidateBasic() error {
 _, err := sdk.AccAddressFromBech32(msg.Creator)

 amount, err := sdk.ParseCoinsNormalized(msg.Amount)
 fee, _ := sdk.ParseCoinsNormalized(msg.Fee)
 collateral, _ := sdk.ParseCoinsNormalized(msg.Collateral)

 if err != nil {
  return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
 }
 if !amount.IsValid() {
  return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "amount is not valid Coins object")
 }
 if amount.Empty() {
  return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "amount is empty")
 }
 if !fee.IsValid() {
  return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "fee is not a valid Coins object")
 }
 if !collateral.IsValid() {
  return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "collateral is not a valid Coins object")
 }
 if collateral.Empty() {
  return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "collateral is empty")
 }
 return nil
}
```

以上でRequest Loanのmsg処理の作成は完了したので、ルートディレクトリでChainを立ち上げる

```:
% ignite chain serve

Cosmos SDK's version is: stargate - v0.45.4

🛠️  Building proto...
📦 Installing dependencies...
🛠️  Building the blockchain...
💿 Initializing the app...
🙂 Created account "alice" with address "cosmos1zv0j5m07km934cr5xc7hvwa025dfm9dykrrflq" with mnemonic: "force rally noodle exist enemy empty pioneer buyer various autumn have mix sunny endorse chuckle famous travel common fragile balance coil vague boil pipe"
🙂 Created account "bob" with address "cosmos1ax649dnyzktn49gf97yse9f00rn0hdmhednyzv" with mnemonic: "walk level attract side issue pill drift code survey clever dry torch hammer deer change crunch heavy tornado flee coconut carpet carry absent traffic"
🌍 Tendermint node: http://0.0.0.0:26657
🌍 Blockchain API: http://0.0.0.0:1317
🌍 Token faucet: http://0.0.0.0:4500
```

別ターミナルを開き、トランザクションを送信してみる

```:
% loand tx loan request-loan 100token 2token 200token 500 --from alice
```

成功すると下記のような結果が表示される

```:
{"body":{"messages":[{"@type":"/username.loan.loan.MsgRequestLoan","creator":"cosmos1zv0j5m07km934cr5xc7hvwa025dfm9dykrrflq","amount":"100token","fee":"2token","collateral":"200token","deadline":"500"}],"memo":"","timeout_height":"0","extension_options":[],"non_critical_extension_options":[]},"auth_info":{"signer_infos":[],"fee":{"amount":[],"gas_limit":"200000","payer":"","granter":""}},"signatures":[]}

confirm transaction before signing and broadcasting [y/N]: y
code: 0
codespace: ""
data: 0A240A222F757365726E616D652E6C6F616E2E6C6F616E2E4D7367526571756573744C6F616E
events:
- attributes:
  - index: true
    key: ZmVl
    value: ""
  type: tx
- attributes:
  - index: true
    key: YWNjX3NlcQ==
    value: Y29zbW9zMXp2MGo1bTA3a205MzRjcjV4YzdodndhMDI1ZGZtOWR5a3JyZmxxLzE=
  type: tx
- attributes:
  - index: true
    key: c2lnbmF0dXJl
    value: UFEzMTZHY2czYXNLd1RDN3VTdlNJMmsvVnFQRXVkVFpQeHRwVmNFeDZwcGVyNUY4UGswckFUVjdCSnJTenFJalU2eUNoa1YxTTJYZHUwNzBlNk4yU0E9PQ==
  type: tx
- attributes:
  - index: true
    key: YWN0aW9u
    value: cmVxdWVzdF9sb2Fu
  type: message
- attributes:
  - index: true
    key: c3BlbmRlcg==
    value: Y29zbW9zMXp2MGo1bTA3a205MzRjcjV4YzdodndhMDI1ZGZtOWR5a3JyZmxx
  - index: true
    key: YW1vdW50
    value: MjAwdG9rZW4=
  type: coin_spent
- attributes:
  - index: true
    key: cmVjZWl2ZXI=
    value: Y29zbW9zMWd1NG03OXlqOGNoOGVtN2MyMnZ6dDNxcGFyZzY5eW1tNzVxZjZs
  - index: true
    key: YW1vdW50
    value: MjAwdG9rZW4=
  type: coin_received
- attributes:
  - index: true
    key: cmVjaXBpZW50
    value: Y29zbW9zMWd1NG03OXlqOGNoOGVtN2MyMnZ6dDNxcGFyZzY5eW1tNzVxZjZs
  - index: true
    key: c2VuZGVy
    value: Y29zbW9zMXp2MGo1bTA3a205MzRjcjV4YzdodndhMDI1ZGZtOWR5a3JyZmxx
  - index: true
    key: YW1vdW50
    value: MjAwdG9rZW4=
  type: transfer
- attributes:
  - index: true
    key: c2VuZGVy
    value: Y29zbW9zMXp2MGo1bTA3a205MzRjcjV4YzdodndhMDI1ZGZtOWR5a3JyZmxx
  type: message
gas_used: "71449"
gas_wanted: "200000"
height: "50"
info: ""
logs:
- events:
  - attributes:
    - key: receiver
      value: cosmos1gu4m79yj8ch8em7c22vzt3qparg69ymm75qf6l
    - key: amount
      value: 200token
    type: coin_received
  - attributes:
    - key: spender
      value: cosmos1zv0j5m07km934cr5xc7hvwa025dfm9dykrrflq
    - key: amount
      value: 200token
    type: coin_spent
  - attributes:
    - key: action
      value: request_loan
    - key: sender
      value: cosmos1zv0j5m07km934cr5xc7hvwa025dfm9dykrrflq
    type: message
  - attributes:
    - key: recipient
      value: cosmos1gu4m79yj8ch8em7c22vzt3qparg69ymm75qf6l
    - key: sender
      value: cosmos1zv0j5m07km934cr5xc7hvwa025dfm9dykrrflq
    - key: amount
      value: 200token
    type: transfer
  log: ""
  msg_index: 0
raw_log: '[{"events":[{"type":"coin_received","attributes":[{"key":"receiver","value":"cosmos1gu4m79yj8ch8em7c22vzt3qparg69ymm75qf6l"},{"key":"amount","value":"200token"}]},{"type":"coin_spent","attributes":[{"key":"spender","value":"cosmos1zv0j5m07km934cr5xc7hvwa025dfm9dykrrflq"},{"key":"amount","value":"200token"}]},{"type":"message","attributes":[{"key":"action","value":"request_loan"},{"key":"sender","value":"cosmos1zv0j5m07km934cr5xc7hvwa025dfm9dykrrflq"}]},{"type":"transfer","attributes":[{"key":"recipient","value":"cosmos1gu4m79yj8ch8em7c22vzt3qparg69ymm75qf6l"},{"key":"sender","value":"cosmos1zv0j5m07km934cr5xc7hvwa025dfm9dykrrflq"},{"key":"amount","value":"200token"}]}]}]'
timestamp: ""
tx: null
txhash: 47CEE7C04F600EB922CC8DB7D1FDB1C5B5B8EEA342A3D569F4EC7557283A13A3
```

さらに下記コマンドを実行することで作成したLoanの内容を確認できる

```:
% loand query loan list-loan
Loan:
- amount: 100token
  borrower: cosmos1zv0j5m07km934cr5xc7hvwa025dfm9dykrrflq
  collateral: 200token
  deadline: "500"
  fee: 2token
  id: "0"
  lender: ""
  state: requested
pagination:
  next_key: null
  total: "0"
```

## Approve loan

続いて作成したLoanをLender側が承認する処理を作成する
igniteCLIにApproveLoan用のMsgを作成してもらう

```:
% ignite scaffold message approve-loan id:uint
```

loanモジュールがApproveLoanMsgを受け取った時の処理をKeeper内に記述していく

```go:x/loan/keeper/msg_server_approve_loan.go
package keeper

import (
 "context"
 "fmt"

 "github.com/username/loan/x/loan/types"
 sdk "github.com/cosmos/cosmos-sdk/types"
 sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (k msgServer) ApproveLoan(goCtx context.Context, msg *types.MsgApproveLoan) (*types.MsgApproveLoanResponse, error) {
 ctx := sdk.UnwrapSDKContext(goCtx)

 loan, found := k.GetLoan(ctx, msg.Id)
 if !found {
  return nil, sdkerrors.Wrapf(sdkerrors.ErrKeyNotFound, "key %d doesn't exist", msg.Id)
 }

 // TODO: for some reason the error doesn't get printed to the terminal
 if loan.State != "requested" {
  return nil, sdkerrors.Wrapf(types.ErrWrongLoanState, "%v", loan.State)
 }

 lender, _ := sdk.AccAddressFromBech32(msg.Creator)
 borrower, _ := sdk.AccAddressFromBech32(loan.Borrower)
 amount, err := sdk.ParseCoinsNormalized(loan.Amount)
 if err != nil {
  return nil, sdkerrors.Wrap(types.ErrWrongLoanState, "Cannot parse coins in loan amount")
 }

 k.bankKeeper.SendCoins(ctx, lender, borrower, amount)

 loan.Lender = msg.Creator
 loan.State = "approved"

 k.SetLoan(ctx, loan)

 return &types.MsgApproveLoanResponse{}, nil
}
```

BankModuleと繋げるため、BankKeeperにも必要なInterfaceを追加しておく

```go:expected_keeper.go
type BankKeeper interface {
 SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
    // SendCoins(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule sdk.AccAddress, amt sdk.Coins) error
    // Tutorialには↑で書かれていたが、渡す引数からして↓が正しい？
 SendCoins(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule sdk.AccAddress, amt sdk.Coins) error
}
```

また、Keeperで使っているエラーを使うため、typesのerror.goも編集する

```go:error.go
package types

// DONTCOVER

import (
 sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// x/loan module sentinel errors
var (
 ErrSample = sdkerrors.Register(ModuleName, 1100, "sample error")
)

var (
 ErrWrongLoanState = sdkerrors.Register(ModuleName, 1, "wrong loan state error")
)
```

Request Loanで作った情報を一旦リセットするため-rオプションをつけてchainを立ち上げる

```:
% ignite chain serve -r
```

改めてLoanを作成し、確認

```:
% loand tx loan request-loan 100token 2token 200token 500 --from bob -y
~~~~
% loand query loan list-loan
~~("requested"のLoanが一つある)~~
```

続いて、AliceがLoanを承認するMsgを送信する

```:
% loand tx loan approve-loan 0 --from alice -y
code: 0
codespace: ""
data: 0A240A222F757365726E616D652E6C6F616E2E6C6F616E2E4D7367417070726F76654C6F616E
events:
- attributes:
  - index: true
    key: ZmVl
    value: ""
  type: tx
- attributes:
  - index: true
    key: YWNjX3NlcQ==
    value: Y29zbW9zMXJqNXdmbDhuY3NrcXk1bDlscXJxMzk5eTB1cnBxd3lqc2QydDNjLzE=
  type: tx
- attributes:
  - index: true
    key: c2lnbmF0dXJl
    value: ckhLVWFuOERxSkRzcHd1NTYwa2E1Tk1PRmJhUUhRcExFWGN0d1IyTjZGOXpXTUtYclI4QWtaQzJKUmRIRGJtNzRqRG1PVHNwQkorb0dSdUFMKzdVZlE9PQ==
  type: tx
- attributes:
  - index: true
    key: YWN0aW9u
    value: YXBwcm92ZV9sb2Fu
  type: message
- attributes:
  - index: true
    key: c3BlbmRlcg==
    value: Y29zbW9zMXJqNXdmbDhuY3NrcXk1bDlscXJxMzk5eTB1cnBxd3lqc2QydDNj
  - index: true
    key: YW1vdW50
    value: MTAwdG9rZW4=
  type: coin_spent
- attributes:
  - index: true
    key: cmVjZWl2ZXI=
    value: Y29zbW9zMWFuNmt2a3M5bmE2NjdlNGVjYzJ6M3Q3NTlsc2t4dXRxNDdtbTlj
  - index: true
    key: YW1vdW50
    value: MTAwdG9rZW4=
  type: coin_received
- attributes:
  - index: true
    key: cmVjaXBpZW50
    value: Y29zbW9zMWFuNmt2a3M5bmE2NjdlNGVjYzJ6M3Q3NTlsc2t4dXRxNDdtbTlj
  - index: true
    key: c2VuZGVy
    value: Y29zbW9zMXJqNXdmbDhuY3NrcXk1bDlscXJxMzk5eTB1cnBxd3lqc2QydDNj
  - index: true
    key: YW1vdW50
    value: MTAwdG9rZW4=
  type: transfer
- attributes:
  - index: true
    key: c2VuZGVy
    value: Y29zbW9zMXJqNXdmbDhuY3NrcXk1bDlscXJxMzk5eTB1cnBxd3lqc2QydDNj
  type: message
gas_used: "59285"
gas_wanted: "200000"
height: "13"
info: ""
logs:
- events:
  - attributes:
    - key: receiver
      value: cosmos1an6kvks9na667e4ecc2z3t759lskxutq47mm9c
    - key: amount
      value: 100token
    type: coin_received
  - attributes:
    - key: spender
      value: cosmos1rj5wfl8ncskqy5l9lqrq399y0urpqwyjsd2t3c
    - key: amount
      value: 100token
    type: coin_spent
  - attributes:
    - key: action
      value: approve_loan
    - key: sender
      value: cosmos1rj5wfl8ncskqy5l9lqrq399y0urpqwyjsd2t3c
    type: message
  - attributes:
    - key: recipient
      value: cosmos1an6kvks9na667e4ecc2z3t759lskxutq47mm9c
    - key: sender
      value: cosmos1rj5wfl8ncskqy5l9lqrq399y0urpqwyjsd2t3c
    - key: amount
      value: 100token
    type: transfer
  log: ""
  msg_index: 0
raw_log: '[{"events":[{"type":"coin_received","attributes":[{"key":"receiver","value":"cosmos1an6kvks9na667e4ecc2z3t759lskxutq47mm9c"},{"key":"amount","value":"100token"}]},{"type":"coin_spent","attributes":[{"key":"spender","value":"cosmos1rj5wfl8ncskqy5l9lqrq399y0urpqwyjsd2t3c"},{"key":"amount","value":"100token"}]},{"type":"message","attributes":[{"key":"action","value":"approve_loan"},{"key":"sender","value":"cosmos1rj5wfl8ncskqy5l9lqrq399y0urpqwyjsd2t3c"}]},{"type":"transfer","attributes":[{"key":"recipient","value":"cosmos1an6kvks9na667e4ecc2z3t759lskxutq47mm9c"},{"key":"sender","value":"cosmos1rj5wfl8ncskqy5l9lqrq399y0urpqwyjsd2t3c"},{"key":"amount","value":"100token"}]}]}]'
timestamp: ""
tx: null
txhash: 88018E09620B78E56D85FB82D3C602C75814B41E4104B6D6F025AC57634FFEDC
```

再度クエリして確認すると、LoanのStateが”approved”になっている

```:
% loand query loan list-loan 
Loan:
- amount: 100token
  borrower: cosmos1an6kvks9na667e4ecc2z3t759lskxutq47mm9c
  collateral: 200token
  deadline: "500"
  fee: 2token
  id: "0"
  lender: cosmos1rj5wfl8ncskqy5l9lqrq399y0urpqwyjsd2t3c
  state: approved
pagination:
  next_key: null
  total: "0"
```

以上でLenderがLoanに承認を行う部分は完了

## Repay loan

BorrowerがLoanの返済を行い、担保を返してもらう部分の処理

Msgを受け取った後のKeeper処理を更新していく(msg_server_repay_loan.go)

処理の中身としては、①BorrwerがLenderにLoan返済を行う、②BorrowerがLenderに手数料を払う、③LenderがBorrowerに担保を返す

```go:msg_server_repay_loan.go
package keeper

import (
 "context"
 "fmt"

 "github.com/username/loan/x/loan/types"
 sdk "github.com/cosmos/cosmos-sdk/types"
 sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (k msgServer) RepayLoan(goCtx context.Context, msg *types.MsgRepayLoan) (*types.MsgRepayLoanResponse, error) {
 ctx := sdk.UnwrapSDKContext(goCtx)

 loan, found := k.GetLoan(ctx, msg.Id)
 if !found {
  return nil, sdkerrors.Wrap(sdkerrors.ErrKeyNotFound, fmt.Sprintf("key %d doesn't exist", msg.Id))
 }

 if loan.State != "approved" {
  return nil, sdkerrors.Wrapf(types.ErrWrongLoanState, "%v", loan.State)
 }

 lender, _ := sdk.AccAddressFromBech32(loan.Lender)
 borrower, _ := sdk.AccAddressFromBech32(loan.Borrower)

 if msg.Creator != loan.Borrower {
  return nil, sdkerrors.Wrap(sdkerrors.ErrUnauthorized, "Cannot repay: not the borrower")
 }

 amount, _ := sdk.ParseCoinsNormalized(loan.Amount)
 fee, _ := sdk.ParseCoinsNormalized(loan.Fee)
 collateral, _ := sdk.ParseCoinsNormalized(loan.Collateral)

 err := k.bankKeeper.SendCoins(ctx, borrower, lender, amount)
 if err != nil {
  return nil, sdkerrors.Wrap(types.ErrWrongLoanState, "Cannot send coins")
 }
 err = k.bankKeeper.SendCoins(ctx, borrower, lender, fee)
 if err != nil {
  return nil, sdkerrors.Wrap(types.ErrWrongLoanState, "Cannot send coins")
 }
 err = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, borrower, collateral)
 if err != nil {
  return nil, sdkerrors.Wrap(types.ErrWrongLoanState, "Cannot send coins")
 }

 loan.State = "repayed"

 k.SetLoan(ctx, loan)

 return &types.MsgRepayLoanResponse{}, nil
}
```

chainを起動し、取引を確認する

```:
% ignite chain serve -r
~~~~
% loand tx loan request-loan 100token 2token 200token 500 --from bob -y
~~~~
% loand query loan list-loan
~~~~
% loand tx loan approve-loan 0 --from alice -y
~~~~
% loand query bank balances <alice_address>
~~~~
% loand tx loan repay-loan 0 --from bob -y
code: 0
codespace: ""
data: 0A220A202F757365726E616D652E6C6F616E2E6C6F616E2E4D736752657061794C6F616E
events:
- attributes:
  - index: true
    key: ZmVl
    value: ""
  type: tx
- attributes:
  - index: true
    key: YWNjX3NlcQ==
    value: Y29zbW9zMXVjN2tkeDVhOHFldWR3a2RrdDJnejI3c2Rja2VnOWVqbmQzMDgwLzE=
  type: tx
- attributes:
  - index: true
    key: c2lnbmF0dXJl
    value: cTQ4VEtCVmh0RjBhOEpBaE92dmM4RXNFaEtWZ3VJYjhBd2loTUtQc1ZQQVdBb0FMVFp5TG9wejJyY24wQzNSeUNpWko4OFpPYjNZaTV4T2RYUUMwREE9PQ==
  type: tx
- attributes:
  - index: true
    key: YWN0aW9u
    value: cmVwYXlfbG9hbg==
  type: message
- attributes:
  - index: true
    key: c3BlbmRlcg==
    value: Y29zbW9zMXVjN2tkeDVhOHFldWR3a2RrdDJnejI3c2Rja2VnOWVqbmQzMDgw
  - index: true
    key: YW1vdW50
    value: MTAwdG9rZW4=
  type: coin_spent
- attributes:
  - index: true
    key: cmVjZWl2ZXI=
    value: Y29zbW9zMXVldmdrODlmOXJlbXg3ZzRydnEwdHBjcWUwNGZheXFtMnB4czB4
  - index: true
    key: YW1vdW50
    value: MTAwdG9rZW4=
  type: coin_received
- attributes:
  - index: true
    key: cmVjaXBpZW50
    value: Y29zbW9zMXVldmdrODlmOXJlbXg3ZzRydnEwdHBjcWUwNGZheXFtMnB4czB4
  - index: true
    key: c2VuZGVy
    value: Y29zbW9zMXVjN2tkeDVhOHFldWR3a2RrdDJnejI3c2Rja2VnOWVqbmQzMDgw
  - index: true
    key: YW1vdW50
    value: MTAwdG9rZW4=
  type: transfer
- attributes:
  - index: true
    key: c2VuZGVy
    value: Y29zbW9zMXVjN2tkeDVhOHFldWR3a2RrdDJnejI3c2Rja2VnOWVqbmQzMDgw
  type: message
- attributes:
  - index: true
    key: c3BlbmRlcg==
    value: Y29zbW9zMXVjN2tkeDVhOHFldWR3a2RrdDJnejI3c2Rja2VnOWVqbmQzMDgw
  - index: true
    key: YW1vdW50
    value: MnRva2Vu
  type: coin_spent
- attributes:
  - index: true
    key: cmVjZWl2ZXI=
    value: Y29zbW9zMXVldmdrODlmOXJlbXg3ZzRydnEwdHBjcWUwNGZheXFtMnB4czB4
  - index: true
    key: YW1vdW50
    value: MnRva2Vu
  type: coin_received
- attributes:
  - index: true
    key: cmVjaXBpZW50
    value: Y29zbW9zMXVldmdrODlmOXJlbXg3ZzRydnEwdHBjcWUwNGZheXFtMnB4czB4
  - index: true
    key: c2VuZGVy
    value: Y29zbW9zMXVjN2tkeDVhOHFldWR3a2RrdDJnejI3c2Rja2VnOWVqbmQzMDgw
  - index: true
    key: YW1vdW50
    value: MnRva2Vu
  type: transfer
- attributes:
  - index: true
    key: c2VuZGVy
    value: Y29zbW9zMXVjN2tkeDVhOHFldWR3a2RrdDJnejI3c2Rja2VnOWVqbmQzMDgw
  type: message
- attributes:
  - index: true
    key: c3BlbmRlcg==
    value: Y29zbW9zMWd1NG03OXlqOGNoOGVtN2MyMnZ6dDNxcGFyZzY5eW1tNzVxZjZs
  - index: true
    key: YW1vdW50
    value: MjAwdG9rZW4=
  type: coin_spent
- attributes:
  - index: true
    key: cmVjZWl2ZXI=
    value: Y29zbW9zMXVjN2tkeDVhOHFldWR3a2RrdDJnejI3c2Rja2VnOWVqbmQzMDgw
  - index: true
    key: YW1vdW50
    value: MjAwdG9rZW4=
  type: coin_received
- attributes:
  - index: true
    key: cmVjaXBpZW50
    value: Y29zbW9zMXVjN2tkeDVhOHFldWR3a2RrdDJnejI3c2Rja2VnOWVqbmQzMDgw
  - index: true
    key: c2VuZGVy
    value: Y29zbW9zMWd1NG03OXlqOGNoOGVtN2MyMnZ6dDNxcGFyZzY5eW1tNzVxZjZs
  - index: true
    key: YW1vdW50
    value: MjAwdG9rZW4=
  type: transfer
- attributes:
  - index: true
    key: c2VuZGVy
    value: Y29zbW9zMWd1NG03OXlqOGNoOGVtN2MyMnZ6dDNxcGFyZzY5eW1tNzVxZjZs
  type: message
gas_used: "79542"
gas_wanted: "200000"
height: "96"
info: ""
logs:
- events:
  - attributes:
    - key: receiver
      value: cosmos1uevgk89f9remx7g4rvq0tpcqe04fayqm2pxs0x
    - key: amount
      value: 100token
    - key: receiver
      value: cosmos1uevgk89f9remx7g4rvq0tpcqe04fayqm2pxs0x
    - key: amount
      value: 2token
    - key: receiver
      value: cosmos1uc7kdx5a8qeudwkdkt2gz27sdckeg9ejnd3080
    - key: amount
      value: 200token
    type: coin_received
  - attributes:
    - key: spender
      value: cosmos1uc7kdx5a8qeudwkdkt2gz27sdckeg9ejnd3080
    - key: amount
      value: 100token
    - key: spender
      value: cosmos1uc7kdx5a8qeudwkdkt2gz27sdckeg9ejnd3080
    - key: amount
      value: 2token
    - key: spender
      value: cosmos1gu4m79yj8ch8em7c22vzt3qparg69ymm75qf6l
    - key: amount
      value: 200token
    type: coin_spent
  - attributes:
    - key: action
      value: repay_loan
    - key: sender
      value: cosmos1uc7kdx5a8qeudwkdkt2gz27sdckeg9ejnd3080
    - key: sender
      value: cosmos1uc7kdx5a8qeudwkdkt2gz27sdckeg9ejnd3080
    - key: sender
      value: cosmos1gu4m79yj8ch8em7c22vzt3qparg69ymm75qf6l
    type: message
  - attributes:
    - key: recipient
      value: cosmos1uevgk89f9remx7g4rvq0tpcqe04fayqm2pxs0x
    - key: sender
      value: cosmos1uc7kdx5a8qeudwkdkt2gz27sdckeg9ejnd3080
    - key: amount
      value: 100token
    - key: recipient
      value: cosmos1uevgk89f9remx7g4rvq0tpcqe04fayqm2pxs0x
    - key: sender
      value: cosmos1uc7kdx5a8qeudwkdkt2gz27sdckeg9ejnd3080
    - key: amount
      value: 2token
    - key: recipient
      value: cosmos1uc7kdx5a8qeudwkdkt2gz27sdckeg9ejnd3080
    - key: sender
      value: cosmos1gu4m79yj8ch8em7c22vzt3qparg69ymm75qf6l
    - key: amount
      value: 200token
    type: transfer
  log: ""
  msg_index: 0
raw_log: '[{"events":[{"type":"coin_received","attributes":[{"key":"receiver","value":"cosmos1uevgk89f9remx7g4rvq0tpcqe04fayqm2pxs0x"},{"key":"amount","value":"100token"},{"key":"receiver","value":"cosmos1uevgk89f9remx7g4rvq0tpcqe04fayqm2pxs0x"},{"key":"amount","value":"2token"},{"key":"receiver","value":"cosmos1uc7kdx5a8qeudwkdkt2gz27sdckeg9ejnd3080"},{"key":"amount","value":"200token"}]},{"type":"coin_spent","attributes":[{"key":"spender","value":"cosmos1uc7kdx5a8qeudwkdkt2gz27sdckeg9ejnd3080"},{"key":"amount","value":"100token"},{"key":"spender","value":"cosmos1uc7kdx5a8qeudwkdkt2gz27sdckeg9ejnd3080"},{"key":"amount","value":"2token"},{"key":"spender","value":"cosmos1gu4m79yj8ch8em7c22vzt3qparg69ymm75qf6l"},{"key":"amount","value":"200token"}]},{"type":"message","attributes":[{"key":"action","value":"repay_loan"},{"key":"sender","value":"cosmos1uc7kdx5a8qeudwkdkt2gz27sdckeg9ejnd3080"},{"key":"sender","value":"cosmos1uc7kdx5a8qeudwkdkt2gz27sdckeg9ejnd3080"},{"key":"sender","value":"cosmos1gu4m79yj8ch8em7c22vzt3qparg69ymm75qf6l"}]},{"type":"transfer","attributes":[{"key":"recipient","value":"cosmos1uevgk89f9remx7g4rvq0tpcqe04fayqm2pxs0x"},{"key":"sender","value":"cosmos1uc7kdx5a8qeudwkdkt2gz27sdckeg9ejnd3080"},{"key":"amount","value":"100token"},{"key":"recipient","value":"cosmos1uevgk89f9remx7g4rvq0tpcqe04fayqm2pxs0x"},{"key":"sender","value":"cosmos1uc7kdx5a8qeudwkdkt2gz27sdckeg9ejnd3080"},{"key":"amount","value":"2token"},{"key":"recipient","value":"cosmos1uc7kdx5a8qeudwkdkt2gz27sdckeg9ejnd3080"},{"key":"sender","value":"cosmos1gu4m79yj8ch8em7c22vzt3qparg69ymm75qf6l"},{"key":"amount","value":"200token"}]}]}]'
timestamp: ""
tx: null
txhash: 39F86A4287341D2A3496527EC548A30807D6787071EE0082381BAA6AD78FD38D
% loand query loan list-loan
balances:
- amount: "100000000"
  denom: stake
- amount: "20002"  <=== tokenのamountが元に戻っているだけでなく、手数料分も得ている
  denom: token
pagination:
  next_key: null
  total: "0"
```

以上でLoan返済処理は完了

## Liquidate loan

Lenderが貸し付けたLoanに対して、Borrowerが期限内にRepayを行わなかった際に実行する清算処理を実装する

Scaffoldingする

```:
% ignite scaffold message liquidate-loan id:uint
```

LenderからのMsgを受け取った後の処理をKeeperに実装していく(msg_server_liquidate_loan.go)

```go:msg_server_liquidate_loan.go
package keeper

import (
 "context"
 "fmt"
 "strconv"

 "github.com/username/loan/x/loan/types"
 sdk "github.com/cosmos/cosmos-sdk/types"
 sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (k msgServer) LiquidateLoan(goCtx context.Context, msg *types.MsgLiquidateLoan) (*types.MsgLiquidateLoanResponse, error) {
 ctx := sdk.UnwrapSDKContext(goCtx)

 loan, found := k.GetLoan(ctx, msg.Id)
 if !found {
  return nil, sdkerrors.Wrap(sdkerrors.ErrKeyNotFound, fmt.Sprintf("key %d doesn't exist", msg.Id))
 }

 if loan.Lender != msg.Creator {
  return nil, sdkerrors.Wrap(sdkerrors.ErrUnauthorized, "Cannot liquidate: not the lender")
 }

 if loan.State != "approved" {
  return nil, sdkerrors.Wrapf(types.ErrWrongLoanState, "%v", loan.State)
 }

 lender, _ := sdk.AccAddressFromBech32(loan.Lender)
 collateral, _ := sdk.ParseCoinsNormalized(loan.Collateral)

 deadline, err := strconv.ParseInt(loan.Deadline, 10, 64)
 if err != nil {
  panic(err)
 }

 if ctx.BlockHeight() < deadline {
  return nil, sdkerrors.Wrap(types.ErrDeadline, "Cannot liquidate before deadline")
 }

 k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, lender, collateral)

 loan.State = "liquidated"

 k.SetLoan(ctx, loan)

 return &types.MsgLiquidateLoanResponse{}, nil
}
```

期限切れが起きていないLoanに対してMsgを送った時に発するError処理も追記しておく

```go:
package types

// DONTCOVER

import (
 sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// x/loan module sentinel errors
var (
 ErrSample = sdkerrors.Register(ModuleName, 1100, "sample error")
)

var (
 ErrWrongLoanState = sdkerrors.Register(ModuleName, 1, "wrong loan state error")
 ErrDeadline       = sdkerrors.Register(ModuleName, 2, "deadline")
)

```

chainを起動し、期限切れになるLoanを作成する(期限であるBlockheightを1にする)

```:
% ignite chain serve -r
~~~~
% loand tx loan request-loan 100token 2token 200token 1 --from bob -y
~~~~
%　loand query loan list-loan
~~~~
%　loand tx loan approve-loan 0 --from alice -y
~~~~
%　loand tx loan liquidate-loan 0 --from alice -y
code: 0
codespace: ""
data: 0A260A242F757365726E616D652E6C6F616E2E6C6F616E2E4D73674C69717569646174654C6F616E
events:
- attributes:
  - index: true
    key: ZmVl
    value: ""
  type: tx
- attributes:
  - index: true
    key: YWNjX3NlcQ==
    value: Y29zbW9zMTI1Mmw3ZndxN3I0ZGwweGVxbHhtMzZhOXNlNXljOXN4czdrbm11LzI=
  type: tx
- attributes:
  - index: true
    key: c2lnbmF0dXJl
    value: L01ZOXM1N2NEUTdlcGlvT213VGp5VE44Y1czVlExbUhjVXgvZjVNRHJqcG9nZmVFMjNSRmZiYVM0VUEzUkV5VjR3b29BeU5PcURjT1dDWFNRbVNyV2c9PQ==
  type: tx
- attributes:
  - index: true
    key: YWN0aW9u
    value: bGlxdWlkYXRlX2xvYW4=
  type: message
- attributes:
  - index: true
    key: c3BlbmRlcg==
    value: Y29zbW9zMWd1NG03OXlqOGNoOGVtN2MyMnZ6dDNxcGFyZzY5eW1tNzVxZjZs
  - index: true
    key: YW1vdW50
    value: MjAwdG9rZW4=
  type: coin_spent
- attributes:
  - index: true
    key: cmVjZWl2ZXI=
    value: Y29zbW9zMTI1Mmw3ZndxN3I0ZGwweGVxbHhtMzZhOXNlNXljOXN4czdrbm11
  - index: true
    key: YW1vdW50
    value: MjAwdG9rZW4=
  type: coin_received
- attributes:
  - index: true
    key: cmVjaXBpZW50
    value: Y29zbW9zMTI1Mmw3ZndxN3I0ZGwweGVxbHhtMzZhOXNlNXljOXN4czdrbm11
  - index: true
    key: c2VuZGVy
    value: Y29zbW9zMWd1NG03OXlqOGNoOGVtN2MyMnZ6dDNxcGFyZzY5eW1tNzVxZjZs
  - index: true
    key: YW1vdW50
    value: MjAwdG9rZW4=
  type: transfer
- attributes:
  - index: true
    key: c2VuZGVy
    value: Y29zbW9zMWd1NG03OXlqOGNoOGVtN2MyMnZ6dDNxcGFyZzY5eW1tNzVxZjZs
  type: message
gas_used: "57123"
gas_wanted: "200000"
height: "72"
info: ""
logs:
- events:
  - attributes:
    - key: receiver
      value: cosmos1252l7fwq7r4dl0xeqlxm36a9se5yc9sxs7knmu
    - key: amount
      value: 200token
    type: coin_received
  - attributes:
    - key: spender
      value: cosmos1gu4m79yj8ch8em7c22vzt3qparg69ymm75qf6l
    - key: amount
      value: 200token
    type: coin_spent
  - attributes:
    - key: action
      value: liquidate_loan
    - key: sender
      value: cosmos1gu4m79yj8ch8em7c22vzt3qparg69ymm75qf6l
    type: message
  - attributes:
    - key: recipient
      value: cosmos1252l7fwq7r4dl0xeqlxm36a9se5yc9sxs7knmu
    - key: sender
      value: cosmos1gu4m79yj8ch8em7c22vzt3qparg69ymm75qf6l
    - key: amount
      value: 200token
    type: transfer
  log: ""
  msg_index: 0
raw_log: '[{"events":[{"type":"coin_received","attributes":[{"key":"receiver","value":"cosmos1252l7fwq7r4dl0xeqlxm36a9se5yc9sxs7knmu"},{"key":"amount","value":"200token"}]},{"type":"coin_spent","attributes":[{"key":"spender","value":"cosmos1gu4m79yj8ch8em7c22vzt3qparg69ymm75qf6l"},{"key":"amount","value":"200token"}]},{"type":"message","attributes":[{"key":"action","value":"liquidate_loan"},{"key":"sender","value":"cosmos1gu4m79yj8ch8em7c22vzt3qparg69ymm75qf6l"}]},{"type":"transfer","attributes":[{"key":"recipient","value":"cosmos1252l7fwq7r4dl0xeqlxm36a9se5yc9sxs7knmu"},{"key":"sender","value":"cosmos1gu4m79yj8ch8em7c22vzt3qparg69ymm75qf6l"},{"key":"amount","value":"200token"}]}]}]'
timestamp: ""
tx: null
txhash: 5C67C2CC939963D34211C8C7B1220B20846A6F651868F0966CD598303331411E
% loand query bank balances cosmos1252l7fwq7r4dl0xeqlxm36a9se5yc9sxs7knmu
balances:
- amount: "100000000"
  denom: stake
- amount: "20100"  <= 担保になっていた200tokenが入金されている
  denom: token
pagination:
  next_key: null
  total: "0"
```

以上で清算処理は完了

## Cancel loan

作成したLoanを、誰かがApproveする前にキャンセルする処理

Scaffoldingする

```:
% ignite s message cancel-loan id:uint
```

Keeper処理を作成する(msg_server_cancel_loan)

注意点として、LoanをキャンセルできるのはLoanを作成したBorrowerだけに限定する

```go:msg_server_cancel_loan.go
package keeper

import (
 "context"
 "fmt"

 "github.com/username/loan/x/loan/types"
 sdk "github.com/cosmos/cosmos-sdk/types"
 sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (k msgServer) CancelLoan(goCtx context.Context, msg *types.MsgCancelLoan) (*types.MsgCancelLoanResponse, error) {
 ctx := sdk.UnwrapSDKContext(goCtx)

 loan, found := k.GetLoan(ctx, msg.Id)
 if !found {
  return nil, sdkerrors.Wrap(sdkerrors.ErrKeyNotFound, fmt.Sprintf("key %d doesn't exist", msg.Id))
 }

 if loan.Borrower != msg.Creator {
  return nil, sdkerrors.Wrap(sdkerrors.ErrUnauthorized, "Cannot cancel: not the borrower")
 }

 if loan.State != "requested" {
  return nil, sdkerrors.Wrapf(types.ErrWrongLoanState, "%v", loan.State)
 }

 borrower, _ := sdk.AccAddressFromBech32(loan.Borrower)
 collateral, _ := sdk.ParseCoinsNormalized(loan.Collateral)
 k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, borrower, collateral)

 loan.State = "cancelled"

 k.SetLoan(ctx, loan)

 return &types.MsgCancelLoanResponse{}, nil
}
```

以上で実装は完了
