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
CREATE TABLE IF NOT EXISTS "userethaddr" (
   "uid" INTEGER PRIMARY KEY AUTOINCREMENT,
   "clientid" TEXT NULL,
   "ethaddress" TEXT NULL,
   "profile" TEXT NULL,
   "bio" TEXT NULL
);
   `
	/*
		CREATE TABLE IF NOT EXISTS "pubmsgscan" (
	   "uid" INTEGER PRIMARY KEY AUTOINCREMENT,
	   "lastscantime" INTEGER NULL,
	   "other1" TEXT NULL,
	   "created" INTEGER default (datetime('now', 'localtime'))
	);
	*/
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
	if lastscantime == 0 {
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
