package restful

import (
	"database/sql"
	"sync"

	"math/big"

	_ "github.com/mattn/go-sqlite3"
	"go.cryptoscope.co/ssb/restful/params"
)

// PubDB init
type PubDB struct {
	db    *sql.DB
	lock  sync.Mutex
	mlock sync.Mutex
	Name  string
}

func OpenPubDB(pubDataSource string) (DB *PubDB, err error) {
	db, err := sql.Open("sqlite3", pubDataSource)
	if err != nil {
		return nil, err
	}

	sql_table := `
CREATE TABLE IF NOT EXISTS "pubmsgscan" (
   "uid" INTEGER PRIMARY KEY AUTOINCREMENT,
   "lastscantime" INTEGER NULL,
   "other1" TEXT NULL,
   "created" INTEGER NULL  
);
CREATE TABLE IF NOT EXISTS "userprofile" (
   "uid" INTEGER PRIMARY KEY AUTOINCREMENT,
   "clientid" TEXT NULL,
   "clientname" TEXT NULL default '',
   "alias" TEXT NULL default '',
   "bio" TEXT NULL default '🇨🇳',
   "other1" TEXT NULL default ''
);
CREATE TABLE IF NOT EXISTS "likedetail" (
   "uid" INTEGER PRIMARY KEY AUTOINCREMENT,
   "messagekey" TEXT NULL,
   "author" TEXT NULL,
   "thismsglikesum" int NULL default 0,
   "liketime" INTEGER NULL default 0
);
CREATE TABLE IF NOT EXISTS "violationrecord" (
   "uid" INTEGER PRIMARY KEY AUTOINCREMENT,
   "recordtime" INTEGER NULL,
   "plaintiff" TEXT NULL,
   "defendant" TEXT NULL,
   "messagekey" TEXT NULL,
   "reasons" TEXT NULL,
   "dealtag" TEXT NULL DEFAULT '0',
   "dealtime" INTEGER NULL,
   "dealreward" TEXT NULL default ''
);
CREATE TABLE IF NOT EXISTS "sensitivewordrecord" (
   "uid" INTEGER PRIMARY KEY AUTOINCREMENT,
   "pubid" TEXT NULL,
   "messagescantime" INTEGER NULL,
   "content" TEXT NULL,
   "messagekey" TEXT NULL,
   "author" TEXT NULL,
   "dealtag" TEXT NULL DEFAULT '0',
   "dealtime" INTEGER NULL
);
CREATE TABLE IF NOT EXISTS "usertaskcollect" (
   "uid" INTEGER PRIMARY KEY AUTOINCREMENT,
   "collectfrompub" TEXT NULL,
   "author" TEXT NULL,
   "messagekey" TEXT NULL,
   "messagetype" TEXT NULL,
   "messageroot" TEXT NULL,
   "messagetime" INTEGER NULL,
   "nfttxhash" TEXT NULL,
   "nfttokenid" TEXT NULL,
   "nftstoreurl" TEXT NULL
);
CREATE TABLE IF NOT EXISTS "usersetlikeinfo" (
   "uid" INTEGER PRIMARY KEY AUTOINCREMENT,
   "messagekey" TEXT NULL,
   "author" TEXT NULL,
   "liketag" int NULL default 0,
   "setliketime" INTEGER NULL default 0
);
CREATE TABLE IF NOT EXISTS "rewardresult" (
   "uid" INTEGER PRIMARY KEY AUTOINCREMENT,
   "clientid" TEXT NULL,
   "ethaddress" TEXT NULL,
   "grantsuccess" TEXT NULL,
   "granttoken" BIGINT NULL default 0,
   "rewardreason" TEXT NULL,
   "messagekey" TEXT NULL,
   "messagetime" INTEGER NULL default 0,
   "rewardtime" INTEGER NULL default 0
);
   `
	_, err = db.Exec(sql_table)
	if err != nil {
		return nil, err
	}
	return &PubDB{db: db}, nil
}

// UpdateRewardResult
func (pdb *PubDB) UpdateRewardResult(cid, partnerAddress, grantSuccess string, msgTime int64) (affectid int64, err error) {
	stmt, err := pdb.db.Prepare("update rewardresult set grantsuccess=? where clientid=? and ethaddress=? and messagetime=?")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(grantSuccess, cid, partnerAddress, msgTime)
	if err != nil {
		return 0, err
	}
	affectid, err = res.LastInsertId()
	return
}

