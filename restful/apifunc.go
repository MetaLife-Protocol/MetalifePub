package restful

import (
	"fmt"
	"net/http"
	"time"

	"strings"

	"net"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ip2location/ip2location-go/v9"
	"go.cryptoscope.co/ssb/restful/params"
)

// clientPublicIP
func clientPublicIP(r *http.Request) string {
	var ip string
	for _, ip = range strings.Split(r.Header.Get("X-Forwarded-For"), ",") {
		ip = strings.TrimSpace(ip)
		if ip != "" && !HasLocalIPddr(ip) {
			return ip
		}
	}
	ip = strings.TrimSpace(r.Header.Get("X-Real-Ip"))
	if ip != "" && !HasLocalIPddr(ip) {
		return ip
	}
	if ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr)); err == nil {
		if !HasLocalIPddr(ip) {
			return ip
		}
	}
	return ""
}

// HasLocalIPddr
func HasLocalIPddr(ip string) bool {
	return HasLocalIPAddr(ip)
}

// HasLocalIPAddr
func HasLocalIPAddr(ip string) bool {
	return HasLocalIP(net.ParseIP(ip))
}

// HasLocalIP
func HasLocalIP(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}

	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}

	return ip4[0] == 10 || // 10.0.0.0/8
		(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) || // 172.16.0.0/12
		(ip4[0] == 169 && ip4[1] == 254) || // 169.254.0.0/16
		(ip4[0] == 192 && ip4[1] == 168) // 192.168.0.0/16
}

// GetPublicIPLocation
func GetPublicIPLocation(w rest.ResponseWriter, r *rest.Request) {
	clientpublicip := clientPublicIP(r.Request)
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> GetPublicIpLocation ,err=%s", resp.ErrorMsg))
		writejson(w, resp)
	}()
	/*var req IPLoacation
	err := r.DecodeJsonPayload(&req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var ip = req.PublicIp*/
	if clientpublicip == "" {
		clientpublicip = strings.Split(params.InviteCodeOfPub2, ":")[0]
	}
	var ip = clientpublicip

	db, err := ip2location.OpenDB(params.Ip2LocationLiteDbPath)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	result, err := db.Get_all(ip)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	countryLong := result.Country_long

	pbi := &PubInfoByIP{}
	pbi.ReqPublicIP = clientpublicip
	pbi.ContryShort = result.Country_short
	pbi.ContryLong = countryLong
	pbi.Region = result.Region
	pbi.City = result.City

	if countryLong == "China" {
		pbi.FirstChoicePubHost = fmt.Sprintf("%s:%d", strings.Split(params.InviteCodeOfPub1, ":")[0], params.ServePort)
		pbi.FirstChoicePubInviteCode = params.InviteCodeOfPub1
		pbi.SecondChoicePubHost = fmt.Sprintf("%s:%d", strings.Split(params.InviteCodeOfPub2, ":")[0], params.ServePort)
		pbi.SecondChoicePubInviteCode = params.InviteCodeOfPub2
	} else {
		pbi.FirstChoicePubHost = fmt.Sprintf("%s:%d", strings.Split(params.InviteCodeOfPub2, ":")[0], params.ServePort)
		pbi.FirstChoicePubInviteCode = params.InviteCodeOfPub2
		pbi.SecondChoicePubHost = fmt.Sprintf("%s:%d", strings.Split(params.InviteCodeOfPub1, ":")[0], params.ServePort)
		pbi.SecondChoicePubInviteCode = params.InviteCodeOfPub1
	}
	resp = NewAPIResponse(err, pbi)
}

// GetSomeoneLike
func GetRewardInfo(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> GetRewardInfo ,err=%s", resp.ErrorMsg))
		writejson(w, resp)
	}()
	var req RewardingReq
	err := r.DecodeJsonPayload(&req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var clientid = req.ClientID
	//var grandsuccess = req.GrantSuccess
	//var rewardreason = req.RewardReason
	var timefrom = req.TimeFrom
	var timeTo = req.TimeTo

	rresult, err := likeDB.SelectRewardResult(clientid, timefrom, timeTo)
	resp = NewAPIResponse(err, rresult)
}

