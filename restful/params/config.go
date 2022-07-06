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
		ServePort,
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

var PhotonHost = "127.0.0.1:11001"

var PhotonAddress = ""

var TokenAddress = "0xA27F8f580C01Db0682Ce185209FFb84121a2F711"

var SMTTokenAddress = "0x6601F810eaF2fa749EEa10533Fd4CC23B8C791dc"

var SettleTime = 40000

var ServePort = 10008

// MsgScanInterval 消息二轮扫描的时间间隔
var MsgScanInterval = time.Second * 15

// minBalanceInchannel pub与客户端通道的最小资金，保障三方转账余额足够
var MinBalanceInchannel = 1000

// PubID this
var PubID = ""

// RewardOfReportProblematicPost default 100
var RewardOfReportProblematicPost = 100

// RewardOfSignup
var RewardOfSignup = 1000

// RewardOfSignupSMT
var RewardOfSignupSMT = 50

// RewardOfDailyLogin
var RewardOfDailyLogin = 10

// RewardOfPostMessage
var RewardOfPostMessage = 5

// RewardOfPostComment
var RewardOfPostComment = 2

// RewardOfMintNft
var RewardOfMintNft = 10

// RewardOfLikePost
var RewardOfLikePost = 1

// SensitiveWordsFilePath
var SensitiveWordsFilePath = ""

// Ip2LocationLiteDbPath
var Ip2LocationLiteDbPath = ""

// MaxDailyRewarding pub send mlt, instead of supernode
var MaxDailyRewarding = 500

// MaxSignupReward
var MaxSignupReward = 2000

//RoundTimeOfBackPay
var RoundTimeOfBackPay = time.Minute * 10

//RoundTimeOfCheckChannelBalance
var RoundTimeOfCheckChannelBalance = time.Minute * 120
