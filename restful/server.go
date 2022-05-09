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

	"bufio"
	"go.cryptoscope.co/ssb"
	"go.cryptoscope.co/ssb/dfa"
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

var dfax *dfa.DFA

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
		//pub's whoami
		rest.Get("/ssb/api/pub-whoami", GetPubWhoami),

		//likes of all client
		rest.Get("/ssb/api/likes", GetAllLikes),

		//likes of someone client
		rest.Post("/ssb/api/likes", GetSomeoneLike),

		//get all 'about' message,e.g:'about'='eth address'
		rest.Get("/ssb/api/node-info", clientid2Profiles),

		//get the 'about' message by client id ,e.g:'about'='eth address'
		rest.Post("/ssb/api/node-info", clientid2Profile),

		//register client's eth address to it's ID
		rest.Post("/ssb/api/id2eth", UpdateEthAddr),

		// tipped someone off 举报
		rest.Post("/ssb/api/tipped-who-off", TippedOff),

		//tipped off infomation 所有举报的信息汇总
		rest.Get("/ssb/api/tippedoff-info", GetTippedOffInfo),

		//tipped off infomation 对举报的信息进行处理，认证，如属实，则对该账号进行黑名单处理
		rest.Post("/ssb/api/tippedoff-deal", DealTippedOff),
	)
	if err != nil {
		level.Error(log).Log("make router err", err)
		return
	}

	api.SetApp(router)

	listen := fmt.Sprintf("%s:%d", Config.Host, Config.Port)
	server := &http.Server{Addr: listen, Handler: api.MakeHandler()}
	go server.ListenAndServe()
	fmt.Println(fmt.Sprintf(PrintTime() + "ssb restful api and message analysis service start...\nWelcome..."))

	go DoMessageTask(ctx)

	//go dealBlacklist()

	<-quitSignal
	err = server.Shutdown(context.Background())
	if err != nil {
		fmt.Println(fmt.Sprintf(PrintTime()+"API restful service Shutdown err : %s", err))
	}
}

// TippedOff
func TippedOff(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> TippedWhoOff ,err=%s", resp.ToFormatString()))
		writejson(w, resp)
	}()
	var req TippedOffStu
	err := r.DecodeJsonPayload(&req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var plaintiff = req.Plaintiff
	var defendant = req.Defendant
	var mkey = req.MessageKey
	var reasons = req.Reasons

	var recordtime = time.Now().UnixNano() / 1e6
	lstid, err := likeDB.InsertViolation(recordtime, plaintiff, defendant, mkey, reasons)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if lstid == -1 {
		resp = NewAPIResponse(err, "You've already reported it, thank your again👍")
		return
	}

	resp = NewAPIResponse(err, "Success, the pub administrator will verify as soon as possible, thank you for your report👍")
}

// TippedOffInfo get infos
func GetTippedOffInfo(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		//fmt.Println(fmt.Sprintf("Restful Api Call ----> GetTippedOffInfo ,err=%s", resp.ToFormatString()))
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> GetTippedOffInfo ,err=%s", resp.ErrorMsg))
		writejson(w, resp)
	}()
	var req TippedOffStu
	err := r.DecodeJsonPayload(&req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	datas, err := likeDB.SelectViolationByWhere(req.Plaintiff, req.Defendant, req.MessageKey, req.Reasons, req.DealTag)

	resp = NewAPIResponse(err, datas)
}

