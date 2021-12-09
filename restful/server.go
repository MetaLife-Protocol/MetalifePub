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
	"go.cryptoscope.co/ssb"
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
)

var Config *params.ApiConfig

var longCtx context.Context

var quitSignal chan struct{}

var client *ssbClient.Client

var log kitlog.Logger

func Start(ctx context.Context) {
	Config = params.NewApiServeConfig()
	longCtx = ctx

	sclient, err := newClient(nil)
	if err != nil {
		level.Error(log).Log("ssb restful api service start err", err)
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
	)
	if err != nil {
		//fmt.Println(fmt.Sprintf("make router : %s",err))
		level.Error(log).Log("make router err", err)
		return
	}
	api.SetApp(router)
	listen := fmt.Sprintf("%s:%d", Config.Host, Config.Port)
	server := &http.Server{Addr: listen, Handler: api.MakeHandler()}
	go server.ListenAndServe()
	fmt.Println(fmt.Sprintf("ssb restful api service start..."))
	<-quitSignal
	err = server.Shutdown(context.Background())
	if err != nil {
		fmt.Println(fmt.Sprintf("API restful service Shutdown err : %s", err))
	}
}

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
	err = ExecOnce()
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
	err = ExecOnce()
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

func ExecOnce() error {
	/*client, err := newClient(nil)
	if err != nil {
		return err
	}
	*/
	//var args = getStreamArgs(ctx)
	fmt.Println("ExecOnce(1)")
	src, err := client.Source(longCtx, muxrpc.TypeJSON, muxrpc.Method{"createLogStream"}, nil)
	if err != nil {
		return fmt.Errorf("source stream call failed: %w", err)
	}
	fmt.Println("ExecOnce(2)")
	err = jsonDrain1(src)
	if err != nil {
		err = fmt.Errorf("message pump failed: %w", err)
	}
	return err

}

func newClient(ctx *cli.Context) (*ssbClient.Client, error) {
	// todo
	sockPath := "~/.ssb-go/socket" //ctx.String("unixsock")
	if sockPath != "" {
		client, err := ssbClient.NewUnix(sockPath, ssbClient.WithContext(nil))
		if err != nil {
			fmt.Println(fmt.Sprintf("client unix-path based init failed err=%s", err))
			return newTCPClient(ctx)
		}
		fmt.Println(fmt.Sprintf("client connected method unix sock"))
		return client, nil
	}

	// Assume TCP connection
	return newTCPClient(ctx)
}

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
			return nil, fmt.Errorf("init: base64 decode of --remoteKey failed: %w", err)
		}
		copy(remotePubKey, rpk)
	}

	plainAddr, err := net.ResolveTCPAddr("tcp", ctx.String("addr"))
	if err != nil {
		return nil, fmt.Errorf("int: failed to resolve TCP address: %w", err)
	}

	shsAddr := netwrap.WrapAddr(plainAddr, secretstream.Addr{PubKey: remotePubKey})
	client, err := ssbClient.NewTCP(localKey, shsAddr,
		ssbClient.WithSHSAppKey(ctx.String("shscap")),
		ssbClient.WithContext(nil))
	if err != nil {
		return nil, fmt.Errorf("init: failed to connect to %s: %w", shsAddr.String(), err)
	}
	fmt.Println(fmt.Sprintf("client connected method tcp"))
	return client, nil
}

func jsonDrain1(r *muxrpc.ByteSource) error {

	var buf = &bytes.Buffer{}

	TempMsgMap = make(map[string]*TempdMessage)
	Name2Hex = make(map[string]string)
	LikeCountMap = make(map[string]*LasterNumLikes)
	LikeDetail = []string{}
	fmt.Println("has jsonDrain1")
	for r.Next(context.TODO()) { // read/write loop for messages

		buf.Reset()
		err := r.Reader(func(r io.Reader) error {
			_, err := buf.ReadFrom(r)
			return err
		})
		if err != nil {
			return err
		}

		//jsonReply, err := json.MarshalIndent(buf.Bytes(), "", "  ")
		if err != nil {
			return err
		}

		/*_, err = buf.WriteTo(os.Stdout)
		if err != nil {
			return err
		}*/

		var msgStruct legacy.DeserializedMessage

		err = json.Unmarshal(buf.Bytes(), &msgStruct)
		if err != nil {
			fmt.Println(fmt.Sprintf("Muxrpc.ByteSource Unmarshal to json err =%s", err))
			return err
		}

		/*fmt.Println("******receive a message******")*/
		/*fmt.Println(fmt.Sprintf("[message]previous\t:%v", msgStruct.Previous))
		fmt.Println(fmt.Sprintf("[message]sequence\t:%v", msgStruct.Sequence))
		fmt.Println(fmt.Sprintf("[message]author\t:%v", msgStruct.Author))
		fmt.Println(fmt.Sprintf("[message]timestamp\t:%v", msgStruct.Timestamp))
		fmt.Println(fmt.Sprintf("[message]hash\t:%v", msgStruct.Hash))*/

		//1、记录消息ID和author的关系
		if msgStruct.Previous != nil { //这里需要过滤掉根消息Previous=null
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
					if string(cvs.Vote.Expression) == "Like" {
						LikeDetail = append(LikeDetail, cvs.Vote.Link)
					}
				}
			} else {
				/*fmt.Println(fmt.Sprintf("Unmarshal for vote , err %v", err))*/
			}

			//3、about即修改备注名为hex-address的信息
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
		/*fmt.Println(fmt.Sprintf("===================================================================%v", msgStruct.Sequence))*/
	}
	// 编写LikeCount 被like的author收集到的点zan总数量
	for _, likeLink := range LikeDetail {
		author, ok := TempMsgMap[likeLink]
		if ok {
			_, ok := LikeCountMap[author.Author]
			if !ok {
				infos := LasterNumLikes{
					ClientID:         author.Author,
					ClientAddress:    "i do not know it's Eth Address",
					LasterAddVoteNum: 0,
					LasterVoteNum:    0,
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
		}

		/*fmt.Println("likelink:"+likeLink)*/

	}
	/*//print for test
	fmt.Println("消息ID**********发布人")
	for key := range TempMsgMap{ //取map中的值
		fmt.Println(key,"**********",TempMsgMap[key].Author)
	}
	fmt.Println("客户端ID**********以太坊地址")
	for key := range Name2Hex{ //取map中的值
		fmt.Println(key,"**********",Name2Hex[key])
	}
	fmt.Println("计算出的发币结果")
	fmt.Println(fmt.Sprintf("request test result:%s",LikeCountMap))*/

	return r.Err()
}

// LikeDetail 存储一轮搜索到的
var LikeDetail []string

// LikeCount for save message for search likes's author
var TempMsgMap map[string]*TempdMessage

// Name2Hex for save message for search likes's author
var Name2Hex map[string]string

// LikeCount for api service link(eg:%vSK7+wJ7ceZNVUCkTQliXrhgfffr5njb5swTrEZLDiM=.sha256)
var LikeCountMap map[string]*LasterNumLikes

type ContentVoteStru struct {
	Type string    `json:"type"`
	Vote *VoteStru `json:"vote"`
}
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