func GetRewardSubtotals(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> GetRewardSubtotals ,err=%s", resp.ErrorMsg))
		writejson(w, resp)
	}()
	var req RewardingReq
	err := r.DecodeJsonPayload(&req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var clientid = req.ClientID
	var grandsuccess = req.GrantSuccess
	//var rewardreason = req.RewardReason
	var timefrom = req.TimeFrom
	var timeTo = req.TimeTo

	rresult, err := likeDB.SelectRewardSum(clientid, grandsuccess, timefrom, timeTo)

	resp = NewAPIResponse(err, rresult)
}

// GetAllSetLikes
func GetAllSetLikes(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> GetAllSetLikes ,err=%s", resp.ErrorMsg))
		writejson(w, resp)
	}()

	setlikes, err := likeDB.SelectUserSetLikeInfo("")
	resp = NewAPIResponse(err, setlikes)
}

// GetSomeoneLike
func GetSomeoneSetLikes(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> GetSomeoneSetLikes ,err=%s", resp.ErrorMsg))
		writejson(w, resp)
	}()
	var req Name2ProfileReponse
	err := r.DecodeJsonPayload(&req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var cid = req.ID
	setlikes, err := likeDB.SelectUserSetLikeInfo(cid)
	resp = NewAPIResponse(err, setlikes)
}

// NotifyCreatedNFT
func NotifyCreatedNFT(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> NotifyCreatedNFT ,err=%s", resp.ErrorMsg))
		writejson(w, resp)
	}()
	var req ReqCreatedNFT
	err := r.DecodeJsonPayload(&req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var cid = req.ClientID
	var ctime = req.NftCreatedTime
	var tx = req.NfttxHash
	var tokenid = req.NftTokenId
	var storeurl = req.NftStoredUrl
	_, err = likeDB.InsertUserTaskCollect(params.PubID, cid, "", "4", "", ctime, tx, tokenid, storeurl)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	{ //发送激励
		name2addr, err := GetNodeProfile(cid)
		if err != nil || len(name2addr) != 1 {
			fmt.Println(fmt.Errorf(MintNft+" Reward %s ethereum address failed, err= not found or %s", cid, err))
		} else {
			ehtAddr := name2addr[0].EthAddress
			go PubRewardToken(ehtAddr, int64(params.RewardOfMintNft), cid, MintNft, "", time.Now().UnixNano()/1e6)
		}
	}

	resp = NewAPIResponse(err, "Success")
}

// NotifyUserLogin
func NotifyUserLogin(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> NotifyUserLogin ,err=%s", resp.ErrorMsg))
		writejson(w, resp)
	}()
	var req ReqUserLoginApp
	err := r.DecodeJsonPayload(&req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var cid = req.ClientID
	var logintime = req.LoginTime
	_, err = likeDB.InsertUserTaskCollect(params.PubID, cid, "", "1", "", logintime, "", "", "")
	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	{ //发送激励
		name2addr, err := GetNodeProfile(cid)
		if err != nil || len(name2addr) != 1 {
			fmt.Println(fmt.Errorf(DailyLogin+" Reward %s ethereum address failed, err= not found or %s", cid, err))
		} else {
			ehtAddr := name2addr[0].EthAddress
			go PubRewardToken(ehtAddr, int64(params.RewardOfDailyLogin), cid, DailyLogin, "", time.Now().UnixNano()/1e6)
		}
	}
	resp = NewAPIResponse(err, "Success")
}

// GetUserDailyTasks
func GetUserDailyTasks(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> GetUserDailyTasks ,err=%s", resp.ErrorMsg))
		writejson(w, resp)
	}()
	var req ReqUserTask
	err := r.DecodeJsonPayload(&req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var author = req.Author
	var msgtype = req.MessageType
	var starttime = req.StartTime
	var endtime = req.EndTime

	taskcollctions, err := likeDB.GetUserTaskCollect(author, msgtype, starttime, endtime)
	resp = NewAPIResponse(err, taskcollctions)
}