// DealTippedOff
func DealTippedOff(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> DealTippedOff ,err=%s", resp.ToFormatString()))
		writejson(w, resp)
	}()
	var req TippedOffStu
	err := r.DecodeJsonPayload(&req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var dtime = time.Now().UnixNano() / 1e6
	_, err = likeDB.UpdateViolation(req.DealTag, dtime, req.Dealreward, req.Plaintiff, req.Defendant, req.MessageKey)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if req.DealTag == "1" { ////for table violationrecord, dealtag=0举报 =1属实 =2事实不清,不予处理
		//1 unfollow and block 'the defendant' and sign him to blacklist
		err = contactSomeone(nil, req.Defendant, false, true)
		if err != nil {
			resp = NewAPIResponse(err, fmt.Sprintf("Unfollow and block %s failed", req.Defendant))
		}
		fmt.Println(fmt.Sprintf(PrintTime()+"Success to Unfollow and block %s", req.Defendant))

		//2 pub另行支付给‘the plaintiff’发token
		name2addr, err := GetNodeProfile(req.Plaintiff)
		if err != nil || len(name2addr) != 1 {
			resp = NewAPIResponse(fmt.Errorf("DealTippedOff-Get plaintiff's ethereum address failed, err= not found or %s", err), "failed")
			return
		}
		addrPlaintiff := name2addr[0].EthAddress

		err = sendToken(addrPlaintiff, int64(params.ReportRewarding), true, false)
		if err != nil {
			resp = NewAPIResponse(fmt.Errorf("DealTippedOff-Failed to Award to %s for ReportRewarding,err= %s", req.Plaintiff, err), "failed")
			return
		}
		_, err = likeDB.UpdateViolation(req.DealTag, dtime, fmt.Sprintf("%d%s", params.ReportRewarding, "e15-"), req.Plaintiff, req.Defendant, req.MessageKey)
		if err != nil {
			rest.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp = NewAPIResponse(err, fmt.Sprintf("success, [%s] has been block by [pub administrator], and pub send award token to [%s]", req.Defendant, req.Plaintiff))
		return
	}
	resp = NewAPIResponse(err, "success")
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

	fmt.Println(fmt.Sprintf(PrintTime()+"Client = [%s] , method = [%s] , linked pub server = [%s]", "connected", "TCP", shsAddr.String()))
	//127.0.0.1:8008|@HZnU6wM+F17J0RSLXP05x3Lag2jGv3F3LzHMjh72coE=.ed25519
	params.PubID = strings.Split(shsAddr.String(), "|")[1]
	fmt.Println(fmt.Sprintf(PrintTime()+"Init: success to work on pub [%s]", params.PubID))

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
		_, err = likedb.UpdateLastScanTime(0)
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
	//init db
	if err := initDb(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	time.Sleep(time.Second * 1)

	//init sensitive words
	f, err := os.Open(params.SensitiveWordsFilePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		word := scanner.Text()
		SensitiveWords = append(SensitiveWords, word)
	}
	if err := scanner.Err(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	dfax = dfa.New()
	dfax.AddBadWords(SensitiveWords)

	time.Sleep(time.Second * 1)

	//ssb-message work
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
			fmt.Println(fmt.Sprintf(PrintTime()+"Source stream call failed: %w ,will try other tcp connect socket...", err))
			otherClient, err := newClient(ctx)
			if err != nil {
				fmt.Println(fmt.Sprintf(PrintTime()+"Try set up a ssb client tcp socket failed , will try again...", err))
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
			fmt.Println(fmt.Sprintf(PrintTime()+"Message pump failed: %w", err))
			time.Sleep(time.Second * 5)
			continue
		}

		var calcsumthisTurn = len(TempMsgMap)
		fmt.Println(fmt.Sprintf(PrintTime()+"A round of message data analysis has been completed ,from TimeSanmp [%v] to [%v] ,message number = [%d]", lastAnalysisTimesnamp, calcComplateTime, calcsumthisTurn))
		lastAnalysisTimesnamp = calcComplateTime

		//检查pub 与 所有metalife内已注册eth地址的账户的通道余额，按规定补充
		checkPubChannelBalance()

		time.Sleep(params.MsgScanInterval)
	}
}

func contactSomeone(ctx *cli.Context, dealwho string, isfollow, isblock bool) (err error) {
	arg := map[string]interface{}{
		"contact":   dealwho,
		"type":      "contact",
		"following": isfollow,
		"blocking":  isblock,
	}
	var v string
	err = client.Async(longCtx, &v, muxrpc.TypeString, muxrpc.Method{"publish"}, arg)
	if err != nil {
		return fmt.Errorf("publish call failed: %w", err)
	}
	/*newMsg, err := refs.ParseMessageRef(v)
	if err != nil {
		return err
	}*/
	//log.Log("event", "published", "type", "contact", "ref", newMsg.String())
	return
}

// GetPubWhoami
func GetPubWhoami(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> GetPubWhoami ,err=%s", resp.ToFormatString()))
		writejson(w, resp)
	}()

	pinfo := &Whoami{}
	pinfo.Pub_Id = params.PubID
	pinfo.Pub_Eth_Address = params.PhotonAddress
	resp = NewAPIResponse(nil, pinfo)
	return
}

// clientid2Profile
func clientid2Profiles(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> node-infos ,err=%s", resp.ErrorMsg))
		writejson(w, resp)
	}()

	name2addr, err := GetAllNodesProfile()
	resp = NewAPIResponse(err, name2addr)
	return
}