// InsertRewardResult
func (pdb *PubDB) RecordRewardResult(clientId, ethAddress, grantSuccess string, grantToken int64, rewardReason, messageKey string, messageTime, rewardTime int64) (lastid int64, err error) {
	grantToken = grantToken / params.Ether
	stmt, err := pdb.db.Prepare("INSERT INTO rewardresult(clientid,ethaddress,grantsuccess,granttoken,rewardreason,messagekey,messagetime,rewardtime) VALUES (?,?,?,?,?,?,?,?)")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(clientId, ethAddress, grantSuccess, grantToken, rewardReason, messageKey, messageTime, rewardTime)
	if err != nil {
		return 0, err
	}
	lastid, err = res.LastInsertId()
	return
}

// SelectRewardResult
func (pdb *PubDB) SelectRewardResult(clientid string, timefrom, timeto int64) (rinfo []*RewardResult, err error) {
	var rows *sql.Rows
	if clientid == "" {
		rows, err = pdb.db.Query("SELECT * FROM rewardresult where rewardtime>=? and rewardtime<?", timefrom, timeto)
	} else {
		rows, err = pdb.db.Query("SELECT * FROM rewardresult where clientid=? and rewardtime>=? and rewardtime<?", clientid, timefrom, timeto)
	}
	if err != nil {
		return nil, err
	}
	infos := []*RewardResult{}
	defer rows.Close()
	for rows.Next() {
		var uid int64
		var cid string
		var ethaddr string
		var grantsuccess string
		var granttoken int64
		var reason string
		var megkey string
		var msgtime int64
		var rewardtime int64
		err = rows.Scan(&uid, &cid, &ethaddr, &grantsuccess, &granttoken, &reason, &megkey, &msgtime, &rewardtime)
		if err != nil {
			return nil, err
		}
		var r *RewardResult
		amount := new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(granttoken))
		r = &RewardResult{
			ClientID:         cid,
			ClientEthAddress: ethaddr,
			GrantSuccess:     grantsuccess,
			GrantTokenAmount: amount,
			RewardReason:     reason,
			MessageKey:       megkey,
			MessageTime:      msgtime,
			RewardTime:       rewardtime,
		}
		infos = append(infos, r)
	}
	rinfo = infos
	return
}

//SelectRewardSum
func (pdb *PubDB) SelectRewardSum(clientid, grantsuccess string, timefrom, timeto int64) (rsum []*RewardSum, err error) {
	var rows *sql.Rows
	if grantsuccess == "" {
		rows, err = pdb.db.Query("SELECT rewardreason,sum(granttoken) FROM rewardresult where clientid=? and rewardtime>=? and rewardtime<? group by rewardreason", clientid, timefrom, timeto)
	} else {
		rows, err = pdb.db.Query("SELECT rewardreason,sum(granttoken) FROM rewardresult where clientid=? and grantsuccess=? and rewardtime>=? and rewardtime<? group by rewardreason", clientid, grantsuccess, timefrom, timeto)
	}
	if err != nil {
		return nil, err
	}
	infos := []*RewardSum{}
	defer rows.Close()
	for rows.Next() {
		var reason string
		var granttoken int64
		err = rows.Scan(&reason, &granttoken)
		if err != nil {
			return nil, err
		}
		var r *RewardSum
		amount := new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(granttoken))
		r = &RewardSum{
			RewardReason:      reason,
			GrantTokenAmounts: amount,
		}
		infos = append(infos, r)
	}
	rsum = infos
	return
}

// SelectRewardResult
func (pdb *PubDB) SelectHistoryReward(clientId, rewardreason string, starttime, endtime int64) (awardTokenNum int64, err error) {
	rows, err := pdb.db.Query("SELECT count(granttoken) FROM rewardresult where clientid=? and rewardreason=? and rewardtime>=? AND rewardtime<?", clientId, rewardreason, starttime, endtime)
	if err != nil {
		return 0, err
	}
	awardTokenNum = 0
	defer rows.Close()
	for rows.Next() {
		var awardtokennum int64
		errnil := rows.Scan(&awardtokennum)
		if errnil != nil {
			return 0, errnil
		}
		awardTokenNum = awardtokennum
		break
	}
	return
}

// InsertUserSetLike liketag=1 or -1
func (pdb *PubDB) InsertUserSetLikeInfo(messagekey, author string, liketag int, setliketime int64) (lastid int64, err error) {
	stmt, err := pdb.db.Prepare("INSERT INTO usersetlikeinfo(messagekey,author,liketag,setliketime) VALUES (?,?,?,?)")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(messagekey, author, liketag, setliketime)
	if err != nil {
		return 0, err
	}
	lastid, err = res.LastInsertId()
	return
}

