package restful

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/ant0ine/go-json-rest/rest"
	"go.cryptoscope.co/muxrpc/v2"
	"go.cryptoscope.co/netwrap"
	"go.cryptoscope.co/secretstream"
	ssbClient "go.cryptoscope.co/ssb/client"
	"go.cryptoscope.co/ssb/message/legacy"
	"go.cryptoscope.co/ssb/plugins/legacyinvites"
	"go.cryptoscope.co/ssb/restful/params"
	kitlog "go.mindeco.de/log"
	"go.mindeco.de/log/level"
	"golang.org/x/crypto/ed25519"
	"gopkg.in/urfave/cli.v2"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"
	/*"go.cryptoscope.co/ssb/message"
	"go.mindeco.de/ssb-refs"*/

	"go.cryptoscope.co/ssb"
	"go.cryptoscope.co/ssb/message"
	"go.mindeco.de/ssb-refs"
	"os"
)

var Config *params.ApiConfig

var longCtx context.Context

var quitSignal chan struct{}

var client *ssbClient.Client

var log kitlog.Logger

var lastAnalysisTimesnamp int64

var likeDB *PubDB

// Start
func Start(ctx *cli.Context) {
	Config = params.NewApiServeConfig()
	longCtx = ctx

	sclient, err := newClient(ctx)
	if err != nil {
		level.Error(log).Log("Ssb restful api and message analysis service start err", err)
		return
	}
	client = sclient

	quitSignal := make(chan struct{})
	api := rest.NewApi()
	if Config.Debug {
		api.Use(rest.DefaultDevStack...)
	} else {
		api.Use(rest.DefaultProdStack...)
	}
	api.Use(rest.DefaultDevStack...)
	router, err := rest.MakeRouter(
		//rest.Get("/ssb/api/apply-invite-code", ApplyInviteCode),
		//不需要，直接走ssb消息发过来rest.Post("ssb/debug/get-message",getMessage),
		rest.Get("/ssb/api/likes", GetLikes),
		rest.Get("/ssb/api/node-address", clientid2Address),
		rest.Post("/ssb/api/id2eth", clientid2Address),
	)
	/*
			//登记CLIENT ID AND YOUR ETH ADDRESS
			http-api:	http://106.52.171.12:18008/ssb/api/id2eth
			POST
			参数：
			{
		            "client_id": "@/q3ohp8l7x2H5zULoCTFM8lH3TZk/ueYb8cA7LxIHyE=.ed25519",
		            "client_eth_address": "0x6d946D646879d31a45bCE89a68B24cab165E9A2A"
		        }
			返回：
			{
		    "error_code": 0,
		    "error_message": "SUCCESS",
		    "data":{
				"result":"ok",
				"timestamp":1637901258137
				}
			}
	*/
	if err != nil {
		level.Error(log).Log("make router err", err)
		return
	}

	api.SetApp(router)

	listen := fmt.Sprintf("%s:%d", Config.Host, Config.Port)
	server := &http.Server{Addr: listen, Handler: api.MakeHandler()}
	go server.ListenAndServe()
	fmt.Println(fmt.Sprintf("ssb restful api and message analysis service start...\n"))

	go DoMessageTask(ctx)

	<-quitSignal
	err = server.Shutdown(context.Background())
	if err != nil {
		fmt.Println(fmt.Sprintf("API restful service Shutdown err : %s", err))
	}

}

// newClient creat a client link to ssb-server
func newClient(ctx *cli.Context) (*ssbClient.Client, error) {
	sockPath := ctx.String("unixsock")
	if sockPath != "" {
		client, err := ssbClient.NewUnix(sockPath, ssbClient.WithContext(longCtx))
		if err != nil {
			level.Debug(log).Log("client", "unix-path based init failed", "err", err)
			level.Info(log).Log("client", "Now try switching to TCP working mode and init it")
			return newTCPClient(ctx)
		}
		level.Info(log).Log("client", "connected", "method", "unix sock")
		return client, nil
	}

	// Assume TCP connection
	return newTCPClient(ctx)
}

