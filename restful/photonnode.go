package restful

import (
	"encoding/json"
	"fmt"

	"errors"
	"go.cryptoscope.co/ssb/restful/channel"
	"math/big"
	"net/http"
	"time"
)

//var log kitlog.Logger

// PhotonNode a photon node
type PhotonNode struct {
	Host       string
	Address    string
	Name       string
	APIAddress string

	DebugCrash bool
	Running    bool
}

type TransferPayload struct {
	Amount   *big.Int `json:"amount"`
	IsDirect bool     `json:"is_direct"`
	Secret   string   `json:"secret"`
	Sync     bool     `json:"sync"`
	//RouteInfo []pfsproxy.FindPathResponse `json:"route_info,omitempty"`
	Data string `json:"data"`
}

// ChannelBigInt
type Channel struct {
	Name                string   `json:"name"`
	SelfAddress         string   `json:"self_address"`
	ChannelIdentifier   string   `json:"channel_identifier"`
	PartnerAddress      string   `json:"partner_address"`
	Balance             *big.Int `json:"balance"`
	LockedAmount        *big.Int `json:"locked_amount"`
	PartnerBalance      *big.Int `json:"partner_balance"`
	PartnerLockedAmount *big.Int `json:"partner_locked_amount"`
	TokenAddress        string   `json:"token_address"`
	State               int      `json:"state"`
	SettleTimeout       *big.Int `json:"settle_timeout"`
	RevealTimeout       *big.Int `json:"reveal_timeout"`
}

// PhotonNodeRuntime
type PhotonNodeRuntime struct {
	MainChainBalance *big.Int // 主链货币余额
}

// GetChannelWithBigInt :
func (node *PhotonNode) GetChannelWithBigInt(partnerNode *PhotonNode, tokenAddr string) (*Channel, error) {
	req := &Req{
		FullURL: node.Host + "/api/1/channels",
		Method:  http.MethodGet,
		Payload: "",
		Timeout: time.Second * 30,
	}
	body, err := req.Invoke()
	if err != nil {
		return nil, err
	}
	var nodeChannels []Channel
	err = json.Unmarshal(body, &nodeChannels)
	if err != nil {
		fmt.Println(fmt.Sprintf("GetChannel Unmarshal err= %s", err))
		return nil, err
	}
	if len(nodeChannels) == 0 {
		return nil, nil
	}
	for _, channel := range nodeChannels {
		if channel.PartnerAddress == partnerNode.Address && channel.TokenAddress == tokenAddr {
			channel.SelfAddress = node.Address
			channel.Name = "CH-" + node.Name + "-" + partnerNode.Name
			return &channel, nil
		}
	}
	return nil, nil
}

// OpenChannel :
func (node *PhotonNode) OpenChannelBigInt(partnerAddress, tokenAddress string, balance *big.Int, settleTimeout int, waitSeconds ...int) error {
	type OpenChannelPayload struct {
		PartnerAddress string   `json:"partner_address"`
		TokenAddress   string   `json:"token_address"`
		Balance        *big.Int `json:"balance"`
		SettleTimeout  int      `json:"settle_timeout"`
		NewChannel     bool     `json:"new_channel"`
	}
	p, err := json.Marshal(OpenChannelPayload{
		PartnerAddress: partnerAddress,
		TokenAddress:   tokenAddress,
		Balance:        balance,
		SettleTimeout:  settleTimeout,
		NewChannel:     true,
	})
	req := &Req{
		FullURL: node.Host + "/api/1/deposit",
		Method:  http.MethodPut,
		Payload: string(p),
		Timeout: time.Second * 60,
	}
	body, err := req.Invoke()
	if err != nil {
		fmt.Println(fmt.Sprintf("[Pub]OpenChannelApi err %s", err))
		return err
	}
	fmt.Println(fmt.Sprintf("[Pub]OpenChannelApi returned %s", string(body)))
	ch := channel.ChannelDataDetail{}
	err = json.Unmarshal(body, &ch)
	if err != nil {
		fmt.Println(fmt.Sprintf("OpenChannel Unmarshal err= %s", err))
		//panic(err)
	}
	var ws int
	if len(waitSeconds) > 0 {
		ws = waitSeconds[0]
	} else {
		ws = 45 //d等三块,应该会被打包进去的.
	}
	var i int
	for i = 0; i < ws; i++ {
		time.Sleep(time.Second * 3)
		_, err = node.SpecifiedChannel(ch.ChannelIdentifier)
		//找到这个channel了才返回
		if err == nil {
			break
		}
	}
	if i == ws {
		//return errors.New("timeout")
		return errors.New("timeout")
	}
	return nil
}

