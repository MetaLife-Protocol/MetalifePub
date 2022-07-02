package restful

import (
	"math/big"
	"time"

	"go.cryptoscope.co/ssb/restful/params"
)

var rewardPeriod = time.Second * 90

/*
RewardDailyTask
巡查数据库usertaskcollect
rewardPeriod处理一次，发送成功则记录，发送不成功则一直重试
			处理：
				1、登录	即时处理，发送激励，每日不超过MaxDailyRewarding
				2、发帖	即时处理，发送激励，每日不超过MaxDailyRewarding
				3、评论	即时处理，发送激励，每日不超过MaxDailyRewarding
				4、NFT	即时处理，发送激励，每日不超过MaxDailyRewarding
			另外：
				5、注册，即时回复，延后激励，条件：在线（重试40分钟）发送激励MLT，接着链上发送SMT
				6、我点赞别人，即时处理，发送激励，每日不超过MaxDailyRewarding
				7、我的受赞，暂时由supernode处理
				4、举报，pub提供接口，发送激励
*/
func RewardProcess() {
	//1、处理usertaskcollect,1-登录 2-发帖 3-评论 4-铸造NFT
	//2、处理未发成功激励的事件
	/*var starttime = req.StartTime
	var endtime = req.EndTime
	taskcollctions, err := likeDB.GetUserTaskCollect(author, msgtype, starttime, endtime)*/
}

//func RecordRewarding2Db()
func ExceedRewardLimit(clientID, rewardType string) bool {

	t := time.Now()
	todaymorning := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	var starttime = todaymorning.UnixNano() / 1e6
	var endtime = time.Now().UnixNano() / 1e6
	//如果存在未发送成功的记录,也记为本次比较的数量，因为延后会继续处理未成功的事件
	num, err := likeDB.SelectHistoryReward(clientID, rewardType, starttime, endtime)
	if err != nil {
		return true
	}
	historyTokens := big.NewInt(int64(num))
	maxRewardTokes := new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(int64(params.MaxDailyRewarding)))
	if historyTokens.Cmp(maxRewardTokes) == -1 {
		return false
	} else {
		return true
	}
	/*taskcollctions, err := likeDB.GetUserTaskCollect(clientID, rewardType, starttime, endtime)
	if err != nil {
		fmt.Println(fmt.Sprintf("do ExceedRewardLimit GetUserTaskCollect err=%s", err))
		return false
	}
	var historyNum = 0
	for _, task := range taskcollctions {
		task.ClientEthAddress
	}*/
	return true
}