// newTCPClient create tcp client to support remote applications
func newTCPClient(ctx *cli.Context) (*ssbClient.Client, error) {
	localKey, err := ssb.LoadKeyPair(ctx.String("key"))
	if err != nil {
		return nil, err
	}

	var remotePubKey = make(ed25519.PublicKey, ed25519.PublicKeySize)
	copy(remotePubKey, localKey.ID().PubKey())
	if rk := ctx.String("remoteKey"); rk != "" {
		rk = strings.TrimSuffix(rk, ".ed25519")
		rk = strings.TrimPrefix(rk, "@")
		rpk, err := base64.StdEncoding.DecodeString(rk)
		if err != nil {
			return nil, fmt.Errorf("Init: base64 decode of --remoteKey failed: %w", err)
		}
		copy(remotePubKey, rpk)
	}

	plainAddr, err := net.ResolveTCPAddr("tcp", ctx.String("addr"))
	if err != nil {
		return nil, fmt.Errorf("Init: failed to resolve TCP address: %w", err)
	}

	shsAddr := netwrap.WrapAddr(plainAddr, secretstream.Addr{PubKey: remotePubKey})
	client, err := ssbClient.NewTCP(localKey, shsAddr,
		ssbClient.WithSHSAppKey(ctx.String("shscap")),
		ssbClient.WithContext(longCtx))
	if err != nil {
		return nil, fmt.Errorf("Init: failed to connect to %s: %w", shsAddr.String(), err)
	}

	fmt.Println(fmt.Sprintf("Client = [%s] , method = [%s] , linked pub server = [%s]", "connected", "[TCP]", shsAddr.String()))

	return client, nil
}

// initDb
func initDb(ctx *cli.Context) error {
	pubdatadir := ctx.String("datadir")

	likedb, err := OpenPubDB(pubdatadir)
	if err != nil {
		fmt.Errorf(fmt.Sprintf("Failed to create database", err))
	}

	lstime, err := likedb.SelectLastScanTime()
	if err != nil {
		fmt.Errorf(fmt.Sprintf("Failed to init database", err))
	}
	if lstime == 0 {
		_, err = likedb.UpdateLastScanTime(0) //1647503842641
		if err != nil {
			fmt.Errorf(fmt.Sprintf("Failed to init database", err))
		}
	}
	lastAnalysisTimesnamp = lstime
	likeDB = likedb

	return nil
}

// DoMessageTask get message from the server copy
func DoMessageTask(ctx *cli.Context) {
	if err := initDb(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	time.Sleep(time.Second * 1)

	for {
		//构建符合条件的message请求
		var ref refs.FeedRef
		if id := ctx.String("id"); id != "" {
			var err error
			ref, err = refs.ParseFeedRef(id)
			if err != nil {
				panic(err)
			}
		}
		args := message.CreateHistArgs{
			ID:     ref,
			Seq:    ctx.Int64("seq"),
			AsJSON: ctx.Bool("asJSON"),
		}
		args.Gt = message.RoundedInteger(lastAnalysisTimesnamp)
		args.Limit = -1
		args.Seq = 0
		args.Keys = false
		args.Values = false
		args.Private = false
		src, err := client.Source(longCtx, muxrpc.TypeJSON, muxrpc.Method{"createLogStream"}, args)
		if err != nil {
			//client可能失效,则需要重建新的连接
			fmt.Println(fmt.Sprintf("Source stream call failed: %w ,will try other tcp connect socket...", err))
			otherClient, err := newClient(ctx)
			if err != nil {
				fmt.Println(fmt.Sprintf("Try set up a ssb client tcp socket failed , will try again...", err))
				time.Sleep(time.Second * 10)
				continue
			}

			client = otherClient
			continue
		}

		//从上一次的计算点（数据库记录的毫秒时间戳）到最后一条记录的解析
		calcComplateTime, err := SsbMessageAnalysis(src)
		if err != nil {
			fmt.Println(fmt.Sprintf("Message pump failed: %w", err))
			time.Sleep(time.Second * 5)
			continue
		}

		fmt.Println(fmt.Sprintf("\nA round of message data analysis has been completed , from TimeSanmp[%v] to [%v]", lastAnalysisTimesnamp, calcComplateTime))
		lastAnalysisTimesnamp = calcComplateTime
		time.Sleep(time.Second * 30)
	}
}

// change to

func clientid2Address(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf("Restful Api Call ----> clientid2Address ,err=%s", resp.ToFormatString()))
		writejson(w, resp)
	}()

	name2addr, err := GetNodeEthAddress()
	resp = NewAPIResponse(err, name2addr)
}

