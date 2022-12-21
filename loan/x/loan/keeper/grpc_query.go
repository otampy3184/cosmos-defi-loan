package keeper

import (
	"github.com/username/loan/x/loan/types"
)

var _ types.QueryServer = Keeper{}