// clientid2Profile
func clientid2Profile(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> node-infos ,err=%s", resp.ErrorMsg))
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
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> UpdateEthAddr ,err=%s", resp.ToFormatString()))
		writejson(w, resp)
	}()
	var req = &Name2ProfileReponse{}
	err := r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = HexToAddress(req.EthAddress)
	if err != nil {
		resp = NewAPIResponse(err, nil)
		return
	}
	_, err = likeDB.UpdateUserProfile(req.ID, req.Name, req.EthAddress)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//和客户端建立一个奖励通道
	err = ChannelDeal(req.EthAddress)
	if err != nil {
		resp = NewAPIResponse(fmt.Errorf("fail to create a channel to %s, because %s", req.EthAddress, err), nil)
		return
	}
	resp = NewAPIResponse(err, "success")
}

// GetAllNodesProfile
func GetAllNodesProfile() (datas []*Name2ProfileReponse, err error) {
	profiles, err := likeDB.SelectUserProfile("")
	if err != nil {
		fmt.Println(fmt.Sprintf(PrintTime()+"Failed to db-SelectUserProfileAll", err))
		return
	}
	datas = profiles
	return
}

// GetNodeProfile
func GetNodeProfile(cid string) (datas []*Name2ProfileReponse, err error) {
	profile, err := likeDB.SelectUserProfile(cid)
	if err != nil {
		fmt.Println(fmt.Sprintf(PrintTime()+"Failed to db-SelectUserEthAddrAll", err))
		return
	}
	datas = profile
	return
}

// GetAllLikes
func GetAllLikes(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> GetAllLikes ,err=%s", resp.ErrorMsg))
		writejson(w, resp)
	}()

	likes, err := CalcGetLikeSum("")

	resp = NewAPIResponse(err, likes)
}

// GetSomeoneLike
func GetSomeoneLike(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> GetSomeoneLike ,err=%s", resp.ErrorMsg))
		writejson(w, resp)
	}()
	var req Name2ProfileReponse
	err := r.DecodeJsonPayload(&req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var cid = req.ID
	like, err := CalcGetLikeSum(cid)
	resp = NewAPIResponse(err, like)
}

