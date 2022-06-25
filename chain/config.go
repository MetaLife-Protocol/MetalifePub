package chain

import "time"

// ChainCfg
type ChainCfg struct {
	Name                  string        // 链名
	BlockPeriod           time.Duration // 出块间隔
	ConfirmBlockNumber    uint64        // 事件/交易确认块数
	BlockNumberPollPeriod time.Duration // 新块查询轮询间隔
	BlockNumberLogPeriod  uint64        // 块号日志打印间隔
	RPCTimeout            time.Duration // 区块链节点RPC接口超时时间
}

// SMC Spectrum chain
var SMC *ChainCfg