func ApplyInviteCode(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf("Restful Api Call ----> ApplyInviteCode ,err=%s", resp.ToFormatString()))
		writejson(w, resp)
	}()

	inviteCode, err := GetInviteCode()
	resp = NewAPIResponse(err, inviteCode)
}

func GetLikes(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf("Restful Api Call ----> GetLikes ,err=%s", resp.ToFormatString()))
		writejson(w, resp)
	}()

	name2addr, err := GetLikeSum()
	resp = NewAPIResponse(err, name2addr)
}

func GetInviteCode() (invitecode string, err error) {
	var args legacyinvites.CreateArguments
	args.Uses = 1
	//_,err = client.Source(longCtx, &invitecode, muxrpc.TypeString, muxrpc.Method{"invite", "create"}, args)
	src, err := client.Source(longCtx, muxrpc.TypeString, muxrpc.Method{"invite", "create"}, args)
	//src, err := client.Source(longCtx, muxrpc.TypeJSON, muxrpc.Method{"createLogStream"}, nil)
	if err != nil {
		return "", err
	}
	scode, err := src.Bytes()
	invitecode = string(scode)
	return
}

func GetLikeSum() (datas []*LasterNumLikes, err error) {
	//err = ExecOnce()
	time.Sleep(time.Second * 2)
	if err != nil {
		fmt.Println(fmt.Sprintf("exec log err : %s", err))
		return
	}

	for _, lk := range LikeCountMap {
		d := &LasterNumLikes{
			ClientID:         lk.ClientID,
			ClientAddress:    lk.ClientAddress,
			LasterAddVoteNum: lk.LasterAddVoteNum,
			LasterVoteNum:    lk.LasterVoteNum,
		}
		datas = append(datas, d)
		//todo
		go ChannelDeal(lk.ClientAddress)
	}
	return datas, nil
}

func GetNodeEthAddress() (datas []*Name2EthAddrReponse, err error) {
	//err = ExecOnce()
	time.Sleep(time.Second * 2)
	if err != nil {
		fmt.Println(fmt.Sprintf("exec log err : %s", err))
		return
	}

	for key, ethaddr := range Name2Hex {
		d := &Name2EthAddrReponse{
			Name:       key,
			EthAddress: ethaddr,
		}
		datas = append(datas, d)
		//todo
		go ChannelDeal(ethaddr)
	}
	return datas, nil
}

type Name2EthAddrReponse struct {
	Name       string `json:"client_id"`
	EthAddress string `json:"client_eth_address"`
}

