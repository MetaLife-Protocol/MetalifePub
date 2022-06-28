package restful

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ant0ine/go-json-rest/rest"
	"go.cryptoscope.co/ssb/restful/params"
)

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
			resp = NewAPIResponse(err, fmt.Sprintf("Unfollow and block %s failed", req.Defendant))
			return
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
