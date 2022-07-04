package restful

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"

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

	/*"go.cryptoscope.co/ssb/message"
	"go.mindeco.de/ssb-refs"*/

	"bufio"
	"os"

	"math"

	"errors"

	"go.cryptoscope.co/ssb"
	"go.cryptoscope.co/ssb/dfa"
	"go.cryptoscope.co/ssb/message"
	refs "go.mindeco.de/ssb-refs"
)

var Config *params.ApiConfig

var longCtx context.Context

var quitSignal chan struct{}

var client *ssbClient.Client

var log kitlog.Logger

var lastAnalysisTimesnamp int64

var likeDB *PubDB

var dfax *dfa.DFA

const (
	SignUp                = "sign up"
	PostMessage           = "post message"
	PostComment           = "post comment"
	MintNft               = "mint a nft"
	DailyLogin            = "daily login"
	LikePost              = "like a post"
	ReceiveLike           = "receive a like"
	ReportProblematicPost = "report problematic post"
)

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

		/*
			ssb pub信息
		*/
		//pub's whoami
		rest.Get("/ssb/api/pub-whoami", GetPubWhoami),

		/*
			ssb节点注册,信息查询,例如查询其绑定的钱包地址
		*/
		//get all 'about' message,e.g:'about'='eth address'
		rest.Get("/ssb/api/node-info", clientid2Profiles),
		//get the 'about' message by client id ,e.g:'about'='eth address'
		rest.Post("/ssb/api/node-info", clientid2Profile),
		//register client's eth address to it's ID
		rest.Post("/ssb/api/id2eth", UpdateEthAddr),

		/*
			受赞统计
		*/
		//likes of all client
		rest.Get("/ssb/api/likes", GetAllLikes),
		//likes of someone client
		rest.Post("/ssb/api/likes", GetSomeoneLike),

		/*
			点赞统计
		*/
		//get set like infos of all
		rest.Get("/ssb/api/set-like-info", GetAllSetLikes),
		//get set like info of someone client
		rest.Post("/ssb/api/set-like-info", GetSomeoneSetLikes),

		/*
			举报
		*/
		// tipped someone off 举报
		rest.Post("/ssb/api/tipped-who-off", TippedOff),
		//tipped off infomation 所有举报的信息汇总
		rest.Get("/ssb/api/tippedoff-info", GetTippedOffInfo),
		//tippedoff-deal pub管理员对举报的信息进行处理，认证，如属实，则对该账号进行黑名单处理
		rest.Post("/ssb/api/tippedoff-deal", DealTippedOff),

		/*
			敏感词
		*/
		//DealSensitiveWord pub管理对敏感词的处理/block or ignore
		rest.Post("/ssb/api/sensitive-word-deal", DealSensitiveWord),
		//get all sensitive-word-events from pub
		rest.Post("/ssb/api/sensitive-word-events", GetEventSensitiveWord),

		/*
			用户每日任务,数据类型：1-登录 2-发帖(Pub自动处理) 3-评论(Pub自动处理) 4-铸造NFT
		*/
		//notify pub the login infomation, pub will collect through this interface
		rest.Post("/ssb/api/notify-login", NotifyUserLogin),
		//[temporary scheme] notify the pub that user have created a NFT in metalife app
		rest.Post("/ssb/api/notify-created-nft", NotifyCreatedNFT),
		//get some user daily task infos from pub,
		//a message may appear in multiple pubs, and the client removes redundant data through messagekey and pub id
		//used by supernode to awarding or ssb-client
		rest.Post("/ssb/api/get-user-daily-task", GetUserDailyTasks),

		/*
			激励查询
		*/
		//get all or someones' reward information in PUB RULE
		rest.Get("/ssb/api/get-reward-info", GetRewardInfo),
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

	//检查pub 与 所有metalife内已注册eth地址的账户的通道余额，按规定补充
	//每隔10分钟检查一次
	go checkPubChannelBalance()

	//补发激励，
	go backPay()

	<-quitSignal
	err = server.Shutdown(context.Background())
	if err != nil {
		fmt.Println(fmt.Sprintf(PrintTime()+"API restful service Shutdown err : %s", err))
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
		fmt.Println(fmt.Errorf("Failed to create database", err))
	}

	lstime, err := likedb.SelectLastScanTime()
	if err != nil {
		fmt.Println(fmt.Errorf("Failed to init database", err))
	}
	if lstime == 0 {
		_, err = likedb.UpdateLastScanTime(0)
		if err != nil {
			fmt.Println(fmt.Errorf("Failed to init database", err))
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
			fmt.Println(fmt.Errorf(PrintTime()+"Source stream call failed: %w ,will try other tcp connect socket...", err))
			otherClient, err := newClient(ctx)
			if err != nil {
				fmt.Println(fmt.Errorf(PrintTime()+"Try set up a ssb client tcp socket failed , will try again...", err))
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

		time.Sleep(params.MsgScanInterval)
	}
}

func contactSomeone(ctx *cli.Context, dealwho string, isfollow, isblock bool) (err error) {
	if dealwho == params.PubID {
		return fmt.Errorf("Permission denied, from pub : %s", dealwho)
	}
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

func privatePublish(ctx *cli.Context, recpobj, root, branch string) (err error) {
	arg := map[string]interface{}{
		"text": ctx.Args().First(),
		"type": "post",
	}
	if r := ctx.String("root"); r != "" {
		arg["root"] = r
		if b := ctx.String("branch"); b != "" {
			arg["branch"] = b
		} else {
			arg["branch"] = r
		}
	}
	var v string
	if recps := ctx.StringSlice("recps"); len(recps) > 0 {
		err = client.Async(longCtx, &v,
			muxrpc.TypeString,
			muxrpc.Method{"private", "publish"}, arg, recps)
	} else {
		err = client.Async(longCtx, &v,
			muxrpc.TypeString,
			muxrpc.Method{"publish"}, arg)
	}
	if err != nil {
		return fmt.Errorf("publish call failed: %w", err)
	}
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
			fmt.Println(fmt.Errorf("Muxrpc.ByteSource Unmarshal to json err =%s", err))
			return 0, err
		}

		//1、记录本轮所有消息ID和author的关系,保存下来,被点赞的消息基本不会在本轮被扫描到
		msgkey := fmt.Sprintf("%v", msgStruct.Key)
		msgauther := fmt.Sprintf("%v", msgStruct.Value.Author)
		var msgtime = msgStruct.Value.Timestamp
		var msgTime = int64(msgtime*math.Pow10(2)) / 100

		TempMsgMap[msgkey] = &TempdMessage{
			Author: msgauther,
		}
		_, err = likeDB.InsertLikeDetail(msgkey, msgauther)
		if err != nil {
			fmt.Println(fmt.Errorf(PrintTime()+"Failed to InsertLikeDetail, err=%s", err))
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
					timesp := time.Unix(int64(msgStruct.Value.Timestamp)/1e3, 0).Format("2006-01-02 15:04:05")
					if cvs.Vote.Expression == "Unlike" {
						UnLikeDetail = append(UnLikeDetail, cvs.Vote.Link)
						fmt.Println(PrintTime() + "unlike-time: " + timesp + "---MessageKey: " + cvs.Vote.Link)

						//统计我取消点赞的
						_, err = likeDB.InsertUserSetLikeInfo(msgkey, msgauther, -1, msgTime)
						if err != nil {
							fmt.Println(fmt.Errorf(PrintTime()+" %s set a unlike FAILED, err=%s", msgauther, err))
						}
						fmt.Println(fmt.Sprintf(PrintTime()+" %s set a unlike, msgkey=%s", msgauther, msgkey))
					} else {
						//get the Like tag ,因为like肯定在发布message后,先记录被like的link，再找author
						LikeDetail = append(LikeDetail, cvs.Vote.Link)
						fmt.Println(PrintTime() + "  like-time: " + timesp + "---MessageKey: " + cvs.Vote.Link)

						//统计我点赞的
						_, err = likeDB.InsertUserSetLikeInfo(msgkey, msgauther, 1, msgTime)
						if err != nil {
							fmt.Println(fmt.Errorf(PrintTime()+" %s set a like FAILED, err=%s", msgauther, err))
						}
						fmt.Println(fmt.Sprintf(PrintTime()+" %s set a like, msgkey=%s", msgauther, msgkey))

						{ //发送激励
							//如果点赞了，又取消了，不影响token的发放
							name2addr, err := GetNodeProfile(msgauther)
							if err != nil || len(name2addr) != 1 {
								fmt.Println(fmt.Errorf(LikePost+" Reward %s ethereum address failed, err= not found or %s", msgauther, err))
							} else {
								ehtAddr := name2addr[0].EthAddress
								go PubRewardToken(ehtAddr, int64(params.RewardOfLikePost), msgauther, LikePost, msgkey, msgTime)
							}
						}
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
				fmt.Println(fmt.Errorf(PrintTime()+"Unmarshal for about , err %v", err))
			}

			//4、contact触发对blakclist的处理, 通过pub关注重新进来的黑名单的消息来持续block该账户
			if msgauther == params.PubID {
				ccs := ContentContactStru{}
				err = json.Unmarshal(msgStruct.Value.Content, &ccs)
				if err == nil {
					if ccs.Type == "contact" {
						if IsBlackList(ccs.Contact) && ccs.Following && ccs.Pub {
							//block he
							err = contactSomeone(nil, ccs.Contact, true, true)
							if err != nil {
								fmt.Println(fmt.Errorf(PrintTime()+"[black-list]Unfollow and Block %s FAILED, err=%s", ccs.Contact, err))
							}
							fmt.Println(fmt.Sprintf(PrintTime()+"[black-list]Unfollow and Block %s SUCCESS", ccs.Contact))
						}
					}
				} else {
					fmt.Println(fmt.Errorf(PrintTime()+"[black-list]Unmarshal for contact, err %v", err))
				}
			}

			//5、POST Message
			cps := ContentPostStru{}
			err = json.Unmarshal(msgStruct.Value.Content, &cps)
			if err == nil {
				if cps.Type == "post" {
					postContent := cps.Text
					//5.1敏感词处理
					_, _, b := dfax.Check(postContent)
					if b && (msgauther != params.PubID) {
						/*//block he
						err = contactSomeone(nil, msgauther, true, true)
						if err != nil {
							fmt.Println(fmt.Sprintf(PrintTime()+"[sensitive-check]Unfollow and Block %s FAILED, err=%s", msgauther, err))
						}
						fmt.Println(fmt.Sprintf(PrintTime()+"[sensitive-check]Unfollow and Block %s SUCCESS", msgauther))*/
						//fix:处理违规消息由 "直接block" 转为 "提供接口人工审核处理"
						_, err = likeDB.InsertSensitiveWordRecord(params.PubID, nowUnixTime, postContent, msgkey, msgauther, "0")
						if err != nil {
							fmt.Println(fmt.Errorf(PrintTime()+"[sensitive-check]InsertSensitiveWordRecord FAILED, err=%s", err))
						}
						fmt.Println(fmt.Sprintf(PrintTime()+"[sensitive-check]InsertSensitiveWordRecord SUCCESS, author=%s, message=%s, msgkey=%s", msgauther, postContent, msgkey))
					}
					//5.2我发表的invitation
					if cps.Root == "" && PostWordCountBigThan10(postContent) { //1-登录 2-发表帖子 3-评论 4-铸造NFT
						_, err = likeDB.InsertUserTaskCollect(params.PubID, msgauther, msgkey, "2", "", msgTime, "", "", "")
						if err != nil {
							fmt.Println(fmt.Errorf(PrintTime()+"[UserTaskCollect-post]InsertUserTaskCollect FAILED, err=%s", err))
						}
						fmt.Println(fmt.Sprintf(PrintTime()+"[UserTaskCollect-post]InsertUserTaskCollect SUCCESS, author=%s, msgkey=%s", msgauther, msgkey))

						{ //发送激励
							name2addr, err := GetNodeProfile(msgauther)
							if err != nil || len(name2addr) != 1 {
								fmt.Println(fmt.Errorf(PostMessage+" Reward %s ethereum address failed, err= not found or %s", msgauther, err))
							} else {
								ehtAddr := name2addr[0].EthAddress
								go PubRewardToken(ehtAddr, int64(params.RewardOfPostMessage), msgauther, PostMessage, msgkey, msgTime)
							}
						}
					}
					//5.3我发表的comment
					if cps.Root != "" && PostWordCountBigThan10(postContent) {
						_, err = likeDB.InsertUserTaskCollect(params.PubID, msgauther, msgkey, "3", cps.Root, msgTime, "", "", "")
						if err != nil {
							fmt.Println(fmt.Errorf(PrintTime()+"[UserTaskCollect-comment]InsertUserTaskCollect FAILED, err=%s", err))
						}
						fmt.Println(fmt.Sprintf(PrintTime()+"[UserTaskCollect-comment]InsertUserTaskCollect SUCCESS, author=%s, msgkey=%s", msgauther, msgkey))

						{ //发送激励
							name2addr, err := GetNodeProfile(msgauther)
							if err != nil || len(name2addr) != 1 {
								fmt.Println(fmt.Errorf(PostComment+" Reward %s ethereum address failed, err= not found or %s", msgauther, err))
							} else {
								ehtAddr := name2addr[0].EthAddress
								go PubRewardToken(ehtAddr, int64(params.RewardOfPostComment), msgauther, PostComment, msgkey, msgTime)
							}
						}
					}
				}
			} else {
				fmt.Println(fmt.Errorf("json.Unmarshal(msgStruct.Value.Content err=%s", err))
			}
		}
	}

	//save message-result to database
	for _, likeLink := range LikeDetail { //被点赞的ID集合,标记被点赞的记录
		_, err := likeDB.UpdateLikeDetail(1, nowUnixTime, likeLink)
		if err != nil {
			fmt.Println(fmt.Errorf(PrintTime()+"Failed to UpdateLikeDetail", err))
			return 0, err
		}
	}

	for _, unLikeLink := range UnLikeDetail { //被取消点赞的ID集合
		_, err := likeDB.UpdateLikeDetail(-1, nowUnixTime, unLikeLink)
		if err != nil {
			fmt.Println(fmt.Errorf(PrintTime()+"Failed to UpdateLikeDetail", err))
			return 0, err
		}
	}

	_, err := likeDB.UpdateLastScanTime(nowUnixTime)
	if err != nil {
		fmt.Println(fmt.Errorf(PrintTime()+"Failed to UpdateLastScanTime", err))
		return 0, err
	}
	//更新table userethaddr
	for key := range ClientID2Name {
		_, err := likeDB.UpdateUserProfile(key, ClientID2Name[key], "")
		if err != nil {
			fmt.Println(fmt.Errorf(PrintTime()+"Failed to UpdateUserEthAddr", err))
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
func NewChannelDeal(partnerAddress string, clientID string, messageTime int64) (err error) {
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

	channel00, err := photonNode.GetChannelWith(partnerNode, params.TokenAddress)
	if err != nil {
		fmt.Println(fmt.Errorf(PrintTime()+SignUp+" GetChannelWith %s", err))
		return
	}
	if channel00 == nil {
		if ExceedRewardLimit(clientID, SignUp) {
			//如果一个SSB-ID连续注册地址达到2次以上，则该账号以后无法得到注册激励
			fmt.Println(fmt.Errorf(PrintTime()+SignUp+" reward %s to ethaddr=%s REJECT,reason:ExceedRewardLimit", clientID, partnerAddress))
			return
		}
		//create new channel with 1 mlt
		initRegistAmount := int64(params.MinBalanceInchannel + params.RewardOfSignup)
		err = photonNode.OpenChannel(partnerNode.Address, params.TokenAddress, new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(initRegistAmount)), params.SettleTime)
		if err != nil {
			fmt.Println(fmt.Errorf(PrintTime()+SignUp+" create channel, err=%s", err))
			return
		}
		fmt.Println(fmt.Sprintf(PrintTime()+SignUp+" create channel success[%s], with %s", clientID, partnerAddress))

		netStatus := false
		for i := 0; i < 20; i++ {
			nodeS, err := photonNode.GetNodeStatus(partnerAddress)
			if err != nil {
				fmt.Println(fmt.Errorf(PrintTime()+SignUp+" GetNodeStatus[%s], err=%s", clientID, err))
			}
			netStatus = nodeS.IsOnline
			if netStatus {
				break
			}
			time.Sleep(time.Second * 120)
		}
		if !netStatus {
			fmt.Println(fmt.Errorf(PrintTime()+SignUp+" GetNodeStatus[%s](retry 20 times) %s online=%v", clientID, partnerAddress, netStatus))
			return errors.New("partner offline")
		}
		//registration award 新地址才发送注册激励
		amount := new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(int64(params.RewardOfSignup)))
		err = photonNode.SendTrans(params.TokenAddress, amount, partnerAddress, true, false)
		if err != nil {
			return err
		}
		fmt.Println(fmt.Sprintf(PrintTime()+SignUp+" award[%s] to %s, err=%s", clientID, partnerAddress, err))

		//继续发送SMT激励
		err = photonNode.TransferSMT(partnerAddress, new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(int64(params.RewardOfSignupSMT))).String())
		if err != nil {
			return err
		}
		fmt.Println(fmt.Sprintf(PrintTime()+SignUp+" award(SMT)[%s] to %s, err=%s", clientID, partnerAddress, err))

		{
			//=======Record Reward Result=======
			nowTime := time.Now().UnixNano() / 1e6
			_, err = likeDB.RecordRewardResult(clientID, partnerAddress, "success", amount.Int64(), SignUp, "", messageTime, nowTime)
			fmt.Println(fmt.Sprintf(PrintTime()+"===> Pub[RecordRewardResult] reword to eth-address=%s for clientid=%s, reason=%s, err=%s", partnerAddress, clientID, err))
		}

	} else {
		fmt.Println(fmt.Errorf(PrintTime()+"[Pub-Client-ChannelDeal-OK]channel has exist[%s], with %s", clientID, partnerAddress))
	}

	return
}

