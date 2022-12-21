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