func SsbMessageAnalysis(r *muxrpc.ByteSource) (int64, error) {

	var buf = &bytes.Buffer{}

	TempMsgMap = make(map[string]*TempdMessage)
	Name2Hex = make(map[string]string)
	LikeCountMap = make(map[string]*LasterNumLikes)
	LikeDetail = []string{}
	for r.Next(context.TODO()) { // read/write loop for messages

		buf.Reset()
		err := r.Reader(func(r io.Reader) error {
			_, err := buf.ReadFrom(r)
			return err
		})
		if err != nil {
			return 0, err
		}

		/*_, err = buf.WriteTo(os.Stdout)
		if err != nil {
			return err
		}
		continue*/

		var msgStruct legacy.DeserializedMessage

		err = json.Unmarshal(buf.Bytes(), &msgStruct)
		if err != nil {
			fmt.Println(fmt.Sprintf("Muxrpc.ByteSource Unmarshal to json err =%s", err))
			return 0, err
		}

		/*fmt.Println("******receive a message******")*/
		/*fmt.Println(fmt.Sprintf("[message]previous\t:%v", msgStruct.Previous))
		fmt.Println(fmt.Sprintf("[message]sequence\t:%v", msgStruct.Sequence))
		fmt.Println(fmt.Sprintf("[message]author\t:%v", msgStruct.Author))
		fmt.Println(fmt.Sprintf("[message]timestamp\t:%v", msgStruct.Timestamp))
		fmt.Println(fmt.Sprintf("[message]hash\t:%v", msgStruct.Hash))*/

		//1、记录本轮所有消息ID和author的关系
		if msgStruct.Previous != nil { //这里需要过滤掉[根消息]Previous=null
			TempMsgMap[fmt.Sprintf("%v", msgStruct.Previous)] = &TempdMessage{
				Author: fmt.Sprintf("%v", msgStruct.Author),
			}
		}

		//2、记录like的统计结果
		contentJust := string(msgStruct.Content[0])
		if contentJust == "{" {
			//1、like的信息
			cvs := ContentVoteStru{}
			err = json.Unmarshal(msgStruct.Content, &cvs)
			if err == nil {
				if string(cvs.Type) == "vote" {
					/*fmt.Println(fmt.Sprintf("[vote]link :%v", cvs.Vote.Link))
					fmt.Println(fmt.Sprintf("[vote]expression :%v", cvs.Vote.Expression))*/
					//get the Like tag ,因为like肯定在发布message后,先记录被like的link，再找author
					if string(cvs.Vote.Expression) != "️Unlike" { //1:❤️ 2:👍 3:✌️ 4:👍
						LikeDetail = append(LikeDetail, cvs.Vote.Link)
					}
					//get the Unlike tag ,先记录被like的link，再找author；由于图谱深度不一样，按照时间顺序查询存在问题，则先统一记录
					if string(cvs.Vote.Expression) == "Unlike" {
						UnLikeDetail = append(UnLikeDetail, cvs.Vote.Link)
					}
				}
			} else {
				/*fmt.Println(fmt.Sprintf("Unmarshal for vote , err %v", err))*/
				//todo 可以根据协议的扩展，记录其他的vote数据，目前没有这个需求
			}

			//3、about即修改备注名为hex-address的信息,注意:修改N次name,只需要返回最新的即可
			cau := ContentAboutStru{}
			err = json.Unmarshal(msgStruct.Content, &cau)
			if err == nil {
				if string(cau.Type) == "about" {
					/*fmt.Println(fmt.Sprintf("[about]about :%v", cau.About))
					fmt.Println(fmt.Sprintf("[about]name :%v", cau.Name))*/
					Name2Hex[fmt.Sprintf("%v", cau.About)] =
						fmt.Sprintf("%v", cau.Name)

				}
			} else {
				fmt.Println(fmt.Sprintf("Unmarshal for about , err %v", err))
			}
		}

		//记录客户端id和Address的绑定关系
		/*fmt.Println(fmt.Sprintf("======>%v", msgStruct.Sequence))*/
	}
	// 编写LikeCount 被like的author收集到的点zan总数量
	for _, likeLink := range LikeDetail { //被点赞的ID集合
		author, ok := TempMsgMap[likeLink]
		if ok {
			_, ok := LikeCountMap[author.Author]
			if !ok {
				infos := LasterNumLikes{
					ClientID:         author.Author,
					ClientAddress:    "i do not know it's Eth Address",
					LasterAddVoteNum: 1,
					LasterVoteNum:    1,
				}
				LikeCountMap[author.Author] = &infos
			} else {
				LikeCountMap[author.Author].LasterAddVoteNum++
				LikeCountMap[author.Author].LasterVoteNum++
			}

			hexStr, ok := Name2Hex[author.Author]
			if ok {
				LikeCountMap[author.Author].ClientAddress = hexStr
			}
			//fmt.Println("likelink:" + likeLink)
		}

	}

	for _, unLikeLink := range UnLikeDetail { //被取消点赞的ID集合
		author, ok := TempMsgMap[unLikeLink]
		if ok {
			LikeCountMap[author.Author].LasterAddVoteNum--
			LikeCountMap[author.Author].LasterVoteNum--
			//fmt.Println("unlikelink:" + unLikeLink)
		}

	}

	//不能以最后一条消息的时间作为本轮计算的时间点,后期改为从服务器上取得pub的时间,
	//计算周期越小越好,最大程度避免在统计中有新消息过来

	nowUnix := time.Now().Unix()
	_, err := likeDB.UpdateLastScanTime(nowUnix)
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to UpdateLastScanTime", err))
		return 0, err
	}

	/*//print for test
	fmt.Println("本轮消息ID**********发布人")
	for key := range TempMsgMap { //取map中的值
		fmt.Println(key, "**********", TempMsgMap[key].Author)
	}*/
	fmt.Println("发布人ID**********以太坊地址")
	for key := range Name2Hex { //取map中的值
		fmt.Println(key, "**********", Name2Hex[key])
	}
	fmt.Println("计算出的点赞结果")
	for key := range LikeCountMap { //取map中的值
		fmt.Println(fmt.Sprintf("%s**********request test result:%s", key, LikeCountMap[key]))
	}

	return nowUnix, nil
}