// PubRewardToken  pub paid additionally
// It is stipulated that 'the award' needs to be paid additionally by pub, and the 'min-balance-inchannel' is not used
func PubRewardToken(partnerAddress string, xamount int64, clientID, reason, messageKey string, messageTime int64) (err error) {
	_, err = HexToAddress(partnerAddress)
	if err != nil {
		err = fmt.Errorf("[sendToken]verify eth-address=[%s], error=%s", partnerAddress, err)
		return
	}
	photonNode := &PhotonNode{
		Host:       "http://" + params.PhotonHost,
		Address:    params.PhotonAddress,
		APIAddress: params.PhotonHost,
		DebugCrash: false,
	}
	netStatus := false
	for i := 0; i < 18; i++ {
		nodeS, err := photonNode.GetNodeStatus(partnerAddress)
		if err != nil {
			fmt.Println(fmt.Sprintf(PrintTime()+reason+"[sendToken]GetNodeStatus, err=%s", err))
		}
		netStatus = nodeS.IsOnline
		if netStatus {
			break
		}
		time.Sleep(time.Second * 10)
	}
	if ExceedRewardLimit(clientID, reason) {
		fmt.Println(fmt.Errorf(PrintTime()+reason+" reward %s to ethaddr=%s REJECT,reason:ExceedRewardLimit", clientID, partnerAddress))
		return
	}
	amount := new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(xamount))
	if !netStatus {
		fmt.Println(fmt.Errorf(PrintTime()+reason+"[sendToken]GetNodeStatus(retry 120 minites) %s online= %s", partnerAddress, netStatus))
		//如果此时客户端不在线，则先记录，后续补发
		{
			//=======Record Reward Result=======
			_, err = likeDB.RecordRewardResult(clientID, partnerAddress, "fail", amount.Int64(), reason, messageKey, messageTime, 0)
			fmt.Println(fmt.Sprintf(PrintTime()+"===> Pub[RecordRewardResult] reword to eth-address=%s for clientid=%s, reason=%s, err=%s", partnerAddress, clientID, err))
		}
		return errors.New("partner offline")
	}

	/*err = photonNode.Deposit(partnerAddress, params.TokenAddress, amount, 48)
	if err != nil {
		fmt.Println(fmt.Errorf(PrintTime()+reason+" [sendToken]Deposit error=%s", err))
		return err
	}*/
	//直接链下转，保证通道有足够的余额
	/*channelexist := photonNode.CheckChannelExist(partnerAddress, params.TokenAddress)
	if !channelexist {
		fmt.Println(fmt.Sprintf(PrintTime()+reason+" reward %s to ethaddr=%s FAILED, because channel not exist", clientID, partnerAddress))
		{
			//=======Record Reward Result=======
			_, err = likeDB.RecordRewardResult(clientID, partnerAddress, "fail", amount.Int64(), reason, messageKey, messageTime, 0)
			fmt.Println(fmt.Sprintf(PrintTime()+"===> Pub[RecordRewardResult] reword to eth-address=%s for clientid=%s, reason=%s, err=%s", partnerAddress, clientID, err))
		}
		return errors.New("channel no exist with " + partnerAddress)
	}*/
	//如果不在线了，查询通道没有任何意义
	err = photonNode.SendTrans(params.TokenAddress, amount, partnerAddress, true, false)
	if err != nil {
		fmt.Println(fmt.Errorf(PrintTime()+reason+" [sendToken]SendTrans error=%s", err))
		return err
	}
	fmt.Println(fmt.Sprintf(PrintTime()+reason+" reward %s to ethaddr=%s SUCCESS", clientID, partnerAddress))
	{
		//=======Record Reward Result=======
		nowTime := time.Now().UnixNano() / 1e6
		_, err = likeDB.RecordRewardResult(clientID, partnerAddress, "success", amount.Int64(), reason, messageKey, messageTime, nowTime)
		if err != nil {
			fmt.Println(fmt.Sprintf(PrintTime()+"===> Pub[RecordRewardResult] reword to eth-address=%s for clientid=%s, reason=%s, FAILED, err=%s", partnerAddress, clientID, reason, err))
		}
		fmt.Println(fmt.Sprintf(PrintTime()+"===> Pub[RecordRewardResult] reword to eth-address=%s for clientid=%s, reason=%s, SUCCESS", partnerAddress, clientID, reason))
	}
	return
}