// GetAllNodesProfile
func CalcGetLikeSum(someoneOrAll string) (datas map[string]*LasterNumLikes, err error) {
	likes, err := likeDB.SelectLikeSum(someoneOrAll)
	if err != nil {
		fmt.Println(fmt.Sprintf(PrintTime()+"Failed to db-SelectLikeSum", err))
		return
	}
	datas = likes
	return
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
			fmt.Println(fmt.Sprintf(PrintTime()+"Failed to InsertLikeDetail", err))
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
						timesp := time.Unix(int64(的我们负责，别人运营的出了问题他们负责，另外，客户端可以不经过pub发公msgStruct.Value.Timestamp)/1e3, 0).Format("2006-01-02 15:04:05")
						fmt.Println("like-time:\t" + timesp + "MessageKey:\t" + cvs.Vote.Link)
					}*/
					//get the Unlike tag ,先记录被like的link，再找author；由于图谱深度不一样，按照时间顺序查询存在问题，则先统一记录
					if cvs.Vote.Expression == "Unlike" {
						UnLikeDetail = append(UnLikeDetail, cvs.Vote.Link)
						timesp := time.Unix(int64(msgStruct.Value.Timestamp)/1e3, 0).Format("2006-01-02 15:04:05")
						fmt.Println(PrintTime() + "unlike-time: " + timesp + "---MessageKey: " + cvs.Vote.Link)
					} else {
						//get the Like tag ,因为like肯定在发布message后,先记录被like的link，再找author
						LikeDetail = append(LikeDetail, cvs.Vote.Link)
						timesp := time.Unix(int64(msgStruct.Value.Timestamp)/1e3, 0).Format("2006-01-02 15:04:05")
						fmt.Println(PrintTime() + "  like-time: " + timesp + "---MessageKey: " + cvs.Vote.Link)
					}
				}
			} else {
				/*fmt.Println(fmt.Sprintf("Unmarshal  for vote , err %v", err))*/
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
				fmt.Println(fmt.Sprintf(PrintTime()+"Unmarshal for about , err %v", err))
			}

			//4、contact触发对blakclist的处理
			ccs := ContentContactStru{}
			err = json.Unmarshal(msgStruct.Value.Content, &ccs)
			if err == nil {
				if ccs.Type == "contact" {
					if IsBlackList(ccs.Contact) && ccs.Following && !ccs.Blocking && ccs.Pub {
						//block he
						err = contactSomeone(nil, ccs.Contact, false, true)
						if err != nil {
							fmt.Println(fmt.Sprintf(PrintTime()+"[black-list]Unfollow and Block %s FAILED, err=%s", ccs.Contact, err))
						}
						fmt.Println(fmt.Sprintf(PrintTime()+"[black-list]Unfollow and Block %s SUCCESS", ccs.Contact))
					}
				}
			} else {
				fmt.Println(fmt.Sprintf(PrintTime()+"[black-list]Unmarshal for contact , err %v", err))
			}

			//5、敏感词处理
			cps := ContentPostStru{}
			err = json.Unmarshal(msgStruct.Value.Content, &cps)
			if err == nil {
				if cps.Type == "post" {
					postContent := cps.Text
					_, _, b := dfax.Check(postContent)
					if b {
						//block he
						err = contactSomeone(nil, msgauther, false, true)
						if err != nil {
							fmt.Println(fmt.Sprintf(PrintTime()+"[sensitive-check]Unfollow and Block %s FAILED, err=%s", msgauther, err))
						}
						fmt.Println(fmt.Sprintf(PrintTime()+"[sensitive-check]Unfollow and Block %s SUCCESS", msgauther))
					}
				}
			}
		}
	}

	//save message-result to database
	for _, likeLink := range LikeDetail { //被点赞的ID集合,标记被点赞的记录
		_, err := likeDB.UpdateLikeDetail(1, nowUnixTime, likeLink)
		if err != nil {
			fmt.Println(fmt.Sprintf(PrintTime()+"Failed to UpdateLikeDetail", err))
			return 0, err
		}
	}

	for _, unLikeLink := range UnLikeDetail { //被取消点赞的ID集合
		_, err := likeDB.UpdateLikeDetail(-1, nowUnixTime, unLikeLink)
		if err != nil {
			fmt.Println(fmt.Sprintf(PrintTime()+"Failed to UpdateLikeDetail", err))
			return 0, err
		}
	}

	_, err := likeDB.UpdateLastScanTime(nowUnixTime)
	if err != nil {
		fmt.Println(fmt.Sprintf(PrintTime()+"Failed to UpdateLastScanTime", err))
		return 0, err
	}
	//更新table userethaddr
	for key := range ClientID2Name {
		_, err := likeDB.UpdateUserProfile(key, ClientID2Name[key], "")
		if err != nil {
			fmt.Println(fmt.Sprintf(PrintTime()+"Failed to UpdateUserEthAddr", err))
			return 0, err
		}
	}
	//fmt.Println(fmt.Sprintf(PrintTime()+"A round of message data analysis has been completed ,message number = [%v]", len(TempMsgMap)))

	/*//print for test
	for key,value := range TempMsgMap {
		fmt.Println(key, "<-this round message ID---ClientID->", value.Author)
	}
	for key := range ClientID2Name { //取map中的值err
		fmt.Println(key, "<-ClientID---Name->", ClientID2Name[key])
	}*/

	return nowUnixTime, nil
}