// LikeDetail 存储一轮搜索到的被Like的消息ID
var LikeDetail []string

// LikeDetail 存储一轮搜索到的被Unlike的消息ID
var UnLikeDetail []string

// LikeCount for save message for search likes's author
var TempMsgMap map[string]*TempdMessage

// Name2Hex for save message for search likes's author
var Name2Hex map[string]string

// LikeCount for api service link(eg:%vSK7+wJ7ceZNVUCkTQliXrhgfffr5njb5swTrEZLDiM=.sha256)
var LikeCountMap map[string]*LasterNumLikes

/*
	"content":
	{
		"type":"vote",
		"vote":
		{
			"link":"%GSonxYwRNuQqyl+0QF1OpmMpdkBlHCEgLnV9m7872hQ=.sha256",
			"value":1,
			"expression":"Like"
		}
	},
*/
// ContentVoteStru
type ContentVoteStru struct {
	Type string    `json:"type"`
	Vote *VoteStru `json:"vote"`
}

//VoteStru
type VoteStru struct {
	Link       string `json:"link"`
	value      int    `json:"value"`
	Expression string `json:"expression"`
}

type ContentAboutStru struct {
	Type  string `json:"type"`
	About string `json:"about"`
	Name  string `json:"name"`
}

// LasterNumLikes
// ClientID 客户端ID
// ClientAddress 客户端执行的Hex address
// VoteLink为被点赞的内容ID
// LasterAddVoteNum 为新增的点赞数量
// LasterAddVoteNum 收集到了总的点赞数量（如果发放奖励在先先，有取消点赞的，不收回奖励
type LasterNumLikes struct {
	ClientID         string `json:"client_id"`
	ClientAddress    string `json:"client_eth_address"`
	LasterAddVoteNum int64  `json:"laster_add_vote_num"`
	LasterVoteNum    int64  `json:"laster_vote_num"`
	//VoteLink         []string `json:"laster_add_vote_num"`
}

// TempdMessage 用于一次搜索的结果统计
type TempdMessage struct {
	Author string `json:"author"`
}

//ChannelDeal
func ChannelDeal(partnerAddress string) (err error) {
	if len(partnerAddress) != 42 || partnerAddress[0:2] != "0x" {
		err = fmt.Errorf("ETH ADDRESS error for " + partnerAddress)
		return nil
	} //todo address sum check
	photonNode := &PhotonNode{
		Host:       "http://" + params.PhotonHost,
		Address:    params.PhotonAddress,
		APIAddress: params.PhotonHost,
		DebugCrash: false,
	}
	partnerNode := &PhotonNode{
		//:utils.APex2(rs.Config.PubAddress),
		Address:    partnerAddress,
		DebugCrash: false,
	}
	channel00 := photonNode.GetChannelWithBigInt(partnerNode, params.TokenAddress)
	if channel00 == nil {
		//create new channel 0.1smt
		err = photonNode.OpenChannelBigInt(partnerNode.Address, params.TokenAddress, new(big.Int).Mul(big.NewInt(params.Finney), big.NewInt(100)), params.SettleTime)
		if err != nil {
			fmt.Println(fmt.Sprintf("[Pub]create channel err %s", err))
			return
		}
		fmt.Println(fmt.Sprintf("[Pub]create channel success ,with %s", partnerAddress))
	} else {
		fmt.Println(fmt.Sprintf("[Pub]channel has exist, with %s", partnerAddress))
	}

	//todo 主动检查补充余额
	return
}