func checkPubChannelBalance() {
	time.Sleep(time.Second * 5) //数据库可能没准备好
	name2addr, err := GetAllNodesProfile()
	for _, info := range name2addr {
		clientaddrStr := info.EthAddress
		if clientaddrStr == "" {
			continue
		}
		_, err = HexToAddress(clientaddrStr)
		if err != nil {
			fmt.Println(fmt.Errorf("[Pub-CheckPubChannelBalance]verify clientid=[%s] 's eth-address=%s, error=%s", info.ID, clientaddrStr, err))
			continue
		}
		pubNode := &PhotonNode{
			Host:       "http://" + params.PhotonHost,
			Address:    params.PhotonAddress,
			APIAddress: params.PhotonHost,
			DebugCrash: false,
		}
		channelX, err := pubNode.GetChannelWith(
			&PhotonNode{Address: clientaddrStr, DebugCrash: false},
			params.TokenAddress)
		if err != nil || channelX == nil {
			fmt.Println(fmt.Errorf("[Pub-CheckPubChannelBalance]between pub %v and %v client,there has no channel,so no work todo", params.PhotonAddress, clientaddrStr))
			continue
		}
		var minNum = new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(int64(params.MinBalanceInchannel)))
		var nowNum = channelX.Balance
		var diffNum = new(big.Int).Sub(minNum, nowNum)
		if minNum.Cmp(nowNum) == 1 {
			//补充至MinBalanceInchannel
			err0 := pubNode.Deposit(clientaddrStr, params.TokenAddress, diffNum, 48)
			if err0 != nil {
				fmt.Println(fmt.Errorf("[Pub-CheckPubChannelBalance]between pub %v and %v client,Deposit to channel err=%s", params.PhotonAddress, clientaddrStr, err0))
				continue
			}
			fmt.Println(fmt.Sprintf(PrintTime()+"[Pub-CheckPubChannelBalance]between pub %v and %v client,Deposit to channel SUCCESS, num=%v", params.PhotonAddress, clientaddrStr, diffNum))
		}
		time.Sleep(time.Second)
	}

	time.AfterFunc(10*time.Minute, checkPubChannelBalance)
}

func backPay() {

}

func IsBlackList(defendant string) bool {
	blacklists, err := likeDB.SelectViolationByWhere("", defendant, "", "", "1")
	if err != nil {
		fmt.Println(fmt.Errorf(PrintTime()+"selectBlacklist-Failed to get blacklist, err=%s", err))
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
