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
		rest.Get("/ssb/api/likes", GetAllLikes),

		//get all 'about' message,e.g:'about'='eth address'
		rest.Get("/ssb/api/node-info", clientid2Profiles),

		//get the 'about' message by client id ,e.g:'about'='eth address'
		rest.Post("/ssb/api/node-info", clientid2Profile),

		//register client's eth address to it's ID
		rest.Post("/ssb/api/id2eth", UpdateEthAddr),
	)
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
		args.Keys = true
		args.Values = true
		args.Private = false
		src, err := client.Source(longCtx, muxrpc.TypeJSON, muxrpc.Method{"createLogStream"}, args)
		if err != nil {
			//client可能失效,则需要重建新的连接,链接资源的释放在ssb-server端
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
		time.Sleep(time.Second)
		calcComplateTime, err := SsbMessageAnalysis(src)
		if err != nil {
			fmt.Println(fmt.Sprintf("Message pump failed: %w", err))
			time.Sleep(time.Second * 5)
			continue
		}
		fmt.Println(fmt.Sprintf("A round of message data analysis has been completed , from TimeSanmp[%v] to [%v]", lastAnalysisTimesnamp, calcComplateTime))
		lastAnalysisTimesnamp = calcComplateTime

		time.Sleep(params.MsgScanInterval)
	}
}

// change to tcp scene

// clientid2Profile
func clientid2Profiles(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf("Restful Api Call ----> clientid2Profile ,err=%s", resp.ToFormatString()))
		writejson(w, resp)
	}()

	name2addr, err := GetAllNodesProfile()
	resp = NewAPIResponse(err, name2addr)
}

// clientid2Profile
func clientid2Profile(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf("Restful Api Call ----> getEthAddrByID ,err=%s", resp.ToFormatString()))
		writejson(w, resp)
	}()
	var req Name2ProfileReponse
	err := r.DecodeJsonPayload(&req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var cid = req.ID
	name2addr, err := GetNodeProfile(cid)
	resp = NewAPIResponse(err, name2addr)
}

//UpdateEthAddr
func UpdateEthAddr(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf("Restful Api Call ----> getEthAddrByID ,err=%s", resp.ToFormatString()))
		writejson(w, resp)
	}()
	var req = &Name2ProfileReponse{}
	err := r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	//req.Other1=eth address在客户端做合法性判断
	_, err = likeDB.UpdateUserProfile(req.ID, req.Name, req.EthAddress)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//和客户端建立一个奖励通道
	err = ChannelDeal(req.EthAddress)
	if err != nil {
		fmt.Println(err)
		resp = NewAPIResponse(fmt.Errorf("fail to create a channel to %s", req.EthAddress), nil)
	}
	resp = NewAPIResponse(err, "success")
}

// GetAllNodesProfile
func GetAllNodesProfile() (datas []*Name2ProfileReponse, err error) {
	profiles, err := likeDB.SelectUserProfile("")
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to db-SelectUserProfileAll", err))
		return
	}
	datas = profiles
	return
}

// GetNodeProfile
func GetNodeProfile(cid string) (datas []*Name2ProfileReponse, err error) {
	profile, err := likeDB.SelectUserProfile(cid)
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to db-SelectUserEthAddrAll", err))
		return
	}
	datas = profile
	return
}

func GetAllLikes(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf("Restful Api Call ----> GetAllLikes ,err=%s", resp.ToFormatString()))
		writejson(w, resp)
	}()

	likes, err := CalcGetLikeSum()

	resp = NewAPIResponse(err, likes)
}

// GetAllNodesProfile
func CalcGetLikeSum() (datas map[string]*LasterNumLikes, err error) {
	likes, err := likeDB.SelectLikeSum("")
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to db-SelectUserProfile", err))
		return
	}
	datas = likes
	return
}

type Name2ProfileReponse struct {
	ID         string `json:"client_id"`
	Name       string `json:"client_Name"`
	Alias      string `json:"client_alias"`
	Bio        string `json:"client_bio"`
	EthAddress string `json:"client_eth_address"`
}