//SelectLastScanTime
func (pdb *PubDB) SelectUserSetLikeInfo(clientid string) (likesum map[string]*LasterNumLikes, err error) {
	var rows *sql.Rows
	if clientid == "" {
		rows, err = pdb.db.Query("SELECT usersetlikeinfo.author,usersetlikeinfo.liketag,userprofile.clientname,userprofile.other1 FROM usersetlikeinfo left outer join userprofile on usersetlikeinfo.author=userprofile.clientid")
	} else {
		rows, err = pdb.db.Query("SELECT usersetlikeinfo.author,usersetlikeinfo.liketag,userprofile.clientname,userprofile.other1 FROM usersetlikeinfo left outer join userprofile on usersetlikeinfo.author=userprofile.clientid where usersetlikeinfo.author=?", clientid)
	}
	if err != nil {
		return nil, err
	}
	likeCountMap := make(map[string]*LasterNumLikes)
	defer rows.Close()
	for rows.Next() {
		var author string
		var onemsglikes int
		var cname string
		var ethaddr string
		errnil := rows.Scan(&author, &onemsglikes, &cname, &ethaddr)
		if errnil != nil {
			continue
			//return nil, err
		}
		var l *LasterNumLikes
		l = &LasterNumLikes{
			ClientID:         author,
			LasterLikeNum:    onemsglikes,
			Name:             cname,
			ClientEthAddress: ethaddr,
			MessageFromPub:   params.PubID,
		}
		if _, ok := likeCountMap[author]; ok {
			likeCountMap[author].LasterLikeNum += onemsglikes
		} else {
			likeCountMap[author] = l
		}
	}
	likesum = likeCountMap
	return
}

//InsertDataCalcTime  Violation record
func (pdb *PubDB) InsertLastScanTime(ts int64) (lastid int64, err error) {
	stmt, err := pdb.db.Prepare("INSERT INTO pubmsgscan(lastscantime) VALUES (?)")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(ts)
	if err != nil {
		return 0, err
	}
	lastid, err = res.LastInsertId()

	return
}

//UpdateLastScanTime
func (pdb *PubDB) UpdateLastScanTime(ts int64) (affectid int64, err error) {

	lastscantime, err := pdb.SelectLastScanTime()
	if err != nil {
		return 0, err
	}
	if lastscantime == -1 {
		pdb.InsertLastScanTime(ts)
		return 1, nil
	}
	stmt, err := pdb.db.Prepare("update pubmsgscan set lastscantime=?")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(ts)
	if err != nil {
		return 0, err
	}
	affectid, err = res.LastInsertId()
	return
}

//SelectLastScanTime
func (pdb *PubDB) SelectLastScanTime() (lastscantime int64, err error) {
	rows, err := pdb.db.Query("SELECT lastscantime FROM pubmsgscan limit 1")
	if err != nil {
		return 0, err
	}
	lastscantime = -1
	//rows的数据类型是*sql.Rows，rows调用Close()方法代表读结束
	defer rows.Close()
	for rows.Next() {
		var lasttime int64

		err = rows.Scan(&lasttime)
		if err != nil {
			return 0, err
		}
		lastscantime = lasttime
		break
	}
	return
}

//DeleteLastScanTime
func (pdb *PubDB) DeleteLastScanTime() (affectid int64, err error) {
	stmt, err := pdb.db.Prepare("delete from userinfo")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec()
	if err != nil {
		return 0, err
	}
	affectid, err = res.LastInsertId()

	return
}

// InsertUserProfile
func (pdb *PubDB) InsertUserProfile(clientid, cname, other1 string) (lastid int64, err error) {
	stmt, err := pdb.db.Prepare("INSERT INTO userprofile(clientid,clientname,other1) VALUES (?,?,?)")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(clientid, cname, other1)
	if err != nil {
		return 0, err
	}
	lastid, err = res.LastInsertId()

	return
}

