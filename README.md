# Cosmos SDK Loan Module

CosmosSDKが提供してくれる必要なModule群を一括でIgniteCLIで用意する

## 初期設定

```:
ignite scaffold chain github.com/username/loan --no-module
```

loanに移動する

```;
cd loan
```

モジュール作成のために必要なファイルを用意する

```;
ignite scaffold module loan --dep bank
```

上のコマンドを実行することで”loan”というモジュールを作成してくれる
ユーザーカスタマイズのモジュールは”x”ディレクトリ以下に作成される
”---dep bank”オプションを利用することで”bank”モジュールを利用することになる
さらに”loan”モジュールがローンの情報を保存できるようにするため、以下のコマンドを実行する

```;
ignite scaffold list loan amount fee collateral deadline state borrower lender --no-message
```

実行後、loan/proto/loan/loan.protoというファイルに、上のコマンドで指定した要素を持った型が定義されている

```proto:
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
ignite scaffold message request-loan amount fee collateral deadline
```

x/loan/keeper/msg_server_request_loan.goにrequrest_loanの関数が設定されている(以下初期状態)

```go:
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

```go:
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

```go:
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

```go:
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
ignite chain serve

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
loand tx loan request-loan 100token 2token 200token 500 --from alice
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
loand query loan list-loan
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