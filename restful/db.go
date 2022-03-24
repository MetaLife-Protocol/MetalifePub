package restful

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

// PubDB init
type PubDB struct {
	db *sql.DB
	/*lock                    sync.Mutex
	mlock                   sync.Mutex
	Name                    string*/
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
   `
	_, err = db.Exec(sql_table)
	if err != nil {
		return nil, err
	}
	return &PubDB{db: db}, nil
}

//InsertDataCalcTime
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
func (pdb *PubDB) SelectLikeSum(clientid string) (likesum map[string]*LasterNumLikes, err error) { ////LikeCountMap = make(map[string]*LasterNumLikes)
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
