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