// UpdateUserProfile
func (pdb *PubDB) UpdateUserProfile(clientid, cname, other1 string) (affectid int64, err error) {
	profile, err := pdb.SelectUserProfile(clientid)
	if err != nil {
		return 0, err
	}
	if len(profile) == 0 {
		_, err = pdb.InsertUserProfile(clientid, cname, other1)
		if err != nil {
			return 0, err
		}
		return 1, nil
	}
	var stmt *sql.Stmt
	if other1 == "" {
		stmt, err = pdb.db.Prepare("update userprofile set clientname=? WHERE clientid=?")
		if err != nil {
			return 0, err
		}
		res, err := stmt.Exec(cname, clientid)
		if err != nil {
			return 0, err
		}
		affectid, err = res.LastInsertId()

	} else {
		stmt, err = pdb.db.Prepare("update userprofile set other1=? WHERE clientid=?")
		if err != nil {
			return 0, err
		}
		res, err := stmt.Exec(other1, clientid)
		if err != nil {
			return 0, err
		}
		affectid, err = res.LastInsertId()
	}
	return
}

// SelectUserProfile
func (pdb *PubDB) SelectUserProfile(clientid string) (name2profile []*Name2ProfileReponse, err error) {
	var rows *sql.Rows
	if clientid == "" {
		rows, err = pdb.db.Query("SELECT * FROM userprofile")
	} else {
		rows, err = pdb.db.Query("SELECT * FROM userprofile where clientid=?", clientid)
	}
	if err != nil {
		return nil, err
	}
	name2prof := []*Name2ProfileReponse{}
	defer rows.Close()
	for rows.Next() {
		var uid int64
		var cid string
		var cname string
		var alias string
		var bio string
		var other1 string
		err = rows.Scan(&uid, &cid, &cname, &alias, &bio, &other1)
		if err != nil {
			return nil, err
		}
		var n *Name2ProfileReponse
		n = &Name2ProfileReponse{
			ID:         cid,
			Name:       cname,
			Alias:      alias,
			Bio:        bio,
			EthAddress: other1,
		}
		name2prof = append(name2prof, n)
	}
	name2profile = name2prof
	return
}

//InsertLikeDetail
func (pdb *PubDB) InsertLikeDetail(msgid, author string) (lastid int64, err error) {
	stmt, err := pdb.db.Prepare("INSERT INTO likedetail(messagekey,author) VALUES (?,?)")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(msgid, author)
	if err != nil {
		return 0, err
	}
	lastid, err = res.LastInsertId()

	return
}

//UpdateLikeDetail
func (pdb *PubDB) UpdateLikeDetail(liketag int, ts int64, msgid string) (affectid int64, err error) {
	stmt, err := pdb.db.Prepare("update likedetail set thismsglikesum=thismsglikesum+?,liketime=? where messagekey=?")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(liketag, ts, msgid)
	if err != nil {
		return 0, err
	}
	affectid, err = res.LastInsertId()
	return
}

//SelectLastScanTime
func (pdb *PubDB) SelectLikeSum(clientid string) (likesum map[string]*LasterNumLikes, err error) {
	var rows *sql.Rows
	if clientid == "" {
		rows, err = pdb.db.Query("SELECT likedetail.author,likedetail.thismsglikesum,userprofile.clientname,userprofile.other1 FROM likedetail left outer join userprofile on likedetail.author=userprofile.clientid")
	} else {
		rows, err = pdb.db.Query("SELECT likedetail.author,likedetail.thismsglikesum,userprofile.clientname,userprofile.other1 FROM likedetail left outer join userprofile on likedetail.author=userprofile.clientid where likedetail.author=?", clientid)
	}
	if err != nil {
		return nil, err
	}
	likeCountMap := make(map[string]*LasterNumLikes)
	defer rows.Close()
	for rows.Next() {
		var cid string
		var onemsglikes int
		var cname string
		var ethaddr string
		errnil := rows.Scan(&cid, &onemsglikes, &cname, &ethaddr)
		if errnil != nil {
			continue
			//return nil, err
		}
		var l *LasterNumLikes
		l = &LasterNumLikes{
			ClientID:         cid,
			LasterLikeNum:    onemsglikes,
			Name:             cname,
			ClientEthAddress: ethaddr,
			MessageFromPub:   params.PubID,
		}
		if _, ok := likeCountMap[cid]; ok {
			likeCountMap[cid].LasterLikeNum += onemsglikes
		} else {
			likeCountMap[cid] = l
		}
	}
	likesum = likeCountMap
	return
}

//InsertViolation  Violation record
func (pdb *PubDB) InsertViolation(recordtime int64, plaintiff, defendant, messagekey, reason string) (lastid int64, err error) {
	xnum, err := pdb.CountViolationByWhere(plaintiff, defendant, messagekey)
	if err != nil {
		return 0, err
	}
	if xnum > 0 {
		return -1, err
	}

	stmt, err := pdb.db.Prepare("INSERT INTO violationrecord(recordtime,plaintiff,defendant,messagekey,reasons,dealtime) VALUES (?,?,?,?,?,?)")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(recordtime, plaintiff, defendant, messagekey, reason, recordtime)
	if err != nil {
		return 0, err
	}
	lastid, err = res.LastInsertId()

	return
}