func (node *PhotonNode) SpecifiedChannel(channelIdentifier string) (c channel.ChannelDataDetail, err error) {
	req := &Req{
		FullURL: fmt.Sprintf(node.Host+"/api/1/channels/%s", channelIdentifier),
		Method:  http.MethodGet,
		Timeout: time.Second * 20,
	}
	body, err := req.Invoke()
	if err != nil {
		//log.Error(fmt.Sprintf("[SuperNode]SpecifiedChannel err :%s", err))
		fmt.Println(fmt.Sprintf("[Pub]SpecifiedChannel err %s", err))
		return
	}
	err = json.Unmarshal(body, &c)
	if err != nil {
		return
	}
	return

}

func (node *PhotonNode) SendTrans(tokenAddress string, amount *big.Int, targetAddress string, isDirect bool, sync bool) error {
	p, err := json.Marshal(TransferPayload{
		Amount:   amount,
		IsDirect: isDirect,
		Sync:     sync,
	})
	req := &Req{
		FullURL: node.Host + "/api/1/transfers/" + tokenAddress + "/" + targetAddress,
		Method:  http.MethodPost,
		Payload: string(p),
		Timeout: time.Second * 60,
	}
	body, err := req.Invoke()
	if err != nil {
		fmt.Println(fmt.Sprintf("[Pub]SendTransApi err=%s,body=%s ", err, string(body)))
	}
	return err
}

func (node *PhotonNode) Deposit(partnerAddress, tokenAddress string, balance *big.Int, waitSeconds ...int) error {
	type OpenChannelPayload struct {
		PartnerAddress string   `json:"partner_address"`
		TokenAddress   string   `json:"token_address"`
		Balance        *big.Int `json:"balance"`
		SettleTimeout  int64    `json:"settle_timeout"`
		NewChannel     bool     `json:"new_channel"`
	}
	p, err := json.Marshal(OpenChannelPayload{
		PartnerAddress: partnerAddress,
		TokenAddress:   tokenAddress,
		Balance:        balance,
		SettleTimeout:  0,
		NewChannel:     false,
	})
	req := &Req{
		FullURL: node.Host + "/api/1/deposit",
		Method:  http.MethodPut,
		Payload: string(p),
		Timeout: time.Second * 20,
	}
	body, err := req.Invoke()
	if err != nil {
		fmt.Println(fmt.Sprintf("[Pub]DepositApi err=%s ", err))

		return err
	}
	fmt.Println(fmt.Sprintf("[Pub]Deposit returned=%s ", string(body)))
	ch := channel.ChannelDataDetail{}
	err = json.Unmarshal(body, &ch)
	if err != nil {
		fmt.Println(fmt.Sprintf("Deposit Unmarshal err= %s", err))
		//panic(err)
	}
	var ws int
	if len(waitSeconds) > 0 {
		ws = waitSeconds[0]
	} else {
		ws = 45 //d等三块,应该会被打包进去的.
	}
	var i int
	for i = 0; i < ws; i++ {
		time.Sleep(time.Second * 3)
		_, err = node.SpecifiedChannel(ch.ChannelIdentifier)
		//找到这个channel了才返回
		if err == nil {
			break
		}
	}
	if i == ws {
		return errors.New("timeout")
	}
	return nil
}