func SsbMessageAnalysis(r *muxrpc.ByteSource) (int64, error) {
	var buf = &bytes.Buffer{}
	TempMsgMap = make(map[string]*TempdMessage)
	ClientID2Name = make(map[string]string)
	LikeDetail = []string{}
	UnLikeDetail = []string{}

	//不能以最后一条消息的时间作为本轮计算的时间点,后期改为从服务器上取得pub的时间,
	//计算周期越小越好,加载完本轮所有消息的时间点即为下一轮的开始时间，这样规避了在计算过程中有新消息被同步进入pub
	//注意：manyvse等客户端向服务器同步数据，延迟时间不定，如果无网状态发送过来的消息被视为空
	nowUnixTime := time.Now().UnixNano() / 1e6

	for r.Next(context.TODO()) {
		//在本轮for计算周期内如果有数据
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
			return 0,err
		}
		continue*/

		var msgStruct DeserializedMessageStu
		err = json.Unmarshal(buf.Bytes(), &msgStruct)
		if err != nil {
			fmt.Println(fmt.Sprintf("Muxrpc.ByteSource Unmarshal to json err =%s", err))
			return 0, err
		}

		//1、记录本轮所有消息ID和author的关系,保存下来,被点赞的消息基本不会在本轮被扫描到
		msgkey := fmt.Sprintf("%v", msgStruct.Key)
		msgauther := fmt.Sprintf("%v", msgStruct.Value.Author)
		TempMsgMap[msgkey] = &TempdMessage{
			Author: msgauther,
		}
		_, err = likeDB.InsertLikeDetail(msgkey, msgauther)
		if err != nil {
			fmt.Println(fmt.Sprintf("Failed to InsertLikeDetail", err))
			return 0, err
		}

		//2、记录like的统计结果
		contentJust := string(msgStruct.Value.Content[0])
		if contentJust == "{" {
			//1、like的信息
			cvs := ContentVoteStru{}
			err = json.Unmarshal(msgStruct.Value.Content, &cvs)
			if err == nil {
				if string(cvs.Type) == "vote" {
					/*if cvs.Vote.Expression != "️Unlike" { //1:❤️ 2:👍 3:✌️ 4:👍这种判断不知道什么是错误的：可以同时有点赞和取消点赞的判断
						LikeDetail = append(LikeDetail, cvs.Vote.Link)
						timesp := time.Unix(int64(msgStruct.Value.Timestamp)/1e3, 0).Format("2006-01-02 15:04:05")
						fmt.Println("like-time:\t" + timesp + "MessageKey:\t" + cvs.Vote.Link)
					}*/
					//get the Unlike tag ,先记录被like的link，再找author；由于图谱深度不一样，按照时间顺序查询存在问题，则先统一记录
					if cvs.Vote.Expression == "Unlike" {
						UnLikeDetail = append(UnLikeDetail, cvs.Vote.Link)
						timesp := time.Unix(int64(msgStruct.Value.Timestamp)/1e3, 0).Format("2006-01-02 15:04:05")
						fmt.Println("unlike-time:\t" + timesp + "\tMessageKey:\t" + cvs.Vote.Link)
					} else {
						//get the Like tag ,因为like肯定在发布message后,先记录被like的link，再找author
						LikeDetail = append(LikeDetail, cvs.Vote.Link)
						timesp := time.Unix(int64(msgStruct.Value.Timestamp)/1e3, 0).Format("2006-01-02 15:04:05")
						fmt.Println("like-time:\t" + timesp + "\tMessageKey:\t" + cvs.Vote.Link)
					}
				}
			} else {
				/*fmt.Println(fmt.Sprintf("Unmarshal for vote , err %v", err))*/
				//todox 可以根据协议的扩展，记录其他的vote数据，目前没有这个需求
			}

			//3、about即修改备注名为hex-address的信息,注意:修改N次name,只需要返回最新的即可
			//此为备份方案：认定Name为ethaddr,需要同步修改API，name字段代替other1
			cau := ContentAboutStru{}
			err = json.Unmarshal(msgStruct.Value.Content, &cau)
			if err == nil {
				if cau.Type == "about" {
					ClientID2Name[fmt.Sprintf("%v", cau.About)] =
						fmt.Sprintf("%v", cau.Name)
				}
			} else {
				fmt.Println(fmt.Sprintf("Unmarshal for about , err %v", err))
			}
		}
	}

	//save message-result to database
	for _, likeLink := range LikeDetail { //被点赞的ID集合,标记被点赞的记录
		_, err := likeDB.UpdateLikeDetail(1, nowUnixTime, likeLink)
		if err != nil {
			fmt.Println(fmt.Sprintf("Failed to UpdateLikeDetail", err))
			return 0, err
		}
	}

	for _, unLikeLink := range UnLikeDetail { //被取消点赞的ID集合
		_, err := likeDB.UpdateLikeDetail(-1, nowUnixTime, unLikeLink)
		if err != nil {
			fmt.Println(fmt.Sprintf("Failed to UpdateLikeDetail", err))
			return 0, err
		}
	}

	_, err := likeDB.UpdateLastScanTime(nowUnixTime)
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to UpdateLastScanTime", err))
		return 0, err
	}
	//更新table userethaddr
	for key := range ClientID2Name {
		_, err := likeDB.UpdateUserProfile(key, ClientID2Name[key], "")
		if err != nil {
			fmt.Println(fmt.Sprintf("Failed to UpdateUserEthAddr", err))
			return 0, err
		}
	}
	fmt.Println(fmt.Sprintf("A round of message data analysis has been completed , message number= %v", len(TempMsgMap)))

	/*//print for test
	for key,value := range TempMsgMap {
		fmt.Println(key, "<-this round message ID---ClientID->", value.Author)
	}
	for key := range ClientID2Name { //取map中的值
		fmt.Println(key, "<-ClientID---Name->", ClientID2Name[key])
	}*/

	return nowUnixTime, nil
}