func (pdb *PubDB) UpdateViolation(dealtag string, dealtime int64, dealreward, plaintiff, defendant, messagekey string) (affectid int64, err error) {
	stmt, err := pdb.db.Prepare("update violationrecord set dealtag=?,dealtime=?,dealreward=? where plaintiff=? and defendant=? and messagekey=?")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(dealtag, dealtime, dealreward, plaintiff, defendant, messagekey)
	if err != nil {
		return 0, err
	}
	affectid, err = res.LastInsertId()
	return
}

//CountViolationByWhere
func (pdb *PubDB) CountViolationByWhere(lplaintiff, defendant, messagekey string) (num int, err error) {
	rows, err := pdb.db.Query("SELECT count(*) FROM violationrecord where plaintiff=? and defendant=? and messagekey=?", lplaintiff, defendant, messagekey)
	if err != nil {
		return 0, err
	}
	num = 0
	defer rows.Close()
	for rows.Next() {
		var count int
		err = rows.Scan(&count)
		if err != nil {
			return 0, err
		}
		num = count
		break
	}
	return
}

//SelectLastScanTime
func (pdb *PubDB) SelectViolationByWhere(plaintiff, defendant, messagekey, reasons, dealtag string) (num []*TippedOffStu, err error) {
	sqlstr := "SELECT * FROM violationrecord"
	if plaintiff != "" || defendant != "" || messagekey != "" || reasons != "" || dealtag != "" {
		sqlstr += " where uid!=-1"
		if plaintiff != "" {
			sqlstr += " and plaintiff='" + plaintiff + "'"
		}
		if defendant != "" {
			sqlstr += " and defendant='" + defendant + "'"
		}
		if messagekey != "" {
			sqlstr += " and messagekey='" + messagekey + "'"
		}
		if reasons != "" {
			sqlstr += " and reasons='" + reasons + "'"
		}
		if dealtag != "" {
			sqlstr += " and dealtag='" + dealtag + "'"
		}
	}
	//fmt.Println(sqlstr)
	rows, err := pdb.db.Query(sqlstr)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		var xuid int64
		var xplaintiff string
		var xdefendant string
		var xmessageKey string
		var xreasons string
		var xdealTag string
		var xrecordtime int64
		var xdealtime int64
		var xdealreward string
		errnil := rows.Scan(&xuid, &xrecordtime, &xplaintiff, &xdefendant, &xmessageKey, &xreasons, &xdealTag, &xdealtime, &xdealreward)
		if errnil != nil {
			continue
			//return nil, err
		}
		var l *TippedOffStu
		l = &TippedOffStu{
			Plaintiff:  xplaintiff,
			Defendant:  xdefendant,
			MessageKey: xmessageKey,
			Reasons:    xreasons,
			DealTag:    xdealTag,
			Recordtime: xrecordtime,
			Dealtime:   xdealtime,
			Dealreward: xdealreward,
		}
		num = append(num, l)
	}
	return
}

// InsertSensitiveWordRecord dealtag:0-init data 1-right 2-no
func (pdb *PubDB) InsertSensitiveWordRecord(pubid string, messagetime int64, content, messagekey, author, dealtag string) (lastid int64, err error) {
	stmt, err := pdb.db.Prepare("INSERT INTO sensitivewordrecord(pubid,messagescantime,content,messagekey,author,dealtag,dealtime) VALUES (?,?,?,?,?,?,0)")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(pubid, messagetime, content, messagekey, author, dealtag)
	if err != nil {
		return 0, err
	}
	lastid, err = res.LastInsertId()

	return
}

// UpdateSensitiveWordRecord
func (pdb *PubDB) UpdateSensitiveWordRecord(dealtag string, dealtime int64, messagekey string) (affectid int64, err error) {
	stmt, err := pdb.db.Prepare("update sensitivewordrecord set dealtag=?,dealtime=? where messagekey=?")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(dealtag, dealtime, messagekey)
	if err != nil {
		return 0, err
	}
	affectid, err = res.LastInsertId()
	return
}