// ChannelDeal
func ChannelDeal(partnerAddress string) (err error) {
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
		fmt.Println(fmt.Sprintf(PrintTime()+"[Pub-Client-ChannelDeal-ERROR]GetChannelWithBigInterr %s", err))
		return
	}
	if channel00 == nil {
		//create new channel with 0.1smt
		err = photonNode.OpenChannelBigInt(partnerNode.Address, params.TokenAddress, new(big.Int).Mul(big.NewInt(params.Finney), big.NewInt(100)), params.SettleTime)
		if err != nil {
			fmt.Println(fmt.Sprintf(PrintTime()+"[Pub-Client-ChannelDeal-ERROR]create channel err %s", err))
			return
		}
		fmt.Println(fmt.Sprintf(PrintTime()+"[Pub-Client-ChannelDeal-OK]create channel success, with %s", partnerAddress))

		//registration award
		err = sendToken(partnerAddress, int64(params.RegistrationAwarding), true, false)
		fmt.Println(fmt.Sprintf(PrintTime()+"[Pub-Client-ChannelDeal-OK]send registration award to %s, err=%s", partnerAddress, err))

	} else {
		fmt.Println(fmt.Sprintf(PrintTime()+"[Pub-Client-ChannelDeal-OK]channel has exist, with %s", partnerAddress))
	}

	return
}

// sendToken  pub paid additionally
// It is stipulated that 'the award' needs to be paid additionally by pub, and the 'min-balance-inchannel' is not used
func sendToken(partnerAddress string, xamount int64, isdirect, sync bool) (err error) {
	_, err = HexToAddress(partnerAddress)
	if err != nil {
		err = fmt.Errorf("[sendToken]verify eth-address [%s] error=%s", partnerAddress, err)
		return
	}
	photonNode := &PhotonNode{
		Host:       "http://" + params.PhotonHost,
		Address:    params.PhotonAddress,
		APIAddress: params.PhotonHost,
		DebugCrash: false,
	}
	amount := new(big.Int).Mul(big.NewInt(params.Finney), big.NewInt(xamount))
	err = photonNode.Deposit(partnerAddress, params.TokenAddress, amount, 48)
	if err != nil {
		return err
	}
	err = photonNode.SendTrans(params.TokenAddress, amount, partnerAddress, isdirect, sync)
	return
}

