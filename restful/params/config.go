package params

import "time"

type ApiConfig struct {
	Host  string
	Port  int
	Debug bool
}

type LasterNumLikes struct {
	ClientID         string `json:"client_id"`
	ClientAddress    string `json:"client_address"`
	LasterAddVoteNum int64  `json:"laster_add_vote_num"`
	LasterVoteNum    int64  `json:"laster_vote_num"`
	VoteLink         string `json:"laster_add_vote_num"`
}

func NewApiServeConfig() *ApiConfig {
	return &ApiConfig{
		"0.0.0.0",
		18008,
		true,
	}

}

//var PubTcpHostAddress = "106.52.171.12:8008"
var PubTcpHostAddress = "54.179.3.93:8008"

const (
	// These are the multipliers for ether denominations.
	// Example: To get the wei value of an amount in 'douglas', use
	//
	//    new(big.Int).Mul(value, big.NewInt(params.Douglas))
	//
	Wei      = 1
	Ada      = 1e3
	Babbage  = 1e6
	Shannon  = 1e9
	Szabo    = 1e12
	Finney   = 1e15
	Ether    = 1e18
	Einstein = 1e21
	Douglas  = 1e42
)

var PhotonHost = "127.0.0.1:15001"

var PhotonAddress = "0x0D0EFCcda4f079C0dD1B728297A43eE54d7170Cd"

var TokenAddress = "0x6601F810eaF2fa749EEa10533Fd4CC23B8C791dc"

var SettleTime = 100

// MsgScanInterval 消息二轮扫描的时间间隔
var MsgScanInterval = time.Second * 15