// GetEventSensitiveWord
func GetEventSensitiveWord(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> GetEventSensitiveWord ,err=%s", resp.ErrorMsg))
		writejson(w, resp)
	}()
	var req EventSensitive
	err := r.DecodeJsonPayload(&req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var tag = req.DealTag
	senvents, err := likeDB.SelectSensitiveWordRecord(tag)
	resp = NewAPIResponse(err, senvents)
}

// DealSensitiveWord
func DealSensitiveWord(w rest.ResponseWriter, r *rest.Request) {
	var resp *APIResponse
	defer func() {
		fmt.Println(fmt.Sprintf(PrintTime()+"Restful Api Call ----> DealSensitiveWord ,err=%s", resp.ToFormatString()))
		writejson(w, resp)
	}()

	var req EventSensitive
	err := r.DecodeJsonPayload(&req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var msgkey = req.MessageKey
	var dealtag = req.DealTag
	var dealtime = time.Now().UnixNano() / 1e6
	var author = req.MessageAuthor
	_, err = likeDB.UpdateSensitiveWordRecord(dealtag, dealtime, msgkey)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if req.DealTag == "1" { ////for table sensitivewordrecord, dealtag=0初始化  =1属实 =2否定
		// block 'the author who publish sensitive word' ONCE
		err = contactSomeone(nil, author, true, true)
		if err != nil {
			resp = NewAPIResponse(err, fmt.Sprintf("block %s failed", author))
			return
		}
		fmt.Println(fmt.Sprintf(PrintTime()+"Success to block %s", author))
	}
	resp = NewAPIResponse(err, "success")
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

	if defendant == params.PubID {
		resp = NewAPIResponse(err, fmt.Sprintf("Permission denied, from pub : %s", params.PubID))
		return
	}
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
		err = contactSomeone(nil, req.Defendant, true, true)
		if err != nil {
			resp = NewAPIResponse(err, fmt.Sprintf("Unfollow and block %s failed, err=%s", req.Defendant, err))
			return
		}
		fmt.Println(fmt.Sprintf(PrintTime()+"Success to Unfollow and block %s", req.Defendant))

		{ //发送激励
			name2addr, err := GetNodeProfile(req.Plaintiff)
			if err != nil || len(name2addr) != 1 {
				fmt.Println(fmt.Errorf(ReportProblematicPost+" Reward %s ethereum address failed, err= not found or %s", req.Plaintiff, err))
			} else {
				ehtAddr := name2addr[0].EthAddress
				go PubRewardToken(ehtAddr, int64(params.RewardOfReportProblematicPost), req.Plaintiff, ReportProblematicPost, req.MessageKey, dtime)
			}
		}

		_, err = likeDB.UpdateViolation(req.DealTag, dtime, fmt.Sprintf("%d%s", params.RewardOfReportProblematicPost, "e18-"), req.Plaintiff, req.Defendant, req.MessageKey)
		if err != nil {
			rest.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp = NewAPIResponse(err, fmt.Sprintf("success, [%s] has been block by [pub administrator], and pub send award token to [%s]", req.Defendant, req.Plaintiff))
		return
	}
	resp = NewAPIResponse(err, "success")
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

	/*//此处跳过校验，前端不好处理
	_, err = HexToAddress(req.EthAddress)
	if err != nil {
		resp = NewAPIResponse(err, nil)
		return
	}*/
	ethAddress := common.HexToAddress(req.EthAddress)
	_, err = likeDB.UpdateUserProfile(req.ID, req.Name, ethAddress.String())
	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//和客户端建立一个奖励通道
	//修改为先返回结果
	/*err = NewChannelDeal(ethAddress.String())
	if err != nil {
		resp = NewAPIResponse(fmt.Errorf("fail to create a channel to %s, because %s", ethAddress.String(), err), nil)
		return
	}*/
	go NewChannelDeal(ethAddress.String(), req.ID, time.Now().UnixNano()/1e6)
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

func PostWordCountBigThan10(words string) bool {
	wlen := 0
	wordsSlice := strings.Split(words, " ")
	for _, word := range wordsSlice {
		if word != "" {
			wlen++
		}
	}
	return wlen > 10
}