func checkPubChannelBalance() (err error) {
	name2addr, err := GetAllNodesProfile()
	for _, info := range name2addr {
		clientaddrStr := info.EthAddress
		_, err = HexToAddress(clientaddrStr)
		if err != nil {
			err = fmt.Errorf("[Pub-CheckPubChannelBalance]verify clientid [%v] 's ETH-ADDRSS error=%s", info.ID, err)
			return
		}
		pubNode := &PhotonNode{
			Host:       "http://" + params.PhotonHost,
			Address:    params.PhotonAddress,
			APIAddress: params.PhotonHost,
			DebugCrash: false,
		}
		channelX, err := pubNode.GetChannelWithBigInt(
			&PhotonNode{Address: clientaddrStr, DebugCrash: false},
			params.TokenAddress)
		if err != nil || channelX == nil {
			err = fmt.Errorf("[Pub-CheckPubChannelBalance]between pub %v and %v client,there has no channel,so no work todo", params.PhotonAddress, clientaddrStr)
			continue
		}
		var minNum = new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(int64(params.MinBalanceInchannel)))
		var nowNum = channelX.Balance
		var diffNum = new(big.Int).Sub(minNum, nowNum)
		if minNum.Cmp(nowNum) == 1 {
			//补充至MinBalanceInchannel
			err0 := pubNode.Deposit(clientaddrStr, params.TokenAddress, diffNum, 48)
			if err0 != nil {
				err = fmt.Errorf("[Pub-CheckPubChannelBalance]between pub %v and %v client,Deposit to channel err=", params.PhotonAddress, clientaddrStr, err0)
				continue
			}
			fmt.Println(fmt.Sprintf(PrintTime()+"[Pub-CheckPubChannelBalance]between pub %v and %v client,Deposit to channel success, num=%v", params.PhotonAddress, clientaddrStr, err0, diffNum))
		}
		time.Sleep(time.Second)
	}
	return
}

func IsBlackList(defendant string) bool {
	blacklists, err := likeDB.SelectViolationByWhere("", defendant, "", "", "1")
	if err != nil {
		fmt.Println(fmt.Sprintf(PrintTime()+"selectBlacklist-Failed to get blacklist, err=%s", err))
		return false
	}
	if len(blacklists) > 0 {
		return true
	}
	return false
}

/*// dealBlacklist
func dealBlacklist() {
	for {
		time.Sleep(time.Second * 600)
		//get blacklist info
		blacklists, err := likeDB.SelectViolationByWhere("", "", "", "", "1")
		if err != nil {
			fmt.Println(fmt.Sprintf(PrintTime()+"dealBlacklist-Failed to get blacklist, err=%s", err))
		}
		for _, info := range blacklists {
			dealObj := info.Defendant
			//block him
			err = contactSomeone(nil, dealObj, false, true)
			if err != nil {
				fmt.Println(fmt.Sprintf("dealBlacklist-Unfollow and block %s failed", dealObj))
			}
			fmt.Println(fmt.Sprintf(PrintTime()+"dealBlacklist-Success to Unfollow and Block %s", dealObj))
			time.Sleep(time.Second * 3)
			//award plaintiff
			plaintiff := info.Plaintiff
			dealReward := info.Dealreward
			if strings.Index(dealReward, "-") != -1 {
				//awards have been issued
			} else {
				// No awards have been issued yet, for some reason
				name2addr, err := GetNodeProfile(plaintiff)
				if err != nil {
					fmt.Println(fmt.Sprintf("dealBlacklist-Get plaintiff's profile failed, err=%s", err))
					continue
				}
				if len(name2addr) != 1 {
					continue
				}
				addrPlaintiff := name2addr[0].EthAddress

				//另行支付
				err = sendToken(addrPlaintiff, int64(params.ReportRewarding), true, false)
				if err != nil {
					fmt.Println(fmt.Sprintf(PrintTime()+"dealBlacklist-Failed to Award to %s for ReportRewarding, err=%s", plaintiff, err))
					continue
				}
				fmt.Println(fmt.Sprintf(PrintTime()+"dealBlacklist-Success to Award to %s for ReportRewarding", plaintiff))
				_, err = likeDB.UpdateViolation(info.DealTag, info.Dealtime, string(params.ReportRewarding)+"-", plaintiff, dealObj, info.MessageKey)
				if err != nil {
					fmt.Println(fmt.Sprintf(PrintTime()+"dealBlacklist-Failed to Update ReportRewarding to %s", plaintiff))
					continue
				}

			}

		}
	}
}*/