// SelectSensitiveWordRecord
func (pdb *PubDB) SelectSensitiveWordRecord(selecttag string) (eventsSensitiveWord []*EventSensitive, err error) {
	rows, err := pdb.db.Query("SELECT * FROM sensitivewordrecord where dealtag=?", selecttag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var xuid int64
		var xpubid string
		var xmessagescantime int64
		var xcontent string
		var xmessagekey string
		var xauthor string
		var xdealtag string
		var xdealtime int64

		errnil := rows.Scan(&xuid, &xpubid, &xmessagescantime, &xcontent, &xmessagekey, &xauthor, &xdealtag, &xdealtime)
		if errnil != nil {
			continue
			//return nil, err
		}
		var e *EventSensitive
		e = &EventSensitive{
			PubID:           xpubid,
			MessageScanTime: xmessagescantime,
			MessageText:     xcontent,
			MessageKey:      xmessagekey,
			MessageAuthor:   xauthor,
			DealTag:         xdealtag,
			DealTime:        xdealtime,
		}
		eventsSensitiveWord = append(eventsSensitiveWord, e)
	}
	return
}

// InsertUserTaskCollect messagetype:1-登录 2-发表帖子 3-评论 4-铸造NFT
func (pdb *PubDB) InsertUserTaskCollect(pubid, author, messagekey, messagetype, messageroot string, messagetime int64, nfttxhash, nfttokenid, nftstoreurl string) (lastid int64, err error) {
	stmt, err := pdb.db.Prepare("INSERT INTO usertaskcollect(collectfrompub,author,messagekey,messagetype,messageroot,messagetime,nfttxhash,nfttokenid,nftstoreurl) VALUES (?,?,?,?,?,?,?,?,?)")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(pubid, author, messagekey, messagetype, messageroot, messagetime, nfttxhash, nfttokenid, nftstoreurl)
	if err != nil {
		return 0, err
	}
	lastid, err = res.LastInsertId()

	return
}

// GetUserTaskCollect
func (pdb *PubDB) GetUserTaskCollect(author, messagetype string, starttime, endtime int64) (usertasks []*UserTasks, err error) {
	var rows *sql.Rows
	if author != "" {
		rows, err = pdb.db.Query("SELECT usertaskcollect.collectfrompub,"+
			"usertaskcollect.author,"+
			"usertaskcollect.messagekey,"+
			"usertaskcollect.messagetype,"+
			"usertaskcollect.messageroot,"+
			"usertaskcollect.messagetime,"+
			"usertaskcollect.nfttxhash,"+
			"usertaskcollect.nfttokenid,"+
			"usertaskcollect.nftstoreurl,"+
			"userprofile.other1 "+
			"FROM usertaskcollect left outer join userprofile on usertaskcollect.author=userprofile.clientid "+
			"WHERE usertaskcollect.author=? AND usertaskcollect.messagetype=? AND usertaskcollect.messagetime>=? AND usertaskcollect.messagetime<=?", author, messagetype, starttime, endtime)
	} else {
		rows, err = pdb.db.Query("SELECT usertaskcollect.collectfrompub,"+
			"usertaskcollect.author,"+
			"usertaskcollect.messagekey,"+
			"usertaskcollect.messagetype,"+
			"usertaskcollect.messageroot,"+
			"usertaskcollect.messagetime,"+
			"usertaskcollect.nfttxhash,"+
			"usertaskcollect.nfttokenid,"+
			"usertaskcollect.nftstoreurl,"+
			"userprofile.other1 "+
			"FROM usertaskcollect left outer join userprofile on usertaskcollect.author=userprofile.clientid "+
			"WHERE usertaskcollect.messagetype=? AND usertaskcollect.messagetime>=? AND usertaskcollect.messagetime<=?", messagetype, starttime, endtime)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var collectfrompub string
		var author string
		var messagekey string
		var messagetype string
		var messageroot string
		var messagetime int64
		var nfttxhash string
		var nfttokenid string
		var nftstoreurl string
		var clientethaddr string

		errnil := rows.Scan(&collectfrompub, &author, &messagekey, &messagetype, &messageroot, &messagetime, &nfttxhash, &nfttokenid, &nftstoreurl, &clientethaddr)
		if errnil != nil {
			continue
		}
		var e *UserTasks
		e = &UserTasks{
			CollectFromPub:   collectfrompub,
			Author:           author,
			MessageKey:       messagekey,
			MessageType:      messagetype,
			MessageRoot:      messageroot,
			MessageTime:      messagetime,
			NfttxHash:        nfttxhash,
			NftTokenId:       nfttokenid,
			NftStoredUrl:     nftstoreurl,
			ClientEthAddress: clientethaddr,
		}
		usertasks = append(usertasks, e)
	}
	return
}