// LikeDetail 存储一轮搜索到的被Like的消息ID
var LikeDetail []string

// LikeDetail 存储一轮搜索到的被Unlike的消息ID
var UnLikeDetail []string

// LikeCount for save message for search likes's author
var TempMsgMap map[string]*TempdMessage

// ClientID2Name for save message for search likes's author clientid---name
var ClientID2Name map[string]string

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
type LasterNumLikes struct {
	ClientID         string `json:"client_id"`
	LasterLikeNum    int    `json:"laster_like_num"`
	Name             string `json:"client_name"`
	ClientEthAddress string `json:"client_eth_address"`
}

// TempdMessage 用于一次搜索的结果统计
type TempdMessage struct {
	Author string `json:"author"`
}

// ChannelDeal
func ChannelDeal(partnerAddress string) (err error) {
	if len(partnerAddress) != 42 || partnerAddress[0:2] != "0x" {
		err = fmt.Errorf("ETH ADDRESS error for " + partnerAddress)
		return nil
	} //todox address sum check
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
	channel00, err := photonNode.GetChannelWithBigInt(partnerNode, params.TokenAddress)
	if err != nil {
		fmt.Println(fmt.Sprintf("[Pub-Client-ChannelDeal-ERROR]GetChannelWithBigInterr %s", err))
		return
	}
	if channel00 == nil {
		//create new channel with 0.1smt
		err = photonNode.OpenChannelBigInt(partnerNode.Address, params.TokenAddress, new(big.Int).Mul(big.NewInt(params.Finney), big.NewInt(100)), params.SettleTime)
		if err != nil {
			fmt.Println(fmt.Sprintf("[Pub-Client-ChannelDeal-ERROR]create channel err %s", err))
			return
		}
		fmt.Println(fmt.Sprintf("[Pub-Client-ChannelDeal-OK]create channel success ,with %s", partnerAddress))
	} else {
		fmt.Println(fmt.Sprintf("[Pub-Client-ChannelDeal-OK]channel has exist, with %s", partnerAddress))
	}

	//todo 主动检查补充余额?不需要 注册了eth地址,肯定客户端处于在线状态
	return
}
