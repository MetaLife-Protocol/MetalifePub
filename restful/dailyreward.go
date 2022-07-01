package restful

import "time"

var rewardPeriod = time.Second * 90

/*
RewardDailyTask
巡查数据库usertaskcollect
rewardPeriod处理一次，发送成功则记录，发送不成功则一直重试

*/
func RewardDailyTask() {

	//taskcollctions, err := likeDB.GetUserTaskCollect("", 1, starttime, endtime)
}
