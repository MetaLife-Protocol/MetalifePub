package restful

import (
	"encoding/json"
	"time"

	refs "go.mindeco.de/ssb-refs"
)

type MessageValue struct {
	Previous  *refs.MessageRef `json:"previous"`
	Sequence  int64            `json:"sequence"`
	Author    refs.FeedRef     `json:"author"`
	Timestamp float64          `json:"timestamp"`
	Hash      string           `json:"hash"`
	Content   json.RawMessage  `json:"content"`
	Signature string           `json:"signature"`
}

type DeserializedMessageStu struct {
	Key       string        `json:"key"`
	Value     *MessageValue `json:"value"`
	Timestamp float64       `json:"timestamp"`
}

// LikeDetail 存储一轮搜索到的被Like的消息ID
var LikeDetail []string

// LikeDetail 存储一轮搜索到的被Unlike的消息ID
var UnLikeDetail []string

// LikeCount for save message for search likes's author
var TempMsgMap map[string]*TempdMessage

// ClientID2Name for save message for search likes's author clientid---name
var ClientID2Name map[string]string

func PrintTime() string {
	return "[" + time.Now().Format("2006-01-02 15:04:05") + "] "
}

// ContentVoteStru
type ContentVoteStru struct {
	Type string    `json:"type"`
	Vote *VoteStru `json:"vote"`
}

// ContentContactStru
type ContentContactStru struct {
	Type      string `json:"type"`
	Contact   string `json:"contact"`
	Following bool   `json:"following"`
	//Blocking  bool   `json:"blocking"`
	Pub bool `json:"pub"`
}

// TippedOff reasons:"xxx|xxx|xxx"
type TippedOffStu struct {
	Plaintiff  string `json:"plaintiff"`
	Defendant  string `json:"defendant"`
	MessageKey string `json:"messagekey"`
	Reasons    string `json:"reasons"`
	DealTag    string `json:"dealtag"`
	Recordtime int64  `json:"recordtime"`
	Dealtime   int64  `json:"dealtime"`
	Dealreward string `json:"dealreward"`
}

// VoteStru
type VoteStru struct {
	Link       string `json:"link"`
	value      int    `json:"value"`
	Expression string `json:"expression"`
}

// ContentAboutStru
type ContentAboutStru struct {
	Type  string `json:"type"`
	About string `json:"about"`
	Name  string `json:"name"`
}

// ContentPostStru
type ContentPostStru struct {
	Type     string          `json:"type"`
	Text     string          `json:"text"`
	Root     string          `json:"root"`
	Mentions json.RawMessage `json:"mentions"`
	Branch   string          `json:"branch"`
}

// LasterNumLikes
type LasterNumLikes struct {
	ClientID         string `json:"client_id"`
	LasterLikeNum    int    `json:"laster_like_num"`
	Name             string `json:"client_name"`
	ClientEthAddress string `json:"client_eth_address"`
	MessageFromPub   string `json:"message_from_pub"`
}

// TempdMessage 用于一次搜索的结果统计
type TempdMessage struct {
	Author string `json:"author"`
}

// Name2ProfileReponse
type Name2ProfileReponse struct {
	ID         string `json:"client_id"`
	Name       string `json:"client_Name"`
	Alias      string `json:"client_alias"`
	Bio        string `json:"client_bio"`
	EthAddress string `json:"client_eth_address"`
}

//Whoami
type Whoami struct {
	Pub_Id          string `json:"pub_id"`
	Pub_Eth_Address string `json:"pub_eth_address"`
}

// SensitiveWords
var SensitiveWords []string

// EventSensitive
type EventSensitive struct {
	PubID           string `json:"pub_id"`
	MessageScanTime int64  `json:"message_scan_time"`
	MessageText     string `json:"message_text"`
	MessageKey      string `json:"message_key"`
	MessageAuthor   string `json:"message_author"`
	DealTag         string `json:"deal_tag"`
	DealTime        int64  `json:"deal_time"`
}

// UserTasks
type UserTasks struct {
	CollectFromPub string `json:"pub_id"`
	Author         string `json:"author"`
	MessageKey     string `json:"message_key"`
	MessageType    string `json:"message_type"`
	MessageRoot    string `json:"message_root"`
	MessageTime    int64  `json:"message_time"`

	NfttxHash    string `json:"nft_tx_hash"`
	NftTokenId   string `json:"nft_token_id"`
	NftStoredUrl string `json:"nft_store_url"`

	ClientEthAddress string `json:"client_eth_address"`
}

// ReqUserTask
type ReqUserTask struct {
	Author      string `json:"author"`
	MessageType string `json:"message_type"`
	StartTime   int64  `json:"start_time"`
	EndTime     int64  `json:"end_time"`
}

// ReqUserLoginApp
type ReqUserLoginApp struct {
	ClientID  string `json:"client_id"`
	LoginTime int64  `json:"login_time"`
}

// ReqCreatedNFT
type ReqCreatedNFT struct {
	ClientID       string `json:"client_id"`
	NftCreatedTime int64  `json:"nft_created_time"`
	NfttxHash      string `json:"nft_tx_hash"`
	NftTokenId     string `json:"nft_token_id"`
	NftStoredUrl   string `json:"nft_store_url"`
}
